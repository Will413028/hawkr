// Package sanction 是買家的社群處分與隱藏：隱藏與還原買家寫的留言與討論區留言（理由、來源與稽核在 hides，可多次隱藏與還原）、警告買家
// （sanctions）、作者讀自己的處分與被隱藏的內容、給 moderation 驗證內容歸屬。這些是根套件給 moderation 的方法，會在 moderation 的
// transaction 內被呼叫：只用 db.Conn(ctx)，不開讀取快照，撞唯一鍵用 ON CONFLICT、不在同一個 transaction 繼續下 SQL（單體規則 3）。
// 貼文與店家身分的內容不收：它們走 seller 的內容下架措施（措施是店家處分效果的唯一來源）。
package sanction

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Deps 是處分服務的依賴。
type Deps struct {
	DB    *db.DB
	Cache readcache.Cache
	NewID func() uuid.UUID
	Now   func() time.Time
}

// Service 是處分與隱藏的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// checkReason 檢查理由的欄位。reason 與 sourceRef 要先經 trimtext.Trim（單體規則 9：DB 的 CHECK 拒絕頭尾有空白的值）。
func checkReason(category, reason, sourceRef string, adminID int64) error {
	if !policy.Category(category).CommunityAssessable() {
		return apperr.InvalidInputOn([]string{"Category"}, "%s", text.HideReasonCategoryInvalid())
	}
	if utf8.RuneCountInString(reason) > model.MaxHideReasonRunes {
		return apperr.InvalidInputOn([]string{"Text"}, "%s", text.HideReasonTooLong(text.HideReasonTooLongArgs{Max: model.MaxHideReasonRunes}))
	}
	if n := utf8.RuneCountInString(sourceRef); n > model.MaxSourceRefRunes {
		return apperr.InvalidInputOn([]string{"SourceRef"}, "%s", text.HideSourceRefTooLong(text.HideSourceRefTooLongArgs{Max: model.MaxSourceRefRunes}))
	}
	if adminID <= 0 {
		return apperr.InvalidInputOn([]string{"AdminID"}, "%s", text.HideAdminRequired())
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Owner 回傳內容的歸屬（給 moderation 在建立處分前驗證）：內容不存在回 Exists 為假，不是錯誤。
func (s *Service) Owner(ctx context.Context, t model.Target) (model.ContentOwner, error) {
	now := s.d.Now().UTC()
	switch t.Kind {
	case model.TargetComment:
		r, err := reads.One[reads.OwnerRow](ctx, s.d.DB.Conn(ctx), reads.GetCommentOwner(t.ID, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ContentOwner{}, nil
		}
		if err != nil {
			return model.ContentOwner{}, fmt.Errorf("social：讀取留言的歸屬 %s：%w", t.ID, err)
		}
		return model.ContentOwner{Exists: r.ExistsNow, Author: model.Actor{Kind: model.ActorKind(r.ActorKind), ID: r.ActorID}, StoreID: r.StoreID, Readable: r.Readable}, nil
	case model.TargetForumEntry:
		r, err := reads.One[reads.OwnerRow](ctx, s.d.DB.Conn(ctx), reads.GetForumEntryOwner(t.ID, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ContentOwner{}, nil
		}
		if err != nil {
			return model.ContentOwner{}, fmt.Errorf("social：讀取討論區留言的歸屬 %s：%w", t.ID, err)
		}
		return model.ContentOwner{Exists: r.ExistsNow, Author: model.Actor{Kind: model.ActorKind(r.ActorKind), ID: r.ActorID}, StoreID: r.StoreID, Readable: r.Readable}, nil
	case model.TargetPost:
		r, err := reads.One[reads.PostOwnerRow](ctx, s.d.DB.Conn(ctx), reads.GetPostOwner(t.ID, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ContentOwner{}, nil
		}
		if err != nil {
			return model.ContentOwner{}, fmt.Errorf("social：讀取貼文的歸屬 %s：%w", t.ID, err)
		}
		return model.ContentOwner{Exists: r.ExistsNow, Author: model.Actor{Kind: model.ActorStore, ID: r.StoreID}, StoreID: r.StoreID, Readable: r.Readable}, nil
	}
	return model.ContentOwner{}, apperr.InvalidInputOn([]string{"Target"}, "%s", text.HideTargetKindInvalid())
}

// lockTarget 鎖住隱藏的目標列（與刪除、編輯同一把鎖），讓之後的 Owner 檢查和寫入看到同一個已提交的狀態：與並行的刪除序列化後，隱藏一則剛被刪除的
// 內容回 NotFound，而不是對一則已經不存在的內容寫隱藏。計數的正確性不靠它（trigger 的 reevaluate 自己先鎖內容列）。目標不存在時不鎖，由 Owner 回 NotFound。
func (s *Service) lockTarget(ctx context.Context, t model.Target) error {
	q := s.q(ctx)
	var err error
	if t.Kind == model.TargetComment {
		_, err = q.LockComment(ctx, t.ID)
	} else {
		_, err = q.LockForumEntry(ctx, t.ID)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("social：鎖定隱藏的目標 %s：%w", t.ID, err)
	}
	return nil
}

func (s *Service) requireBuyerContent(ctx context.Context, t model.Target) (model.ContentOwner, error) {
	if t.Kind != model.TargetComment && t.Kind != model.TargetForumEntry {
		return model.ContentOwner{}, apperr.InvalidInputOn([]string{"Target"}, "%s", text.HideTargetNotHideable())
	}
	if err := s.lockTarget(ctx, t); err != nil {
		return model.ContentOwner{}, err
	}
	o, err := s.Owner(ctx, t)
	if err != nil {
		return model.ContentOwner{}, err
	}
	if !o.Exists {
		return model.ContentOwner{}, apperr.NotFound("%s", text.HideContentNotFound())
	}
	if o.Author.Kind != model.ActorBuyer {
		return model.ContentOwner{}, apperr.FailedPrecondition("%s", text.HideStoreContent())
	}
	return o, nil
}

func toHide(r store.SocialHide, created bool) model.Hide {
	h := model.Hide{ID: r.ID, Category: r.ReasonCategory, Text: r.ReasonText, AdminID: r.AdminID, HiddenAt: r.HiddenAt, RestoredAt: r.RestoredAt, Created: created}
	if r.SourceRef != nil {
		h.SourceRef = *r.SourceRef
	}
	switch {
	case r.CommentID != nil:
		h.Target = model.Target{Kind: model.TargetComment, ID: *r.CommentID}
	case r.ForumEntryID != nil:
		h.Target = model.Target{Kind: model.TargetForumEntry, ID: *r.ForumEntryID}
	}
	return h
}

// bust 排入隱藏與還原的快取失效：留言的計數在貼文列表裡，討論區在店面頁。
func (s *Service) bust(ctx context.Context, t model.Target, o model.ContentOwner, postID func() (uuid.UUID, error)) error {
	if t.Kind == model.TargetForumEntry {
		return s.d.Cache.BustStore(ctx, o.StoreID)
	}
	id, err := postID()
	if err != nil {
		return err
	}
	return s.d.Cache.BustPost(ctx, id, o.StoreID)
}

func (s *Service) postOf(ctx context.Context, t model.Target) func() (uuid.UUID, error) {
	return func() (uuid.UUID, error) {
		c, err := s.q(ctx).GetComment(ctx, t.ID)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("social：讀取留言 %s：%w", t.ID, err)
		}
		return c.SocialComment.PostID, nil
	}
}

// Hide 隱藏一則買家寫的內容；同一則已有未還原的隱藏時回既有的那一筆（Created 為假，冪等，不排快取失效）。
func (s *Service) Hide(ctx context.Context, in model.HideInput) (model.Hide, error) {
	in.Text, in.SourceRef = trimtext.Trim(in.Text), trimtext.Trim(in.SourceRef)
	if err := checkReason(in.Category, in.Text, in.SourceRef, in.AdminID); err != nil {
		return model.Hide{}, err
	}
	var out model.Hide
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		o, err := s.requireBuyerContent(ctx, in.Target)
		if err != nil {
			return err
		}
		q := s.q(ctx)
		id := s.d.NewID()
		now := s.d.Now().UTC()
		var row store.SocialHide
		if in.Target.Kind == model.TargetComment {
			row, err = q.InsertCommentHide(ctx, store.InsertCommentHideParams{ID: id, CommentID: &in.Target.ID, ReasonCategory: in.Category, ReasonText: in.Text, SourceRef: optional(in.SourceRef), AdminID: in.AdminID, Now: now})
		} else {
			row, err = q.InsertForumEntryHide(ctx, store.InsertForumEntryHideParams{ID: id, ForumEntryID: &in.Target.ID, ReasonCategory: in.Category, ReasonText: in.Text, SourceRef: optional(in.SourceRef), AdminID: in.AdminID, Now: now})
		}
		if err != nil {
			return fmt.Errorf("social：隱藏 %s：%w", in.Target.ID, err)
		}
		out = toHide(row, row.ID == id)
		if !out.Created {
			return nil
		}
		return s.bust(ctx, in.Target, o, s.postOf(ctx, in.Target))
	})
	return out, err
}

// Restore 是管理員更正：還原目前未還原的隱藏（留列不刪，之後可以再隱藏），內容立即可見。沒有未還原的隱藏回 FailedPrecondition。
func (s *Service) Restore(ctx context.Context, in model.RestoreInput) (model.Hide, error) {
	in.Reason = trimtext.Trim(in.Reason)
	if in.AdminID <= 0 {
		return model.Hide{}, apperr.InvalidInputOn([]string{"AdminID"}, "%s", text.HideAdminRequired())
	}
	if n := utf8.RuneCountInString(in.Reason); n < 1 || n > model.MaxHideReasonRunes {
		return model.Hide{}, apperr.InvalidInputOn([]string{"Reason"}, "%s", text.HideRestoreReason(text.HideRestoreReasonArgs{Max: model.MaxHideReasonRunes}))
	}
	var out model.Hide
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		o, err := s.requireBuyerContent(ctx, in.Target)
		if err != nil {
			return err
		}
		q := s.q(ctx)
		now := s.d.Now().UTC()
		var row store.SocialHide
		if in.Target.Kind == model.TargetComment {
			row, err = q.RestoreCommentHide(ctx, store.RestoreCommentHideParams{CommentID: &in.Target.ID, AdminID: &in.AdminID, Reason: &in.Reason, Now: &now})
		} else {
			row, err = q.RestoreForumEntryHide(ctx, store.RestoreForumEntryHideParams{ForumEntryID: &in.Target.ID, AdminID: &in.AdminID, Reason: &in.Reason, Now: &now})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("%s", text.HideNotHidden())
		}
		if err != nil {
			return fmt.Errorf("social：還原 %s：%w", in.Target.ID, err)
		}
		out = toHide(row, false)
		return s.bust(ctx, in.Target, o, s.postOf(ctx, in.Target))
	})
	return out, err
}

// Warn 對買家寫一筆警告（WARNING，沒有期限）。
func (s *Service) Warn(ctx context.Context, in model.WarnInput) (model.Sanction, error) {
	in.Text, in.SourceRef = trimtext.Trim(in.Text), trimtext.Trim(in.SourceRef)
	if in.BuyerID <= 0 {
		return model.Sanction{}, apperr.InvalidInputOn([]string{"BuyerID"}, "%s", text.SanctionBuyerRequired())
	}
	if err := checkReason(in.Category, in.Text, in.SourceRef, in.AdminID); err != nil {
		return model.Sanction{}, err
	}
	row, err := s.q(ctx).InsertSanction(ctx, store.InsertSanctionParams{
		ID: s.d.NewID(), BuyerID: in.BuyerID, Kind: string(model.SanctionWarning), ReasonCategory: in.Category, ReasonText: in.Text,
		SourceRef: optional(in.SourceRef), AdminID: in.AdminID, Now: s.d.Now().UTC(),
	})
	if err != nil {
		return model.Sanction{}, fmt.Errorf("social：警告買家 %d：%w", in.BuyerID, err)
	}
	return toSanction(row), nil
}

func toSanction(r store.SocialSanction) model.Sanction {
	out := model.Sanction{ID: r.ID, BuyerID: r.BuyerID, Kind: model.SanctionKind(r.Kind), Category: r.ReasonCategory, Text: r.ReasonText, AdminID: r.AdminID, CreatedAt: r.CreatedAt, EndsAt: r.EndsAt}
	if r.SourceRef != nil {
		out.SourceRef = *r.SourceRef
	}
	return out
}

// MySanctions 是買家自己的處分紀錄，新的在前（keyset，不回總數）。
func (s *Service) MySanctions(ctx context.Context, buyerID int64, after *model.SanctionCursor, size int32) (model.SanctionPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.SanctionPage{}, err
	}
	at, id := after.Key()
	p := store.ListSanctionsParams{BuyerID: buyerID, AfterCreated: at, AfterID: id, MaxRows: size + 1}
	rows, err := s.q(ctx).ListSanctions(ctx, p)
	if err != nil {
		return model.SanctionPage{}, fmt.Errorf("social：列出處分：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.SanctionPage{Sanctions: make([]model.Sanction, len(rows))}
	for i, r := range rows {
		out.Sanctions[i] = toSanction(r)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.SanctionCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}

// MyHiddenContent 是買家自己被隱藏（未還原）的內容與理由，新的在前：作者讀得到理由與自己的內容，別人讀不到。
func (s *Service) MyHiddenContent(ctx context.Context, buyerID int64, after *model.HiddenCursor, size int32) (model.HiddenPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.HiddenPage{}, err
	}
	at, id := after.Key()
	p := store.ListHiddenContentOfParams{BuyerID: buyerID, AfterHidden: at, AfterID: id, MaxRows: size + 1}
	rows, err := s.q(ctx).ListHiddenContentOf(ctx, p)
	if err != nil {
		return model.HiddenPage{}, fmt.Errorf("social：列出被隱藏的內容：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.HiddenPage{Items: make([]model.HiddenContent, len(rows))}
	for i, r := range rows {
		kind := model.TargetComment
		if r.ContentKind == string(model.TargetForumEntry) {
			kind = model.TargetForumEntry
		}
		item := model.HiddenContent{
			HideID: r.HideID, Target: model.Target{Kind: kind, ID: r.ContentID},
			Body: r.Body, CreatedAt: r.CreatedAt, Category: r.ReasonCategory, Text: r.ReasonText, HiddenAt: r.HiddenAt,
		}
		if kind == model.TargetComment {
			item.PostID = &r.PostID
		} else {
			item.StoreID = &r.StoreID
		}
		out.Items[i] = item
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.HiddenCursor{HiddenAt: last.HiddenAt, HideID: last.HideID}
	}
	return out, nil
}
