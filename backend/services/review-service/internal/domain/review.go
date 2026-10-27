package domain

import (
	"errors"
	"time"
)

// ErrAlreadyReviewed 同一 order_item_id 重複評論（DB unique constraint violation）
var ErrAlreadyReviewed = errors.New("already reviewed")

const (
	MinRating            = 1
	MaxRating            = 5
	MaxContentLength     = 1000
	MaxImageCount        = 4 // 評論圖片最多 4 張
	ReviewWindowDays     = 30
	OrderStatusCompleted = "50"
	DefaultPageSize      = 20
	MaxPageSize          = 500
	// MaxStoreRatingStatsBatch 是 ListStoreRatingStats 一次最多接受的店家數。
	MaxStoreRatingStatsBatch = 100
)

// Review 訂單商品評論（一筆 = 買家對一個商品項目的評論）
type Review struct {
	CreatedAt    time.Time
	RepliedAt    time.Time // 賣家回覆時間（#1174，零值=未回覆，已回/未回唯一判斷源）
	Content      string
	ReplyContent string // 賣家回覆內容（#1174）
	OrderNo      string
	ItemImageKey string // 商品圖 objectKey 快照（49 §5.3；顯示 URL 由 Gateway 組）
	SkuName      string
	BuyerName    string
	BuyerAvatar  string
	// BuyerDeleted = 這位買家已註銷，BuyerName 是墓碑字串（#2830）。
	// 顯示端要區分「墓碑」與「真名」一律讀本欄，不要對 BuyerName 做字串比對。
	BuyerDeleted bool
	ImageKeys    []string // 評論附圖 objectKey 陣列（49 §6.2）
	BuyerID      int64
	ProductID    int64
	StoreID      int64
	ID           int64
	Price        int64
	OrderItemID  int64
	Rating       int32
}

// OrderEligibility 訂單評論資格
type OrderEligibility struct {
	CompletedAt time.Time
	OrderNo     string
	Status      string
	BuyerID     int64
	StoreID     int64
}

// OrderItemInfo 訂單中的商品項目（從 order-service 取得）
type OrderItemInfo struct {
	SkuName        string
	ImageURL       string // 相容：order 組好的 CDN URL（image_object_key 空時的剝殼 fallback 來源）
	ImageObjectKey string // 理想解：order 直接回的 objectKey（49 §5.3 item_image 回填）
	OrderItemID    int64
	ProductID      int64
	Price          int64
}

// SellerStats 賣家評論統計
type SellerStats struct {
	TotalCount   int64
	AvgRating    float64
	Rating5Count int64
	Rating4Count int64
	Rating3Count int64
	Rating2Count int64
	Rating1Count int64
}

// StoreRatingStat 是一家有評論的店的統計（與 SellerStats 的 TotalCount、AvgRating 同一個定義）。
type StoreRatingStat struct {
	StoreID    int64
	TotalCount int64
	AvgRating  float64
}

// ProductStats 商品評論統計
type ProductStats struct {
	TotalCount int64
	AvgRating  float64
}
