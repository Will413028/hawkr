//go:build integration

package review_test

import (
	"context"
	"slices"
	"testing"
	"time"

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
}

// 兩個事件都訂：先申請後關閉，或直接關閉，複本只插入一次。
func TestDeactivationSubscribersInsertOnce(t *testing.T) {
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

	direct := f.newBuyer(t)
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: direct.BuyerID, AccountID: direct.AccountID, OccurredAt: f.clock()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.inactive_authors WHERE buyer_id = $1", direct.BuyerID); n != 1 {
		t.Fatalf("直接關閉也寫複本：%d", n)
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

// 保底掃描：在 inactive_authors 又還有評價列、而且沒有排隊中的清除 job 的買家補排一個；沒有評價列的買家與還在營運的買家都不排。
func TestSweepPurgeEnqueuesOnlyInactiveAuthorsWithReviewsLeft(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	stuck, drained, active := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	f.submit(t, stuck, storeID, 700, 5, "")
	f.submit(t, active, storeID, 700, 4, "")
	f.requestDeletion(t, stuck)
	f.requestDeletion(t, drained)
	f.jobs.reset()

	n, err := f.module.SweepPurge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || f.jobs.count("review.purge_author") != 1 {
		t.Fatalf("只補排有評價列的刪帳者：n=%d %v", n, f.jobs.kinds())
	}

	f.jobs.reset()
	if err := f.module.PurgeAuthor(ctx, stuck.BuyerID); err != nil {
		t.Fatal(err)
	}
	f.jobs.reset()
	n, err = f.module.SweepPurge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || f.jobs.count("review.purge_author") != 0 {
		t.Fatalf("清除完成後不再排：n=%d %v", n, f.jobs.kinds())
	}
}

// A6-D12：每批刪除在同 transaction 對該批每個不同的 (店, 商品) 發一個 ReviewAggregateChanged；做完之後重跑不再發。
func TestPurgePublishesOneAggregateChangePerPairPerBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s1, s2 := f.exposableStore(t), f.exposableStore(t)
	b := f.newBuyer(t)
	f.exec(t, `INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, sku_name, price, created_at)
		SELECT gen_random_uuid(), 'N' || n, 100000 + n, $1, CASE WHEN n % 2 = 0 THEN $2::bigint ELSE $3::bigint END, 700 + n % 2, 5, 'sku', 1, $4
		FROM generate_series(1, 250) AS n`, b.BuyerID, s1, s2, f.clock())
	f.requestDeletion(t, b)
	f.mu.Lock()
	f.changed = nil
	f.mu.Unlock()

	if err := f.module.PurgeAuthor(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	got := slices.Clone(f.changed)
	f.mu.Unlock()
	count := map[review.Target]int{}
	for _, e := range got {
		count[review.Target{StoreID: e.StoreID, ProductID: e.ProductID}]++
	}
	want := map[review.Target]int{{StoreID: s1, ProductID: 700}: 2, {StoreID: s2, ProductID: 701}: 2}
	if len(got) != 4 || len(count) != len(want) {
		t.Fatalf("2 批 x 2 組 = 4 個事件：%+v", got)
	}
	for k, n := range want {
		if count[k] != n {
			t.Fatalf("%v 應發 %d 次：%v", k, n, count)
		}
	}

	if err := f.module.PurgeAuthor(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.changed) != len(got) {
		t.Fatalf("重跑不再發：%d -> %d", len(got), len(f.changed))
	}
}

// F1：Submit 與刪帳的 Deactivate 以作者鎖串行。Deactivate 的 transaction 持鎖時 Submit 在鎖上等；Deactivate 提交後 Submit 要被擋下，
// 不能留下刪帳者的評價（清除 job 找不到列就結束，之後不會再有人刪）。
func TestSubmitWaitsForDeactivationAndIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)

	lockSQL := "SELECT pg_advisory_xact_lock(hashtextextended('review.author:' || $1::text, 0))"
	tx, err := f.db.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, lockSQL, b.BuyerID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, Content: "晚到的"})
		done <- err
	}()

	deadline := time.Now().Add(10 * time.Second)
	for f.authorLockWaiters(t, b.BuyerID) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Submit 應該在作者鎖上等，卻已回傳：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Submit 沒有等在作者鎖上")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO review.inactive_authors (buyer_id, since) VALUES ($1, $2)", b.BuyerID, f.clock()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := <-done; kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("鎖釋放後要回 FailedPrecondition：%v", err)
	}
	if n := f.reviewsOf(t, b); n != 0 {
		t.Fatalf("刪帳者不能留下評價：%d", n)
	}
}

// authorLockWaiters 數等在這位作者的 advisory lock 上的 session（只看這個 key，不受平行測試影響）：
// bigint key 的高 32 位在 classid、低 32 位在 objid，objsubid = 1。
func (f *fixture) authorLockWaiters(t *testing.T, buyerID int64) int64 {
	t.Helper()
	return f.scalar(t, `
SELECT count(*) FROM pg_locks l,
  LATERAL (SELECT hashtextextended('review.author:' || $1::bigint::text, 0) AS k) h
WHERE l.locktype = 'advisory' AND NOT l.granted AND l.objsubid = 1
  AND l.classid::bigint = (h.k >> 32) & 4294967295 AND l.objid::bigint = h.k & 4294967295`, buyerID)
}

// F1 的另一側：Deactivate 也要持作者鎖。測試持有同一把鎖（等同 Submit 的 transaction 持鎖中），真正的 OnBuyerDeleted 在另一個
// transaction 必須等在鎖上、不能先寫 inactive_authors；鎖釋放後才完成。拿掉 Deactivate 的 LockAuthor 時它不會等，這個測試失敗；
// 拿掉 Submit 的 LockAuthor 則由 TestSubmitWaitsForDeactivationAndIsRejected 失敗。
func TestDeactivateWaitsForAuthorLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b := f.newBuyer(t)

	tx, err := f.db.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('review.author:' || $1::text, 0))", b.BuyerID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- f.db.InTx(ctx, func(ctx context.Context) error {
			return f.module.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: b.BuyerID, AccountID: b.AccountID, OccurredAt: f.clock()})
		})
	}()

	deadline := time.Now().Add(10 * time.Second)
	for f.authorLockWaiters(t, b.BuyerID) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Deactivate 應該在作者鎖上等，卻已回傳：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Deactivate 沒有等在作者鎖上")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.inactive_authors WHERE buyer_id = $1", b.BuyerID); n != 0 {
		t.Fatalf("鎖釋放前不能已寫入 inactive_authors：%d", n)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.inactive_authors WHERE buyer_id = $1", b.BuyerID); n != 1 {
		t.Fatalf("鎖釋放後 Deactivate 完成：%d", n)
	}
}
