// Package write 是 im 的寫入路徑：送出、已讀、隱藏與還原。
package write

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Buyers 是送出用到的 buyer 公開 API；正式組裝傳 *buyer.Module。
type Buyers interface {
	AccountIDByBuyerID(ctx context.Context, buyerID int64) (int64, error)
	ProfileByBuyerID(ctx context.Context, buyerID int64) (buyer.Profile, error)
	IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error)
}

// Stores 是送出用到的 seller 公開 API；正式組裝傳 *seller.Module。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
}

// Deps 是寫入路徑的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Media  media.Media
	Jobs   jobs.Enqueuer
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
	// inactive 回報一方是否在 inactive_parties；預設讀資料庫，守門的單元測試以替身取代。
	inactive func(ctx context.Context, kind model.PartyKind, id int64) (bool, error)
}

// New 建立寫入路徑。
func New(d Deps) *Service {
	s := &Service{d: d}
	s.inactive = func(ctx context.Context, kind model.PartyKind, id int64) (bool, error) {
		return s.q(ctx).IsInactiveParty(ctx, store.IsInactivePartyParams{Kind: string(kind), PartyID: id})
	}
	return s
}

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// MarkRead 把呼叫者這一方在對話的已讀游標推到 uptoSeq：只前進不後退，也不超過 last_seq。對話不屬於呼叫者回 NotFound。
func (s *Service) MarkRead(ctx context.Context, conversationID uuid.UUID, p model.Party, uptoSeq int64) error {
	l := uitext.LocaleFrom(ctx)
	if err := p.Authorize(); err != nil {
		return err
	}
	var err error
	if p.Kind == model.PartyBuyer {
		_, err = s.q(ctx).MarkReadBuyer(ctx, store.MarkReadBuyerParams{UptoSeq: uptoSeq, ID: conversationID, BuyerID: p.BuyerID})
	} else {
		_, err = s.q(ctx).MarkReadStore(ctx, store.MarkReadStoreParams{UptoSeq: uptoSeq, ID: conversationID, StoreID: p.Store.StoreID})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("%s", text.ImConversationNotFound(l))
	}
	return err
}

// MarkAllRead 把呼叫者這一方的全部對話標成已讀。
func (s *Service) MarkAllRead(ctx context.Context, p model.Party) error {
	if err := p.Authorize(); err != nil {
		return err
	}
	var err error
	if p.Kind == model.PartyBuyer {
		_, err = s.q(ctx).MarkAllReadBuyer(ctx, p.BuyerID)
	} else {
		_, err = s.q(ctx).MarkAllReadStore(ctx, p.Store.StoreID)
	}
	return err
}

// Hide 隱藏一則訊息（moderation 的 transaction 內呼叫）；已有未還原的隱藏時回既有的那一筆（Created 為假，冪等）。
// 預覽在讀取時推導，所以隱藏不必更新對話列。
func (s *Service) Hide(ctx context.Context, in model.HideInput) (model.Hide, error) {
	in.Text, in.SourceRef = trimtext.Trim(in.Text), trimtext.Trim(in.SourceRef)
	if err := checkReason(in); err != nil {
		return model.Hide{}, err
	}
	var out model.Hide
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.LockMessage(ctx, in.MessageID); err != nil {
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
		return nil
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
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.LockMessage(ctx, in.MessageID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("%s", text.HideMessageNotFound())
			}
			return err
		}
		row, err := q.RestoreHide(ctx, store.RestoreHideParams{MessageID: in.MessageID, AdminID: &in.AdminID, Reason: &in.Reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("%s", text.HideNotHidden())
		}
		if err != nil {
			return err
		}
		out = rows.Hide(row, false)
		return nil
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
