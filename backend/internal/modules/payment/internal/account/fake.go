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
	resolveErrs  []error
	resolves     []string
	fetchErrs    []error
	closeErrs    []error
	closed       []string
	ensures      int
	ensureKeys   []string
	ensureEmails []string
	fetches      int
	sessions     int
	violations   []string
}

// NewFake 建立 Fake：預設是 provider 收件、託管介面。
func NewFake() *Fake {
	return &Fake{
		caps:     Capabilities{RequirementCollection: model.CollectionProvider, OnboardingSurface: model.SurfaceHosted},
		byKey:    map[string]string{},
		accounts: map[string]Snapshot{},
	}
}

// WithCapabilities 換掉能力描述（測試平台收件與內嵌介面）。
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

// FailEnsure 讓接下來的建立依序回這些錯誤（排完就成功）；FailFetch 同理。
func (f *Fake) FailEnsure(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureErrs = append(f.ensureErrs, errs...)
}

// FailEnsureAfterCreate 讓接下來的建立依序在帳戶已建立之後回這些錯誤（回應遺失；排完就成功）。
func (f *Fake) FailEnsureAfterCreate(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lostErrs = append(f.lostErrs, errs...)
}

// FailResolve 讓接下來的取得依序回這些錯誤（排完就成功）。
func (f *Fake) FailResolve(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolveErrs = append(f.resolveErrs, errs...)
}

// Resolves 回傳每次 ResolveAccount 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) Resolves() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.resolves...)
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

// Calls 回傳建立、重讀、開通 session 的呼叫次數。
func (f *Fake) Calls() (ensures, fetches, sessions int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensures, f.fetches, f.sessions
}

// EnsureEmails 回傳每次 EnsureAccount 收到的聯絡信箱（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) EnsureEmails() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ensureEmails...)
}

// EnsureKeys 回傳每次 EnsureAccount 收到的冪等鍵（依呼叫順序，失敗的呼叫也算）。
func (f *Fake) EnsureKeys() []string {
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

// Accounts 回傳目前建立過的外部帳戶數。
func (f *Fake) Accounts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byKey)
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

// accountFor 要持有 f.mu：回傳冪等鍵對應的帳戶，沒有就建立。
func (f *Fake) accountFor(key string) string {
	if id, ok := f.byKey[key]; ok {
		return id
	}
	f.next++
	id := fmt.Sprintf("fake_acct_%d", f.next)
	f.byKey[key] = id
	f.accounts[id] = Snapshot{
		Transfers:    CapabilityRestricted,
		Details:      []StatusDetail{{Resolution: ResolutionSellerAction, Reason: "routine_onboarding"}},
		Requirements: json.RawMessage(`{}`),
	}
	return id
}

// EnsureAccount 實作 Provider：一步完成，同一個冪等鍵回同一個帳戶（信箱不同也一樣）。
func (f *Fake) EnsureAccount(ctx context.Context, in EnsureInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "EnsureAccount")
	f.ensures++
	f.ensureKeys = append(f.ensureKeys, in.IdempotencyKey)
	f.ensureEmails = append(f.ensureEmails, in.ContactEmail)
	if err := pop(&f.ensureErrs); err != nil {
		return "", err
	}
	id := f.accountFor(in.IdempotencyKey)
	if err := pop(&f.lostErrs); err != nil {
		return "", err
	}
	return id, nil
}

// ResolveAccount 實作 Provider：同一個冪等鍵回同一個帳戶，沒有就建立。
func (f *Fake) ResolveAccount(ctx context.Context, in ResolveInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "ResolveAccount")
	f.resolves = append(f.resolves, in.IdempotencyKey)
	if err := pop(&f.resolveErrs); err != nil {
		return "", err
	}
	return f.accountFor(in.IdempotencyKey), nil
}

// CreateOnboardingSession 實作 Provider。
func (f *Fake) CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "CreateOnboardingSession")
	f.sessions++
	if in.ExternalAccountID == "" {
		return model.OnboardingSession{}, ErrPermanent
	}
	if f.caps.OnboardingSurface == model.SurfaceEmbedded {
		return model.OnboardingSession{Surface: model.SurfaceEmbedded, ClientSecret: "fake_secret_" + in.ExternalAccountID}, nil
	}
	return model.OnboardingSession{Surface: model.SurfaceHosted, URL: "https://onboarding.example.invalid/" + in.ExternalAccountID}, nil
}

// FetchAccount 實作 Provider。
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
