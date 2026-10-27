// Package content 是 content 模組的公開 API：首頁 banner 與 App 版本門檻。
package content

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/appversion"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/banner"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB    *db.DB
	Media media.Media
	Jobs  jobs.Enqueuer
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}
type BannerActionType = model.BannerActionType
type BannerAction = model.BannerAction
type Banner = model.Banner
type BannerCursor = model.BannerCursor
type BannerPage = model.BannerPage
type BannerInput = model.BannerInput
type UpdateBannerInput = model.UpdateBannerInput
type AppType = model.AppType
type AppVersionGate = model.AppVersionGate
type VersionCheck = model.VersionCheck

const (
	BannerActionPopup   = model.BannerActionPopup
	BannerActionURL     = model.BannerActionURL
	BannerActionProduct = model.BannerActionProduct
	BannerActionPost    = model.BannerActionPost
	AppTypeBuyer        = model.AppTypeBuyer
	AppTypeSeller       = model.AppTypeSeller
)

// AppTypes 是所有的 App 類型。
func AppTypes() []AppType { return model.AppTypes() }

// Module 是 content 模組。
type Module struct {
	banner     *banner.Service
	appversion *appversion.Service
	mediaSync  *media.RefSyncer
}

// New 建立模組。
func New(d Deps) *Module {
	return &Module{
		banner: banner.New(banner.Deps{
			DB: d.DB, Media: d.Media, Jobs: d.Jobs, NewID: d.NewID, Now: d.Now,
		}),
		appversion: appversion.New(appversion.Deps{DB: d.DB, Logger: d.Logger}),
		mediaSync:  mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger),
	}
}

// SyncMedia 同步一個 media ref（banner 的兩張圖，ref_id 是列的 UUID）的引用：Link 業務列目前的圖，Unlink
// 退場的圖。它是 content.media_sync job 的內容，給 worker 與測試呼叫；回傳是否要再跑一輪。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// AddWorkers 登記本模組的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) {
	mediaref.AddWorkers(workers, m.mediaSync)
}

// PeriodicJobs 回傳 media 引用的保底掃描（啟動即執行、每 15 分鐘）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return mediaref.PeriodicJobs() }

// SweepMedia 執行 media 引用的保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref；回傳補排的 ref 數。它是
// content.media_sweep job 的內容，給 worker 與測試呼叫。
func (m *Module) SweepMedia(ctx context.Context) (int, error) {
	return m.mediaSync.Sweep(ctx)
}

// ListBanners 回傳一頁 banner。
func (m *Module) ListBanners(ctx context.Context, after *BannerCursor, size int32) (BannerPage, error) {
	return m.banner.ListBanners(ctx, after, size)
}

// ListLiveBanners 回傳前台此刻要顯示的一頁 banner。
func (m *Module) ListLiveBanners(ctx context.Context, after *BannerCursor, size int32) (BannerPage, error) {
	return m.banner.ListLiveBanners(ctx, after, size)
}

// Banner 回傳一則 banner。
func (m *Module) Banner(ctx context.Context, id uuid.UUID) (Banner, error) {
	return m.banner.Banner(ctx, id)
}

// CreateBanner 新增 banner。
func (m *Module) CreateBanner(ctx context.Context, adminID int64, in BannerInput) (Banner, error) {
	return m.banner.CreateBanner(ctx, adminID, in)
}

// UpdateBanner 整筆覆寫 banner。
func (m *Module) UpdateBanner(ctx context.Context, in UpdateBannerInput) (Banner, error) {
	return m.banner.UpdateBanner(ctx, in)
}

// DeleteBanner 刪除 banner。
func (m *Module) DeleteBanner(ctx context.Context, id uuid.UUID, version int64) error {
	return m.banner.DeleteBanner(ctx, id, version)
}

// CheckAppVersion 判斷是否必須更新 App。
func (m *Module) CheckAppVersion(ctx context.Context, appType AppType, clientVersion string) (VersionCheck, error) {
	return m.appversion.CheckAppVersion(ctx, appType, clientVersion)
}

// AppVersionGate 回傳一組門檻設定。
func (m *Module) AppVersionGate(ctx context.Context, appType AppType) (AppVersionGate, error) {
	return m.appversion.AppVersionGate(ctx, appType)
}

// ListAppVersionGates 回傳全部門檻設定。
func (m *Module) ListAppVersionGates(ctx context.Context) ([]AppVersionGate, error) {
	return m.appversion.ListAppVersionGates(ctx)
}

// CreateAppVersionGate 建立一組門檻。
func (m *Module) CreateAppVersionGate(ctx context.Context, g AppVersionGate) (AppVersionGate, error) {
	return m.appversion.CreateAppVersionGate(ctx, g)
}

// UpdateAppVersionGate 整筆覆寫門檻。
func (m *Module) UpdateAppVersionGate(ctx context.Context, g AppVersionGate, version int64) (AppVersionGate, error) {
	return m.appversion.UpdateAppVersionGate(ctx, g, version)
}
