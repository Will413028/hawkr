package charge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// FakeName 是 Fake 的 provider 代碼：與收款帳戶的 Fake（"fake"）不同，兩個 provider 在 inbox 各自登記。
const FakeName = "fake_charge"

// FakeEventHeader 是 Fake 的事件載體：header 帶事件 ID，body 是 PaymentIntent ID；body 可以寫成 "<ID>|<狀態>"，後面是事件「宣稱」的狀態（沒有簽章，也不被信任：
// 處理者一律重讀，測試用它造出與 provider 實際狀態不符的事件）。
const FakeEventHeader = "X-Fake-Charge-Event-Id"

// FakeIgnoredEventHeader 讓 Fake 的 ParseEvent 把事件當成不處理的事件（回 ErrIgnoredEvent）。
const FakeIgnoredEventHeader = "X-Fake-Charge-Event-Ignored"

type fakeIntent struct {
	snap    Snapshot
	orderNo string
	storeID int64
	group   string
}

type fakeRefund struct {
	in   RefundInput
	snap RefundSnapshot
}

type fakeTransfer struct {
	in   TransferInput
	snap TransferSnapshot
}

// Fake 是記憶體內的 provider：測試與本機用。每個呼叫都檢查不在 DB transaction 內、有 deadline（規則 5），違規記在 Violations。
// 同一把冪等鍵重送回同一個物件（內容不同是永久失敗，與 Stripe 的 idempotency_error 一致）。
//
// 建立呼叫的四種結果都能驅動：成功（不注入）、終局拒絕（FailX(ErrPermanent)）、可重試拒絕（FailX(ErrRetryableRefusal)，沒有建立物件）、不明
// （FailX(一般錯誤) 是呼叫沒有到達 provider，FailXAfterCreate(一般錯誤) 是物件已建立、回應遺失）。查回（Find*）依嘗試 ID（建立時的冪等鍵）比對：
// 物件存在就是「找到」，不存在是「查無」，FailLookup 注入查回本身的失敗。
type Fake struct {
	// OnCreate 在 CreatePayment 進行中（建立之前）執行一次，測試用來製造交錯。
	OnCreate func()

	mu           sync.Mutex
	next         int
	intents      map[string]*fakeIntent
	byKey        map[string]string
	createErrs   []error
	lostErrs     []error
	getErrs      []error
	cancelErrs   []error
	creates      int
	createKeys   []string
	createDescs  []string
	gets         int
	cancels      int
	refunds      map[string]*fakeRefund
	refundByKey  map[string]string
	refundErrs   []error
	refundLost   []error
	refundSeen   []string
	refundCalls  int
	refundNext   RefundState
	transfers    map[string]*fakeTransfer
	transferKeys map[string]string
	transferErrs []error
	reversals    map[string]ReversalSnapshot
	reversalKeys map[string]string
	reversalOf   map[string]string
	reversalErrs []error
	// transferLost 與 reversalLost 是「已建立、回應遺失」的錯誤佇列；transferCalls 與 reversalCalls 計呼叫次數（失敗的也算）。
	transferSeen  []string
	reversalSeen  []string
	transferLost  []error
	reversalLost  []error
	transferCalls int
	reversalCalls int
	lookupErrs    []error
	lookups       int
	violations    []string
}

// NewFake 建立 Fake。
func NewFake() *Fake {
	return &Fake{
		intents: map[string]*fakeIntent{}, byKey: map[string]string{}, refunds: map[string]*fakeRefund{}, refundByKey: map[string]string{}, refundNext: RefundSucceeded,
		transfers: map[string]*fakeTransfer{}, transferKeys: map[string]string{}, reversals: map[string]ReversalSnapshot{}, reversalKeys: map[string]string{}, reversalOf: map[string]string{},
	}
}

func push(dst *[]error, errs []error) { *dst = append(*dst, errs...) }

// FailCreate 讓接下來的 CreatePayment 依序在建立之前回這些錯誤（排完就成功）。
func (f *Fake) FailCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.createErrs, errs)
}

// FailCreateAfterCreate 讓接下來的 CreatePayment 依序在 PaymentIntent 已建立之後回這些錯誤（回應遺失，呼叫方沒有拿到 ID；排完就成功）。
func (f *Fake) FailCreateAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.lostErrs, errs)
}

// FailGet 讓接下來的 GetPayment 依序回這些錯誤（排完就成功）。
func (f *Fake) FailGet(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.getErrs, errs)
}

// FailCancel 讓接下來的 CancelPayment 依序回這些錯誤（排完就成功）。
func (f *Fake) FailCancel(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.cancelErrs, errs)
}

// FailRefund、FailTransfer 與 FailReversal 讓接下來對應的建立依序回這些錯誤（排完就成功）。
func (f *Fake) FailRefund(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.refundErrs, errs)
}

// FailRefundAfterCreate 讓接下來的 CreateRefund 依序在退款已建立之後回這些錯誤（回應遺失，呼叫方沒有拿到 ID；排完就成功），用來造「呼叫過、結果不明」。
func (f *Fake) FailRefundAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.refundLost, errs)
}

// NextRefundState 設定之後建立的退款的初始狀態（預設 RefundSucceeded；RefundPending 用來測 provider 還在處理）。
func (f *Fake) NextRefundState(state RefundState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refundNext = state
}

// SetRefundState 讓退款變成這個狀態（含不認得的）；failureReason 只在 RefundFailed 有意義。
func (f *Fake) SetRefundState(externalID string, state RefundState, failureReason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.refunds[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有退款 %s", externalID)
	}
	r.snap.State, r.snap.FailureReason = state, failureReason
	return nil
}

// SetRefundAmount 改 provider 那邊退款的金額（測試查回或建立的結果與列對不起來）。
func (f *Fake) SetRefundAmount(externalID string, amount int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.refunds[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有退款 %s", externalID)
	}
	r.snap.Amount = amount
	return nil
}

// RefundCalls 回傳 CreateRefund 的呼叫次數（失敗的呼叫也算）。
func (f *Fake) RefundCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refundCalls
}

// RefundKeys 回傳每次 CreateRefund 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) RefundKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.refundSeen...)
}

// RefundIDs 回傳 provider 那邊已建立的退款的外部 ID（依 ID 排序）。
func (f *Fake) RefundIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sortedKeys(f.refunds)
}

// Refunds 回傳 provider 那邊已建立的退款數（同一把鍵只算一筆）。
func (f *Fake) Refunds() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.refunds)
}

// FailTransfer 見 FailRefund。
func (f *Fake) FailTransfer(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.transferErrs, errs)
}

// FailReversal 見 FailRefund。
func (f *Fake) FailReversal(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.reversalErrs, errs)
}

// FailTransferAfterCreate 讓接下來的 CreateTransfer 依序在轉帳已建立之後回這些錯誤（回應遺失，呼叫方沒有拿到 ID；排完就成功），用來造「呼叫過、結果不明」。
func (f *Fake) FailTransferAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.transferLost, errs)
}

// FailReversalAfterCreate 見 FailTransferAfterCreate，對象是 ReverseTransfer。
func (f *Fake) FailReversalAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.reversalLost, errs)
}

// FailLookup 讓接下來的 Find*（不分種類）依序回這些錯誤（排完就正常查回）。
func (f *Fake) FailLookup(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	push(&f.lookupErrs, errs)
}

// LookupCalls 回傳 Find* 的呼叫次數（失敗的也算）。
func (f *Fake) LookupCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lookups
}

// SetTransferAmount 改 provider 那邊轉帳的金額（測試查回或建立的結果與列對不起來）。
func (f *Fake) SetTransferAmount(externalID string, amount int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tr, ok := f.transfers[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有轉帳 %s", externalID)
	}
	tr.snap.Amount = amount
	return nil
}

// SetReversalAmount 改 provider 那邊撤回的金額（測試查回或建立的結果與列對不起來）。
func (f *Fake) SetReversalAmount(externalID string, amount int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reversals[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有撤回 %s", externalID)
	}
	r.Amount = amount
	f.reversals[externalID] = r
	return nil
}

// TransferCalls 回傳 CreateTransfer 的呼叫次數（失敗的呼叫也算）。
func (f *Fake) TransferCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.transferCalls
}

// ReversalCalls 回傳 ReverseTransfer 的呼叫次數（失敗的呼叫也算）。
func (f *Fake) ReversalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reversalCalls
}

// TransferKeys 回傳每次 CreateTransfer 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) TransferKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.transferSeen...)
}

// ReversalKeys 回傳每次 ReverseTransfer 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) ReversalKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reversalSeen...)
}

// TransferIDs 回傳 provider 那邊已建立的轉帳的外部 ID（依 ID 排序）。
func (f *Fake) TransferIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sortedKeys(f.transfers)
}

// ReversalIDs 回傳 provider 那邊已建立的撤回的外部 ID（依 ID 排序）。
func (f *Fake) ReversalIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sortedKeys(f.reversals)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Transfers 回傳 provider 那邊已建立的轉帳數（同一把鍵只算一筆）。
func (f *Fake) Transfers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.transfers)
}

// Settle 讓 PaymentIntent 變成這個狀態（模擬買家付款、取消或需要驗證）；入帳時帶上 charge 與入帳時間。
func (f *Fake) Settle(externalID string, state State, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	in, ok := f.intents[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有 PaymentIntent %s", externalID)
	}
	in.snap.State = state
	if state == StateSucceeded {
		in.snap.ChargeID = "fake_ch_" + strings.TrimPrefix(externalID, "fake_pi_")
		in.snap.PaidAt = at
	}
	return nil
}

// FailAttempt 讓 PaymentIntent 回到等待付款並帶上最近一次付款嘗試的失敗原因碼。
func (f *Fake) FailAttempt(externalID, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	in, ok := f.intents[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有 PaymentIntent %s", externalID)
	}
	in.snap.State, in.snap.LastError = StateAwaitingPayment, code
	return nil
}

// SetAmount 改 provider 那邊的金額（測試快照與列對不起來）。
func (f *Fake) SetAmount(externalID string, amount int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	in, ok := f.intents[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有 PaymentIntent %s", externalID)
	}
	in.snap.Amount = amount
	return nil
}

// SetState 讓 PaymentIntent 回傳任意狀態值（含不認得的），測試 adapter 對應之外的路徑。
func (f *Fake) SetState(externalID string, state State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	in, ok := f.intents[externalID]
	if !ok {
		return fmt.Errorf("fake：沒有 PaymentIntent %s", externalID)
	}
	in.snap.State = state
	return nil
}

// Intents 回傳目前建立過的 PaymentIntent 數。
func (f *Fake) Intents() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.intents)
}

// IntentFor 回傳這張訂單（transfer_group）的 PaymentIntent ID 們（依建立順序不保證，測試只用在單一時）。
func (f *Fake) IntentFor(orderNo string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id, in := range f.intents {
		if in.group == orderNo {
			out = append(out, id)
		}
	}
	return out
}

// Calls 回傳建立、讀取、取消的呼叫次數（失敗的呼叫也算）。
func (f *Fake) Calls() (creates, gets, cancels int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.creates, f.gets, f.cancels
}

// CreateKeys 回傳每次 CreatePayment 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) CreateKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.createKeys...)
}

// CreateDescriptions 回傳每次 CreatePayment 收到的付款說明（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) CreateDescriptions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.createDescs...)
}

// Violations 回傳違反呼叫規則的紀錄（在 transaction 內、沒有 deadline）。
func (f *Fake) Violations() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.violations...)
}

// check 要持有 f.mu。
func (f *Fake) check(ctx context.Context, op string) {
	if _, ok := db.TxFrom(ctx); ok {
		f.violations = append(f.violations, op+" 在 DB transaction 內被呼叫")
	}
	if _, ok := ctx.Deadline(); !ok {
		f.violations = append(f.violations, op+" 沒有 deadline")
	}
}

func pop(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	return err
}

// Name 實作 Provider。
func (f *Fake) Name() string { return FakeName }

// CreatePayment 實作 Provider：同一個冪等鍵回同一個 PaymentIntent；同一把鍵送了不同的金額或訂單是永久失敗。
func (f *Fake) CreatePayment(ctx context.Context, in CreateInput) (Snapshot, error) {
	f.mu.Lock()
	hook := f.OnCreate
	f.OnCreate = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CreatePayment")
	f.creates++
	f.createKeys = append(f.createKeys, in.IdempotencyKey)
	f.createDescs = append(f.createDescs, in.Description)
	if err := pop(&f.createErrs); err != nil {
		return Snapshot{}, err
	}
	if in.Amount <= 0 || in.TransferGroup == "" || in.IdempotencyKey == "" || in.Description == "" {
		return Snapshot{}, fmt.Errorf("%w：金額、說明、transfer_group 與冪等鍵必填", ErrPermanent)
	}
	if id, ok := f.byKey[in.IdempotencyKey]; ok {
		cur := f.intents[id]
		if cur.snap.Amount != in.Amount || cur.group != in.TransferGroup {
			return Snapshot{}, fmt.Errorf("%w：冪等鍵對應的請求內容不同（金流商回 idempotency_error）", ErrPermanent)
		}
		return cur.snap, pop(&f.lostErrs)
	}
	f.next++
	id := fmt.Sprintf("fake_pi_%d", f.next)
	f.byKey[in.IdempotencyKey] = id
	f.intents[id] = &fakeIntent{
		snap:    Snapshot{ExternalID: id, State: StateAwaitingPayment, Amount: in.Amount, ClientSecret: id + "_secret_" + uuid.NewV4().String()},
		orderNo: in.OrderNo, storeID: in.StoreID, group: in.TransferGroup,
	}
	if err := pop(&f.lostErrs); err != nil {
		return Snapshot{}, err
	}
	return f.intents[id].snap, nil
}

// lookup 計次並回注入的查回失敗；要持有 f.mu。
func (f *Fake) lookup(ctx context.Context, op string) error {
	f.check(ctx, op)
	f.lookups++
	return pop(&f.lookupErrs)
}

// FindPayment 實作 Provider：依建立時的冪等鍵（嘗試 ID）查回。
func (f *Fake) FindPayment(ctx context.Context, attemptID string) (Snapshot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.lookup(ctx, "FindPayment"); err != nil {
		return Snapshot{}, false, err
	}
	id, ok := f.byKey[attemptID]
	if !ok {
		return Snapshot{}, false, nil
	}
	return f.intents[id].snap, true, nil
}

// FindRefund 實作 Provider：依建立時的冪等鍵（嘗試 ID）查回。
func (f *Fake) FindRefund(ctx context.Context, externalPaymentID, attemptID string) (RefundSnapshot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.lookup(ctx, "FindRefund"); err != nil {
		return RefundSnapshot{}, false, err
	}
	id, ok := f.refundByKey[attemptID]
	if !ok || f.refunds[id].in.ExternalPaymentID != externalPaymentID {
		return RefundSnapshot{}, false, nil
	}
	return f.refunds[id].snap, true, nil
}

// FindTransfer 實作 Provider：依 transfer_group 與建立時的冪等鍵（嘗試 ID）查回。
func (f *Fake) FindTransfer(ctx context.Context, transferGroup, attemptID string) (TransferSnapshot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.lookup(ctx, "FindTransfer"); err != nil {
		return TransferSnapshot{}, false, err
	}
	id, ok := f.transferKeys[attemptID]
	if !ok || f.transfers[id].in.TransferGroup != transferGroup {
		return TransferSnapshot{}, false, nil
	}
	return f.transfers[id].snap, true, nil
}

// FindReversal 實作 Provider：依轉帳 ID 與建立時的冪等鍵（嘗試 ID）查回。
func (f *Fake) FindReversal(ctx context.Context, externalTransferID, attemptID string) (ReversalSnapshot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.lookup(ctx, "FindReversal"); err != nil {
		return ReversalSnapshot{}, false, err
	}
	id, ok := f.reversalKeys[attemptID]
	if !ok || f.reversalOf[id] != externalTransferID {
		return ReversalSnapshot{}, false, nil
	}
	return f.reversals[id], true, nil
}

// GetPayment 實作 Provider：不存在是永久失敗。
func (f *Fake) GetPayment(ctx context.Context, externalID string) (Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "GetPayment")
	f.gets++
	if err := pop(&f.getErrs); err != nil {
		return Snapshot{}, err
	}
	in, ok := f.intents[externalID]
	if !ok {
		return Snapshot{}, fmt.Errorf("%w：沒有 PaymentIntent %s", ErrPermanent, externalID)
	}
	return in.snap, nil
}

// CancelPayment 實作 Provider：冪等；已入帳的回入帳的快照。
func (f *Fake) CancelPayment(ctx context.Context, externalID string) (Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CancelPayment")
	f.cancels++
	if err := pop(&f.cancelErrs); err != nil {
		return Snapshot{}, err
	}
	in, ok := f.intents[externalID]
	if !ok {
		return Snapshot{}, fmt.Errorf("%w：沒有 PaymentIntent %s", ErrPermanent, externalID)
	}
	if in.snap.State != StateSucceeded {
		in.snap.State = StateCanceled
	}
	return in.snap, nil
}

// CreateRefund 實作 Provider：同一個冪等鍵回同一筆退款；付款未入帳、金額不是正數或累計（非失敗的退款加總）超過付款金額是永久失敗；
// FailRefund 注入可重試拒絕與一般錯誤，FailRefundAfterCreate 注入「已建立、回應遺失」。
func (f *Fake) CreateRefund(ctx context.Context, in RefundInput) (RefundSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CreateRefund")
	f.refundCalls++
	f.refundSeen = append(f.refundSeen, in.IdempotencyKey)
	if err := pop(&f.refundErrs); err != nil {
		return RefundSnapshot{}, err
	}
	if id, ok := f.refundByKey[in.IdempotencyKey]; ok {
		return f.refunds[id].snap, pop(&f.refundLost)
	}
	pi, ok := f.intents[in.ExternalPaymentID]
	if !ok || pi.snap.State != StateSucceeded || in.Amount <= 0 {
		return RefundSnapshot{}, fmt.Errorf("%w：不能退款 %s", ErrPermanent, in.ExternalPaymentID)
	}
	var taken int64
	for _, r := range f.refunds {
		if r.in.ExternalPaymentID == in.ExternalPaymentID && r.snap.State != RefundFailed {
			taken += r.snap.Amount
		}
	}
	if taken+in.Amount > pi.snap.Amount {
		return RefundSnapshot{}, fmt.Errorf("%w：退款累計超過付款金額 %s", ErrPermanent, in.ExternalPaymentID)
	}
	f.next++
	id := fmt.Sprintf("fake_re_%d", f.next)
	snap := RefundSnapshot{ExternalID: id, State: f.refundNext, Amount: in.Amount}
	f.refunds[id], f.refundByKey[in.IdempotencyKey] = &fakeRefund{in: in, snap: snap}, id
	if err := pop(&f.refundLost); err != nil {
		return RefundSnapshot{}, err
	}
	return snap, nil
}

// GetRefund 實作 Provider。
func (f *Fake) GetRefund(ctx context.Context, externalID string) (RefundSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "GetRefund")
	r, ok := f.refunds[externalID]
	if !ok {
		return RefundSnapshot{}, fmt.Errorf("%w：沒有退款 %s", ErrPermanent, externalID)
	}
	return r.snap, nil
}

// CreateTransfer 實作 Provider：同一個冪等鍵回同一筆轉帳；轉帳金額超過付款是永久失敗；FailTransfer 可以注入「平台餘額不足」這類可重試拒絕（ErrRetryableRefusal）。
func (f *Fake) CreateTransfer(ctx context.Context, in TransferInput) (TransferSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CreateTransfer")
	f.transferCalls++
	f.transferSeen = append(f.transferSeen, in.IdempotencyKey)
	if err := pop(&f.transferErrs); err != nil {
		return TransferSnapshot{}, err
	}
	if id, ok := f.transferKeys[in.IdempotencyKey]; ok {
		return f.transfers[id].snap, pop(&f.transferLost)
	}
	if in.Amount <= 0 || in.SourceChargeID == "" || in.DestinationAccountID == "" {
		return TransferSnapshot{}, fmt.Errorf("%w：轉帳的金額、來源與去向必填", ErrPermanent)
	}
	f.next++
	id := fmt.Sprintf("fake_tr_%d", f.next)
	snap := TransferSnapshot{ExternalID: id, Amount: in.Amount}
	f.transfers[id], f.transferKeys[in.IdempotencyKey] = &fakeTransfer{in: in, snap: snap}, id
	if err := pop(&f.transferLost); err != nil {
		return TransferSnapshot{}, err
	}
	return snap, nil
}

// ReverseTransfer 實作 Provider：同一個冪等鍵回同一次撤回；撤回超過轉帳的剩餘額度是永久失敗（Stripe 回 400）；賣家餘額不足不是錯誤。
func (f *Fake) ReverseTransfer(ctx context.Context, in ReversalInput) (ReversalSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "ReverseTransfer")
	f.reversalCalls++
	f.reversalSeen = append(f.reversalSeen, in.IdempotencyKey)
	if err := pop(&f.reversalErrs); err != nil {
		return ReversalSnapshot{}, err
	}
	if id, ok := f.reversalKeys[in.IdempotencyKey]; ok {
		return f.reversals[id], pop(&f.reversalLost)
	}
	tr, ok := f.transfers[in.ExternalTransferID]
	if !ok || in.Amount <= 0 || in.Amount > tr.snap.Amount-tr.snap.ReversedAmount {
		return ReversalSnapshot{}, fmt.Errorf("%w：不能撤回轉帳 %s", ErrPermanent, in.ExternalTransferID)
	}
	f.next++
	id := fmt.Sprintf("fake_trr_%d", f.next)
	tr.snap.ReversedAmount += in.Amount
	snap := ReversalSnapshot{ExternalID: id, Amount: in.Amount}
	f.reversals[id], f.reversalKeys[in.IdempotencyKey], f.reversalOf[id] = snap, id, in.ExternalTransferID
	if err := pop(&f.reversalLost); err != nil {
		return ReversalSnapshot{}, err
	}
	return snap, nil
}

// ParseEvent 實作 Provider：沒有簽章，header 帶事件 ID，body 是 PaymentIntent ID 或退款 ID（fake_re_ 開頭是退款事件；可帶 "|宣稱的狀態"，被忽略）。
func (f *Fake) ParseEvent(header http.Header, body []byte) (Event, error) {
	if header.Get(FakeIgnoredEventHeader) != "" {
		return Event{}, ErrIgnoredEvent
	}
	object, _, _ := strings.Cut(strings.TrimSpace(string(body)), "|")
	id := strings.TrimSpace(header.Get(FakeEventHeader))
	if id == "" || object == "" {
		return Event{}, errors.Join(ErrInvalidEvent, errors.New("缺少事件 ID 或 PaymentIntent"))
	}
	if strings.HasPrefix(object, "fake_re_") {
		return Event{ID: id, Kind: EventRefund, ObjectID: object}, nil
	}
	return Event{ID: id, Kind: EventPayment, ObjectID: object}, nil
}
