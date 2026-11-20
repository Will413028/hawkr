// Package buyeragent 是買家助理的 ADK agent 與 3 支 tool（get_my_orders、get_buyer_order_detail、track_my_package）。
//
// agent 與 tool 由 Deps 為「一輪」建構：tool 的完成結果經 Deps.OnTool 回報給輪次執行器，tool 的身分範圍取自輪次的買家，
// 不信任模型傳來的任何參數（docs/tech/llm-app-backend-pitfalls：每個會執行動作的工具自己驗授權與租戶範圍）。
package buyeragent

import (
	"context"
	"time"
)

// Orders 是買家助理讀訂單的 port，由 composition root 以訂單服務的 client 實作。buyerID 是目前的 buyer_id。
type Orders interface {
	// MyOrders 回買家的訂單；statusFilter 空字串是全部。
	MyOrders(ctx context.Context, buyerID int64, statusFilter string, page, pageSize int) ([]Order, error)
	// OrderDetail 回買家的單筆訂單；不存在或不屬於該買家時回錯誤。
	OrderDetail(ctx context.Context, buyerID int64, orderNo string) (OrderDetail, error)
}

// Order 是訂單列表的一列。
type Order struct {
	OrderNo    string
	Status     string
	SellerName string
	Amount     int64
	CreateTime time.Time
	ItemCount  int64
}

// OrderDetail 是單筆訂單的摘要。
type OrderDetail struct {
	OrderNo        string
	Status         string
	SellerName     string
	ShippingMethod string
	ShippingStatus string
	RecipientName  string
	PaymentMethod  string
	Amount         int64
	CreateTime     time.Time
	ShippingFee    int64
}
