// Package imageai 是 imageai 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package imageai

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/admin"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/limit"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/quota"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/style"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/transform"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
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

// Schema 是本模組的 schema 名稱。
const Schema = "imageai"

const (
	// DefaultWeeklyLimit 是沒有個別設定的店每週可修圖的次數。
	DefaultWeeklyLimit = model.DefaultWeeklyLimit
	// MaxWeeklyLimit 是後台能為一家店設定的每週次數上限。
	MaxWeeklyLimit = model.MaxWeeklyLimit
)

// ErrQuotaExhausted 是本週額度用完。
var ErrQuotaExhausted = model.ErrQuotaExhausted

// ErrNothingToRefund 是 RefundQuota 找不到可退的次數（那一週的計數已不存在或已歸零）。
var ErrNothingToRefund = model.ErrNothingToRefund

// Usage 是一次扣額度之後的本週狀態。
type Usage = model.Usage

// 一次修圖的輸入、輸出與風格；StoreID 是呼叫者的店家，由入口層從 session 推導。
type (
	TransformInput  = model.TransformInput
	TransformOutput = model.TransformOutput
	Style           = model.Style
	StyleList       = model.StyleList
	StyleCategory   = model.StyleCategory
)

// 後台的風格與額度（ImageAiAdminService）。
type (
	AdminStyle       = model.AdminStyle
	AdminQuota       = model.AdminQuota
	Actor            = model.Actor
	CreateStyleInput = model.CreateStyleInput
	UpdateStyleInput = model.UpdateStyleInput
	UpdateQuotaInput = model.UpdateQuotaInput
)

const (
	// CategoryProduct 是商品圖用途。
	CategoryProduct = model.CategoryProduct
	// CategoryPost 是貼文圖用途。
	CategoryPost = model.CategoryPost
	// MaxImageBytes 是修圖原圖的大小上限。
	MaxImageBytes = model.MaxImageBytes
	// MaxPromptRunes 是修圖提示詞的字元數上限。
	MaxPromptRunes = model.MaxPromptRunes
)

// StoreRetired 是 seller 在店家退場時同 transaction 發布的事件（本模組的訂閱者見 Module.OnStoreRetired）。
type StoreRetired = seller.StoreRetired

// Config 是模組的設定。
type Config struct {
	// MaxConcurrent 是每個 pod 同時進行的修圖數上限。
	MaxConcurrent int
}

// ReadConfig 讀 IMAGEAI_MAX_CONCURRENT（選填，預設 21，至少 1）。
func ReadConfig(e *config.Env) Config {
	n := e.Int("IMAGEAI_MAX_CONCURRENT", model.DefaultMaxConcurrent)
	if n < 1 {
		e.Fail("IMAGEAI_MAX_CONCURRENT 至少要 1，得到 %d", n)
	}
	return Config{MaxConcurrent: n}
}

// Deps 是模組需要的外部依賴；新增 use case 時由根套件逐欄組裝。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
	// Seller 在扣額度前確認店家未退場（*seller.Module）。
	Seller StoreGuard
	// Images 是圖片模型（llm.NewImage(...).Generator()）；停用時回 llm.ErrImageDisabled 的實作。
	Images llm.ImageGenerator
	Log    *slog.Logger
	// MaxConcurrent 是每個 pod 同時進行的修圖數上限；0 取預設。
	MaxConcurrent int
	// Stores 讓後台確認店家存在（*seller.Module）；後台的額度方法才需要。
	Stores StoreReader
	// Media 與 Jobs 是風格縮圖的轉正與引用同步（後台的風格方法與 worker 才需要）。
	Media media.Media
	Jobs  jobs.Enqueuer
	// NewID 產生風格的 ID（UUIDv7）；後台新增風格才需要。
	NewID func() uuid.UUID
}

// StoreReader 讀店家（*seller.Module 滿足它）；店家不存在回 NotFound。
type StoreReader interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// StoreGuard 在呼叫方的 transaction 內鎖住店家列、確認尚未退場；已退場與不存在回 NotFound（*seller.Module 滿足它）。
type StoreGuard interface {
	LockNotRetired(ctx context.Context, storeID int64) error
}

// Module 是 imageai 模組。
type Module struct {
	quota     *quota.Service
	limiter   *limit.Limiter
	styles    *style.Service
	transform *transform.Service
	admin     *admin.Service
	mediaSync *media.RefSyncer
}

// New 建立模組。
func New(d Deps) *Module {
	n := d.MaxConcurrent
	if n == 0 {
		n = model.DefaultMaxConcurrent
	}
	q := quota.New(d.DB, d.Now)
	l := limit.New(n)
	m := &Module{
		quota: q, limiter: l, styles: style.New(d.DB),
		transform: transform.New(transform.Deps{Slots: l, Quota: q, Guard: d.Seller, Tx: d.DB.InTx, Generator: d.Images, Log: d.Log}),
	}
	if d.Media != nil && d.Jobs != nil {
		m.mediaSync = mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Log)
	}
	m.admin = admin.New(admin.Deps{
		Tx:     d.DB.InTx,
		Q:      admin.QueriesOf(d.DB),
		Audit:  adminaudit.New(d.DB, d.Now),
		Thumbs: &mediaref.Thumbnails{DB: d.DB, Media: d.Media, Jobs: d.Jobs},
		Stores: adminStores{read: d.Stores, guard: d.Seller},
		Used: func(ctx context.Context, storeID int64) (int32, error) {
			return q.Used(ctx, storeID, quota.WeekStart(d.Now()))
		},
		NewID: d.NewID,
		Now:   d.Now,
	})
	return m
}

// adminStores 把 seller 的兩個方法接成後台要的介面。
type adminStores struct {
	read  StoreReader
	guard StoreGuard
}

func (a adminStores) Exists(ctx context.Context, storeID int64) error {
	_, err := a.read.Store(ctx, storeID)
	return err
}

func (a adminStores) LockNotRetired(ctx context.Context, storeID int64) error {
	return a.guard.LockNotRetired(ctx, storeID)
}

// Transform 以提示詞修一張圖並回傳這次之後的本週用量。輸入不合（Image、MimeType、Prompt 欄位）回 InvalidInput；同時修圖滿了回 ResourceExhausted
// （帶 RetryAfter，不扣額度）；本週額度用完回 ResourceExhausted 與 ReasonImageQuotaExceeded；店家已退場回 PermissionDenied；模型擋下回 InvalidInput 與
// ReasonImageSafetyBlocked 或 ReasonImageRecitationBlocked，沒有產出圖片回 Unavailable 與 ReasonImageNotProduced，供應商無法呼叫回 Unavailable。
// 扣了額度之後的任何失敗都退回額度。
func (m *Module) Transform(ctx context.Context, in TransformInput) (TransformOutput, error) {
	return m.transform.Transform(ctx, in)
}

// ListStyles 列出 category 用途的啟用中風格（不含預設風格）與預設提示詞；category 不合法回 InvalidInput（欄位 Category）。
func (m *Module) ListStyles(ctx context.Context, category StyleCategory) (StyleList, error) {
	return m.styles.List(ctx, category)
}

// CurrentUsage 是店家此刻的本週用量，唯讀、不扣額度；讀取失敗回錯誤。
func (m *Module) CurrentUsage(ctx context.Context, storeID int64) (Usage, error) {
	return m.quota.Current(ctx, storeID)
}

// AcquireTransformSlot 不等待地佔一個同時修圖的名額；滿了回 ok=false。release 可重複呼叫，只釋放一次。
func (m *Module) AcquireTransformSlot() (release func(), ok bool) { return m.limiter.TryAcquire() }

// ConsumeQuota 扣店家本週一次；用完回 ErrQuotaExhausted（Usage 仍帶上限與重置時間）。
func (m *Module) ConsumeQuota(ctx context.Context, storeID int64) (Usage, error) {
	return m.quota.Consume(ctx, storeID)
}

// RefundQuota 退回 ConsumeQuota 扣的那一次（用回傳的 Usage.PeriodStart）；沒有可退的回 ErrNothingToRefund。永遠在呼叫端的 transaction 之外執行。
func (m *Module) RefundQuota(ctx context.Context, storeID int64, periodStart time.Time) error {
	return m.quota.Refund(ctx, storeID, periodStart)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：刪該店的個別額度與用量。冪等。
func (m *Module) OnStoreRetired(ctx context.Context, e StoreRetired) error {
	return m.quota.OnStoreRetired(ctx, e.StoreID)
}

// SyncMedia 同步一個 media ref（風格縮圖，ref_id 是風格的 UUID）的引用：Link 風格目前的圖，Unlink 退場的圖。它是 imageai.media_sync job 的內容，
// 給 worker 與測試呼叫；回傳是否要再跑一輪。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// SweepMedia 執行 media 引用的保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref；回傳補排的 ref 數。它是 imageai.media_sweep job 的內容。
func (m *Module) SweepMedia(ctx context.Context) (int, error) { return m.mediaSync.Sweep(ctx) }

// AddWorkers 登記本模組的 River worker（media 引用同步）。
func (m *Module) AddWorkers(workers *river.Workers) { mediaref.AddWorkers(workers, m.mediaSync) }

// PeriodicJobs 回傳 media 引用的保底掃描（啟動即執行、每 15 分鐘）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return mediaref.PeriodicJobs() }

// ListStylesForAdmin 列出風格（含停用的），依顯示順序；category 為 nil 是全部用途，不合法回 InvalidInput（欄位 Category）。
func (m *Module) ListStylesForAdmin(ctx context.Context, category *StyleCategory) ([]AdminStyle, error) {
	return m.admin.ListStyles(ctx, category)
}

// CreateStyle 新增風格（縮圖必須是新上傳的 staged token；同 transaction 寫稽核）。輸入不合（Category、Name、Label、Prompt、ThumbnailToken、Enabled 欄位）回 InvalidInput；
// 用途內名稱重複回 AlreadyExists；設成預設時取消同用途原本的預設。
func (m *Module) CreateStyle(ctx context.Context, in CreateStyleInput) (AdminStyle, error) {
	return m.admin.CreateStyle(ctx, in)
}

// UpdateStyle 整筆覆寫風格可改的欄位（Version 必填，不符回 VersionConflict；ThumbnailToken 空字串沿用目前的圖；同 transaction 寫稽核）。
// 預設風格不在這裡改；停用目前的預設回 FailedPrecondition；風格不存在回 NotFound。
func (m *Module) UpdateStyle(ctx context.Context, in UpdateStyleInput) (AdminStyle, error) {
	return m.admin.UpdateStyle(ctx, in)
}

// SetDefaultStyle 把風格設成它用途的預設風格，同 transaction 取消原本的預設並寫一筆稽核（detail 記原預設的 id）；version 為 0 不檢查版本，不符回 VersionConflict。
// 目標停用回 FailedPrecondition，不存在回 NotFound；已經是預設時不改任何東西、不寫稽核，回傳該風格。
func (m *Module) SetDefaultStyle(ctx context.Context, actor Actor, id uuid.UUID, version int64) (AdminStyle, error) {
	return m.admin.SetDefault(ctx, actor, id, version)
}

// ReadStyleForAdmin 回傳一個風格（後台），不存在回 NotFound；給 handler 套 update_mask 的基底。
func (m *Module) ReadStyleForAdmin(ctx context.Context, id uuid.UUID) (AdminStyle, error) {
	return m.admin.Style(ctx, id)
}

// DeleteStyle 刪除風格並釋放縮圖（同 transaction 寫稽核）；version 為 0 不檢查版本。用途的預設風格回 FailedPrecondition，不存在回 NotFound。
func (m *Module) DeleteStyle(ctx context.Context, actor Actor, id uuid.UUID, version int64) error {
	return m.admin.DeleteStyle(ctx, actor, id, version)
}

// GetQuota 回傳一家店的每週額度與本週使用量；沒有個別設定回預設額度（Custom 為 false）。店家不存在回 NotFound。
func (m *Module) GetQuota(ctx context.Context, storeID int64) (AdminQuota, error) {
	return m.admin.GetQuota(ctx, storeID)
}

// UpdateQuota 為一家店個別設定每週額度（同 transaction 寫稽核）：沒有個別設定且沒帶版本時建立；已有個別設定卻沒帶版本回 AlreadyExists（VERSION_CONFLICT）；
// 版本過期回 VersionConflict；weekly_limit 超出 0 到 MaxWeeklyLimit 回 InvalidInput（欄位 WeeklyLimit）；店家不存在或已退場回 NotFound。
func (m *Module) UpdateQuota(ctx context.Context, in UpdateQuotaInput) (AdminQuota, error) {
	return m.admin.UpdateQuota(ctx, in)
}

// DeleteQuota 刪除一家店的個別設定（同 transaction 寫稽核）；店家不存在或沒有個別設定回 NotFound；version 為 0 不檢查版本。
func (m *Module) DeleteQuota(ctx context.Context, actor Actor, storeID int64, version int64) error {
	return m.admin.DeleteQuota(ctx, actor, storeID, version)
}
