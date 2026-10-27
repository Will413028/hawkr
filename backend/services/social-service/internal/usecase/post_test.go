package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func newTestPostUC(repo *mockPostRepo, media *mockMediaClient, seller *mockSellerClient) *PostUseCase {
	return NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, media, seller, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)
}

func TestPostUC_GetPost_Success(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	post, err := uc.GetPost(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if post.ID != 99 {
		t.Errorf("post.ID = %d, want 99", post.ID)
	}
}

func TestPostUC_GetPost_NotFound(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return nil, errors.New("no rows")
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	_, err := uc.GetPost(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestPostUC_ListPosts_PageDefaults(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockPostRepo{
		listFn: func(_ context.Context, _ int64, limit, offset int64) ([]*gen.CmsPost, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		countFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	_, _, err := uc.ListPosts(context.Background(), 1, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 20 {
		t.Errorf("limit = %d, want 20 (default pageSize)", gotLimit)
	}
	if gotOffset != 0 {
		t.Errorf("offset = %d, want 0 (page 1)", gotOffset)
	}
}

func TestPostUC_UpdatePost_Forbidden(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil // owner is seller 10
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	_, err := uc.UpdatePost(context.Background(), 1, 999, "new content", "", "", nil, false, nil, nil)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error = %v, want ErrForbidden", err)
	}
}

func TestPostUC_UpdatePost_Success(t *testing.T) {
	updated := &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 1, SellerID: 10, Content: testStringPointer("updated"), Image: testStringPointer("posts/a.jpg")}
	callCount := 0
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			callCount++
			return updated, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error {
			return nil
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	post, err := uc.UpdatePost(context.Background(), 1, 10, "updated", "111", "", nil, false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if post.Content == nil || *post.Content != "updated" {
		t.Errorf("content = %v, want %q", post.Content, "updated")
	}
}

// #1489 原本要求「刪除貼文要釋放圖片引用（ref_type=POST, ref_id=postID）讓資產可被
// GC」；#2774 之後刪除路徑不再直接呼叫 media，所以這裡要驗的是「一次都沒呼叫」。
// 舊行為（best-effort Release + WARN）正是「DB 已刪、media 失敗、且永遠無法續跑」的
// 成因：重試會在 GetByID（deleted_at IS NULL）就短路，再也走不到 Release。
//
// 釋放也不會在別處補上：2026-09-05 拍板軟刪刻意保留引用（image_asset_id 沒被動過，
// 釋放會製造「業務列指著、卻沒有引用」，資產隨後被 GC 實體刪除），media reconciler
// 的候選集合因此濾掉軟刪列。真正的釋放只在帳號終止（seller.deleted handler）發生。
//
// 這支測試取代了 TestPostUC_DeletePost_ReleasesImageReference，後者斷言的正是
// 現在被移除的那個行為。
func TestPostUC_DeletePost_DoesNotCallMedia(t *testing.T) {
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) { return 1, nil },
	}
	var releaseCalls int
	media := &mockMediaClient{
		releaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error {
			releaseCalls++
			return nil
		},
	}
	uc := newTestPostUC(repo, media, &mockSellerClient{})
	if err := uc.DeletePost(context.Background(), 777, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if releaseCalls != 0 {
		t.Errorf("releaseCalls = %d, want 0 —— 釋放是 worker 的職責，不是刪除路徑的", releaseCalls)
	}
}

// releaseCall 記錄一次 ReleaseReferences 呼叫的三個參數，供下面幾支測試斷言。
type releaseCall struct {
	refType string
	refIDs  []string
	keep    []string
}

// 換圖必須在 commit 之後送出 ③ Release(keep=[新資產])。順序契約見 pkg/media.LinkImages：
// ① Link（致命）→ ② 業務列寫入（致命）→ ③ Release（best-effort），六個媒體擁有者共用。
//
// 這支測試曾經被反寫成 TestPostUC_UpdatePost_ReplaceDoesNotReleaseOld，斷言「不得呼叫」
// ——那是把釋放移交給 media reconciler 定期重放的那一版設計。釋放改回寫入路徑之後，
// 不送 ③ 的後果是舊資產的引用永遠留著、永遠不被 GC 回收（儲存洩漏）。
func TestPostUC_UpdatePost_ReplaceReleasesOldKeepingNew(t *testing.T) {
	updated := &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 1, SellerID: 10, Content: testStringPointer("updated")}
	var gotImageURL string
	var gotImageAssetID *int64
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return updated, nil },
		updateFn: func(_ context.Context, _ int64, _ string, imageURL string, imageAssetID *int64, _ *int64) error {
			gotImageURL, gotImageAssetID = imageURL, imageAssetID
			return nil
		},
	}
	var calls []releaseCall
	mediaCli := &mockMediaClient{
		releaseReferencesFn: func(_ context.Context, refType string, refIDs, keep []string) error {
			calls = append(calls, releaseCall{refType: refType, refIDs: refIDs, keep: keep})
			return nil
		},
		finalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "222", ObjectKey: "posts/new.jpg"}, nil
		},
	}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "updated", "", "new-staged-token", nil, false, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("ReleaseReferences 呼叫 %d 次，want 1 —— 沒送 ③ 的話舊資產的引用永遠留著、永遠不被 GC 回收", len(calls))
	}
	got := calls[0]
	if got.refType != pkgmedia.RefTypePost {
		t.Errorf("refType = %q, want %q", got.refType, pkgmedia.RefTypePost)
	}
	if len(got.refIDs) != 1 || got.refIDs[0] != "1" {
		t.Errorf("refIDs = %v, want [1]", got.refIDs)
	}
	// keep 必須是**新**資產。傳錯（空集合、或舊 asset id）會在 media 的同一個交易裡
	// 把剛掛上的新引用一起刪掉，貼文還顯示著那張圖、資產卻進了 GC 的孤兒名單。
	if len(got.keep) != 1 || got.keep[0] != "222" {
		t.Errorf("keepAssetIDs = %v, want [222] —— keep 不是新資產就會連新引用一起釋放", got.keep)
	}
	if gotImageURL != "posts/new.jpg" {
		t.Errorf("imageURL passed to Update = %q, want %q", gotImageURL, "posts/new.jpg")
	}
	if gotImageAssetID == nil || *gotImageAssetID != 222 {
		t.Errorf("imageAssetID passed to Update = %v, want *int64(222)", gotImageAssetID)
	}
}

// 清圖：desired 變成空集合，③ 必須以 keep 空送出，把該貼文全部引用釋放。
func TestPostUC_UpdatePost_ClearImageReleasesAll(t *testing.T) {
	existing := &gen.CmsPost{
		PublishStatus: domain.PostPublishStatusDraft, // 草稿可以清圖；已發布或排程中不行（B1）
		ID:            1,
		SellerID:      10,
		Image:         testStringPointer("posts/old.jpg"),
		ImageAssetID:  nullableTestInt64(111),
	}
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return existing, nil },
		updateFn:  func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	var calls []releaseCall
	mediaCli := &mockMediaClient{releaseReferencesFn: func(_ context.Context, refType string, refIDs, keep []string) error {
		calls = append(calls, releaseCall{refType: refType, refIDs: refIDs, keep: keep})
		return nil
	}}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "no image any more", "", "", nil, false, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("ReleaseReferences 呼叫 %d 次，want 1 —— 清圖沒送 ③，那張圖的引用永遠留著", len(calls))
	}
	if len(calls[0].keep) != 0 {
		t.Errorf("keepAssetIDs = %v, want empty（keep 空 = 釋放全部）", calls[0].keep)
	}
}

// legacy 列（image_asset_id 是 NULL、image 有值，回填前的舊資料）被清圖時同樣要送 ③。
// 條件若寫成「舊 image_asset_id 非 NULL」就會漏掉這一類列，而它們在 media 上可能有引用。
//
// 這與「不得照 image_asset_id 推導 legacy 列的 desired」**不是同一件事**（見 schema.sql
// 該欄註解）：那裡的 desired 是「讀不出來」（貼文還顯示著那張圖，釋放等於實刪活資產），
// 這裡是「使用者剛把它清掉」，desired 確實是空集合。
func TestPostUC_UpdatePost_ClearImageOnLegacyRowStillReleases(t *testing.T) {
	legacy := &gen.CmsPost{
		PublishStatus: domain.PostPublishStatusDraft,
		ID:            1,
		SellerID:      10,
		Image:         testStringPointer("public/post/legacy.jpg"),
		ImageAssetID:  nil,
	}
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return legacy, nil },
		updateFn:  func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	var called bool
	mediaCli := &mockMediaClient{releaseReferencesFn: func(_ context.Context, _ string, _, keep []string) error {
		called = true
		if len(keep) != 0 {
			t.Errorf("keepAssetIDs = %v, want empty", keep)
		}
		return nil
	}}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "cleared", "", "", nil, false, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("legacy 列清圖沒送 ③ —— 條件寫成「舊 image_asset_id 非 NULL」就會漏掉這一整類列")
	}
}

// 本來就沒有圖的貼文做內容編輯，不該送出任何 ③：那是一次確定不會改變任何東西的 RPC，
// 每一次貼文編輯都多打一趟 media。
func TestPostUC_UpdatePost_NoImageNeverReleases(t *testing.T) {
	bare := &gen.CmsPost{PublishStatus: domain.PostPublishStatusDraft, ID: 1, SellerID: 10}
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return bare, nil },
		updateFn:  func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	var called bool
	mediaCli := &mockMediaClient{releaseReferencesFn: func(_ context.Context, _ string, _, _ []string) error {
		called = true
		return nil
	}}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "text only", "", "", nil, false, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("沒有圖的貼文不該送出 ReleaseReferences")
	}
}

// ③ 是 best-effort：它唯一能造成的損害是舊資產不被回收（洩漏，可事後 SQL 清）。
// 把它的錯誤往上拋會讓一次成功的貼文編輯回報失敗，而業務列早已 commit——使用者
// 會再送一次，那才是真正的資料問題。
func TestPostUC_UpdatePost_ReleaseFailureDoesNotFailUpdate(t *testing.T) {
	existing := &gen.CmsPost{
		PublishStatus: domain.PostPublishStatusPublished,
		ID:            1,
		SellerID:      10,
		Image:         testStringPointer("posts/old.jpg"),
		ImageAssetID:  nullableTestInt64(111),
	}
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return existing, nil },
		updateFn:  func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	mediaCli := &mockMediaClient{
		releaseReferencesFn: func(_ context.Context, _ string, _, _ []string) error {
			return errors.New("media unavailable")
		},
		finalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "222", ObjectKey: "posts/new.jpg"}, nil
		},
	}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "updated", "", "new-staged-token", nil, false, nil, nil); err != nil {
		t.Fatalf("UpdatePost err = %v，want nil —— ③ 失敗只是洩漏，不該讓已 commit 的編輯回報失敗", err)
	}
}

// #2772 fix round (task review Important 3): the keepAssetID branch — a
// content-only edit that keeps the existing image — was completely
// unexercised at the usecase layer before this test; every other UpdatePost
// test in this file passes keepAssetID="". A regression that drops
// post.ImageAssetID here (e.g. by forgetting to thread it through, or by
// passing nil) reproduces the exact pre-Task-4 shape: image stays the same
// while image_asset_id goes NULL, so the row's declared desired set goes empty
// while the post still displays that image — the reconciler's very next round
// would then release the reference to it, and media GC would delete the asset
// for good. Nothing below this layer can catch it: UpdatePost's SQL writes
// whatever asset id it is handed, so deciding to pass the unchanged one is the
// usecase's call alone — that is what this test pins.
func TestPostUC_UpdatePost_KeepAssetIDPassesThroughUnchanged(t *testing.T) {
	existing := &gen.CmsPost{
		PublishStatus: domain.PostPublishStatusPublished,
		ID:            1,
		SellerID:      10,
		Image:         testStringPointer("posts/old.jpg"),
		ImageAssetID:  nullableTestInt64(111),
	}
	var gotImageURL string
	var gotImageAssetID *int64
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) { return existing, nil },
		updateFn: func(_ context.Context, _ int64, _ string, imageURL string, imageAssetID *int64, _ *int64) error {
			gotImageURL, gotImageAssetID = imageURL, imageAssetID
			return nil
		},
	}
	var releaseCalled bool
	mediaCli := &mockMediaClient{releaseReferencesFn: func(_ context.Context, _ string, _, _ []string) error {
		releaseCalled = true
		return nil
	}}
	uc := newTestPostUC(repo, mediaCli, &mockSellerClient{})
	if _, err := uc.UpdatePost(context.Background(), 1, 10, "content only edit", "existing", "", nil, false, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 宣告沒變 ⇒ ③ 是確定的 no-op，不送。送了不會壞資料（keep 就是現值），但每一次
	// 內容編輯都會多一趟 media RPC。
	if releaseCalled {
		t.Error("內容編輯（宣告未變）不該送出 ReleaseReferences")
	}
	if gotImageURL != "posts/old.jpg" {
		t.Errorf("imageURL passed to Update = %q, want %q (unchanged)", gotImageURL, "posts/old.jpg")
	}
	if gotImageAssetID == nil || *gotImageAssetID != 111 {
		t.Errorf("imageAssetID passed to Update = %v, want *int64(111) —— 沒把既有 asset id 原樣傳過去，"+
			"content-only 編輯就會把 desired 清空，reconciler 下一輪釋放貼文還在顯示的那張圖", gotImageAssetID)
	}
}

func TestPostUC_ToggleLike_Like(t *testing.T) {
	var createLikeCalled bool
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 1}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return nil, errNotFoundMock // not liked yet
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			createLikeCalled = true
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := newTestPostUCWithPublisher(repo, pub)
	liked, err := uc.ToggleLike(context.Background(), 1, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !liked {
		t.Error("expected liked=true")
	}
	if !createLikeCalled {
		t.Error("CreateLike was not called")
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	want := publishCounterDeltaCall{entityType: "post", entityID: 1, field: "like_count", delta: 1}
	if pub.publishCounterDeltaCalls[0] != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", pub.publishCounterDeltaCalls[0], want)
	}
}

func TestPostUC_ToggleLike_Unlike(t *testing.T) {
	var deleteLikeCalled bool
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 1}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return &gen.CmsPostLike{}, nil // already liked
		},
		deleteLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			deleteLikeCalled = true
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	uc := newTestPostUCWithPublisher(repo, pub)
	liked, err := uc.ToggleLike(context.Background(), 1, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if liked {
		t.Error("expected liked=false (unlike)")
	}
	if !deleteLikeCalled {
		t.Error("DeleteLike was not called")
	}
	if len(pub.publishCounterDeltaCalls) != 1 {
		t.Fatalf("publishCounterDeltaCalls = %d, want 1", len(pub.publishCounterDeltaCalls))
	}
	want := publishCounterDeltaCall{entityType: "post", entityID: 1, field: "like_count", delta: -1}
	if pub.publishCounterDeltaCalls[0] != want {
		t.Errorf("publishCounterDeltaCalls[0] = %+v, want %+v", pub.publishCounterDeltaCalls[0], want)
	}
}

func TestPostUC_ToggleLike_PostNotFound(t *testing.T) {
	var createLikeCalled bool
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return nil, errNotFoundMock
		},
		createLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) error {
			createLikeCalled = true
			return nil
		},
	}
	uc := newTestPostUC(repo, &mockMediaClient{}, &mockSellerClient{})
	_, err := uc.ToggleLike(context.Background(), 999999999999999999, 100, 0)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if createLikeCalled {
		t.Error("CreateLike must not be called when post does not exist (would insert orphan row)")
	}
}

func TestPostUC_IsLiked_ZeroUserID(t *testing.T) {
	uc := newTestPostUC(&mockPostRepo{}, &mockMediaClient{}, &mockSellerClient{})
	if uc.IsLiked(context.Background(), 1, 0) {
		t.Error("expected false for userID=0")
	}
}

func TestPostUC_GetSellerInfo_ZeroID(t *testing.T) {
	uc := newTestPostUC(&mockPostRepo{}, &mockMediaClient{}, &mockSellerClient{})
	name, logo := uc.GetSellerInfo(context.Background(), 0)
	if name != "" || logo != "" {
		t.Errorf("expected empty strings for sellerID=0, got %q %q", name, logo)
	}
}

func newTestPostUCWithProduct(repo *mockPostRepo, media *mockMediaClient, seller *mockSellerClient, product *mockProductClient) *PostUseCase {
	return NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, media, seller, &mockBuyerClient{}, product, nil, &config.Config{}, func() int64 { return 99999 }, nil)
}

func TestPostUC_GetProductsByIDs_Success(t *testing.T) {
	product := &mockProductClient{
		getProductsByIDsFn: func(_ context.Context, ids []string) (map[string]*ProductBasicInfo, error) {
			result := make(map[string]*ProductBasicInfo)
			for _, id := range ids {
				result[id] = &ProductBasicInfo{
					ID:       id,
					Name:     "商品" + id,
					ImageURL: "https://cdn.example.com/img/" + id + ".jpg",
					Price:    "450",
				}
			}
			return result, nil
		},
	}
	uc := newTestPostUCWithProduct(&mockPostRepo{}, &mockMediaClient{}, &mockSellerClient{}, product)

	result := uc.GetProductsByIDs(context.Background(), []string{"100", "200"})
	if len(result) != 2 {
		t.Fatalf("got %d products, want 2", len(result))
	}
	if result["100"].Name != "商品100" {
		t.Errorf("product 100 name = %q, want %q", result["100"].Name, "商品100")
	}
	if result["200"].Price != "450" {
		t.Errorf("product 200 price = %q, want %q", result["200"].Price, "450")
	}
}

func TestPostUC_GetProductsByIDs_Empty(t *testing.T) {
	uc := newTestPostUC(&mockPostRepo{}, &mockMediaClient{}, &mockSellerClient{})
	result := uc.GetProductsByIDs(context.Background(), nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestPostUC_GetProductsByIDs_Error(t *testing.T) {
	product := &mockProductClient{
		getProductsByIDsFn: func(_ context.Context, _ []string) (map[string]*ProductBasicInfo, error) {
			return nil, errors.New("product-service unavailable")
		},
	}
	uc := newTestPostUCWithProduct(&mockPostRepo{}, &mockMediaClient{}, &mockSellerClient{}, product)

	result := uc.GetProductsByIDs(context.Background(), []string{"100"})
	if result != nil {
		t.Errorf("expected nil on error, got %v", result)
	}
}

// ==================== ReportPost ====================
//
// ReportPost now delegates to ReportUseCase.SubmitReport (task 5, B-135
// unified path). These tests exercise the delegation itself — validation,
// dedup, and rate-limit behavior are covered by report_test.go's
// TestSubmitReport_* suite and are not re-asserted here.

type fakePostRepoWithPost struct {
	domain.PostRepository
	post *gen.CmsPost
}

func (f *fakePostRepoWithPost) GetByID(ctx context.Context, id int64) (*gen.CmsPost, error) {
	return f.post, nil
}

func TestReportPost_DelegatesToSubmitReport(t *testing.T) {
	reportRepo := &fakeReportRepo{}
	reportUC := NewReportUseCase(reportRepo, nil, nil, nil, nil, func() int64 { return 42 })
	postRepo := &fakePostRepoWithPost{post: &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 100, SellerID: 7}}
	uc := &PostUseCase{postRepo: postRepo, reportUC: reportUC, tsid: func() int64 { return 42 }}

	report, err := uc.ReportPost(context.Background(), 1, 100, "SPAM", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TargetType != "POST" || report.TargetID != 100 {
		t.Errorf("expected POST/100, got %s/%d", report.TargetType, report.TargetID)
	}
}

func TestReportPost_InvalidReasonCode(t *testing.T) {
	reportUC := NewReportUseCase(&fakeReportRepo{}, nil, nil, nil, nil, func() int64 { return 1 })
	postRepo := &fakePostRepoWithPost{post: &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 100, SellerID: 7}}
	uc := &PostUseCase{postRepo: postRepo, reportUC: reportUC}

	_, err := uc.ReportPost(context.Background(), 1, 100, "ADULT", "test") // 舊值已廢棄
	if !errors.Is(err, ErrInvalidReason) {
		t.Fatalf("expected ErrInvalidReason, got %v", err)
	}
}

func TestReportPost_NotFound(t *testing.T) {
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, _ int64) (*gen.CmsPost, error) {
			return nil, errors.New("no rows")
		},
	}
	reportUC := NewReportUseCase(&fakeReportRepo{}, nil, nil, nil, nil, func() int64 { return 1 })
	uc := &PostUseCase{postRepo: postRepo, reportUC: reportUC}

	_, err := uc.ReportPost(context.Background(), 1, 999, "SPAM", "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// ==================== RestorePost ====================

func TestPostUC_RestorePost_Success(t *testing.T) {
	var restoreCalled bool
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			// 回傳 hidden 狀態的貼文，確保 RestorePost 會被呼叫
			return &gen.CmsPost{
				PublishStatus: domain.PostPublishStatusPublished,
				ID:            id,
				SellerID:      10,
				HiddenAt:      testTimePointer(time.Now()),
			}, nil
		},
		restorePostFn: func(_ context.Context, _ int64) error {
			restoreCalled = true
			return nil
		},
	}
	uc := NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	err := uc.RestorePost(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !restoreCalled {
		t.Error("RestorePost was not called on repository")
	}
}

// ==================== ListMyPosts ====================

func TestPostUC_ListMyPosts_PageDefaults(t *testing.T) {
	var gotLimit, gotOffset int64
	repo := &mockPostRepo{
		listMyFn: func(_ context.Context, _ int64, _ string, limit, offset int64) ([]*gen.CmsPost, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
	}
	uc := NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	_, _, err := uc.ListMyPosts(context.Background(), 10, "", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 20 {
		t.Errorf("default limit = %d, want 20", gotLimit)
	}
	if gotOffset != 0 {
		t.Errorf("default offset = %d, want 0", gotOffset)
	}
}

func TestPostUC_ListMyPosts_HiddenStatusFilter(t *testing.T) {
	var gotStatus string
	repo := &mockPostRepo{
		listMyFn: func(_ context.Context, _ int64, status string, _, _ int64) ([]*gen.CmsPost, error) {
			gotStatus = status
			return []*gen.CmsPost{
				{PublishStatus: domain.PostPublishStatusPublished, ID: 1, HiddenAt: testTimePointer(time.Now())},
			}, nil
		},
	}
	uc := NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	posts, _, err := uc.ListMyPosts(context.Background(), 10, "hidden", 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStatus != "hidden" {
		t.Errorf("status passed to repo = %q, want hidden", gotStatus)
	}
	if len(posts) != 1 {
		t.Errorf("got %d posts, want 1", len(posts))
	}
	if posts[0].HiddenAt == nil {
		t.Error("expected hidden post to have HiddenAt set")
	}
}

func TestPostUC_CreatePost_FinalizeImage(t *testing.T) {
	var finalizeCalled bool
	media := &mockMediaClient{
		finalizeAndLinkFn: func(_ context.Context, stagedToken, _, _ string) (AssetInfo, error) {
			finalizeCalled = true
			return AssetInfo{AssetId: "555", ObjectKey: "images/photo.jpg"}, nil
		},
	}
	var gotImageURL string
	var gotAssetID *int64
	repo := &mockPostRepo{
		createFn: func(_ context.Context, _, _ int64, _ string, imageURL string, imageAssetID *int64, _ *int64) (*gen.CmsPost, error) {
			gotImageURL = imageURL
			gotAssetID = imageAssetID
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 1}, nil
		},
	}
	uc := newTestPostUC(repo, media, &mockSellerClient{})
	_, err := uc.CreatePost(context.Background(), 10, "hello", "staged-token-123", nil, nil, domain.PostPublishStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !finalizeCalled {
		t.Error("FinalizeAndLink was not called")
	}
	if gotImageURL != "images/photo.jpg" {
		t.Errorf("imageURL = %q, want %q", gotImageURL, "images/photo.jpg")
	}
	// #2773/#2772: proves finalizeAndLinkAsset's parsed asset id actually reaches
	// the repository call, not just the object key. Without this, a regression
	// that drops the id silently (e.g. finalizeAndLinkAsset returning nil) would
	// leave image_asset_id NULL on every new post with an image — that row is a
	// legacy-shaped row (image set, asset id NULL), which ListLiveMediaRefs
	// deliberately excludes, so the reconciler never touches it and the reference
	// leaks forever, with no other test catching it.
	if gotAssetID == nil || *gotAssetID != 555 {
		t.Errorf("imageAssetID = %v, want *int64(555)", gotAssetID)
	}
}

// 49 §6.2：cms_post.image 只存 objectKey，即使 MEDIA_BASE_URL 已設定也不得把完整 URL 落庫
// （顯示 URL 由 Gateway 組）。base 非空時仍必須存裸 key，釘住「服務端不組對外 URL」。
func TestPostUC_CreatePost_StoresObjectKeyNotURL(t *testing.T) {
	media := &mockMediaClient{
		finalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "555", ObjectKey: "images/photo.jpg"}, nil
		},
	}
	var gotImageURL string
	repo := &mockPostRepo{
		createFn: func(_ context.Context, _, _ int64, _ string, imageURL string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			gotImageURL = imageURL
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: 1}, nil
		},
	}
	cfg := &config.Config{MediaBaseURL: "https://test-media.example.com"}
	uc := NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, media, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, cfg, func() int64 { return 99999 }, nil)
	if _, err := uc.CreatePost(context.Background(), 10, "hello", "staged-token-123", nil, nil, domain.PostPublishStatusPublished); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotImageURL != "images/photo.jpg" {
		t.Errorf("stored image = %q, want bare objectKey %q (not a CDN URL)", gotImageURL, "images/photo.jpg")
	}
}

// --- post.created publish 帶 sellerAvatar 絕對 URL（CDN）---

func TestPostUC_CreatePost_PublishesAbsoluteSellerAvatar(t *testing.T) {
	const sellerID int64 = 10
	const followerID int64 = 999
	const objectKey = "seller_avatars/avatar.jpg"
	const mediaBaseURL = "https://test-media.example.com"

	media := &mockMediaClient{
		finalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "555", ObjectKey: "images/post.jpg"}, nil
		},
	}
	postRepo := &mockPostRepo{
		createFn: func(_ context.Context, id, _ int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	seller := &mockSellerClient{
		getSellerInfoFn: func(_ context.Context, _ int64) (*AccountInfo, error) {
			return &AccountInfo{Nickname: "店家A", Avatar: objectKey}, nil
		},
	}
	buyerWithSubs := &subscriberMockBuyerClient{
		mockBuyerClient: &mockBuyerClient{},
		subs:            []int64{followerID},
	}
	pub := &mockContentEventPublisher{}
	cfg := &config.Config{MediaBaseURL: mediaBaseURL}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, media, seller, buyerWithSubs, &mockProductClient{}, pub, cfg, func() int64 { return 1 }, nil)

	_, err := uc.CreatePost(context.Background(), sellerID, "hello world", "staged-token", nil, nil, domain.PostPublishStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.postCreatedCalls) != 1 {
		t.Fatalf("expected 1 post.created publish, got %d", len(pub.postCreatedCalls))
	}
	got := pub.postCreatedCalls[0]
	// 49 §6.3/S3：post.created payload 帶裸 objectKey，顯示 URL 由 notification worker 於 push 前組。
	want := objectKey
	if got.sellerAvatar != want {
		t.Errorf("sellerAvatar = %q, want bare objectKey %q", got.sellerAvatar, want)
	}
	if got.followerID != followerID {
		t.Errorf("followerID = %d, want %d", got.followerID, followerID)
	}
}

func TestPostUC_CreatePost_AbsoluteSellerAvatarPassThroughWhenAlreadyURL(t *testing.T) {
	const sellerID int64 = 10
	const followerID int64 = 999
	const fullURL = "https://other-cdn.example.com/avatars/x.jpg"

	media := &mockMediaClient{
		finalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "555", ObjectKey: "images/post.jpg"}, nil
		},
	}
	postRepo := &mockPostRepo{
		createFn: func(_ context.Context, id, _ int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	seller := &mockSellerClient{
		getSellerInfoFn: func(_ context.Context, _ int64) (*AccountInfo, error) {
			return &AccountInfo{Nickname: "店家A", Avatar: fullURL}, nil
		},
	}
	buyerWithSubs := &subscriberMockBuyerClient{
		mockBuyerClient: &mockBuyerClient{},
		subs:            []int64{followerID},
	}
	pub := &mockContentEventPublisher{}
	cfg := &config.Config{MediaBaseURL: "https://test-media.example.com"}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, media, seller, buyerWithSubs, &mockProductClient{}, pub, cfg, func() int64 { return 1 }, nil)

	_, err := uc.CreatePost(context.Background(), sellerID, "hello", "staged-token", nil, nil, domain.PostPublishStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.postCreatedCalls) != 1 {
		t.Fatalf("expected 1 post.created publish, got %d", len(pub.postCreatedCalls))
	}
	if pub.postCreatedCalls[0].sellerAvatar != fullURL {
		t.Errorf("sellerAvatar = %q, want pass-through %q", pub.postCreatedCalls[0].sellerAvatar, fullURL)
	}
}

// ==================== ② store re-key：ToggleLike self-like publish skip ====================
func newTestPostUCWithPublisher(repo *mockPostRepo, pub *mockContentEventPublisher) *PostUseCase {
	return NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, pub, &config.Config{}, func() int64 { return 99999 }, nil)
}

// Stale：貼文屬舊 store 1001（SellerID == userID），現任 store 9999。這其實不是 self-like
// （貼文屬舊 store），應 publish。舊碼 post.SellerID(1001)==userID(1001) → skip（bug）。
func TestPostUC_ToggleLike_SelfLike_StaleSurrogatePublishes(t *testing.T) {
	const userID int64 = 1001
	const staleSurrogate int64 = 1001
	const currentStoreID int64 = 9999
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: staleSurrogate}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return nil, errNotFoundMock // not liked yet → like path → publish check
		},
	}
	pub := &mockContentEventPublisher{}
	uc := newTestPostUCWithPublisher(repo, pub)
	liked, err := uc.ToggleLike(context.Background(), 1, userID, currentStoreID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !liked {
		t.Error("expected liked=true")
	}
	if len(pub.postLikedCalls) != 1 {
		t.Errorf("post.liked publishes = %d, want 1 (post belongs to the old store 1001, not caller's current store 9999 — not a self-like)", len(pub.postLikedCalls))
	}
}

// Current：post.SellerID == 現任 store 2002 → 真正的 self-like，skip publish。
func TestPostUC_ToggleLike_SelfLike_CurrentSurrogateSkips(t *testing.T) {
	const userID int64 = 1001
	const currentStoreID int64 = 2002
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: currentStoreID}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return nil, errNotFoundMock
		},
	}
	pub := &mockContentEventPublisher{}
	uc := newTestPostUCWithPublisher(repo, pub)
	if _, err := uc.ToggleLike(context.Background(), 1, userID, currentStoreID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.postLikedCalls) != 0 {
		t.Errorf("post.liked publishes = %d, want 0 (genuine self-like: post.SellerID == caller's current store 2002)", len(pub.postLikedCalls))
	}
}

func TestPostUC_ToggleLike_PublishesResolvedAccountID_NotRawStoreID(t *testing.T) {
	const storeID int64 = 9999           // post.SellerID — a store surrogate
	const resolvedAccountID int64 = 1001 // what sellerClient.ResolvePrincipal resolves storeID to
	const likerID int64 = 2002

	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: storeID}, nil
		},
		getLikeFn: func(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
			return nil, errNotFoundMock // not liked yet → like path → publish check
		},
	}
	seller := &mockSellerClient{
		resolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == storeID {
				return resolvedAccountID
			}
			return sid
		},
	}
	pub := &mockContentEventPublisher{}
	uc := NewPostUseCase(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 88888 }), &mockReportRepo{}, nil, &mockMediaClient{}, seller, &mockBuyerClient{}, &mockProductClient{}, pub, &config.Config{}, func() int64 { return 99999 }, nil)

	// storeID(caller)=0 → not a seller → definitely not self-like → publish always fires.
	liked, err := uc.ToggleLike(context.Background(), 1, likerID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !liked {
		t.Error("expected liked=true")
	}
	if len(pub.postLikedCalls) != 1 {
		t.Fatalf("expected 1 post.liked publish, got %d", len(pub.postLikedCalls))
	}
	if pub.postLikedCalls[0].sellerID != resolvedAccountID {
		t.Errorf("PublishPostLiked sellerID = %d, want resolved account_id %d, not raw store_id surrogate %d",
			pub.postLikedCalls[0].sellerID, resolvedAccountID, storeID)
	}
	// storeID (Phase A 新欄位) 必須是未解析的 raw cms_post.seller_id，與已解析的
	// sellerID 落在不同身分空間；兩者必須不相等，否則就是誤把 resolved 值複製進了
	// raw 欄位。
	if pub.postLikedCalls[0].storeID != storeID {
		t.Errorf("PublishPostLiked storeID = %d, want raw store_id %d (must not be the resolved value)", pub.postLikedCalls[0].storeID, storeID)
	}
	if pub.postLikedCalls[0].storeID == pub.postLikedCalls[0].sellerID {
		t.Errorf("PublishPostLiked storeID (%d) must differ from resolved sellerID (%d)", pub.postLikedCalls[0].storeID, pub.postLikedCalls[0].sellerID)
	}
}

// ==================== Hashtag integration ====================

// CreatePost with tags should fan out to HashtagUseCase.Apply, which
// upserts each canonical tag and (since the new post is publicly visible)
// applies +1 counter delta per tag.
func TestPostUC_CreatePost_WithTags(t *testing.T) {
	upsertCalls := 0
	// Return distinct hashtag IDs per call (the tsid passed in is the same value
	// each invocation in this test fixture, so we must derive uniqueness from
	// canonical name to mimic real upsert behaviour).
	idByCanonical := map[string]int64{"美食": 1001, "宅配": 1002}
	hashtagRepo := &mockHashtagRepo{
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, canonical string) (int64, error) {
			upsertCalls++
			return idByCanonical[canonical], nil
		},
	}
	postRepo := &mockPostRepo{
		createFn: func(_ context.Context, id, sellerID int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	hashtagUC := NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 })
	uc := NewPostUseCase(postRepo, hashtagUC, &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	_, err := uc.CreatePost(context.Background(), 10, "hello world", "tok", nil, []string{"美食", "宅配"}, domain.PostPublishStatusPublished)
	if err != nil {
		t.Fatalf("CreatePost err = %v", err)
	}
	if upsertCalls != 2 {
		t.Errorf("upsert calls = %d, want 2", upsertCalls)
	}
	// New post is public-visible → counter +1 per tag.
	if len(hashtagRepo.adjustCalls) != 2 {
		t.Errorf("adjust calls = %d, want 2 (visible new post → +1 per tag)", len(hashtagRepo.adjustCalls))
	}
	for _, c := range hashtagRepo.adjustCalls {
		if c.delta != 1 {
			t.Errorf("delta = %d, want 1 for new visible post", c.delta)
		}
	}
}

// UpdatePost with tagsPresent=false must not touch the hashtag repo at all
// (PATCH semantics: nil tags field = "do not modify").
func TestPostUC_UpdatePost_TagsPresentFalseSkipsHashtag(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, Image: testStringPointer("posts/a.jpg")}, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	hashtagUC := NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 })
	uc := NewPostUseCase(postRepo, hashtagUC, &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	_, err := uc.UpdatePost(context.Background(), 1, 10, "new", "111", "", nil, false /* tagsPresent */, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 0 {
		t.Errorf("hashtag should not be touched when tagsPresent=false; adjustCalls=%v", hashtagRepo.adjustCalls)
	}
}

// UpdatePost with tagsPresent=true and tags=[] must clear all existing bindings
// (PATCH semantics: empty array = "clear all"). This locks the contract that
// Task 11 handler relies on — a regression where UpdatePost mistakenly adds a
// `len(tags) > 0` guard would silently break the documented "clear all" flow.
func TestPostUC_UpdatePost_TagsPresentEmptyArrayClearsAll(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		// Post has 2 existing bindings; Apply will diff and remove both.
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{
				{PostID: 1, HashtagID: 100, DisplayName: "美食", Position: 0},
				{PostID: 1, HashtagID: 200, DisplayName: "宅配", Position: 1},
			}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			// Visible post (deleted_at + hidden_at both null).
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10, Image: testStringPointer("posts/a.jpg")}, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	hashtagUC := NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 })
	uc := NewPostUseCase(postRepo, hashtagUC, &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	_, err := uc.UpdatePost(context.Background(), 1, 10, "new", "111", "", nil, true /* tagsPresent */, []string{} /* empty = clear */, nil)
	if err != nil {
		t.Fatalf("UpdatePost err = %v", err)
	}
	// Expect both counter -1 calls (visible post → counter delta applies; both
	// bindings removed → 2× -1 in hashtag.id ASC order).
	if len(hashtagRepo.adjustCalls) != 2 {
		t.Fatalf("adjust calls = %d, want 2 (clear of 2 bindings)", len(hashtagRepo.adjustCalls))
	}
	for i, c := range hashtagRepo.adjustCalls {
		want := []int64{100, 200}[i]
		if c.hashtagID != want || c.delta != -1 {
			t.Errorf("[%d] adjust = {id=%d delta=%d}, want {id=%d delta=-1}", i, c.hashtagID, c.delta, want)
		}
	}
}

// ==================== publish_status × hashtag counter（56 §5.1.5）====================
//
// cms_hashtag.post_count 是公開自動完成的唯一過濾條件（SearchHashtagsByPrefix /
// SearchHashtagsByContain 的述詞是 post_count > 0），而 AdjustHashtagPostCount 是裸的
// post_count + delta、沒有 GREATEST(0, …) 底板。因此計數的每一個 ± 都必須與
// 「這則貼文買家看不看得到」逐一對齊，否則偏移不可自癒也無對帳程序。
// 以下四支鎖住「代行草稿（publish_status = 4401）從頭到尾都不計數」這條不變式；
// 上面既有的四支則鎖住「賣家自建貼文（DB DEFAULT 4402）行為逐位元不變」。

// 代行建立的草稿即使 deleted_at / hidden_at 皆為 NULL 也不得 +1 ——
// 否則只被草稿用過的標籤會出現在公開自動完成清單裡。
//
// 可見性判斷讀的是 CreatePost 的輸入參數 publishStatus（sameTx 閉包跑的當下，
// postRepo.Create 尚未回傳，post 變數還不存在），不是回傳列。mock 的回傳刻意
// 設成 Published——與輸入的 Draft 相反：如果日後有人把來源改回讀回傳列，
// Apply 收到的 visible 會變 true，這支測試立刻轉紅。兩邊若填一致，這支測試
// 就分不出程式碼讀的到底是哪一個，等於失去判別力。
func TestPostUC_CreatePost_DraftDoesNotCountHashtags(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, canonical string) (int64, error) {
			return map[string]int64{"美食": 1001, "宅配": 1002}[canonical], nil
		},
	}
	postRepo := &mockPostRepo{
		createFn: func(_ context.Context, id, sellerID int64, _ string, _ string, _ *int64, _ *int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: sellerID}, nil
		},
	}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	if _, err := uc.CreatePost(context.Background(), 10, "hello", "", nil, []string{"美食", "宅配"}, domain.PostPublishStatusDraft); err != nil {
		t.Fatalf("CreatePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %v, want none (draft is not public-visible)", hashtagRepo.adjustCalls)
	}
}

// 草稿改標籤同樣不動計數（Apply 收到 visible=false）。
func TestPostUC_UpdatePost_DraftDoesNotCountHashtags(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{{PostID: 1, HashtagID: 100, DisplayName: "美食", Position: 0}}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusDraft, ID: id, SellerID: 10}, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	if _, err := uc.UpdatePost(context.Background(), 1, 10, "new", "", "", nil, true, []string{}, nil); err != nil {
		t.Fatalf("UpdatePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %v, want none (draft is not public-visible)", hashtagRepo.adjustCalls)
	}
}

// FIX 2（#2772 review round 2）的迴歸測試：visible 必須讀 tx 內重讀（GetByIDTx）而不是
// UpdatePost 進入時 GetByID 讀到的那一列。這裡直接模擬 UpdatePost 的 UPDATE 拿到列鎖之後
// 才看得到的狀態——getByIDFn 回傳 UpdatePost 一開始讀到的 Draft（模擬 PublishScheduledPost
// 尚未 commit 前的快照），getByIDTxFn 回傳 Published（模擬 PublishScheduledPost 在 GetByID
// 與這裡的 UPDATE 之間插進來、已經 commit、但沒有 bump version，故 UpdatePost 自己的 CAS
// 依然贏）。若 Apply 收到的 visible 是從 getByIDFn 那個過期快照算出來的，這裡會斷言 0 次
// adjust 而轉紅；只有真的改讀 GetByIDTx 才會看到 +1。
func TestPostUC_UpdatePost_VisibleReadsTxScopedStateNotStaleGetByID(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return nil, nil // no existing bindings; tags=["美食"] diffs as a pure add
		},
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, _ string) (int64, error) {
			return 100, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			// UpdatePost 進入時讀到的過期快照：草稿。
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusDraft, ID: id, SellerID: 10}, nil
		},
		getByIDTxFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			// UPDATE 拿到列鎖之後才看得到的真相：已被 PublishScheduledPost 轉為已發布
			// （該路徑不 bump version，見 db/queries/post_schedule.sql 的
			// PublishScheduledPost 查詢註解），但 deleted_at／hidden_at 皆未變。
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusPublished, ID: id, SellerID: 10}, nil
		},
		updateFn: func(_ context.Context, _ int64, _ string, _ string, _ *int64, _ *int64) error { return nil },
	}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	if _, err := uc.UpdatePost(context.Background(), 1, 10, "new", "", "", nil, true, []string{"美食"}, nil); err != nil {
		t.Fatalf("UpdatePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 1 || hashtagRepo.adjustCalls[0].delta != 1 {
		t.Fatalf("adjust calls = %v, want exactly one +1 (visible must come from the tx-scoped re-read, not the stale pre-UPDATE GetByID)", hashtagRepo.adjustCalls)
	}
}

// 刪除草稿不得 -1：它從未被 +1 過，減下去會把該標籤的計數拉低甚至為負，
// 讓「有真實已發布貼文在用」的標籤從公開自動完成裡消失。
func TestPostUC_DeletePost_DraftDoesNotDecrementHashtags(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{601, 602}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{PublishStatus: domain.PostPublishStatusDraft, ID: id, SellerID: 10}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) { return 1, nil },
	}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	if err := uc.DeletePost(context.Background(), 1, 10); err != nil {
		t.Fatalf("DeletePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %v, want none (draft was never counted)", hashtagRepo.adjustCalls)
	}
}

// 恢復一則被平台強制下架的草稿不得 +1（RestorePost 不改 publish_status，
// 恢復後它仍是草稿）——否則只被草稿用過的標籤會出現在公開自動完成清單裡。
func TestPostUC_RestorePost_DraftDoesNotIncrementHashtags(t *testing.T) {
	hashtagRepo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{701}, nil
		},
	}
	postRepo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{
				PublishStatus: domain.PostPublishStatusDraft,
				ID:            id,
				SellerID:      10,
				HiddenAt:      testTimePointer(time.Now()),
			}, nil
		},
		restorePostFn: func(_ context.Context, _ int64) error { return nil },
	}
	uc := NewPostUseCase(postRepo, NewHashtagUseCase(hashtagRepo, func() int64 { return 77777 }), &mockReportRepo{}, nil, &mockMediaClient{}, &mockSellerClient{}, &mockBuyerClient{}, &mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, nil)

	if err := uc.RestorePost(context.Background(), 1); err != nil {
		t.Fatalf("RestorePost err = %v", err)
	}
	if len(hashtagRepo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %v, want none (draft was never counted)", hashtagRepo.adjustCalls)
	}
}

// #2757：跨店刪除必須在任何副作用之前就停住。
// 這個測試不是在驗錯誤碼，是在驗「四個副作用一個都沒發生」。
func TestPostUC_DeletePost_WrongOwner_NoSideEffects(t *testing.T) {
	var softDeleteCalls, releaseCalls int
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 10, PublishStatus: domain.PostPublishStatusPublished}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) {
			softDeleteCalls++
			return 1, nil
		},
	}
	media := &mockMediaClient{
		releaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error {
			releaseCalls++
			return nil
		},
	}
	ht := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{501}, nil
		},
	}
	rec := newTagRecorder(t)
	uc := NewPostUseCase(repo, NewHashtagUseCase(ht, func() int64 { return 88888 }),
		&mockReportRepo{}, nil, media, &mockSellerClient{}, &mockBuyerClient{},
		&mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, rec.client())

	err := uc.DeletePost(context.Background(), 777, 99) // seller 99 不擁有 post 777

	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
	if softDeleteCalls != 0 {
		t.Errorf("softDeleteCalls = %d, want 0", softDeleteCalls)
	}
	if releaseCalls != 0 {
		t.Errorf("releaseCalls = %d, want 0 —— 這正是 #2757 的攻擊面：解除他人貼文的圖片引用", releaseCalls)
	}
	if len(ht.adjustCalls) != 0 {
		t.Errorf("hashtag counter mutations = %v, want none", ht.adjustCalls)
	}
	rec.expectNoBust(t)
}

// #2757：affected rows = 0（並發重複刪除、或 read 與 write 之間被別人刪掉）
// 同樣不得產生任何副作用——counter 沒有下限，扣兩次就永久漂移。
func TestPostUC_DeletePost_ZeroRows_NoSideEffects(t *testing.T) {
	var releaseCalls int
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 10, PublishStatus: domain.PostPublishStatusPublished}, nil
		},
		softDeleteFn: func(_ context.Context, _, _ int64) (int64, error) { return 0, nil },
	}
	media := &mockMediaClient{
		releaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error {
			releaseCalls++
			return nil
		},
	}
	ht := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{501}, nil
		},
	}
	rec := newTagRecorder(t)
	uc := NewPostUseCase(repo, NewHashtagUseCase(ht, func() int64 { return 88888 }),
		&mockReportRepo{}, nil, media, &mockSellerClient{}, &mockBuyerClient{},
		&mockProductClient{}, nil, &config.Config{}, func() int64 { return 99999 }, rec.client())

	err := uc.DeletePost(context.Background(), 777, 10)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if releaseCalls != 0 {
		t.Errorf("releaseCalls = %d, want 0", releaseCalls)
	}
	if len(ht.adjustCalls) != 0 {
		t.Errorf("hashtag counter mutations = %v, want none", ht.adjustCalls)
	}
	rec.expectNoBust(t)
}
