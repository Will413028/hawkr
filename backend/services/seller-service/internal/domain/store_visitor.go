package domain

import (
	"context"
	"time"
)

// StoreVisitorRow 代表一筆商店訪客記錄（買家專屬——訪客必須解析出現任 buyer_id 才會被記錄，見 gateway GetSellerInfo）
type StoreVisitorRow struct {
	LastVisitedAt  time.Time
	VisitorBuyerID int64
	VisitCount     int32
}

// StoreVisitorRepository 定義商店訪客資料的持久化介面
type StoreVisitorRepository interface {
	UpsertVisit(ctx context.Context, storeID, visitorBuyerID int64) error
	List(ctx context.Context, storeID int64, limit, offset int32) ([]*StoreVisitorRow, error)
	Count(ctx context.Context, storeID int64) (int64, error)
	CountByDateRange(ctx context.Context, storeID int64, from, to time.Time) (int64, error)
}
