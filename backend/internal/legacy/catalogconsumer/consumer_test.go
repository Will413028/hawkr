package catalogconsumer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/catalogserver"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// fakeCatalog 照 catalog 的規則記帳：歸還以鍵去重並指向原扣減，銷量以 order_no 去重、商品不存在整筆回滾。
type fakeCatalog struct {
	deducted map[string][]catalog.StockItem // 原扣減（以扣減鍵）
	restored map[string]bool                // 已歸還的鍵
	stock    map[uuid.UUID]int32
	sales    map[uuid.UUID]int32
	orders   map[string]bool
	products map[uuid.UUID]bool
	restores []string // 收到的歸還鍵
	err      error
}

func newFake() *fakeCatalog {
	return &fakeCatalog{deducted: map[string][]catalog.StockItem{}, restored: map[string]bool{}, stock: map[uuid.UUID]int32{}, sales: map[uuid.UUID]int32{}, orders: map[string]bool{}, products: map[uuid.UUID]bool{}}
}

func (f *fakeCatalog) RestoreStock(_ context.Context, key, deductKey string, items []catalog.StockItem) error {
	f.restores = append(f.restores, key+"<-"+deductKey)
	if f.err != nil {
		return f.err
	}
	if f.restored[key] {
		return nil
	}
	if _, ok := f.deducted[deductKey]; !ok {
		return catalog.ErrDeductionNotFound
	}
	for _, it := range items {
		f.stock[it.SkuID] += it.Quantity
	}
	f.restored[key] = true
	return nil
}

func (f *fakeCatalog) RecordSales(_ context.Context, orderNo string, items []catalog.SalesItem) error {
	if f.err != nil {
		return f.err
	}
	if f.orders[orderNo] {
		return nil
	}
	for _, it := range items {
		if !f.products[it.ProductID] {
			return catalog.ErrProductNotFound
		}
	}
	for _, it := range items {
		f.sales[it.ProductID] += it.Quantity
	}
	f.orders[orderNo] = true
	return nil
}

func cancelled(tx string, sku uuid.UUID, qty string) []byte {
	return fmt.Appendf(nil, `{"event_id":"e1","order_no":"O1","stock_tx_id":%q,"items":[{"sku_id":%q,"quantity":%q}],"store_id":5,"cancel_reason":null}`, tx, sku.String(), qty)
}

func TestCancelledOrderRestoresStockOnceWhenResent(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake()
	f.stock[sku] = 6
	f.deducted["tx-1"] = []catalog.StockItem{{SkuID: sku, Quantity: 4}}
	c := Consumer{Catalog: f}
	for i := range 3 {
		if err := c.OrderCancelled(context.Background(), cancelled("tx-1", sku, "4")); err != nil {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
	}
	if f.stock[sku] != 10 {
		t.Fatalf("庫存 = %d，同一個 stock_tx_id 重送三次只還一次，要 10", f.stock[sku])
	}
	if len(f.restores) != 3 || f.restores[0] != catalogserver.RestoreKey("tx-1")+"<-tx-1" {
		t.Fatalf("歸還鍵 = %v，要用 RestoreKey(stock_tx_id) 指向原扣減 stock_tx_id", f.restores)
	}
}

func TestCancelledOrderWithoutTheOriginalDeductionIsAcked(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake()
	if err := (Consumer{Catalog: f}).OrderCancelled(context.Background(), cancelled("never", sku, "1")); err != nil {
		t.Fatalf("找不到原扣減要視為補償成功：%v", err)
	}
	if f.stock[sku] != 0 {
		t.Fatalf("庫存 = %d，不得歸還", f.stock[sku])
	}
}

func TestCancelledOrderWithoutItemsRestoresNothing(t *testing.T) {
	t.Parallel()
	f := newFake()
	if err := (Consumer{Catalog: f}).OrderCancelled(context.Background(), []byte(`{"order_no":"O1","stock_tx_id":"tx","items":[]}`)); err != nil || len(f.restores) != 0 {
		t.Fatalf("err = %v、歸還呼叫 %v，沒有品項不得呼叫 catalog（空品項在 catalog 是歸還全部）", err, f.restores)
	}
}

func TestCancelledOrderAcceptsNumericQuantity(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake()
	f.deducted["tx"] = []catalog.StockItem{{SkuID: sku, Quantity: 2}}
	body := fmt.Appendf(nil, `{"stock_tx_id":"tx","items":[{"sku_id":%q,"quantity":2}]}`, sku.String())
	if err := (Consumer{Catalog: f}).OrderCancelled(context.Background(), body); err != nil || f.stock[sku] != 2 {
		t.Fatalf("err = %v、庫存 %d，數字的 quantity 也要還", err, f.stock[sku])
	}
}

func TestCancelledBadPayloadsArePermanentWithoutWriting(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7().String()
	for name, body := range map[string]string{
		"不是 JSON":         `{`,
		"缺 stock_tx_id":   `{"items":[{"sku_id":"` + sku + `","quantity":"1"}]}`,
		"sku_id 不是 UUID":  `{"stock_tx_id":"tx","items":[{"sku_id":"123","quantity":"1"}]}`,
		"quantity 不是數字":   `{"stock_tx_id":"tx","items":[{"sku_id":"` + sku + `","quantity":"x"}]}`,
		"quantity 為零":     `{"stock_tx_id":"tx","items":[{"sku_id":"` + sku + `","quantity":"0"}]}`,
		"quantity 為 null": `{"stock_tx_id":"tx","items":[{"sku_id":"` + sku + `","quantity":null}]}`,
	} {
		f := newFake()
		err := (Consumer{Catalog: f}).OrderCancelled(context.Background(), []byte(body))
		if err == nil || !isPermanent(err) || len(f.restores) != 0 {
			t.Errorf("%s：err = %v、歸還呼叫 %d 次，要 permanent 且不呼叫 catalog", name, err, len(f.restores))
		}
	}
}

func TestCancelledClassifiesCatalogErrors(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	for _, c := range []struct {
		err       error
		permanent bool
	}{
		{catalog.ErrRestoreExceedsDeduction, true},
		{catalog.ErrIdempotencyKeyReused, true},
		{catalog.ErrInvalidStockInput, true},
		{errors.New("db down"), false},
	} {
		f := newFake()
		f.err = c.err
		err := (Consumer{Catalog: f}).OrderCancelled(context.Background(), cancelled("tx", sku, "1"))
		if err == nil || isPermanent(err) != c.permanent {
			t.Errorf("%v：err = %v permanent = %v，要 permanent = %v", c.err, err, err != nil && isPermanent(err), c.permanent)
		}
	}
}

// saleLine 是銷量事件的一個品項；quantity 是 JSON 數字。
type saleLine struct {
	product string
	qty     int
}

func sales(order string, lines ...saleLine) []byte {
	items := ""
	for i, l := range lines {
		if i > 0 {
			items += ","
		}
		items += fmt.Sprintf(`{"product_id":%q,"quantity":%d}`, l.product, l.qty)
	}
	return fmt.Appendf(nil, `{"event_id":"e","order_no":%q,"occurred_at":"2026-12-01T00:00:00Z","items":[%s]}`, order, items)
}

func TestSalesAreRecordedOnceForTheSameOrderNo(t *testing.T) {
	t.Parallel()
	p1, p2 := uuid.NewV7(), uuid.NewV7()
	f := newFake()
	f.products[p1], f.products[p2] = true, true
	c := Consumer{Catalog: f}
	for i := range 3 {
		if err := c.OrderSalesAccumulated(context.Background(), sales("O1", saleLine{p1.String(), 2}, saleLine{p2.String(), 5})); err != nil {
			t.Fatalf("第 %d 次：%v", i+1, err)
		}
	}
	if f.sales[p1] != 2 || f.sales[p2] != 5 {
		t.Fatalf("銷量 = %v，同一個 order_no 重送只記一次，要 p1=2、p2=5", f.sales)
	}
	if err := c.OrderSalesAccumulated(context.Background(), sales("O2", saleLine{p1.String(), 1})); err != nil || f.sales[p1] != 3 {
		t.Fatalf("另一個 order_no 要照記：err = %v、p1 = %d", err, f.sales[p1])
	}
}

func TestSalesSkipsEmptyIdsAndNonPositiveQuantities(t *testing.T) {
	t.Parallel()
	p := uuid.NewV7()
	f := newFake()
	f.products[p] = true
	body := sales("O1", saleLine{"", 3}, saleLine{p.String(), 0}, saleLine{p.String(), -1}, saleLine{p.String(), 4})
	if err := (Consumer{Catalog: f}).OrderSalesAccumulated(context.Background(), body); err != nil || f.sales[p] != 4 {
		t.Fatalf("err = %v、銷量 %d，只記 quantity 4 那筆", err, f.sales[p])
	}
	if err := (Consumer{Catalog: f}).OrderSalesAccumulated(context.Background(), sales("O2", saleLine{"", 3})); err != nil || f.orders["O2"] {
		t.Fatalf("全部被略過要整則略過且不登記訂單：err = %v", err)
	}
}

func TestSalesBadPayloadsArePermanent(t *testing.T) {
	t.Parallel()
	p := uuid.NewV7()
	f := newFake()
	f.products[p] = true
	for name, body := range map[string][]byte{
		"不是 JSON":            []byte(`{`),
		"缺 order_no":         []byte(`{"items":[{"product_id":"` + p.String() + `","quantity":1}]}`),
		"product_id 不是 UUID": sales("O1", saleLine{"12345", 1}),
		"商品不存在":              sales("O1", saleLine{uuid.NewV7().String(), 1}),
	} {
		err := (Consumer{Catalog: f}).OrderSalesAccumulated(context.Background(), body)
		if err == nil || !isPermanent(err) || len(f.sales) != 0 || f.orders["O1"] {
			t.Errorf("%s：err = %v，要 permanent 且不記銷量", name, err)
		}
	}
}

func TestSalesRetriesStorageFailures(t *testing.T) {
	t.Parallel()
	p := uuid.NewV7()
	f := newFake()
	f.err = errors.New("db down")
	if err := (Consumer{Catalog: f}).OrderSalesAccumulated(context.Background(), sales("O1", saleLine{p.String(), 1})); err == nil || isPermanent(err) {
		t.Fatalf("資料庫錯誤要可重試：%v", err)
	}
}
