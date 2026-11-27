//go:build integration

package review_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// runRatings 跑一輪評分投影（週期 job 的內容）。
func (f *fixture) runRatings(t *testing.T) {
	t.Helper()
	if err := f.module.RunRatingProjection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type queueRow struct {
	seq      int64
	attempts int32
	claimed  bool
	failed   bool
	dueAt    time.Time
	priority int16
	token    uuid.UUID
}

func (f *fixture) queueRow(t *testing.T, productID uuid.UUID) (queueRow, bool) {
	t.Helper()
	var r queueRow
	err := f.db.Conn(context.Background()).QueryRow(context.Background(),
		`SELECT seq, attempts, claimed_until IS NOT NULL, failed_at IS NOT NULL, due_at, priority, COALESCE(claim_token, uuid_nil()) FROM review.rating_recompute_queue WHERE product_id = $1`, productID,
	).Scan(&r.seq, &r.attempts, &r.claimed, &r.failed, &r.dueAt, &r.priority, &r.token)
	if errors.Is(err, pgx.ErrNoRows) {
		return queueRow{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return r, true
}

// 提交同 transaction 登記待重算（每商品一列、序號遞增），不碰 catalog；worker 跑過才寫入絕對值並清掉列。
func TestSubmitRequestsRatingRecomputeAndWorkerProjectsIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	f.submit(t, f.newBuyer(t), storeID, 700, 5, "好")
	if row, ok := f.queueRow(t, pid(700)); !ok || row.seq != 1 || f.ratings.callCount() != 0 {
		t.Fatalf("提交只登記：%+v ok=%v calls=%d", row, ok, f.ratings.callCount())
	}
	f.submit(t, f.newBuyer(t), storeID, 700, 2, "差")
	if row, _ := f.queueRow(t, pid(700)); row.seq != 2 {
		t.Fatalf("同商品第二次登記序號加一：%+v", row)
	}

	f.runRatings(t)
	if got := f.ratings.snapshot(); len(got) != 1 || got[0].productID != pid(700) || got[0].average != 3.5 || got[0].count != 2 {
		t.Fatalf("worker 寫絕對值：%+v", got)
	}
	if _, ok := f.queueRow(t, pid(700)); ok {
		t.Fatal("處理完刪列")
	}
	f.runRatings(t)
	if f.ratings.callCount() != 1 {
		t.Fatal("沒有列就沒有呼叫")
	}
}

// 商品在 catalog 不存在：評價照常成功；worker 記錄後刪列，不進重試。
func TestReviewSucceedsWhenProductIsMissingAndWorkerDropsTheRow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	f.ratings.missing[pid(404)] = true
	r := f.submit(t, f.newBuyer(t), storeID, 404, 4, "商品不在 catalog")
	if _, ok := f.queueRow(t, pid(404)); !ok {
		t.Fatal("提交登記待重算")
	}
	f.runRatings(t)
	if _, ok := f.queueRow(t, pid(404)); ok || f.ratings.callCount() != 0 {
		t.Fatalf("商品不存在：刪列、不寫入、不重試 calls=%d", f.ratings.callCount())
	}
	if st := must[review.ProductStats](t)(f.module.ProductStats(context.Background(), pid(404))); st.Count != 1 || r.ID == (uuid.UUID{}) {
		t.Fatalf("評價保留：%+v", st)
	}
}

// 處理中到的新請求不遺失：SetRating 執行當下另一個 transaction 提交了新評價（序號加一），worker 完成時序號已變，不刪列而重排；
// 下一輪才寫入含新評價的值。
func TestRequestArrivingWhileProcessingIsNotLost(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	f.submit(t, f.newBuyer(t), storeID, 700, 5, "第一則")
	b2 := f.newBuyer(t)
	f.ratings.onSet = func() { f.submit(t, b2, storeID, 700, 1, "處理中到的") }

	f.runRatings(t)
	first := f.ratings.snapshot()
	if len(first) != 1 || first[0].count != 1 || first[0].average != 5 {
		t.Fatalf("第一輪寫的是認領當下讀到的統計：%+v", first)
	}
	row, ok := f.queueRow(t, pid(700))
	if !ok || row.seq != 2 || row.claimed {
		t.Fatalf("序號已變：列要保留並放掉租約：%+v ok=%v", row, ok)
	}

	f.runRatings(t)
	if got := f.ratings.snapshot(); len(got) != 2 || got[1].count != 2 || got[1].average != 3 {
		t.Fatalf("第二輪寫入含新評價的值：%+v", got)
	}
	if _, ok := f.queueRow(t, pid(700)); ok {
		t.Fatal("序號沒變才刪列")
	}
}

// 清快取在 SetRating 之後（同一個 transaction，commit 後才可見）：先 lock、再 set、最後 revalidate；
// 商品頁與店家頁（商品卡顯示評分與則數）的 tag 都要清。
func TestProductCacheIsBustedAfterSetRating(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	f.submit(t, f.newBuyer(t), storeID, 700, 4, "好")
	f.trace.reset()
	f.jobs.reset()

	f.runRatings(t)
	if got, want := f.trace.snapshot(), []string{"lock", "set", "revalidate"}; !slices.Equal(got, want) {
		t.Fatalf("順序 %v，要 %v", got, want)
	}
	tags := revalidations(f.jobs)
	st := must[seller.Store](t)(f.sellers.Store(context.Background(), storeID))
	want := append([]string{"product:" + pid(700).String()}, revalidate.StoreTags(st.StoreID, st.StoreCode)...)
	if len(tags) != 1 || !slices.Equal(tags[0], want) {
		t.Fatalf("worker 清商品頁與店家頁的 tag：%v，要 %v", tags, want)
	}
}

// 統計與 catalog 現值相同就不寫入、不清快取（全量對帳不會製造寫入與失效）。
func TestUnchangedRatingIsNotWrittenNorBusted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
	f.ratings.setCurrent(pid(700), 4, 1)
	f.jobs.reset()
	f.runRatings(t)
	if f.ratings.callCount() != 0 || f.jobs.count("revalidate") != 0 {
		t.Fatalf("沒有變就不動：calls=%d jobs=%v", f.ratings.callCount(), f.jobs.kinds())
	}
	if _, ok := f.queueRow(t, pid(700)); ok {
		t.Fatal("仍要刪列")
	}
}

// 買家停用不經評價寫入：統計變了卻沒有待重算列；全量對帳登記有評價的商品，worker 才補正。
func TestReconcileFixesDriftFromDeactivatedBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	f.submit(t, leaving, storeID, 700, 1, "會離開")
	f.submit(t, staying, storeID, 700, 5, "留下")
	f.runRatings(t)
	if got := f.ratings.snapshot(); len(got) != 1 || got[0].count != 2 {
		t.Fatalf("起點 %+v", got)
	}

	f.requestDeletion(t, leaving)
	f.runRatings(t)
	if f.ratings.callCount() != 1 {
		t.Fatal("停用不經評價寫入，沒有待重算列，worker 不會動")
	}

	n, err := f.module.ReconcileRatings(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("對帳登記有評價的商品：n=%d err=%v", n, err)
	}
	f.runRatings(t)
	if got := f.ratings.snapshot(); len(got) != 2 || got[1].count != 1 || got[1].average != 5 {
		t.Fatalf("補正成只算留下的買家：%+v", got)
	}
}

// 失敗在同列累計次數、指數退避、放掉租約；退避期內不重跑，到期後才重跑；成功後刪列。
func TestFailureBacksOffOnTheSameRow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
	f.ratings.err = errors.New("catalog 暫時不可用")

	f.runRatings(t)
	row, ok := f.queueRow(t, pid(700))
	if !ok || row.attempts != 1 || row.claimed || row.failed || !row.dueAt.After(f.clock()) {
		t.Fatalf("第一次失敗：%+v ok=%v", row, ok)
	}
	f.runRatings(t)
	if row, _ := f.queueRow(t, pid(700)); row.attempts != 1 {
		t.Fatalf("退避期內不重跑：%+v", row)
	}
	f.advance(2 * time.Minute)
	f.runRatings(t)
	if row, _ := f.queueRow(t, pid(700)); row.attempts != 2 {
		t.Fatalf("到期後重跑再失敗：%+v", row)
	}

	f.ratings.mu.Lock()
	f.ratings.err = nil
	f.ratings.mu.Unlock()
	f.advance(3 * time.Minute)
	f.runRatings(t)
	if _, ok := f.queueRow(t, pid(700)); ok || f.ratings.callCount() != 1 {
		t.Fatal("恢復後成功並刪列")
	}
}

// 即時請求排在對帳登記的大量待辦之前（即使對帳的列到期得更早），而且一輪 Run 反覆認領到沒有到期列為止，不只一批。
func TestLiveRequestIsClaimedBeforeReconcileBacklogAndRunDrainsAllBatches(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
	const backlog = 250
	ids := make([]uuid.UUID, backlog)
	for i := range ids {
		ids[i] = pid(2000 + i)
	}
	f.exec(t, `INSERT INTO review.rating_recompute_queue (product_id, due_at, priority) SELECT unnest($1::uuid[]), $2, 1`, ids, f.clock().Add(-time.Hour))
	if row, _ := f.queueRow(t, pid(700)); row.priority != 0 {
		t.Fatalf("即時請求的 priority 是 0：%+v", row)
	}

	f.runRatings(t)
	order := f.ratings.lockOrder()
	if len(order) != backlog+1 || order[0] != pid(700) {
		t.Fatalf("即時請求先處理，且一輪清完所有批次：first=%v n=%d", order[0], len(order))
	}
	if n := f.scalar(t, `SELECT count(*) FROM review.rating_recompute_queue`); n != 0 {
		t.Fatalf("一輪 Run 要清到沒有到期列：剩 %d", n)
	}
}

// 對帳不重置已停手的列（只有即時請求能重置）：永遠失敗的商品不會每小時被對帳救活重試。
func TestReconcileDoesNotResetStoppedRowButLiveRequestDoes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	f.submit(t, f.newBuyer(t), storeID, 700, 4, "好")
	f.exec(t, `UPDATE review.rating_recompute_queue SET seq = 3, attempts = 5, failed_at = now(), last_error = 'x', claimed_until = NULL, claim_token = NULL WHERE product_id = $1`, pid(700))

	if n, err := f.module.ReconcileRatings(context.Background()); err != nil || n != 1 {
		t.Fatalf("對帳照常挑到這個商品：n=%d err=%v", n, err)
	}
	if row, _ := f.queueRow(t, pid(700)); !row.failed || row.attempts != 5 || row.seq != 3 {
		t.Fatalf("對帳不動已停手的列：%+v", row)
	}
	f.runRatings(t)
	if f.ratings.callCount() != 0 {
		t.Fatal("已停手的列不被認領")
	}

	f.submit(t, f.newBuyer(t), storeID, 700, 2, "新的")
	if row, _ := f.queueRow(t, pid(700)); row.failed || row.attempts != 0 || row.seq != 4 || row.priority != 0 {
		t.Fatalf("即時請求重置停手的列：%+v", row)
	}
}

// ctx 在處理中取消（job 逾時）：失敗仍記在列上（脫離取消的 ctx），並放掉租約。
func TestFailureIsRecordedEvenWhenContextIsCanceled(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.ratings.err = context.Canceled
	f.ratings.onLock = cancel

	if err := f.module.RunRatingProjection(ctx); err != nil {
		t.Fatal(err)
	}
	if row, ok := f.queueRow(t, pid(700)); !ok || row.attempts != 1 || row.claimed || row.failed {
		t.Fatalf("取消後的失敗要記下並放掉租約：%+v ok=%v", row, ok)
	}
}

// 租約在處理中被別人接手（claim_token 換了）：完成不刪列、失敗不計次數，那一列留給新的擁有者。
func TestStaleLeaseOwnerCannotFinishOrFailTheRow(t *testing.T) {
	t.Parallel()
	takeover := func(f *fixture, t *testing.T) uuid.UUID {
		tok := uuid.NewV7()
		f.exec(t, `UPDATE review.rating_recompute_queue SET claim_token = $1 WHERE product_id = $2`, tok, pid(700))
		return tok
	}
	t.Run("finish", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
		var tok uuid.UUID
		f.ratings.onSet = func() { tok = takeover(f, t) }
		f.runRatings(t)
		if row, ok := f.queueRow(t, pid(700)); !ok || !row.claimed || row.token != tok || row.seq != 1 {
			t.Fatalf("接手後舊 worker 不能刪列或放掉租約：%+v ok=%v", row, ok)
		}
	})
	t.Run("fail", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		f.submit(t, f.newBuyer(t), f.exposableStore(t), 700, 4, "好")
		var tok uuid.UUID
		f.ratings.err = errors.New("catalog 暫時不可用")
		f.ratings.onLock = func() { tok = takeover(f, t) }
		f.runRatings(t)
		if row, ok := f.queueRow(t, pid(700)); !ok || !row.claimed || row.token != tok || row.attempts != 0 {
			t.Fatalf("接手後舊 worker 不能記失敗：%+v ok=%v", row, ok)
		}
	})
}
