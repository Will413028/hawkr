package seller

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

// 收款開通（Stripe 計畫；port 與快取在 internal/payment）。賣家的方法以店家為身分（呼叫方先從能力檢查取得店家）；管理員的方法由呼叫方
// （admin 的 Connect 方法）先確認是管理員。建立、重讀與開通 session 的外部呼叫都在 transaction 外。

// PaymentProvider 是收款開通的 port（Stripe adapter 與 PaymentFake 實作）。
type PaymentProvider = payment.Provider

// PaymentFake 是記憶體內的 provider，測試與本機用。
type PaymentFake = payment.Fake

// PaymentSnapshot 是外部帳戶某一刻的狀態。
type PaymentSnapshot = payment.Snapshot

// PaymentCapabilityState 是轉帳能力的中立狀態。
type PaymentCapabilityState = payment.CapabilityState

// 轉帳能力的狀態。
const (
	PaymentCapabilityActive      = payment.CapabilityActive
	PaymentCapabilityPending     = payment.CapabilityPending
	PaymentCapabilityRestricted  = payment.CapabilityRestricted
	PaymentCapabilityRejected    = payment.CapabilityRejected
	PaymentCapabilityUnsupported = payment.CapabilityUnsupported
	PaymentCapabilityUnknown     = payment.CapabilityUnknown
)

// PaymentResolution 是狀態細節要由誰行動才能解開。
type PaymentResolution = payment.Resolution

// 狀態細節的行動方。
const (
	PaymentResolutionSellerAction   = payment.ResolutionSellerAction
	PaymentResolutionPlatformAction = payment.ResolutionPlatformAction
	PaymentResolutionNone           = payment.ResolutionNone
	PaymentResolutionUnknown        = payment.ResolutionUnknown
)

// PaymentStatusDetail 是轉帳能力某個限制的行動方與 provider 的原因碼。
type PaymentStatusDetail = payment.StatusDetail

// PaymentCapabilities 是 provider 的能力描述（誰收件、用什麼介面）。
type PaymentCapabilities = payment.Capabilities

// PaymentEvent 是驗過簽章的 provider 事件。
type PaymentEvent = payment.Event

// PaymentEnsureInput 是建立外部帳戶的輸入。
type PaymentEnsureInput = payment.EnsureInput

// PaymentSessionInput 是開通 session 的輸入。
type PaymentSessionInput = payment.SessionInput

// PaymentAccountStatus 是收款帳戶的中立狀態。
type PaymentAccountStatus = model.PaymentAccountStatus

// RequirementCollection 是誰向賣家收身分與銀行資料。
type RequirementCollection = model.RequirementCollection

// OnboardingSurface 是開通用的介面。
type OnboardingSurface = model.OnboardingSurface

// PaymentAccount 是店家的收款帳戶。
type PaymentAccount = model.PaymentAccount

// OnboardingSession 是開通用的 session。
type OnboardingSession = model.OnboardingSession

// 收款帳戶的狀態。
const (
	PaymentNotCreated     = model.PaymentNotCreated
	PaymentUnderReview    = model.PaymentUnderReview
	PaymentActionRequired = model.PaymentActionRequired
	PaymentEnabled        = model.PaymentEnabled
	PaymentDisabled       = model.PaymentDisabled
)

// 收件方式與開通介面。
const (
	CollectionProvider = model.CollectionProvider
	CollectionPlatform = model.CollectionPlatform
	SurfaceHosted      = model.SurfaceHosted
	SurfaceEmbedded    = model.SurfaceEmbedded
)

// ErrPaymentPermanent 標示重試也不會成功的 provider 失敗。
var ErrPaymentPermanent = payment.ErrPermanent

// ErrPaymentInvalidEvent 標示事件的簽章或格式不合。
var ErrPaymentInvalidEvent = payment.ErrInvalidEvent

// ErrPaymentIgnoredEvent 標示驗過簽章但不處理的事件。
var ErrPaymentIgnoredEvent = payment.ErrIgnoredEvent

// PaymentFakeIgnoredEventHeader 讓 PaymentFake 把事件當成不處理的事件。
const PaymentFakeIgnoredEventHeader = payment.FakeIgnoredEventHeader

// PaymentFakeEventHeader 是 PaymentFake 的事件 ID header。
const PaymentFakeEventHeader = payment.FakeEventHeader

// NewStripePayment 建立 Stripe 的收款 provider（Accounts v2）；collection 是 CollectionProvider（託管頁面）或 CollectionPlatform（內嵌元件）。
func NewStripePayment(client *stripe.Client, collection RequirementCollection) (PaymentProvider, error) {
	p, err := payment.NewStripe(client, collection)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewPaymentFake 建立 PaymentFake。
func NewPaymentFake() *PaymentFake { return payment.NewFake() }

// PaymentAccountStatusValues 回傳全部收款帳戶狀態。
func PaymentAccountStatusValues() []PaymentAccountStatus { return model.PaymentAccountStatusValues() }

// PaymentStatusOf 由快照推導收款帳戶的狀態與要存的原因碼。
func PaymentStatusOf(log *slog.Logger, s PaymentSnapshot) (PaymentAccountStatus, string) {
	return payment.StatusOf(log, s)
}

// PaymentAccountOf 讀店家的收款帳戶；店家還沒有回 NotFound。
func (m *Module) PaymentAccountOf(ctx context.Context, storeID int64) (PaymentAccount, error) {
	return m.payment.Account(ctx, storeID)
}

// CreatePaymentOnboardingSession 為店家的收款帳戶產生開通 session（託管連結或內嵌元件）。
func (m *Module) CreatePaymentOnboardingSession(ctx context.Context, storeID int64, returnURL, refreshURL string) (OnboardingSession, error) {
	return m.payment.CreateOnboardingSession(ctx, storeID, returnURL, refreshURL)
}

// RefreshPaymentAccount 管理員的「重新整理狀態」：排入同步 job。
func (m *Module) RefreshPaymentAccount(ctx context.Context, storeID int64) error {
	return m.payment.Refresh(ctx, storeID)
}

// RetryPaymentAccountCreate 管理員的「重試建立」：只有建立失敗的帳戶可以重試。
func (m *Module) RetryPaymentAccountCreate(ctx context.Context, storeID int64) (PaymentAccount, error) {
	return m.payment.RetryCreate(ctx, storeID)
}

// ReceivePaymentEvent 接收 provider 的 webhook：驗簽、先寫 inbox 再排同步 job。
func (m *Module) ReceivePaymentEvent(ctx context.Context, header http.Header, body []byte) error {
	return m.payment.ReceiveEvent(ctx, header, body)
}

// PaymentConfigured 回傳是否設定了收款開通的 provider。
func (m *Module) PaymentConfigured() bool { return m.payment.Configured() }

// EnsurePaymentAccount 是建立 job（seller.payment_ensure）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時失敗改記為建立失敗。
func (m *Module) EnsurePaymentAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.payment.Ensure(ctx, storeID, finalAttempt)
}

// SyncPaymentAccount 是同步 job（seller.payment_sync）的內容，給 worker 與測試呼叫；again 為 true 表示讀取期間又收到事件。
func (m *Module) SyncPaymentAccount(ctx context.Context, storeID int64) (again bool, err error) {
	return m.payment.Sync(ctx, storeID)
}

// SweepPaymentAccounts 是定期掃描（seller.payment_sweep）的內容，給 worker 與測試呼叫；回傳排入同步與建立的帳戶數。
func (m *Module) SweepPaymentAccounts(ctx context.Context) (synced, ensured int, err error) {
	return m.payment.Sweep(ctx)
}

// PaymentCapabilities 回傳目前 provider 的能力描述（誰收件、用什麼介面）；沒有設定收款開通時 ok 為 false。
func (m *Module) PaymentCapabilities() (caps PaymentCapabilities, ok bool) {
	return m.payment.Capabilities()
}
