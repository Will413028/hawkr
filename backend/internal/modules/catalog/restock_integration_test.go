//go:build integration

package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/restock"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func (f *productFixture) stocked(storeID, acct int64, title string, stock int32, active bool) (Product, uuid.UUID) {
	f.t.Helper()
	in := f.input(title)
	in.Skus[0].StockQty = stock
	p := f.create(storeID, acct, in)
	if active {
		f.activate(storeID, p.ID)
	}
	f.clearQueues()
	return p, p.Skus[0].ID
}

func (f *productFixture) clearQueues() {
	f.t.Helper()
	f.exec(`DELETE FROM catalog.restock_queue`)
	f.exec(`DELETE FROM catalog.restock_purge_queue`)
}

func (f *productFixture) queued(sku uuid.UUID) int {
	f.t.Helper()
	return f.count(`SELECT count(*) FROM catalog.restock_queue WHERE sku_id = $1`, sku)
}

type queueRow struct {
	DueAt, RequestedAt time.Time
	ClaimedUntil       *time.Time
	Attempts           int32
	LastError          *string
	FailedAt           *time.Time
}

func (f *productFixture) row(table string, sku uuid.UUID) queueRow {
	f.t.Helper()
	var r queueRow
	err := f.d.Pool().QueryRow(f.ct, `SELECT due_at, requested_at, claimed_until, attempts, last_error, failed_at FROM catalog.`+table+` WHERE sku_id = $1`, sku).
		Scan(&r.DueAt, &r.RequestedAt, &r.ClaimedUntil, &r.Attempts, &r.LastError, &r.FailedAt)
	if err != nil {
		f.t.Fatalf("%s row of %v: %v", table, sku, err)
	}
	return r
}

func (f *productFixture) arm(sku uuid.UUID) {
	f.t.Helper()
	f.exec(`INSERT INTO catalog.restock_queue (sku_id, due_at, requested_at) VALUES ($1, $2, $2)
		ON CONFLICT (sku_id) DO UPDATE SET due_at = $2, requested_at = $2, claimed_until = NULL, attempts = 0, last_error = NULL, failed_at = NULL`, sku, f.clock())
}

func (f *productFixture) run() {
	f.t.Helper()
	if err := f.m.restock.Run(f.ct); err != nil {
		f.t.Fatal(err)
	}
}

func (f *productFixture) setStock(storeID int64, sku uuid.UUID, quantity, expected int32) {
	f.t.Helper()
	if err := f.m.SetStock(f.ct, storeID, uuid.NewV7().String(), sku, quantity, expected); err != nil {
		f.t.Fatal(err)
	}
}

func TestRestockIsEnqueuedOnEveryZeroToPurchasablePath(t *testing.T) {
	t.Parallel()
	type env struct {
		f       *productFixture
		storeID int64
		sku     uuid.UUID
	}
	paths := []struct {
		name  string
		setup func(e *env)
		act   func(e *env) error
	}{
		{"SetStock", func(e *env) {}, func(e *env) error {
			return e.f.m.SetStock(e.f.ct, e.storeID, "k-set", e.sku, 5, 0)
		}},
		{"AdjustStock", func(e *env) {}, func(e *env) error {
			return e.f.m.AdjustStock(e.f.ct, e.storeID, "k-adj", e.sku, 4)
		}},
		{"RestoreStock", func(e *env) {
			e.f.setStock(e.storeID, e.sku, 3, 0)
			if err := e.f.m.DeductStock(e.f.ct, "order-1", []StockItem{{SkuID: e.sku, Quantity: 3}}); err != nil {
				e.f.t.Fatal(err)
			}
			e.f.clearQueues()
		}, func(e *env) error {
			return e.f.m.RestoreStock(e.f.ct, "restore-1", "order-1", nil)
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			f := newProductFixture(t)
			storeID, acct := f.store()
			_, sku := f.stocked(storeID, acct, "補貨", 0, true)
			e := &env{f: f, storeID: storeID, sku: sku}
			p.setup(e)
			if got := f.queued(sku); got != 0 {
				t.Fatalf("no row before the transition: %d", got)
			}
			if err := p.act(e); err != nil {
				t.Fatal(err)
			}
			if got := f.queued(sku); got != 1 {
				t.Fatalf("0 -> purchasable must enqueue exactly one row for the SKU, got %d", got)
			}
			r := f.row("restock_queue", sku)
			if want := f.clock().Add(restock.Delay); !r.DueAt.Equal(want) || r.ClaimedUntil != nil || r.Attempts != 0 || r.FailedAt != nil {
				t.Fatalf("the row is due %v after the request clock, unclaimed, no failures: %+v, want due %v", restock.Delay, r, want)
			}
		})
	}
}

func TestRestockIsEnqueuedWhenAProductBecomesActiveWithStock(t *testing.T) {
	t.Parallel()
	for _, from := range []ProductStatus{StatusDraft, StatusArchived} {
		t.Run(string(from), func(t *testing.T) {
			t.Parallel()
			f := newProductFixture(t)
			storeID, acct := f.store()
			p, sku := f.stocked(storeID, acct, "上架", 4, false)
			if from == StatusArchived {
				if err := setStatusP(f.m, f.ct, storeID, p.ID, StatusArchived); err != nil {
					t.Fatal(err)
				}
				f.clearQueues()
			}
			f.activate(storeID, p.ID)
			if got := f.queued(sku); got != 1 {
				t.Fatalf("%s -> ACTIVE with stock must enqueue a row, got %d", from, got)
			}
			f.advance(restock.Delay)
			f.run()
			if got := f.rn.skus(); !slices.Equal(got, []uuid.UUID{sku}) {
				t.Fatalf("the activated stocked SKU is notified, got %v", got)
			}
		})
	}
}

// 商品切成 ACTIVE 與另一個 SKU 的 0 到正庫存並行時，兩邊的 transaction 互相看不到對方：啟用不讀庫存，每個沒刪除的 SKU 都排，執行時再確認。
func TestActivationEnqueuesEveryLiveSkuRegardlessOfStock(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("並行啟用")
	in.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M", "L"}}}
	in.Skus = []SkuInput{{Option1: "S", Price: 100}, {Option1: "M", Price: 100}, {Option1: "L", Price: 100}}
	p := f.create(storeID, acct, in)
	f.clearQueues()
	f.activate(storeID, p.ID)
	for _, sk := range p.Skus {
		if got := f.queued(sk.ID); got != 1 {
			t.Fatalf("SKU %v has no stock but the activation must still enqueue it, got %d rows", sk.ID, got)
		}
	}
	f.setStock(storeID, p.Skus[1].ID, 6, 0)
	f.advance(restock.Delay)
	f.run()
	if got := f.rn.skus(); !slices.Equal(got, []uuid.UUID{p.Skus[1].ID}) {
		t.Fatalf("only the SKU that has stock when the worker runs is notified, got %v", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.restock_queue`); got != 0 {
		t.Fatalf("every processed row is deleted, %d left", got)
	}
}

func TestRestockIsNotEnqueuedWithoutAZeroToPurchasableTransition(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()

	_, sku := f.stocked(storeID, acct, "有貨", 5, true)
	f.setStock(storeID, sku, 7, 5)
	if err := f.m.AdjustStock(f.ct, storeID, "adj-up", sku, 3); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeductStock(f.ct, "order-pos", []StockItem{{SkuID: sku, Quantity: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.RestoreStock(f.ct, "restore-pos", "order-pos", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.queued(sku); got != 0 {
		t.Fatalf("stock > 0 to > 0 must not enqueue a row, got %d", got)
	}

	f.setStock(storeID, sku, 0, 10)
	if got := f.queued(sku); got != 0 {
		t.Fatalf("a decrease to 0 must not enqueue a row, got %d", got)
	}

	for _, st := range []ProductStatus{StatusDraft, StatusArchived} {
		p, s := f.stocked(storeID, acct, "未上架"+string(st), 0, false)
		if st == StatusArchived {
			f.activate(storeID, p.ID)
			if err := setStatusP(f.m, f.ct, storeID, p.ID, StatusArchived); err != nil {
				t.Fatal(err)
			}
			f.clearQueues()
		}
		f.setStock(storeID, s, 5, 0)
		if got := f.queued(s); got != 0 {
			t.Fatalf("a %s product is not purchasable, got %d rows", st, got)
		}
	}

	other, otherAcct := f.store()
	_, s2 := f.stocked(other, otherAcct, "不曝光", 0, true)
	f.setExposable(other, false)
	f.setStock(other, s2, 5, 0)
	if got := f.queued(s2); got != 0 {
		t.Fatalf("a store that is not exposable makes the SKU not purchasable, got %d rows", got)
	}

	p3, s3 := f.stocked(storeID, acct, "將刪除", 3, true)
	if err := f.m.DeductStock(f.ct, "order-deleted", []StockItem{{SkuID: s3, Quantity: 3}}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p3.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.clearQueues()
	if err := f.m.RestoreStock(f.ct, "restore-deleted", "order-deleted", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.queued(s3); got != 0 {
		t.Fatalf("a deleted SKU is not purchasable, got %d rows", got)
	}
}

func TestRestockIsEnqueuedInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "交易", 0, true)

	errRollback := errors.New("rollback on purpose")
	err := f.d.InTx(f.ct, func(ctx context.Context) error {
		if err := f.m.SetStock(ctx, storeID, "tx-1", sku, 5, 0); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction error = %v", err)
	}
	if got := f.queued(sku); got != 0 {
		t.Fatalf("a rolled-back transaction must leave no row, got %d", got)
	}
	var stock int32
	if err := f.d.Pool().QueryRow(f.ct, `SELECT stock_qty FROM catalog.product_skus WHERE id = $1`, sku).Scan(&stock); err != nil || stock != 0 {
		t.Fatalf("stock after rollback = %d, %v", stock, err)
	}

	if err := f.d.InTx(f.ct, func(ctx context.Context) error { return f.m.SetStock(ctx, storeID, "tx-2", sku, 5, 0) }); err != nil {
		t.Fatal(err)
	}
	if got := f.queued(sku); got != 1 {
		t.Fatalf("a committed transaction leaves the row, got %d", got)
	}
}

func TestRestockTransitionsWithinTheWindowCoalesceIntoOneNotification(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "合併", 0, true)
	f.setStock(storeID, sku, 5, 0)
	first := f.row("restock_queue", sku)
	f.advance(time.Minute)
	f.setStock(storeID, sku, 0, 5)
	f.setStock(storeID, sku, 3, 0)
	if got := f.count(`SELECT count(*) FROM catalog.restock_queue`); got != 1 {
		t.Fatalf("transitions of the same SKU share one row, got %d", got)
	}
	if r := f.row("restock_queue", sku); !r.DueAt.Equal(first.DueAt) || !r.RequestedAt.Equal(first.RequestedAt) {
		t.Fatalf("an unclaimed row is left as it is (earliest due, original request): %+v, first %+v", r, first)
	}

	f.run()
	if len(f.rn.skus()) != 0 {
		t.Fatalf("not due yet: %v", f.rn.skus())
	}
	f.advance(restock.Delay)
	f.run()
	f.run()
	if got := f.rn.skus(); !slices.Equal(got, []uuid.UUID{sku}) {
		t.Fatalf("two transitions in the window notify once, got %v", got)
	}
	if got := f.queued(sku); got != 0 {
		t.Fatalf("a notified row is deleted, got %d", got)
	}
}

// 處理中（已被認領）的列又來新轉換：完成時不能刪掉它，要重新排，第二次轉換才不會遺失。
func TestRestockTransitionWhileTheRowIsClaimedIsNotLost(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "處理中", 0, true)
	f.setStock(storeID, sku, 5, 0)
	f.advance(restock.Delay)
	f.rn.onNotify = func() {
		f.setStock(storeID, sku, 0, 5)
		f.setStock(storeID, sku, 3, 0)
	}
	f.run()
	f.rn.onNotify = nil
	if got := len(f.rn.skus()); got != 1 {
		t.Fatalf("first run notifies once, got %d", got)
	}
	if got := f.queued(sku); got != 1 {
		t.Fatalf("the transition during processing keeps the row, got %d rows", got)
	}
	r := f.row("restock_queue", sku)
	if want := f.clock().Add(restock.Delay); r.ClaimedUntil != nil || !r.DueAt.Equal(want) || r.Attempts != 0 {
		t.Fatalf("the row is re-armed (unclaimed, due %v): %+v", want, r)
	}
	f.run()
	if got := len(f.rn.skus()); got != 1 {
		t.Fatalf("the re-armed row waits out the delay, got %d notifications", got)
	}
	f.advance(restock.Delay)
	f.run()
	if got := f.rn.skus(); !slices.Equal(got, []uuid.UUID{sku, sku}) {
		t.Fatalf("the second transition is notified later, got %v", got)
	}
	if got := f.queued(sku); got != 0 {
		t.Fatalf("done, got %d rows", got)
	}
}

// worker 認領後 crash：租約沒過不重複處理，租約過了重新處理。
func TestRestockClaimedRowIsReprocessedOnlyAfterTheLeaseExpires(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "crash", 0, true)
	f.setStock(storeID, sku, 5, 0)
	f.advance(restock.Delay)
	f.exec(`UPDATE catalog.restock_queue SET claimed_until = $2 WHERE sku_id = $1`, sku, f.clock().Add(restock.Lease))

	f.run()
	if got := len(f.rn.skus()); got != 0 {
		t.Fatalf("a row under a live lease belongs to its worker, got %d notifications", got)
	}
	f.advance(restock.Lease - time.Second)
	f.run()
	if got := len(f.rn.skus()); got != 0 {
		t.Fatalf("the lease has not expired yet, got %d notifications", got)
	}
	f.advance(2 * time.Second)
	f.run()
	if got := f.rn.skus(); !slices.Equal(got, []uuid.UUID{sku}) {
		t.Fatalf("an expired lease is reclaimed and processed, got %v", got)
	}
	if got := f.queued(sku); got != 0 {
		t.Fatalf("processed, got %d rows", got)
	}
}

func TestRestockNotifierFailureBacksOffAndStopsAtMaxAttempts(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "失敗", 0, true)
	f.setStock(storeID, sku, 5, 0)
	f.advance(restock.Delay)
	f.rn.failNotify = errors.New("fanout down")

	for attempt := int32(1); attempt <= restock.MaxAttempts; attempt++ {
		f.run()
		r := f.row("restock_queue", sku)
		if r.Attempts != attempt || r.ClaimedUntil != nil || r.LastError == nil || !strings.Contains(*r.LastError, "fanout down") {
			t.Fatalf("attempt %d: the failure is recorded on the same row: %+v", attempt, r)
		}
		if want := f.clock().Add(restock.Backoff(attempt)); !r.DueAt.Equal(want) {
			t.Fatalf("attempt %d: due %v, want %v (exponential backoff)", attempt, r.DueAt, want)
		}
		if got := len(f.rn.notified); got != int(attempt) {
			t.Fatalf("attempt %d: %d calls", attempt, got)
		}
		if attempt < restock.MaxAttempts {
			f.run()
			if got := len(f.rn.notified); got != int(attempt) {
				t.Fatalf("attempt %d: retried before the backoff passed, %d calls", attempt, got)
			}
			if r.FailedAt != nil {
				t.Fatalf("attempt %d: not stopped yet: %+v", attempt, r)
			}
		}
		f.advance(restock.Backoff(attempt))
	}
	r := f.row("restock_queue", sku)
	if r.FailedAt == nil || r.Attempts != restock.MaxAttempts {
		t.Fatalf("after %d failures the row stops: %+v", restock.MaxAttempts, r)
	}
	f.advance(24 * time.Hour)
	f.run()
	if got := len(f.rn.notified); got != restock.MaxAttempts {
		t.Fatalf("a stopped row is not retried, %d calls", got)
	}

	f.rn.failNotify = nil
	f.setStock(storeID, sku, 0, 5)
	f.setStock(storeID, sku, 2, 0)
	r = f.row("restock_queue", sku)
	if r.FailedAt != nil || r.Attempts != 0 || r.LastError != nil {
		t.Fatalf("a new request resets a stopped row: %+v", r)
	}
	f.advance(restock.Delay)
	f.run()
	if got := len(f.rn.notified); got != restock.MaxAttempts+1 || f.queued(sku) != 0 {
		t.Fatalf("the reset row notifies and is deleted: %d calls, %d rows", got, f.queued(sku))
	}
}

func TestRestockRechecksPurchasabilityWhenItRuns(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p, sku := f.stocked(storeID, acct, "重新確認", 0, true)
	f.setStock(storeID, sku, 5, 0)

	cases := []struct {
		name   string
		change func()
		undo   func()
	}{
		{"stock back to 0", func() { f.setStock(storeID, sku, 0, 5) }, func() { f.setStock(storeID, sku, 5, 0) }},
		{"product archived", func() {
			if err := setStatusP(f.m, f.ct, storeID, p.ID, StatusArchived); err != nil {
				t.Fatal(err)
			}
		}, func() { f.activate(storeID, p.ID) }},
		{"store not exposable", func() { f.setExposable(storeID, false) }, func() { f.setExposable(storeID, true) }},
		{"product deleted", func() {
			if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
				t.Fatal(err)
			}
		}, func() {}},
	}
	for _, c := range cases {
		f.clearQueues()
		f.arm(sku)
		c.change()
		f.run()
		if got := f.rn.skus(); len(got) != 0 {
			t.Fatalf("%s: a row whose SKU is not purchasable must not notify, got %v", c.name, got)
		}
		if got := f.queued(sku); got != 0 {
			t.Fatalf("%s: the row is deleted, got %d", c.name, got)
		}
		c.undo()
	}
}

func TestRestockNotifiesWithTheLatestSnapshotOfTheSkuThatChanged(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("舊名")
	in.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M"}}}
	in.Skus = []SkuInput{{Option1: "S", Price: 100}, {Option1: "M", Price: 100}}
	p := f.create(storeID, acct, in)
	f.activate(storeID, p.ID)
	restocked, sibling := p.Skus[0].ID, p.Skus[1].ID
	f.clearQueues()

	f.setStock(storeID, restocked, 5, 0)
	if f.queued(sibling) != 0 || f.queued(restocked) != 1 {
		t.Fatalf("only the restocked SKU gets a row: sibling %d, restocked %d", f.queued(sibling), f.queued(restocked))
	}

	in2 := in
	in2.Title = "新名"
	in2.Gallery = []Image{{Token: "restock-new-cover"}}
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, in2); err != nil {
		t.Fatal(err)
	}
	f.advance(restock.Delay)
	f.run()
	if len(f.rn.notified) != 1 {
		t.Fatalf("one notification, got %+v", f.rn.notified)
	}
	got := f.rn.notified[0]
	var coverKey string
	if err := f.d.Pool().QueryRow(f.ct, `SELECT object_key FROM catalog.product_images WHERE product_id = $1 AND kind = 'GALLERY' ORDER BY position LIMIT 1`, p.ID).Scan(&coverKey); err != nil {
		t.Fatal(err)
	}
	if got.SKUID != restocked || got.ProductID != p.ID || got.StoreID != storeID || got.Name != "新名" || got.ImageKey != coverKey || coverKey == "" || !got.OccurredAt.Equal(f.clock()) {
		t.Fatalf("notification = %+v, want sku %v product %v store %d name 新名 image %q at %v", got, restocked, p.ID, storeID, coverKey, f.clock())
	}

	f.arm(sibling)
	f.run()
	if !slices.Equal(f.rn.skus(), []uuid.UUID{restocked}) {
		t.Fatalf("only the restocked SKU is notified, got %v", f.rn.skus())
	}
}

func TestDeletingAProductOrSkuEnqueuesAPurgeOfItsSubscriptions(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("清訂閱")
	in.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M"}}}
	in.Skus = []SkuInput{{Option1: "S", Price: 100}, {Option1: "M", Price: 100}}
	p := f.create(storeID, acct, in)
	f.clearQueues()

	purged := func() []uuid.UUID {
		f.t.Helper()
		f.rn.purged = nil
		f.run()
		var out []uuid.UUID
		for _, ids := range f.rn.purged {
			out = append(out, ids...)
		}
		return out
	}
	queuedPurges := func() int { return f.count(`SELECT count(*) FROM catalog.restock_purge_queue`) }

	keep := in
	keep.Axes = []Axis{{Name: "尺寸", Values: []string{"S"}}}
	keep.Skus = []SkuInput{{Option1: "S", Price: 100}}
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, keep); err != nil {
		t.Fatal(err)
	}
	var removed, kept uuid.UUID
	for _, sk := range p.Skus {
		if sk.Option1 == "M" {
			removed = sk.ID
		} else {
			kept = sk.ID
		}
	}
	if got := f.count(`SELECT count(*) FROM catalog.restock_purge_queue WHERE sku_id = $1`, removed); got != 1 || queuedPurges() != 1 {
		t.Fatalf("removing a SKU enqueues exactly that SKU's purge, got %d (total %d)", got, queuedPurges())
	}
	if got := purged(); !slices.Equal(got, []uuid.UUID{removed}) {
		t.Fatalf("the worker purges exactly the removed SKU, got %v", got)
	}
	if got := queuedPurges(); got != 0 {
		t.Fatalf("a purged row is deleted, got %d", got)
	}

	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, keep); err != nil {
		t.Fatal(err)
	}
	if got := queuedPurges(); got != 0 {
		t.Fatalf("an update that keeps every SKU purges nothing, got %d", got)
	}

	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.rn.failPurge = errors.New("purge down")
	if got := purged(); !slices.Equal(got, []uuid.UUID{kept}) {
		t.Fatalf("deleting the product purges its remaining SKU, got %v", got)
	}
	r := f.row("restock_purge_queue", kept)
	if r.Attempts != 1 || r.ClaimedUntil != nil || r.LastError == nil || !r.DueAt.Equal(f.clock().Add(restock.Backoff(1))) {
		t.Fatalf("a failed purge stays on its row with a backoff: %+v", r)
	}
	f.rn.failPurge = nil
	f.advance(restock.Backoff(1))
	if got := purged(); !slices.Equal(got, []uuid.UUID{kept}) || queuedPurges() != 0 {
		t.Fatalf("the retry purges and deletes the row, got %v (%d rows left)", got, queuedPurges())
	}
}
