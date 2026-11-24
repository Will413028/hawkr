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
	"net/http"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/stream"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/suppress"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
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

// StoreCodes 讀取多家店的 store_code（store_id 到 store_code）。
type StoreCodes = app.StoreCodes

// Contacts 是渠道 job 對 identity 的需求（*identity.Module 滿足它）：帳號的聯絡信箱與來源，只寄已驗證的信箱。
type Contacts = deliver.Contacts

// Locales 回傳帳號的 preferred_locale：推播與 email 在送出時依它組字（identity 擁有這個欄位）。
type Locales = deliver.Locales

// SuppressionKeys 是 email suppression 名單的位址雜湊密鑰（Current 寫入與查詢，Previous 只在輪替期間查詢）。
type SuppressionKeys = suppress.Keys

// MinSuppressionKeyBytes 是密鑰的最短長度。
const MinSuppressionKeyBytes = suppress.MinKeyBytes

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
	// Contacts、Locales、Push、Mail 是渠道 job（推播與 email）的依賴；Push 與 Mail 在沒有設定真實 provider 的環境是各自的替身（platform/fcm、platform/resend 的 Fake）。
	Contacts Contacts
	Locales  Locales
	Push     fcm.Sender
	Mail     resend.Sender
	// ImageURL 把型別的圖片 object key 轉成推播用的絕對網址；nil 表示沒有設定媒體網域，推播不帶圖。
	ImageURL func(key string) string
	// SuppressionKeys 是 email suppression 名單的 HMAC 密鑰（Current 必填才寄得出 email；Previous 只在輪替期間給，D22）。
	SuppressionKeys SuppressionKeys
	// Inbox 是 platform 的 webhook inbox：給了就登記 Resend 的退信與投訴事件（每個行程只建一個模組）；ResendWebhookSecret 為空時 webhook 回 FailedPrecondition。
	Inbox               *inbox.Inbox
	ResendWebhookSecret string
	// Directory 列出公告的收件人（*buyer.Module 與 *seller.Module 經 NewDirectory 組成）；只有公告派送用，沒有公告的測試可以不給。
	Directory announce.Directory
	// Resolver 是 worker 端解析收件人、封鎖檢查與追蹤者展開要問的地方（*buyer.Module 與 *seller.Module 經 NewResolver 組成）；
	// 只有宣告 Expand 或 Broadcast 的型別用到，沒有這類型別的測試可以不給。
	Resolver Resolver
	// Validity 是送出前重查來源事實（D27）要問的地方（*coupon.Module 經 NewValidity 組成）；沒有宣告重查的型別的測試可以不給。
	Validity Validity
	// StoreCodes 讀取店家的 store_code，內容型別的深連結讀取時用（*seller.Module 經 NewStoreCodes 組成）；沒有內容型別的測試可以不給。
	StoreCodes StoreCodes
	// ListenConnConfig 是串流 LISTEN 專用連線的設定，必須指向直連或 session 模式的 Postgres，不能經 transaction pooling；nil 時以主連線池的設定另開（不從池借）。
	ListenConnConfig *pgx.ConnConfig
}

// Module 是 notification 模組。
type Module struct {
	intake   *intake.Service
	purge    *purge.Service
	app      *app.Service
	announce *announce.Service
	deliver  *deliver.Service
	suppress *suppress.Service
	hub      *stream.Hub

	pool      *db.DB
	listenCfg *pgx.ConnConfig
}

// New 建立模組。
func New(d Deps) *Module { return newModule(d, registry.Registry{}) }

func newModule(d Deps, reg app.Registry) *Module {
	appSvc := app.New(app.Deps{DB: d.DB, Accounts: d.Identity, Registry: reg, Signals: dbSignals{d.DB}, StoreCodes: d.StoreCodes, Now: d.Now, Logger: d.Logger})
	hub := stream.NewHub(inboxReader(appSvc), d.Logger, d.Now)
	locks := intake.Locks{Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller}
	hasher := suppress.NewHasher(d.SuppressionKeys)
	return &Module{
		intake: intake.New(intake.Deps{
			DB: d.DB, Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller, Registry: reg, Resolver: d.Resolver, Signals: dbSignals{d.DB}, Jobs: d.Jobs, Now: d.Now, NewID: d.NewID, Logger: d.Logger,
		}),
		purge: purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger}),
		app:   appSvc,
		announce: announce.New(announce.Deps{
			DB: d.DB, Locks: intake.Locks{Accounts: d.Identity, Buyers: d.Buyer, Stores: d.Seller}, Directory: d.Directory,
			Audit: adminaudit.New(d.DB, d.Now), Signals: dbSignals{d.DB}, Jobs: d.Jobs, Now: d.Now, NewID: d.NewID, Logger: d.Logger,
		}),
		deliver: deliver.New(deliver.Deps{
			DB: d.DB, Locks: locks, Contacts: d.Contacts, Locales: d.Locales, Registry: reg, Validity: d.Validity, Sources: appSvc,
			Push: d.Push, ImageURL: d.ImageURL, Mail: d.Mail, Suppression: hasher, Now: d.Now, Logger: d.Logger,
		}),
		suppress: suppress.New(suppress.Deps{DB: d.DB, Inbox: d.Inbox, WebhookSecret: d.ResendWebhookSecret, Hasher: hasher, Now: d.Now}),
		hub:      hub, pool: d.DB, listenCfg: d.ListenConnConfig,
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

func (s dbSignals) InboxChanged(ctx context.Context, accountID int64, audience model.Audience, storeID int64, hint model.StateHint, itemID uuid.UUID) error {
	return stream.NotifyInbox(ctx, s.db, stream.Key{AccountID: accountID, Audience: audience}, storeID, hint, itemID)
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

// ErrClosed 是串流所屬的服務正在關閉：Watch 與 Watcher.Next 回這個錯誤時，client 應依 backoff 重連。
var ErrClosed = stream.ErrClosed

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

// OnCommentCreated 是 social.CommentCreated 的同步訂閱者，同 OnSuspended。
func (m *Module) OnCommentCreated(ctx context.Context, e social.CommentCreated) error {
	return m.intake.OnCommentCreated(ctx, e)
}

// OnCommentRepliedToBuyer 是 social.CommentReplied 的同步訂閱者，同 OnSuspended。
func (m *Module) OnCommentRepliedToBuyer(ctx context.Context, e social.CommentReplied) error {
	return m.intake.OnCommentRepliedToBuyer(ctx, e)
}

// OnCommentRepliedToStore 是 social.CommentReplied 的同步訂閱者，同 OnSuspended。
func (m *Module) OnCommentRepliedToStore(ctx context.Context, e social.CommentReplied) error {
	return m.intake.OnCommentRepliedToStore(ctx, e)
}

// OnCommentLikedBuyerAuthor 是 social.CommentLiked 的同步訂閱者，同 OnSuspended。
func (m *Module) OnCommentLikedBuyerAuthor(ctx context.Context, e social.CommentLiked) error {
	return m.intake.OnCommentLikedBuyerAuthor(ctx, e)
}

// OnCommentLikedStoreAuthor 是 social.CommentLiked 的同步訂閱者，同 OnSuspended。
func (m *Module) OnCommentLikedStoreAuthor(ctx context.Context, e social.CommentLiked) error {
	return m.intake.OnCommentLikedStoreAuthor(ctx, e)
}

// OnPostLiked 是 social.PostLiked 的同步訂閱者，同 OnSuspended。
func (m *Module) OnPostLiked(ctx context.Context, e social.PostLiked) error {
	return m.intake.OnPostLiked(ctx, e)
}

// OnPostPublished 是 social.PostPublished 的同步訂閱者，同 OnSuspended。
func (m *Module) OnPostPublished(ctx context.Context, e social.PostPublished) error {
	return m.intake.OnPostPublished(ctx, e)
}

// OnSellerFollowed 是 buyer.SellerFollowed 的同步訂閱者，同 OnSuspended。
func (m *Module) OnSellerFollowed(ctx context.Context, e buyer.SellerFollowed) error {
	return m.intake.OnSellerFollowed(ctx, e)
}

// OnRestockTargeted 是 buyer.RestockTargeted 的同步訂閱者，同 OnSuspended。
func (m *Module) OnRestockTargeted(ctx context.Context, e buyer.RestockTargeted) error {
	return m.intake.OnRestockTargeted(ctx, e)
}

// OnVoucherExpiring 是 coupon.VoucherExpiring 的同步訂閱者，同 OnSuspended；組裝時要以 coupon.ExpiringHandler 登記（SubscribeNamed），發布方才會要求它存在。
func (m *Module) OnVoucherExpiring(ctx context.Context, e coupon.VoucherExpiring) error {
	return m.intake.OnVoucherExpiring(ctx, e)
}

// OnReviewCreated 是 review.ReviewCreated 的同步訂閱者，同 OnSuspended。
func (m *Module) OnReviewCreated(ctx context.Context, e review.ReviewCreated) error {
	return m.intake.OnReviewCreated(ctx, e)
}

// OnMessageSentToBuyer 是 im.MessageSent 的同步訂閱者，同 OnSuspended。
func (m *Module) OnMessageSentToBuyer(ctx context.Context, e im.MessageSent) error {
	return m.intake.OnMessageSentToBuyer(ctx, e)
}

// OnMessageSentToStore 是 im.MessageSent 的同步訂閱者，同 OnSuspended。
func (m *Module) OnMessageSentToStore(ctx context.Context, e im.MessageSent) error {
	return m.intake.OnMessageSentToStore(ctx, e)
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

// PurgeTokensOnSuspended 是 accountsuspension.Suspended 的同步訂閱者：在發布的 transaction 內刪掉帳號的全部裝置 token（店家停權不走這個事件，token 不動）。
func (m *Module) PurgeTokensOnSuspended(ctx context.Context, e accountsuspension.Suspended) error {
	return m.intake.PurgeTokensOnSuspended(ctx, e)
}

// PurgeTokensOnAccountDeleted 是 identity.AccountDeleted 的同步訂閱者：在發布的 transaction 內刪掉帳號的全部裝置 token。
func (m *Module) PurgeTokensOnAccountDeleted(ctx context.Context, e identity.AccountDeleted) error {
	return m.intake.PurgeTokensOnAccountDeleted(ctx, e)
}

// ReceiveResendEvent 是 Resend 退信與投訴 webhook 的入口：驗簽、同一個 transaction 寫 platform/inbox 並排入派送 job（commit 之後才回 2xx）。
// 沒有設定密鑰回 FailedPrecondition，簽章或格式不合回 InvalidInput。
func (m *Module) ReceiveResendEvent(ctx context.Context, header http.Header, body []byte) error {
	return m.suppress.Receive(ctx, header, body)
}

// RemoveEmailSuppression 把位址從 email suppression 名單移除（管理員處理；只有內部方法，沒有 RPC）：只移除永久退信，回傳是否有移除；
// 位址是投訴回 FailedPrecondition（投訴不可移除），位址格式不合回 InvalidInput，不在名單回 false。
func (m *Module) RemoveEmailSuppression(ctx context.Context, address string) (bool, error) {
	return m.suppress.RemoveHardBounce(ctx, address)
}

// AddWorkers 登記 River worker（意圖展開、渠道投遞、保留期清除、個資清除）。由 cmd/hawkr 的 worker 接上。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.intake.AddWorkers(workers)
	m.deliver.AddWorkers(workers)
	purge.AddWorkers(workers, m.purge)
	m.announce.AddWorkers(workers)
}

// PeriodicJobs 回傳週期 job：保留期清除與過期裝置 token 清除。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return purge.PeriodicJobs() }
