package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

// SellerOrder is an order summary used by the AI assistant.
type SellerOrder struct {
	OrderNo      string
	Status       string            // numeric code as string, e.g. "0", "10"
	BuyerName    string            // 收件人姓名（from OrderSummaryView.recipient_name）
	Items        []SellerOrderItem // 訂單品項明細
	Amount       int64
	CreateTimeMs int64
}

// SellerOrderItem is a single item within a seller order.
type SellerOrderItem struct {
	ProductName string
	SkuName     string
	SkuID       string
	Quantity    int
}

type OrderDetailSummary struct {
	OrderNo        string
	Status         string
	BuyerID        string
	ShipmentNo     string
	ShippingStatus string
	Amount         int64
}

// BuyerOrder is an order summary for the buyer AI assistant.
type BuyerOrder struct {
	OrderNo      string
	Status       string
	SellerName   string
	Amount       int64
	CreateTimeMs int64
	ItemCount    int
}

// BuyerOrderDetail is a concise buyer order detail for the AI assistant.
type BuyerOrderDetail struct {
	OrderNo        string
	Status         string
	SellerName     string
	ShippingMethod string
	ShippingStatus string
	RecipientName  string
	PaymentMethod  string
	Amount         int64
	CreateTimeMs   int64
	ShippingFee    int64
}

// storeID/buyerID params below are already the caller's CURRENT store_id /
// buyer_id (resolved by gateway before the call reaches ai-service —
// account_id 命名債正名), never a raw login account_id. The named types make
// mixing the two a compile error rather than a silent wrong-row lookup.
type OrderClient interface {
	GetSellerOrders(ctx context.Context, storeID identity.StoreID, statusFilter string, page, pageSize int) ([]*SellerOrder, error)
	GetSellerOrderDetail(ctx context.Context, storeID identity.StoreID, orderNo string) (*OrderDetailSummary, error)
	GetMyOrders(ctx context.Context, buyerID identity.BuyerID, statusFilter string, page, pageSize int) ([]*BuyerOrder, error)
	GetBuyerOrderDetail(ctx context.Context, buyerID identity.BuyerID, orderNo string) (*BuyerOrderDetail, error)
}
