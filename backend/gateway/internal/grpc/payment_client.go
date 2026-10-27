package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// PaymentClient 是 Payment Service 的 gRPC 客戶端
type PaymentClient struct {
	conn   *grpc.ClientConn
	client pb.PaymentServiceClient
	// responsibility 是 FulfillmentResponsibilityService（責任判定 maker-checker）的
	// stub。與 client 共用同一條連線——同一個 .proto、同一個 payment-service、同一個
	// port，獨立 service name 只是為了讓授權與寫入邊界分得開。
	//
	// ⚠️ 它是**欄位**而不是像 logisticsFinance() 那樣現建：那條軸的守衛（decision 值域、
	// 雙人制）需要「不合法輸入時下游零呼叫」的陰性斷言，而現建 stub 的寫法讓 handler
	// 測試無法注入替身。未經 NewPaymentClient 建立的 client 上它是 nil ⇒ 呼叫即 panic，
	// 那是正確的 fail-loud：比對著一條 nil 連線靜默送出請求好。
	responsibility pb.FulfillmentResponsibilityServiceClient
}

// NewPaymentClient 建立新的 Payment gRPC 客戶端
func NewPaymentClient() (*PaymentClient, error) {
	host := getEnvOrDefault("PAYMENT_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("PAYMENT_SERVICE_GRPC_PORT", "9925")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to payment service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to payment gRPC service at %s", target)

	return &PaymentClient{
		conn:           conn,
		client:         pb.NewPaymentServiceClient(conn),
		responsibility: pb.NewFulfillmentResponsibilityServiceClient(conn),
	}, nil
}

// Close 關閉 gRPC 連線
func (c *PaymentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== Payment Methods ====================

// CreatePayment 建立支付交易
func (c *PaymentClient) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.CreatePayment(ctx, req)
}

func (c *PaymentClient) GetPayment(ctx context.Context, userID int64, tradeNo string) (*pb.GetPaymentResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()

	return c.client.GetPayment(rpcCtx, &pb.GetPaymentRequest{
		TradeNo: tradeNo,
	})
}

// GetPaymentHistory §2A.3.6 查詢同 order_id 的所有付款嘗試。userID 語意同
// GetPayment——必須是已解析的 buyer_id。
func (c *PaymentClient) GetPaymentHistory(ctx context.Context, userID int64, orderId string) (*pb.GetPaymentHistoryResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()

	return c.client.GetPaymentHistory(rpcCtx, &pb.GetPaymentHistoryRequest{
		OrderId: orderId,
	})
}

// GetSellerPaymentHistory #2501 賣家視角付款歷程。storeID 是呼叫端（賣家）已解析好的
// 現任 store_id（account_id → storeid.Resolver），以 request 欄位傳給 payment-service；
// payment-service 仍以 canonical order 反解的 store_id 再次比對，跨店回同形 NOT_FOUND。
// storeID 也透過 x-account-id metadata 傳遞供 audit（授權比對走 request.store_id）。
func (c *PaymentClient) GetSellerPaymentHistory(ctx context.Context, storeID int64, orderId string) (*pb.GetSellerPaymentHistoryResponse, error) {
	rpcCtx, cancel := ctxWithAccount(ctx, 30*time.Second, storeID)
	defer cancel()

	return c.client.GetSellerPaymentHistory(rpcCtx, &pb.GetSellerPaymentHistoryRequest{
		OrderId: orderId,
		StoreId: storeID,
	})
}

func (c *PaymentClient) SyncPaymentStatus(ctx context.Context, userID int64, tradeNo string) (*pb.SyncPaymentStatusResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 15*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 15*time.Second)
	}
	defer cancel()

	return c.client.SyncPaymentStatus(rpcCtx, &pb.SyncPaymentStatusRequest{
		TradeNo: tradeNo,
	})
}

// ClosePayment 關閉支付交易。userID 語意同 SyncPaymentStatus。
func (c *PaymentClient) ClosePayment(ctx context.Context, userID int64, tradeNo string) (*pb.ClosePaymentResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()

	return c.client.ClosePayment(rpcCtx, &pb.ClosePaymentRequest{
		TradeNo: tradeNo,
	})
}

// ==================== Refund Methods ====================

// GetRefund 查詢退款詳情。userID 語意同 SyncPaymentStatus。
func (c *PaymentClient) GetRefund(ctx context.Context, userID int64, refundNo string) (*pb.GetRefundResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()

	return c.client.GetRefund(rpcCtx, &pb.GetRefundRequest{
		RefundNo: refundNo,
	})
}

// GetRefundsByTradeNo 查詢交易的所有退款。userID 語意同 SyncPaymentStatus。
func (c *PaymentClient) GetRefundsByTradeNo(ctx context.Context, userID int64, tradeNo string) (*pb.GetRefundsByTradeNoResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()

	return c.client.GetRefundsByTradeNo(rpcCtx, &pb.GetRefundsByTradeNoRequest{
		TradeNo: tradeNo,
	})
}

// GetPublicPaymentStatus 查詢付款狀態（公開，免驗證 — 供付款結果回調頁面使用）
func (c *PaymentClient) GetPublicPaymentStatus(ctx context.Context, tradeNo string) (*pb.GetPublicPaymentStatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetPublicPaymentStatus(ctx, &pb.GetPublicPaymentStatusRequest{
		TradeNo: tradeNo,
	})
}

// GetPaymentReturnPage 取得付款結果回調頁面（含 HTML + Deep Link）
func (c *PaymentClient) GetPaymentReturnPage(ctx context.Context, req *pb.GetPaymentReturnPageRequest) (*pb.GetPaymentReturnPageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetPaymentReturnPage(ctx, req)
}

// GetPaymentRedirectForm 依 redirect_id 取回 canonical 付款中轉表單（#1394，一次性 consume）
// 供 GET /payment/redirect?id= 使用；record 不存在/過期/已用 → gRPC NOT_FOUND
func (c *PaymentClient) GetPaymentRedirectForm(ctx context.Context, redirectID string) (*pb.GetPaymentRedirectFormResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.GetPaymentRedirectForm(ctx, &pb.GetPaymentRedirectFormRequest{
		RedirectId: redirectID,
	})
}

// HandlePaymentPageReturn 信用卡付款完成回調處理（解密 + 更新 DB + 回傳 Deep Link）
func (c *PaymentClient) HandlePaymentPageReturn(ctx context.Context, req *pb.HandlePaymentPageReturnRequest) (*pb.HandlePaymentPageReturnResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.HandlePaymentPageReturn(ctx, req)
}

// ==================== Admin 異常查詢 API（§29）====================

// ListPaymentTransactionsForAdmin 列付款異常
func (c *PaymentClient) ListPaymentTransactionsForAdmin(ctx context.Context, req *pb.ListPaymentTransactionsForAdminRequest) (*pb.ListPaymentTransactionsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ListPaymentTransactionsForAdmin(ctx, req)
}

// GetPaymentTransactionForAdmin 取付款詳情
func (c *PaymentClient) GetPaymentTransactionForAdmin(ctx context.Context, req *pb.GetPaymentTransactionForAdminRequest) (*pb.GetPaymentTransactionForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetPaymentTransactionForAdmin(ctx, req)
}

// ListRefundsForAdmin 列退款異常
func (c *PaymentClient) ListRefundsForAdmin(ctx context.Context, req *pb.ListRefundsForAdminRequest) (*pb.ListRefundsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ListRefundsForAdmin(ctx, req)
}

// GetRefundForAdmin 取退款詳情
func (c *PaymentClient) GetRefundForAdmin(ctx context.Context, req *pb.GetRefundForAdminRequest) (*pb.GetRefundForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetRefundForAdmin(ctx, req)
}

// ListGrantsForAdmin 列撥款異常
func (c *PaymentClient) ListGrantsForAdmin(ctx context.Context, req *pb.ListGrantsForAdminRequest) (*pb.ListGrantsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ListGrantsForAdmin(ctx, req)
}

// ValidateApplePayMerchant 對 Apple validationURL 取 merchantSession（05 號設計書 §5.11）
func (c *PaymentClient) ValidateApplePayMerchant(ctx context.Context, req *pb.ValidateApplePayMerchantRequest) (*pb.ValidateApplePayMerchantResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ValidateApplePayMerchant(ctx, req)
}
