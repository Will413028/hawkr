//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// fakeProductStores 是商品→店家 port 的替身：stores 沒有的商品視為不存在，err 非 nil 時整批失敗；記下每次呼叫的商品 ID。
type fakeProductStores struct {
	mu     sync.Mutex
	stores map[int64]int64
	err    error
	calls  [][]int64
}

func (p *fakeProductStores) StoreIDs(_ context.Context, ids []int64) (map[int64]int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, slices.Clone(ids))
	if p.err != nil {
		return nil, p.err
	}
	out := map[int64]int64{}
	for _, id := range ids {
		if storeID, ok := p.stores[id]; ok {
			out[id] = storeID
		}
	}
	return out, nil
}

func (p *fakeProductStores) set(productID, storeID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stores == nil {
		p.stores = map[int64]int64{}
	}
	p.stores[productID] = storeID
}

func (p *fakeProductStores) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// viewRow 讀一家店一個台北日的彙總；沒有列回 0。
func (f *fixture) viewRow(t *testing.T, storeID int64, day string) int64 {
	t.Helper()
	return f.count(t, "SELECT COALESCE(SUM(views), 0) FROM seller.product_views_daily WHERE store_id = $1 AND day = $2::date", storeID, day)
}

// pendingEvents 是還沒處理的事件數。
func (f *fixture) pendingEvents(t *testing.T) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE processed_at IS NULL")
}

// view 在 at 記一次這家店的商品瀏覽並立刻彙總（儀表板的統計測試用）。
func (f *fixture) view(t *testing.T, storeID int64, at time.Time) {
	t.Helper()
	f.now = at
	f.products.set(900000+storeID, storeID)
	must(t, f.module.RecordProductView(context.Background(), 900000+storeID))
	n, err := f.module.RollupProductViews(context.Background())
	must(t, err)
	if n != 1 {
		t.Fatalf("rollup processed %d events, want 1", n)
	}
}

// buckets 是全部每日彙總的快照，依鍵排序。
func (f *fixture) buckets(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT store_id, day::text, views FROM seller.product_views_daily ORDER BY store_id, day")
	must(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var storeID, views int64
		var day string
		must(t, rows.Scan(&storeID, &day, &views))
		out = append(out, fmt.Sprintf("%d/%s=%d", storeID, day, views))
	}
	must(t, rows.Err())
	return out
}

// 寫入端只追加原始事件：不解析商品所屬的店家（不呼叫 port）、不碰每日彙總。
func TestRecordProductViewOnlyAppendsAnEvent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	must(t, f.module.RecordProductView(ctx, 10))
	must(t, f.module.RecordProductView(ctx, 10))
	if got := f.pendingEvents(t); got != 2 {
		t.Fatalf("pending events=%d, want 2 (總次數、不去重)", got)
	}
	if f.products.callCount() != 0 || len(f.buckets(t)) != 0 {
		t.Fatalf("record path resolved stores (%d port calls) or wrote buckets %v", f.products.callCount(), f.buckets(t))
	}
	// 事件重送（同一個 event_id）不會多一列。
	id := uuid.NewV7()
	for range 2 {
		must(t, f.module.IngestProductView(ctx, ProductView{EventID: id, ProductID: 11, OccurredAt: f.now}))
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE event_id = $1", id); got != 1 {
		t.Fatalf("redelivered event rows=%d, want 1", got)
	}
	for _, productID := range []int64{0, -5} {
		if err := f.module.RecordProductView(ctx, productID); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("productID %d: %v", productID, err)
		}
	}
}

// 彙總以店家（store_id）為鍵、以事件 occurred_at 的台北日曆日為界，與處理時間無關：台北 23:59 的事件過了午夜才處理，
// 仍然記在前一天。
func TestRollupBucketsByStoreAndTaipeiDayOfOccurrence(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	if s.StoreID == accountID {
		t.Fatal("fixture must keep store_id and account_id apart")
	}
	f.products.set(10, s.StoreID)
	f.products.set(11, s.StoreID)
	// UTC 的 10/3 15:59:59 還是台北的 10/3，16:00:00 已經是台北的 10/4。
	for i, at := range []time.Time{
		time.Date(2026, 10, 3, 15, 59, 59, 0, time.UTC), time.Date(2026, 10, 3, 15, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC),
	} {
		must(t, f.module.IngestProductView(ctx, ProductView{EventID: uuid.NewV7(), ProductID: 10 + int64(i%2), OccurredAt: at}))
	}
	// 處理發生在台北的 10/4 中午，晚了半天。
	f.now = time.Date(2026, 10, 4, 12, 0, 0, 0, twtime.Taipei)
	n, err := f.module.RollupProductViews(ctx)
	must(t, err)
	if n != 3 {
		t.Fatalf("rollup processed %d, want 3", n)
	}
	if got := f.viewRow(t, s.StoreID, "2026-10-03"); got != 2 {
		t.Fatalf("10/3 台北的瀏覽=%d, want 2", got)
	}
	if got := f.viewRow(t, s.StoreID, "2026-10-04"); got != 1 {
		t.Fatalf("10/4 台北的瀏覽=%d, want 1", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_views_daily WHERE store_id = $1", accountID); got != 0 {
		t.Fatalf("以帳號 ID 為鍵的列 %d 筆：瀏覽必須記在 store_id 上", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE outcome = 'COUNTED' AND store_id = $1 AND processed_at = $2", s.StoreID, f.now); got != 3 {
		t.Fatalf("counted events=%d, want 3 with store_id and processed_at", got)
	}
	// 沒有去重：同一天一直加。
	for range 3 {
		must(t, f.module.IngestProductView(ctx, ProductView{EventID: uuid.NewV7(), ProductID: 10, OccurredAt: time.Date(2026, 10, 4, 3, 0, 0, 0, twtime.Taipei)}))
	}
	_, err = f.module.RollupProductViews(ctx)
	must(t, err)
	if got := f.viewRow(t, s.StoreID, "2026-10-04"); got != 4 {
		t.Fatalf("不去重：10/4 的瀏覽=%d, want 4", got)
	}
}

// 兩家店、多個商品：各自累加，一批只呼叫一次 port（去重後的商品 ID）。
func TestRollupResolvesDistinctProductsInOneCall(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	f.products.set(10, a.StoreID)
	f.products.set(20, b.StoreID)
	for _, productID := range []int64{10, 20, 10, 10} {
		must(t, f.module.RecordProductView(ctx, productID))
	}
	if _, err := f.module.RollupProductViews(ctx); err != nil {
		t.Fatal(err)
	}
	day := f.now.In(twtime.Taipei).Format(time.DateOnly)
	if f.viewRow(t, a.StoreID, day) != 3 || f.viewRow(t, b.StoreID, day) != 1 {
		t.Fatalf("buckets=%v", f.buckets(t))
	}
	if f.products.callCount() != 1 || !slices.Equal(f.products.calls[0], []int64{10, 20}) {
		t.Fatalf("port calls=%v, want one call with [10 20]", f.products.calls)
	}
}

// 商品 port 暫時失敗：這批沒有任何事件被標記、沒有計數；恢復後重試只計一次。
func TestRollupTransientPortErrorMarksNothingAndRetryCountsOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.products.set(10, s.StoreID)
	must(t, f.module.RecordProductView(ctx, 10))
	must(t, f.module.RecordProductView(ctx, 10))
	f.products.err = errors.New("product service down")
	if _, err := f.module.RollupProductViews(ctx); err == nil {
		t.Fatal("rollup swallowed the port error")
	}
	if f.pendingEvents(t) != 2 || len(f.buckets(t)) != 0 {
		t.Fatalf("port failure left pending=%d buckets=%v", f.pendingEvents(t), f.buckets(t))
	}
	f.products.err = nil
	n, err := f.module.RollupProductViews(ctx)
	must(t, err)
	day := f.now.In(twtime.Taipei).Format(time.DateOnly)
	if n != 2 || f.viewRow(t, s.StoreID, day) != 2 || f.pendingEvents(t) != 0 {
		t.Fatalf("retry processed %d, buckets=%v", n, f.buckets(t))
	}
}

// 商品不存在或沒有店家：標為 UNRESOLVED，不計入任何地方，也不再重試（不會無限重試）。
func TestRollupMarksUnresolvedProductsAndDoesNotRetryThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.products.set(30, 0) // 商品存在但沒有店家
	must(t, f.module.RecordProductView(ctx, 404))
	must(t, f.module.RecordProductView(ctx, 30))
	n, err := f.module.RollupProductViews(ctx)
	must(t, err)
	if n != 2 || len(f.buckets(t)) != 0 {
		t.Fatalf("processed=%d buckets=%v", n, f.buckets(t))
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE outcome = 'UNRESOLVED' AND store_id IS NULL AND processed_at IS NOT NULL"); got != 2 {
		t.Fatalf("unresolved events=%d, want 2", got)
	}
	calls := f.products.callCount()
	if n, err = f.module.RollupProductViews(ctx); err != nil || n != 0 || f.products.callCount() != calls {
		t.Fatalf("rerun processed=%d err=%v port calls %d→%d: unresolved events must not be retried", n, err, calls, f.products.callCount())
	}
}

// 商品 port 解析出來的店家不在 seller.stores：該事件標為 UNRESOLVED（不計入），同一批其他事件照常計入，不讓每日彙總違反
// store_id 的外鍵而卡住整批。
func TestRollupMarksEventsOfUnknownStoresUnresolvedAndTheBatchContinues(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	const unknownStore = int64(987654321)
	f.products.set(40, s.StoreID)
	f.products.set(41, unknownStore)
	f.products.set(42, s.StoreID)
	for _, productID := range []int64{40, 41, 42} {
		must(t, f.module.RecordProductView(ctx, productID))
	}
	n, err := f.module.RollupProductViews(ctx)
	must(t, err)
	if n != 3 {
		t.Fatalf("processed=%d, want 3: 不認得的店家不能讓整批失敗", n)
	}
	if got := f.viewRow(t, s.StoreID, "2026-10-01"); got != 2 {
		t.Fatalf("known store views=%d, want 2", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_views_daily WHERE store_id = $1", unknownStore); got != 0 {
		t.Fatalf("unknown store has %d daily rows, want 0", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE product_id = 41 AND outcome = 'UNRESOLVED' AND store_id IS NULL AND processed_at IS NOT NULL"); got != 1 {
		t.Fatalf("unknown-store event must be UNRESOLVED, got %d", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE outcome = 'COUNTED'"); got != 2 {
		t.Fatalf("counted events=%d, want 2", got)
	}
	if n, err = f.module.RollupProductViews(ctx); err != nil || n != 0 {
		t.Fatalf("rerun processed=%d err=%v: 已標記的事件不重試", n, err)
	}
}

// 重跑不重複計數；一批的上限讓大量積壓分成多批，總數仍然正確。
func TestRollupRerunDoesNotDoubleCountAndDrainsBacklog(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.products.set(10, s.StoreID)
	const n = 250 // 超過一批（100）
	for range n {
		must(t, f.module.RecordProductView(ctx, 10))
	}
	for range 3 {
		if _, err := f.module.RollupProductViews(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.viewRow(t, s.StoreID, f.now.In(twtime.Taipei).Format(time.DateOnly)); got != n {
		t.Fatalf("views=%d, want %d", got, n)
	}
}

// 並行的彙總不丟也不重複計數。
func TestRollupConcurrentRunsCountEachEventOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.products.set(10, s.StoreID)
	const events, runners = 300, 4
	for range events {
		must(t, f.module.RecordProductView(ctx, 10))
	}
	errs := make(chan error, runners)
	for range runners {
		go func() { _, err := f.module.RollupProductViews(ctx); errs <- err }()
	}
	for range runners {
		must(t, <-errs)
	}
	_, err := f.module.RollupProductViews(ctx) // 被別的 job 鎖住而跳過的列，下一輪補上
	must(t, err)
	if got := f.viewRow(t, s.StoreID, f.now.In(twtime.Taipei).Format(time.DateOnly)); got != events || f.pendingEvents(t) != 0 {
		t.Fatalf("views=%d pending=%d, want %d and 0", got, f.pendingEvents(t), events)
	}
}

// 重算的結果與逐步彙總相同；改壞的彙總會被修好，區間外的列不動；UNRESOLVED 與沒處理的事件不進重算。
func TestRebuildMatchesIncrementalRollup(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	f.products.set(10, a.StoreID)
	f.products.set(20, b.StoreID)
	f.now = time.Date(2026, 10, 7, 12, 0, 0, 0, twtime.Taipei)
	for _, ev := range []struct {
		product int64
		at      time.Time
	}{
		{10, time.Date(2026, 10, 2, 23, 59, 0, 0, twtime.Taipei)}, {10, time.Date(2026, 10, 3, 0, 0, 0, 0, twtime.Taipei)},
		{20, time.Date(2026, 10, 3, 9, 0, 0, 0, twtime.Taipei)}, {10, time.Date(2026, 10, 5, 9, 0, 0, 0, twtime.Taipei)},
		{20, time.Date(2026, 10, 5, 10, 0, 0, 0, twtime.Taipei)}, {20, time.Date(2026, 10, 5, 11, 0, 0, 0, twtime.Taipei)},
		{404, time.Date(2026, 10, 5, 12, 0, 0, 0, twtime.Taipei)},
	} {
		must(t, f.module.IngestProductView(ctx, ProductView{EventID: uuid.NewV7(), ProductID: ev.product, OccurredAt: ev.at}))
	}
	if _, err := f.module.RollupProductViews(ctx); err != nil {
		t.Fatal(err)
	}
	// 還沒處理的事件：重算不能把它算進去。
	must(t, f.module.IngestProductView(ctx, ProductView{EventID: uuid.NewV7(), ProductID: 10, OccurredAt: time.Date(2026, 10, 6, 9, 0, 0, 0, twtime.Taipei)}))
	incremental := f.buckets(t)
	if len(incremental) != 5 {
		t.Fatalf("incremental buckets=%v", incremental)
	}
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	// 弄壞區間內外各一列與一個多餘的列，再重算 10/3 到 10/7。
	for _, q := range []string{
		fmt.Sprintf("UPDATE seller.product_views_daily SET views = 999 WHERE store_id = %d AND day = '2026-10-05'", a.StoreID),
		fmt.Sprintf("INSERT INTO seller.product_views_daily (store_id, day, views) VALUES (%d, '2026-10-06', 5)", a.StoreID),
		fmt.Sprintf("UPDATE seller.product_views_daily SET views = 777 WHERE store_id = %d AND day = '2026-10-02'", a.StoreID),
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	must(t, f.module.RebuildProductViews(ctx, day(3), day(7)))
	want := slices.Clone(incremental)
	for i, row := range want {
		if row == fmt.Sprintf("%d/2026-10-02=1", a.StoreID) {
			want[i] = fmt.Sprintf("%d/2026-10-02=777", a.StoreID) // 區間外維持原樣
		}
	}
	if got := f.buckets(t); !slices.Equal(got, want) {
		t.Fatalf("rebuilt buckets=%v, want %v", got, want)
	}
	// 完整重算（10/2 起）與逐步彙總一致。
	must(t, f.module.RebuildProductViews(ctx, day(2), day(7)))
	if got := f.buckets(t); !slices.Equal(got, incremental) {
		t.Fatalf("full rebuild=%v, want incremental %v", got, incremental)
	}
}

// 重算與解析路徑一樣只認 seller.stores 裡有的店：切換之前已計入的事件帶著舊 seller-service 的店號（沒有 FK），重算略過它們，
// 不被 product_views_daily 的 FK 擋下，也不影響其他店。
func TestRebuildSkipsCountedEventsOfStoresThatDoNotExist(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.products.set(10, s.StoreID)
	f.now = time.Date(2026, 10, 7, 12, 0, 0, 0, twtime.Taipei)
	at := time.Date(2026, 10, 6, 9, 0, 0, 0, twtime.Taipei)
	must(t, f.module.IngestProductView(ctx, ProductView{EventID: uuid.NewV7(), ProductID: 10, OccurredAt: at}))
	if _, err := f.module.RollupProductViews(ctx); err != nil {
		t.Fatal(err)
	}
	const legacyStore = int64(9)
	if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.product_view_events (event_id, product_id, occurred_at, store_id, processed_at, outcome)
		VALUES ($1, 11, $2, $3, $2, 'COUNTED')`, uuid.NewV7(), at, legacyStore); err != nil {
		t.Fatal(err)
	}
	must(t, f.module.RebuildProductViews(ctx, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)))
	if got := f.viewRow(t, s.StoreID, "2026-10-06"); got != 1 {
		t.Fatalf("existing store views=%d, want 1", got)
	}
	if got := f.count(t, "SELECT count(*) FROM seller.product_views_daily WHERE store_id = $1", legacyStore); got != 0 {
		t.Fatalf("legacy store has %d daily rows, want 0", got)
	}
}

// 重算只涵蓋完整落在原始事件保留期內的台北日，且區間要合法。
func TestRebuildRejectsRangesOutsideRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.now = time.Date(2026, 10, 10, 12, 0, 0, 0, twtime.Taipei)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	for name, r := range map[string][2]time.Time{
		"超出保留期":  {day(2), day(5)}, // 10/3 12:00 之前的日子（10/3 本身 00:00 早於 10/3 12:00）
		"起日晚於迄日": {day(8), day(7)},
		"迄日在未來":  {day(8), day(11)},
	} {
		if err := f.module.RebuildProductViews(ctx, r[0], r[1]); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if err := f.module.RebuildProductViews(ctx, day(4), day(10)); err != nil {
		t.Fatalf("10/4 起完全在保留期內: %v", err)
	}
}

// 清理只刪「已處理」且「發生時間早於 7 天前」的事件；沒處理的（再舊也留著，它還沒進彙總）與較新的都留著。
func TestPurgeOnlyDeletesProcessedEventsOlderThanRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.products.set(10, s.StoreID)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, twtime.Taipei)
	f.now = now
	add := func(at time.Time) uuid.UUID {
		id := uuid.NewV7()
		must(t, f.module.IngestProductView(ctx, ProductView{EventID: id, ProductID: 10, OccurredAt: at}))
		return id
	}
	oldProcessed := add(now.Add(-7*24*time.Hour - time.Second))
	boundary := add(now.Add(-7 * 24 * time.Hour))
	recent := add(now.Add(-24 * time.Hour))
	if _, err := f.module.RollupProductViews(ctx); err != nil {
		t.Fatal(err)
	}
	oldPending := add(now.Add(-30 * 24 * time.Hour))
	before := f.buckets(t)
	n, err := f.module.PurgeProductViewEvents(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	exists := func(id uuid.UUID) bool {
		return f.count(t, "SELECT count(*) FROM seller.product_view_events WHERE event_id = $1", id) == 1
	}
	if exists(oldProcessed) || !exists(boundary) || !exists(recent) || !exists(oldPending) {
		t.Fatalf("purge kept old processed=%v, boundary=%v recent=%v old pending=%v", exists(oldProcessed), exists(boundary), exists(recent), exists(oldPending))
	}
	if after := f.buckets(t); !slices.Equal(before, after) {
		t.Fatalf("purge changed the daily buckets: %v → %v", before, after)
	}
	// 清理後沒處理的舊事件仍然會被彙總，之後才輪到清理。
	if _, err := f.module.RollupProductViews(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err = f.module.PurgeProductViewEvents(ctx); err != nil || n != 1 || exists(oldPending) {
		t.Fatalf("second purge n=%d err=%v", n, err)
	}
}

// 期間 [start, end) 以台北日曆日計、左閉右開；兩端各差一秒就分到不同的日子。
func TestProductViewsBetweenCountsTaipeiDaysHalfOpen(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	day := time.Date(2026, 2, 2, 0, 0, 0, 0, twtime.Taipei)
	for _, at := range []time.Time{
		day.Add(-time.Second),                  // 前一天 23:59:59：期間外
		day,                                    // 起日 00:00：算
		day.AddDate(0, 0, 1).Add(time.Hour),    // 隔天：算
		day.AddDate(0, 0, 2).Add(-time.Second), // 終點前一秒：算
		day.AddDate(0, 0, 2),                   // 終點 00:00：期間外
	} {
		f.view(t, s.StoreID, at)
	}
	f.now = day.AddDate(0, 0, 5)
	got, err := f.module.ProductViewsBetween(ctx, s.StoreID, day, day.AddDate(0, 0, 2))
	must(t, err)
	if got != 3 {
		t.Fatalf("views=%d, want 3", got)
	}
	// 一天的期間。
	if got, err = f.module.ProductViewsBetween(ctx, s.StoreID, day, day.AddDate(0, 0, 1)); err != nil || got != 1 {
		t.Fatalf("single day: views=%d err=%v", got, err)
	}
	// 沒有彙總列的期間是 0。
	if got, err = f.module.ProductViewsBetween(ctx, s.StoreID, day.AddDate(0, 0, 30), day.AddDate(0, 0, 40)); err != nil || got != 0 {
		t.Fatalf("empty range: views=%d err=%v", got, err)
	}
}

// 別家店的瀏覽不混進來；storeID 不是正數、期間的兩端不是台北 00:00、起點不早於終點都被拒絕。
func TestProductViewsBetweenIsIsolatedPerStoreAndValidatesInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	f.view(t, a.StoreID, f.now)
	local := f.now.In(twtime.Taipei)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, twtime.Taipei)
	next := day.AddDate(0, 0, 1)
	got, err := f.module.ProductViewsBetween(ctx, b.StoreID, day, next)
	must(t, err)
	if got != 0 {
		t.Fatalf("store b saw store a's views: %d", got)
	}
	if got, err = f.module.ProductViewsBetween(ctx, a.StoreID, day, next); err != nil || got != 1 {
		t.Fatalf("store a: views=%d err=%v", got, err)
	}
	for name, args := range map[string]struct {
		store      int64
		start, end time.Time
	}{
		"storeID 0": {0, day, next},
		"起點不是午夜":    {a.StoreID, day.Add(time.Hour), next},
		"終點不是午夜":    {a.StoreID, day, next.Add(time.Hour)},
		"空期間":       {a.StoreID, day, day},
		"倒置":        {a.StoreID, next, day},
	} {
		if _, err := f.module.ProductViewsBetween(ctx, args.store, args.start, args.end); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s: %v", name, err)
		}
	}
}
