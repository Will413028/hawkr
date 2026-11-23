// Package notification 是 notification 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package notification

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"math"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/stream"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
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

// Buyers 是 worker 對 buyer 的需求（*buyer.Module 滿足它）：以 FOR KEY SHARE 鎖住收件買家列，並回報買家是否已關閉。
type Buyers = intake.Buyers

// Stores 是 worker 對 seller 的需求（*seller.Module 滿足它）：以 FOR KEY SHARE 鎖住收件店家列，店家已退場回 NotFound。
type Stores = intake.Stores

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB       *db.DB
	Identity Accounts
	Buyer    Buyers
	Seller   Stores
	Jobs     jobs.Enqueuer
	Now      func() time.Time
	NewID    func() uuid.UUID
	Logger   *slog.Logger
	// Directory 列出公告的收件人（*buyer.Module 與 *seller.Module 經 NewDirectory 組成）；只有公告派送用，沒有公告的測試可以不給。
	Directory announce.Directory
	// ListenConnConfig 是串流 LISTEN 專用連線的設定，必須指向直連或 session 模式的 Postgres，不能經 transaction pooling；nil 時以主連線池的設定另開（不從池借）。
	ListenConnConfig *pgx.ConnConfig
}

// Module 是 notification 模組。
type Module struct {
	intake   *intake.Service
	purge    *purge.Service
	app      *app.Service
	announce *announce.Service
	hub      *stream.Hub

	pool      *db.DB
	listenCfg *pgx.ConnConfig
}

// New 建立模組。
func New(d Deps) *Module { return newModule(d, registry.Registry{}) }

func newModule(d Deps, reg app.Registry) *Module {
	appSvc := app.New(app.Deps{DB: d.DB, Accounts: d.Identity, Registry: reg, Now: d.Now, Logger: d.Logger})
	hub := stream.NewHub(inboxReader(appSvc), d.Logger)
	return &Module{
		intake: intake.New(intake.Deps{
			DB: d.DB, Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller, Registry: reg, Signals: dbSignals{d.DB}, Jobs: d.Jobs, Now: d.Now, NewID: d.NewID, Logger: d.Logger,
		}),
		purge: purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger}),
		app:   appSvc,
		announce: announce.New(announce.Deps{
			DB: d.DB, Locks: intake.Locks{Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller}, Directory: d.Directory,
			Audit: adminaudit.New(d.DB, d.Now), Signals: dbSignals{d.DB}, Jobs: d.Jobs, Now: d.Now, NewID: d.NewID, Logger: d.Logger,
		}),
		hub: hub, pool: d.DB, listenCfg: d.ListenConnConfig,
	}
}

// inboxReader 讀串流訊號的內容：未讀數與最新一則都走 app 用例，與 UnreadCount 同一條規則（只算註冊的型別，含公告）。
func inboxReader(a *app.Service) func(context.Context, stream.Key, int64) (stream.Inbox, error) {
	return func(ctx context.Context, key stream.Key, storeID int64) (stream.Inbox, error) {
		r := app.Recipient{AccountID: key.AccountID, Audience: key.Audience, StoreID: storeID}
		unread, err := a.UnreadCount(ctx, r)
		if err != nil {
			return stream.Inbox{}, err
		}
		latest, err := a.LatestInboxItemID(ctx, r)
		if err != nil {
			return stream.Inbox{}, err
		}
		return stream.Inbox{UnreadCount: int32(min(unread, math.MaxInt32)), LatestItemID: latest}, nil //nolint:gosec // 已夾在 int32 範圍內
	}
}

// dbSignals 把 worker 的收件匣變化轉成同 transaction 的 pg_notify（intake 不依賴 stream）。
type dbSignals struct{ db *db.DB }

func (s dbSignals) InboxChanged(ctx context.Context, accountID int64, audience model.Audience, storeID int64, hint model.StateHint) error {
	return stream.NotifyInbox(ctx, s.db, stream.Key{AccountID: accountID, Audience: audience}, storeID, hint)
}

// StateHint 是串流訊號帶的狀態重讀提示。
type StateHint = model.StateHint

// 狀態重讀提示；StateHintNone 是「不需要重讀」，不會出現在訊號裡。
const (
	StateHintNone         = model.StateHintNone
	StateHintOrder        = model.StateHintOrder
	StateHintAfterSale    = model.StateHintAfterSale
	StateHintSellerStatus = model.StateHintSellerStatus
)

// SellerWorkspaceChange 是賣家工作區哪一類資料變了。
type SellerWorkspaceChange = stream.SellerChange

// 賣家工作區的變更種類。
const (
	SellerOrderChanged     = stream.SellerOrder
	SellerAfterSaleChanged = stream.SellerAfterSale
)

// MaxStreamsPerAccount 是每個帳號同時開著的通知串流上限。
const MaxStreamsPerAccount = stream.MaxPerAccount

// Watcher 是一條通知串流的訂閱：Next 阻塞到下一則訊號（ctx 取消或訂閱關閉時回錯誤），Close 釋出（一定要呼叫）。
type Watcher interface {
	Next(ctx context.Context) (StreamEvent, error)
	Close()
}

// StreamEvent 是串流的一則訊號：Inbox 與賣家工作區（Workspace 非零）恰有一個。
type StreamEvent = stream.Event

// InboxSignal 是收件匣有變化的訊號內容。
type InboxSignal = stream.Inbox

// WatchParams 是訂閱一條串流的條件：STORE 受眾必須帶呼叫者現任的店（StoreID），BUYER 受眾不帶。呼叫者是否有資格（買家身分、店家存在）由呼叫端先確認。
type WatchParams struct {
	AccountID int64
	Audience  Audience
	StoreID   int64
}

// Watch 訂閱 (帳號, 受眾) 的訊號串流，第一次 Next 回連線建立時的 inbox 訊號。帳號同時開著的串流達 MaxStreamsPerAccount 回 resource_exhausted。
// 訂閱者在本行程內，訊號經 Run 的 LISTEN 連線從資料庫收到；上限是每個 serve 行程各自計算，不跨行程。
func (m *Module) Watch(p WatchParams) (Watcher, error) {
	sub, err := m.hub.Subscribe(stream.Key{AccountID: p.AccountID, Audience: p.Audience}, p.StoreID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Run 持有串流的 LISTEN 專用連線，直到 ctx 結束；只有 serve 角色跑（worker 沒有串流，只發通知）。連線來自 Deps.ListenConnConfig。
func (m *Module) Run(ctx context.Context) error {
	cfg := m.listenCfg
	if cfg == nil {
		cfg = m.pool.Pool().Config().ConnConfig
	}
	return m.hub.Run(ctx, stream.DialPG(cfg))
}

// PublishSellerWorkspace 通知這家店的 STORE 受眾訂閱者：賣家工作區的資料有變化，client 重讀列表與統計。
// 給之後搬進單體的 order、aftersales 模組在自己的 transaction 內呼叫（ctx 帶那個 transaction，pg_notify 在 commit 後才送達所有 serve 行程，rollback 不送）；沒有訂閱者時什麼都不會發生。
func (m *Module) PublishSellerWorkspace(ctx context.Context, storeID int64, change SellerWorkspaceChange) error {
	return stream.NotifyWorkspace(ctx, m.pool, storeID, change)
}

// OnSuspended 是 accountsuspension.Suspended 的同步訂閱者：只在發布的 transaction 內登記意圖並排 job。
func (m *Module) OnSuspended(ctx context.Context, e accountsuspension.Suspended) error {
	return m.intake.OnSuspended(ctx, e)
}

// OnUnsuspended 是 accountsuspension.Unsuspended 的同步訂閱者，同 OnSuspended。
func (m *Module) OnUnsuspended(ctx context.Context, e accountsuspension.Unsuspended) error {
	return m.intake.OnUnsuspended(ctx, e)
}

// OnAccountDeleted 是 identity.AccountDeleted 的同步訂閱者：只在發布的 transaction 內排個資清除 job。
func (m *Module) OnAccountDeleted(ctx context.Context, e identity.AccountDeleted) error {
	return m.purge.OnAccountDeleted(ctx, e)
}

// OnBuyerDeleted 是 buyer.BuyerDeleted 的同步訂閱者，同 OnAccountDeleted。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return m.purge.OnBuyerDeleted(ctx, e)
}

// OnStoreRetired 是 seller.StoreRetired 的同步訂閱者，同 OnAccountDeleted。
func (m *Module) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return m.purge.OnStoreRetired(ctx, e)
}

// AddWorkers 登記 River worker（意圖展開、保留期清除、個資清除）。由 cmd/hawkr 的 worker 接上。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.intake.AddWorkers(workers)
	purge.AddWorkers(workers, m.purge)
	m.announce.AddWorkers(workers)
}

// PeriodicJobs 回傳週期 job：保留期清除。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return purge.PeriodicJobs() }
