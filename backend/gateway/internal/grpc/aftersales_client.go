package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// AftersalesClient 是 aftersales-service 的 gRPC 用戶端
type AftersalesClient struct {
	conn   *grpc.ClientConn
	client pb.AfterSalesServiceClient
}

// NewAftersalesClient 建立新的 aftersales gRPC 用戶端
func NewAftersalesClient() (*AftersalesClient, error) {
	host := getEnvOrDefault("AFTERSALES_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("AFTERSALES_SERVICE_GRPC_PORT", "9923")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to aftersales service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to aftersales gRPC service at %s", target)

	return &AftersalesClient{
		conn:   conn,
		client: pb.NewAfterSalesServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *AftersalesClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== 買家端 ====================

// CreateAfterSaleCase 申請售後
func (c *AftersalesClient) CreateAfterSaleCase(ctx context.Context, req *pb.CreateAfterSaleCaseRequest) (*pb.CreateAfterSaleCaseResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.CreateAfterSaleCase(ctx, req)
}

// GetMyAfterSaleCases 我的售後列表
func (c *AftersalesClient) GetMyAfterSaleCases(ctx context.Context, req *pb.GetMyAfterSaleCasesRequest) (*pb.GetMyAfterSaleCasesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetMyAfterSaleCases(ctx, req)
}

// GetAfterSaleCaseDetail 售後詳情
func (c *AftersalesClient) GetAfterSaleCaseDetail(ctx context.Context, req *pb.GetAfterSaleCaseDetailRequest) (*pb.GetAfterSaleCaseDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetAfterSaleCaseDetail(ctx, req)
}

// SupplementEvidence 補件
func (c *AftersalesClient) SupplementEvidence(ctx context.Context, req *pb.SupplementEvidenceRequest) (*pb.SupplementEvidenceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SupplementEvidence(ctx, req)
}

// CancelAfterSaleCase 取消售後
func (c *AftersalesClient) CancelAfterSaleCase(ctx context.Context, req *pb.CancelAfterSaleCaseRequest) (*pb.CancelAfterSaleCaseResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.CancelAfterSaleCase(ctx, req)
}

// GetRefundPageUrl ATM 退款頁面 URL
func (c *AftersalesClient) GetRefundPageUrl(ctx context.Context, req *pb.GetRefundPageUrlRequest) (*pb.GetRefundPageUrlResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetRefundPageUrl(ctx, req)
}

// ==================== 賣家端 ====================

// GetSellerAfterSaleCases 賣家售後列表
func (c *AftersalesClient) GetSellerAfterSaleCases(ctx context.Context, req *pb.GetSellerAfterSaleCasesRequest) (*pb.GetSellerAfterSaleCasesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerAfterSaleCases(ctx, req)
}

// GetSellerAfterSaleCaseDetail 售後詳情（賣家視角）
func (c *AftersalesClient) GetSellerAfterSaleCaseDetail(ctx context.Context, req *pb.GetSellerAfterSaleCaseDetailRequest) (*pb.GetSellerAfterSaleCaseDetailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerAfterSaleCaseDetail(ctx, req)
}

// ResolveAfterSaleCase 提交決策
func (c *AftersalesClient) ResolveAfterSaleCase(ctx context.Context, req *pb.ResolveAfterSaleCaseRequest) (*pb.ResolveAfterSaleCaseResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ResolveAfterSaleCase(ctx, req)
}

// ConfirmReturnReceipt 確認收貨
func (c *AftersalesClient) ConfirmReturnReceipt(ctx context.Context, req *pb.ConfirmReturnReceiptRequest) (*pb.ConfirmReturnReceiptResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ConfirmReturnReceipt(ctx, req)
}

// CancelShipment 取消出貨
func (c *AftersalesClient) CancelShipment(ctx context.Context, req *pb.CancelShipmentRequest) (*pb.CancelShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.CancelShipment(ctx, req)
}

// ==================== 回調處理 ====================

// HandleRefundPageReturn ATM 退款頁面完成回調（解密 + 更新退款任務 + 回傳 Deep Link）
func (c *AftersalesClient) HandleRefundPageReturn(ctx context.Context, req *pb.HandleRefundPageReturnRequest) (*pb.HandleRefundPageReturnResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.HandleRefundPageReturn(ctx, req)
}

// RetryPrintReturnLabel 7-11 退貨出貨便代碼人工重試列印（§5.3.7）
func (c *AftersalesClient) RetryPrintReturnLabel(ctx context.Context, req *pb.RetryPrintReturnLabelRequest) (*pb.RetryPrintReturnLabelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.RetryPrintReturnLabel(ctx, req)
}

// ==================== §10 申訴與平台裁決 ====================

// SubmitAppeal 買家提交申訴
func (c *AftersalesClient) SubmitAppeal(ctx context.Context, req *pb.SubmitAppealRequest) (*pb.SubmitAppealResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SubmitAppeal(ctx, req)
}

// GetArbitration 買家查詢裁決
func (c *AftersalesClient) GetArbitration(ctx context.Context, req *pb.GetArbitrationRequest) (*pb.GetArbitrationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetArbitration(ctx, req)
}

// GetSellerArbitration 賣家 read-only 查裁決
func (c *AftersalesClient) GetSellerArbitration(ctx context.Context, req *pb.GetSellerArbitrationRequest) (*pb.GetArbitrationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerArbitration(ctx, req)
}

// ListAppealsForAdmin Admin 待裁決列表
func (c *AftersalesClient) ListAppealsForAdmin(ctx context.Context, req *pb.ListAppealsForAdminRequest) (*pb.ListAppealsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListAppealsForAdmin(ctx, req)
}

// DecideAppealForAdmin Admin 裁決
func (c *AftersalesClient) DecideAppealForAdmin(ctx context.Context, req *pb.DecideAppealForAdminRequest) (*pb.DecideAppealForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.DecideAppealForAdmin(ctx, req)
}

// GetAppealDetailForAdmin Admin 單案詳情（§10.6.6）
func (c *AftersalesClient) GetAppealDetailForAdmin(ctx context.Context, req *pb.GetAppealDetailForAdminRequest) (*pb.GetAppealDetailForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetAppealDetailForAdmin(ctx, req)
}

// ListAfterSaleCasesForAdmin §29 Admin 異常售後列表
func (c *AftersalesClient) ListAfterSaleCasesForAdmin(ctx context.Context, req *pb.ListAfterSaleCasesForAdminRequest) (*pb.ListAfterSaleCasesForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ListAfterSaleCasesForAdmin(ctx, req)
}

// ForceConfirmReceiptForAdmin §5.3.12 Admin 強制確認收貨
func (c *AftersalesClient) ForceConfirmReceiptForAdmin(ctx context.Context, req *pb.ForceConfirmReceiptForAdminRequest) (*pb.ForceConfirmReceiptForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ForceConfirmReceiptForAdmin(ctx, req)
}
