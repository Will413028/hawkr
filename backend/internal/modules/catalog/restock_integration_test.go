//go:build integration

package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/restock"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	kindSku   = "catalog.restock_sku"
	kindStore = "catalog.restock_store"
	kindPurge = "catalog.restock_purge"
)

// stocked 建一個商品（一個 SKU，庫存 stock），active 為真就切成 ACTIVE，回傳商品與它的 SKU。建立與上架可能排出的 job 都清掉，測試從乾淨的 job 表開始。
func (f *productFixture) stocked(storeID, acct int64, title string, stock int32, active bool) (Product, uuid.UUID) {
	f.t.Helper()
	in := f.input(title)
	in.Skus[0].StockQty = stock
	p := f.create(storeID, acct, in)
	if active {
		f.activate(storeID, p.ID)
	}
	f.clearJobs()
	return p, p.Skus[0].ID
}

func (f *productFixture) clearJobs() {
	f.t.Helper()
	f.exec(`DELETE FROM river.river_job`)
	f.exec(`DELETE FROM catalog.restock_pending`)
}

// jobsOf 回傳這個種類的 job 數，idKey 與 id 非空時只算參數裡 idKey 等於 id 的。
func (f *productFixture) jobsOf(kind string, idKey string, id any) int {
	f.t.Helper()
	if idKey == "" {
		return f.count(`SELECT count(*) FROM river.river_job WHERE kind = $1`, kind)
	}
	return f.count(`SELECT count(*) FROM river.river_job WHERE kind = $1 AND args ->> $2 = $3::text`, kind, idKey, fmt.Sprint(id))
}

func (f *productFixture) skuJobs(sku uuid.UUID) int { return f.jobsOf(kindSku, "sku_id", sku) }

func (f *productFixture) setStock(storeID int64, sku uuid.UUID, quantity, expected int32) {
	f.t.Helper()
	if err := f.m.SetStock(f.ct, storeID, uuid.NewV7().String(), sku, quantity, expected); err != nil {
		f.t.Fatal(err)
	}
}

func TestRestockJobIsEnqueuedOnEveryZeroToPurchasablePath(t *testing.T) {
	t.Parallel()
	type env struct {
		f       *productFixture
		storeID int64
		sku     uuid.UUID
	}
	paths := []struct {
		name  string
		setup func(e *env) // 讓 SKU 在 ACTIVE、可曝光的店家處於庫存 0（或要上架的草稿）
		act   func(e *env) error
	}{
		{"SetStock", func(e *env) {}, func(e *env) error {
			return e.f.m.SetStock(e.f.ct, e.storeID, "k-set", e.sku, 5, 0)
		}},
		{"AdjustStock", func(e *env) {}, func(e *env) error {
			return e.f.m.AdjustStock(e.f.ct, e.storeID, "k-adj", e.sku, 4)
		}},
		{"RestoreStock", func(e *env) {
			// 先有一次把庫存扣到 0 的扣減，再歸還它。
			e.f.setStock(e.storeID, e.sku, 3, 0)
			if err := e.f.m.DeductStock(e.f.ct, "order-1", []StockItem{{SkuID: e.sku, Quantity: 3}}); err != nil {
				e.f.t.Fatal(err)
			}
			e.f.clearJobs()
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
			if got := f.skuJobs(sku); got != 0 {
				t.Fatalf("no job before the transition: %d", got)
			}
			if err := p.act(e); err != nil {
				t.Fatal(err)
			}
			if got := f.skuJobs(sku); got != 1 {
				t.Fatalf("0 -> purchasable must enqueue exactly one job for the SKU, got %d", got)
			}
			var at time.Time
			if err := f.d.Pool().QueryRow(f.ct, `SELECT scheduled_at FROM river.river_job WHERE kind = $1`, kindSku).Scan(&at); err != nil {
				t.Fatal(err)
			}
			if want := f.clock().Add(restock.Delay); !at.Equal(want) {
				t.Fatalf("the job is delayed by %v from the request clock: scheduled_at = %v, want %v", restock.Delay, at, want)
			}
		})
	}
}

func TestRestockJobIsEnqueuedWhenAProductBecomesActiveWithStock(t *testing.T) {
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
				f.clearJobs()
			}
			f.activate(storeID, p.ID)
			if got := f.skuJobs(sku); got != 1 {
				t.Fatalf("%s -> ACTIVE with stock must enqueue a job, got %d", from, got)
			}
		})
	}
}

func TestRestockJobIsNotEnqueuedWithoutAZeroToPurchasableTransition(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()

	// 庫存 >0 到 >0：三條增加的路徑都不排。
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
	if got := f.skuJobs(sku); got != 0 {
		t.Fatalf("stock > 0 to > 0 must not enqueue, got %d", got)
	}

	// 庫存減少（到 0）不排。
	f.setStock(storeID, sku, 0, 10)
	if got := f.skuJobs(sku); got != 0 {
		t.Fatalf("a decrease to 0 must not enqueue, got %d", got)
	}

	// 草稿與已下架的商品庫存 0 -> >0：不可買，不排。
	for _, st := range []ProductStatus{StatusDraft, StatusArchived} {
		p, s := f.stocked(storeID, acct, "未上架"+string(st), 0, false)
		if st == StatusArchived {
			f.activate(storeID, p.ID)
			if err := setStatusP(f.m, f.ct, storeID, p.ID, StatusArchived); err != nil {
				t.Fatal(err)
			}
		}
		f.setStock(storeID, s, 5, 0)
		if got := f.skuJobs(s); got != 0 {
			t.Fatalf("a %s product is not purchasable, got %d jobs", st, got)
		}
	}

	// 店家不可曝光：不可買，不排。
	other, otherAcct := f.store()
	_, s2 := f.stocked(other, otherAcct, "不曝光", 0, true)
	f.setExposable(other, false)
	f.setStock(other, s2, 5, 0)
	if got := f.skuJobs(s2); got != 0 {
		t.Fatalf("a store that is not exposable makes the SKU not purchasable, got %d jobs", got)
	}

	// 已刪除的商品：SKU 還能歸還庫存（原扣減仍可歸還），但不可買，不排。
	p3, s3 := f.stocked(storeID, acct, "將刪除", 3, true)
	if err := f.m.DeductStock(f.ct, "order-deleted", []StockItem{{SkuID: s3, Quantity: 3}}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p3.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.clearJobs()
	if err := f.m.RestoreStock(f.ct, "restore-deleted", "order-deleted", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.skuJobs(s3); got != 0 {
		t.Fatalf("a deleted SKU is not purchasable, got %d jobs", got)
	}
}

func TestRestockJobIsEnqueuedInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "交易", 0, true)

	errRollback := errors.New("rollback on purpose")
	err := f.d.InTx(f.ct, func(ctx context.Context) error {
		if err := f.m.SetStock(ctx, storeID, "tx-1", sku, 5, 0); err != nil {
			return err
		}
		// 同一個 transaction 內已經看得到排入的 job（和庫存一起在途），回滾後兩者都沒有。
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction error = %v", err)
	}
	if got := f.skuJobs(sku); got != 0 {
		t.Fatalf("a rolled-back transaction must leave no job, got %d", got)
	}
	var stock int32
	if err := f.d.Pool().QueryRow(f.ct, `SELECT stock_qty FROM catalog.product_skus WHERE id = $1`, sku).Scan(&stock); err != nil || stock != 0 {
		t.Fatalf("stock after rollback = %d, %v", stock, err)
	}

	if err := f.d.InTx(f.ct, func(ctx context.Context) error { return f.m.SetStock(ctx, storeID, "tx-2", sku, 5, 0) }); err != nil {
		t.Fatal(err)
	}
	if got := f.skuJobs(sku); got != 1 {
		t.Fatalf("a committed transaction leaves the job, got %d", got)
	}
}

func TestRestockJobsOfTheSameSkuMergeWhileNotStarted(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "合併", 0, true)
	f.setStock(storeID, sku, 5, 0)
	f.setStock(storeID, sku, 0, 5)
	f.setStock(storeID, sku, 3, 0)
	if got := f.skuJobs(sku); got != 1 {
		t.Fatalf("two transitions before the job starts merge into one job, got %d", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.restock_pending WHERE job_key = $1`, "sku:"+sku.String()); got != 1 {
		t.Fatalf("one pending marker stands for the not-yet-started job, got %d", got)
	}
}

// 一個 SKU 的 job 正在執行（River 的 running）時又發生 0 到大於 0 的轉換：它不能被執行中的 job 吞掉（D26：只合併還沒開始的 job）。
// 做法：第一個 job 標成 running，通知呼叫（發生在 job 開始之後）內做第二次轉換；第二次轉換必須排出自己的 job。
func TestRestockTransitionWhileTheSkuJobIsRunningIsNotLost(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "執行中", 0, true)
	f.setStock(storeID, sku, 5, 0)
	f.exec(`UPDATE river.river_job SET state = 'running', attempt = 1, attempted_at = now(), attempted_by = ARRAY['test'] WHERE kind = $1`, kindSku)
	f.rn.onNotify = func() {
		f.setStock(storeID, sku, 0, 5)
		f.setStock(storeID, sku, 3, 0)
	}
	if err := f.m.restock.RunSku(f.ct, sku); err != nil {
		t.Fatal(err)
	}
	if got := f.skuJobs(sku); got != 2 {
		t.Fatalf("the transition during the running job enqueues its own job: running one + new one = 2, got %d", got)
	}
	if got := f.count(`SELECT count(*) FROM river.river_job WHERE kind = $1 AND state <> 'running'`, kindSku); got != 1 {
		t.Fatalf("exactly one not-started job carries the second transition, got %d", got)
	}
	f.rn.onNotify = nil
	if err := f.m.restock.RunSku(f.ct, sku); err != nil {
		t.Fatal(err)
	}
	if got := f.rn.skus(); len(got) != 2 {
		t.Fatalf("the second job notifies again for the second transition, got %d notifications", len(got))
	}
}

// 店家層的 job 同樣只合併還沒開始的：開始（消耗標記）之後的再次曝光排自己的 job。
func TestStoreExposureAfterTheStoreJobStartedEnqueuesANewJob(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, _ := f.store()
	for range 2 {
		if err := f.m.restock.StoreExposed(f.ct, storeID); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.jobsOf(kindStore, "store_id", storeID); got != 1 {
		t.Fatalf("exposures before the store job starts merge, got %d", got)
	}
	if err := f.m.restock.RunStore(f.ct, storeID); err != nil {
		t.Fatal(err)
	}
	if err := f.m.restock.StoreExposed(f.ct, storeID); err != nil {
		t.Fatal(err)
	}
	if got := f.jobsOf(kindStore, "store_id", storeID); got != 2 {
		t.Fatalf("an exposure after the job started enqueues a new job, got %d", got)
	}
}

func TestRestockJobNotifiesWithTheLatestSnapshotOfTheSkuThatChanged(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := f.input("舊名")
	in.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M"}}}
	in.Skus = []SkuInput{{Option1: "S", Price: 100}, {Option1: "M", Price: 100}}
	p := f.create(storeID, acct, in)
	f.activate(storeID, p.ID)
	restocked, sibling := p.Skus[0].ID, p.Skus[1].ID
	f.clearJobs()

	f.setStock(storeID, restocked, 5, 0)
	if f.skuJobs(sibling) != 0 || f.skuJobs(restocked) != 1 {
		t.Fatalf("only the restocked SKU gets a job: sibling %d, restocked %d", f.skuJobs(sibling), f.skuJobs(restocked))
	}

	// job 執行前商品改名換主圖：通知帶的是執行當下的最新快照。
	in2 := in
	in2.Title = "新名"
	in2.Gallery = []Image{{Token: "restock-new-cover"}}
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, in2); err != nil {
		t.Fatal(err)
	}
	f.advance(restock.Delay)
	if err := f.m.restock.RunSku(f.ct, restocked); err != nil {
		t.Fatal(err)
	}
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

	// 同商品的另一個 SKU 還是 0：它的 job（就算被誤排）也不通知。
	if err := f.m.restock.RunSku(f.ct, sibling); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.rn.skus(), []uuid.UUID{restocked}) {
		t.Fatalf("only the restocked SKU is notified, got %v", f.rn.skus())
	}
}

func TestRestockJobRechecksPurchasabilityWhenItRuns(t *testing.T) {
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
	}
	for _, c := range cases {
		c.change()
		if err := f.m.restock.RunSku(f.ct, sku); err != nil {
			t.Fatal(err)
		}
		if got := f.rn.skus(); len(got) != 0 {
			t.Fatalf("%s: a job that finds the SKU not purchasable must not notify, got %v", c.name, got)
		}
		c.undo()
	}

	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.m.restock.RunSku(f.ct, sku); err != nil {
		t.Fatal(err)
	}
	if got := f.rn.skus(); len(got) != 0 {
		t.Fatalf("a deleted product must not notify, got %v", got)
	}
}

func TestStoreExposureEnqueuesOneStoreJobThatNotifiesOnlyPurchasableSkus(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, buyable := f.stocked(storeID, acct, "可買", 5, true)
	_, soldOut := f.stocked(storeID, acct, "售完", 0, true)
	_, draft := f.stocked(storeID, acct, "草稿", 5, false)
	other, otherAcct := f.store()
	_, elsewhere := f.stocked(other, otherAcct, "別家店", 5, true)

	f.setExposable(storeID, false)
	f.clearJobs()
	f.setExposable(storeID, true)

	event := func(exposable bool) seller.StoreExposureChanged {
		return seller.StoreExposureChanged{OccurredAt: f.clock(), EventID: uuid.NewV7(), StoreID: storeID, Exposable: exposable, Version: 2}
	}
	if err := f.m.OnStoreExposureChanged(f.ct, event(false)); err != nil {
		t.Fatal(err)
	}
	if got := f.jobsOf(kindStore, "", nil); got != 0 {
		t.Fatalf("a store that stops being exposable enqueues nothing, got %d", got)
	}
	if err := f.m.OnStoreExposureChanged(f.ct, event(true)); err != nil {
		t.Fatal(err)
	}
	if err := f.m.OnStoreExposureChanged(f.ct, event(true)); err != nil {
		t.Fatal(err)
	}
	if got := f.jobsOf(kindStore, "store_id", storeID); got != 1 {
		t.Fatalf("one store-level job (merged), got %d", got)
	}
	if got := f.jobsOf(kindSku, "", nil); got != 0 {
		t.Fatalf("the SKUs are not expanded inside the event's transaction, got %d SKU jobs", got)
	}

	if err := f.m.restock.RunStore(f.ct, storeID); err != nil {
		t.Fatal(err)
	}
	if f.skuJobs(buyable) != 1 || f.skuJobs(soldOut) != 0 || f.skuJobs(draft) != 0 || f.skuJobs(elsewhere) != 0 {
		t.Fatalf("the store job expands to the store's purchasable SKUs only: buyable %d, sold out %d, draft %d, other store %d",
			f.skuJobs(buyable), f.skuJobs(soldOut), f.skuJobs(draft), f.skuJobs(elsewhere))
	}
	for _, sku := range []uuid.UUID{buyable, soldOut, draft} {
		if err := f.m.restock.RunSku(f.ct, sku); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(f.rn.skus(), []uuid.UUID{buyable}) {
		t.Fatalf("only the purchasable SKU is notified, got %v", f.rn.skus())
	}

	// 店家此刻不可曝光時展開不出任何 SKU。
	f.clearJobs()
	f.setExposable(storeID, false)
	if err := f.m.restock.RunStore(f.ct, storeID); err != nil {
		t.Fatal(err)
	}
	if got := f.jobsOf(kindSku, "", nil); got != 0 {
		t.Fatalf("a store that is not exposable expands to nothing, got %d", got)
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
	f.clearJobs()

	purged := func() [][]uuid.UUID {
		f.t.Helper()
		f.rn.purged = nil
		rows, err := f.d.Pool().Query(f.ct, `SELECT args FROM river.river_job WHERE kind = $1 ORDER BY id`, kindPurge)
		if err != nil {
			f.t.Fatal(err)
		}
		defer rows.Close()
		var out [][]uuid.UUID
		for rows.Next() {
			var a restock.PurgeArgs
			if err := rows.Scan(&a); err != nil {
				f.t.Fatal(err)
			}
			out = append(out, a.SkuIDs)
			if err := f.m.restock.RunPurge(f.ct, a.SkuIDs); err != nil {
				f.t.Fatal(err)
			}
		}
		return out
	}

	// 修改商品拿掉一個 SKU：只清被拿掉的那個。
	keep := in
	keep.Axes = []Axis{{Name: "尺寸", Values: []string{"S"}}}
	keep.Skus = []SkuInput{{Option1: "S", Price: 100}}
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, keep); err != nil {
		t.Fatal(err)
	}
	var removed uuid.UUID
	for _, sk := range p.Skus {
		if sk.Option1 == "M" {
			removed = sk.ID
		}
	}
	if got := purged(); len(got) != 1 || !slices.Equal(got[0], []uuid.UUID{removed}) || !slices.Equal(f.rn.purged[0], []uuid.UUID{removed}) {
		t.Fatalf("removing a SKU purges exactly that SKU's subscriptions, got %v", got)
	}

	// 沒有拿掉 SKU 的修改不排。
	f.clearJobs()
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, p.ID, keep); err != nil {
		t.Fatal(err)
	}
	if got := f.jobsOf(kindPurge, "", nil); got != 0 {
		t.Fatalf("an update that keeps every SKU purges nothing, got %d", got)
	}

	// 刪除商品：清它還在的 SKU。
	kept := p.Skus[0].ID
	if p.Skus[0].Option1 != "S" {
		kept = p.Skus[1].ID
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	if got := purged(); len(got) != 1 || !slices.Equal(got[0], []uuid.UUID{kept}) {
		t.Fatalf("deleting the product purges its remaining SKU, got %v", got)
	}
}

// failingJobs 讓排入失敗，其餘轉給真的 River client。
type failingJobs struct{ jobs.Tracker }

var errEnqueue = errors.New("enqueue failed on purpose")

func (failingJobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	return 0, errEnqueue
}

// 排入失敗時標記不能留下：標記與 job 在同一個 transaction 內，沒有 transaction 的呼叫方也一樣（request 自己開）。
// 留下的標記會讓之後每次轉換都以為有 job 而合併，通知永遠漏掉。
func TestRestockEnqueueFailureLeavesNoMarker(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	_, sku := f.stocked(storeID, acct, "排入失敗", 5, true)
	failing := restock.New(restock.Deps{DB: f.d, Jobs: failingJobs{newJobs(t, f.d)}, Now: f.clock, Notifier: f.rn})

	if err := failing.StockIncreased(f.ct, sku, 5); !errors.Is(err, errEnqueue) {
		t.Fatalf("StockIncreased error = %v，預期排入的錯誤", err)
	}
	if err := failing.StoreExposed(f.ct, storeID); !errors.Is(err, errEnqueue) {
		t.Fatalf("StoreExposed error = %v，預期排入的錯誤", err)
	}
	if got := f.count(`SELECT count(*) FROM catalog.restock_pending`); got != 0 {
		t.Fatalf("a failed enqueue must leave no marker, got %d", got)
	}
	// 修好之後同一個轉換能排出 job。
	if err := f.m.restock.StockIncreased(f.ct, sku, 5); err != nil {
		t.Fatal(err)
	}
	if got := f.skuJobs(sku); got != 1 {
		t.Fatalf("after the failure the next transition enqueues, got %d jobs", got)
	}
}

// 標記綁著它的 job：job 已結束、取消或被刪就不會再消耗標記，下一次轉換要接手排新 job；
// 還沒開始、執行中、等重試的 job 還會消耗標記，下一次轉換合併（不排新 job、標記仍綁原 job）。
func TestRestockMarkerFollowsItsJob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		set      string // 對 river_job 的處理
		takeover bool
	}{
		{"deleted", `DELETE FROM river.river_job WHERE kind = $1`, true},
		{"cancelled", `UPDATE river.river_job SET state = 'cancelled', finalized_at = now() WHERE kind = $1`, true},
		{"discarded", `UPDATE river.river_job SET state = 'discarded', finalized_at = now(), attempt = 1, attempted_at = now(), attempted_by = ARRAY['t'] WHERE kind = $1`, true},
		{"completed", `UPDATE river.river_job SET state = 'completed', finalized_at = now(), attempt = 1, attempted_at = now(), attempted_by = ARRAY['t'] WHERE kind = $1`, true},
		{"available", `UPDATE river.river_job SET state = 'available', scheduled_at = now() WHERE kind = $1`, false},
		{"running", `UPDATE river.river_job SET state = 'running', attempt = 1, attempted_at = now(), attempted_by = ARRAY['t'] WHERE kind = $1`, false},
		{"retryable", `UPDATE river.river_job SET state = 'retryable', attempt = 1, attempted_at = now(), attempted_by = ARRAY['t'], scheduled_at = now() + interval '1 minute' WHERE kind = $1`, false},
	}
	for _, kind := range []string{kindSku, kindStore} {
		for _, c := range cases {
			t.Run(kind+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				f := newProductFixture(t)
				storeID, acct := f.store()
				_, sku := f.stocked(storeID, acct, "標記", 5, true)
				key, transition := "sku:"+sku.String(), func() {
					if err := f.m.restock.StockIncreased(f.ct, sku, 5); err != nil {
						t.Fatal(err)
					}
				}
				if kind == kindStore {
					key, transition = fmt.Sprintf("store:%d", storeID), func() {
						if err := f.m.restock.StoreExposed(f.ct, storeID); err != nil {
							t.Fatal(err)
						}
					}
				}
				marker := func() int64 {
					var id int64
					if err := f.d.Pool().QueryRow(f.ct, `SELECT job_id FROM catalog.restock_pending WHERE job_key = $1`, key).Scan(&id); err != nil {
						t.Fatal(err)
					}
					return id
				}
				transition()
				first := marker()
				f.exec(c.set, kind)
				transition()
				second := marker()
				if c.takeover && second == first {
					t.Fatalf("job is %s: the marker must be taken over by a new job, still bound to %d", c.name, first)
				}
				if !c.takeover && second != first {
					t.Fatalf("job is %s and will still consume the marker: merge, but the marker moved %d -> %d", c.name, first, second)
				}
				// 標記綁的 job 一定存在且還會執行，且這個 kind 沒有第二個還會執行的 job。
				if got := f.count(`SELECT count(*) FROM river.river_job WHERE id = $1 AND state IN ('pending','scheduled','available','running','retryable')`, second); got != 1 {
					t.Fatalf("the marker's job %d must be live, got %d", second, got)
				}
				if got := f.count(`SELECT count(*) FROM river.river_job WHERE kind = $1 AND state IN ('pending','scheduled','available','running','retryable')`, kind); got != 1 {
					t.Fatalf("exactly one live job per key, got %d", got)
				}
			})
		}
	}
}
