//go:build integration

package discovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// setup 起一個只套用了本模組 migration 的空 database：integration 測試只跑在自己的 schema 上。
func setup(t *testing.T) *db.DB {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "discovery", Dir: Migrations()}))
	return d
}

var clock = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

// fakes 是四個 port 的替身。每個 port 有自己的錯誤開關。
type fakes struct {
	mu sync.Mutex

	public     []int64               // product port 的回傳
	exposable  map[int64]time.Time   // seller：可曝光的店家與第一次可曝光的時間
	orders     map[int64]OrderStats  // order port
	ratings    map[int64]RatingStats // review port：只有放進來的店有評價
	productErr error
	orderErr   error
	sellerErr  error
	ratingErr  map[int64]error // 針對單一店家：批次裡含有它就失敗
	// ratingExtra 是 review port 額外回傳的（沒被詢問的）店家，用來模擬違反契約。
	ratingExtra map[int64]RatingStats

	// onOrders 在 order port 被呼叫時執行一次（在 mutex 之外），用來製造重算的交錯。
	onOrders func()
	// onSeller 在 seller port 第一次被呼叫時執行一次（在 mutex 之外）。
	onSeller func()

	orderDays     []int
	sellerBatch   []int
	ratingBatches []int
	sellerCalled  int
}

func newFakes() *fakes {
	return &fakes{exposable: map[int64]time.Time{}, orders: map[int64]OrderStats{}, ratings: map[int64]RatingStats{}, ratingErr: map[int64]error{}, ratingExtra: map[int64]RatingStats{}}
}

// add 加一家有公開商品且可曝光的店。
func (f *fakes) add(id int64, opened time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.public = append(f.public, id)
	f.exposable[id] = opened
}

func (f *fakes) StoreOrderStats(_ context.Context, days int) ([]OrderStats, error) {
	f.mu.Lock()
	hook := f.onOrders
	f.onOrders = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orderDays = append(f.orderDays, days)
	if f.orderErr != nil {
		return nil, f.orderErr
	}
	out := make([]OrderStats, 0, len(f.orders))
	for _, o := range f.orders {
		out = append(out, o)
	}
	return out, nil
}

func (f *fakes) StoresWithPublicProducts(context.Context) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.productErr != nil {
		return nil, f.productErr
	}
	return slices.Clone(f.public), nil
}

func (f *fakes) StoreRatingStats(_ context.Context, ids []int64) (map[int64]RatingStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ratingBatches = append(f.ratingBatches, len(ids))
	out := map[int64]RatingStats{}
	for _, id := range ids {
		if err := f.ratingErr[id]; err != nil {
			return nil, err
		}
		if r, ok := f.ratings[id]; ok {
			out[id] = r
		}
	}
	for id, r := range f.ratingExtra {
		out[id] = r
	}
	return out, nil
}

func (f *fakes) ExposableStores(_ context.Context, ids []int64) ([]seller.ExposableStore, error) {
	f.mu.Lock()
	hook := f.onSeller
	f.onSeller = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sellerCalled++
	f.sellerBatch = append(f.sellerBatch, len(ids))
	if f.sellerErr != nil {
		return nil, f.sellerErr
	}
	var out []seller.ExposableStore
	for _, id := range ids {
		if opened, ok := f.exposable[id]; ok {
			out = append(out, seller.ExposableStore{StoreID: id, PublishedAt: opened})
		}
	}
	return out, nil
}

func (f *fakes) suspend(ids ...int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.exposable, id)
	}
}

type fixture struct {
	d      *db.DB
	module *Module
	f      *fakes
	logs   *logCapture

	clockMu sync.Mutex
	now     time.Time
}

// logCapture 收集 log 紀錄，讓測試檢查層級與屬性。
type logCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (c *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, r.Clone())
	return nil
}
func (c *logCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(string) slog.Handler      { return c }

// attr 回傳層級是 level 的紀錄裡，鍵為 key 的屬性值（找不到是空字串）。
func (c *logCapture) attr(level slog.Level, key string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, r := range c.records {
		if r.Level != level {
			continue
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == key {
				out = append(out, a.Value.String())
			}
			return true
		})
	}
	return out
}

func (x *fixture) clock() time.Time {
	x.clockMu.Lock()
	defer x.clockMu.Unlock()
	return x.now
}

// advance 讓時鐘前進：每次重算的開始時間都要晚於上一次成功的那次（重算以開始時間排序，同一個時間點的第二次會被丟棄）。
func (x *fixture) advance(d time.Duration) {
	x.clockMu.Lock()
	defer x.clockMu.Unlock()
	x.now = x.now.Add(d)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := setup(t)
	f := newFakes()
	x := &fixture{d: d, f: f, logs: &logCapture{}, now: clock}
	x.module = New(Deps{
		DB: d, Orders: f, Products: f, Reviews: f, Stores: f,
		Now: x.clock, Logger: slog.New(x.logs),
	})
	return x
}

// recompute 先讓時鐘前進一分鐘再重算，所以連續呼叫是依序的、各自有更晚的開始時間。
func (x *fixture) recompute(t *testing.T) int {
	t.Helper()
	x.advance(time.Minute)
	r, err := x.module.RecomputeSellerScores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Written {
		t.Fatal("the run was discarded")
	}
	return r.Candidates
}

func (x *fixture) recommended(t *testing.T, limit int) []int64 {
	t.Helper()
	got, err := x.module.RecommendedSellers(context.Background(), limit)
	if err != nil {
		t.Fatal(err)
	}
	return storeIDs(got.Sellers)
}

func storeIDs(rows []RankedSeller) []int64 {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.StoreID
	}
	return ids
}

func (x *fixture) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := x.d.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func days(n int) time.Time { return clock.AddDate(0, 0, -n) }

func TestRecomputeWritesTheSnapshotAndReadFollowsScoreOrder(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(200)) // 老店，有大量成交與好評
	x.f.add(2, days(200)) // 老店，沒有任何資料
	x.f.add(3, days(1))   // 新店，沒有任何資料
	x.f.add(4, days(200)) // 老店，取消很多
	x.f.orders[1] = OrderStats{StoreID: 1, DistinctBuyers: 40, QualifyingOrders: 60}
	x.f.orders[4] = OrderStats{StoreID: 4, DistinctBuyers: 3, QualifyingOrders: 3, SellerCancelledOrders: 30}
	x.f.orders[99] = OrderStats{StoreID: 99, DistinctBuyers: 500, QualifyingOrders: 500} // 沒有公開商品的店：不是候選
	x.f.ratings[1] = RatingStats{Count: 50, Average: 4.8}

	if n := x.recompute(t); n != 4 {
		t.Fatalf("candidates = %d", n)
	}
	if !slices.Equal(x.f.orderDays, []int{30}) {
		t.Fatalf("order stats must be asked once for 30 days: %v", x.f.orderDays)
	}
	if got := x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE formula_version = 1 AND computed_at = $1", x.clock()); got != 4 {
		t.Fatalf("rows with version and computed_at = %d", got)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE rating_average IS NULL AND rating_count = 0") != 3 ||
		x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 1 AND rating_count = 50 AND rating_average = 4.8 AND distinct_buyers = 40 AND qualifying_orders = 60") != 1 {
		t.Fatal("stored inputs do not match")
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 99") != 0 {
		t.Fatal("store without public products must not be stored")
	}
	// 店 1 需求滿分且評價最好；店 3 有新店加分；店 2 什麼都沒有；店 4 被取消扣分壓在最後。
	rec, err := x.module.RecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Sellers
	if ids := storeIDs(got); !slices.Equal(ids, []int64{1, 3, 2, 4}) {
		t.Fatalf("order = %v", ids)
	}
	if got[0].RecentOrders != 60 || got[1].RecentOrders != 0 {
		t.Fatalf("recent orders: %+v", got)
	}
	if rec.ComputedAt == nil || !rec.ComputedAt.Equal(x.clock()) {
		t.Fatalf("ComputedAt = %v, want the run's start %v", rec.ComputedAt, x.clock())
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 4 AND reliability_penalty > 0.5") != 1 {
		t.Fatal("penalty not stored")
	}
}

func TestStoresWithoutPublicProductsOrExposureAreNotCandidates(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(5))
	x.f.add(2, days(5))
	x.f.exposable[3] = days(5) // 可曝光但沒有公開商品
	x.f.public = append(x.f.public, 4)
	// 4 有公開商品但 seller 說不可曝光（不在 exposable）
	if n := x.recompute(t); n != 2 {
		t.Fatalf("candidates = %d", n)
	}
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{2, 1}) {
		t.Fatalf("got %v", got)
	}
}

func TestReplaceDropsStoresThatAreNoLongerCandidatesAndEmptyClears(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(5))
	x.f.add(2, days(5))
	x.recompute(t)
	x.f.public = []int64{2}
	x.recompute(t)
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{2}) {
		t.Fatalf("got %v", got)
	}
	x.f.public = nil
	if n := x.recompute(t); n != 0 {
		t.Fatalf("candidates = %d", n)
	}
	if got := x.recommended(t, 10); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestUpstreamFailureKeepsThePreviousSnapshot(t *testing.T) {
	t.Parallel()
	boom := errors.New("upstream down")
	cases := map[string]func(*fakes){
		"product": func(f *fakes) { f.productErr = boom },
		"order":   func(f *fakes) { f.orderErr = boom },
		"seller":  func(f *fakes) { f.sellerErr = boom },
		"review":  func(f *fakes) { f.ratingErr[2] = boom },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			x := newFixture(t)
			x.f.add(1, days(5))
			x.f.add(2, days(3))
			x.f.orders[1] = OrderStats{StoreID: 1, DistinctBuyers: 5, QualifyingOrders: 5}
			x.recompute(t)
			before := x.recommended(t, 10)

			// 世界變了（新增一家店、停掉一家），但上游壞掉：快照不能被改。
			x.f.add(3, days(1))
			breakIt(x.f)
			if _, err := x.module.RecomputeSellerScores(context.Background()); !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
			x.f.sellerErr = nil // 讀取也會問 seller；這裡要驗的是快照
			if got := x.recommended(t, 10); !slices.Equal(got, before) {
				t.Fatalf("snapshot changed: %v -> %v", before, got)
			}
			if x.count(t, "SELECT count(*) FROM discovery.seller_scores") != 2 {
				t.Fatal("row count changed")
			}
		})
	}
}

func TestFailedRatingIsNeverTurnedIntoZero(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	for id := int64(1); id <= 30; id++ {
		x.f.add(id, days(5))
	}
	x.f.ratingErr[17] = errors.New("review down")
	if _, err := x.module.RecomputeSellerScores(context.Background()); err == nil {
		t.Fatal("a failed rating fetch must fail the run")
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores") != 0 {
		t.Fatal("nothing may be written when a rating fetch failed")
	}
}

// 評價以批次取（每批最多 100 家，依序呼叫），每個候選店都被問到一次；沒有評價的店不在回傳裡，當作沒有評價。
func TestRatingsAreFetchedInBatchesOfAtMostOneHundredAndEveryCandidateIsAskedOnce(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	for id := int64(1); id <= 250; id++ {
		x.f.add(id, days(5))
	}
	x.f.ratings[7] = RatingStats{Count: 3, Average: 4}
	x.f.ratings[250] = RatingStats{Count: 12, Average: 5}
	if n := x.recompute(t); n != 250 {
		t.Fatalf("candidates = %d", n)
	}
	if !slices.Equal(x.f.ratingBatches, []int{100, 100, 50}) {
		t.Fatalf("rating batches = %v", x.f.ratingBatches)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE rating_count > 0") != 2 ||
		x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 250 AND rating_count = 12 AND rating_average = 5") != 1 {
		t.Fatal("ratings not stored as returned")
	}
}

// 上游回傳沒有被問的店家是違反契約：整次失敗、不寫入，不靜默略過那一筆。
func TestReviewReturningAnUnaskedStoreFailsTheRun(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(5))
	x.recompute(t)
	x.f.ratingExtra[999] = RatingStats{Count: 1, Average: 5}
	x.advance(time.Minute)
	if _, err := x.module.RecomputeSellerScores(context.Background()); err == nil {
		t.Fatal("a contract violation must fail the run")
	}
	if got := x.logs.attr(slog.LevelError, "upstream"); !slices.Contains(got, "review") {
		t.Fatalf("error logs do not name the upstream: %v", got)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores") != 1 {
		t.Fatal("the previous snapshot must be kept")
	}
}

func TestEligibilityIsAskedInBatchesOfAtMostMaxBatchSize(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	for id := int64(1); id <= 250; id++ {
		x.f.add(id, days(5))
	}
	if n := x.recompute(t); n != 250 {
		t.Fatalf("candidates = %d", n)
	}
	if !slices.Equal(x.f.sellerBatch, []int{100, 100, 50}) {
		t.Fatalf("batches = %v", x.f.sellerBatch)
	}
}

// 重算之後才被停權的店，下一次讀取就不出現（資格是讀取時即時問的），不必等下一輪重算。
func TestStoreSuspendedAfterTheRunIsFilteredAtReadTime(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(1))
	x.f.add(2, days(2))
	x.f.add(3, days(3))
	x.recompute(t)
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("before: %v", got)
	}
	x.f.suspend(1)
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{2, 3}) {
		t.Fatalf("after suspension: %v", got)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores") != 3 {
		t.Fatal("the snapshot itself is untouched by a read")
	}
}

func TestLimitMustBeBetweenOneAndFifty(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	for _, limit := range []int{-1, 0, 51, 1000} {
		_, err := x.module.RecommendedSellers(context.Background(), limit)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("limit %d: err = %v", limit, err)
		}
	}
	for _, limit := range []int{1, 50} {
		if _, err := x.module.RecommendedSellers(context.Background(), limit); err != nil {
			t.Errorf("limit %d: %v", limit, err)
		}
	}
}

func TestReadPagesPastManyFilteredTopRows(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	// 沒有任何訂單與評價：店 i 開店 i 小時前，i 越小分數越高。前 150 名都在重算後失去資格。
	const total = 260
	for i := int64(1); i <= total; i++ {
		x.f.add(i, clock.Add(-time.Duration(i)*time.Hour))
	}
	x.recompute(t)
	for i := int64(1); i <= 150; i++ {
		x.f.suspend(i)
	}
	x.f.sellerBatch = nil
	got := x.recommended(t, 50)
	want := make([]int64, 50)
	for i := range want {
		want[i] = int64(151 + i)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	if len(x.f.sellerBatch) != 2 || slices.Max(x.f.sellerBatch) > seller.MaxBatchSize {
		t.Fatalf("eligibility batches = %v", x.f.sellerBatch)
	}

	// 全部都失去資格：讀到快照的盡頭就停，回空清單（有界），不是無限翻頁。
	for i := int64(151); i <= total; i++ {
		x.f.suspend(i)
	}
	x.f.sellerBatch = nil
	if got := x.recommended(t, 50); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if len(x.f.sellerBatch) != 3 {
		t.Fatalf("pages = %v", x.f.sellerBatch)
	}
}

func TestEqualScoresOrderByPublishedAtThenStoreIDDescending(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	// 都超過 30 天、沒有資料：分數完全相同。
	x.f.add(5, days(100))
	x.f.add(9, days(100))
	x.f.add(2, days(50))
	x.f.add(7, days(100))
	x.recompute(t)
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{2, 9, 7, 5}) {
		t.Fatalf("got %v", got)
	}
}

func TestEmptySnapshotReadsAsEmpty(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	if got := x.recommended(t, 10); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestEligibilityErrorAtReadTimeFailsTheRead(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(1))
	x.recompute(t)
	x.f.sellerErr = fmt.Errorf("seller down")
	if _, err := x.module.RecommendedSellers(context.Background(), 5); err == nil {
		t.Fatal("exposure must fail closed")
	}
}

// insertScore 直接寫一列（略過公式），用來驗 schema 的 CHECK。
func (x *fixture) insertScore(id int64, score, demand string, ratingCount int, ratingAverage string) error {
	_, err := x.d.Pool().Exec(context.Background(), fmt.Sprintf(`INSERT INTO discovery.seller_scores
		(store_id, score, demand, quality, reliability_penalty, newness, distinct_buyers, qualifying_orders, seller_cancelled_orders, rating_count, rating_average, published_at, formula_version, computed_at)
		VALUES (%d, %s, %s, 0, 0, 0, 0, 0, 0, %d, %s, now(), 1, now())`, id, score, demand, ratingCount, ratingAverage))
	return err
}

func TestSchemaChecksComponentsAndFiniteScoreButNotTheFormulaWeights(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	if err := x.insertScore(1, "0", "1.5", 0, "NULL"); err == nil {
		t.Fatal("component above 1 accepted")
	}
	if err := x.insertScore(2, "0", "0", 0, "4"); err == nil {
		t.Fatal("rating average without reviews accepted")
	}
	for _, bad := range []string{"'NaN'", "'Infinity'", "'-Infinity'"} {
		if err := x.insertScore(3, bad, "0", 0, "NULL"); err == nil {
			t.Fatalf("score %s accepted", bad)
		}
	}
	// 範圍取決於公式版本的權重（在程式裡），資料庫不寫死：換一版公式的分數不該被擋。
	for i, ok := range []string{"-3", "0", "7.5"} {
		if err := x.insertScore(int64(10+i), ok, "0", 0, "NULL"); err != nil {
			t.Fatalf("finite score %s rejected: %v", ok, err)
		}
	}
}

func TestSchemaBoundsTheRecordedError(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	exec := func(sql string) error {
		_, err := x.d.Pool().Exec(context.Background(), sql)
		return err
	}
	if err := exec(`INSERT INTO discovery.recompute_state (job, last_attempt_started_at, last_failed_at, last_error) VALUES ('seller_scores', now(), now(), '')`); err == nil {
		t.Fatal("empty error accepted")
	}
	if err := exec(`INSERT INTO discovery.recompute_state (job, last_attempt_started_at, last_failed_at, last_error) VALUES ('seller_scores', now(), now(), repeat('x', 501))`); err == nil {
		t.Fatal("error over 500 characters accepted")
	}
	if err := exec(`INSERT INTO discovery.recompute_state (job, last_attempt_started_at, last_failed_at) VALUES ('seller_scores', now(), now())`); err == nil {
		t.Fatal("a failure time without an error accepted")
	}
}

// 新店加分從「第一次可曝光」起算：審核拖了 25 天才核准的店，在它第一次曝光那天拿滿分；新店期用完之後，同分以最近才可曝光的在前。
func TestNewnessCountsFromTheFirstExposableDay(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	runAt := x.clock().Add(time.Minute) // recompute 開始的時間
	x.f.add(1, runAt)                   // 申請後 25 天才核准：seller 回的 PublishedAt 是今天（公開時間，不是核准時間）（這次重算的時間）
	x.f.add(2, runAt.AddDate(0, 0, -29))
	x.f.add(3, runAt.AddDate(0, 0, -400))
	x.f.add(4, runAt.AddDate(0, 0, -200))
	x.recompute(t)
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 1 AND newness = 1") != 1 {
		t.Fatal("a store on its first exposable day must get the full newness")
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 2 AND newness > 0 AND newness < 0.1") != 1 {
		t.Fatal("a store 29 days into its exposure must have almost none left")
	}
	// 3、4 都過了新店期、分數相同：最近才可曝光的（4）在前。
	if got := x.recommended(t, 10); !slices.Equal(got, []int64{1, 2, 4, 3}) {
		t.Fatalf("got %v", got)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE store_id = 1 AND published_at = $1", runAt) != 1 {
		t.Fatal("published_at not stored as returned by seller")
	}
}

// 排名用到的訂單事實隨排行一起回傳；排名用的評價只留在快照內，不在回傳裡。
func TestRankedSellersCarryTheFactsUsedForRanking(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	x.f.add(1, days(100))
	x.f.add(2, days(100))
	x.f.orders[1] = OrderStats{StoreID: 1, DistinctBuyers: 4, QualifyingOrders: 9}
	x.f.ratings[1] = RatingStats{Count: 50, Average: 4.8}
	x.recompute(t)
	rec, err := x.module.RecommendedSellers(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Sellers) != 2 {
		t.Fatalf("sellers = %+v", rec.Sellers)
	}
	a, b := rec.Sellers[0], rec.Sellers[1]
	if a.StoreID != 1 || a.RecentOrders != 9 {
		t.Fatalf("first = %+v", a)
	}
	if b.StoreID != 2 || b.RecentOrders != 0 {
		t.Fatalf("second = %+v", b)
	}
}

// 「還沒算過」與「算過了、沒有合格店家」要分得出來：ComputedAt 是 nil 或有值。
func TestFreshnessDistinguishesNeverComputedFromNoEligibleStores(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	ctx := context.Background()
	rec, err := x.module.RecommendedSellers(ctx, 10)
	if err != nil || rec.ComputedAt != nil || len(rec.Sellers) != 0 || rec.Sellers == nil {
		t.Fatalf("never attempted: %+v err=%v", rec, err)
	}

	// 只有失敗的嘗試：還是沒有快照，狀態列記下失敗與原因。
	x.f.productErr = errors.New("catalog is down")
	x.advance(time.Minute)
	if _, err := x.module.RecomputeSellerScores(ctx); err == nil {
		t.Fatal("expected failure")
	}
	rec, err = x.module.RecommendedSellers(ctx, 10)
	if err != nil || rec.ComputedAt != nil {
		t.Fatalf("only failures: %+v err=%v", rec, err)
	}
	var failedAt time.Time
	var lastErr string
	if err := x.d.Pool().QueryRow(ctx, "SELECT last_failed_at, last_error FROM discovery.recompute_state WHERE job = 'seller_scores' AND last_succeeded_at IS NULL").Scan(&failedAt, &lastErr); err != nil {
		t.Fatal(err)
	}
	if !failedAt.Equal(x.clock()) || !strings.Contains(lastErr, "product") || !strings.Contains(lastErr, "catalog is down") {
		t.Fatalf("failure state: %v %q", failedAt, lastErr)
	}
	if got := x.logs.attr(slog.LevelError, "upstream"); !slices.Equal(got, []string{"product"}) {
		t.Fatalf("error logs name the upstream: %v", got)
	}

	// 成功但沒有任何合格店家：ComputedAt 有值，清單空。
	x.f.productErr = nil
	x.recompute(t)
	succeededAt := x.clock()
	rec, err = x.module.RecommendedSellers(ctx, 10)
	if err != nil || rec.ComputedAt == nil || !rec.ComputedAt.Equal(succeededAt) || len(rec.Sellers) != 0 {
		t.Fatalf("computed with no stores: %+v err=%v", rec, err)
	}

	// 之後又失敗：快照與 ComputedAt 不變；錯誤文字是有界的（超長、多位元組的訊息被截斷成合法的字串）。
	x.f.orderErr = errors.New(strings.Repeat("訂單服務壞了", 400))
	x.advance(time.Minute)
	if _, err := x.module.RecomputeSellerScores(ctx); err == nil {
		t.Fatal("expected failure")
	}
	rec, err = x.module.RecommendedSellers(ctx, 10)
	if err != nil || rec.ComputedAt == nil || !rec.ComputedAt.Equal(succeededAt) {
		t.Fatalf("after a later failure: %+v err=%v", rec, err)
	}
	var n int
	var started, finished, attempt, failed time.Time
	if err := x.d.Pool().QueryRow(ctx, `SELECT char_length(last_error), last_succeeded_started_at, last_succeeded_at, last_attempt_started_at, last_failed_at
		FROM discovery.recompute_state WHERE job = 'seller_scores'`).Scan(&n, &started, &finished, &attempt, &failed); err != nil {
		t.Fatal(err)
	}
	if n != 500 || !started.Equal(succeededAt) || !finished.Equal(succeededAt) || !attempt.Equal(x.clock()) || !failed.Equal(x.clock()) {
		t.Fatalf("state: len=%d started=%v finished=%v attempt=%v failed=%v", n, started, finished, attempt, failed)
	}
}

// 兩次重算交錯：先開始的那次較晚結束，不能蓋掉後開始、先結束的那次。
func TestAnOlderRunFinishingLaterDoesNotOverwriteANewerSnapshot(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	ctx := context.Background()
	for id := int64(1); id <= 3; id++ {
		x.f.add(id, days(100))
	}
	x.f.ratings[1] = RatingStats{Count: 10, Average: 5} // 舊的世界：店 1 第一

	// 第一次重算（A）讀完資料前卡在 order port，等第二次（B）完成。
	entered, release := make(chan struct{}), make(chan struct{})
	x.f.onOrders = func() {
		close(entered)
		<-release
	}
	x.advance(time.Minute)
	startedA := x.clock()
	type outcome struct {
		r   RecomputeResult
		err error
	}
	doneA := make(chan outcome, 1)
	go func() {
		r, err := x.module.RecomputeSellerScores(ctx)
		doneA <- outcome{r, err}
	}()
	<-entered // A 已經取了開始時間、也讀過 product、seller。

	// 世界改變：店 3 變成第一。B 在 A 之後開始、先完成。
	x.f.mu.Lock()
	delete(x.f.ratings, 1)
	x.f.ratings[3] = RatingStats{Count: 10, Average: 5}
	x.f.mu.Unlock()
	x.advance(time.Minute)
	startedB := x.clock()
	rb, err := x.module.RecomputeSellerScores(ctx)
	if err != nil || !rb.Written {
		t.Fatalf("B: %+v err=%v", rb, err)
	}
	close(release)
	oa := <-doneA
	if oa.err != nil {
		t.Fatalf("A: %v", oa.err)
	}
	if oa.r.Written {
		t.Fatal("the older run must discard its result")
	}
	if got := x.logs.attr(slog.LevelWarn, "started_at"); len(got) != 1 {
		t.Fatalf("discard must be logged once: %v", got)
	}
	rec, err := x.module.RecommendedSellers(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Sellers[0].StoreID != 3 {
		t.Fatalf("B's snapshot was overwritten by A: %+v", rec.Sellers)
	}
	if rec.ComputedAt == nil || !rec.ComputedAt.Equal(startedB) || startedB.Equal(startedA) {
		t.Fatalf("ComputedAt=%v, want B's start %v (A started %v)", rec.ComputedAt, startedB, startedA)
	}
	if x.count(t, "SELECT count(*) FROM discovery.seller_scores WHERE computed_at <> $1", startedB) != 0 {
		t.Fatal("rows from two runs mixed")
	}

	// 反過來（較早開始的先完成）兩次都寫入，最後是較晚開始的那份。
	x.f.mu.Lock()
	x.f.ratings = map[int64]RatingStats{1: {Count: 10, Average: 1}, 2: {Count: 10, Average: 5}}
	x.f.mu.Unlock()
	x.recompute(t)
	if got := x.recommended(t, 1); !slices.Equal(got, []int64{2}) {
		t.Fatalf("got %v", got)
	}
}

// 讀取跨多頁時，重算在中途提交也不會讓讀取重複或漏掉店家：整個讀取看同一份快照。
func TestReadSeesOneSnapshotEvenWhenARecomputeCommitsBetweenPages(t *testing.T) {
	t.Parallel()
	x := newFixture(t)
	ctx := context.Background()
	const total = 150
	for i := int64(1); i <= total; i++ {
		x.f.add(i, x.clock().Add(-time.Duration(i)*time.Hour)) // i 越小越新、分數越高
	}
	x.recompute(t)
	snapshotA := *mustComputedAt(t, x, ctx)
	// 前 100 名在讀取時都失去資格：讀完第一頁還湊不到 limit，要翻到第二頁。
	for i := int64(1); i <= 100; i++ {
		x.f.suspend(i)
	}
	// 讀第一頁問 seller 的當下，另一個重算提交了新的快照：店 120 成為第一名（原本在第二頁）。
	x.f.onSeller = func() {
		x.f.mu.Lock()
		x.f.orders[120] = OrderStats{StoreID: 120, DistinctBuyers: 500, QualifyingOrders: 500}
		x.f.mu.Unlock()
		x.advance(time.Minute)
		r, err := x.module.RecomputeSellerScores(context.Background())
		if err != nil || !r.Written {
			t.Errorf("concurrent recompute: %+v err=%v", r, err)
		}
	}
	rec, err := x.module.RecommendedSellers(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int64, 50)
	for i := range want {
		want[i] = int64(101 + i)
	}
	if got := storeIDs(rec.Sellers); !slices.Equal(got, want) {
		t.Fatalf("read mixed two snapshots: %v", got)
	}
	if rec.ComputedAt == nil || !rec.ComputedAt.Equal(snapshotA) {
		t.Fatalf("ComputedAt=%v belongs to a different snapshot than the rows (want %v)", rec.ComputedAt, snapshotA)
	}
	// 下一次讀取看見新的快照：店 120 第一。
	next, err := x.module.RecommendedSellers(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if next.Sellers[0].StoreID != 120 || next.ComputedAt == nil || !next.ComputedAt.After(snapshotA) {
		t.Fatalf("next read: %+v computedAt=%v", next.Sellers, next.ComputedAt)
	}
}

func mustComputedAt(t *testing.T, x *fixture, ctx context.Context) *time.Time {
	t.Helper()
	rec, err := x.module.RecommendedSellers(ctx, 1)
	if err != nil || rec.ComputedAt == nil {
		t.Fatalf("no snapshot: %+v err=%v", rec, err)
	}
	return rec.ComputedAt
}
