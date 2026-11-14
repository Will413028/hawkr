package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// FakeName 是 Fake 的 provider 代碼。
const FakeName = "fake"

// FakeEventHeader 是 Fake 的事件載體：header 帶事件 ID，body 是外部帳戶 ID（沒有簽章）。
const FakeEventHeader = "X-Fake-Event-Id"

// FakeIgnoredEventHeader 讓 Fake 的 ParseEvent 把事件當成不處理的事件（回 ErrIgnoredEvent）。
const FakeIgnoredEventHeader = "X-Fake-Event-Ignored"

// Fake 是記憶體內的 provider：測試與本機用。每個呼叫都檢查不在 DB transaction 內、有 deadline（規則 5），違規記在 Violations。
// AutoActive 為真時，每個外部帳戶（含不認得的 ID，例如種子資料的帳戶）都回轉帳能力已啟用的快照，本機不需要真的 provider 就能走完開通。
type Fake struct {
	AutoActive bool
	// OnFetch 在 FetchAccount 進行中（讀取之前）執行一次，測試用來製造「讀取期間又收到事件」的交錯。
	OnFetch func()

	mu           sync.Mutex
	caps         Capabilities
	next         int
	byKey        map[string]string
	accounts     map[string]Snapshot
	ensureErrs   []error
	lostErrs     []error
	configErrs   []error
	resolveErrs  []error
	resolves     []int64
	byStore      map[int64][]string
	fetchErrs    []error
	closeErrs    []error
	closed       []string
	closedIDs    map[string]bool
	ensures      int
	ensureKeys   []string
	configures   int
	ensureEmails []string
	fetches      int
	sessions     int
	violations   []string
}

// NewFake 建立 Fake：預設是 provider 收件；只支援託管連結（不支援任何內嵌執行環境）。
func NewFake() *Fake {
	return &Fake{
		caps:      Capabilities{RequirementCollection: model.CollectionProvider},
		byKey:     map[string]string{},
		byStore:   map[int64][]string{},
		accounts:  map[string]Snapshot{},
		closedIDs: map[string]bool{},
	}
}

// WithCapabilities 換掉能力描述（測試新列寫入的收件方式）。
func (f *Fake) WithCapabilities(c Capabilities) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.caps = c
	return f
}

// Set 設定外部帳戶的快照。
func (f *Fake) Set(externalID string, s Snapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accounts[externalID] = s
}

// FailAcquire 讓接下來的「取得 ID」（建立的第一步）依序回這些錯誤（排完就成功）；FailFetch 同理。
func (f *Fake) FailAcquire(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureErrs = append(f.ensureErrs, errs...)
}

// FailAcquireAfterCreate 讓接下來的「取得 ID」依序在帳戶已建立之後回這些錯誤（回應遺失，列上沒有拿到 ID；排完就成功）。
func (f *Fake) FailAcquireAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lostErrs = append(f.lostErrs, errs...)
}

// FailConfigure 讓接下來的「完成設定」（建立的第二步）依序回這些錯誤（排完就成功）；此時帳戶與它的 ID 已經存在。
func (f *Fake) FailConfigure(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configErrs = append(f.configErrs, errs...)
}

// ExpireKeys 模擬冪等鍵過期（金流商只保存 30 天）：之後同一把鍵取得 ID 會建出新的帳戶；已建立的帳戶不受影響。
func (f *Fake) ExpireKeys() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byKey = map[string]string{}
}

// FailResolve 讓接下來的「列出對帳」依序回這些錯誤（排完就成功）。
func (f *Fake) FailResolve(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolveErrs = append(f.resolveErrs, errs...)
}

// Resolves 回傳每次 ResolveAccount 收到的店家 ID（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) Resolves() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.resolves...)
}

// FailFetch 讓接下來的重讀依序回這些錯誤（排完就成功）。
func (f *Fake) FailFetch(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchErrs = append(f.fetchErrs, errs...)
}

// FailClose 讓接下來的關閉依序回這些錯誤（排完就成功）。
func (f *Fake) FailClose(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeErrs = append(f.closeErrs, errs...)
}

// Closes 回傳每次 CloseAccount 收到的外部帳戶 ID（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) Closes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.closed...)
}

// Calls 回傳取得 ID（建立的第一步）、重讀、開通 session 的呼叫次數。
func (f *Fake) Calls() (ensures, fetches, sessions int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensures, f.fetches, f.sessions
}

// ConfigureEmails 回傳每次 ConfigureAccount 收到的聯絡信箱（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) ConfigureEmails() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ensureEmails...)
}

// AcquireKeys 回傳每次 AcquireAccount 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) AcquireKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ensureKeys...)
}

// Violations 回傳違反呼叫規則的紀錄（在 transaction 內、沒有 deadline）。
func (f *Fake) Violations() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.violations...)
}

// Accounts 回傳目前建立過的外部帳戶數（含已關閉的）。
func (f *Fake) Accounts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, ids := range f.byStore {
		n += len(ids)
	}
	return n
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

// Capabilities 實作 Provider。
func (f *Fake) Capabilities() Capabilities {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.caps
}

// accountFor 要持有 f.mu：回傳冪等鍵對應的帳戶，沒有就為 storeID 建立（帳戶記著它的 store_id，列出對帳用）。
func (f *Fake) accountFor(key string, storeID int64) string {
	if id, ok := f.byKey[key]; ok {
		return id
	}
	f.next++
	id := fmt.Sprintf("fake_acct_%d", f.next)
	f.byKey[key] = id
	f.byStore[storeID] = append(f.byStore[storeID], id)
	f.accounts[id] = Snapshot{
		Transfers:    CapabilityRestricted,
		Details:      []StatusDetail{{Resolution: ResolutionSellerAction, Reason: "routine_onboarding"}},
		Requirements: json.RawMessage(`{}`),
	}
	return id
}

// AcquireAccount 實作 Provider：同一個冪等鍵回同一個帳戶；帳戶已關閉之後重放同一把鍵是永久失敗（measured：409 idempotency_error）。
func (f *Fake) AcquireAccount(ctx context.Context, in AcquireInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "AcquireAccount")
	f.ensures++
	f.ensureKeys = append(f.ensureKeys, in.IdempotencyKey)
	if err := pop(&f.ensureErrs); err != nil {
		return "", err
	}
	if id, ok := f.byKey[in.IdempotencyKey]; ok && f.closedIDs[id] {
		return "", fmt.Errorf("%w：冪等鍵對應的帳戶已關閉（金流商回 409 idempotency_error）", ErrPermanent)
	}
	id := f.accountFor(in.IdempotencyKey, in.StoreID)
	if err := pop(&f.lostErrs); err != nil {
		return "", err
	}
	return id, nil
}

// ConfigureAccount 實作 Provider：可重複送出；帳戶不存在是永久失敗。
func (f *Fake) ConfigureAccount(ctx context.Context, in ConfigureInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "ConfigureAccount")
	f.configures++
	f.ensureEmails = append(f.ensureEmails, in.ContactEmail)
	if err := pop(&f.configErrs); err != nil {
		return err
	}
	if _, ok := f.accounts[in.ExternalID]; !ok && !f.AutoActive {
		return fmt.Errorf("%w：沒有外部帳戶 %s", ErrPermanent, in.ExternalID)
	}
	return nil
}

// ResolveAccount 實作 Provider：只找不建。列出沒有關閉的帳戶，回傳 store_id 相符的全部（新的在前）。
func (f *Fake) ResolveAccount(ctx context.Context, in ResolveInput) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "ResolveAccount")
	f.resolves = append(f.resolves, in.StoreID)
	if err := pop(&f.resolveErrs); err != nil {
		return nil, err
	}
	ids := f.byStore[in.StoreID]
	var open []string
	for i := len(ids) - 1; i >= 0; i-- {
		if !f.closedIDs[ids[i]] {
			open = append(open, ids[i])
		}
	}
	return open, nil
}

// CreateOnboardingSession 實作 Provider。
func (f *Fake) CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CreateOnboardingSession")
	f.sessions++
	if in.ExternalAccountID == "" || in.Kind != SessionHosted {
		return nil, ErrPermanent
	}
	return model.HostedSession{URL: "https://onboarding.example.invalid/" + in.ExternalAccountID}, nil
}

// FetchAccount 實作 Provider；已關閉（CloseAccount）的帳戶回 Closed 的快照，與 Stripe 讀到已關閉帳戶一致。
func (f *Fake) FetchAccount(ctx context.Context, externalID string) (Snapshot, error) {
	f.mu.Lock()
	hook := f.OnFetch
	f.OnFetch = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "FetchAccount")
	f.fetches++
	if err := pop(&f.fetchErrs); err != nil {
		return Snapshot{}, err
	}
	if f.closedIDs[externalID] {
		return Snapshot{Closed: true, Requirements: json.RawMessage(`{}`)}, nil
	}
	if f.AutoActive {
		return Snapshot{Transfers: CapabilityActive, PayoutsActive: true, Requirements: json.RawMessage(`{}`)}, nil
	}
	s, ok := f.accounts[externalID]
	if !ok {
		return Snapshot{}, fmt.Errorf("%w：沒有外部帳戶 %s", ErrPermanent, externalID)
	}
	return s, nil
}

// CloseAccount 實作 Provider：冪等，同一個帳戶關第二次也成功；不認得的帳戶是永久失敗。AutoActive 時不認得的 ID（例如種子資料的帳戶）也視為存在。
func (f *Fake) CloseAccount(ctx context.Context, externalID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CloseAccount")
	f.closed = append(f.closed, externalID)
	if err := pop(&f.closeErrs); err != nil {
		return err
	}
	if _, ok := f.accounts[externalID]; !ok && !f.AutoActive {
		return fmt.Errorf("%w：沒有外部帳戶 %s", ErrPermanent, externalID)
	}
	f.closedIDs[externalID] = true
	return nil
}

// ParseEvent 實作 Provider：沒有簽章，header 帶事件 ID，body 是外部帳戶 ID。
func (f *Fake) ParseEvent(header http.Header, body []byte) (Event, error) {
	if header.Get(FakeIgnoredEventHeader) != "" {
		return Event{}, ErrIgnoredEvent
	}
	id, account := strings.TrimSpace(header.Get(FakeEventHeader)), strings.TrimSpace(string(body))
	if id == "" || account == "" {
		return Event{}, errors.Join(ErrInvalidEvent, errors.New("缺少事件 ID 或外部帳戶"))
	}
	return Event{ID: id, ExternalAccountID: account}, nil
}
