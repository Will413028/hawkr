// Package content 是 content 模組的公開 API：首頁 banner 與 App 版本門檻。
package content

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/appversion"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/banner"
	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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
	DB *db.DB
	// Social 回答貼文是不是公開可讀：banner 的貼文目標建立時要公開可讀、讀取時隱藏失效的。組裝點給 *social.Module。
	Social Social
	// Media 是 media 模組（*media.Module）：banner 的圖在寫入的 transaction 內 Confirm，被換掉或終態的資產 MarkDeleted。
	Media model.Media
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

// Social 是 content 對 social 的需求（banner 的貼文目標）。
type Social = banner.Social

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
}

// New 建立模組。
func New(d Deps) *Module {
	if d.Social == nil {
		panic("content：缺 Social")
	}
	return &Module{
		banner: banner.New(banner.Deps{
			DB: d.DB, Social: d.Social, Media: d.Media, NewID: d.NewID, Now: d.Now,
		}),
		appversion: appversion.New(appversion.Deps{DB: d.DB, Logger: d.Logger}),
	}
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
