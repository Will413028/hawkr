package catalogserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// fakeCatalog 照 catalog 的庫存規則記帳：冪等鍵全域唯一（扣減與歸還共用一個空間）、同鍵同內容重送不再改庫存、歸還指向原扣減、庫存不足整次不生效。
type fakeCatalog struct {
	stock map[uuid.UUID]int32
	ops   map[string]fakeOp
	snaps map[uuid.UUID]catalog.SkuSnapshot
	calls int
	err   error
	// SkuCartInfo 與 CheckoutSnapshot 收到的 ID（每次呼叫累加）。
	queried []uuid.UUID
}

type fakeOp struct {
	kind  string
	items []catalog.StockItem
}

func newFake(stock map[uuid.UUID]int32) *fakeCatalog {
	return &fakeCatalog{stock: stock, ops: map[string]fakeOp{}, snaps: map[uuid.UUID]catalog.SkuSnapshot{}}
}

func (f *fakeCatalog) DeductStock(_ context.Context, key string, items []catalog.StockItem) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	if key == "" || len(items) == 0 {
		return catalog.ErrInvalidStockInput
	}
	if op, ok := f.ops[key]; ok {
		if op.kind != "DEDUCT" {
			return catalog.ErrIdempotencyKeyReused
		}
		return nil
	}
	for _, it := range items {
		have, ok := f.stock[it.SkuID]
		if !ok {
			return catalog.ErrSkuNotFound
		}
		if have < it.Quantity {
			return &catalog.InsufficientStockError{SkuID: it.SkuID}
		}
	}
	for _, it := range items {
		f.stock[it.SkuID] -= it.Quantity
	}
	f.ops[key] = fakeOp{"DEDUCT", items}
	return nil
}

func (f *fakeCatalog) RestoreStock(_ context.Context, key, deductKey string, items []catalog.StockItem) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	if key == "" || len(items) == 0 {
		return catalog.ErrInvalidStockInput
	}
	if op, ok := f.ops[key]; ok {
		if op.kind != "RESTORE" {
			return catalog.ErrIdempotencyKeyReused
		}
		return nil
	}
	orig, ok := f.ops[deductKey]
	if !ok || orig.kind != "DEDUCT" {
		return catalog.ErrDeductionNotFound
	}
	for _, it := range items {
		var deducted int32
		for _, o := range orig.items {
			if o.SkuID == it.SkuID {
				deducted = o.Quantity
			}
		}
		if it.Quantity > deducted {
			return catalog.ErrRestoreExceedsDeduction
		}
	}
	for _, it := range items {
		f.stock[it.SkuID] += it.Quantity
	}
	f.ops[key] = fakeOp{"RESTORE", items}
	return nil
}

func (f *fakeCatalog) CheckoutSnapshot(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]catalog.SkuSnapshot, error) {
	return f.SkuCartInfo(ctx, ids)
}

func (f *fakeCatalog) SkuCartInfo(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]catalog.SkuSnapshot, error) {
	f.queried = append(f.queried, ids...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]catalog.SkuSnapshot{}
	for _, id := range ids {
		if s, ok := f.snaps[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func server(f *fakeCatalog) *Server {
	return New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// line 是一個品項：SKU 與數量（數字字串）。
type line struct {
	sku uuid.UUID
	qty string
}

func items(lines ...line) []*pb.StockItem {
	var out []*pb.StockItem
	for _, l := range lines {
		out = append(out, &pb.StockItem{SkuId: l.sku.String(), Quantity: l.qty})
	}
	return out
}

func deduct(t *testing.T, s *Server, tx string, in []*pb.StockItem) *pb.StockOperationResponse {
	t.Helper()
	r, err := s.DeductStock(context.Background(), connect.NewRequest(&pb.DeductStockRequest{StockTxId: tx, Items: in}))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg
}

func restore(t *testing.T, s *Server, tx string, in []*pb.StockItem) *pb.StockOperationResponse {
	t.Helper()
	r, err := s.RestoreStock(context.Background(), connect.NewRequest(&pb.RestoreStockRequest{StockTxId: tx, Items: in}))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg
}

func TestDeductIsIdempotentOnStockTxID(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{sku: 10})
	s := server(f)
	for i := range 2 {
		if r := deduct(t, s, "tx-1", items(line{sku, "3"})); r.GetStatus() != statusSuccess {
			t.Fatalf("第 %d 次：status = %s（%s），要 SUCCESS", i+1, r.GetStatus(), r.GetMessage())
		}
	}
	if f.stock[sku] != 7 {
		t.Fatalf("庫存 = %d，同一個 stock_tx_id 送兩次只扣一次，要 7", f.stock[sku])
	}
	if got := deduct(t, s, "tx-2", items(line{sku, "3"})); got.GetStatus() != statusSuccess || f.stock[sku] != 4 {
		t.Fatalf("另一個 stock_tx_id 要照扣：%s、庫存 %d", got.GetStatus(), f.stock[sku])
	}
}

func TestRestoreIsIdempotentOnStockTxID(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{sku: 10})
	s := server(f)
	deduct(t, s, "tx-1", items(line{sku, "4"}))
	for i := range 2 {
		if r := restore(t, s, "tx-1", items(line{sku, "4"})); r.GetStatus() != statusSuccess {
			t.Fatalf("第 %d 次：status = %s（%s），要 SUCCESS", i+1, r.GetStatus(), r.GetMessage())
		}
	}
	if f.stock[sku] != 10 {
		t.Fatalf("庫存 = %d，歸還送兩次只還一次，要 10", f.stock[sku])
	}
}

func TestRestoreWithoutTheOriginalDeductionIsNoop(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{sku: 10})
	r := restore(t, server(f), "never-deducted", items(line{sku, "1"}))
	if r.GetStatus() != statusNoopNotFound || f.stock[sku] != 10 {
		t.Fatalf("status = %s、庫存 %d，找不到原扣減要 NOOP_NOT_FOUND 且不動庫存", r.GetStatus(), f.stock[sku])
	}
}

func TestRestoreBeyondTheDeductionIsRefused(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{sku: 10})
	s := server(f)
	deduct(t, s, "tx-1", items(line{sku, "2"}))
	r := restore(t, s, "tx-1", items(line{sku, "5"}))
	if r.GetStatus() != statusSystemError || f.stock[sku] != 8 {
		t.Fatalf("status = %s、庫存 %d，超過原扣減量要拒絕且不動庫存", r.GetStatus(), f.stock[sku])
	}
}

func TestInsufficientStockReportsRequestedAndAvailable(t *testing.T) {
	t.Parallel()
	a, b := uuid.NewV7(), uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{a: 5, b: 1})
	f.snaps[b] = catalog.SkuSnapshot{SkuID: b, StockQty: 1}
	r := deduct(t, server(f), "tx-1", items(line{a, "2"}, line{b, "3"}))
	if r.GetStatus() != statusInsufficientStock || len(r.GetInsufficientItems()) != 1 {
		t.Fatalf("status = %s、明細 %v，要 INSUFFICIENT_STOCK 與一筆明細", r.GetStatus(), r.GetInsufficientItems())
	}
	got := r.GetInsufficientItems()[0]
	if got.GetSkuId() != b.String() || got.GetRequestedQty() != "3" || got.GetAvailableQty() != "1" {
		t.Fatalf("明細 = %v，要 sku %s、requested 3、available 1", got, b)
	}
	if f.stock[a] != 5 {
		t.Fatalf("庫存不足整次不生效：a = %d，要 5", f.stock[a])
	}
}

func TestMissingSkuIsReportedAsInsufficientWithZeroAvailable(t *testing.T) {
	t.Parallel()
	known, gone, deleted := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{known: 5})
	f.snaps[known] = catalog.SkuSnapshot{SkuID: known, StockQty: 5}
	f.snaps[deleted] = catalog.SkuSnapshot{SkuID: deleted, StockQty: 9, Deleted: true}
	r := deduct(t, server(f), "tx-1", items(line{known, "1"}, line{gone, "1"}, line{deleted, "2"}))
	if r.GetStatus() != statusInsufficientStock || len(r.GetInsufficientItems()) != 2 {
		t.Fatalf("status = %s、明細 %v，查無與已刪除的 SKU 要各一筆", r.GetStatus(), r.GetInsufficientItems())
	}
	for _, it := range r.GetInsufficientItems() {
		if it.GetAvailableQty() != "0" || it.GetSkuId() == known.String() {
			t.Fatalf("明細 %v：available 要 0，而且不含存在的 SKU", it)
		}
	}
}

func TestInvalidStockRequestsAreSystemErrorWithoutCallingCatalog(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	for name, c := range map[string]struct {
		tx string
		in []*pb.StockItem
	}{
		"缺 stock_tx_id":     {"", items(line{sku, "1"})},
		"沒有品項":              {"tx", nil},
		"sku_id 不是 UUID":    {"tx", []*pb.StockItem{{SkuId: "123", Quantity: "1"}}},
		"sku_id 全零":         {"tx", []*pb.StockItem{{SkuId: uuid.Nil().String(), Quantity: "1"}}},
		"quantity 不是數字":     {"tx", []*pb.StockItem{{SkuId: sku.String(), Quantity: "x"}}},
		"quantity 為零":       {"tx", items(line{sku, "0"})},
		"quantity 為負":       {"tx", items(line{sku, "-1"})},
		"quantity 超過 int32": {"tx", items(line{sku, "4294967296"})},
	} {
		f := newFake(map[uuid.UUID]int32{sku: 5})
		if r := deduct(t, server(f), c.tx, c.in); r.GetStatus() != statusSystemError || f.calls != 0 {
			t.Errorf("%s：status = %s、catalog 呼叫 %d 次，要 SYSTEM_ERROR 且不呼叫 catalog", name, r.GetStatus(), f.calls)
		}
		if r := restore(t, server(f), c.tx, c.in); r.GetStatus() != statusSystemError || f.calls != 0 {
			t.Errorf("%s（歸還）：status = %s、catalog 呼叫 %d 次，要 SYSTEM_ERROR 且不呼叫 catalog", name, r.GetStatus(), f.calls)
		}
	}
}

func TestStorageFailureIsSystemErrorNotSuccess(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(map[uuid.UUID]int32{sku: 5})
	f.err = errors.New("db down")
	s := server(f)
	if r := deduct(t, s, "tx", items(line{sku, "1"})); r.GetStatus() != statusSystemError {
		t.Fatalf("扣減：status = %s，要 SYSTEM_ERROR", r.GetStatus())
	}
	if r := restore(t, s, "tx", items(line{sku, "1"})); r.GetStatus() != statusSystemError {
		t.Fatalf("歸還：status = %s，要 SYSTEM_ERROR", r.GetStatus())
	}
}

func snap(sku, product uuid.UUID, price int64, compare *int64, stock int32, active bool) catalog.SkuSnapshot {
	return catalog.SkuSnapshot{
		SkuID: sku, ProductID: product, StoreID: 7, ProductTitle: "手工皂", SkuName: "薰衣草 / 大", CoverKey: "img/1",
		Price: price, CompareAtPrice: compare, StockQty: stock, Status: catalog.StatusActive, Active: active, Deleted: !active,
	}
}

func TestGetProductSimpleMapsTheSnapshot(t *testing.T) {
	t.Parallel()
	sku, product := uuid.NewV7(), uuid.NewV7()
	compare := int64(150)
	f := newFake(nil)
	f.snaps[sku] = snap(sku, product, 120, &compare, 9, true)
	r, err := server(f).GetProductSimple(context.Background(), connect.NewRequest(&pb.GetProductSimpleRequest{SkuId: sku.String()}))
	if err != nil {
		t.Fatal(err)
	}
	d := r.Msg.GetData()
	if r.Msg.GetCode() != codeOK || d.GetSkuId() != sku.String() || d.GetProductId() != product.String() || d.GetStoreId() != "7" || !d.GetIsActive() ||
		d.GetName() != "手工皂" || d.GetSkuName() != "薰衣草 / 大" || d.GetImageObjectKey() != "img/1" ||
		d.GetPrice() != "120" || d.GetListPrice() != "150" || d.GetMaxLimit() != "9" || len(d.GetDeliveryTemps()) != 0 {
		t.Fatalf("回應 = code %d %v", r.Msg.GetCode(), d)
	}
}

func TestGetProductSimpleListPriceEqualsPriceWithoutCompareAt(t *testing.T) {
	t.Parallel()
	sku := uuid.NewV7()
	f := newFake(nil)
	f.snaps[sku] = snap(sku, uuid.NewV7(), 120, nil, 1, true)
	r, _ := server(f).GetProductSimple(context.Background(), connect.NewRequest(&pb.GetProductSimpleRequest{SkuId: sku.String()}))
	if r.Msg.GetData().GetPrice() != "120" || r.Msg.GetData().GetListPrice() != "120" {
		t.Fatalf("沒有原價時 list_price 同售價：%v", r.Msg.GetData())
	}
}

func TestGetProductSimpleRejectsBadAndMissingSkus(t *testing.T) {
	t.Parallel()
	s := server(newFake(nil))
	for name, c := range map[string]struct {
		id   string
		code int32
	}{"空": {"", codeInvalidParam}, "不是 UUID": {"123", codeInvalidParam}, "全零": {uuid.Nil().String(), codeInvalidParam}, "不存在": {uuid.NewV7().String(), codeNotFound}} {
		r, err := s.GetProductSimple(context.Background(), connect.NewRequest(&pb.GetProductSimpleRequest{SkuId: c.id}))
		if err != nil || r.Msg.GetCode() != c.code || r.Msg.GetData() != nil {
			t.Errorf("%s：code = %d err = %v data = %v，要 %d 且沒有 data", name, r.Msg.GetCode(), err, r.Msg.GetData(), c.code)
		}
	}
}

func TestBatchGetSkuCartInfoOmitsMissingAndKeepsInactive(t *testing.T) {
	t.Parallel()
	live, delisted, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f := newFake(nil)
	f.snaps[live] = snap(live, uuid.NewV7(), 100, nil, 3, true)
	f.snaps[delisted] = snap(delisted, uuid.NewV7(), 100, nil, 3, false)
	r, err := server(f).BatchGetSkuCartInfo(context.Background(), connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: []string{live.String(), delisted.String(), gone.String(), "garbage"}}))
	if err != nil || r.Msg.GetCode() != codeOK {
		t.Fatalf("code = %d err = %v", r.Msg.GetCode(), err)
	}
	got := r.Msg.GetSkus()
	if len(got) != 2 || !got[live.String()].GetIsActive() || got[delisted.String()].GetIsActive() || got[live.String()].GetStockQty() != "3" || got[live.String()].GetPrice() != "100" {
		t.Fatalf("skus = %v，要 live（上架）與 delisted（is_active 為假），不含查無與格式錯誤的", got)
	}
}

func TestBatchCallsTreatNilUUIDAsUnknownAndDoNotQueryIt(t *testing.T) {
	t.Parallel()
	live := uuid.NewV7()
	f := newFake(nil)
	f.snaps[live] = snap(live, uuid.NewV7(), 100, nil, 3, true)
	s := server(f)
	nilID := uuid.Nil().String()

	cart, err := s.BatchGetSkuCartInfo(context.Background(), connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: []string{nilID, live.String()}}))
	if err != nil || cart.Msg.GetCode() != codeOK || len(cart.Msg.GetSkus()) != 1 {
		t.Fatalf("cart：code = %d err = %v skus = %v，要只回 live", cart.Msg.GetCode(), err, cart.Msg.GetSkus())
	}
	co, err := s.BatchGetProductForCheckout(context.Background(), connect.NewRequest(&pb.BatchGetProductForCheckoutRequest{SkuIds: []string{nilID, live.String()}}))
	if err != nil || co.Msg.GetCode() != codeOK || len(co.Msg.GetData()) != 1 {
		t.Fatalf("checkout：code = %d err = %v data = %v，要只回 live", co.Msg.GetCode(), err, co.Msg.GetData())
	}
	for _, id := range f.queried {
		if id == uuid.Nil() {
			t.Fatalf("catalog 收到了全零的 ID：%v", f.queried)
		}
	}

	only, err := s.BatchGetSkuCartInfo(context.Background(), connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: []string{nilID}}))
	if err != nil || only.Msg.GetCode() != codeOK || len(only.Msg.GetSkus()) != 0 {
		t.Fatalf("只有全零：code = %d err = %v skus = %v，要空結果", only.Msg.GetCode(), err, only.Msg.GetSkus())
	}
}

func TestBatchLimitsAreRejectedWithInvalidParam(t *testing.T) {
	t.Parallel()
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = uuid.NewV7().String()
		}
		return out
	}
	s := server(newFake(nil))
	if r, _ := s.BatchGetSkuCartInfo(context.Background(), connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: ids(201)})); r.Msg.GetCode() != codeInvalidParam {
		t.Errorf("cart 201 個：code = %d，要 400", r.Msg.GetCode())
	}
	if r, _ := s.BatchGetSkuCartInfo(context.Background(), connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: ids(200)})); r.Msg.GetCode() != codeOK {
		t.Errorf("cart 200 個：code = %d，要 0", r.Msg.GetCode())
	}
	if r, _ := s.BatchGetProductForCheckout(context.Background(), connect.NewRequest(&pb.BatchGetProductForCheckoutRequest{SkuIds: ids(101)})); r.Msg.GetCode() != codeInvalidParam {
		t.Errorf("checkout 101 個：code = %d，要 400", r.Msg.GetCode())
	}
}

func TestBatchGetProductForCheckoutCarriesPriceFieldsAndNoShippingData(t *testing.T) {
	t.Parallel()
	onSale, plain, archived := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	compare := int64(150)
	f := newFake(nil)
	f.snaps[onSale] = snap(onSale, uuid.NewV7(), 120, &compare, 4, true)
	f.snaps[plain] = snap(plain, uuid.NewV7(), 80, nil, 0, true)
	f.snaps[archived] = snap(archived, uuid.NewV7(), 80, nil, 2, false)
	r, err := server(f).BatchGetProductForCheckout(context.Background(), connect.NewRequest(&pb.BatchGetProductForCheckoutRequest{SkuIds: []string{onSale.String(), plain.String(), archived.String()}}))
	if err != nil || r.Msg.GetCode() != codeOK {
		t.Fatalf("code = %d err = %v", r.Msg.GetCode(), err)
	}
	d := r.Msg.GetData()
	if len(d) != 3 {
		t.Fatalf("data = %v，要三筆（含已下架）", d)
	}
	if x := d[onSale.String()]; x.GetPrice() != "120" || x.GetListPrice() != "150" || x.GetSalePrice() != "120" || x.GetStockQty() != "4" || !x.GetIsActive() || x.GetProductName() != "手工皂" || x.GetSkuName() != "薰衣草 / 大" {
		t.Errorf("特價 SKU = %v", x)
	}
	if x := d[plain.String()]; x.GetPrice() != "80" || x.GetListPrice() != "80" || x.GetSalePrice() != "" {
		t.Errorf("沒有原價的 SKU：list_price 同售價、sale_price 空：%v", x)
	}
	if x := d[archived.String()]; x.GetIsActive() {
		t.Errorf("已下架的 SKU is_active 要為假：%v", x)
	}
	for id, x := range d {
		if x.GetWeight() != "" || x.GetTemperature() != "" || len(x.GetShippingMethods()) != 0 || x.GetLengthCm() != "" {
			t.Errorf("%s：配送與重量資料不在 catalog，不得填值：%v", id, x)
		}
	}
}
