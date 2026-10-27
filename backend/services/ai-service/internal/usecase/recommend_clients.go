package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

var ErrProductOwnerUnavailable = errors.New("product: cannot resolve the owning store of this product")

type ProductItem struct {
	Name      string
	ImageURL  string
	ID        int64
	PriceMin  int64
	PriceMax  int64
	Rating    float64
	StoreID   identity.StoreID
	CreatedAt int64
}

type SellerStatsItem struct {
	StoreID       identity.StoreID
	TotalProducts int32
	TotalOrders   int32
	TotalSales    float64
	Rating        float64
	Followers     int32
}

type SellerMetaItem struct {
	SellerName string

	LogoKey string

	StoreCode string
	AccountID identity.AccountID
}

type ProductSearchFilter struct {
	Keyword     string
	PriceMaxNTD *int
}

type ProductClient interface {
	GetProductList(ctx context.Context, page, pageSize int) ([]*ProductItem, int64, error)
	ListSellerProducts(ctx context.Context, storeID identity.StoreID, page, pageSize int) ([]*ProductItem, error)
	GetStockBatch(ctx context.Context, skuIDs []string) (map[string]int64, error)
	SearchProducts(ctx context.Context, filter ProductSearchFilter) ([]*ProductItem, error)

	GetProductStoreID(ctx context.Context, productID int64, callerStoreID identity.StoreID) (identity.StoreID, error)
}

type SellerClient interface {
	GetSellerStats(ctx context.Context, storeID identity.StoreID) (*SellerStatsItem, error)
	BatchGetSellerMeta(ctx context.Context, storeIDs []identity.StoreID) (map[identity.StoreID]*SellerMetaItem, error)

	SearchProspects(ctx context.Context, params ProspectSearchParams) ([]ProspectHit, error)
}

type ProspectSearchParams struct {
	Category string
	Region   string
	Keywords string

	CategoryIDs []string
	Limit       int
}

type ProspectHit struct {
	ID              int64
	DisplayName     string
	Category        string
	Region          string
	ProductSummary  string
	Positioning     string
	ResearchSummary string

	PublicWebsite  string
	PriorityLetter string
	PriorityStars  int
	UpdatedAt      time.Time

	// 品牌素材 objectKey（顯示 URL 由 Gateway 組）。站外業者沒有平台 store，
	// 這兩欄不是賣家身分，也不表示已驗證。
	LogoKey  string
	CoverKey string

	HitCategory    bool
	HitRegion      bool
	HitKeywords    bool
	HitCategoryIDs bool
}
