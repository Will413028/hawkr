//go:build integration

package catalog

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var epoch = time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

type fixture struct {
	t              *testing.T
	d              *db.DB
	m              *Module
	ct             context.Context
	rn             *recordingNotifier
	products       *productFixture
	storeID        int64
	accountID      int64
	otherStoreID   int64
	otherAccountID int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	products := newProductFixture(t)
	accountID, err := sellerfixture.NewAccount(products.ct, products.d, epoch)
	if err != nil {
		t.Fatal(err)
	}
	storeID, err := sellerfixture.ExposableStore(products.ct, products.d, accountID, epoch)
	if err != nil {
		t.Fatal(err)
	}
	otherAccountID, err := sellerfixture.NewAccount(products.ct, products.d, epoch)
	if err != nil {
		t.Fatal(err)
	}
	otherStoreID, err := sellerfixture.ExposableStore(products.ct, products.d, otherAccountID, epoch)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{
		t: t, d: products.d, m: products.m, ct: products.ct, rn: products.rn, products: products,
		storeID: storeID, accountID: accountID, otherStoreID: otherStoreID, otherAccountID: otherAccountID,
	}
}

func (f *fixture) exec(sql string, args ...any) error {
	_, err := f.d.Pool().Exec(f.ct, sql, args...)
	return err
}

func (f *fixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) sku(qty int) uuid.UUID {
	f.t.Helper()
	in := f.products.input("庫存測試")
	in.Skus[0].StockQty = int32(qty)
	p := f.products.create(f.storeID, f.accountID, in)
	f.products.activate(f.storeID, p.ID)
	return p.Skus[0].ID
}

func (f *fixture) stockOf(sku uuid.UUID) int {
	f.t.Helper()
	var n int
	f.must(f.d.Pool().QueryRow(f.ct, `SELECT stock_qty FROM catalog.product_skus WHERE id = $1`, sku).Scan(&n))
	return n
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	f.must(f.d.Pool().QueryRow(f.ct, sql, args...).Scan(&n))
	return n
}

func items(pairs ...any) []StockItem {
	var out []StockItem
	for c := range slices.Chunk(pairs, 2) {
		out = append(out, StockItem{SkuID: c[0].(uuid.UUID), Quantity: int32(c[1].(int))})
	}
	return out
}

func wantConstraint(t *testing.T, err error, name string) {
	t.Helper()
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.ConstraintName != name {
		t.Fatalf("要被 %s 擋下，實際：%v", name, err)
	}
}

// 同一個冪等鍵併發重送只扣一次。
func TestConcurrentSameKeyDeductsOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)
	const n = 16
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.m.DeductStock(f.ct, "order:A", items(sku, 3))
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("重送 %d 要回成功：%v", i, err)
		}
	}
	if got := f.stockOf(sku); got != 7 {
		t.Fatalf("庫存 = %d，want 7（只扣一次）", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_movements WHERE sku_id = $1`, sku); got != 1 {
		t.Fatalf("帳本 %d 筆，want 1", got)
	}
}

func TestReplaySameKeyIsNoOpAndDifferentContentIsRejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b := f.sku(10), f.sku(10)
	f.must(f.m.DeductStock(f.ct, "k", items(a, 2, b, 1)))
	f.must(f.m.DeductStock(f.ct, "k", items(b, 1, a, 2)))
	if f.stockOf(a) != 8 || f.stockOf(b) != 9 {
		t.Fatalf("重送不得再扣：a=%d b=%d", f.stockOf(a), f.stockOf(b))
	}
	for name, in := range map[string][]StockItem{"數量不同": items(a, 3, b, 1), "品項不同": items(a, 2), "SKU 不同": items(a, 2, f.sku(5), 1)} {
		if err := f.m.DeductStock(f.ct, "k", in); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("%s：err = %v，want ErrIdempotencyKeyReused", name, err)
		}
	}
	if f.stockOf(a) != 8 {
		t.Fatalf("被拒的重送不得改庫存：%d", f.stockOf(a))
	}
	if err := f.m.RestoreStock(f.ct, "k", "k", nil); !errors.Is(err, ErrDeductionNotFound) && !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("以扣減的鍵歸還自己：%v", err)
	}
}

// 庫存不足：整次不生效（含前面已扣的 SKU 與操作列），庫存不為負；補貨後同一個鍵可以重試。
func TestInsufficientStockRollsBackTheWholeOperation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b := f.sku(5), f.sku(1)
	err := f.m.DeductStock(f.ct, "o1", items(a, 2, b, 2))
	var ins *InsufficientStockError
	if !errors.As(err, &ins) {
		t.Fatalf("err = %v，want InsufficientStockError", err)
	}
	if f.stockOf(a) != 5 || f.stockOf(b) != 1 {
		t.Fatalf("整次不生效：a=%d b=%d", f.stockOf(a), f.stockOf(b))
	}
	if f.count(`SELECT count(*) FROM catalog.stock_operations`) != 0 {
		t.Fatal("失敗的操作不得留下帳本")
	}
	f.must(f.exec(`UPDATE catalog.product_skus SET stock_qty = 2 WHERE id = $1`, b))
	f.must(f.m.DeductStock(f.ct, "o1", items(a, 2, b, 2)))
	if f.stockOf(a) != 3 || f.stockOf(b) != 0 {
		t.Fatalf("補貨後重試：a=%d b=%d", f.stockOf(a), f.stockOf(b))
	}
}

// 不同的鍵搶有限的庫存：剛好賣完，不超賣。
func TestConcurrentDifferentKeysNeverOversell(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(5)
	const n = 20
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f.m.DeductStock(f.ct, "order:"+uuid.NewV7().String(), items(sku, 1))
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		var ins *InsufficientStockError
		switch {
		case err == nil:
			ok++
		case !errors.As(err, &ins):
			t.Errorf("非預期的錯誤：%v", err)
		}
	}
	if ok != 5 || f.stockOf(sku) != 0 {
		t.Fatalf("成功 %d 筆、庫存 %d，want 5 與 0", ok, f.stockOf(sku))
	}
}

// 兩個多品項扣減以相反順序列出同兩個 SKU：依 ID 排序鎖定，不死結。
func TestMultiItemDeductsInOppositeOrderDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b := f.sku(1000), f.sku(1000)
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in := items(a, 1, b, 1)
			if i%2 == 1 {
				in = items(b, 1, a, 1)
			}
			if err := f.m.DeductStock(f.ct, "d"+uuid.NewV7().String(), in); err != nil {
				t.Errorf("扣減失敗：%v", err)
			}
		}()
	}
	wg.Wait()
	if f.stockOf(a) != 960 || f.stockOf(b) != 960 {
		t.Fatalf("a=%d b=%d", f.stockOf(a), f.stockOf(b))
	}
}

func TestDeductRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := f.sku(5)
	for name, c := range map[string]struct {
		key string
		in  []StockItem
	}{
		"空鍵":     {"", items(a, 1)},
		"頭尾空白":   {" k", items(a, 1)},
		"沒有品項":   {"k", nil},
		"數量為 0":  {"k", items(a, 0)},
		"負數":     {"k", items(a, -1)},
		"重複 SKU": {"k", items(a, 1, a, 1)},
	} {
		if err := f.m.DeductStock(f.ct, c.key, c.in); !errors.Is(err, ErrInvalidStockInput) {
			t.Errorf("%s：err = %v", name, err)
		}
	}
	if f.stockOf(a) != 5 {
		t.Fatal("無效輸入不得改庫存")
	}
	if err := f.m.DeductStock(f.ct, "k", items(uuid.NewV7(), 1)); !errors.Is(err, ErrSkuNotFound) {
		t.Errorf("不存在的 SKU：%v", err)
	}
}

// 在呼叫方的 transaction 內扣減：呼叫方回滾時庫存與帳本一起回滾。
func TestDeductJoinsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(5)
	boom := errors.New("boom")
	err := f.d.InTx(f.ct, func(ctx context.Context) error {
		if err := f.m.DeductStock(ctx, "tx", items(sku, 2)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || f.stockOf(sku) != 5 || f.count(`SELECT count(*) FROM catalog.stock_operations`) != 0 {
		t.Fatalf("err=%v stock=%d", err, f.stockOf(sku))
	}
	f.must(f.d.InTx(f.ct, func(ctx context.Context) error { return f.m.DeductStock(ctx, "tx", items(sku, 2)) }))
	if f.stockOf(sku) != 3 {
		t.Fatalf("stock = %d", f.stockOf(sku))
	}
}

func TestDeletedSkuCannotBeDeductedButCanBeRestored(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(5)
	f.must(f.m.DeductStock(f.ct, "d", items(sku, 2)))
	f.must(f.exec(`UPDATE catalog.product_skus SET deleted_at = $2 WHERE id = $1`, sku, epoch))
	if err := f.m.DeductStock(f.ct, "d2", items(sku, 1)); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("已刪除的 SKU：%v", err)
	}
	f.must(f.m.RestoreStock(f.ct, "r", "d", nil))
	if f.stockOf(sku) != 5 {
		t.Fatalf("已刪除的 SKU 仍要記回庫存：%d", f.stockOf(sku))
	}
}

// 歸還依原扣減，累計不超過原扣減量。
func TestRestoreFollowsTheOriginalDeduction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b, other := f.sku(10), f.sku(10), f.sku(10)
	f.must(f.m.DeductStock(f.ct, "d", items(a, 4, b, 3)))

	if err := f.m.RestoreStock(f.ct, "r0", "nope", nil); !errors.Is(err, ErrDeductionNotFound) {
		t.Errorf("不存在的扣減：%v", err)
	}
	if err := f.m.RestoreStock(f.ct, "r0", "d", items(a, 5)); !errors.Is(err, ErrRestoreExceedsDeduction) {
		t.Errorf("超過原扣減：%v", err)
	}
	if err := f.m.RestoreStock(f.ct, "r0", "d", items(other, 1)); !errors.Is(err, ErrRestoreExceedsDeduction) {
		t.Errorf("原扣減沒有的 SKU：%v", err)
	}
	if f.stockOf(a) != 6 || f.stockOf(other) != 10 {
		t.Fatalf("被拒的歸還不得改庫存：a=%d other=%d", f.stockOf(a), f.stockOf(other))
	}

	f.must(f.m.RestoreStock(f.ct, "r1", "d", items(a, 3)))
	f.must(f.m.RestoreStock(f.ct, "r1", "d", items(a, 3)))
	if err := f.m.RestoreStock(f.ct, "r2", "d", items(a, 2)); !errors.Is(err, ErrRestoreExceedsDeduction) {
		t.Errorf("累計 3+2 超過 4：%v", err)
	}
	if err := f.m.RestoreStock(f.ct, "r1", "d", items(a, 1)); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("同鍵不同量：%v", err)
	}
	f.must(f.m.RestoreStock(f.ct, "r3", "d", nil))
	if f.stockOf(a) != 10 || f.stockOf(b) != 10 {
		t.Fatalf("a=%d b=%d，want 10 與 10", f.stockOf(a), f.stockOf(b))
	}
	if err := f.m.RestoreStock(f.ct, "r4", "d", nil); !errors.Is(err, ErrRestoreExceedsDeduction) {
		t.Errorf("已全部歸還：%v", err)
	}
}

// 同一次扣減的併發歸還（不同鍵）：總量不超過原扣減。
func TestConcurrentRestoresNeverExceedTheDeduction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)
	f.must(f.m.DeductStock(f.ct, "d", items(sku, 6)))
	const n = 12
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f.m.RestoreStock(f.ct, "r"+uuid.NewV7().String(), "d", items(sku, 2))
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrRestoreExceedsDeduction):
			t.Errorf("非預期的錯誤：%v", err)
		}
	}
	if ok != 3 || f.stockOf(sku) != 10 {
		t.Fatalf("成功 %d 筆、庫存 %d，want 3 與 10", ok, f.stockOf(sku))
	}
}

// 歸還先鎖原扣減那一列再算剩餘量：別的 transaction 持有該列的 FOR NO KEY UPDATE 時（與 FOR UPDATE 衝突、與插入歸還列的外鍵檢查所取的
// FOR KEY SHARE 不衝突），歸還必須等它；沒鎖原扣減的實作會直接做完。以 pg_blocking_pids 等到它被擋住，不靠併發的時序。
func TestRestoreWaitsForTheLockOnTheDeduction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)
	f.must(f.m.DeductStock(f.ct, "d", items(sku, 6)))
	holder, err := f.d.Pool().Begin(f.ct)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(f.ct) }()
	if _, err := holder.Exec(f.ct, `SELECT id FROM catalog.stock_operations WHERE kind = 'DEDUCT' AND idempotency_key = 'd' FOR NO KEY UPDATE`); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := holder.QueryRow(f.ct, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- f.m.RestoreStock(f.ct, "r", "d", items(sku, 2)) }()
	deadline := time.Now().Add(30 * time.Second)
	for blocked := false; !blocked; {
		select {
		case err := <-done:
			t.Fatalf("restore finished (err=%v) without waiting for the lock on the deduction", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("restore never blocked on the deduction row")
		}
		blocked = f.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND $1::int = ANY(pg_blocking_pids(pid))`, pid) > 0
		if !blocked {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := holder.Rollback(f.ct); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := f.stockOf(sku); got != 6 {
		t.Fatalf("庫存 = %d，want 6（扣 6、還 2）", got)
	}
}

func TestStockCannotGoNegativeAtTheSchemaLevel(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(1)
	wantConstraint(t, f.exec(`UPDATE catalog.product_skus SET stock_qty = -1 WHERE id = $1`, sku), "skus_stock_check")
}

func TestSellerAdjustMovesStockAndWritesTheLedger(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(5)
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:1", sku, 7))
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:2", sku, -4))
	if got := f.stockOf(sku); got != 8 {
		t.Fatalf("庫存 = %d，want 8", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_operations o JOIN catalog.stock_movements m ON m.operation_id = o.id
		WHERE o.kind = 'ADJUST' AND m.sku_id = $1`, sku); got != 2 {
		t.Fatalf("ADJUST 帳本 %d 筆，want 2", got)
	}
}

func TestAdjustReplayIsNoOpAndDifferentContentIsRejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku, other := f.sku(5), f.sku(5)
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:r", sku, 3))
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:r", sku, 3))
	if got := f.stockOf(sku); got != 8 {
		t.Fatalf("重送後庫存 = %d，want 8（只生效一次）", got)
	}
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:r", sku, 4); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("不同變動量重送 err = %v，want ErrIdempotencyKeyReused", err)
	}
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:r", other, 3); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("不同 SKU 重送 err = %v，want ErrIdempotencyKeyReused", err)
	}
	if err := f.m.AdjustStock(f.ct, f.otherStoreID, "adj:r", sku, 3); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("別家店用同一個 key 調整這個 SKU err = %v，want ErrSkuNotFound", err)
	}
	foreign := f.sku(5)
	f.must(f.exec(`UPDATE catalog.products SET store_id = $2 WHERE id = (SELECT product_id FROM catalog.product_skus WHERE id = $1)`, foreign, f.otherStoreID))
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:r", foreign, 3); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("同店同一個 key 重送別家店的 SKU err = %v，want ErrSkuNotFound（不是 ErrIdempotencyKeyReused）", err)
	}
	if got := f.stockOf(sku); got != 8 {
		t.Fatalf("跨店重送後庫存 = %d，want 8", got)
	}
	f.must(f.m.DeductStock(f.ct, "adj:d", items(sku, 1)))
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:d", sku, -1))
	f.must(f.m.DeductStock(f.ct, "adj:d", items(sku, 1)))
	if got := f.stockOf(sku); got != 6 {
		t.Fatalf("結帳鍵與賣家調整鍵各自獨立：庫存 = %d，want 6（扣 1、調整 -1、重送不再扣）", got)
	}
}

// 賣家調整的冪等鍵只在店家內唯一：別家店用同一個鍵各自生效，也不會因此讀到或擋到對方的操作。
func TestAdjustKeyIsScopedPerStore(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b := f.sku(5), f.sku(5)
	f.must(f.exec(`UPDATE catalog.products SET store_id = $2 WHERE id = (SELECT product_id FROM catalog.product_skus WHERE id = $1)`, b, f.otherStoreID))
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:shared", a, 3))
	f.must(f.m.AdjustStock(f.ct, f.otherStoreID, "adj:shared", b, 7))
	if got := f.stockOf(a); got != 8 {
		t.Fatalf("店家 %d 的 SKU 庫存 = %d，want 8", f.storeID, got)
	}
	if got := f.stockOf(b); got != 12 {
		t.Fatalf("店家 %d 的 SKU 庫存 = %d，want 12（同一個鍵在別家店獨立生效）", f.otherStoreID, got)
	}
	f.must(f.m.AdjustStock(f.ct, f.otherStoreID, "adj:shared", b, 7))
	if err := f.m.AdjustStock(f.ct, f.otherStoreID, "adj:shared", b, 1); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("同店不同內容重送 err = %v，want ErrIdempotencyKeyReused", err)
	}
	if got := f.stockOf(b); got != 12 {
		t.Fatalf("重送後庫存 = %d，want 12", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE idempotency_key = 'adj:shared'`); got != 2 {
		t.Fatalf("操作列 %d 筆，want 2（每家店一列）", got)
	}
}

func TestConcurrentSameKeyAdjustsOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(1)
	const n = 12
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.m.AdjustStock(f.ct, f.storeID, "adj:c", sku, 5)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("重送 %d 要回成功：%v", i, err)
		}
	}
	if got := f.stockOf(sku); got != 6 {
		t.Fatalf("庫存 = %d，want 6（只調整一次）", got)
	}
}

func TestAdjustNeverGoesNegativeAndRollsBackTheOperation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(3)
	var short *InsufficientStockError
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:neg", sku, -4); !errors.As(err, &short) || short.SkuID != sku {
		t.Fatalf("err = %v，want *InsufficientStockError", err)
	}
	if got := f.stockOf(sku); got != 3 {
		t.Fatalf("庫存 = %d，want 3（沒有生效）", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE idempotency_key = 'adj:neg'`); got != 0 {
		t.Fatalf("失敗的調整留下 %d 列操作，want 0", got)
	}
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:neg", sku, -3))
	if got := f.stockOf(sku); got != 0 {
		t.Fatalf("庫存 = %d，want 0", got)
	}
}

func TestAdjustOverflowIsRejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(2147483647)
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:big", sku, 1); !errors.Is(err, ErrInvalidStockInput) {
		t.Fatalf("err = %v，want ErrInvalidStockInput", err)
	}
	if got := f.stockOf(sku); got != 2147483647 {
		t.Fatalf("庫存 = %d，want 不變", got)
	}
}

func TestAdjustOnAnotherStoresSkuOrDeletedSkuIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(5)
	if err := f.m.AdjustStock(f.ct, f.otherStoreID, "adj:x", sku, 10); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("跨店調整 err = %v，want ErrSkuNotFound", err)
	}
	if got := f.stockOf(sku); got != 5 {
		t.Fatalf("庫存 = %d，want 5（跨店調整不得生效）", got)
	}
	f.must(f.exec(`UPDATE catalog.product_skus SET deleted_at = $2 WHERE id = $1`, sku, epoch))
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:del", sku, 1); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("已刪除 SKU err = %v，want ErrSkuNotFound", err)
	}
	if err := f.m.AdjustStock(f.ct, f.storeID, "adj:none", uuid.NewV7(), 1); !errors.Is(err, ErrSkuNotFound) {
		t.Fatalf("不存在的 SKU err = %v，want ErrSkuNotFound", err)
	}
}
