package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
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

	mu         sync.Mutex
	caps       Capabilities
	next       int
	byKey      map[string]string
	accounts   map[string]Snapshot
	ensureErrs []error
	fetchErrs  []error
	ensures    int
	ensureKeys []string
	fetches    int
	sessions   int
	violations []string
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

// FailFetch 讓接下來的重讀依序回這些錯誤（排完就成功）。
func (f *Fake) FailFetch(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchErrs = append(f.fetchErrs, errs...)
}

// Calls 回傳建立、重讀、開通 session 的呼叫次數。
func (f *Fake) Calls() (ensures, fetches, sessions int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensures, f.fetches, f.sessions
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

// EnsureAccount 實作 Provider：同一個冪等鍵回同一個帳戶。
func (f *Fake) EnsureAccount(ctx context.Context, in EnsureInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.check(ctx, "EnsureAccount")
	f.ensures++
	f.ensureKeys = append(f.ensureKeys, in.IdempotencyKey)
	if err := pop(&f.ensureErrs); err != nil {
		return "", err
	}
	if id, ok := f.byKey[in.IdempotencyKey]; ok {
		return id, nil
	}
	f.next++
	id := fmt.Sprintf("fake_acct_%d", f.next)
	f.byKey[in.IdempotencyKey] = id
	f.accounts[id] = Snapshot{
		Transfers:    CapabilityRestricted,
		Details:      []StatusDetail{{Resolution: ResolutionSellerAction, Reason: "routine_onboarding"}},
		Requirements: json.RawMessage(`{}`),
	}
	return id, nil
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
