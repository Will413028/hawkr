// Package write 是評價的寫入路徑：提交、店家回覆、隱藏與還原。
package write

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/textfilter"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Buyers 是提交時查上傳者帳號的 buyer 公開 API（media 轉正要確認 staged token 屬於上傳的帳號）。
type Buyers interface {
	AccountIDByBuyerID(ctx context.Context, buyerID int64) (int64, error)
}

// Stores 是讀店家（取 store_code 組快取失效的 tag）的 seller 公開 API。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
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
type Service struct{ d Deps }

// New 建立寫入路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// Submit 提交一則評價。順序：輸入驗證 → 詞典 → 訂單資格（唯一的快照來源）→ 買家是否仍營運 → 期限 → 項目 → transaction 外轉正圖片 →
// transaction 內寫列、排媒體同步與快取失效、發事件。
func (s *Service) Submit(ctx context.Context, in model.SubmitInput) (model.Review, error) {
	l := uitext.LocaleFrom(ctx)
	content, rating, err := s.checkSubmit(l, &in)
	if err != nil {
		return model.Review{}, err
	}

	order, err := s.d.Orders.OrderForReview(ctx, in.OrderNo)
	switch {
	case errors.Is(err, model.ErrOrderNotFound):
		return model.Review{}, model.WithReason(apperr.NotFound("%s", text.ReviewOrderNotFound(l)), model.ReasonOrderNotFound)
	case err != nil:
		return model.Review{}, apperr.WithCause(apperr.Unavailable("%s", text.ReviewOrderUnavailable(l)), err)
	}
	if order.Status != model.OrderStatusCompleted {
		return model.Review{}, model.WithReason(apperr.FailedPrecondition("%s", text.ReviewOrderNotReviewable(l)), model.ReasonOrderNotReviewable)
	}
	if order.BuyerID != in.BuyerID {
		return model.Review{}, apperr.PermissionDenied("%s", text.ReviewOrderNotYours(l))
	}
	inactive, err := s.q(ctx).IsInactiveAuthor(ctx, in.BuyerID)
	if err != nil {
		return model.Review{}, err
	}
	if inactive {
		return model.Review{}, apperr.FailedPrecondition("%s", text.ReviewAuthorInactive(l))
	}
	if s.d.Now().After(order.CompletedAt.Add(model.ReviewWindow)) {
		return model.Review{}, model.WithReason(apperr.FailedPrecondition("%s", text.ReviewWindowExpired(l)), model.ReasonWindowExpired)
	}
	item, ok := findItem(order.Items, in.OrderItemID)
	if !ok {
		return model.Review{}, apperr.InvalidInputOn([]string{"OrderItemID"}, "%s", text.ReviewItemNotInOrder(l))
	}

	var assets []media.Asset
	if len(in.StagedTokens) > 0 {
		accountID, err := s.d.Buyers.AccountIDByBuyerID(ctx, in.BuyerID)
		if err != nil {
			return model.Review{}, err
		}
		assets, err = media.FinalizeList(ctx, s.d.Media, media.Uploader{ID: accountID}, "StagedTokens", in.StagedTokens, media.RefTypeOrderReview, nil)
		if err != nil {
			return model.Review{}, err
		}
	}

	var out model.Review
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now().UTC()
		id := s.d.NewID()
		row, err := q.InsertReview(ctx, store.InsertReviewParams{
			ID: id, OrderNo: order.OrderNo, OrderItemID: item.OrderItemID, BuyerID: in.BuyerID, StoreID: order.StoreID, ProductID: item.ProductID,
			Rating: rating, Content: optional(content), SkuName: item.SkuName, ItemImageKey: optional(item.ImageKey), Price: item.Price, CreatedAt: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return model.WithReason(apperr.AlreadyExists("%s", text.ReviewAlreadyExists(l)), model.ReasonAlreadyExists)
		}
		if err != nil {
			return err
		}
		images := make([]model.Image, len(assets))
		for i, a := range assets {
			if err := q.InsertReviewImage(ctx, store.InsertReviewImageParams{ReviewID: id, Position: int16(i), AssetID: a.ID, ObjectKey: a.ObjectKey}); err != nil {
				return err
			}
			images[i] = model.Image{AssetID: a.ID, ObjectKey: a.ObjectKey}
		}
		if len(assets) > 0 {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.ReviewRef(id)); err != nil {
				return err
			}
		}
		if err := s.bust(ctx, order.StoreID); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.ReviewCreated{EventID: s.d.NewID(), OccurredAt: now, ReviewID: id, StoreID: order.StoreID, ProductID: item.ProductID}); err != nil {
			return err
		}
		out = rows.Review(row, images)
		return nil
	})
	return out, err
}

func (s *Service) checkSubmit(l uitext.Locale, in *model.SubmitInput) (string, int16, error) {
	if in.OrderNo == "" {
		return "", 0, apperr.InvalidInputOn([]string{"OrderNo"}, "%s", text.ReviewOrderRequired(l))
	}
	if in.OrderItemID <= 0 {
		return "", 0, apperr.InvalidInputOn([]string{"OrderItemID"}, "%s", text.ReviewOrderItemInvalid(l))
	}
	if in.Rating < model.MinRating || in.Rating > model.MaxRating {
		return "", 0, apperr.InvalidInputOn([]string{"Rating"}, "%s", text.ReviewRatingInvalid(l))
	}
	if len(in.StagedTokens) > model.MaxImages {
		return "", 0, apperr.InvalidInputOn([]string{"StagedTokens"}, "%s", text.ReviewTooManyImages(l, text.ReviewTooManyImagesArgs{Max: model.MaxImages}))
	}
	if !utf8.ValidString(in.Content) {
		return "", 0, apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewContentInvalidText(l))
	}
	content := trimtext.Trim(in.Content)
	if utf8.RuneCountInString(content) > model.MaxContentRunes {
		return "", 0, apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewContentTooLong(l, text.ReviewContentTooLongArgs{Max: model.MaxContentRunes}))
	}
	if hit, _ := textfilter.Check(content); hit {
		return "", 0, model.WithReason(apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewModerationRejected(l)), model.ReasonModerationRejected)
	}
	return content, int16(in.Rating), nil
}

func findItem(items []model.OrderItem, id int64) (model.OrderItem, bool) {
	for _, it := range items {
		if it.OrderItemID == id {
			return it, true
		}
	}
	return model.OrderItem{}, false
}

// bust 排入店面頁的快取失效（評價數與評分在店面頁）；必須在 transaction 內呼叫。商品頁的 tag 在 A6-1c 隨 revalidate.ProductTag 加入。
func (s *Service) bust(ctx context.Context, storeID int64) error {
	st, err := s.d.Stores.Store(ctx, storeID)
	switch {
	case err == nil:
		return s.d.Jobs.Enqueue(ctx, revalidate.Args{Tags: revalidate.StoreTags(st.StoreID, st.StoreCode)}, nil)
	case apperr.KindOf(err) == apperr.KindNotFound:
		return nil
	default:
		return err
	}
}

// Reply 是店家回覆（可覆蓋，每次都重跑詞典）：店家要有 CapMarketing（停權 fail-closed）；擁有權與公開可讀都在 UPDATE 的 WHERE，
// 0 列時補查分流。不發事件、不排失效（舊行為：回覆不改任何聚合）。
func (s *Service) Reply(ctx context.Context, in model.ReplyInput) (model.Review, error) {
	l := uitext.LocaleFrom(ctx)
	if err := seller.Authorize(in.Store, seller.CapMarketing); err != nil {
		return model.Review{}, err
	}
	if !utf8.ValidString(in.Content) {
		return model.Review{}, apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewReplyInvalidText(l))
	}
	content := trimtext.Trim(in.Content)
	if content == "" {
		return model.Review{}, apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewReplyRequired(l))
	}
	if utf8.RuneCountInString(content) > model.MaxContentRunes {
		return model.Review{}, apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewReplyTooLong(l, text.ReviewReplyTooLongArgs{Max: model.MaxContentRunes}))
	}
	if hit, _ := textfilter.Check(content); hit {
		return model.Review{}, model.WithReason(apperr.InvalidInputOn([]string{"Content"}, "%s", text.ReviewModerationRejected(l)), model.ReasonModerationRejected)
	}

	q := s.q(ctx)
	now := s.d.Now().UTC()
	row, err := q.ReplyToReview(ctx, store.ReplyToReviewParams{ID: in.ReviewID, StoreID: in.Store.StoreID, Content: &content, Now: &now})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, derr := q.GetReviewForDiagnosis(ctx, in.ReviewID)
		switch {
		case errors.Is(derr, pgx.ErrNoRows):
			return model.Review{}, apperr.NotFound("%s", text.ReviewNotFound(l))
		case derr != nil:
			return model.Review{}, derr
		case existing.StoreID != in.Store.StoreID:
			return model.Review{}, apperr.PermissionDenied("%s", text.ReviewNotYourReview(l))
		}
		return model.Review{}, model.WithReason(apperr.FailedPrecondition("%s", text.ReviewNotReplyable(l)), model.ReasonNotReplyable)
	}
	if err != nil {
		return model.Review{}, err
	}
	images, err := q.ListReviewImages(ctx, []uuid.UUID{row.ID})
	if err != nil {
		return model.Review{}, err
	}
	return rows.Review(row, rows.Images(images)[row.ID]), nil
}

// Hide 隱藏一則評價（moderation 的 transaction 內呼叫）；已有未還原的隱藏時回既有的那一筆（Created 為假，冪等，不發事件）。
// 新隱藏時同 transaction 發 ReviewAggregateChanged 並排快取失效。
func (s *Service) Hide(ctx context.Context, in model.HideInput) (model.Hide, error) {
	in.Text, in.SourceRef = trimtext.Trim(in.Text), trimtext.Trim(in.SourceRef)
	if err := checkReason(in); err != nil {
		return model.Hide{}, err
	}
	var out model.Hide
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		r, err := q.LockReview(ctx, in.ReviewID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.HideReviewNotFound())
		}
		if err != nil {
			return err
		}
		id, now := s.d.NewID(), s.d.Now().UTC()
		row, err := q.InsertHide(ctx, store.InsertHideParams{
			ID: id, ReviewID: in.ReviewID, ReasonCategory: in.Category, ReasonText: in.Text, SourceRef: optional(in.SourceRef), AdminID: in.AdminID, Now: now,
		})
		if err != nil {
			return err
		}
		out = toHide(row, row.ID == id)
		if !out.Created {
			return nil
		}
		return s.aggregateChanged(ctx, r.StoreID, r.ProductID, now)
	})
	return out, err
}

// Restore 是管理員更正：還原目前未還原的隱藏（留列不刪，之後可以再隱藏），評價立即可見。沒有未還原的隱藏回 FailedPrecondition。
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
		q := s.q(ctx)
		r, err := q.LockReview(ctx, in.ReviewID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.HideReviewNotFound())
		}
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		row, err := q.RestoreHide(ctx, store.RestoreHideParams{ReviewID: in.ReviewID, AdminID: &in.AdminID, Reason: &in.Reason, Now: &now})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("%s", text.HideNotHidden())
		}
		if err != nil {
			return err
		}
		out = toHide(row, false)
		return s.aggregateChanged(ctx, r.StoreID, r.ProductID, now)
	})
	return out, err
}

func (s *Service) aggregateChanged(ctx context.Context, storeID, productID int64, now time.Time) error {
	if err := s.bust(ctx, storeID); err != nil {
		return err
	}
	return events.Publish(ctx, s.d.Events, model.ReviewAggregateChanged{EventID: s.d.NewID(), OccurredAt: now, StoreID: storeID, ProductID: productID})
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
	if utf8.RuneCountInString(in.Text) > model.MaxHideReasonRunes {
		return apperr.InvalidInputOn([]string{"Text"}, "%s", text.HideReasonTooLong(text.HideReasonTooLongArgs{Max: model.MaxHideReasonRunes}))
	}
	if utf8.RuneCountInString(in.SourceRef) > model.MaxSourceRefRunes {
		return apperr.InvalidInputOn([]string{"SourceRef"}, "%s", text.HideSourceRefTooLong(text.HideSourceRefTooLongArgs{Max: model.MaxSourceRefRunes}))
	}
	return nil
}

func toHide(r store.ReviewHide, created bool) model.Hide {
	h := model.Hide{ID: r.ID, ReviewID: r.ReviewID, Category: r.ReasonCategory, Text: r.ReasonText, AdminID: r.AdminID, HiddenAt: r.HiddenAt, RestoredAt: r.RestoredAt, Created: created}
	if r.SourceRef != nil {
		h.SourceRef = *r.SourceRef
	}
	return h
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
