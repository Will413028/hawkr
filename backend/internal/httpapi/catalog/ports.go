// Package catalog 是 catalog 模組的 Connect handler：公開的商品瀏覽與搜尋（hawkr.catalog.v1alpha1.CatalogPublicService）、賣家管理自己店家的商品
// （SellerCatalogAppService）與後台的唯讀查詢（hawkr.admin.catalog.v1alpha1.CatalogAdminService）。
// 這裡只做欄位的轉換、身分的推導與店家卡片的組裝；規則、可見性與冪等在 internal/modules/catalog。
package catalog

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// PublicModule 是公開讀取用到的 catalog 方法（*catalog.Module）。每個方法都帶請求的 clock（at）：店家可不可曝光依它判斷，不用 client 的時間。
type PublicModule interface {
	PublicProduct(ctx context.Context, productID uuid.UUID, at time.Time) (catalog.Product, error)
	ListPublicProducts(ctx context.Context, at time.Time, f catalog.PublicFilter, sort catalog.ProductSort, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
	SuggestProductNames(ctx context.Context, at time.Time, query string, limit int32) ([]string, error)
	SuggestHashtags(ctx context.Context, at time.Time, query string, contains bool, limit int32) ([]catalog.HashtagCount, error)
	TrendingHashtags(ctx context.Context, at time.Time, limit int32) ([]catalog.HashtagCount, error)
	Categories(ctx context.Context) ([]catalog.Category, error)
}

// SellerModule 是賣家方法用到的 catalog 方法（*catalog.Module）；storeID 一律是 handler 由登入身分推導的店家。
type SellerModule interface {
	ListSellerProducts(ctx context.Context, storeID int64, f catalog.SellerFilter, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
	CountSellerProducts(ctx context.Context, storeID int64, f catalog.SellerFilter) (int32, error)
	Product(ctx context.Context, storeID int64, productID uuid.UUID) (catalog.Product, error)
	OwnSku(ctx context.Context, storeID int64, skuID uuid.UUID) (catalog.SkuState, error)
	CreateProduct(ctx context.Context, storeID int64, uploader media.Uploader, requestID uuid.UUID, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	UpdateProduct(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, version int64, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	SetProductStatus(ctx context.Context, storeID int64, productID uuid.UUID, version int64, to catalog.ProductStatus, actor *delegatedactor.Actor) error
	DeleteProduct(ctx context.Context, storeID int64, productID uuid.UUID, version int64) error
	AdjustStock(ctx context.Context, storeID int64, key string, skuID uuid.UUID, delta int32) error
	SetStock(ctx context.Context, storeID int64, key string, skuID uuid.UUID, quantity, expected int32) error
}

// AdminModule 是後台用到的 catalog 方法（*catalog.Module）。
type AdminModule interface {
	AdminProduct(ctx context.Context, productID uuid.UUID) (catalog.Product, error)
	ListAdminProducts(ctx context.Context, f catalog.AdminFilter, after *catalog.ListCursor, size int32) (catalog.ProductPage, int32, error)
}

// StoreSource 是店家卡片的來源（*seller.Module）：商品卡片上的店家。
type StoreSource interface {
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
}

// ActingStores 是賣家方法的店家來源（*seller.Module）：回傳呼叫帳號此刻的店，只在它有指定能力時成功。
type ActingStores interface {
	ActingStore(ctx context.Context, accountID int64, c seller.Capability) (seller.Store, error)
}
