// Package write 是 im 的寫入路徑：送出、已讀、隱藏與還原。
package write

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/live"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Buyers 是送出用到的 buyer 公開 API；正式組裝傳 *buyer.Module。
type Buyers interface {
	AccountIDByBuyerID(ctx context.Context, buyerID int64) (int64, error)
	ProfileByBuyerID(ctx context.Context, buyerID int64) (buyer.Profile, error)
	IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error)
	// LockNotClosed 在 transaction 內以 FOR KEY SHARE 確認買家尚未關閉（刪除中放行）；已關閉與不存在回 NotFound。
	LockNotClosed(ctx context.Context, buyerID int64) error
}

// Stores 是送出用到的 seller 公開 API；正式組裝傳 *seller.Module。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
	// StoresBlockingBuyer 與 BuyersBlockedByStore 是 Presence 的批次查詢（回傳的 map 只含封鎖的一方）。
	StoresBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error)
	BuyersBlockedByStore(ctx context.Context, storeID int64, buyerIDs []int64) (map[int64]bool, error)
	// LockNotRetired 在 transaction 內以 FOR KEY SHARE 確認店家尚未退場；已退場與不存在回 NotFound。
	LockNotRetired(ctx context.Context, storeID int64) error
}

// Deps 是寫入路徑的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Media  model.Media
	Buyers Buyers
	Stores Stores
	Orders model.Orders
	NewID  func() uuid.UUID
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 是寫入路徑的 use case。
type Service struct {
	d Deps
	// inactive 回報一方是否已關閉（買家）或已退場（店家），讀擁有者的 pub 函式、不取鎖；預設讀資料庫，守門的單元測試以替身取代。
	inactive func(ctx context.Context, kind model.PartyKind, id int64) (bool, error)
}

// New 建立寫入路徑。
func New(d Deps) *Service {
	s := &Service{d: d}
	s.inactive = func(ctx context.Context, kind model.PartyKind, id int64) (bool, error) {
		return s.q(ctx).PartyInactive(ctx, store.PartyInactiveParams{Kind: string(kind), PartyID: id})
	}
	return s
}

// inTx 開 im 的寫入 transaction 並設時間上限（model.WriteTxTimeout）；ctx 已在外層 transaction 內時加入它，上限由外層決定，不覆寫。
func (s *Service) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	_, nested := db.TxFrom(ctx)
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if !nested {
			ms := strconv.FormatInt(model.WriteTxTimeout.Milliseconds(), 10)
			if _, err := s.d.DB.Conn(ctx).Exec(ctx,
				`SELECT set_config('statement_timeout', $1, true), set_config('idle_in_transaction_session_timeout', $1, true), set_config('transaction_timeout', $1, true)`, ms); err != nil {
				return err
			}
		}
		return fn(ctx)
	})
}

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// MarkRead 把呼叫者這一方在對話的已讀游標推到 uptoSeq：只前進不後退，也不超過 last_seq。對話不屬於呼叫者回 NotFound。
// 游標真的前進時在同一個 transaction 發 NOTIFY（r）；冪等重送不發。
func (s *Service) MarkRead(ctx context.Context, conversationID uuid.UUID, p model.Party, uptoSeq int64) error {
	l := uitext.LocaleFrom(ctx)
	if err := p.Authorize(); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var buyerID, storeID, readSeq int64
		var advanced bool
		var side string
		var err error
		if p.Kind == model.PartyBuyer {
			var r store.MarkReadBuyerRow
			r, err = q.MarkReadBuyer(ctx, store.MarkReadBuyerParams{UptoSeq: uptoSeq, ID: conversationID, BuyerID: p.BuyerID})
			buyerID, storeID, readSeq, advanced, side = r.BuyerID, r.StoreID, r.ReadSeq, r.Advanced, live.SideBuyer
		} else {
			var r store.MarkReadStoreRow
			r, err = q.MarkReadStore(ctx, store.MarkReadStoreParams{UptoSeq: uptoSeq, ID: conversationID, StoreID: p.Store.StoreID})
			buyerID, storeID, readSeq, advanced, side = r.BuyerID, r.StoreID, r.ReadSeq, r.Advanced, live.SideStore
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.ImConversationNotFound(l))
		}
		if err != nil {
			return err
		}
		if !advanced {
			return nil
		}
		return q.Notify(ctx, live.Read(conversationID, buyerID, storeID, readSeq, side).Encode())
	})
}

// MarkAllRead 把呼叫者這一方的全部對話標成已讀。有對話被改動時只發一個 NOTIFY（u，只帶這一方），不逐對話發，也不通知對方。
func (s *Service) MarkAllRead(ctx context.Context, p model.Party) error {
	if err := p.Authorize(); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var n int64
		var err error
		payload := live.ReadBatch(p.BuyerID, 0)
		if p.Kind == model.PartyBuyer {
			n, err = q.MarkAllReadBuyer(ctx, p.BuyerID)
		} else {
			payload = live.ReadBatch(0, p.Store.StoreID)
			n, err = q.MarkAllReadStore(ctx, p.Store.StoreID)
		}
		if err != nil || n == 0 {
			return err
		}
		return q.Notify(ctx, payload.Encode())
	})
}

// Hide 隱藏一則訊息（moderation 的 transaction 內呼叫）；已有未還原的隱藏時回既有的那一筆（Created 為假，冪等）。
// 預覽在讀取時推導，所以隱藏不必更新對話列。
func (s *Service) Hide(ctx context.Context, in model.HideInput) (model.Hide, error) {
	in.Text, in.SourceRef = trimtext.Trim(in.Text), trimtext.Trim(in.SourceRef)
	if err := checkReason(in); err != nil {
		return model.Hide{}, err
	}
	var out model.Hide
	err := s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		msg, err := q.LockMessage(ctx, in.MessageID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("%s", text.HideMessageNotFound())
			}
			return err
		}
		id := s.d.NewID()
		row, err := q.InsertHide(ctx, store.InsertHideParams{
			ID: id, MessageID: in.MessageID, ReasonCategory: in.Category, ReasonText: in.Text, SourceRef: optional(in.SourceRef), AdminID: in.AdminID, Now: s.d.Now().UTC(),
		})
		if err != nil {
			return err
		}
		out = rows.Hide(row, row.ID == id)
		if row.ID != id {
			return nil
		}
		return q.Notify(ctx, live.Visible(msg.ConversationID, msg.BuyerID, msg.StoreID, msg.Seq).Encode())
	})
	return out, err
}

// Restore 是管理員更正：還原目前未還原的隱藏（留列不刪，之後可以再隱藏）。沒有未還原的隱藏回 FailedPrecondition。
func (s *Service) Restore(ctx context.Context, in model.RestoreInput) (model.Hide, error) {
	in.Reason = trimtext.Trim(in.Reason)
	if in.AdminID <= 0 {
		return model.Hide{}, apperr.InvalidInputOn([]string{"AdminID"}, "%s", text.HideAdminRequired())
	}
	if !utf8.ValidString(in.Reason) {
		return model.Hide{}, apperr.InvalidInputOn([]string{"Reason"}, "%s", text.HideTextInvalid())
	}
	if n := utf8.RuneCountInString(in.Reason); n < 1 || n > model.MaxHideReasonRunes {
		return model.Hide{}, apperr.InvalidInputOn([]string{"Reason"}, "%s", text.HideRestoreReason(text.HideRestoreReasonArgs{Max: model.MaxHideReasonRunes}))
	}
	var out model.Hide
	err := s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		msg, err := q.LockMessage(ctx, in.MessageID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("%s", text.HideMessageNotFound())
			}
			return err
		}
		row, err := q.RestoreHide(ctx, store.RestoreHideParams{MessageID: in.MessageID, AdminID: &in.AdminID, Reason: &in.Reason, Now: s.d.Now().UTC(), SourceRef: optional(in.SourceRef)})
		if errors.Is(err, pgx.ErrNoRows) {
			if active, aerr := q.HasActiveHide(ctx, in.MessageID); aerr != nil {
				return aerr
			} else if active {
				return apperr.FailedPrecondition("%s", text.HideSourceMismatch())
			}
			return apperr.FailedPrecondition("%s", text.HideNotHidden())
		}
		if err != nil {
			return err
		}
		out = rows.Hide(row, false)
		return q.Notify(ctx, live.Visible(msg.ConversationID, msg.BuyerID, msg.StoreID, msg.Seq).Encode())
	})
	return out, err
}

func checkReason(in model.HideInput) error {
	if in.AdminID <= 0 {
		return apperr.InvalidInputOn([]string{"AdminID"}, "%s", text.HideAdminRequired())
	}
	valid := false
	for _, c := range policy.CommunityAssessable() {
		valid = valid || string(c) == in.Category
	}
	if !valid {
		return apperr.InvalidInputOn([]string{"Category"}, "%s", text.HideCategoryInvalid())
	}
	if !utf8.ValidString(in.Text) || !utf8.ValidString(in.SourceRef) {
		return apperr.InvalidInputOn([]string{"Text"}, "%s", text.HideTextInvalid())
	}
	if utf8.RuneCountInString(in.Text) > model.MaxHideReasonRunes {
		return apperr.InvalidInputOn([]string{"Text"}, "%s", text.HideReasonTooLong(text.HideReasonTooLongArgs{Max: model.MaxHideReasonRunes}))
	}
	if utf8.RuneCountInString(in.SourceRef) > model.MaxSourceRefRunes {
		return apperr.InvalidInputOn([]string{"SourceRef"}, "%s", text.HideSourceRefTooLong(text.HideSourceRefTooLongArgs{Max: model.MaxSourceRefRunes}))
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
