package usecase

import (
	"context"
	"errors"
)

var ErrOrderNotFound = errors.New("order not found")

// CustomerAccountInfo holds buyer account fields fetched from account-service.
type CustomerAccountInfo struct {
	Nickname string
	Avatar   string
	Phone    string
	Email    string
}

// AccountClient fetches buyer account info from account-service.
type AccountClient interface {
	GetAccountInfo(ctx context.Context, accountID int64) (*CustomerAccountInfo, error)
	GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*CustomerAccountInfo, error)
}

// BuyerMeta holds buyer display name and avatar fetched from buyer-service.
type BuyerMeta struct {
	Nickname  string
	AvatarURL string
}

type BuyerProfile struct {
	AccountID int64
	Nickname  string
	Phone     string
	AvatarURL string
}

type BuyerClient interface {
	GetDefaultAddress(ctx context.Context, buyerID int64) (string, error)
	GetBuyerMetaBatchByBuyerIDs(ctx context.Context, buyerIDs []int64) (map[int64]*BuyerMeta, error)
	GetProfileByBuyerID(ctx context.Context, buyerID int64) (*BuyerProfile, error)
}

type CouponClient interface {
	// GetBuyerCouponCount takes the seller's store_id and the buyer's
	// buyer_id — ListBuyerCouponsForSeller matches coupon rows on the same
	// buyer_id that orders carry, so this one is already on the right axis.
	GetBuyerCouponCount(ctx context.Context, storeID, buyerID int64) (int32, error)
}

// OrderInfo holds the store/buyer identity resolved for a given order_no.
type OrderInfo struct {
	OrderNo string
	StoreID int64
	BuyerID int64
}

// OrderClient resolves order identity from order-service — needed because
// aftersales.refund.completed's payload carries only order_no, not
// store_id/buyer identity. Concrete implementation: Task 3.
type OrderClient interface {
	GetOrderByNo(ctx context.Context, orderNo string) (*OrderInfo, error)
}
