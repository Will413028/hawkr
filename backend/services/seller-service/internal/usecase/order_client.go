package usecase

import "context"

// SellerRevenue represents a seller's revenue ranking data from Order Service.
type SellerRevenue struct {
	StoreID      int64
	TotalRevenue int64
	OrderCount   int32
}

// OrderClient defines the interface for order-service operations.
type OrderClient interface {
	GetTopSellersByRevenue(ctx context.Context, days, limit int32) ([]SellerRevenue, error)
}

// ActiveMerchantOrderCountClient queries active order count for a seller/merchant.
type ActiveMerchantOrderCountClient interface {
	GetActiveMerchantOrderCount(ctx context.Context, merchantAccountID int64) (int, error)
}
