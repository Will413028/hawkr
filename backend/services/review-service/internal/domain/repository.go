package domain

import "context"

// ReviewRepository 評論資料存取介面
type ReviewRepository interface {
	Insert(ctx context.Context, r *Review) (*Review, error)
	GetByOrderItemID(ctx context.Context, orderItemID int64) (*Review, error)
	GetByID(ctx context.Context, id int64) (*Review, error)
	ListBySellerID(ctx context.Context, storeID int64, rating int32, limit, offset int32) ([]*Review, error)
	CountBySellerID(ctx context.Context, storeID int64, rating int32) (int64, error)
	GetStatsBySellerID(ctx context.Context, storeID int64) (*SellerStats, error)
	// ListStoreRatingStats 批次版的 GetStatsBySellerID：只回有評論的店，依 store_id 升冪。
	ListStoreRatingStats(ctx context.Context, storeIDs []int64) ([]*StoreRatingStat, error)
	ListByBuyerID(ctx context.Context, buyerID int64, limit, offset int32) ([]*Review, error)
	CountByBuyerID(ctx context.Context, buyerID int64) (int64, error)
	ListByProductID(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*Review, error)
	CountByProductID(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error)
	GetStatsByProductID(ctx context.Context, productID int64) (*ProductStats, error)
	HideReview(ctx context.Context, reviewID int64, reason string) error
	// ReplyReview 賣家回覆評論（#1174）：UPDATE WHERE 含擁有權+可見性，0 rows 回 (nil,nil) 由 usecase 補查分流。可編輯覆蓋。
	ReplyReview(ctx context.Context, reviewID, storeID int64, content string) (*Review, error)
}
