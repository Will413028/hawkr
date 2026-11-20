package live

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Streams 是串流存活標記的儲存：每條訂閱一列，同時是 presence 的來源與每個 Party 並行串流數的全域計數。
type Streams interface {
	// Open 在一個 transaction 內以 advisory lock 序列化同一個 Party，數未過期的列，已達 max 回 ResourceExhausted，否則寫入本串流的列。
	Open(ctx context.Context, kind model.PartyKind, partyID int64, id uuid.UUID, max int, ttl time.Duration) error
	// Extend 延長 ids 的存活期。
	Extend(ctx context.Context, ids []uuid.UUID, ttl time.Duration) error
	// Delete 刪除 ids 的列。
	Delete(ctx context.Context, ids []uuid.UUID) error
}

// DBStreams 是以資料庫列實作的 Streams；時間只由資料庫的 now() 產生與比較。
type DBStreams struct{ DB *db.DB }

func (d DBStreams) Open(ctx context.Context, kind model.PartyKind, partyID int64, id uuid.UUID, max int, ttl time.Duration) error {
	return d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(d.DB.Conn(ctx))
		if err := q.LockPartyStreams(ctx, store.LockPartyStreamsParams{Kind: string(kind), PartyID: partyID}); err != nil {
			return err
		}
		n, err := q.CountLivePartyStreams(ctx, store.CountLivePartyStreamsParams{Kind: string(kind), PartyID: partyID})
		if err != nil {
			return err
		}
		if n >= int64(max) {
			return apperr.ResourceExhausted(0, "%s", text.ImStreamLimit(uitext.LocaleFrom(ctx)))
		}
		return q.InsertPresenceStream(ctx, store.InsertPresenceStreamParams{StreamID: id, Kind: string(kind), PartyID: partyID, TtlMs: ttl.Milliseconds()})
	})
}

func (d DBStreams) Extend(ctx context.Context, ids []uuid.UUID, ttl time.Duration) error {
	_, err := store.New(d.DB.Conn(ctx)).ExtendPresenceStreams(ctx, store.ExtendPresenceStreamsParams{Ids: ids, TtlMs: ttl.Milliseconds()})
	return err
}

func (d DBStreams) Delete(ctx context.Context, ids []uuid.UUID) error {
	return store.New(d.DB.Conn(ctx)).DeletePresenceStreams(ctx, ids)
}
