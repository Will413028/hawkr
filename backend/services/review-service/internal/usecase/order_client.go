package usecase

import (
	"context"

	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// OrderClient 從 order-service 查詢訂單評論資格（service-to-service）
type OrderClient interface {
	// GetOrderForReview 取得訂單評論資格 + 訂單中的商品項目列表
	GetOrderForReview(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error)
}
