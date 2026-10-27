package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ShippingClient 是 shipping-service 的 gRPC 用戶端
type ShippingClient struct {
	conn           *grpc.ClientConn
	shippingClient pb.ShippingServiceClient
}

// NewShippingClient 建立新的 shipping gRPC 用戶端
func NewShippingClient() (*ShippingClient, error) {
	host := getEnvOrDefault("SHIPPING_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("SHIPPING_SERVICE_GRPC_PORT", "9929")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to shipping service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to shipping gRPC service at %s", target)

	return &ShippingClient{
		conn:           conn,
		shippingClient: pb.NewShippingServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *ShippingClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PrintCvsLabel 超商出貨單列印（同步等待 Webhook 最多 55 秒，gRPC timeout 設 58 秒）
func (c *ShippingClient) PrintCvsLabel(ctx context.Context, req *pb.PrintCvsLabelRequest) (*pb.PrintCvsLabelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 58*time.Second)
	defer cancel()
	return c.shippingClient.PrintCvsLabel(ctx, req)
}

// GetShipmentTracking 買家查看訂單物流追蹤（即時同步 PayUni 當前狀態）
func (c *ShippingClient) GetShipmentTracking(ctx context.Context, req *pb.GetShipmentTrackingRequest) (*pb.GetShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.GetShipmentTracking(ctx, req)
}

// GetSellerShipmentTracking 賣家查看訂單物流追蹤（即時同步 PayUni 當前狀態）
func (c *ShippingClient) GetSellerShipmentTracking(ctx context.Context, req *pb.GetSellerShipmentTrackingRequest) (*pb.GetShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.GetSellerShipmentTracking(ctx, req)
}

// BatchPrintCvsLabel 批次超商出貨單列印
func (c *ShippingClient) BatchPrintCvsLabel(ctx context.Context, req *pb.BatchPrintCvsLabelRequest) (*pb.BatchPrintCvsLabelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.shippingClient.BatchPrintCvsLabel(ctx, req)
}

// GenerateHomeLabelPdf 產宅配託運單 PDF
func (c *ShippingClient) GenerateHomeLabelPdf(ctx context.Context, req *pb.GenerateHomeLabelPdfRequest) (*pb.GenerateHomeLabelPdfResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.GenerateHomeLabelPdf(ctx, req)
}

// BatchGenerateHomeLabelPdf 批次宅配取號 PDF（fan-out 並行，同步等全部完成）
// 50 筆 × 併發 10 ≈ 5 波 × ~10s/筆，預留 timeout 120 秒以容納慢項
func (c *ShippingClient) BatchGenerateHomeLabelPdf(ctx context.Context, req *pb.BatchGenerateHomeLabelPdfRequest) (*pb.BatchGenerateHomeLabelPdfResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	return c.shippingClient.BatchGenerateHomeLabelPdf(ctx, req)
}

// CallTcat 呼叫黑貓預約取件
func (c *ShippingClient) CallTcat(ctx context.Context, req *pb.CallTcatRequest) (*pb.CallTcatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.CallTcat(ctx, req)
}

// ConfirmCvsShipment 超商確認出貨
func (c *ShippingClient) ConfirmCvsShipment(ctx context.Context, req *pb.ConfirmCvsShipmentRequest) (*pb.ConfirmCvsShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ConfirmCvsShipment(ctx, req)
}

// OnboardCvsShipping 超商 C2C 建店
func (c *ShippingClient) OnboardCvsShipping(ctx context.Context, req *pb.OnboardCvsShippingRequest) (*pb.OnboardShippingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.OnboardCvsShipping(ctx, req)
}

// OnboardHomeDeliveryShipping 宅配建店
func (c *ShippingClient) OnboardHomeDeliveryShipping(ctx context.Context, req *pb.OnboardHomeDeliveryRequest) (*pb.OnboardShippingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.OnboardHomeDeliveryShipping(ctx, req)
}

// ListShippingOnboardings 列出建店設定
func (c *ShippingClient) ListShippingOnboardings(ctx context.Context, req *pb.ListShippingOnboardingsRequest) (*pb.ListShippingOnboardingsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListShippingOnboardings(ctx, req)
}

// GetShippingOnboarding 查詢建店詳情
func (c *ShippingClient) GetShippingOnboarding(ctx context.Context, req *pb.GetShippingOnboardingRequest) (*pb.GetShippingOnboardingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.GetShippingOnboarding(ctx, req)
}

// ToggleShippingOnboarding 切換建店設定啟用/停用
func (c *ShippingClient) ToggleShippingOnboarding(ctx context.Context, req *pb.ToggleShippingOnboardingRequest) (*pb.OnboardShippingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ToggleShippingOnboarding(ctx, req)
}

// ListSellerShippingMethods 查詢賣家可用配送方式
func (c *ShippingClient) ListSellerShippingMethods(ctx context.Context, req *pb.ListSellerShippingMethodsRequest) (*pb.ListSellerShippingMethodsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListSellerShippingMethods(ctx, req)
}

// GetStoreMapParams 取得門市地圖加密參數
func (c *ShippingClient) GetStoreMapParams(ctx context.Context, req *pb.GetStoreMapParamsRequest) (*pb.GetStoreMapParamsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.GetStoreMapParams(ctx, req)
}

// DecryptStoreMapResult 解密門市選擇結果
func (c *ShippingClient) DecryptStoreMapResult(ctx context.Context, req *pb.DecryptStoreMapResultRequest) (*pb.DecryptStoreMapResultResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.DecryptStoreMapResult(ctx, req)
}

// GetStoreMapRedirectForm 依 redirect_id 取回 canonical 門市中轉表單（#1394，一次性 consume）
// 供 GET /store_map/redirect?id= 使用；record 不存在/過期/已用 → gRPC NOT_FOUND
func (c *ShippingClient) GetStoreMapRedirectForm(ctx context.Context, redirectID string) (*pb.GetStoreMapRedirectFormResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.GetStoreMapRedirectForm(ctx, &pb.GetStoreMapRedirectFormRequest{
		RedirectId: redirectID,
	})
}

// AddBuyerStorePickup 新增買家常用門市
func (c *ShippingClient) AddBuyerStorePickup(ctx context.Context, req *pb.AddBuyerStorePickupRequest) (*pb.AddBuyerStorePickupResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.AddBuyerStorePickup(ctx, req)
}

// UpdateBuyerStorePickup 更新買家常用門市
func (c *ShippingClient) UpdateBuyerStorePickup(ctx context.Context, req *pb.UpdateBuyerStorePickupRequest) (*pb.UpdateBuyerStorePickupResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.UpdateBuyerStorePickup(ctx, req)
}

// ListBuyerStorePickups 列出買家常用門市
func (c *ShippingClient) ListBuyerStorePickups(ctx context.Context, req *pb.ListBuyerStorePickupsRequest) (*pb.ListBuyerStorePickupsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListBuyerStorePickups(ctx, req)
}

// DeleteBuyerStorePickup 刪除買家常用門市
func (c *ShippingClient) DeleteBuyerStorePickup(ctx context.Context, req *pb.DeleteBuyerStorePickupRequest) (*pb.DeleteBuyerStorePickupResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.DeleteBuyerStorePickup(ctx, req)
}

// ImportHolidays 匯入假日
func (c *ShippingClient) ImportHolidays(ctx context.Context, req *pb.ImportHolidaysRequest) (*pb.ImportHolidaysResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.shippingClient.ImportHolidays(ctx, req)
}

// ImportHolidaysFromContent 由 CSV 內容匯入假日（檔案上傳版）
func (c *ShippingClient) ImportHolidaysFromContent(ctx context.Context, req *pb.ImportHolidaysFromContentRequest) (*pb.ImportHolidaysResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.shippingClient.ImportHolidaysFromContent(ctx, req)
}

// GetHolidayCoverage 查詢各年度假日覆蓋現況
func (c *ShippingClient) GetHolidayCoverage(ctx context.Context, req *pb.GetHolidayCoverageRequest) (*pb.GetHolidayCoverageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.shippingClient.GetHolidayCoverage(ctx, req)
}

// ListBusinessDays 查詢可出貨工作天
func (c *ShippingClient) ListBusinessDays(ctx context.Context, req *pb.ListBusinessDaysRequest) (*pb.ListBusinessDaysResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.shippingClient.ListBusinessDays(ctx, req)
}

// ==================== Admin 異常查詢 API（§29）====================

// ListShipmentsForAdmin 列物流異常
func (c *ShippingClient) ListShipmentsForAdmin(ctx context.Context, req *pb.ListShipmentsForAdminRequest) (*pb.ListShipmentsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListShipmentsForAdmin(ctx, req)
}

// GetShipmentForAdmin 取物流詳情
func (c *ShippingClient) GetShipmentForAdmin(ctx context.Context, req *pb.GetShipmentForAdminRequest) (*pb.GetShipmentForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.GetShipmentForAdmin(ctx, req)
}

// ListSellerShipmentSettingsForAdmin 列建店異常
func (c *ShippingClient) ListSellerShipmentSettingsForAdmin(ctx context.Context, req *pb.ListSellerShipmentSettingsForAdminRequest) (*pb.ListSellerShipmentSettingsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListSellerShipmentSettingsForAdmin(ctx, req)
}

// ==================== §5.21 配送異常賣家通知 ====================

// ListSellerShipmentAnomalies 列賣家配送異常 dashboard
func (c *ShippingClient) ListSellerShipmentAnomalies(ctx context.Context, req *pb.ListSellerShipmentAnomaliesRequest) (*pb.ListSellerShipmentAnomaliesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.ListSellerShipmentAnomalies(ctx, req)
}

// GetSellerShipmentAnomalyStats 賣家配送異常統計
func (c *ShippingClient) GetSellerShipmentAnomalyStats(ctx context.Context, req *pb.GetSellerShipmentAnomalyStatsRequest) (*pb.GetSellerShipmentAnomalyStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.shippingClient.GetSellerShipmentAnomalyStats(ctx, req)
}

// AcknowledgeShipmentAnomaly 賣家標記異常為「已處理」
func (c *ShippingClient) AcknowledgeShipmentAnomaly(ctx context.Context, req *pb.AcknowledgeShipmentAnomalyRequest) (*pb.AcknowledgeShipmentAnomalyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.shippingClient.AcknowledgeShipmentAnomaly(ctx, req)
}

// IgnoreShipmentAnomaly 賣家標記異常為「已忽略」
func (c *ShippingClient) IgnoreShipmentAnomaly(ctx context.Context, req *pb.IgnoreShipmentAnomalyRequest) (*pb.IgnoreShipmentAnomalyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.shippingClient.IgnoreShipmentAnomaly(ctx, req)
}

// ListShipmentAnomaliesForAdmin Admin 平台級配送異常監控
func (c *ShippingClient) ListShipmentAnomaliesForAdmin(ctx context.Context, req *pb.ListShipmentAnomaliesForAdminRequest) (*pb.ListShipmentAnomaliesForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.ListShipmentAnomaliesForAdmin(ctx, req)
}

// AcknowledgeShipmentAnomalyForAdmin Admin 標記異常為「已處理」（issue #1327）
func (c *ShippingClient) AcknowledgeShipmentAnomalyForAdmin(ctx context.Context, req *pb.AcknowledgeShipmentAnomalyForAdminRequest) (*pb.AcknowledgeShipmentAnomalyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.AcknowledgeShipmentAnomalyForAdmin(ctx, req)
}

// IgnoreShipmentAnomalyForAdmin Admin 標記異常為「已忽略」（issue #1327）
func (c *ShippingClient) IgnoreShipmentAnomalyForAdmin(ctx context.Context, req *pb.IgnoreShipmentAnomalyForAdminRequest) (*pb.IgnoreShipmentAnomalyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.shippingClient.IgnoreShipmentAnomalyForAdmin(ctx, req)
}
