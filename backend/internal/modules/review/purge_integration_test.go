//go:build integration

package review_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// requestDeletion 讓買家進入刪除中（buyer 的 BeginDeletion，狀態的唯一來源），再呼叫 review 的訂閱者排清除 job。
func (f *fixture) requestDeletion(t *testing.T, b buyer.Profile) {
	t.Helper()
	if err := f.buyers.BeginDeletion(context.Background(), b.BuyerID); err != nil {
		t.Fatal(err)
	}
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

// RD4：刪帳申請（BeginDeletion）提交的同一刻起，買家的評價不可讀、不計分（公開述詞 join buyer.pub_inactive_buyers()）；列還在，
// 沒有訂閱者也沒有 job。
func TestDeactivationHidesReviewsImmediatelyViaThePubJoin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	gone := f.submit(t, leaving, storeID, 700, 1, "要走的")
	kept := f.submit(t, staying, storeID, 700, 5, "留下的")
	if s := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); s.Count != 2 {
		t.Fatalf("申請前兩則都計：%+v", s)
	}
	f.jobs.reset()

	if err := f.buyers.BeginDeletion(ctx, leaving.BuyerID); err != nil {
		t.Fatal(err)
	}

	page := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{}, nil, 20))
	if !sameIDs(ids(page), kept.ID) {
		t.Fatalf("申請後只剩留下的：%v", ids(page))
	}
	if s := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); s.Count != 1 || s.Average != 5 {
		t.Fatalf("申請後商品統計不計他：%+v", s)
	}
	if s := must[review.StoreStats](t)(f.module.StoreStats(ctx, storeID)); s.Count != 1 || s.Average != 5 {
		t.Fatalf("申請後店家統計不計他：%+v", s)
	}
	if _, err := f.module.GetReview(ctx, gone.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("單筆讀取不給刪帳者的評價：%v", err)
	}
	if n := f.reviewsOf(t, leaving); n != 1 {
		t.Fatalf("可見性不刪列：%d", n)
	}
	if len(f.jobs.kinds()) != 0 {
		t.Fatalf("沒有跑訂閱者就沒有 job：%v", f.jobs.kinds())
	}
	f.requestDeletion(t, leaving)
	if f.jobs.count("review.purge_author") != 1 {
		t.Fatalf("訂閱者排一個清除 job：%v", f.jobs.kinds())
	}
}

// 兩個事件都訂：先申請後關閉，或直接關閉，訂閱者都只排清除 job（River 以買家合併），不寫任何複本。
func TestSubscribersEnqueueOnePurgeJob(t *testing.T) {
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
	direct := f.newBuyer(t)
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: direct.BuyerID, AccountID: direct.AccountID, OccurredAt: f.clock()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.jobs.count("review.purge_author") != 3 {
		t.Fatalf("每次訂閱都排清除 job（River 以買家合併）：%v", f.jobs.kinds())
	}
	if n := f.scalar(t, "SELECT count(*) FROM pg_tables WHERE schemaname = 'review' AND tablename = 'inactive_authors'"); n != 0 {
		t.Fatalf("複本表已刪除：%d", n)
	}
}

// RD4：purge job 刪列與圖片列、把附圖標為刪除並排快取失效；其他買家不動；重跑結果相同。
func TestRD4PurgeDeletesRowsAndImagesAndMarksTheAssetsDeletedAndIsRerunnable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	submit := func(b buyer.Profile, product int, tokens ...string) review.Review {
		it := f.order(b, storeID, product)
		return must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 4, StagedTokens: tokens}))
	}
	leavingTokens, leavingAssets := f.stage(t, leaving, 2)
	stayingTokens, stayingAssets := f.stage(t, staying, 1)
	withImages := submit(leaving, 700, leavingTokens...)
	withoutImages := submit(leaving, 701)
	other := submit(staying, 700, stayingTokens...)
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
	if n := f.reviewsOf(t, staying); n != 1 || !sameIDs(ids(must[review.Page](t)(f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{}, nil, 20))), other.ID) {
		t.Fatalf("別人的評價不動：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.review_images WHERE review_id = $1", other.ID); n != 1 {
		t.Fatalf("別人的附圖不動：%d", n)
	}
	for _, a := range leavingAssets {
		if got := f.assetState(t, a); got != "DELETING" {
			t.Fatalf("刪除評價的附圖 state=%s，預期 DELETING", got)
		}
	}
	if got := f.assetState(t, stayingAssets[0]); got != "ACTIVE" {
		t.Fatalf("別人的附圖 state=%s，預期 ACTIVE", got)
	}
	tags := revalidations(f.jobs)
	if len(tags) != 1 || !slices.Contains(tags[0], revalidate.ProductTag(pid(700))) || !slices.Contains(tags[0], revalidate.ProductTag(pid(701))) {
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
		SELECT gen_random_uuid(), 'N' || n, 100000 + n, $1, $2, lpad(to_hex(700 + n % 3), 32, '0')::uuid, 5, 'sku', 1, $3
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

// 保底掃描：在 buyer.pub_inactive_buyers() 又還有評價列、而且沒有排隊中的清除 job 的買家補排一個；沒有評價列的買家與還在營運的買家都不排。
func TestSweepPurgeEnqueuesOnlyInactiveBuyersWithReviewsLeft(t *testing.T) {
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

// 保底掃描分頁走完全部：超過一頁（100）時最後面的買家也會被補排，前面的買家一直失敗不會讓它餓死。
func TestSweepPurgeReachesBuyersBeyondOneBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const buyers = 230
	last := int64(100000 + buyers - 1)
	storeID := f.exposableStore(t)
	f.exec(t, `INSERT INTO buyer.buyers (buyer_id, account_id, status, display_name, created_at, updated_at, deleted_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000000 + g, 'CLOSED', 'b', now(), now(), now() FROM generate_series(100000, $1::bigint) AS g`, last)
	f.exec(t, `INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, sku_name, price, created_at)
		SELECT gen_random_uuid(), 'N' || g, g, g, $2, lpad(to_hex(700), 32, '0')::uuid, 5, 'sku', 1, $3 FROM generate_series(100000, $1::bigint) AS g`, last, storeID, f.clock())
	f.jobs.reset()
	n, err := f.module.SweepPurge(ctx)
	if err != nil || n != buyers {
		t.Fatalf("掃到 %d 位買家 err=%v，預期 %d", n, err, buyers)
	}
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	seen := map[int64]int{}
	for _, a := range f.jobs.args {
		var v struct {
			BuyerID int64 `json:"buyer_id"`
		}
		b, _ := json.Marshal(a)
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		seen[v.BuyerID]++
	}
	if len(seen) != buyers || seen[last] != 1 || seen[100000] != 1 {
		t.Fatalf("補排了 %d 位不同的買家，最後一位 %d 次、第一位 %d 次", len(seen), seen[last], seen[100000])
	}
}

// 刪除時每批依商品 ID 升序同步 catalog 評分；做完之後重跑不再呼叫。
func TestPurgeSyncsProductRatingPerProductPerBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s1, s2 := f.exposableStore(t), f.exposableStore(t)
	b := f.newBuyer(t)
	f.exec(t, `INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, sku_name, price, created_at)
		SELECT gen_random_uuid(), 'N' || n, 100000 + n, $1, CASE WHEN n % 2 = 0 THEN $2::bigint ELSE $3::bigint END, lpad(to_hex(700 + n % 2), 32, '0')::uuid, 5, 'sku', 1, $4
		FROM generate_series(1, 250) AS n`, b.BuyerID, s1, s2, f.clock())
	f.requestDeletion(t, b)
	f.ratings.mu.Lock()
	f.ratings.calls = nil
	f.ratings.mu.Unlock()

	if err := f.module.PurgeAuthor(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	f.ratings.mu.Lock()
	got := slices.Clone(f.ratings.calls)
	f.ratings.mu.Unlock()
	count := map[uuid.UUID]int{}
	for _, c := range got {
		count[c.productID]++
		if c.average != 0 || c.count != 0 {
			t.Fatalf("刪帳後沒有公開評價，應傳 (0, 0)：%+v", c)
		}
	}
	want := map[uuid.UUID]int{pid(700): 2, pid(701): 2}
	if len(got) != 4 || len(count) != len(want) {
		t.Fatalf("2 批 x 2 商品 = 4 次 SetRating：%+v", got)
	}
	for k, n := range want {
		if count[k] != n {
			t.Fatalf("%s 應呼叫 %d 次：%v", k, n, count)
		}
	}

	if err := f.module.PurgeAuthor(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	f.ratings.mu.Lock()
	defer f.ratings.mu.Unlock()
	if len(f.ratings.calls) != len(got) {
		t.Fatalf("重跑不再呼叫：%d -> %d", len(got), len(f.ratings.calls))
	}
}

// F1：Submit 在 transaction 內第一句 buyer.LockActive，與買家轉移的列鎖互斥。BeginDeletion 的 transaction 未提交時 Submit 在買家列上等；
// 提交後 Submit 要被擋下，不能留下刪帳者的評價（清除 job 找不到列就結束，之後不會再有人刪）。
func TestSubmitIsRejectedAfterDeletionRequestAndWaitsForTheGuard(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)

	held, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- f.db.InTx(ctx, func(ctx context.Context) error {
			if err := f.buyers.BeginDeletion(ctx, b.BuyerID); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	select {
	case <-held:
	case err := <-finished:
		t.Fatalf("BeginDeletion 的 transaction 沒有持住：%v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, Content: "晚到的"})
		done <- err
	}()

	deadline := time.Now().Add(10 * time.Second)
	for f.lockWaiters(t) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Submit 應該在買家列的守門鎖上等，卻已回傳：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Submit 沒有等在守門鎖上")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}

	if err := <-done; kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("轉移提交後要回 FailedPrecondition：%v", err)
	}
	if n := f.reviewsOf(t, b); n != 0 {
		t.Fatalf("刪帳者不能留下評價：%d", n)
	}
}

// lockWaiters 數目前等在鎖上的連線（每個整合測試有自己的資料庫，不受平行測試影響）。
func (f *fixture) lockWaiters(t *testing.T) int64 {
	t.Helper()
	return f.scalar(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'")
}
