// Package post 是貼文的生命週期（DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED，刪除是軟刪）、hashtag 與公開讀取。店家身分的寫入都收
// 已經解析好的 seller.Store，在這裡以 seller.Authorize 檢查能力（帳號路徑與代行路徑走同一個入口）：建立、編輯、發布要 CapMarketing，
// 下架與刪除要 CapContentContract。
package post

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/hashtag"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/textcheck"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// ProductVerifier 檢查貼文帶的商品：存在、上架中而且屬於這家店。它是跨行程的呼叫（B4 的 catalog 搬進單體前是舊的 product-service），
// 所以在開 transaction 之前呼叫（單體規則 5 的精神）。
//
// 回傳 nil（合格）、ErrProductNotAvailable、ErrProductNotOwned，或其他錯誤（查詢失敗；不能當成商品不存在）。使用者面的句子由 social 的目錄給，
// port 的實作不產生句子。
type ProductVerifier interface {
	VerifyProduct(ctx context.Context, productID, storeID int64) error
}

// 商品驗證的結果（port 回傳，verifyProduct 轉成使用者面的錯誤）。
var (
	ErrProductNotAvailable = errors.New("social：商品不存在或沒有上架")
	ErrProductNotOwned     = errors.New("social：商品不屬於這家店")
)

// Deps 是貼文服務的依賴。
type Deps struct {
	DB       *db.DB
	Media    media.Media
	Jobs     jobs.Enqueuer
	Events   *events.Bus
	Products ProductVerifier
	Cache    readcache.Cache
	NewID    func() uuid.UUID
	Now      func() time.Time
}

// Service 是貼文的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func versionConflict(ctx context.Context) error {
	return apperr.VersionConflict("%s", text.PostVersionConflict(uitext.LocaleFrom(ctx)))
}

func notFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.PostNotFound(uitext.LocaleFrom(ctx)))
}

// checkPublishable 是「公開中的貼文恰好一張圖、內文必填」唯一的檢查（舊 social 的 checkPublishable）：轉為 PUBLISHED 時與已發布的
// 貼文被編輯時都要滿足；草稿可以不完整。
func checkPublishable(ctx context.Context, body string, hasImage bool) error {
	if !hasImage {
		return apperr.InvalidInputOn([]string{"ImageToken"}, "%s", text.PostPublishImageRequired(uitext.LocaleFrom(ctx)))
	}
	if body == "" {
		return apperr.InvalidInputOn([]string{"Body"}, "%s", text.PostPublishBodyRequired(uitext.LocaleFrom(ctx)))
	}
	return nil
}

func (s *Service) verifyProduct(ctx context.Context, productID *int64, storeID int64) error {
	if productID == nil {
		return nil
	}
	if *productID <= 0 {
		return apperr.InvalidInputOn([]string{"ProductID"}, "%s", text.PostProductInvalid(uitext.LocaleFrom(ctx)))
	}
	err := s.d.Products.VerifyProduct(ctx, *productID, storeID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrProductNotAvailable):
		return apperr.InvalidInputOn([]string{"ProductID"}, "%s", text.PostProductNotAvailable(uitext.LocaleFrom(ctx)))
	case errors.Is(err, ErrProductNotOwned):
		return apperr.InvalidInputOn([]string{"ProductID"}, "%s", text.PostProductNotOwned(uitext.LocaleFrom(ctx)))
	default:
		return apperr.WithCause(apperr.Unavailable("%s", text.PostProductUnverifiable(uitext.LocaleFrom(ctx))), err)
	}
}

// finalizeImage 把新上傳的圖轉正（不建立引用；上傳者是店家所屬的帳號）。沒有 token 回 nil。
func (s *Service) finalizeImage(ctx context.Context, st seller.Store, token string) (*media.Asset, error) {
	if token == "" {
		return nil, nil
	}
	a, err := media.FinalizeField(ctx, s.d.Media, media.Uploader{ID: st.AccountID},
		media.Field{Name: "ImageToken", Message: text.PostImageUnusable(uitext.LocaleFrom(ctx)).Text}, token, media.RefTypePost)
	if err != nil {
		return nil, fmt.Errorf("social：轉正貼文圖片：%w", err)
	}
	return &a, nil
}

func candidates(a *media.Asset) []media.SyncCandidate {
	if a == nil {
		return nil
	}
	return []media.SyncCandidate{{Field: "ImageToken", AssetID: a.ID}}
}

func (s *Service) writeTags(ctx context.Context, q *store.Queries, postID uuid.UUID, tags []hashtag.Tag) error {
	if err := q.DeletePostHashtags(ctx, postID); err != nil {
		return fmt.Errorf("social：清除貼文標籤：%w", err)
	}
	for i, t := range tags {
		if err := q.InsertPostHashtag(ctx, store.InsertPostHashtagParams{PostID: postID, Position: int16(i), Name: t.Name, DisplayName: t.Display}); err != nil {
			return fmt.Errorf("social：寫入貼文標籤：%w", err)
		}
	}
	return nil
}

func (s *Service) publishedEvent(ctx context.Context, p store.SocialPost, now time.Time) error {
	return events.Publish(ctx, s.d.Events, model.PostPublished{
		EventID: uuid.NewV7(), OccurredAt: now, PostID: p.ID, StoreID: p.StoreID,
		Preview: model.TruncateRunes(p.Body, model.PreviewMaxRunes),
	})
}

// Create 新增貼文。Publish 為真時同一個動作發布（公開要件在這裡驗證），草稿不發 PostPublished（SD8）。
func (s *Service) Create(ctx context.Context, st seller.Store, in model.CreatePostInput) (model.Post, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.Post{}, err
	}
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.PostBody, in.Body, model.MaxPostBodyRunes, false)
	if err != nil {
		return model.Post{}, err
	}
	tags, err := hashtag.RequireSet(uitext.LocaleFrom(ctx), in.Hashtags)
	if err != nil {
		return model.Post{}, err
	}
	if in.Publish {
		if err := checkPublishable(ctx, body, in.ImageToken != ""); err != nil {
			return model.Post{}, err
		}
	}
	if err := s.verifyProduct(ctx, in.ProductID, st.StoreID); err != nil {
		return model.Post{}, err
	}
	asset, err := s.finalizeImage(ctx, st, in.ImageToken)
	if err != nil {
		return model.Post{}, err
	}
	id := s.d.NewID()
	ref := mediaref.PostRef(id)
	now := s.d.Now().UTC()
	params := store.InsertPostParams{ID: id, StoreID: st.StoreID, Body: body, ProductID: in.ProductID, Status: string(model.StatusDraft), Now: now}
	if in.Publish {
		params.Status, params.PublishedAt = string(model.StatusPublished), &now
	}
	if asset != nil {
		params.ImageKey, params.ImageAssetID = &asset.ObjectKey, &asset.ID
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := mediaref.Reject(ctx, q, ref, candidates(asset)...); err != nil {
			return err
		}
		row, err := q.InsertPost(ctx, params)
		if err != nil {
			return fmt.Errorf("social：新增貼文：%w", err)
		}
		if err := s.writeTags(ctx, q, id, tags); err != nil {
			return err
		}
		if asset != nil {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, ref); err != nil {
				return err
			}
		}
		if in.Publish {
			if err := s.publishedEvent(ctx, row, now); err != nil {
				return err
			}
			return s.d.Cache.BustPost(ctx, id, st.StoreID)
		}
		return nil
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, id)
}

// lockOwned 鎖住貼文並確認屬於這家店；別家的貼文與不存在一樣回 NotFound。
func (s *Service) lockOwned(ctx context.Context, q *store.Queries, storeID int64, id uuid.UUID) (store.SocialPost, error) {
	row, err := q.LockPost(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.StoreID != storeID {
		return store.SocialPost{}, notFound(ctx)
	}
	if err != nil {
		return store.SocialPost{}, fmt.Errorf("social：鎖定貼文 %s：%w", id, err)
	}
	return row, nil
}

// Update 修改貼文：Version 與目前的不符回 VersionConflict、不寫入。已發布的貼文被編輯後仍要滿足公開要件。
func (s *Service) Update(ctx context.Context, st seller.Store, in model.UpdatePostInput) (model.Post, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.Post{}, err
	}
	if in.Version <= 0 {
		return model.Post{}, apperr.InvalidInput("%s", text.PostUpdateEtagRequired(uitext.LocaleFrom(ctx)))
	}
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.PostBody, in.Body, model.MaxPostBodyRunes, false)
	if err != nil {
		return model.Post{}, err
	}
	var tags []hashtag.Tag
	if in.Hashtags != nil {
		if tags, err = hashtag.RequireSet(uitext.LocaleFrom(ctx), *in.Hashtags); err != nil {
			return model.Post{}, err
		}
	}
	if in.ClearImage && in.ImageToken != "" {
		return model.Post{}, apperr.InvalidInputOn([]string{"ClearImage"}, "%s", text.PostImageReplaceAndClear(uitext.LocaleFrom(ctx)))
	}
	cur, err := s.Own(ctx, st.StoreID, in.PostID)
	if err != nil {
		return model.Post{}, err
	}
	if cur.Version != in.Version {
		return model.Post{}, versionConflict(ctx)
	}
	if err := s.verifyProduct(ctx, in.ProductID, st.StoreID); err != nil {
		return model.Post{}, err
	}
	asset, err := s.finalizeImage(ctx, st, in.ImageToken)
	if err != nil {
		return model.Post{}, err
	}
	ref := mediaref.PostRef(in.PostID)
	now := s.d.Now().UTC()
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, in.PostID)
		if err != nil {
			return err
		}
		if err := mediaref.Reject(ctx, q, ref, candidates(asset)...); err != nil {
			return err
		}
		hasImage := row.ImageKey != nil && !in.ClearImage || asset != nil
		if row.Status == string(model.StatusPublished) {
			if err := checkPublishable(ctx, body, hasImage); err != nil {
				return err
			}
		}
		params := store.UpdatePostParams{ID: in.PostID, Body: body, ProductID: in.ProductID, ClearImage: in.ClearImage, Now: now, ExpectedVersion: in.Version}
		if asset != nil {
			params.NewImageKey, params.NewImageAssetID = &asset.ObjectKey, &asset.ID
		}
		upd, err := q.UpdatePost(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return versionConflict(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：修改貼文 %s：%w", in.PostID, err)
		}
		if in.Hashtags != nil {
			if err := s.writeTags(ctx, q, in.PostID, tags); err != nil {
				return err
			}
		}
		if (asset != nil || in.ClearImage) && row.ImageAssetID != nil {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, ref, *row.ImageAssetID); err != nil {
				return err
			}
		} else if asset != nil {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, ref); err != nil {
				return err
			}
		}
		if upd.Status == string(model.StatusPublished) {
			return s.d.Cache.BustPost(ctx, in.PostID, st.StoreID)
		}
		return nil
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, in.PostID)
}

// Publish 發布貼文（草稿或下架的貼文轉為 PUBLISHED）。published_at 固定在第一次發布（動態牆的順序不能靠下架再發布來洗：
// 下架後再發布沿用原值）；已經是 PUBLISHED 時是冪等的 no-op：不寫入、不發 PostPublished、不遞增 version、不清快取
// （與 Unpublish 對已下架的冪等一致，所以也不檢查 version）。
func (s *Service) Publish(ctx context.Context, st seller.Store, postID uuid.UUID, version int64) (model.Post, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.Post{}, err
	}
	if version <= 0 {
		return model.Post{}, apperr.InvalidInput("%s", text.PostPublishEtagRequired(uitext.LocaleFrom(ctx)))
	}
	now := s.d.Now().UTC()
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, postID)
		if err != nil {
			return err
		}
		if row.Status == string(model.StatusPublished) {
			return nil
		}
		if row.Version != version {
			return versionConflict(ctx)
		}
		if err := checkPublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
			return err
		}
		upd, err := q.SetPostStatus(ctx, store.SetPostStatusParams{ID: postID, Status: string(model.StatusPublished), Now: now, ExpectedVersion: version})
		if errors.Is(err, pgx.ErrNoRows) {
			return versionConflict(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：發布貼文 %s：%w", postID, err)
		}
		if err := s.publishedEvent(ctx, upd, now); err != nil {
			return err
		}
		return s.d.Cache.BustPost(ctx, postID, st.StoreID)
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, postID)
}

// Unpublish 下架貼文（PUBLISHED 轉為 UNPUBLISHED；已經下架的冪等）。草稿沒有發布過，不能下架。
func (s *Service) Unpublish(ctx context.Context, st seller.Store, postID uuid.UUID, version int64) (model.Post, error) {
	if err := seller.Authorize(st, seller.CapContentContract); err != nil {
		return model.Post{}, err
	}
	if version <= 0 {
		return model.Post{}, apperr.InvalidInput("%s", text.PostUnpublishEtagRequired(uitext.LocaleFrom(ctx)))
	}
	now := s.d.Now().UTC()
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, postID)
		if err != nil {
			return err
		}
		switch model.PostStatus(row.Status) {
		case model.StatusUnpublished:
			return nil
		case model.StatusDraft:
			return apperr.FailedPrecondition("%s", text.PostUnpublishDraft(uitext.LocaleFrom(ctx)))
		case model.StatusPublished:
		}
		if row.Version != version {
			return versionConflict(ctx)
		}
		if _, err := q.SetPostStatus(ctx, store.SetPostStatusParams{ID: postID, Status: string(model.StatusUnpublished), Now: now, ExpectedVersion: version}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return versionConflict(ctx)
			}
			return fmt.Errorf("social：下架貼文 %s：%w", postID, err)
		}
		return s.d.Cache.BustPost(ctx, postID, st.StoreID)
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, postID)
}

// Delete 軟刪貼文（已經刪除的冪等：不做副作用）。version 為 0 表示不檢查版本。軟刪保留內容與圖片引用（保留期還沒訂，見 CARRYOVER）。
func (s *Service) Delete(ctx context.Context, st seller.Store, postID uuid.UUID, version int64) error {
	if err := seller.Authorize(st, seller.CapContentContract); err != nil {
		return err
	}
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.LockPost(ctx, postID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("social：鎖定貼文 %s：%w", postID, err)
		}
		if row.StoreID != st.StoreID {
			return notFound(ctx)
		}
		n, err := q.SoftDeletePost(ctx, store.SoftDeletePostParams{ID: postID, Now: &now, ExpectedVersion: version})
		if err != nil {
			return fmt.Errorf("social：刪除貼文 %s：%w", postID, err)
		}
		if n == 0 {
			return versionConflict(ctx)
		}
		return s.d.Cache.BustPost(ctx, postID, st.StoreID)
	})
}

func validStatus(s model.PostStatus) bool {
	for _, v := range model.PostStatusValues() {
		if v == s {
			return true
		}
	}
	return false
}

func errInvalidStatus(ctx context.Context) error {
	return apperr.InvalidInputOn([]string{"Status"}, "%s", text.PostStatusInvalid(uitext.LocaleFrom(ctx)))
}
func errNegative(ctx context.Context) error {
	return apperr.InvalidInput("%s", text.QueryCountNegative(uitext.LocaleFrom(ctx)))
}
func errBadWindow(ctx context.Context) error {
	return apperr.InvalidInput("%s", text.QueryWindowOrder(uitext.LocaleFrom(ctx)))
}
func errWindowTooLong(ctx context.Context) error {
	return apperr.InvalidInput("%s", text.QueryWindowTooLong(uitext.LocaleFrom(ctx), text.QueryWindowTooLongArgs{Days: int(model.MaxTrendingWindow / (24 * time.Hour))}))
}
