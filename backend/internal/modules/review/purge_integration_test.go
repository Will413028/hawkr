//go:build integration

package review_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

func (f *fixture) requestDeletion(t *testing.T, b buyer.Profile) {
	t.Helper()
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		return f.module.OnBuyerDeletionRequested(ctx, buyer.BuyerDeletionRequested{BuyerID: b.BuyerID, AccountID: b.AccountID, OccurredAt: f.clock()})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) reviewsOf(t *testing.T, b buyer.Profile) int64 {
	t.Helper()
	return f.scalar(t, "SELECT count(*) FROM review.reviews WHERE buyer_id = $1", b.BuyerID)
}

// RD4：刪帳申請的那個 transaction 起，買家的評價不可讀、不計分；訂閱者只寫複本與排 job，不刪列。
func TestRD4DeletionRequestUncountsTheReviewsWithoutDeletingRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	gone := f.submit(t, leaving, storeID, 700, 1, "要走的")
	kept := f.submit(t, staying, storeID, 700, 5, "留下的")
	if s := must[review.ProductStats](t)(f.module.ProductStats(ctx, 700)); s.Count != 2 {
		t.Fatalf("申請前兩則都計：%+v", s)
	}
	f.jobs.reset()

	f.requestDeletion(t, leaving)

	page := must[review.Page](t)(f.module.ListProductReviews(ctx, 700, 0, review.Filter{}, nil, 20))
	if !sameIDs(ids(page), kept.ID) {
		t.Fatalf("申請後只剩留下的：%v", ids(page))
	}
	if s := must[review.ProductStats](t)(f.module.ProductStats(ctx, 700)); s.Count != 1 || s.Average != 5 {
		t.Fatalf("申請後商品統計不計他：%+v", s)
	}
	if s := must[review.StoreStats](t)(f.module.StoreStats(ctx, storeID)); s.Count != 1 || s.Average != 5 {
		t.Fatalf("申請後店家統計不計他：%+v", s)
	}
	if _, err := f.module.GetReview(ctx, gone.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("單筆讀取不給刪帳者的評價：%v", err)
	}
	if n := f.reviewsOf(t, leaving); n != 1 {
		t.Fatalf("訂閱者不刪列：%d", n)
	}
	if f.jobs.count("review.purge_author") != 1 {
		t.Fatalf("排一個清除 job：%v", f.jobs.kinds())
	}
	if len(f.deactivated) != 1 || f.deactivated[0].BuyerID != leaving.BuyerID {
		t.Fatalf("發一個 ReviewAuthorDeactivated：%+v", f.deactivated)
	}
}

// 兩個事件都訂：先申請後關閉，或直接關閉，複本只插入一次、事件只發一次。
func TestDeactivationSubscribersInsertOnceAndPublishOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b := f.newBuyer(t)
	f.requestDeletion(t, b)
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: b.BuyerID, AccountID: b.AccountID, OccurredAt: f.clock()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.inactive_authors WHERE buyer_id = $1", b.BuyerID); n != 1 {
		t.Fatalf("複本一列：%d", n)
	}
	if len(f.deactivated) != 1 {
		t.Fatalf("事件只發一次：%+v", f.deactivated)
	}

	direct := f.newBuyer(t)
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: direct.BuyerID, AccountID: direct.AccountID, OccurredAt: f.clock()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.deactivated) != 2 || f.deactivated[1].BuyerID != direct.BuyerID {
		t.Fatalf("直接關閉也發：%+v", f.deactivated)
	}
	if f.jobs.count("review.purge_author") != 3 {
		t.Fatalf("每次訂閱都排清除 job（River 以買家合併）：%v", f.jobs.kinds())
	}
}

// RD4：purge job 刪列與圖片列、記退場資產並排媒體同步與快取失效；其他買家不動；重跑結果相同。
func TestRD4PurgeDeletesRowsAndImagesAndSyncsMediaAndIsRerunnable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	submit := func(b buyer.Profile, product int64, tokens ...string) review.Review {
		it := f.order(b, storeID, product)
		return must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 4, StagedTokens: tokens}))
	}
	withImages := submit(leaving, 700, "p1", "p2")
	withoutImages := submit(leaving, 701)
	other := submit(staying, 700, "q1")
	f.requestDeletion(t, leaving)
	f.jobs.reset()

	if err := f.module.PurgeAuthor(ctx, leaving.BuyerID); err != nil {
		t.Fatal(err)
	}
	if n := f.reviewsOf(t, leaving); n != 0 {
		t.Fatalf("評價要刪光：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.review_images WHERE review_id IN ($1, $2)", withImages.ID, withoutImages.ID); n != 0 {
		t.Fatalf("附圖列隨評價刪除：%d", n)
	}
	if n := f.reviewsOf(t, staying); n != 1 || !sameIDs(ids(must[review.Page](t)(f.module.ListProductReviews(ctx, 700, 0, review.Filter{}, nil, 20))), other.ID) {
		t.Fatalf("別人的評價不動：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.review_images WHERE review_id = $1", other.ID); n != 1 {
		t.Fatalf("別人的附圖不動：%d", n)
	}
	for _, token := range []string{"p1", "p2"} {
		if n := f.scalar(t, "SELECT count(*) FROM review.retired_assets WHERE ref_type = $1 AND ref_id = $2 AND asset_id = $3", media.RefTypeOrderReview, withImages.ID.String(), media.AssetFor(token)); n != 1 {
			t.Fatalf("附圖 %s 記成退場：%d", token, n)
		}
	}
	if f.jobs.count("review.media_sync") != 1 {
		t.Fatalf("有圖的那一則排一個媒體同步（沒圖的不排）：%v", f.jobs.kinds())
	}
	tags := revalidations(f.jobs)
	if len(tags) != 1 || !slices.Contains(tags[0], revalidate.ProductTag(700)) || !slices.Contains(tags[0], revalidate.ProductTag(701)) {
		t.Fatalf("快取失效帶兩個商品的 tag：%v", tags)
	}

	f.jobs.reset()
	if err := f.module.PurgeAuthor(ctx, leaving.BuyerID); err != nil {
		t.Fatal(err)
	}
	if n := f.reviewsOf(t, leaving); n != 0 || len(f.jobs.kinds()) != 0 {
		t.Fatalf("重跑沒有新的寫入與 job：rows=%d jobs=%v", n, f.jobs.kinds())
	}
}

// purge 分批：超過一批的評價全部刪完（一批 200）。
func TestPurgeLoopsOverBatches(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	f.exec(t, `INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, sku_name, price, created_at)
		SELECT gen_random_uuid(), 'N' || n, 100000 + n, $1, $2, 700 + n % 3, 5, 'sku', 1, $3
		FROM generate_series(1, 405) AS n`, b.BuyerID, storeID, f.clock())
	f.requestDeletion(t, b)
	if err := f.module.PurgeAuthor(context.Background(), b.BuyerID); err != nil {
		t.Fatal(err)
	}
	if n := f.reviewsOf(t, b); n != 0 {
		t.Fatalf("405 則要全刪：%d", n)
	}
	tags := revalidations(f.jobs)
	if len(tags) != 3 {
		t.Fatalf("每批一個快取失效 job（405 則 = 3 批）：%d", len(tags))
	}
}

// AuthorTargets 列出買家評過的 (店, 商品)（去重、含隱藏的）；清除後是空的。
func TestAuthorTargetsListsDistinctPairsUntilPurged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s1, s2 := f.exposableStore(t), f.exposableStore(t)
	b := f.newBuyer(t)
	f.submit(t, b, s1, 700, 5, "")
	f.submit(t, b, s1, 700, 4, "")
	hidden := f.submit(t, b, s2, 800, 3, "")
	if _, err := f.module.Hide(ctx, review.HideInput{ReviewID: hidden.ID, Category: "SPAM", Text: "x", AdminID: 1}); err != nil {
		t.Fatal(err)
	}
	got := must[[]review.Target](t)(f.module.AuthorTargets(ctx, b.BuyerID))
	want := []review.Target{{StoreID: s1, ProductID: 700}, {StoreID: s2, ProductID: 800}}
	if !slices.Equal(got, want) {
		t.Fatalf("targets = %v，預期 %v", got, want)
	}
	f.requestDeletion(t, b)
	if err := f.module.PurgeAuthor(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	if got := must[[]review.Target](t)(f.module.AuthorTargets(ctx, b.BuyerID)); len(got) != 0 {
		t.Fatalf("清除後沒有：%v", got)
	}
}
