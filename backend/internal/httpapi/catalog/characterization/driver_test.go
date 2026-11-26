// Package characterization 是 catalog 對外 API 的特性測試（ADR 2026-09-24-go-modular-monolith D8 的第一層
// 安全網、ADR 2026-09-26-external-contract-protobuf-connect D4），共用的部分在 internal/httpapi/characterize。
//
// 情境只寫一次（scenarios_test.go），透過 Driver 操作：REST driver（rest_test.go、gateway_test.go，build tag gateway）經 gateway
// 打舊的 Java product-service；Connect driver（connect_test.go、monolith_integration_test.go，build tag integration）在行程內起
// 單體的 Connect 入口，打 CatalogPublicService、SellerCatalogAppService 與 CatalogAdminService。
//
// 情境鎖定的是 Java product-service 當時的行為，沒有驗證那行為是否正確；疑似缺陷以 CD 編號標明
// （backend/docs/plans/2026-11-24-b4-catalog/defects.md）。計畫決定要改的行為（D 編號）寫成修正後的斷言，在
// REST driver 上以 skipOnLegacy 略過並附編號。product-service 刪除時，REST driver、legacy.sh 與 TestLegacy
// 一起刪除，情境檔的 build tag 改成 integration。
//
// 單體的契約依 D24 修訂（api-baseline-brief.md）：ACTIVE 的寫入至少要有一張主圖（D22，ProductInput.MainImage）、庫存只由
// SetStock／AdjustStock 改（D23、D24 Q13，SellerSetStock）、ARCHIVED 的公開商品頁仍讀得到但不可購買（D24 Q10）、賣家列表的
// 關鍵字由伺服器端比對（D24 Q11）。Connect 的情境另外鎖定 D22 的拒絕與 SetStock 的預期值比對。
//
// 情境只看業務上的結果：ID 當成不透明的字串，商品只有一個 SKU（D18 沒有選項軸的商品），價格是買家付的
// Price 與劃線的 ListPrice（D19），錯誤只比 characterize 的分類與下面的 catalog 錯誤。
//
// 不在這裡覆蓋（盤點 client-inventory.md 的去向）：D5 決定刪除、沒有 client 的路由；認證（範圍裁切）；
// 運費試算（D10）；代行路由（B4-5d 自己的測試）；admin 改狀態（B4-5 改成商品限制措施）；庫存扣還與購物車、
// 結帳的內部 gRPC（B4-2、B4-4 的 integration 與轉接測試）；店家停權與不可曝光（legacy.sh 把兩家種子店設成
// 可曝光，B4-3 integration 覆蓋）。
package characterization

import (
	"context"
	"errors"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// 種子帳號（infra/seed-data/01-account.sql）：兩家賣家的店家 ID。
const (
	StoreSeller  = "1009"
	StoreSeller2 = "1004"
)

// ErrWrongStatus 是商品狀態不允許這個操作：characterize 的分類放不下的失敗。
var ErrWrongStatus = errors.New("商品狀態不允許此操作")

// ErrStockMismatch 是 SetStock 的預期庫存與目前的庫存不符（D24 Q13）。
var ErrStockMismatch = errors.New("庫存已經變動")

// Status 是賣家看到的商品生命週期（D16）。
type Status string

const (
	StatusDraft    Status = "DRAFT"
	StatusActive   Status = "ACTIVE"
	StatusArchived Status = "ARCHIVED"
)

// ProductInput 是建立或編輯商品的內容。Price 是買家付的價格；ListPrice 是劃線的原價，0 表示沒有。
type ProductInput struct {
	Name       string
	CategoryID string
	Price      int64
	ListPrice  int64
	Stock      int64
	Hashtags   []string
	// MainImage 為真時商品帶一張主圖（D22：ACTIVE 的寫入至少一張）；舊的 product-service 沒有這個規則，REST driver 不送。
	MainImage bool
}

// Product 是讀回的商品。Hashtags 是正規化後的名稱；Purchasable 只有公開商品頁會帶。
type Product struct {
	ID          string
	Name        string
	StoreID     string
	CategoryID  string
	Status      Status
	PriceMin    int64
	PriceMax    int64
	Stock       int64
	Hashtags    []string
	Purchasable bool
}

// Category 是分類樹的一個節點；Leaf 表示沒有子分類，商品只能掛在葉節點（D8）。
type Category struct {
	ID       string
	Name     string
	ParentID string
	Leaf     bool
}

// PublicQuery 是公開列表的篩選；空字串與 0 不送。
type PublicQuery struct {
	Keyword    string
	CategoryID string
	StoreID    string
	PriceMin   int64
	PriceMax   int64
}

// Hashtag 是自動完成的一個結果；Count 是可曝光店的上架商品數。
type Hashtag struct {
	Name  string
	Count int64
}

// Driver 是情境操作 catalog 的方式；錯誤對應到 characterize 的分類與上面的 catalog 錯誤。
type Driver interface {
	// Legacy 回報這是不是對舊的 product-service 跑（skipOnLegacy 用）。
	Legacy() bool
	// StoreOf 是呼叫者（賣家）的店家 ID。
	StoreOf(c ch.Caller) string
	// Unknown 是格式正確但不存在的商品 ID。
	Unknown() string

	Categories(ctx context.Context, c ch.Caller) ([]Category, error)

	SellerCreate(ctx context.Context, c ch.Caller, in ProductInput) (Product, error)
	SellerGet(ctx context.Context, c ch.Caller, id string) (Product, error)
	// SellerList 以關鍵字列出呼叫者自己店的商品（含未上架）。
	SellerList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error)
	// SellerUpdate 整體取代商品內容；庫存不在內容裡（D24 Q13，in.Stock 只有舊的 REST 會送）。
	SellerUpdate(ctx context.Context, c ch.Caller, id string, in ProductInput) (Product, error)
	// SellerSetStock 把商品唯一的 SKU 的庫存設成 quantity，expected 是呼叫者看到的現值（D24 Q13：不符是 ErrStockMismatch；
	// 舊的 REST 沒有這個比對，不檢查）。
	SellerSetStock(ctx context.Context, c ch.Caller, id string, quantity, expected int64) error
	// SellerSetListed 上架（true）或下架（false）。
	SellerSetListed(ctx context.Context, c ch.Caller, id string, listed bool) error
	SellerDelete(ctx context.Context, c ch.Caller, id string) error

	PublicGet(ctx context.Context, c ch.Caller, id string) (Product, error)
	PublicList(ctx context.Context, c ch.Caller, q PublicQuery) ([]Product, error)
	Search(ctx context.Context, c ch.Caller, keyword string) ([]Product, error)
	Suggest(ctx context.Context, c ch.Caller, prefix string) ([]string, error)
	HashtagAutocomplete(ctx context.Context, c ch.Caller, prefix string) ([]Hashtag, error)

	AdminList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error)
	AdminGet(ctx context.Context, c ch.Caller, id string) (Product, error)
}
