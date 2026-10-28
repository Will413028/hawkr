package orderprovisioning

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 這支測試守的是 ProcessFunc 的**分類判斷**，不是「跑得完」。
//
// 四件錯了不會有人發現的事：
//  1. 把終態拒絕當可重試 ⇒ acceptance 永遠停在 CREATING_ORDER，前端一直顯示「訂單建立中」。
//  2. 把可重試當終態 ⇒ 一次 DB 抖動就把一筆成交打成 ORDER_FAILED。
//  3. lane 分錯賣方 ⇒ 撥款與開票的法源整組錯掉，而訂單、付款、帳本全部正常。
//  4. 已建單還重送 ⇒ 靠 order-service 的冪等擋著，但我們自己的第 2 步才是第一道。
//
// M1 變異：把 classifyResponse 的 default 改成 return nil ⇒ rejectsWithOrderServiceRejected
// 轉紅；把 500 也歸進終態 ⇒ retriesOnOrderServiceInternalError 轉紅；第 2 步的狀態檢查
// 拿掉 ⇒ skipsWhenAlreadyOrderCreated 轉紅。

const (
	testAcceptanceID = int64(7505210956461375488)
	testBuyerID      = identity.BuyerID(9001)
	testStoreID      = int64(2636840881)
	testSettlement   = int64(7489859548420286732)
)

type recordedFailure struct {
	acceptanceID int64
	reason       string
}

type fakeStore struct {
	acceptance repository.BuyerQuoteAcceptance
	version    repository.SellerResponseVersion
	snapshot   repository.ConfirmedRfqSnapshot

	attachErr error
	attached  struct {
		orderNo         string
		paymentDeadline time.Time
		expectedVersion int32
		calls           int
	}
	failures       []recordedFailure
	outboxFailures []recordedFailure
}

func (f *fakeStore) FindBuyerQuoteAcceptanceByIDAndBuyer(_ context.Context, id int64, buyerID identity.BuyerID) (repository.BuyerQuoteAcceptance, error) {
	if id != f.acceptance.ID || buyerID != f.acceptance.BuyerID {
		return repository.BuyerQuoteAcceptance{}, repository.ErrBuyerQuoteAcceptanceNotFound
	}
	return f.acceptance, nil
}

func (f *fakeStore) FindSellerResponseVersion(_ context.Context, _ int64, _ int32) (repository.SellerResponseVersion, error) {
	return f.version, nil
}

func (f *fakeStore) FindConfirmedRfqSnapshotByIDAndBuyer(_ context.Context, _ int64, _ identity.BuyerID) (repository.ConfirmedRfqSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeStore) AttachOrderToAcceptance(_ context.Context, _ int64, expectedVersion int32, orderNo string, paymentDeadline time.Time) (repository.BuyerQuoteAcceptance, error) {
	f.attached.calls++
	f.attached.orderNo = orderNo
	f.attached.paymentDeadline = paymentDeadline
	f.attached.expectedVersion = expectedVersion
	if f.attachErr != nil {
		return repository.BuyerQuoteAcceptance{}, f.attachErr
	}
	return f.acceptance, nil
}

func (f *fakeStore) MarkAcceptanceOrderFailed(_ context.Context, id int64, _ int32, reason string) (repository.BuyerQuoteAcceptance, error) {
	f.failures = append(f.failures, recordedFailure{acceptanceID: id, reason: reason})
	return f.acceptance, nil
}

func (f *fakeStore) MarkAcceptanceOrderFailedByOutboxID(_ context.Context, outboxID int64, reason string) error {
	f.outboxFailures = append(f.outboxFailures, recordedFailure{acceptanceID: outboxID, reason: reason})
	return nil
}

type fakeCreator struct {
	result RfqOrderResult
	err    error
	calls  []RfqOrderCommand
}

func (f *fakeCreator) CreateRfqOrder(_ context.Context, cmd RfqOrderCommand) (RfqOrderResult, error) {
	f.calls = append(f.calls, cmd)
	return f.result, f.err
}

func newFixture(t *testing.T, lane string) (*processor, *fakeStore, *fakeCreator, *Row) {
	t.Helper()
	store := &fakeStore{
		acceptance: repository.BuyerQuoteAcceptance{
			ID:                     testAcceptanceID,
			Version:                3,
			BuyerID:                testBuyerID,
			SellerResponseID:       555,
			SellerResponseVersion:  2,
			ConfirmedRfqSnapshotID: 777,
			Lane:                   lane,
			AcceptedDealPrice:      30800,
			GoodsAmount:            28000,
			ShippingAmount:         800,
			TaxAmount:              2000,
			OrderProvisioningState: repository.OrderProvisioningCreating,
		},
		version: repository.SellerResponseVersion{
			OfferedQuantity:     "200",
			OfferedQuantityUnit: "公斤",
		},
		snapshot: repository.ConfirmedRfqSnapshot{
			CanonicalPayloadJSON: json.RawMessage(
				`{"item":"文旦柚","itemResolution":{"kind":"CATEGORY_CONFIRMED","label":"文旦柚"}}`),
		},
	}
	if lane == repository.SellerResponseLanePlatform {
		id := testStoreID
		store.acceptance.StoreID = &id
	}
	creator := &fakeCreator{result: RfqOrderResult{
		Code:            0,
		OrderNo:         "RFQ20260921001",
		PaymentDeadline: time.Date(2026, 9, 21, 22, 0, 0, 0, time.UTC),
	}}
	p := &processor{store: store, orders: creator, settlementStoreID: testSettlement, logger: zap.NewNop()}
	row := &Row{
		ID:           42,
		AcceptanceID: testAcceptanceID,
		EventType:    repository.OrderProvisioningOutboxEventOrderShellRequested,
		Payload:      json.RawMessage(`{"schemaVersion":1,"acceptanceId":"7505210956461375488","buyerId":"9001","requestId":"1"}`),
	}
	return p, store, creator, row
}

func TestProcess_PlatformLaneUsesSellerOwnStore(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLanePlatform)

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}

	if len(creator.calls) != 1 {
		t.Fatalf("expected exactly one CreateRfqOrder call, got %d", len(creator.calls))
	}
	cmd := creator.calls[0]
	if cmd.StoreID != testStoreID {
		t.Errorf("PLATFORM lane store id = %d, want %d（賣方就是那個平台賣家）", cmd.StoreID, testStoreID)
	}
	// unit_price ＝ goods + tax；運費另計在 shipping_fee，03 §3.2 的對帳式因此成立。
	if cmd.UnitPrice != 30000 {
		t.Errorf("unit price = %d, want 30000 (goods 28000 + tax 2000)", cmd.UnitPrice)
	}
	if cmd.TotalAmount != 30800 || cmd.ShippingFee != 800 {
		t.Errorf("amounts = total %d / shipping %d, want 30800 / 800", cmd.TotalAmount, cmd.ShippingFee)
	}
	// 真實數量與單位寫在品名裡（⛔ 不得為 sentinel）。
	if cmd.SkuName != "文旦柚 200 公斤" {
		t.Errorf("sku name = %q, want %q", cmd.SkuName, "文旦柚 200 公斤")
	}
	if store.attached.calls != 1 || store.attached.orderNo != "RFQ20260921001" {
		t.Errorf("attach = %d calls / orderNo %q", store.attached.calls, store.attached.orderNo)
	}
	if store.attached.expectedVersion != 3 {
		t.Errorf("CAS expected version = %d, want 3", store.attached.expectedVersion)
	}
	if len(store.failures) != 0 {
		t.Errorf("unexpected terminal failure: %+v", store.failures)
	}
}

func TestProcess_ExternalLaneUsesSettlementStore(t *testing.T) {
	p, _, creator, row := newFixture(t, repository.SellerResponseLaneExternal)

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if creator.calls[0].StoreID != testSettlement {
		t.Errorf("EXTERNAL lane store id = %d, want settlement store %d",
			creator.calls[0].StoreID, testSettlement)
	}
}

func TestProcess_ExternalLaneFailsClosedWhenSettlementStoreMissing(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLaneExternal)
	p.settlementStoreID = 0

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process should swallow terminal rejection, got %v", err)
	}
	if len(creator.calls) != 0 {
		t.Fatalf("must not call order-service without a settlement store")
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonConfigurationMissing {
		t.Errorf("failures = %+v, want one CONFIGURATION_MISSING", store.failures)
	}
}

func TestProcess_UnknownLaneFailsClosed(t *testing.T) {
	p, store, creator, row := newFixture(t, "WISH_POOL")

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(creator.calls) != 0 {
		t.Fatal("⛔ 未知 lane 不得送進 order-service：這一格的輸出會成為撥款與開票的法源")
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonUnknownLane {
		t.Errorf("failures = %+v, want one UNKNOWN_LANE", store.failures)
	}
}

func TestProcess_SkipsWhenAlreadyOrderCreated(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLanePlatform)
	store.acceptance.OrderProvisioningState = repository.OrderProvisioningCreated

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(creator.calls) != 0 {
		t.Fatal("已是 ORDER_CREATED 就不得再建一次單")
	}
	if store.attached.calls != 0 {
		t.Fatal("已是終態就不該再 CAS")
	}
}

func TestProcess_SkipsWhenAlreadyOrderFailed(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLanePlatform)
	store.acceptance.OrderProvisioningState = repository.OrderProvisioningFailed

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(creator.calls) != 0 {
		t.Fatal("ORDER_FAILED 是終態，沒有回頭邊")
	}
}

func TestProcess_IdempotentHitStillWritesBackOrderNo(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLanePlatform)
	creator.result.Replayed = true

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	// ⛔ replayed 是冪等命中不是失敗：order-service 已回查既有訂單，acceptance 仍要拿到 order_no。
	if store.attached.calls != 1 || store.attached.orderNo != "RFQ20260921001" {
		t.Errorf("replayed 也必須回寫 order_no，got %d calls / %q", store.attached.calls, store.attached.orderNo)
	}
	if len(store.failures) != 0 {
		t.Errorf("replayed 不得被當成失敗：%+v", store.failures)
	}
}

func TestProcess_StaleCasIsSuccessNotConflict(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	store.attachErr = repository.ErrBuyerQuoteAcceptanceStale

	// 影響 0 列代表另一個副本先寫了——at-least-once 本來就允許兩個副本跑到這一步。
	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("stale CAS must be treated as success, got %v", err)
	}
}

func TestProcess_RejectsWithOrderServiceRejected(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: 4002, Message: "invalid status"}}

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("terminal rejection must not be retried: %v", err)
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonOrderServiceRejected {
		t.Errorf("failures = %+v, want one ORDER_SERVICE_REJECTED", store.failures)
	}
}

func TestProcess_MapsPlatformCollectionNotConfigured(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLaneExternal)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: bizcode.PlatformCollectionNotConfigured}}

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonConfigurationMissing {
		t.Errorf("failures = %+v, want one CONFIGURATION_MISSING", store.failures)
	}
}

func TestProcess_MapsUnknownCommerceModelToUnknownLane(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: bizcode.FulfillmentCommerceModelUnknown}}

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonUnknownLane {
		t.Errorf("failures = %+v, want one UNKNOWN_LANE", store.failures)
	}
}

func TestProcess_RetriesOnOrderServiceInternalError(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: orderServiceInternalErrorCode, Message: "系統錯誤"}}

	// 500 是抖動的候選 ⇒ 回 error 讓框架退避重試；⛔ 不得當場把 acceptance 打成終態，
	// 也不得落進「code=0 卻沒有訂單號」那條契約破壞分支。
	_, err := p.process(context.Background(), row)
	if err == nil {
		t.Fatal("order-service 的系統錯誤必須回 error 讓框架退避重試")
	}
	if len(store.failures) != 0 {
		t.Errorf("internal error 不得寫終態：%+v", store.failures)
	}
}

func TestProcess_TransportErrorIsRetryable(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{err: errors.New("connection refused")}

	_, err := p.process(context.Background(), row)
	if err == nil {
		t.Fatal("傳輸層錯誤必須回 error 讓框架退避重試")
	}
	if len(store.failures) != 0 {
		t.Errorf("傳輸層錯誤不得寫終態：%+v", store.failures)
	}
}

func TestProcess_SuccessWithoutOrderNoIsTerminal(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: 0, OrderNo: ""}}

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if store.attached.calls != 0 {
		t.Fatal("⛔ 沒有訂單號不得回寫 ORDER_CREATED：chk_prbqa_order_no_matches_state 綁死兩者")
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonOrderServiceRejected {
		t.Errorf("failures = %+v, want one ORDER_SERVICE_REJECTED", store.failures)
	}
}

func TestProcess_SuccessWithoutPaymentDeadlineIsTerminal(t *testing.T) {
	p, store, _, row := newFixture(t, repository.SellerResponseLanePlatform)
	p.orders = &fakeCreator{result: RfqOrderResult{Code: 0, OrderNo: "RFQ20260921001"}}

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if store.attached.calls != 0 {
		t.Fatal("⛔ 零值期限不得寫進 acceptance：0001-01-01 是一個已經過期的付款期限，而買家看得到它")
	}
	if len(store.failures) != 1 || store.failures[0].reason != FailureReasonOrderServiceRejected {
		t.Errorf("failures = %+v, want one ORDER_SERVICE_REJECTED", store.failures)
	}
}

func TestProcess_UnreadableAcceptanceFailsByOutboxID(t *testing.T) {
	p, store, creator, row := newFixture(t, repository.SellerResponseLanePlatform)
	row.AcceptanceID = 123456 // 與 fake 裡的 acceptance 對不上 ⇒ NotFound

	if _, err := p.process(context.Background(), row); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(creator.calls) != 0 {
		t.Fatal("讀不到 acceptance 就不該建單")
	}
	// 沒有 version 可以 CAS ⇒ 走 by-outbox-id 那一支。
	if len(store.outboxFailures) != 1 || store.outboxFailures[0].reason != FailureReasonOrderServiceRejected {
		t.Errorf("outboxFailures = %+v", store.outboxFailures)
	}
}
