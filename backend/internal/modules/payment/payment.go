// Package payment 是 payment 模組的公開 API：現階段只擁有收款帳戶（Stripe 計畫 D28）。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md；行為規格在 docs/modules/payment.md。
package payment

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/account"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
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
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Inbox    *inbox.Inbox
	Provider Provider
	Owners   Owners
	Now      func() time.Time
	Logger   *slog.Logger
}

// Module 是 payment 模組。
type Module struct {
	accounts *account.Service
}

// New 建立模組。
func New(d Deps) *Module {
	return &Module{accounts: account.New(account.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Inbox: d.Inbox, Provider: d.Provider, Owners: d.Owners, Now: d.Now, Logger: d.Logger,
	})}
}

// Provider 是收款開通的 port（Stripe adapter 與 Fake 實作）。
type Provider = account.Provider

// Fake 是記憶體內的 provider，測試與本機用。
type Fake = account.Fake

// Snapshot 是外部帳戶某一刻的狀態。
type Snapshot = account.Snapshot

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

// ResolveInput 是以店家對帳（列出 provider 上沒有關閉的帳戶）的輸入。
type ResolveInput = account.ResolveInput

// MappingDeletion 是管理員「已在金流商處理，刪除對應」的一筆紀錄。
type MappingDeletion = model.MappingDeletion

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

// IdempotencyRetention 是金流商保存冪等鍵結果的時間（30 天）：超過之後建立與清理改以 metadata.store_id 列出對帳。
const IdempotencyRetention = account.IdempotencyRetention

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

// RetryAccountCreate 管理員的「重試建立」：只有建立失敗的帳戶可以重試。
func (m *Module) RetryAccountCreate(ctx context.Context, storeID int64) (Account, error) {
	return m.accounts.RetryCreate(ctx, storeID)
}

// RetryAccountPurge 管理員的「重試關閉」：只有清理進了終局的帳戶可以重試（清除終局、期限從現在重算並重新排入清理 job）。
func (m *Module) RetryAccountPurge(ctx context.Context, storeID int64) (Account, error) {
	return m.accounts.RetryPurge(ctx, storeID)
}

// DeleteAccountMapping 管理員的「已在金流商處理，刪除對應」：只有清理進了終局的帳戶可以做；寫一筆只增不改的紀錄（操作的管理員與時間）並刪列，不呼叫金流商。
func (m *Module) DeleteAccountMapping(ctx context.Context, storeID, adminID int64) error {
	return m.accounts.DeleteMapping(ctx, storeID, adminID)
}

// AccountMappingDeletions 讀回店家的「刪除對應」紀錄（新的在前）。
func (m *Module) AccountMappingDeletions(ctx context.Context, storeID int64) ([]MappingDeletion, error) {
	return m.accounts.MappingDeletions(ctx, storeID)
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

// PurgeAccount 是「關閉後刪除」job（payment.account_purge）的內容，給 worker 與測試呼叫；finalAttempt 為 true 時關閉失敗只記警示 log，本地列（已標成等待清理）留著給週期掃描。
func (m *Module) PurgeAccount(ctx context.Context, storeID int64, finalAttempt bool) error {
	return m.accounts.Purge(ctx, storeID, finalAttempt)
}

// AddWorkers 登記 River worker。
func (m *Module) AddWorkers(workers *river.Workers) { m.accounts.AddWorkers(workers) }

// PeriodicJobs 回傳週期 job。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return m.accounts.PeriodicJobs() }
