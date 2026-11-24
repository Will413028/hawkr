// Package payment 是 payment 模組的公開 API：現階段只擁有收款帳戶（收款開通 ADR D28）。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md；行為規格在 backend/docs/modules/payment.md。
package payment

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/account"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/charge"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Owners 是擁有者（店家）是否營運中的查詢，由組裝點以 seller 的公開 API 實作：payment 是葉節點，不讀上層模組。
type Owners = account.Owners

// Deps 是模組需要的外部依賴；clock 由組裝點注入。Provider 為 nil 表示沒有設定收款開通（核准不建立帳戶，開通相關的呼叫回 FailedPrecondition）。
// Inbox 是 platform 的 webhook inbox：設定了 provider 時 New 把收款事件的處理登記上去。Inbox 與 Owners 只在設定了 provider 時必填。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Inbox  *inbox.Inbox
	// Audit 是 platform 的後台動作紀錄（D62）：管理員的寫入在領域變更的同一個 transaction 內寫一筆；組裝點一律傳入，沒有傳入時後台寫入回錯誤。
	Audit    *adminaudit.Log
	Provider Provider
	Owners   Owners
	Now      func() time.Time
	Logger   *slog.Logger
	// Charge 是付款的 provider（Stripe adapter 與 ChargeFake）；nil 表示沒有設定付款，付款的呼叫回 FailedPrecondition。設定了它時 Inbox 與 PaymentOrders 必填，
	// 它的名稱不得與收款帳戶的 Provider 相同（inbox 以 provider 名稱登記，同名會 panic）。
	Charge ChargeProvider
	// PaymentOrders 是訂單一方的 port（付款建立時讀應付資訊），由組裝點注入（過渡期接 Java order，之後換 ordering 的 in-process 呼叫）。
	PaymentOrders PaymentOrders
}

// Module 是 payment 模組。
type Module struct {
	accounts *account.Service
	charges  *charge.Service
}

// New 建立模組。
func New(d Deps) *Module {
	m := &Module{accounts: account.New(account.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Inbox: d.Inbox, Audit: d.Audit, Provider: d.Provider, Owners: d.Owners, Now: d.Now, Logger: d.Logger,
	})}
	m.charges = charge.New(charge.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Inbox: d.Inbox, Provider: d.Charge, Orders: d.PaymentOrders, Audit: d.Audit, Now: d.Now, Logger: d.Logger,
	})
	return m
}

// Provider 是收款開通的 port（Stripe adapter 與 Fake 實作）。
type Provider = account.Provider

// Fake 是記憶體內的 provider，測試與本機用。
type Fake = account.Fake

// Snapshot 是外部帳戶某一刻的狀態。
type Snapshot = account.Snapshot

// ReasonClosed 是外部帳戶已在 provider 關閉時，帳戶列的原因碼（狀態 DISABLED，終局，不再輪詢）。
const ReasonClosed = account.ReasonClosed

// CapabilityState 是轉帳能力的中立狀態。
type CapabilityState = account.CapabilityState

// 轉帳能力的狀態。
const (
	CapabilityActive      = account.CapabilityActive
	CapabilityPending     = account.CapabilityPending
	CapabilityRestricted  = account.CapabilityRestricted
	CapabilityRejected    = account.CapabilityRejected
	CapabilityUnsupported = account.CapabilityUnsupported
	CapabilityUnknown     = account.CapabilityUnknown
)

// Resolution 是狀態細節要由誰行動才能解開。
type Resolution = account.Resolution

// 狀態細節的行動方。
const (
	ResolutionSellerAction   = account.ResolutionSellerAction
	ResolutionPlatformAction = account.ResolutionPlatformAction
	ResolutionNone           = account.ResolutionNone
	ResolutionUnknown        = account.ResolutionUnknown
)

// StatusDetail 是轉帳能力某個限制的行動方與 provider 的原因碼。
type StatusDetail = account.StatusDetail

// Capabilities 是 provider 的能力描述（新建的帳戶列用哪種收件方式）。
type Capabilities = account.Capabilities

// Event 是驗過簽章的 provider 事件。
type Event = account.Event

// AcquireInput 是建立的第一步（取得外部帳戶 ID）的輸入。
type AcquireInput = account.AcquireInput

// ConfigureInput 是建立的第二步（完成設定）的輸入。
type ConfigureInput = account.ConfigureInput

// SessionInput 是開通 session 的輸入。
type SessionInput = account.SessionInput

// AccountStatus 是收款帳戶的中立狀態。
type AccountStatus = model.PaymentAccountStatus

// RequirementCollection 是誰向賣家收身分與銀行資料。
type RequirementCollection = model.RequirementCollection

// EmbeddedRuntime 是呼叫端能承載的內嵌開通執行環境。
type EmbeddedRuntime = model.EmbeddedRuntime

// Account 是擁有者的收款帳戶。
type Account = model.PaymentAccount

// OnboardingSession 是開通用的 session：HostedSession 或 StripeEmbeddedSession 其中之一（封閉的 interface）。
type OnboardingSession = model.OnboardingSession

// HostedSession 是金流商託管的開通連結。
type HostedSession = model.HostedSession

// StripeEmbeddedSession 是 Stripe 內嵌開通元件的初始化資料。
type StripeEmbeddedSession = model.StripeEmbeddedSession

// 收款帳戶的狀態。
const (
	StatusNotCreated     = model.PaymentNotCreated
	StatusUnderReview    = model.PaymentUnderReview
	StatusActionRequired = model.PaymentActionRequired
	StatusEnabled        = model.PaymentEnabled
	StatusDisabled       = model.PaymentDisabled
)

// 收件方式與內嵌開通執行環境。
const (
	CollectionProvider     = model.CollectionProvider
	CollectionPlatform     = model.CollectionPlatform
	RuntimeStripeConnectJS = model.RuntimeStripeConnectJS
)

// ErrPermanent 標示重試也不會成功的 provider 失敗。
var ErrPermanent = account.ErrPermanent

// ErrInvalidEvent 標示事件的簽章或格式不合。
var ErrInvalidEvent = account.ErrInvalidEvent

// ErrIgnoredEvent 標示驗過簽章但不處理的事件。
var ErrIgnoredEvent = account.ErrIgnoredEvent

// FakeIgnoredEventHeader 讓 Fake 把事件當成不處理的事件。
const FakeIgnoredEventHeader = account.FakeIgnoredEventHeader

// FakeEventHeader 是 Fake 的事件 ID header。
const FakeEventHeader = account.FakeEventHeader

// FakeName 是 Fake 的 provider 代碼。
const FakeName = account.FakeName

// EventRetention 是收款事件原始 payload 在 inbox 的保存期。
const EventRetention = account.EventRetention

// IdempotencyRetention 是金流商保存冪等鍵結果的時間（30 天）：任何重送都在它之內（UnresolvedDeadline 小於它），超過的鍵從不重送。
const IdempotencyRetention = account.IdempotencyRetention

// UnresolvedDeadline 是「呼叫過、沒有外部 ID」的列最久等多久（7 天，D63）：期限內以同一把鍵重送取回 ID，超過進終局（建立失敗或清理終局）。
const UnresolvedDeadline = account.UnresolvedDeadline

// PurgeDeadline 是列被標成等待清理後，仍關不掉或問不到金流商的最長時間（7 天，D60），超過進終局。
const PurgeDeadline = account.PurgeDeadline

// NewStripe 建立 Stripe 的收款 provider（Accounts v2）；collection 是新帳戶列的收件方式（CollectionProvider 或 CollectionPlatform）；publishableKey 必填（既有的平台收件帳戶仍會開內嵌 session）。
func NewStripe(client *stripe.Client, collection RequirementCollection, publishableKey string) (Provider, error) {
	p, err := account.NewStripe(client, collection, publishableKey)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewFake 建立 Fake。
func NewFake() *Fake { return account.NewFake() }

// AccountStatusValues 回傳全部收款帳戶狀態。
func AccountStatusValues() []AccountStatus { return model.PaymentAccountStatusValues() }

// EmbeddedRuntimeValues 回傳全部內嵌開通執行環境。
func EmbeddedRuntimeValues() []EmbeddedRuntime { return model.EmbeddedRuntimeValues() }

// RequirementCollectionValues 回傳全部收件方式。
func RequirementCollectionValues() []RequirementCollection {
	return model.RequirementCollectionValues()
}

// Configured 回傳是否設定了收款開通的 provider。
func (m *Module) Configured() bool { return m.accounts.Configured() }

// RequestAccount 在擁有者核准的 transaction 內（ctx）寫「建立中」的收款帳戶列並排入建立 job；已有列時不改它（關店再開沿用）。聯絡信箱不在這裡傳也不存，
// 建立 job 在呼叫金流商的當下經 Owners 讀（D50）。沒有設定 provider 時什麼都不做。
func (m *Module) RequestAccount(ctx context.Context, storeID int64, now time.Time) error {
	return m.accounts.RequestAccount(ctx, storeID, now)
}

// Account 讀擁有者的收款帳戶；還沒有回 NotFound。呼叫方先確認呼叫者是擁有者或管理員。
func (m *Module) Account(ctx context.Context, storeID int64) (Account, error) {
	return m.accounts.Account(ctx, storeID)
}

// CreateOnboardingSession 為擁有者的收款帳戶產生開通 session（託管連結或內嵌元件）；外部呼叫在 transaction 外。呼叫方先確認呼叫者是擁有者。
// runtimes 是呼叫端宣告能承載的內嵌執行環境（空表示不能內嵌）：帳戶列是 Stripe 的平台收件而且清單含 RuntimeStripeConnectJS 才回內嵌，其餘一律回託管連結。
func (m *Module) CreateOnboardingSession(ctx context.Context, storeID int64, returnURL, refreshURL string, runtimes []EmbeddedRuntime) (OnboardingSession, error) {
	return m.accounts.CreateOnboardingSession(ctx, storeID, returnURL, refreshURL, runtimes)
}

// RefreshAccount 管理員的「重新整理狀態」：與賣家的 SyncAccountUnlessFresh 同一條同步重讀（同一個名額與間隔，D33），回傳重讀後的帳戶；
// 沒有設定 provider、還沒有外部帳戶或帳戶在等待關閉回 FailedPrecondition，沒有帳戶列回 NotFound。呼叫方先確認呼叫者是管理員，且不能在 transaction 內呼叫。
func (m *Module) RefreshAccount(ctx context.Context, storeID int64) (Account, error) {
	return m.accounts.Refresh(ctx, storeID)
}

// AdminActor 是後台動作的操作者與動作（platform adminaudit，D62）：handler 以管理員 ID 與 RPC 的 procedure 名稱組成。
type AdminActor = adminaudit.Actor

// RetryAccountCreate 管理員的「重試建立」：只有建立失敗的帳戶可以重試；建立結果不明的帳戶在冪等鍵保存期（IdempotencyRetention）內以同一把鍵重送、不需要確認；超過保存期要 acknowledgeNoProviderAccount 才換新鍵（D63）。
// 呼叫方的 ctx 不得已在 transaction 內；本方法在自己的 transaction 內寫領域變更與一筆後台紀錄。
func (m *Module) RetryAccountCreate(ctx context.Context, actor AdminActor, storeID int64, acknowledgeNoProviderAccount bool) (Account, error) {
	return m.accounts.RetryCreate(ctx, actor, storeID, acknowledgeNoProviderAccount)
}

// RetryAccountPurge 管理員的「重試關閉」：只有清理進了終局的帳戶可以重試（清除終局、期限從現在重算並重新排入清理 job，同一個 transaction 寫後台紀錄）。
func (m *Module) RetryAccountPurge(ctx context.Context, actor AdminActor, storeID int64) (Account, error) {
	return m.accounts.RetryPurge(ctx, actor, storeID)
}

// DeleteAccountMapping 管理員的「已在金流商處理，刪除對應」：只有清理進了終局的帳戶可以做；同一個 transaction 寫一筆後台紀錄並刪列，不呼叫金流商。
func (m *Module) DeleteAccountMapping(ctx context.Context, actor AdminActor, storeID int64) error {
	return m.accounts.DeleteMapping(ctx, actor, storeID)
}

// ReceiveEvent 接收 provider 的 webhook：驗簽，同一個 transaction 寫進 platform inbox 並排入派送 job。
func (m *Module) ReceiveEvent(ctx context.Context, header http.Header, body []byte) error {
	return m.accounts.ReceiveEvent(ctx, header, body)
}

// ForgetOwners 在擁有者個資清理的 transaction 內（ctx）為每個擁有者排入「關閉外部帳戶後刪除收款帳戶列」的 job。
func (m *Module) ForgetOwners(ctx context.Context, storeIDs []int64) error {
	return m.accounts.ForgetOwners(ctx, storeIDs)
}

// EnsureAccount 是建立 job（payment.account_ensure）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時失敗改記為建立失敗。
func (m *Module) EnsureAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.accounts.Ensure(ctx, storeID, finalAttempt)
}

// SyncAccount 重讀外部帳戶並寫入快取：同步 job（payment.account_sync）的內容，也給營運與測試呼叫；可以與賣家觸發的同步並行，
// 較早開始的讀取不覆蓋較晚開始的（D32）。again 為 true 表示讀取期間又收到事件（job 會稍後再跑一輪；其他呼叫方可以忽略，快取最遲在 job 之後更新）。
// 外部呼叫在 transaction 外，呼叫方不能在 transaction 內呼叫。
func (m *Module) SyncAccount(ctx context.Context, storeID int64) (again bool, err error) {
	return m.accounts.Sync(ctx, storeID)
}

// SyncAccountUnlessFresh 是賣家觸發的同步重讀（seller 的 RefreshMyPaymentAccount；管理員的 RefreshAccount 也走它）：間隔（10 秒）內已有讀取（含失敗的與並行的，以條件式 UPDATE 搶名額）就不向金流商讀、依列上的時間回成功或 ResourceExhausted（帶剩餘秒數，D33）；
// 搶到才同步重讀並寫入快取，讀完仍有未處理事件時排同步 job。沒有設定 provider 或帳戶列的 provider 不符回 FailedPrecondition；重讀失敗回 Unavailable（暫時性）或 FailedPrecondition（金流商沒有這個帳戶）。呼叫方先確認呼叫者是擁有者，且不能在 transaction 內呼叫。
func (m *Module) SyncAccountUnlessFresh(ctx context.Context, storeID int64) error {
	return m.accounts.SyncUnlessFresh(ctx, storeID)
}

// SweepAccounts 是定期掃描（payment.account_sweep）的內容，給 worker 與測試呼叫；回傳排入同步與建立的帳戶數。
func (m *Module) SweepAccounts(ctx context.Context) (synced, ensured int, err error) {
	return m.accounts.Sweep(ctx)
}

// ResolveUnresolvedAccount 是了結 job（payment.account_resolve）的內容，給 worker 與測試呼叫（D63）；finalAttempt 為 true 時暫時性失敗只記警示 log，留給下一次掃描。
func (m *Module) ResolveUnresolvedAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.accounts.Resolve(ctx, storeID, finalAttempt)
}

// ReplayUnresolvedAccount 是管理員重試結果不明的建立失敗列時排入的 job（payment.account_resolve，Replay 為真）的內容，給 worker 與測試呼叫（D63）。
func (m *Module) ReplayUnresolvedAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.accounts.Replay(ctx, storeID, finalAttempt)
}

// PurgeAccount 是「關閉後刪除」job（payment.account_purge）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時關閉失敗只記警示 log，本地列（已標成等待清理）留著給週期掃描。
func (m *Module) PurgeAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.accounts.Purge(ctx, storeID, finalAttempt)
}

// ScanPayoutDeadlines 是每小時掃描（payment.deadline_scan）的內容，給 worker 與測試呼叫：對期限落在未來 7 天內的帳戶各發布一次 PayoutDeadlineApproaching；回傳發布數。
func (m *Module) ScanPayoutDeadlines(ctx context.Context) (published int, err error) {
	return m.accounts.ScanDeadlines(ctx)
}

// AddWorkers 登記 River worker。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.accounts.AddWorkers(workers)
	m.charges.AddWorkers(workers)
}

// PeriodicJobs 回傳週期 job。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	return append(m.accounts.PeriodicJobs(), m.charges.PeriodicJobs()...)
}

// ChargeProvider 是付款的 port（PaymentIntent、退款、轉帳與撤回、付款事件；Fake 給測試與本機）。
type ChargeProvider = charge.Provider

// ChargeFake 是記憶體內的付款 provider，測試與本機用。
type ChargeFake = charge.Fake

// NewChargeFake 建立 ChargeFake。
func NewChargeFake() *ChargeFake { return charge.NewFake() }

// ChargeFakeName 是 ChargeFake 的 provider 代碼。
const ChargeFakeName = charge.FakeName

// ChargeFakeEventHeader 是 ChargeFake 的事件 ID header（body 是 PaymentIntent ID，可帶 "|宣稱的狀態"，宣稱不被信任）。
const ChargeFakeEventHeader = charge.FakeEventHeader

// ChargeFakeIgnoredEventHeader 讓 ChargeFake 把事件當成不處理的事件。
const ChargeFakeIgnoredEventHeader = charge.FakeIgnoredEventHeader

// PaymentOrders 是訂單一方的 port（建立付款時讀歸屬、可付與金額）。
type PaymentOrders = charge.Orders

// PaymentOrder 是建立付款當下訂單一方回的事實。
type PaymentOrder = charge.Order

// PaymentSnapshot 是 PaymentIntent 某一刻的中立狀態。
type PaymentSnapshot = charge.Snapshot

// PaymentState 是 provider 回報的 PaymentIntent 的中立狀態。
type PaymentState = charge.State

// PaymentIntent 的中立狀態。
const (
	PaymentStateAwaitingPayment = charge.StateAwaitingPayment
	PaymentStateProcessing      = charge.StateProcessing
	PaymentStateSucceeded       = charge.StateSucceeded
	PaymentStateCanceled        = charge.StateCanceled
	PaymentStateUnknown         = charge.StateUnknown
)

// Payment 是買家對一張訂單的付款。
type Payment = model.Payment

// PaymentStatus 是付款的狀態。
type PaymentStatus = model.PaymentStatus

// 付款的狀態。
const (
	PaymentCreating        = model.PaymentCreating
	PaymentAwaitingPayment = model.PaymentAwaitingPayment
	PaymentProcessing      = model.PaymentProcessing
	PaymentSucceeded       = model.PaymentSucceeded
	PaymentCanceled        = model.PaymentCanceled
	PaymentCreateFailed    = model.PaymentCreateFailed
	PaymentAnomaly         = model.PaymentAnomaly
)

// ChargeTransferInput 與 ChargeTransferSnapshot 是建立轉帳的 port 型別（測試包裝 provider 用）。
type (
	ChargeTransferInput    = charge.TransferInput
	ChargeTransferSnapshot = charge.TransferSnapshot
)

// ChargeReversalInput 與 ChargeReversalSnapshot 是撤回轉帳的 port 型別（測試包裝 provider 用）。
type (
	ChargeReversalInput    = charge.ReversalInput
	ChargeReversalSnapshot = charge.ReversalSnapshot
)

// PaymentTransfer 是付款轉給賣家的轉帳。
type PaymentTransfer = model.Transfer

// PaymentTransferStatus 是轉帳的狀態。
type PaymentTransferStatus = model.TransferStatus

// 轉帳的狀態。
const (
	TransferPending   = model.TransferPending
	TransferSucceeded = model.TransferSucceeded
	TransferFailed    = model.TransferFailed
	TransferAnomaly   = model.TransferAnomaly
)

// ErrChargeRetryableRefusal 標示付款 provider 明確拒絕、沒有執行，之後重試可能成功（呼叫方換新冪等鍵）。
var ErrChargeRetryableRefusal = charge.ErrRetryableRefusal

// AnomalyKind 是進入 ANOMALY 的物件種類，AnomalyOutcome 是人工結案的結果。
type (
	AnomalyKind    = model.AnomalyKind
	AnomalyOutcome = model.AnomalyOutcome
)

// 進入 ANOMALY 的物件種類與人工結案的結果。
const (
	AnomalyPayment   = model.AnomalyPayment
	AnomalyTransfer  = model.AnomalyTransfer
	AnomalyRefund    = model.AnomalyRefund
	AnomalyReversal  = model.AnomalyReversal
	AnomalyTopup     = model.AnomalyTopup
	AnomalySucceeded = model.AnomalySucceeded
	AnomalyFailed    = model.AnomalyFailed
)

// ResolveAnomaly 是後台對 ANOMALY 的付款、轉帳、退款、撤回（AnomalyReversal）或補差額轉帳（AnomalyTopup）的人工結案；撤回的 id 是撤回列 ID，補差額轉帳的 id 是退款 ID：核對 provider 之後結案成成功（evidence 是 provider 物件 ID，必填）或失敗，
// 領域變更與一筆 adminaudit 在同一個 transaction。語意與鎖順序見 charge.Service.ResolveAnomaly。
func (m *Module) ResolveAnomaly(ctx context.Context, actor AdminActor, kind AnomalyKind, id uuid.UUID, outcome AnomalyOutcome, evidence string) error {
	return m.charges.ResolveAnomaly(ctx, actor, kind, id, outcome, evidence)
}

// ErrChargePermanent 標示付款 provider 重試也不會成功的失敗。
var ErrChargePermanent = charge.ErrPermanent

// ErrChargeInvalidEvent 標示付款事件的簽章或格式不合。
var ErrChargeInvalidEvent = charge.ErrInvalidEvent

// ErrChargeIgnoredEvent 標示驗過簽章但不處理的付款事件。
var ErrChargeIgnoredEvent = charge.ErrIgnoredEvent

// PaymentResendWindow 是搬錢呼叫結果不明時以同一把冪等鍵重送的上限（23 小時，小於冪等鍵保存期），之後只查回。
const PaymentResendWindow = charge.ResendWindow

// PaymentNotFoundAfter 是從第一次呼叫起，查回「查無」才算確定不存在的時間（24 小時），之後才可以換新鍵重建。
const PaymentNotFoundAfter = charge.NotFoundAfter

// PaymentLookupFailureLimit 是連續查回失敗幾次之後進 ANOMALY。
const PaymentLookupFailureLimit = charge.LookupFailureLimit

// PaymentIdempotencyRetention 是本模組依賴的付款冪等鍵保存期（24 小時）；超過的鍵從不重送。
const PaymentIdempotencyRetention = charge.IdempotencyRetention

// ChargeConfigured 回傳是否設定了付款 provider。
func (m *Module) ChargeConfigured() bool { return m.charges.Configured() }

// CreatePayment 建立買家對一張訂單的付款（計畫 D7 修訂、D8）：transaction 內寫付款列與補做 job，提交之後在請求內、transaction 外帶冪等鍵建立 PaymentIntent 並回 client_secret；
// provider 失敗或逾時回狀態 CREATING 的付款（pending），補做 job 以同一把鍵接手，呼叫端用 GetPayment 重讀。同一張訂單只有一個 PaymentIntent。呼叫方先確認呼叫者是買家，且不能在 transaction 內呼叫。
func (m *Module) CreatePayment(ctx context.Context, buyerID int64, orderNo string) (Payment, error) {
	return m.charges.CreatePayment(ctx, buyerID, orderNo)
}

// GetPayment 讀買家自己的付款，AWAITING_PAYMENT 時向 provider 讀 client_secret 一併回傳（不存）；不是呼叫者的付款回 NotFound。呼叫方不能在 transaction 內呼叫。
func (m *Module) GetPayment(ctx context.Context, buyerID int64, orderNo string) (Payment, error) {
	return m.charges.GetPayment(ctx, buyerID, orderNo)
}

// ReceivePaymentEvent 接收付款 provider 的 webhook（獨立的路由與簽章密鑰）：驗簽，同一個 transaction 寫進 platform inbox 並排入派送 job。
func (m *Module) ReceivePaymentEvent(ctx context.Context, header http.Header, body []byte) error {
	return m.charges.ReceiveEvent(ctx, header, body)
}

// CreatePaymentIntent 是補做 job（payment.charge_create）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時暫時性失敗只記警示 log，留給週期掃描。
func (m *Module) CreatePaymentIntent(ctx context.Context, paymentID uuid.UUID, finalAttempt bool) error {
	return m.charges.Create(ctx, paymentID, finalAttempt)
}

// SyncPayment 重讀 PaymentIntent 並寫入狀態：同步 job（payment.charge_sync）的內容，給 worker 與測試呼叫；again 為 true 表示讀取期間又收到事件。外部呼叫在 transaction 外。
func (m *Module) SyncPayment(ctx context.Context, paymentID uuid.UUID) (again bool, err error) {
	return m.charges.Sync(ctx, paymentID)
}

// CancelPaymentAttempt 是取消舊 PaymentIntent 的 job（payment.charge_cancel）的內容，給 worker 與測試呼叫：只取消已被取代、還沒結案的嘗試。
func (m *Module) CancelPaymentAttempt(ctx context.Context, attemptID uuid.UUID) error {
	return m.charges.CancelAttempt(ctx, attemptID)
}

// ExcessPaid 是一次重複入帳：同一張訂單已經以另一個 PaymentIntent 入帳，這個舊 PaymentIntent 也扣到了款（不自動退款）。
type ExcessPaid = model.ExcessPaid

// ExcessPaidPayments 列出重複入帳（後台查詢）；afterID 是上一頁最後一筆的 AttemptID，第一頁傳零值。
func (m *Module) ExcessPaidPayments(ctx context.Context, afterID uuid.UUID, limit int32) ([]ExcessPaid, error) {
	return m.charges.ExcessPaid(ctx, afterID, limit)
}

// SweepPayments 是付款的定期掃描（payment.charge_sweep）的內容，給 worker 與測試呼叫；回傳排入補做與同步的付款數。
func (m *Module) SweepPayments(ctx context.Context) (created, synced int, err error) {
	return m.charges.Sweep(ctx)
}

// RequestPaymentTransfer 是訂單完成後把這筆付款轉給賣家的入口（D6，訂單一方的 GrantPayment 呼叫）：一個 transaction 內鎖付款列、確認已入帳，
// 寫 PENDING 轉帳（金額是付款扣掉非失敗退款，賣家收款帳戶必須已開通）並排入轉帳 job；provider 在 transaction 外由 job 呼叫。
// 已有有效轉帳時回傳它（冪等）；付款不存在回 NotFound，付款未入帳、帳戶未開通或沒有可轉金額回 FailedPrecondition。呼叫方不能在 transaction 內呼叫。
func (m *Module) RequestPaymentTransfer(ctx context.Context, orderNo string) (PaymentTransfer, error) {
	return m.charges.RequestTransfer(ctx, orderNo)
}

// CreatePaymentTransfer 是轉帳 job（payment.transfer_create）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時暫時性失敗只記警示 log，留給週期掃描。
func (m *Module) CreatePaymentTransfer(ctx context.Context, transferID uuid.UUID, finalAttempt bool) error {
	return m.charges.Transfer(ctx, transferID, finalAttempt)
}

// ReversePaymentTransfer 是撤回轉帳 job（payment.transfer_reversal）的內容，給 worker 與測試呼叫：逐列收斂退款進行中的撤回列，
// 每一列成功後同一個 transaction 累加目標（原轉帳或補差額轉帳）的累計撤回額並把該列推進到 REVERSED、重算退款的彙總。
func (m *Module) ReversePaymentTransfer(ctx context.Context, refundID uuid.UUID, finalAttempt bool) error {
	return m.charges.Reverse(ctx, refundID, finalAttempt)
}

// TopUpPaymentTransfer 是補差額轉帳 job（payment.transfer_topup）的內容，給 worker 與測試呼叫：補差額狀態是 SENDING 的退款列（已失敗、金額已算進轉帳）的退款額，
// 以新的冪等鍵轉給賣家，成功後進 DONE。
func (m *Module) TopUpPaymentTransfer(ctx context.Context, refundID uuid.UUID, finalAttempt bool) error {
	return m.charges.TopUp(ctx, refundID, finalAttempt)
}

// EnqueueRefundReversal 排入退款的撤回 job：只在退款列的撤回狀態已是 REVERSING 的同一個 transaction 內呼叫（正常路徑由退款被接受與轉帳成立的 transaction 排入，這裡給測試與補排）。
func (m *Module) EnqueueRefundReversal(ctx context.Context, refundID uuid.UUID) error {
	return m.charges.EnqueueReversal(ctx, refundID)
}

// RefundRequest 是建立退款的輸入（OrderNo、Amount、呼叫端的冪等鍵 RequestKey、是否自動退款 Auto）；Refund 是退款的對外視圖。
type (
	RefundRequest = model.RefundRequest
	PaymentRefund = model.Refund
)

// PaymentRefundStatus 是退款的狀態。
type PaymentRefundStatus = model.RefundStatus

// 退款的狀態。
const (
	RefundPending   = model.RefundPending
	RefundRefunding = model.RefundRefunding
	RefundSucceeded = model.RefundSucceeded
	RefundFailed    = model.RefundFailed
	RefundAnomaly   = model.RefundAnomaly
)

// PaymentReversalStatus 是退款列上向賣家收回（撤回轉帳）的狀態，與退款狀態並行。
type PaymentReversalStatus = model.ReversalStatus

// 向賣家收回的狀態。
const (
	ReversalNone      = model.ReversalNone
	ReversalWaiting   = model.ReversalWaiting
	ReversalReversing = model.ReversalReversing
	ReversalReversed  = model.ReversalReversed
	ReversalReleased  = model.ReversalReleased
	ReversalAnomaly   = model.ReversalAnomaly
)

// PaymentTopupStatus 是退款列上補差額轉帳給賣家的狀態（已算進轉帳金額的退款失敗之後，D11 補充），與退款狀態並行。
type PaymentTopupStatus = model.TopupStatus

// 補差額的狀態。
const (
	TopupNone     = model.TopupNone
	TopupWaiting  = model.TopupWaiting
	TopupSending  = model.TopupSending
	TopupDone     = model.TopupDone
	TopupReleased = model.TopupReleased
	TopupAnomaly  = model.TopupAnomaly
)

// ChargeRefundInput、ChargeRefundSnapshot 與 ChargeRefundState 是退款的 port 型別（測試包裝 provider 用）。
type (
	ChargeRefundInput    = charge.RefundInput
	ChargeRefundSnapshot = charge.RefundSnapshot
	ChargeRefundState    = charge.RefundState
)

// 退款的中立狀態。
const (
	ChargeRefundPending   = charge.RefundPending
	ChargeRefundSucceeded = charge.RefundSucceeded
	ChargeRefundFailed    = charge.RefundFailed
	ChargeRefundUnknown   = charge.RefundUnknown
)

// CreatePaymentRefund 是建立退款的入口（計畫 D4、D5 修訂；步驟 4 的 legacy CreateRefund 與 Java order 的 AutoRefundWorker 呼叫）：一個 transaction 內鎖付款列、
// 以 (付款, RequestKey) 冪等、加總非失敗退款守累計上限，寫 PENDING 退款列並排退款 job（轉帳的任何狀態都可以退款，向賣家收回在 provider 接受之後才開始，D11）；provider 在 transaction 外由 job 呼叫。
// 語意與錯誤見 charge.Service.CreateRefund。呼叫方不能在 transaction 內呼叫。
func (m *Module) CreatePaymentRefund(ctx context.Context, in RefundRequest) (PaymentRefund, error) {
	return m.charges.CreateRefund(ctx, in)
}

// RefundPayment 是退款 job（payment.refund_create）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時暫時性失敗只記警示 log，留給週期掃描。
func (m *Module) RefundPayment(ctx context.Context, refundID uuid.UUID, finalAttempt bool) error {
	return m.charges.Refund(ctx, refundID, finalAttempt)
}

// SyncPaymentRefund 是退款同步 job（payment.refund_sync）的內容，給 worker 與測試呼叫：REFUNDING 的退款重讀 provider 後進 SUCCEEDED 或 FAILED。外部呼叫在 transaction 外；again 為 true 表示讀取期間又收到事件，呼叫方稍後再跑一輪。
func (m *Module) SyncPaymentRefund(ctx context.Context, refundID uuid.UUID) (again bool, err error) {
	return m.charges.SyncRefund(ctx, refundID)
}
