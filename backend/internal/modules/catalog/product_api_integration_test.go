//go:build integration

package catalog

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func (f *productFixture) active(storeID, acct int64, in ProductInput) Product {
	f.t.Helper()
	p := f.create(storeID, acct, in)
	f.activate(storeID, p.ID)
	return p
}

func stocked(in ProductInput, qty int32) ProductInput {
	in.Skus = []SkuInput{{Price: 100, StockQty: qty}}
	return in
}

func (f *productFixture) mustProduct(storeID int64, id uuid.UUID) Product {
	f.t.Helper()
	p, err := f.m.Product(f.ct, storeID, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func TestStaleEtagIsRejectedAndStockChangesDoNotMoveIt(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	up := media.Uploader{ID: acct}
	p := f.create(storeID, acct, stocked(f.input("山藥"), 10))
	if p.Version != 1 {
		t.Fatalf("a new product starts at version 1, got %d", p.Version)
	}
	if _, err := ParseEtag(Etag(p.Version)); err != nil {
		t.Fatal(err)
	}

	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, p.Version+1, f.input("改名"), nil); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("a stale etag on update must be rejected: %v", err)
	}
	if _, err := f.m.SetProductStatus(f.ct, storeID, p.ID, p.Version+1, StatusActive, nil); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("a stale etag on status change must be rejected: %v", err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, p.Version+1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("a stale etag on delete must be rejected: %v", err)
	}
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, 0, f.input("x"), nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("update without an etag is a client error: %v", err)
	}
	if got := f.mustProduct(storeID, p.ID); got.Title != "山藥" || got.Version != p.Version {
		t.Fatalf("a rejected write must leave the product alone: %+v", got)
	}
	f.activate(storeID, p.ID)
	p = f.mustProduct(storeID, p.ID)

	sku := p.Skus[0].ID
	key := func(s string) string { return uuid.NewV7().String() + s }
	f.must(f.m.AdjustStock(f.ct, storeID, key("a"), sku, 5))
	f.must(f.m.SetStock(f.ct, storeID, key("s"), sku, 20, 15))
	f.must(f.m.DeductStock(f.ct, key("d"), []StockItem{{SkuID: sku, Quantity: 3}}))
	f.exec(`UPDATE catalog.products SET sales_count = sales_count + 1, favorite_count = favorite_count + 1 WHERE id = $1`, p.ID)
	if got := f.mustProduct(storeID, p.ID); got.Version != p.Version {
		t.Fatalf("stock and counters must not move the version: %d -> %d", p.Version, got.Version)
	}
	updated, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, p.Version, f.input("山藥二號"), nil)
	if err != nil {
		t.Fatalf("editing with the version read before the stock changes must succeed: %v", err)
	}
	if updated.Version != p.Version+1 {
		t.Fatalf("an update bumps the version by one: %d -> %d", p.Version, updated.Version)
	}
	if updated.Skus[0].StockQty != 17 {
		t.Fatalf("an update leaves the stock alone: %d", updated.Skus[0].StockQty)
	}

	f.mustStatus(f.m.SetProductStatus(f.ct, storeID, p.ID, updated.Version, StatusArchived, nil))
	archived := f.mustProduct(storeID, p.ID)
	if archived.Version != updated.Version+1 {
		t.Fatalf("a status change bumps the version: %d -> %d", updated.Version, archived.Version)
	}
	f.mustStatus(f.m.SetProductStatus(f.ct, storeID, p.ID, archived.Version, StatusActive, nil))
	active := f.mustProduct(storeID, p.ID)
	if active.Version != archived.Version+1 {
		t.Fatalf("a status change bumps the version: %d -> %d", archived.Version, active.Version)
	}
	f.mustStatus(f.m.SetProductStatus(f.ct, storeID, p.ID, active.Version, StatusActive, nil))
	if got := f.mustProduct(storeID, p.ID); got.Version != active.Version {
		t.Fatalf("a no-op status change must not bump the version: %d -> %d", active.Version, got.Version)
	}
	f.mustStatus(f.m.SetProductStatus(f.ct, storeID, p.ID, archived.Version, StatusActive, nil))
	if got := f.mustProduct(storeID, p.ID); got.Version != active.Version {
		t.Fatalf("a retried status change must leave the etag alone: %d -> %d", active.Version, got.Version)
	}
	if _, err := f.m.SetProductStatus(f.ct, storeID, p.ID, archived.Version, StatusArchived, nil); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("a stale etag with a different target is still a conflict: %v", err)
	}
}

func TestDeleteIsIdempotentForTheOwnerAndHidesOtherStores(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, _ := f.store()
	p := f.create(storeID, acct, f.input("x"))
	f.must(f.m.DeleteProduct(f.ct, storeID, p.ID, 0))
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
		t.Fatalf("deleting an already deleted own product succeeds: %v", err)
	}
	if err := f.m.DeleteProduct(f.ct, other, p.ID, 0); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("another store must not learn that it exists: %v", err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, uuid.NewV7(), 0); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("an unknown product is not found: %v", err)
	}
}

func TestCreateIsIdempotentByRequestID(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, otherAcct := f.store()
	up := media.Uploader{ID: acct}
	in := f.input("山藥")
	key := uuid.NewV7()

	first, err := f.m.CreateProduct(f.ct, storeID, up, key, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.m.CreateProduct(f.ct, storeID, up, key, in, nil)
	if err != nil || again.ID != first.ID {
		t.Fatalf("a replay returns the first product: %v %v", again.ID, err)
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("a replay must not create a second product, got %d", n)
	}
	f.activate(storeID, first.ID)
	if got, _ := f.m.CreateProduct(f.ct, storeID, up, key, in, nil); got.Status != StatusActive {
		t.Fatalf("a replay shows the product as it is now: %s", got.Status)
	}

	changed := in
	changed.Title = "地瓜"
	if _, err := f.m.CreateProduct(f.ct, storeID, up, key, changed, nil); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("the same key with different content is rejected: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("a rejected replay must not create a product, got %d", n)
	}

	mine, err := f.m.CreateProduct(f.ct, other, media.Uploader{ID: otherAcct}, key, f.input("別家"), nil)
	if err != nil || mine.ID == first.ID {
		t.Fatalf("another store's key is its own: %v %v", mine.ID, err)
	}

	if _, err := f.m.CreateProduct(f.ct, storeID, up, uuid.Nil(), in, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a create without a request id is a client error: %v", err)
	}

	f.must(f.m.DeleteProduct(f.ct, storeID, first.ID, 0))
	if _, err := f.m.CreateProduct(f.ct, storeID, up, key, in, nil); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a replay of a deleted product is not found: %v", err)
	}
}

func TestConcurrentCreatesWithOneRequestIDMakeOneProduct(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	in := ProductInput{Title: "併發", CategoryID: f.leaf(), Skus: []SkuInput{{Price: 100}}}
	in.Gallery = []Image{{Token: "conc1"}}
	key := uuid.NewV7()
	const n = 10
	ids := make([]uuid.UUID, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			p, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, key, in, nil)
			ids[i], errs[i] = p.ID, err
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("every concurrent request returns the same product: %v vs %v", ids[i], ids[0])
		}
	}
	if got := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); got != 1 {
		t.Fatalf("one product, got %d", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id WHERE p.store_id = $1`, storeID); got != 1 {
		t.Fatalf("one SKU row set, got %d", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.product_images i JOIN catalog.products p ON p.id = i.product_id WHERE p.store_id = $1`, storeID); got != 1 {
		t.Fatalf("one image row, got %d", got)
	}
}

func TestSetStockChecksTheExpectedQuantityAndWritesTheLedger(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)

	var mm *StockMismatchError
	if err := f.m.SetStock(f.ct, f.storeID, "set:bad", sku, 99, 11); !errors.As(err, &mm) || mm.Current != 10 || mm.SkuID != sku {
		t.Fatalf("an expected value that is not the current one is a precondition failure carrying the current value: %v", err)
	}
	if got := f.stockOf(sku); got != 10 {
		t.Fatalf("a mismatch must not write: %d", got)
	}
	if n := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE idempotency_key = 'set:bad'`); n != 0 {
		t.Fatalf("a failed set leaves no operation row (the key stays usable): %d", n)
	}

	f.must(f.m.SetStock(f.ct, f.storeID, "set:1", sku, 25, 10))
	if got := f.stockOf(sku); got != 25 {
		t.Fatalf("stock = %d, want 25", got)
	}
	if got := f.count(`SELECT m.delta FROM catalog.stock_movements m JOIN catalog.stock_operations o ON o.id = m.operation_id WHERE o.idempotency_key = 'set:1' AND o.kind = 'SET'`); got != 15 {
		t.Fatalf("the ledger records the difference, got %d", got)
	}
	f.must(f.m.SetStock(f.ct, f.storeID, "set:same", sku, 25, 25))
	if n := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE idempotency_key = 'set:same'`); n != 1 {
		t.Fatalf("a no-change set still records its operation: %d", n)
	}

	for name, c := range map[string]struct {
		store    int64
		sku      uuid.UUID
		qty, exp int32
		want     error
	}{
		"another store's sku": {f.otherStoreID, sku, 1, 25, ErrSkuNotFound},
		"unknown sku":         {f.storeID, uuid.NewV7(), 1, 0, ErrSkuNotFound},
		"negative quantity":   {f.storeID, sku, -1, 25, ErrInvalidStockInput},
		"negative expected":   {f.storeID, sku, 1, -1, ErrInvalidStockInput},
	} {
		if err := f.m.SetStock(f.ct, c.store, "set:"+name, c.sku, c.qty, c.exp); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}

func TestSetStockReplaysOnceAndRejectsADifferentRequest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku, other := f.sku(10), f.sku(10)
	f.must(f.m.SetStock(f.ct, f.storeID, "set:r", sku, 30, 10))
	f.must(f.m.SetStock(f.ct, f.storeID, "set:r", sku, 30, 10))
	if got := f.stockOf(sku); got != 30 {
		t.Fatalf("stock = %d", got)
	}
	if n := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE idempotency_key = 'set:r'`); n != 1 {
		t.Fatalf("one operation, got %d", n)
	}
	for name, err := range map[string]error{
		"quantity": f.m.SetStock(f.ct, f.storeID, "set:r", sku, 31, 10),
		"expected": f.m.SetStock(f.ct, f.storeID, "set:r", sku, 30, 11),
		"sku":      f.m.SetStock(f.ct, f.storeID, "set:r", other, 30, 10),
		"kind":     f.m.AdjustStock(f.ct, f.storeID, "set:r", sku, 1),
	} {
		if !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("a different %s under the same key is rejected: %v", name, err)
		}
	}
	f.must(f.m.AdjustStock(f.ct, f.storeID, "adj:k", sku, 1))
	if err := f.m.SetStock(f.ct, f.storeID, "adj:k", sku, 31, 31); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("a SET under an ADJUST key is rejected: %v", err)
	}
}

func TestConcurrentSameKeySetsOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)
	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.m.SetStock(f.ct, f.storeID, "set:c", sku, 40, 10)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("request %d: %v", i, err)
		}
	}
	if got := f.stockOf(sku); got != 40 {
		t.Fatalf("stock = %d, want 40", got)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_movements m JOIN catalog.stock_operations o ON o.id = m.operation_id WHERE o.idempotency_key = 'set:c'`); got != 1 {
		t.Fatalf("one movement, got %d", got)
	}
}

func TestConcurrentSetStockWithTheSameExpectedValueLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sku := f.sku(10)
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.m.SetStock(f.ct, f.storeID, fmt.Sprintf("set:w%d", i), sku, int32(100+i), 10)
		}()
	}
	close(start)
	wg.Wait()
	winners := 0
	for i, err := range errs {
		var mm *StockMismatchError
		switch {
		case err == nil:
			winners++
		case errors.As(err, &mm):
		default:
			t.Errorf("request %d: %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one writer sees the value it expected, got %d", winners)
	}
	if got := f.count(`SELECT count(*) FROM catalog.stock_operations WHERE kind = 'SET' AND store_id = $1`, f.storeID); got != 1 {
		t.Fatalf("only the winner leaves an operation, got %d", got)
	}
}

func TestLowStockCountIsPerSkuActiveOnlyWithPerProductThreshold(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, otherAcct := f.store()
	ten := int32(10)
	zero := int32(0)

	low := f.active(storeID, acct, stocked(f.input("偏低"), 3))
	atDefault := f.active(storeID, acct, stocked(f.input("剛好門檻"), 5))
	enough := f.active(storeID, acct, stocked(f.input("充足"), 6))
	soldOut := f.active(storeID, acct, stocked(f.input("售完"), 0))
	f.create(storeID, acct, stocked(f.input("草稿偏低"), 1))
	archived := f.active(storeID, acct, stocked(f.input("下架偏低"), 1))
	f.must(setStatusP(f.m, f.ct, storeID, archived.ID, StatusArchived))
	withThreshold := stocked(f.input("自訂門檻"), 8)
	withThreshold.LowStockThreshold = &ten
	custom := f.active(storeID, acct, withThreshold)
	neverLow := stocked(f.input("門檻零"), 1)
	neverLow.LowStockThreshold = &zero
	f.active(storeID, acct, neverLow)
	multi := f.input("多規格")
	multi.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M"}}}
	multi.Skus = []SkuInput{{Option1: "S", Price: 100, StockQty: 50}, {Option1: "M", Price: 100, StockQty: 2}}
	multiP := f.active(storeID, acct, multi)
	f.active(other, otherAcct, stocked(f.input("別家偏低"), 1))

	count := func(f2 SellerFilter) int32 {
		n, err := f.m.CountSellerProducts(f.ct, storeID, f2)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := count(SellerFilter{Status: StatusActive, StockStatus: StockLow}); got != 4 {
		t.Fatalf("low-stock count = %d, want 4 (low, at-default, custom threshold, multi-sku)", got)
	}
	if got := count(SellerFilter{StockStatus: StockLow}); got != 4 {
		t.Fatalf("a stock filter alone still counts only ACTIVE products (drafts and archived are noise): %d", got)
	}
	if got := count(SellerFilter{Status: StatusDraft, StockStatus: StockLow}); got != 0 {
		t.Fatalf("low stock among drafts = %d, want 0", got)
	}
	if got := count(SellerFilter{StockStatus: StockOutOfStock}); got != 1 {
		t.Fatalf("sold-out is counted separately: %d", got)
	}
	page, err := f.m.ListSellerProducts(f.ct, storeID, SellerFilter{StockStatus: StockLow}, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := f.ids(page)
	slices.SortFunc(got, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	want := []uuid.UUID{low.ID, atDefault.ID, custom.ID, multiP.ID}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(got, want) {
		t.Fatalf("list and count agree on which products are low: %v want %v (enough %v, sold out %v)", got, want, enough.ID, soldOut.ID)
	}
	for _, it := range page.Items {
		if it.TotalStock < 0 {
			t.Fatalf("total stock is reported: %+v", it)
		}
	}
	if _, err := f.m.CountSellerProducts(f.ct, storeID, SellerFilter{StockStatus: "NOPE"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an unknown stock status is invalid: %v", err)
	}
	for _, bad := range []int32{-1, 10000} {
		in := f.input("門檻越界")
		in.LowStockThreshold = &bad
		if _, err := createP(f.m, f.ct, storeID, media.Uploader{ID: acct}, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("threshold %d must be invalid: %v", bad, err)
		}
	}
}

func TestSellerQueryHitsAcrossPagesAndMatchesSkuOptions(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	other, otherAcct := f.store()
	var want []uuid.UUID
	for i := range 5 {
		want = append(want, f.create(storeID, acct, f.input(fmt.Sprintf("蘋果醬 %d", i))).ID)
		f.advance(1_000_000_000)
		f.create(storeID, acct, f.input(fmt.Sprintf("橘子 %d", i)))
		f.advance(1_000_000_000)
	}
	byOption := f.input("襯衫")
	byOption.Axes = []Axis{{Name: "顏色", Values: []string{"珊瑚紅", "海軍藍"}}}
	byOption.Skus = []SkuInput{{Option1: "珊瑚紅", Price: 100}, {Option1: "海軍藍", Price: 100}}
	optionHit := f.create(storeID, acct, byOption)
	f.create(other, otherAcct, f.input("別家蘋果醬"))

	var got []uuid.UUID
	var after *ListCursor
	pages := 0
	for {
		page, err := f.m.ListSellerProducts(f.ct, storeID, SellerFilter{Query: "蘋果"}, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, f.ids(page)...)
		pages++
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	slices.SortFunc(got, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(got, want) || pages != 3 {
		t.Fatalf("the query must find all 5 of the store's matches over %d pages (no repeat, no gap): %v want %v", pages, got, want)
	}
	page, _ := f.m.ListSellerProducts(f.ct, storeID, SellerFilter{Query: "珊瑚"}, nil, 10)
	if ids := f.ids(page); len(ids) != 1 || ids[0] != optionHit.ID {
		t.Fatalf("a SKU option value is searchable: %v", ids)
	}
	if n, _ := f.m.CountSellerProducts(f.ct, storeID, SellerFilter{Query: "蘋果"}); n != 5 {
		t.Fatalf("count follows the query: %d", n)
	}
	if page, _ = f.m.ListSellerProducts(f.ct, storeID, SellerFilter{Query: "  "}, nil, 50); len(page.Items) != 11 {
		t.Fatalf("a blank query does not filter: %d", len(page.Items))
	}
}

func TestPublicSearchFindsAndPagesWithoutRepeatOrGap(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	hidden, hiddenAcct := f.store()
	var want []uuid.UUID
	for i := range 7 {
		in := f.input(fmt.Sprintf("有機蘋果 %d", i))
		in.Skus = []SkuInput{{Price: int64(100 + i%3*50), StockQty: 1}}
		want = append(want, f.active(storeID, acct, in).ID)
		f.activate(storeID, f.create(storeID, acct, f.input(fmt.Sprintf("香蕉 %d", i))).ID)
	}
	f.create(storeID, acct, f.input("草稿蘋果"))
	f.active(hidden, hiddenAcct, f.input("停權蘋果"))
	f.setExposable(hidden, false)
	f.active(storeID, acct, f.input("100%純汁"))
	f.active(storeID, acct, f.input("100X純汁"))
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })

	walk := func(filter PublicFilter, sort ProductSort, size int32) []uuid.UUID {
		var out []uuid.UUID
		var after *ListCursor
		for range 20 {
			page, err := f.m.ListPublicProducts(f.ct, f.clock(), filter, sort, after, size)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, f.ids(page)...)
			if page.Next == nil {
				return out
			}
			after = page.Next
		}
		t.Fatal("pagination did not end")
		return nil
	}
	for _, sort := range []ProductSort{SortRelevance, SortLatest, SortPriceAsc, SortPriceDesc, SortSales, SortRating} {
		got := walk(PublicFilter{Query: "蘋果"}, sort, 3)
		sorted := slices.Clone(got)
		slices.SortFunc(sorted, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		if !slices.Equal(sorted, want) {
			t.Errorf("sort %s: search must hit exactly the visible matches once each over several pages: %v want %v", sort, sorted, want)
		}
	}
	var prices []int64
	for _, id := range walk(PublicFilter{Query: "蘋果"}, SortPriceAsc, 2) {
		p, err := f.m.PublicProduct(f.ct, id, f.clock())
		if err != nil {
			t.Fatal(err)
		}
		prices = append(prices, p.MinPrice)
	}
	if !slices.IsSorted(prices) {
		t.Fatalf("PRICE_ASC order across pages: %v", prices)
	}
	if got := walk(PublicFilter{Query: "100%"}, SortLatest, 10); len(got) != 1 {
		t.Fatalf("a percent sign in the query is literal: %v", got)
	}
	if _, err := f.m.ListPublicProducts(f.ct, f.clock(), PublicFilter{}, SortRelevance, nil, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("relevance needs a query: %v", err)
	}
	if _, err := f.m.ListPublicProducts(f.ct, f.clock(), PublicFilter{}, "NOPE", nil, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an unknown sort is invalid: %v", err)
	}
	if names, err := f.m.SuggestProductNames(f.ct, f.clock(), "蘋果", 3); err != nil || len(names) != 3 {
		t.Fatalf("name suggestions are capped by the limit: %v %v", names, err)
	}
	if names, _ := f.m.SuggestProductNames(f.ct, f.clock(), "   ", 3); len(names) != 0 {
		t.Fatalf("a blank query suggests nothing: %v", names)
	}
}

func TestPublicFiltersAndStoreScopedList(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	a, aAcct := f.store()
	b, bAcct := f.store()
	c, cAcct := f.store()
	var parent uuid.UUID
	f.must(f.d.Pool().QueryRow(f.ct, `SELECT parent_id FROM catalog.categories WHERE id = $1`, f.leaf()).Scan(&parent))
	withTags := func(title string, price int64, tags ...string) ProductInput {
		in := f.input(title)
		in.Hashtags = tags
		in.Skus = []SkuInput{{Price: price, StockQty: 1}}
		return in
	}
	pa := f.active(a, aAcct, withTags("A", 100, "#x", "#y"))
	pb := f.active(b, bAcct, withTags("B", 500, "#x"))
	pc := f.active(c, cAcct, withTags("C", 900))

	list := func(flt PublicFilter) []uuid.UUID {
		page, err := f.m.ListPublicProducts(f.ct, f.clock(), flt, "", nil, 50)
		if err != nil {
			t.Fatal(err)
		}
		got := f.ids(page)
		slices.SortFunc(got, func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) })
		return got
	}
	same := func(name string, got []uuid.UUID, want ...Product) {
		var ids []uuid.UUID
		for _, p := range want {
			ids = append(ids, p.ID)
		}
		slices.SortFunc(ids, func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) })
		if !slices.Equal(got, ids) {
			t.Errorf("%s: %v, want %v", name, got, ids)
		}
	}
	same("a top-level category covers its leaves", list(PublicFilter{CategoryIDs: []uuid.UUID{parent}}), pa, pb, pc)
	same("hashtag any", list(PublicFilter{Hashtags: []string{"x", "y"}}), pa, pb)
	same("hashtag all", list(PublicFilter{Hashtags: []string{"x", "y"}, HashtagAll: true}), pa)
	same("hashtag all ignores duplicate and differently written names", list(PublicFilter{Hashtags: []string{"#x", "x", "X", "y"}, HashtagAll: true}), pa)
	lo, hi := int64(400), int64(600)
	same("price range", list(PublicFilter{PriceMin: &lo, PriceMax: &hi}), pb)
	same("stores", list(PublicFilter{StoreIDs: []int64{a, c}}), pa, pc)
	if _, err := f.m.ListPublicProducts(f.ct, f.clock(), PublicFilter{Hashtags: []string{"bad tag!"}}, "", nil, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an invalid hashtag name is a client error: %v", err)
	}

	of, err := f.m.ListPublicProductsOfStores(f.ct, f.clock(), []int64{a, b}, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.ids(of); len(got) != 2 || slices.Contains(got, pc.ID) {
		t.Fatalf("only the given stores' products: %v", got)
	}
	if empty, err := f.m.ListPublicProductsOfStores(f.ct, f.clock(), nil, nil, 50); err != nil || len(empty.Items) != 0 || empty.Next != nil {
		t.Fatalf("no followed store is an empty feed, never everything: %v %v", empty.Items, err)
	}
	f.setExposable(a, false)
	if of, _ = f.m.ListPublicProductsOfStores(f.ct, f.clock(), []int64{a, b}, nil, 50); len(of.Items) != 1 {
		t.Fatalf("a store that is not exposable drops out of the feed: %d", len(of.Items))
	}
}

func TestArchivedProductPageIsPublicButDraftIsNot(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.active(storeID, acct, f.input("下架"))
	f.must(setStatusP(f.m, f.ct, storeID, p.ID, StatusArchived))
	got, err := f.m.PublicProduct(f.ct, p.ID, f.clock())
	if err != nil || got.Status != StatusArchived {
		t.Fatalf("an archived product's page stays readable with its status: %v %v", got.Status, err)
	}
	page, _ := f.m.ListPublicProducts(f.ct, f.clock(), PublicFilter{}, "", nil, 10)
	if len(page.Items) != 0 {
		t.Fatalf("an archived product is not listed: %v", f.ids(page))
	}
	f.must(setStatusP(f.m, f.ct, storeID, p.ID, StatusDraft))
	if _, err := f.m.PublicProduct(f.ct, p.ID, f.clock()); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a draft is not public: %v", err)
	}
}

func TestAdminListHasTotalSizeAndPagesWithoutRepeatOrGap(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	a, aAcct := f.store()
	b, bAcct := f.store()
	var all []uuid.UUID
	for i := range 3 {
		all = append(all, f.create(a, aAcct, f.input(fmt.Sprintf("甲%d", i))).ID)
		f.advance(1_000_000_000)
	}
	for i := range 2 {
		all = append(all, f.active(b, bAcct, f.input(fmt.Sprintf("乙%d", i))).ID)
		f.advance(1_000_000_000)
	}
	gone := f.create(a, aAcct, f.input("已刪"))
	f.must(f.m.DeleteProduct(f.ct, a, gone.ID, 0))

	walk := func(flt AdminFilter, size int32) (ids []uuid.UUID, totals []int32) {
		var after *ListCursor
		for range 10 {
			page, total, err := f.m.ListAdminProducts(f.ct, flt, after, size)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, f.ids(page)...)
			totals = append(totals, total)
			if page.Next == nil {
				return
			}
			after = page.Next
		}
		t.Fatal("pagination did not end")
		return
	}
	ids, totals := walk(AdminFilter{}, 2)
	if len(ids) != 5 || len(totals) != 3 {
		t.Fatalf("5 products over 3 pages: %d ids, %d pages", len(ids), len(totals))
	}
	for _, tot := range totals {
		if tot != 5 {
			t.Fatalf("total_size is the full count on every page (deleted excluded): %v", totals)
		}
	}
	got := slices.Clone(ids)
	slices.SortFunc(got, func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) })
	slices.SortFunc(all, func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) })
	if !slices.Equal(got, all) {
		t.Fatalf("no repeat, no gap: %v want %v", got, all)
	}
	ids, totals = walk(AdminFilter{StoreID: &b}, 1)
	if len(ids) != 2 || totals[0] != 2 {
		t.Fatalf("store filter: %v %v", ids, totals)
	}
	if ids, totals = walk(AdminFilter{Query: "甲"}, 10); len(ids) != 3 || totals[0] != 3 {
		t.Fatalf("query filter: %v %v", ids, totals)
	}

	draft := f.create(a, aAcct, f.input("後台看得到草稿"))
	if got, err := f.m.AdminProduct(f.ct, draft.ID); err != nil || got.StoreID != a {
		t.Fatalf("admin reads any status of any store: %v", err)
	}
	if _, err := f.m.AdminProduct(f.ct, gone.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("a deleted product is not found: %v", err)
	}
}

func TestHashtagSuggestionsCountOnlyVisibleProducts(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	hidden, hiddenAcct := f.store()
	tagged := func(title string, tags ...string) ProductInput {
		in := f.input(title)
		in.Hashtags = tags
		return in
	}
	f.active(storeID, acct, tagged("1", "#Yam", "#root"))
	f.active(storeID, acct, tagged("2", "#yam"))
	f.create(storeID, acct, tagged("草稿", "#yam"))
	f.active(hidden, hiddenAcct, tagged("停權", "#yam"))
	f.setExposable(hidden, false)

	got, err := f.m.SuggestHashtags(f.ct, f.clock(), "#ya", false, 10)
	if err != nil || len(got) != 1 || got[0].Name != "yam" || got[0].ProductCount != 2 {
		t.Fatalf("suggestions count visible products only: %+v %v", got, err)
	}
	if got, _ = f.m.SuggestHashtags(f.ct, f.clock(), "am", true, 10); len(got) != 1 {
		t.Fatalf("contains: %+v", got)
	}
	if got, _ = f.m.SuggestHashtags(f.ct, f.clock(), "am", false, 10); len(got) != 0 {
		t.Fatalf("prefix only: %+v", got)
	}
	trend, err := f.m.TrendingHashtags(f.ct, f.clock(), 0)
	if err != nil || len(trend) != 2 || trend[0].Name != "yam" || trend[0].ProductCount != 2 {
		t.Fatalf("trending is ordered by visible product count: %+v %v", trend, err)
	}
}

func (f *productFixture) mustStatus(_ Product, err error) {
	f.t.Helper()
	f.must(err)
}

func (f *productFixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}
