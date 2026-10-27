package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// OrderClient 是 order-service 的 gRPC 用戶端
type OrderClient struct {
	conn        *grpc.ClientConn
	orderClient pb.OrderServiceClient
}

// NewOrderClient 建立新的 order gRPC 用戶端
func NewOrderClient() (*OrderClient, error) {
	host := getEnvOrDefault("ORDER_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("ORDER_SERVICE_GRPC_PORT", "9915")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to order gRPC service at %s", target)

	return &OrderClient{
		conn:        conn,
		orderClient: pb.NewOrderServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *OrderClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== 買家端 ====================

// CreateOrder 建立訂單
func (c *OrderClient) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.orderClient.CreateOrder(ctx, req)
}

// CancelOrder 取消訂單
func (c *OrderClient) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.CancelOrder(ctx, req)
}

// GetMyOrders 查詢買家訂單列表
func (c *OrderClient) GetMyOrders(ctx context.Context, req *pb.GetMyOrdersRequest) (*pb.GetMyOrdersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetMyOrders(ctx, req)
}

// GetMyOrderStatusCounts 查詢買家各狀態訂單筆數（訂單 tab 筆數 rollup）
func (c *OrderClient) GetMyOrderStatusCounts(ctx context.Context, req *pb.GetMyOrderStatusCountsRequest) (*pb.GetMyOrderStatusCountsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetMyOrderStatusCounts(ctx, req)
}

// GetOrder 查詢訂單詳情（買家視角）
func (c *OrderClient) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetOrder(ctx, req)
}

// ConfirmReceipt 買家確認收貨
func (c *OrderClient) ConfirmReceipt(ctx context.Context, req *pb.ConfirmReceiptRequest) (*pb.ConfirmReceiptResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.ConfirmReceipt(ctx, req)
}

// ==================== 賣家端 ====================

// GetSellerOrders 查詢賣家訂單列表
func (c *OrderClient) GetSellerOrders(ctx context.Context, req *pb.GetSellerOrdersRequest) (*pb.GetSellerOrdersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetSellerOrders(ctx, req)
}

// GetSellerOrderDetail 查詢賣家訂單詳情
func (c *OrderClient) GetSellerOrderDetail(ctx context.Context, req *pb.GetSellerOrderDetailRequest) (*pb.GetSellerOrderDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetSellerOrderDetail(ctx, req)
}

// GetBuyerOrdersForSeller 賣家查詢指定買家的消費紀錄
func (c *OrderClient) GetBuyerOrdersForSeller(ctx context.Context, req *pb.GetBuyerOrdersForSellerRequest) (*pb.GetBuyerOrdersForSellerResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetBuyerOrdersForSeller(ctx, req)
}

// ==================== 管理端 ====================

// SearchOrders 後台搜尋訂單
func (c *OrderClient) SearchOrders(ctx context.Context, req *pb.SearchOrdersRequest) (*pb.SearchOrdersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.SearchOrders(ctx, req)
}

// GetAdminOrderDetail 後台查詢訂單詳情
func (c *OrderClient) GetAdminOrderDetail(ctx context.Context, req *pb.GetAdminOrderDetailRequest) (*pb.GetAdminOrderDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetAdminOrderDetail(ctx, req)
}

// ==================== 結帳模組 ====================

// CheckoutValidate 結帳即時驗證
func (c *OrderClient) CheckoutValidate(ctx context.Context, req *pb.CheckoutValidateRequest) (*pb.CheckoutValidateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.CheckoutValidate(ctx, req)
}

// CheckoutShippingFee 結帳運費精算
func (c *OrderClient) CheckoutShippingFee(ctx context.Context, req *pb.CheckoutShippingFeeRequest) (*pb.CheckoutShippingFeeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.CheckoutShippingFee(ctx, req)
}

// CheckoutPreview 結帳預覽
func (c *OrderClient) CheckoutPreview(ctx context.Context, req *pb.CheckoutPreviewRequest) (*pb.CheckoutPreviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.orderClient.CheckoutPreview(ctx, req)
}

// CheckoutConfirm 確認結帳（建單）
func (c *OrderClient) CheckoutConfirm(ctx context.Context, req *pb.CheckoutConfirmRequest) (*pb.CheckoutConfirmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.orderClient.CheckoutConfirm(ctx, req)
}

// GetTopSellersByRevenue 查詢近 N 天營收排名前 N 的賣家
func (c *OrderClient) GetTopSellersByRevenue(ctx context.Context, days, limit int32) (*pb.GetTopSellersByRevenueResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.orderClient.GetTopSellersByRevenue(ctx, &pb.GetTopSellersByRevenueRequest{
		Days:  days,
		Limit: limit,
	})
}

// SnapshotPaymentInfo 快照付款資訊到訂單
func (c *OrderClient) SnapshotPaymentInfo(ctx context.Context, req *pb.SnapshotPaymentInfoRequest) (*pb.SnapshotPaymentInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.orderClient.SnapshotPaymentInfo(ctx, req)
}

// RetryGrant Admin 重發撥款任務（收斂規範 §8.4 人工介入閉環）
func (c *OrderClient) RetryGrant(ctx context.Context, req *pb.RetryGrantRequest) (*pb.RetryGrantResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.orderClient.RetryGrant(ctx, req)
}

// ListAbnormalOrdersForAdmin §29 Admin 異常訂單列表
func (c *OrderClient) ListAbnormalOrdersForAdmin(ctx context.Context, req *pb.ListAbnormalOrdersForAdminRequest) (*pb.ListAbnormalOrdersForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.orderClient.ListAbnormalOrdersForAdmin(ctx, req)
}
