// Package post 是貼文的生命週期（DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED，刪除是軟刪）、hashtag 與公開讀取。店家身分的寫入都收
// 已經解析好的 seller.Store，在這裡以 seller.Authorize 檢查能力（帳號路徑與代行路徑走同一個入口）：建立、編輯、發布要 CapMarketing，
// 下架與刪除要 CapContentContract。
package post

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/hashtag"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/textcheck"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// ProductVerifier 檢查貼文帶的商品：存在、上架中而且屬於這家店。由 cmd/hawkr 以 catalog 實作，在 transaction 之前呼叫。
//
// 回傳 nil（合格）、ErrProductNotAvailable、ErrProductNotOwned，或其他錯誤（查詢失敗；不能當成商品不存在）。使用者面的句子由 social 的目錄給，
// port 的實作不產生句子。
type ProductVerifier interface {
	VerifyProduct(ctx context.Context, productID uuid.UUID, storeID int64) error
}

// 商品驗證的結果（port 回傳，verifyProduct 轉成使用者面的錯誤）。
var (
	ErrProductNotAvailable = errors.New("social：商品不存在或沒有上架")
	ErrProductNotOwned     = errors.New("social：商品不屬於這家店")
)

// Deps 是貼文服務的依賴。
type Deps struct {
	DB       *db.DB
	Media    model.Media
	Jobs     jobs.Enqueuer
	Events   *events.Bus
	Products ProductVerifier
	Cache    readcache.Cache
	// Cancel 取消貼文的待發布排程（發布、刪除時；同一個 transaction，貼文列已鎖）。
	Cancel schedule.Canceler
	// Authorizers 是排程到點時依發起者種類重驗授權的實作；每一種都要有（social.New 檢查）。
	Authorizers map[model.ScheduleKind]model.ScheduleAuthorizer
	NewID       func() uuid.UUID
	Now         func() time.Time
	Logger      *slog.Logger
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

func checkPublishable(ctx context.Context, body string, hasImage bool) error {
	if !hasImage {
		return apperr.InvalidInputOn([]string{"ImageToken"}, "%s", text.PostPublishImageRequired(uitext.LocaleFrom(ctx)))
	}
	if body == "" {
		return apperr.InvalidInputOn([]string{"Body"}, "%s", text.PostPublishBodyRequired(uitext.LocaleFrom(ctx)))
	}
	return nil
}

func requirePublishable(ctx context.Context, body string, hasImage bool) error {
	err := checkPublishable(ctx, body, hasImage)
	if err == nil {
		return nil
	}
	return apperr.FailedPrecondition("%s", err.Error())
}

func (s *Service) verifyProduct(ctx context.Context, productID *uuid.UUID, storeID int64) error {
	if productID == nil {
		return nil
	}
	if *productID == uuid.Nil() {
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

func (s *Service) confirmImage(ctx context.Context, st seller.Store, token string) (*media.ConfirmedAsset, error) {
	if token == "" {
		return nil, nil
	}
	a, err := media.ConfirmField(ctx, s.d.Media, media.Uploader{ID: st.AccountID}, "ImageToken", token, media.VisibilityPublic)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

var errReplayed = errors.New("social：貼文的重送沿用原本那一篇")

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

func (s *Service) publishedEvent(ctx context.Context, p store.SocialPost, now time.Time, origin model.PostOrigin, sc store.SocialPostSchedule, actor *delegatedactor.Actor) error {
	e := model.PostPublished{
		EventID: uuid.NewV7(), OccurredAt: now, PostID: p.ID, StoreID: p.StoreID,
		Preview: model.TruncateRunes(p.Body, model.PreviewMaxRunes), Version: p.Version, Origin: origin, Actor: actor,
	}
	if origin == model.OriginScheduled {
		e.ScheduleID, e.ScheduledBy = sc.ID, model.ScheduledBy{Kind: model.ScheduleKind(sc.ScheduledByKind), Ref: sc.ScheduledByRef}
	}
	return emit(ctx, s.d.Events, e, actor)
}

func emit[E any](ctx context.Context, b *events.Bus, e E, actor *delegatedactor.Actor) error {
	if actor != nil {
		return events.PublishRequired(ctx, b, e, delegatedactor.AuditHandler)
	}
	return events.Publish(ctx, b, e)
}

func (s *Service) postWritten(id uuid.UUID, storeID int64, body string, version int64, actor *delegatedactor.Actor) model.PostWritten {
	return model.PostWritten{
		EventID: uuid.NewV7(), OccurredAt: s.d.Now().UTC(), PostID: id, StoreID: storeID,
		Preview: model.TruncateRunes(body, model.PreviewMaxRunes), Version: version, Actor: actor,
	}
}

func (s *Service) publishLocked(ctx context.Context, q *store.Queries, row store.SocialPost, now time.Time, origin model.PostOrigin, sc store.SocialPostSchedule, actor *delegatedactor.Actor) error {
	if err := requirePublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
		return err
	}
	upd, err := q.SetPostStatus(ctx, store.SetPostStatusParams{ID: row.ID, Status: string(model.StatusPublished), Now: now, ExpectedVersion: row.Version})
	if errors.Is(err, pgx.ErrNoRows) {
		return versionConflict(ctx)
	}
	if err != nil {
		return fmt.Errorf("social：發布貼文 %s：%w", row.ID, err)
	}
	if err := s.publishedEvent(ctx, upd, now, origin, sc, actor); err != nil {
		return err
	}
	return s.d.Cache.BustPost(ctx, row.ID, row.StoreID)
}

// Create 新增貼文。Publish 為真時同一個動作發布（公開要件在這裡驗證），草稿不發 PostPublished（SD8）。
//
// 代行者建立的貼文一律是草稿：代行的 CreateStorePost 沒有 publish 欄位，要公開走 PublishStorePost（posts.publish 與 POST_PUBLISH 稽核）。
// 建立時順便發布的 PostPublished 不帶代行者、也沒有稽核，所以代行者加 Publish 在這裡拒絕，不讓它產生一個沒稽核的發布。
func (s *Service) Create(ctx context.Context, st seller.Store, in model.CreatePostInput, actor *delegatedactor.Actor) (model.Post, error) {
	if actor != nil && in.Publish {
		return model.Post{}, errors.New("social：代行建立貼文不可同時發布（沒有 posts.publish 的稽核），請另呼叫發布")
	}
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
	var hash []byte
	if in.RequestID != nil {
		hash = model.RequestHash("post", body, in.ImageToken, optionalUUID(in.ProductID), tagsKey(tags), fmt.Sprint(in.Publish))
		row, ok, err := s.replay(ctx, s.q(ctx), st.StoreID, in.RequestID, hash)
		if err != nil {
			return model.Post{}, err
		}
		if ok {
			return s.Own(ctx, st.StoreID, row.ID)
		}
	}
	if err := s.verifyProduct(ctx, in.ProductID, st.StoreID); err != nil {
		return model.Post{}, err
	}
	id := s.d.NewID()
	now := s.d.Now().UTC()
	params := store.InsertPostParams{ID: id, StoreID: st.StoreID, Body: body, ProductID: in.ProductID, Status: string(model.StatusDraft), Now: now, RequestID: in.RequestID, RequestHash: hash}
	if in.Publish {
		params.Status, params.PublishedAt = string(model.StatusPublished), &now
	}
	var out model.Post
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		asset, err := s.confirmImage(ctx, st, in.ImageToken)
		if err != nil {
			return err
		}
		if asset != nil {
			params.ImageKey, params.ImageAssetID = &asset.ObjectKey, &asset.ID
		}
		row, err := q.InsertPost(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			existing, ok, rerr := s.replay(ctx, q, st.StoreID, in.RequestID, hash)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return fmt.Errorf("social：貼文的 request_id %s 衝突，卻查不到原本那一篇", in.RequestID)
			}
			if out, err = s.Own(ctx, st.StoreID, existing.ID); err != nil {
				return err
			}
			return errReplayed
		}
		if err != nil {
			return fmt.Errorf("social：新增貼文：%w", err)
		}
		if err := s.writeTags(ctx, q, id, tags); err != nil {
			return err
		}
		if in.Publish {
			if err := s.publishedEvent(ctx, row, now, model.OriginDirect, store.SocialPostSchedule{}, nil); err != nil {
				return err
			}
			if err := s.d.Cache.BustPost(ctx, id, st.StoreID); err != nil {
				return err
			}
		}
		if out, err = s.Own(ctx, st.StoreID, id); err != nil {
			return err
		}
		return emit(ctx, s.d.Events, model.PostCreated{PostWritten: s.postWritten(out.ID, out.StoreID, out.Body, out.Version, actor)}, actor)
	})
	if err != nil && in.RequestID != nil && errors.Is(err, media.ErrUploadAlreadyConfirmed) {
		row, ok, rerr := s.replay(ctx, s.q(ctx), st.StoreID, in.RequestID, hash)
		if rerr != nil {
			return model.Post{}, rerr
		}
		if ok {
			return s.Own(ctx, st.StoreID, row.ID)
		}
		return model.Post{}, err
	}
	if err != nil && !errors.Is(err, errReplayed) {
		return model.Post{}, err
	}
	return out, nil
}

func optionalUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func tagsKey(tags []hashtag.Tag) string {
	var b strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&b, "%d:%s|%d:%s;", len(t.Name), t.Name, len(t.Display), t.Display)
	}
	return b.String()
}

func (s *Service) replay(ctx context.Context, q *store.Queries, storeID int64, requestID *uuid.UUID, hash []byte) (store.SocialPost, bool, error) {
	if requestID == nil {
		return store.SocialPost{}, false, nil
	}
	row, err := q.PostByRequest(ctx, store.PostByRequestParams{StoreID: storeID, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SocialPost{}, false, nil
	}
	if err != nil {
		return store.SocialPost{}, false, fmt.Errorf("social：讀取貼文的 request_id %s：%w", requestID, err)
	}
	if !bytes.Equal(row.RequestHash, hash) {
		return store.SocialPost{}, false, apperr.InvalidInputOn([]string{"RequestID"}, "%s", text.RequestIdReused(uitext.LocaleFrom(ctx)))
	}
	return row, true, nil
}

func (s *Service) lockOwned(ctx context.Context, q *store.Queries, storeID int64, id uuid.UUID) (store.SocialPost, error) {
	row, err := q.LockPost(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SocialPost{}, notFound(ctx)
	}
	if err != nil {
		return store.SocialPost{}, fmt.Errorf("social：鎖定貼文 %s：%w", id, err)
	}
	if row.StoreID != storeID {
		return store.SocialPost{}, s.foreignWriteError(ctx, id)
	}
	return row, nil
}

func rejectIfTakenDown(ctx context.Context, q *store.Queries, postID uuid.UUID, msg uitext.Message) error {
	down, err := q.PostIsTakenDown(ctx, &postID)
	if err != nil {
		return fmt.Errorf("social：讀取貼文的下架 %s：%w", postID, err)
	}
	if down {
		return apperr.FailedPrecondition("%s", msg)
	}
	return nil
}

func (s *Service) foreignWriteError(ctx context.Context, id uuid.UUID) error {
	pub, err := reads.One[reads.BoolRow](ctx, s.d.DB.Conn(ctx), reads.PostIsPublic(id, s.d.Now().UTC(), visibility.Viewer{}))
	if err != nil {
		return fmt.Errorf("social：判斷貼文 %s 是否公開：%w", id, err)
	}
	if pub.Value {
		return apperr.PermissionDenied("%s", text.PostOwnerOnly(uitext.LocaleFrom(ctx)))
	}
	return notFound(ctx)
}

// Update 修改貼文：Version 與目前的不符回 VersionConflict、不寫入。已發布的貼文被編輯後仍要滿足公開要件。
func (s *Service) Update(ctx context.Context, st seller.Store, in model.UpdatePostInput, actor *delegatedactor.Actor) (model.Post, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.Post{}, err
	}
	if in.Version <= 0 {
		return model.Post{}, apperr.InvalidInput("%s", text.PostUpdateEtagRequired(uitext.LocaleFrom(ctx)))
	}
	var newBody *string
	if in.Body != nil {
		b, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.PostBody, *in.Body, model.MaxPostBodyRunes, false)
		if err != nil {
			return model.Post{}, err
		}
		newBody = &b
	}
	var err error
	var tags []hashtag.Tag
	if in.Hashtags != nil {
		if tags, err = hashtag.RequireSet(uitext.LocaleFrom(ctx), *in.Hashtags); err != nil {
			return model.Post{}, err
		}
	}
	if in.ClearImage && in.ImageToken != "" {
		return model.Post{}, apperr.InvalidInputOn([]string{"ClearImage"}, "%s", text.PostImageReplaceAndClear(uitext.LocaleFrom(ctx)))
	}
	if in.SetProduct {
		if err := s.verifyProduct(ctx, in.ProductID, st.StoreID); err != nil {
			return model.Post{}, err
		}
	}
	now := s.d.Now().UTC()
	var out model.Post
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, in.PostID)
		if err != nil {
			return err
		}
		if err := rejectIfTakenDown(ctx, q, row.ID, text.PostUpdateTakenDown(uitext.LocaleFrom(ctx))); err != nil {
			return err
		}
		if row.Version != in.Version {
			return versionConflict(ctx)
		}
		body, productID := row.Body, row.ProductID
		if newBody != nil {
			body = *newBody
		}
		if in.SetProduct {
			productID = in.ProductID
		}
		hasImage := row.ImageKey != nil && !in.ClearImage || in.ImageToken != ""
		scheduled, err := s.hasPendingSchedule(ctx, q, in.PostID)
		if err != nil {
			return err
		}
		if row.Status == string(model.StatusPublished) || scheduled {
			if err := checkPublishable(ctx, body, hasImage); err != nil {
				return err
			}
		}
		params := store.UpdatePostParams{ID: in.PostID, Body: body, ProductID: productID, ClearImage: in.ClearImage, Now: now, ExpectedVersion: in.Version}
		asset, err := s.confirmImage(ctx, st, in.ImageToken)
		if err != nil {
			return err
		}
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
		kept := row.ImageAssetID
		if in.ClearImage {
			kept = nil
		}
		if asset != nil {
			kept = &asset.ID
		}
		if retired := media.RetiredAssets(media.AssetIDs(row.ImageAssetID), media.AssetIDs(kept)); len(retired) > 0 {
			if err := s.d.Media.MarkDeleted(ctx, retired); err != nil {
				return err
			}
		}
		if upd.Status == string(model.StatusPublished) {
			if err := s.d.Cache.BustPost(ctx, in.PostID, st.StoreID); err != nil {
				return err
			}
		}
		if out, err = s.Own(ctx, st.StoreID, in.PostID); err != nil {
			return err
		}
		return emit(ctx, s.d.Events, model.PostUpdated{PostWritten: s.postWritten(out.ID, out.StoreID, out.Body, out.Version, actor)}, actor)
	})
	if err != nil {
		return model.Post{}, err
	}
	return out, nil
}

// Publish 發布貼文（草稿或下架的貼文轉為 PUBLISHED）。published_at 固定在第一次發布（動態牆的順序不能靠下架再發布來洗：
// 下架後再發布沿用原值）；被內容下架的貼文一律拒絕（FailedPrecondition），已經是 PUBLISHED 也一樣，檢查在授權、etag 必填與貼文歸屬（lockOwned）之後、重複發布的冪等與 etag 比對之前；
// 沒被下架而已經是 PUBLISHED 時是冪等的 no-op：不寫入、不發 PostPublished、不遞增 version、不清快取
// （與 Unpublish 對已下架的冪等一致，所以也不檢查 version）。
func (s *Service) Publish(ctx context.Context, st seller.Store, postID uuid.UUID, version int64, actor *delegatedactor.Actor) (model.Post, error) {
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
		if err := rejectIfTakenDown(ctx, q, row.ID, text.PostPublishTakenDown(uitext.LocaleFrom(ctx))); err != nil {
			return err
		}
		if row.Status == string(model.StatusPublished) {
			return emit(ctx, s.d.Events, model.PostPublishRepeated{PostWritten: s.postWritten(row.ID, row.StoreID, row.Body, row.Version, actor)}, actor)
		}
		if row.Version != version {
			return versionConflict(ctx)
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{postID}, model.CancelReplaced, now, schedule.CallerBumps); err != nil {
			return err
		}
		if err := s.publishLocked(ctx, q, row, now, model.OriginDirect, store.SocialPostSchedule{}, actor); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, postID)
}

// Unpublish 下架貼文（PUBLISHED 轉為 UNPUBLISHED；已經下架的冪等）。草稿沒有發布過，不能下架。
func (s *Service) Unpublish(ctx context.Context, st seller.Store, postID uuid.UUID, version int64, actor *delegatedactor.Actor) (model.Post, error) {
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
			return emit(ctx, s.d.Events, model.PostUnpublishRepeated{PostWritten: s.postWritten(row.ID, row.StoreID, row.Body, row.Version, actor)}, actor)
		case model.StatusDraft:
			return apperr.FailedPrecondition("%s", text.PostUnpublishDraft(uitext.LocaleFrom(ctx)))
		case model.StatusPublished:
		}
		if row.Version != version {
			return versionConflict(ctx)
		}
		upd, err := q.SetPostStatus(ctx, store.SetPostStatusParams{ID: postID, Status: string(model.StatusUnpublished), Now: now, ExpectedVersion: version})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return versionConflict(ctx)
			}
			return fmt.Errorf("social：下架貼文 %s：%w", postID, err)
		}
		if err := s.d.Cache.BustPost(ctx, postID, st.StoreID); err != nil {
			return err
		}
		return emit(ctx, s.d.Events, model.PostUnpublished{PostWritten: s.postWritten(upd.ID, upd.StoreID, upd.Body, upd.Version, actor)}, actor)
	})
	if err != nil {
		return model.Post{}, err
	}
	return s.Own(ctx, st.StoreID, postID)
}

// DeleteOwn 軟刪貼文。不存在、已刪除與別家未公開的貼文回 NotFound（AIP-135），別家已公開的貼文回 PermissionDenied（lockOwned 的規則）；
// 沒刪到不做副作用。version 為 0 表示不檢查版本。軟刪保留內容與圖片（資產不標記刪除；保留期還沒訂，見 CARRYOVER）。
func (s *Service) DeleteOwn(ctx context.Context, st seller.Store, postID uuid.UUID, version int64) error {
	if err := seller.Authorize(st, seller.CapContentContract); err != nil {
		return err
	}
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := s.lockOwned(ctx, q, st.StoreID, postID); err != nil {
			return err
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{postID}, model.CancelDeleted, now, schedule.CallerBumps); err != nil {
			return err
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
