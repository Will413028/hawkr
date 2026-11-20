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
	// Extend 延長 streams 的存活期；列已經不在（被清理、或資料庫重啟後 UNLOGGED 表是空的）的串流補回，所以仍被計數。
	Extend(ctx context.Context, streams []Stream, ttl time.Duration) error
	// Delete 刪除 ids 的列。
	Delete(ctx context.Context, ids []uuid.UUID) error
	// DatabaseNow 是資料庫目前的時間。
	DatabaseNow(ctx context.Context) (time.Time, error)
}

// Stream 是一條仍在跑的訂閱的存活標記身分。
type Stream struct {
	ID      uuid.UUID
	Kind    model.PartyKind
	PartyID int64
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

func (d DBStreams) Extend(ctx context.Context, streams []Stream, ttl time.Duration) error {
	p := store.ExtendPresenceStreamsParams{TtlMs: ttl.Milliseconds()}
	for _, st := range streams {
		p.Ids, p.Kinds, p.PartyIds = append(p.Ids, st.ID), append(p.Kinds, string(st.Kind)), append(p.PartyIds, st.PartyID)
	}
	_, err := store.New(d.DB.Conn(ctx)).ExtendPresenceStreams(ctx, p)
	return err
}

func (d DBStreams) DatabaseNow(ctx context.Context) (time.Time, error) {
	return store.New(d.DB.Conn(ctx)).DatabaseNow(ctx)
}

// Sweep 刪除過期超過 10 分鐘的存活標記，回傳刪除的列數；讀取只認未過期的列，所以這只是清理。
func (d DBStreams) Sweep(ctx context.Context) (int64, error) {
	return store.New(d.DB.Conn(ctx)).SweepPresenceStreams(ctx)
}

func (d DBStreams) Delete(ctx context.Context, ids []uuid.UUID) error {
	return store.New(d.DB.Conn(ctx)).DeletePresenceStreams(ctx, ids)
}
