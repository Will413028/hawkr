//go:build integration

package payment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

const (
	kindChargeCreate = "payment.charge_create"
	kindChargeSync   = "payment.charge_sync"
)

// chargeOrders 是訂單一方的 port 的測試替身：只回登記過的訂單；記下呼叫是否在 DB transaction 內。
type chargeOrders struct {
	mu     sync.Mutex
	orders map[string]PaymentOrder
	inTx   []bool
}

func (o *chargeOrders) OrderForPayment(ctx context.Context, orderNo string) (PaymentOrder, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := db.TxFrom(ctx)
	o.inTx = append(o.inTx, ok)
	order, found := o.orders[orderNo]
	if !found {
		return PaymentOrder{}, apperr.NotFound("沒有這張訂單")
	}
	return order, nil
}

func (o *chargeOrders) put(order PaymentOrder) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.orders[order.OrderNo] = order
}

type chargeFixture struct {
	db      *db.DB
	jobs    *queue
	inbox   *inbox.Inbox
	fake    *ChargeFake
	orders  *chargeOrders
	module  *Module
	now     time.Time
	changes []PaymentChanged
	// anomalies 是同 transaction 內收到的 MoneyAnomaly（告警事件），anomalyInTx 記下是否真的在 transaction 內。
	anomalies   []MoneyAnomaly
	anomalyInTx []bool
	// refundChanges 與 autoDone 是同 transaction 內收到的 RefundChanged 與 AutoRefundCompleted，refundEventInTx 記下兩者是否真的在 transaction 內。
	refundChanges   []RefundChanged
	autoDone        []AutoRefundCompleted
	refundEventInTx []bool
	inTx            []bool
	logs            bytes.Buffer
}

func setupCharge(t *testing.T) *chargeFixture {
	t.Helper()
	return setupChargeWith(t, nil)
}

// setupChargeWith 以 wrap 包住付款 provider（nil 就用 ChargeFake 本身）；收款帳戶的 Fake 同時登記，證明兩個 provider 的名稱不衝突。
func setupChargeWith(t *testing.T, wrap func(*ChargeFake) ChargeProvider) *chargeFixture {
	t.Helper()
	steps := []testdb.Step{testdb.Migrations(migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()}, migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()}, migrate.Module{Name: "payment", Dir: Migrations()})}
	f := &chargeFixture{db: pg.NewDB(t, steps...), jobs: &queue{}, fake: NewChargeFake(), orders: &chargeOrders{orders: map[string]PaymentOrder{}}, now: time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)}
	bus := events.NewBus()
	events.Subscribe(bus, func(ctx context.Context, e PaymentChanged) error {
		_, ok := db.TxFrom(ctx)
		f.changes = append(f.changes, e)
		f.inTx = append(f.inTx, ok)
		return nil
	})
	events.Subscribe(bus, func(ctx context.Context, e MoneyAnomaly) error {
		_, ok := db.TxFrom(ctx)
		f.anomalies = append(f.anomalies, e)
		f.anomalyInTx = append(f.anomalyInTx, ok)
		return nil
	})
	events.Subscribe(bus, func(ctx context.Context, e RefundChanged) error {
		_, ok := db.TxFrom(ctx)
		f.refundChanges = append(f.refundChanges, e)
		f.refundEventInTx = append(f.refundEventInTx, ok)
		return nil
	})
	events.Subscribe(bus, func(ctx context.Context, e AutoRefundCompleted) error {
		_, ok := db.TxFrom(ctx)
		f.autoDone = append(f.autoDone, e)
		f.refundEventInTx = append(f.refundEventInTx, ok)
		return nil
	})
	log := slog.New(slog.NewTextHandler(&f.logs, nil))
	clock := func() time.Time { return f.now }
	f.inbox = inbox.New(inbox.Deps{DB: f.db, Jobs: f.jobs, Now: clock, Logger: log})
	var charge ChargeProvider = f.fake
	if wrap != nil {
		charge = wrap(f.fake)
	}
	f.module = New(Deps{
		DB: f.db, Events: bus, Jobs: f.jobs, Inbox: f.inbox, Audit: adminaudit.New(f.db, clock), Provider: NewFake(), Owners: &owners{closed: map[int64]bool{}, emails: map[int64]string{}},
		Charge: charge, PaymentOrders: f.orders, Now: clock, Logger: log,
	})
	return f
}

func (f *chargeFixture) order(no string, buyerID, storeID, amount int64) {
	f.orders.put(PaymentOrder{OrderNo: no, BuyerID: buyerID, StoreID: storeID, Amount: amount, Payable: true, Description: "商品"})
}

func (f *chargeFixture) unpayable(no string, buyerID int64) {
	f.orders.put(PaymentOrder{OrderNo: no, BuyerID: buyerID, StoreID: 7, Amount: 100, Payable: false})
}

func (f *chargeFixture) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, sql, args...).Scan(&n))
	return n
}

func (f *chargeFixture) text(t *testing.T, sql string, args ...any) string {
	t.Helper()
	ctx := context.Background()
	var s *string
	must(t, f.db.Conn(ctx).QueryRow(ctx, sql, args...).Scan(&s))
	if s == nil {
		return ""
	}
	return *s
}

func (f *chargeFixture) externalOf(t *testing.T, orderNo string) string {
	t.Helper()
	return f.text(t, "SELECT external_payment_id FROM payment.payments WHERE order_no = $1", orderNo)
}

func (f *chargeFixture) statusOf(t *testing.T, orderNo string) PaymentStatus {
	t.Helper()
	return PaymentStatus(f.text(t, "SELECT status FROM payment.payments WHERE order_no = $1", orderNo))
}

func (f *chargeFixture) keyOf(t *testing.T, orderNo string) string {
	t.Helper()
	return f.text(t, "SELECT create_idempotency_key::text FROM payment.payments WHERE order_no = $1", orderNo)
}

func (f *chargeFixture) create(t *testing.T, buyerID int64, orderNo string) Payment {
	t.Helper()
	p, err := f.module.CreatePayment(context.Background(), buyerID, orderNo)
	must(t, err)
	return p
}

func (f *chargeFixture) paymentIDs(t *testing.T, kind string) []uuid.UUID {
	t.Helper()
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	var out []uuid.UUID
	for _, a := range f.jobs.args {
		if a.Kind() != kind {
			continue
		}
		raw, err := json.Marshal(a)
		must(t, err)
		var v struct {
			PaymentID uuid.UUID `json:"payment_id"`
		}
		must(t, json.Unmarshal(raw, &v))
		out = append(out, v.PaymentID)
	}
	return out
}

func (f *chargeFixture) sync(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	again, err := f.module.SyncPayment(context.Background(), id)
	must(t, err)
	return again
}

func (f *chargeFixture) receive(t *testing.T, eventID, body string) {
	t.Helper()
	ctx := context.Background()
	h := http.Header{}
	h.Set(ChargeFakeEventHeader, eventID)
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte(body)))
	must(t, f.inbox.Dispatch(ctx, ChargeFakeName, eventID))
}

func (f *chargeFixture) changesTo(status PaymentStatus) int {
	n := 0
	for _, c := range f.changes {
		if c.Status == status {
			n++
		}
	}
	return n
}

func (f *chargeFixture) noViolations(t *testing.T) {
	t.Helper()
	if v := f.fake.Violations(); len(v) != 0 {
		t.Fatalf("provider 的呼叫違反規則：%v", v)
	}
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	for _, in := range f.orders.inTx {
		if in {
			t.Fatal("訂單 port 在 DB transaction 內被呼叫")
		}
	}
}

// 建立付款：transaction 只寫「建立中」的列與補做 job，提交之後在請求內、transaction 外帶列上的冪等鍵建立 PaymentIntent 並回 client_secret；provider 與訂單 port 的呼叫都不在 transaction 內。
// mutation：把 provider 呼叫搬進寫列的 transaction → Violations 記錄 transaction 內的呼叫，失敗。
func TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-1", 11, 7, 250)
	p := f.create(t, 11, "O-1")
	if p.Status != PaymentAwaitingPayment || p.ClientSecret == "" || p.Amount != 250 || p.StoreID != 7 || p.BuyerID != 11 || p.Provider != ChargeFakeName {
		t.Fatalf("付款=%+v", p)
	}
	f.noViolations(t)
	if got := f.fake.Intents(); got != 1 {
		t.Fatalf("PaymentIntent %d 個", got)
	}
	if keys := f.fake.CreateKeys(); len(keys) != 1 || keys[0] != f.keyOf(t, "O-1") || len(keys[0]) != 36 {
		t.Fatalf("送出的冪等鍵=%v，列上的=%s", keys, f.keyOf(t, "O-1"))
	}
	if f.externalOf(t, "O-1") == "" || f.statusOf(t, "O-1") != PaymentAwaitingPayment {
		t.Fatalf("列沒有寫入外部 ID 與狀態：%s %s", f.externalOf(t, "O-1"), f.statusOf(t, "O-1"))
	}
	if got := f.paymentIDs(t, kindChargeCreate); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("補做 job=%v", got)
	}
	if f.jobs.outsideTx() {
		t.Fatal("job 在 transaction 外排入")
	}
	if f.changesTo(PaymentAwaitingPayment) != 1 || !f.inTx[0] {
		t.Fatalf("PaymentChanged=%+v inTx=%v", f.changes, f.inTx)
	}
}

// provider 失敗或逾時：回狀態 CREATING 的 pending（不是錯誤、沒有 client_secret），補做 job 已排，以同一把冪等鍵補做，最後只有一個 PaymentIntent。
// mutation：拿掉 ensureRow 排補做 job 的那行 → 沒有補做 job，失敗。
func TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-2", 11, 7, 100)
	f.fake.FailCreate(errors.New("provider 暫時不可用"))
	p := f.create(t, 11, "O-2")
	if p.Status != PaymentCreating || p.ClientSecret != "" {
		t.Fatalf("pending 的付款=%+v", p)
	}
	if got := f.paymentIDs(t, kindChargeCreate); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("失敗後沒有補做 job：%v", got)
	}
	if f.fake.Intents() != 0 || f.count(t, "SELECT count(*) FROM payment.payments WHERE create_called_at IS NOT NULL") != 1 {
		t.Fatalf("失敗的呼叫之前要先記 create_called_at；PaymentIntent %d 個", f.fake.Intents())
	}
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.statusOf(t, "O-2") != PaymentAwaitingPayment || f.fake.Intents() != 1 {
		t.Fatalf("補做之後狀態=%s，PaymentIntent %d 個", f.statusOf(t, "O-2"), f.fake.Intents())
	}
	if keys := f.fake.CreateKeys(); len(keys) != 2 || keys[0] != keys[1] || keys[0] != f.keyOf(t, "O-2") {
		t.Fatalf("兩次呼叫的冪等鍵=%v", keys)
	}
	f.noViolations(t)
}

// 回應遺失（PaymentIntent 已建立、呼叫方沒拿到）：補做以同一把鍵取回同一個 PaymentIntent，不會有第二個。
// mutation：補做時換新鍵 → 建出第二個 PaymentIntent，且送出的鍵不同，失敗。
func TestAReplayAfterALostResponseRecoversTheSameIntent(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-3", 11, 7, 100)
	f.fake.FailCreateAfterCreate(errors.New("回應遺失"))
	p := f.create(t, 11, "O-3")
	if p.Status != PaymentCreating || f.fake.Intents() != 1 {
		t.Fatalf("付款=%+v，PaymentIntent %d 個", p, f.fake.Intents())
	}
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.statusOf(t, "O-3") != PaymentAwaitingPayment || f.fake.Intents() != 1 || f.externalOf(t, "O-3") != f.fake.IntentFor("O-3")[0] {
		t.Fatalf("取回失敗：狀態=%s，PaymentIntent %d 個", f.statusOf(t, "O-3"), f.fake.Intents())
	}
}

// 結果不明（呼叫過、沒有外部 ID）過了 ResendWindow：不再呼叫 provider（鍵不再重送），只查回；查無要滿 NotFoundAfter 才進 CREATE_FAILED，
// 之前仍是 CREATING、同一把鍵；掃描不再排終局列的補做。買家再次建立時同一列換新鍵重設（仍是一張訂單一列）。
// mutation：create() 在過了 ResendWindow 之後仍呼叫 CreatePayment（拿掉 inResendWindow 判斷）→ 下面的呼叫次數斷言失敗。
// mutation：lookupCreate 拿掉 NotFoundAfter 的判斷（查無立刻 FAILED）→ 「滿等待之前仍是 CREATING」失敗。
func TestAnUnresolvedCreateIsLookedUpPastTheResendWindowAndFailsOnlyAfterTheNotFoundWait(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-4", 11, 7, 100)
	f.fake.FailCreate(errors.New("逾時"))
	p := f.create(t, 11, "O-4")
	oldKey := f.keyOf(t, "O-4")
	calls, _, _ := f.fake.Calls()
	start := f.now
	f.now = start.Add(PaymentResendWindow + time.Minute)
	if err := f.module.CreatePaymentIntent(context.Background(), p.ID, false); err == nil {
		t.Fatal("查無但還沒過最短等待，job 應回錯誤稍後再查")
	}
	if f.statusOf(t, "O-4") != PaymentCreating || f.keyOf(t, "O-4") != oldKey || f.fake.LookupCalls() != 1 {
		t.Fatalf("滿等待之前：%s，鍵 %s，查回 %d 次", f.statusOf(t, "O-4"), f.keyOf(t, "O-4"), f.fake.LookupCalls())
	}
	f.now = start.Add(PaymentNotFoundAfter + time.Minute)
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.statusOf(t, "O-4") != PaymentCreateFailed || f.text(t, "SELECT failure_reason FROM payment.payments WHERE order_no = 'O-4'") != "provider_not_found" {
		t.Fatalf("終局=%s", f.statusOf(t, "O-4"))
	}
	if after, _, _ := f.fake.Calls(); after != calls {
		t.Fatalf("過了視窗仍呼叫 CreatePayment：%d → %d 次", calls, after)
	}
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	created, _, err := f.module.SweepPayments(context.Background())
	must(t, err)
	if created != 0 || len(f.paymentIDs(t, kindChargeCreate)) != 0 {
		t.Fatalf("終局的列又被排補做：%d", created)
	}
	again := f.create(t, 11, "O-4")
	if again.ID != p.ID || again.Status != PaymentAwaitingPayment || f.keyOf(t, "O-4") == oldKey || f.count(t, "SELECT count(*) FROM payment.payments") != 1 {
		t.Fatalf("重設=%+v，鍵 %s → %s", again, oldKey, f.keyOf(t, "O-4"))
	}
}

// provider 永久拒絕：列進 CREATE_FAILED，CreatePayment 回這個狀態（不是 pending），補做 job 之後什麼都不呼叫。
func TestAPermanentRefusalPutsThePaymentInTheCreationTerminalState(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-5", 11, 7, 100)
	f.fake.FailCreate(ErrChargePermanent)
	p := f.create(t, 11, "O-5")
	if p.Status != PaymentCreateFailed || p.ClientSecret != "" {
		t.Fatalf("付款=%+v", p)
	}
	calls, _, _ := f.fake.Calls()
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if after, _, _ := f.fake.Calls(); after != calls || f.fake.Intents() != 0 {
		t.Fatalf("終局之後仍呼叫 provider：%d → %d", calls, after)
	}
}

// 一張訂單一個 PaymentIntent（D8）：同一張訂單再次建立（含並行）只有一列、一個 PaymentIntent、同一個外部 ID。
// mutation：schema 的 payments_order 不唯一 → 第二次寫列失敗（ON CONFLICT 找不到唯一索引），失敗。
func TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-6", 11, 7, 100)
	first := f.create(t, 11, "O-6")
	second := f.create(t, 11, "O-6")
	if first.ID != second.ID || f.fake.Intents() != 1 || f.count(t, "SELECT count(*) FROM payment.payments") != 1 {
		t.Fatalf("第二次建立：%+v / %+v，PaymentIntent %d 個", first, second, f.fake.Intents())
	}
	if second.Status != PaymentAwaitingPayment || second.ClientSecret == "" {
		t.Fatalf("第二次沒有重新取得 client_secret：%+v", second)
	}
	f.order("O-6b", 11, 7, 100)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.module.CreatePayment(context.Background(), 11, "O-6b"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := len(f.fake.IntentFor("O-6b")); got != 1 || f.count(t, "SELECT count(*) FROM payment.payments WHERE order_no = 'O-6b'") != 1 {
		t.Fatalf("並行建立：PaymentIntent %d 個", got)
	}
	f.noViolations(t)
}

// 建立付款的前置條件：訂單不可付、零元、不屬於呼叫者、重新送出時內容已變，都不寫列、不呼叫 provider；不能在 transaction 內呼叫。
func TestCreatePaymentRefusesWhatTheOrderOrTheCallerDoesNotAllow(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.unpayable("O-7", 11)
	f.orders.put(PaymentOrder{OrderNo: "O-7z", BuyerID: 11, StoreID: 7, Amount: 0, Payable: true})
	f.order("O-7o", 12, 7, 100)
	for _, c := range []struct {
		buyer int64
		no    string
		kind  apperr.Kind
	}{{11, "O-7", apperr.KindFailedPrecondition}, {11, "O-7z", apperr.KindFailedPrecondition}, {11, "O-7o", apperr.KindNotFound}, {11, "missing", apperr.KindNotFound}, {0, "O-7", apperr.KindInvalidInput}} {
		if _, err := f.module.CreatePayment(ctx, c.buyer, c.no); apperr.KindOf(err) != c.kind {
			t.Fatalf("buyer=%d order=%s：%v，預期 %v", c.buyer, c.no, err, c.kind)
		}
	}
	if f.count(t, "SELECT count(*) FROM payment.payments") != 0 || f.fake.Intents() != 0 {
		t.Fatal("被拒絕的建立寫了列或呼叫了 provider")
	}
	f.order("O-7c", 11, 7, 100)
	f.create(t, 11, "O-7c")
	f.order("O-7c", 11, 7, 999)
	if _, err := f.module.CreatePayment(ctx, 11, "O-7c"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("訂單金額變動之後再次建立：%v", err)
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.module.CreatePayment(ctx, 11, "O-7c")
		return err
	})
	if err == nil {
		t.Fatal("在 transaction 內建立付款沒有被拒絕")
	}
	f.noViolations(t)
}

// GetPayment：只回自己的付款；等待付款時向 provider 讀 client_secret；provider 那邊已往前時不回 secret 並排同步 job；讀取失敗回 Unavailable。
func TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-8", 11, 7, 100)
	created := f.create(t, 11, "O-8")
	got, err := f.module.GetPayment(ctx, 11, "O-8")
	must(t, err)
	if got.Status != PaymentAwaitingPayment || got.ClientSecret == "" || got.ClientSecret != created.ClientSecret {
		t.Fatalf("重讀=%+v", got)
	}
	if _, err := f.module.GetPayment(ctx, 12, "O-8"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("他人的付款：%v", err)
	}
	if _, err := f.module.GetPayment(ctx, 11, "nope"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的付款：%v", err)
	}
	f.fake.FailGet(errors.New("讀取失敗"))
	if _, err := f.module.GetPayment(ctx, 11, "O-8"); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("provider 讀取失敗：%v", err)
	}
	must(t, f.fake.Settle(f.externalOf(t, "O-8"), PaymentStateProcessing, f.now))
	before := len(f.paymentIDs(t, kindChargeSync))
	ahead, err := f.module.GetPayment(ctx, 11, "O-8")
	must(t, err)
	if ahead.ClientSecret != "" || len(f.paymentIDs(t, kindChargeSync)) != before+1 {
		t.Fatalf("provider 已往前：%+v，同步 job %d → %d", ahead, before, len(f.paymentIDs(t, kindChargeSync)))
	}
	f.noViolations(t)
}

// 重複的付款事件只處理一次：同一個事件 ID 收兩次，inbox 只有一列、只排一個派送 job；處理後狀態只改一次、PaymentChanged 只發一次；之後重複同步不再發事件。
// mutation：拿掉 inbox 的 ON CONFLICT 去重 → 第二次收到時寫入失敗，失敗；拿掉 applySnapshot 的「狀態改變才發布」→ 重複同步重發事件，失敗。
func TestADuplicatePaymentEventIsRecordedOnceAndPublishesOnce(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-9", 11, 7, 100)
	p := f.create(t, 11, "O-9")
	ext := f.externalOf(t, "O-9")
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	h := http.Header{}
	h.Set(ChargeFakeEventHeader, "evt_dup")
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte(ext)))
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte(ext)))
	if f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_dup'") != 1 || f.jobs.count("inbox.dispatch") != 1 {
		t.Fatalf("inbox %d 列，派送 job %d 個", f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = 'evt_dup'"), f.jobs.count("inbox.dispatch"))
	}
	must(t, f.inbox.Dispatch(ctx, ChargeFakeName, "evt_dup"))
	if got := f.paymentIDs(t, kindChargeSync); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("handler 排的同步 job=%v", got)
	}
	if f.processed(t, "evt_dup") {
		t.Fatal("handler 不該標記已處理，由涵蓋它的同步標記")
	}
	if again := f.sync(t, p.ID); again {
		t.Fatal("沒有新事件卻要求再跑一輪")
	}
	f.sync(t, p.ID)
	f.sync(t, p.ID)
	if f.changesTo(PaymentSucceeded) != 1 || f.statusOf(t, "O-9") != PaymentSucceeded || !f.processed(t, "evt_dup") {
		t.Fatalf("PaymentChanged(SUCCEEDED) %d 次，狀態=%s，已處理=%v", f.changesTo(PaymentSucceeded), f.statusOf(t, "O-9"), f.processed(t, "evt_dup"))
	}
	last := f.changes[len(f.changes)-1]
	if last.Previous != PaymentAwaitingPayment || last.OrderNo != "O-9" || last.PaidAt == nil || !last.PaidAt.Equal(f.now) || !f.inTx[len(f.inTx)-1] {
		t.Fatalf("事件=%+v inTx=%v", last, f.inTx)
	}
	if f.text(t, "SELECT external_charge_id FROM payment.payments WHERE order_no = 'O-9'") == "" {
		t.Fatal("入帳沒有記 charge")
	}
	f.noViolations(t)
}

func (f *chargeFixture) processed(t *testing.T, eventID string) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM inbox.events WHERE event_id = $1 AND processed_at IS NOT NULL", eventID) == 1
}

// handler 不信事件內容：事件宣稱已入帳、provider 實際還在等待付款時，狀態不變、不發事件；provider 真的入帳後才改。
// mutation：handler 改成依事件宣稱的狀態直接寫入、不排同步重讀 → 狀態變成 SUCCEEDED，失敗。
func TestTheEventHandlerNeverTrustsTheEventContent(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-10", 11, 7, 100)
	p := f.create(t, 11, "O-10")
	ext := f.externalOf(t, "O-10")
	f.receive(t, "evt_forged", ext+"|succeeded")
	f.sync(t, p.ID)
	if f.statusOf(t, "O-10") != PaymentAwaitingPayment || f.changesTo(PaymentSucceeded) != 0 {
		t.Fatalf("偽造的事件改了狀態：%s", f.statusOf(t, "O-10"))
	}
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.receive(t, "evt_real", ext+"|succeeded")
	f.sync(t, p.ID)
	if f.statusOf(t, "O-10") != PaymentSucceeded || f.changesTo(PaymentSucceeded) != 1 {
		t.Fatalf("真的入帳沒有生效：%s", f.statusOf(t, "O-10"))
	}
}

// 入帳是終局：之後讀到取消或等待的快照不覆蓋它（Java 的 FAILED 不覆蓋 PAID）；取消之後入帳的競態由 provider 的成功勝出。
func TestSucceededIsNeverOverwrittenAndALateSuccessBeatsACancel(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-11", 11, 7, 100)
	p := f.create(t, 11, "O-11")
	ext := f.externalOf(t, "O-11")
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.Settle(ext, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.FailAttempt(ext, "card_declined"))
	f.sync(t, p.ID)
	if f.statusOf(t, "O-11") != PaymentSucceeded || f.changesTo(PaymentSucceeded) != 1 || f.changesTo(PaymentCanceled) != 0 {
		t.Fatalf("入帳被覆蓋：%s，事件 %+v", f.statusOf(t, "O-11"), f.changes)
	}
	if !strings.Contains(f.logs.String(), "衝突") {
		t.Fatal("衝突沒有記警示")
	}
	f.order("O-11b", 11, 7, 100)
	q := f.create(t, 11, "O-11b")
	ext2 := f.externalOf(t, "O-11b")
	must(t, f.fake.Settle(ext2, PaymentStateCanceled, f.now))
	f.sync(t, q.ID)
	must(t, f.fake.Settle(ext2, PaymentStateSucceeded, f.now))
	f.sync(t, q.ID)
	if f.statusOf(t, "O-11b") != PaymentSucceeded {
		t.Fatalf("取消之後入帳：%s", f.statusOf(t, "O-11b"))
	}
}

// 付款嘗試失敗：PaymentIntent 回到等待付款並帶失敗原因碼，列記 last_error（同一個 PaymentIntent 上重試，D8）；之後成功清除。
func TestAFailedAttemptKeepsTheSameIntentAndRecordsTheReason(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-12", 11, 7, 100)
	p := f.create(t, 11, "O-12")
	ext := f.externalOf(t, "O-12")
	must(t, f.fake.Settle(ext, PaymentStateProcessing, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.FailAttempt(ext, "card_declined"))
	f.sync(t, p.ID)
	if got := f.text(t, "SELECT last_error FROM payment.payments WHERE order_no = 'O-12'"); got != "card_declined" || f.statusOf(t, "O-12") != PaymentAwaitingPayment {
		t.Fatalf("last_error=%q 狀態=%s", got, f.statusOf(t, "O-12"))
	}
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	if f.text(t, "SELECT last_error FROM payment.payments WHERE order_no = 'O-12'") != "" || f.fake.Intents() != 1 {
		t.Fatal("成功之後 last_error 沒有清除")
	}
}

// 快照與列對不起來（金額不符）：不寫入、不發事件、回 ErrChargePermanent（同步 job 取消）；不認得的狀態不寫入、當一般錯誤重試。
func TestASnapshotThatDisagreesWithTheRowIsNeverWritten(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-13", 11, 7, 100)
	p := f.create(t, 11, "O-13")
	ext := f.externalOf(t, "O-13")
	before := len(f.changes)
	must(t, f.fake.SetAmount(ext, 5))
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	if _, err := f.module.SyncPayment(ctx, p.ID); !errors.Is(err, ErrChargePermanent) {
		t.Fatalf("金額不符：%v", err)
	}
	must(t, f.fake.SetAmount(ext, 100))
	must(t, f.fake.SetState(ext, PaymentStateUnknown))
	_, err := f.module.SyncPayment(ctx, p.ID)
	if err == nil || errors.Is(err, ErrChargePermanent) {
		t.Fatalf("不認得的狀態應回一般錯誤：%v", err)
	}
	if f.statusOf(t, "O-13") != PaymentAwaitingPayment || len(f.changes) != before {
		t.Fatalf("不一致的快照被寫入：%s", f.statusOf(t, "O-13"))
	}
}

// 事件的其他情況：不處理的事件不寫 inbox；簽章或格式不合回 InvalidInput；對不到本地付款的事件只留紀錄並標記已處理、不排 job。
func TestOtherPaymentEventsAreIgnoredRejectedOrRecordedWithoutAJob(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	h := http.Header{}
	h.Set(ChargeFakeEventHeader, "evt_ign")
	h.Set(ChargeFakeIgnoredEventHeader, "1")
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte("fake_pi_9")))
	if f.count(t, "SELECT count(*) FROM inbox.events") != 0 {
		t.Fatal("不處理的事件寫了 inbox")
	}
	if err := f.module.ReceivePaymentEvent(ctx, http.Header{}, []byte("fake_pi_9")); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("格式不合：%v", err)
	}
	f.receive(t, "evt_unknown", "fake_pi_999")
	if !f.processed(t, "evt_unknown") || len(f.paymentIDs(t, kindChargeSync)) != 0 {
		t.Fatal("對不到本地付款的事件沒有標記已處理，或排了 job")
	}
}

// 補掃：閒置的建立中列補排補做 job；超過 readIdle 沒讀過的未結案付款補排同步；有舊的未處理事件的付款補排同步（任何狀態）。
func TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-14a", 11, 7, 100)
	f.fake.FailCreate(errors.New("逾時"))
	pending := f.create(t, 11, "O-14a")
	f.order("O-14b", 11, 7, 100)
	open := f.create(t, 11, "O-14b")
	f.order("O-14c", 11, 7, 100)
	done := f.create(t, 11, "O-14c")
	extDone := f.externalOf(t, "O-14c")
	must(t, f.fake.Settle(extDone, PaymentStateSucceeded, f.now))
	f.sync(t, done.ID)
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	h := http.Header{}
	h.Set(ChargeFakeEventHeader, "evt_late")
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte(extDone)))
	f.now = f.now.Add(40 * time.Minute)
	created, synced, err := f.module.SweepPayments(ctx)
	must(t, err)
	if created != 1 || synced != 2 {
		t.Fatalf("排入 補做 %d 同步 %d", created, synced)
	}
	if got := f.paymentIDs(t, kindChargeCreate); len(got) != 1 || got[0] != pending.ID {
		t.Fatalf("補做 job=%v", got)
	}
	syncs := f.paymentIDs(t, kindChargeSync)
	if len(syncs) != 2 || !containsID(syncs, open.ID) || !containsID(syncs, done.ID) {
		t.Fatalf("同步 job=%v", syncs)
	}
	f.noViolations(t)
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// 付款事件與收款帳戶事件各自登記在 inbox：付款 provider 與收款帳戶同名時組裝就 panic，不是靜默蓋掉其中一個的處理者。
func TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("同名的 provider 沒有讓組裝失敗")
		}
	}()
	setupChargeWith(t, func(f *ChargeFake) ChargeProvider { return sameNameAsAccount{f} })
}

type sameNameAsAccount struct{ *ChargeFake }

func (sameNameAsAccount) Name() string { return FakeName }

// 沒有設定付款 provider 時付款的呼叫回 FailedPrecondition，webhook 回 FailedPrecondition（入口回 503）。
func TestPaymentCallsWithoutAChargeProviderAreFailedPrecondition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if _, err := f.module.CreatePayment(ctx, 1, "O"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("CreatePayment：%v", err)
	}
	if _, err := f.module.GetPayment(ctx, 1, "O"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("GetPayment：%v", err)
	}
	if err := f.module.ReceivePaymentEvent(ctx, http.Header{}, nil); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("ReceivePaymentEvent：%v", err)
	}
	if f.module.ChargeConfigured() {
		t.Fatal("沒有 provider 卻說已設定")
	}
}

// schema 守住的不變量（3b、3c 的 use case 靠它們）：一筆付款最多一筆有效轉帳（失敗的不佔）、撤回累計不超過轉帳金額、退款的呼叫端冪等鍵唯一、
// 等待撤回與已呼叫退款之間的中間狀態要有撤回的外部 ID。
func TestTheSchemaGuardsTheTransferAndRefundInvariants(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-15", 11, 7, 100)
	p := f.create(t, 11, "O-15")
	exec := func(sql string, args ...any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
		return err
	}
	insertTransfer := func(status string) error {
		return exec(`INSERT INTO payment.transfers (id, payment_id, store_id, amount, destination_account_id, status, failure_reason, created_at, updated_at)
VALUES ($1, $2, 7, 100, 'acct_1', $3::text, CASE WHEN $3::text = 'FAILED' THEN 'x' END, now(), now())`, uuid.NewV7(), p.ID, status)
	}
	must(t, insertTransfer("FAILED"))
	must(t, insertTransfer("PENDING"))
	if err := insertTransfer("PENDING"); err == nil {
		t.Fatal("同一筆付款有兩筆有效轉帳")
	}
	if err := insertTransfer("FAILED"); err != nil {
		t.Fatalf("失敗的轉帳不該佔有效名額：%v", err)
	}
	if err := exec("UPDATE payment.transfers SET status = 'SUCCEEDED', external_transfer_id = 'tr_1', called_at = now(), reversed_amount = 101 WHERE status = 'PENDING'"); err == nil {
		t.Fatal("撤回累計超過轉帳金額")
	}
	must(t, exec("UPDATE payment.transfers SET status = 'SUCCEEDED', external_transfer_id = 'tr_1', called_at = now(), reversed_amount = 60 WHERE status = 'PENDING'"))
	insertRefund := func(key, status string, reversalAmount int64) error {
		return exec(`INSERT INTO payment.refunds (id, payment_id, request_key, amount, status, reversal_amount, created_at, updated_at)
VALUES ($1, $2, $3, 50, $4, $5, now(), now())`, uuid.NewV7(), p.ID, key, status, reversalAmount)
	}
	must(t, insertRefund("k1", "PENDING", 0))
	if err := insertRefund("k1", "PENDING", 0); err == nil {
		t.Fatal("同一把退款冪等鍵寫了兩列")
	}
	if err := insertRefund("k2", "REVERSED", 30); err == nil {
		t.Fatal("撤回完成卻沒有撤回的外部 ID")
	}
	if err := insertRefund("k3", "REVERSING", 0); err == nil {
		t.Fatal("撤回中卻沒有要撤的額度")
	}
	must(t, insertRefund("k4", "REVERSING", 30))
	if err := exec("UPDATE payment.payments SET status = 'SUCCEEDED' WHERE id = $1", p.ID); err == nil {
		t.Fatal("入帳的付款沒有 charge 與入帳時間")
	}
}

// 外部 ID 在 provider 已回傳的每個狀態都存得下來：退款 REFUNDING（provider 回了 ID 但還在處理）與建立之後才失敗的 FAILED、轉帳建立之後才失敗的 FAILED；
// 入帳（SUCCEEDED）一定要有 ID；還沒呼叫 provider 的狀態不能有 ID；有 ID 一定要有呼叫時刻。
// mutation：把 refunds_succeeded_check 改回「(status = 'SUCCEEDED') = (external_refund_id IS NOT NULL)」→ REFUNDING 帶 ID 的列寫不進去，失敗。
func TestTheSchemaStoresTheExternalIDOfEveryStateThatHasOne(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-F1", 11, 7, 100)
	p := f.create(t, 11, "O-F1")
	exec := func(sql string, args ...any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
		return err
	}
	ptr := func(s string) *string { return &s }
	insertRefund := func(key, status string, ext *string, called bool) error {
		return exec(`INSERT INTO payment.refunds (id, payment_id, request_key, amount, status, refund_called_at, external_refund_id, failure_reason, created_at, updated_at)
VALUES ($1, $2, $3, 50, $4::text, CASE WHEN $5::bool THEN now() END, $6::text, CASE WHEN $4::text = 'FAILED' THEN 'x' END, now(), now())`, uuid.NewV7(), p.ID, key, status, called, ext)
	}
	must(t, insertRefund("r1", "REFUNDING", ptr("re_1"), true))
	must(t, insertRefund("r2", "FAILED", ptr("re_2"), true))
	must(t, insertRefund("r3", "SUCCEEDED", ptr("re_3"), true))
	if err := insertRefund("r4", "SUCCEEDED", nil, true); err == nil {
		t.Fatal("退款入帳卻沒有外部 ID")
	}
	if err := insertRefund("r5", "PENDING", ptr("re_5"), true); err == nil {
		t.Fatal("還沒呼叫 provider 的退款有外部 ID")
	}
	if err := insertRefund("r6", "FAILED", ptr("re_6"), false); err == nil {
		t.Fatal("有外部 ID 卻沒有呼叫時刻")
	}
	insertTransfer := func(status string, ext *string) error {
		return exec(`INSERT INTO payment.transfers (id, payment_id, store_id, amount, destination_account_id, status, external_transfer_id, called_at, failure_reason, created_at, updated_at)
VALUES ($1, $2, 7, 100, 'acct_1', $3::text, $4::text, CASE WHEN $4::text IS NOT NULL OR $3::text = 'SUCCEEDED' THEN now() END, CASE WHEN $3::text = 'FAILED' THEN 'x' END, now(), now())`, uuid.NewV7(), p.ID, status, ext)
	}
	must(t, insertTransfer("FAILED", ptr("tr_f")))
	if err := insertTransfer("SUCCEEDED", nil); err == nil {
		t.Fatal("轉帳入帳卻沒有外部 ID")
	}
	if err := insertTransfer("PENDING", ptr("tr_p")); err == nil {
		t.Fatal("還沒呼叫 provider 的轉帳有外部 ID")
	}
	must(t, insertTransfer("SUCCEEDED", ptr("tr_s")))
}

// 已判斷過的快照一律標記事件已處理：入帳的列收到非入帳的快照（保留列、記警示）之後，涵蓋它的事件被標記，同步不再要求重跑一輪（job 不再每 2 秒 snooze）。
// mutation：sync.go 的 `if res.judged` 改回 `if res.written` → 事件永遠未處理、again 為 true，失敗。
func TestASnapshotThatKeepsTheSucceededRowStillMarksItsEventsProcessed(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-F3", 11, 7, 100)
	p := f.create(t, 11, "O-F3")
	ext := f.externalOf(t, "O-F3")
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.SetState(ext, PaymentStateAwaitingPayment))
	f.receive(t, "evt_keep", ext)
	if again := f.sync(t, p.ID); again {
		t.Fatal("已判斷過的快照之後仍要求再跑一輪（job 會一直 snooze）")
	}
	if !f.processed(t, "evt_keep") || f.statusOf(t, "O-F3") != PaymentSucceeded {
		t.Fatalf("事件已處理=%v，狀態=%s", f.processed(t, "evt_keep"), f.statusOf(t, "O-F3"))
	}
}

// 補做 job 排在請求內呼叫的上限之後：建立付款時排入的補做帶 ScheduledAt，晚於「現在加請求內的呼叫上限」；掃描排的（列已閒置）立刻執行。
// mutation：ensureRow 的 enqueueCreateAt 改回 enqueueCreate → ScheduledAt 是零值，失敗。
func TestTheCatchUpJobIsScheduledAfterTheInRequestCallTimeout(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-F4a", 11, 7, 100)
	f.fake.FailCreate(errors.New("逾時"))
	f.create(t, 11, "O-F4a")
	got := f.jobs.scheduledAt(kindChargeCreate)
	if len(got) != 1 || !got[0].After(f.now.Add(10*time.Second)) {
		t.Fatalf("補做 job 的 ScheduledAt=%v，現在=%v", got, f.now)
	}
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	f.now = f.now.Add(15 * time.Minute)
	if created, _, err := f.module.SweepPayments(context.Background()); err != nil || created != 1 {
		t.Fatalf("掃描：%d %v", created, err)
	}
	if got := f.jobs.scheduledAt(kindChargeCreate); len(got) != 1 || !got[0].IsZero() {
		t.Fatalf("掃描排的補做應立刻執行：%v", got)
	}
}

// 補做 job 先於請求寫入：請求結束時列已是 AWAITING_PAYMENT、請求自己的快照被較新的讀取擋下，回應仍要帶 client_secret（改走讀取路徑向 provider 取）。
// mutation：createInRequest 拿掉「列是 AWAITING_PAYMENT 卻沒有 secret 就 view」→ ClientSecret 是空的，失敗。
func TestACatchUpJobThatWritesFirstStillLetsTheRequestReturnTheSecret(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-F4b", 11, 7, 100)
	f.fake.OnCreate = func() {
		ids := f.paymentIDs(t, kindChargeCreate)
		if len(ids) != 1 {
			t.Errorf("補做 job=%v", ids)
			return
		}
		f.now = f.now.Add(time.Second)
		must(t, f.module.CreatePaymentIntent(ctx, ids[0], false))
	}
	p := f.create(t, 11, "O-F4b")
	if p.Status != PaymentAwaitingPayment || p.ClientSecret == "" || f.fake.Intents() != 1 {
		t.Fatalf("付款=%+v，PaymentIntent %d 個", p, f.fake.Intents())
	}
	f.noViolations(t)
}

// 已取消的付款、訂單仍可付：同一列換新冪等鍵、清掉舊的外部 ID、建出新的 PaymentIntent（仍是一張訂單一列、一次只有一個有效的 PaymentIntent）。
// mutation：ensureRow 的重設條件拿掉 `|| row.Status == string(model.PaymentCanceled)` → 取消的列沿用舊 PaymentIntent，失敗。
func TestACanceledPaymentOfAStillPayableOrderGetsANewIntentOnTheSameRow(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-F5", 11, 7, 100)
	p := f.create(t, 11, "O-F5")
	oldExt, oldKey := f.externalOf(t, "O-F5"), f.keyOf(t, "O-F5")
	must(t, f.fake.Settle(oldExt, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	if f.statusOf(t, "O-F5") != PaymentCanceled {
		t.Fatalf("狀態=%s", f.statusOf(t, "O-F5"))
	}
	again := f.create(t, 11, "O-F5")
	newExt := f.externalOf(t, "O-F5")
	if again.ID != p.ID || again.Status != PaymentAwaitingPayment || again.ClientSecret == "" || newExt == "" || newExt == oldExt || f.keyOf(t, "O-F5") == oldKey {
		t.Fatalf("重設=%+v，外部 ID %s → %s", again, oldExt, newExt)
	}
	if f.count(t, "SELECT count(*) FROM payment.payments") != 1 || f.fake.Intents() != 2 || f.count(t, "SELECT count(*) FROM payment.payments WHERE external_payment_id = $1", oldExt) != 0 {
		t.Fatalf("一張訂單一列：%d 列，PaymentIntent %d 個", f.count(t, "SELECT count(*) FROM payment.payments"), f.fake.Intents())
	}
	f.unpayable("O-F5", 11)
	must(t, f.fake.Settle(newExt, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	if _, err := f.module.CreatePayment(context.Background(), 11, "O-F5"); apperr.KindOf(err) != apperr.KindFailedPrecondition || f.statusOf(t, "O-F5") != PaymentCanceled {
		t.Fatalf("訂單不可付時不能重設：%v %s", err, f.statusOf(t, "O-F5"))
	}
}

// 判定 anomaly 之後標記事件已處理：已經告警、列不動；事件不再留在 inbox 讓掃描一直重排。
// mutation：Sync 拿掉 ErrAnomaly 分支裡的 markEventsProcessed → 事件仍未處理，失敗。
func TestAnAnomalyMarksItsEventsProcessedAndLeavesTheRowAlone(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-F7", 11, 7, 100)
	p := f.create(t, 11, "O-F7")
	ext := f.externalOf(t, "O-F7")
	f.receive(t, "evt_anom", ext)
	must(t, f.fake.SetAmount(ext, 5))
	if _, err := f.module.SyncPayment(context.Background(), p.ID); !errors.Is(err, ErrChargePermanent) {
		t.Fatalf("金額不符：%v", err)
	}
	if !f.processed(t, "evt_anom") || f.statusOf(t, "O-F7") != PaymentAwaitingPayment || !strings.Contains(f.logs.String(), "不一致") {
		t.Fatalf("事件已處理=%v 狀態=%s", f.processed(t, "evt_anom"), f.statusOf(t, "O-F7"))
	}
}

// 付款說明存在列上、送 provider，冪等重送時不變：訂單改名之後補做仍送建立當時的說明；沒有商品名稱時用訂單編號；超過上限截斷。
// mutation：create() 的 CreateInput.Description 改回 called.OrderNo → 送出的是訂單編號，失敗。
func TestThePaymentDescriptionIsStoredAndSentUnchangedOnReplay(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.orders.put(PaymentOrder{OrderNo: "O-F8", BuyerID: 11, StoreID: 7, Amount: 100, Payable: true, Description: "  藍色 T 恤  "})
	f.fake.FailCreate(errors.New("逾時"))
	p := f.create(t, 11, "O-F8")
	f.orders.put(PaymentOrder{OrderNo: "O-F8", BuyerID: 11, StoreID: 7, Amount: 100, Payable: true, Description: "改名了"})
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if got := f.fake.CreateDescriptions(); len(got) != 2 || got[0] != "藍色 T 恤" || got[1] != "藍色 T 恤" {
		t.Fatalf("送出的說明=%q", got)
	}
	f.orders.put(PaymentOrder{OrderNo: "O-F8b", BuyerID: 11, StoreID: 7, Amount: 100, Payable: true})
	f.create(t, 11, "O-F8b")
	f.orders.put(PaymentOrder{OrderNo: "O-F8c", BuyerID: 11, StoreID: 7, Amount: 100, Payable: true, Description: strings.Repeat("長", 300)})
	f.create(t, 11, "O-F8c")
	got := f.fake.CreateDescriptions()
	if len(got) != 4 || got[2] != "O-F8b" || len([]rune(got[3])) != 200 {
		t.Fatalf("送出的說明=%q", got)
	}
}
