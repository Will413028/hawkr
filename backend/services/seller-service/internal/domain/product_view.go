package domain

import (
	"context"
	"time"
)

// ProductViewRepository 定義商品瀏覽記錄的持久化介面
type ProductViewRepository interface {
	Insert(ctx context.Context, id, sellerID, productID int64, source string) error
	CountByDateRange(ctx context.Context, sellerID int64, from, to time.Time) (int64, error)
}
