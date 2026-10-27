package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// InvoiceServiceClient 是 invoice-service（58 號設計書）的 gRPC 用戶端。
//
// ⚠️ 這裡有**兩個**不同的 deadline，不是一個（§0.1）：
//   - invoiceDeadline（INVOICE_GRPC_DEADLINE_MS，預設 3000）：一般同步端點。開票一律
//     非同步 ⇒ 請求路徑上不會打 AMEGO ⇒ 不必大於任何 AMEGO deadline。設計書逐字寫了
//     「不照抄 aftersales_client.go 的 30 s 硬編碼」——那個數字沒有配套的 p99 預算。
//   - invoiceCredentialTestDeadline（INVOICE_GRPC_DEADLINE_CREDENTIAL_TEST_MS，預設
//     15000）：端點 25／44／66 是全書唯一在請求路徑上同步等 AMEGO 的三支，走 AMEGO
//     查詢類 deadline（10 s）＋ 5 s 餘裕。用 3 s 會讓「測試連線」在 AMEGO 稍慢時固定
//     失敗，而賣家看到的訊息會說成他的 AppKey 有問題。
//
// 本 client 只包裝 Gateway 會用到的 RPC。invoice.proto 另有三支純服務間 RPC
// （ResolveOrderInvoiceModeInternal／ValidateOrderInvoiceInputInternal／
// GetSellerInvoiceCapabilitySimple），呼叫端是 order-service 與結帳鏈，不經 Gateway，
// 故刻意不在此暴露——包進來會造出一組沒有 HTTP 入口、也沒有 Go 呼叫端的死方法。
type InvoiceServiceClient struct {
	conn   *grpc.ClientConn
	client pb.InvoiceServiceClient
}

// invoiceDeadlines 由 env 決定，boot 時讀一次；值域與預設見 §12.1.2。
var (
	invoiceDeadline               = envDurationMs("INVOICE_GRPC_DEADLINE_MS", 3000)
	invoiceCredentialTestDeadline = envDurationMs("INVOICE_GRPC_DEADLINE_CREDENTIAL_TEST_MS", 15000)
)

func envDurationMs(key string, defMs int) time.Duration {
	raw := config.GetEnvOrDefault(key, "")
	if raw == "" {
		return time.Duration(defMs) * time.Millisecond
	}
	var ms int
	if _, err := fmt.Sscanf(raw, "%d", &ms); err != nil || ms <= 0 {
		zap.S().Warnf("invalid %s=%q, falling back to %dms", key, raw, defMs)
		return time.Duration(defMs) * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// NewInvoiceServiceClient 建立 invoice gRPC 用戶端。
func NewInvoiceServiceClient() (*InvoiceServiceClient, error) {
	host := getEnvOrDefault("INVOICE_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("INVOICE_SERVICE_GRPC_PORT", "10041")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to invoice service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to invoice gRPC service at %s", target)

	return &InvoiceServiceClient{
		conn:   conn,
		client: pb.NewInvoiceServiceClient(conn),
	}, nil
}

// Close 關閉連線。
func (c *InvoiceServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== 買家端 ====================

// ListBuyerInvoiceProfiles （端點 1）
func (c *InvoiceServiceClient) ListBuyerInvoiceProfiles(ctx context.Context, req *pb.ListBuyerInvoiceProfilesRequest) (*pb.ListBuyerInvoiceProfilesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListBuyerInvoiceProfiles(ctx, req)
}

// CreateBuyerInvoiceProfile （端點 2）
func (c *InvoiceServiceClient) CreateBuyerInvoiceProfile(ctx context.Context, req *pb.CreateBuyerInvoiceProfileRequest) (*pb.BuyerInvoiceProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.CreateBuyerInvoiceProfile(ctx, req)
}

// UpdateBuyerInvoiceProfile （端點 3）
func (c *InvoiceServiceClient) UpdateBuyerInvoiceProfile(ctx context.Context, req *pb.UpdateBuyerInvoiceProfileRequest) (*pb.BuyerInvoiceProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.UpdateBuyerInvoiceProfile(ctx, req)
}

// DeleteBuyerInvoiceProfile （端點 4）
func (c *InvoiceServiceClient) DeleteBuyerInvoiceProfile(ctx context.Context, req *pb.DeleteBuyerInvoiceProfileRequest) (*pb.DeleteBuyerInvoiceProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.DeleteBuyerInvoiceProfile(ctx, req)
}

// SetDefaultBuyerInvoiceProfile （端點 5）
func (c *InvoiceServiceClient) SetDefaultBuyerInvoiceProfile(ctx context.Context, req *pb.SetDefaultBuyerInvoiceProfileRequest) (*pb.BuyerInvoiceProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.SetDefaultBuyerInvoiceProfile(ctx, req)
}

// GetOrderInvoice （端點 8）
func (c *InvoiceServiceClient) GetOrderInvoice(ctx context.Context, req *pb.GetOrderInvoiceRequest) (*pb.GetOrderInvoiceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetOrderInvoice(ctx, req)
}

// GetInvoiceDocument （端點 9）
func (c *InvoiceServiceClient) GetInvoiceDocument(ctx context.Context, req *pb.GetInvoiceDocumentRequest) (*pb.GetInvoiceDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceDocument(ctx, req)
}

// GetInvoiceDocumentFile （端點 10）
func (c *InvoiceServiceClient) GetInvoiceDocumentFile(ctx context.Context, req *pb.GetInvoiceDocumentFileRequest) (*pb.GetInvoiceDocumentFileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceDocumentFile(ctx, req)
}

// ListBuyerInvoiceDocuments （端點 68）
func (c *InvoiceServiceClient) ListBuyerInvoiceDocuments(ctx context.Context, req *pb.ListBuyerInvoiceDocumentsRequest) (*pb.ListBuyerInvoiceDocumentsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListBuyerInvoiceDocuments(ctx, req)
}

// GetBuyerInvoiceDocument （端點 69）
func (c *InvoiceServiceClient) GetBuyerInvoiceDocument(ctx context.Context, req *pb.GetBuyerInvoiceDocumentRequest) (*pb.GetInvoiceDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetBuyerInvoiceDocument(ctx, req)
}

// ResendInvoiceNotification （端點 32，賣家）
func (c *InvoiceServiceClient) ResendInvoiceNotification(ctx context.Context, req *pb.ResendInvoiceNotificationRequest) (*pb.ResendInvoiceNotificationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ResendInvoiceNotification(ctx, req)
}

// ResendBuyerInvoiceNotification （端點 71，買家）。與賣家版分開是 §5.1 角色分離的
// 硬要求：兩者的擁有者述詞不同（store_id vs buyer_id），而 §8.1.4 的 Tier A 要求
// 擁有者必須進服務端的 WHERE。
func (c *InvoiceServiceClient) ResendBuyerInvoiceNotification(ctx context.Context, req *pb.ResendBuyerInvoiceNotificationRequest) (*pb.ResendInvoiceNotificationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ResendBuyerInvoiceNotification(ctx, req)
}

// CreateInvoiceCorrection （端點 11）
func (c *InvoiceServiceClient) CreateInvoiceCorrection(ctx context.Context, req *pb.CreateInvoiceCorrectionRequest) (*pb.InvoiceCorrectionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.CreateInvoiceCorrection(ctx, req)
}

// GetInvoiceCorrection （端點 12）
func (c *InvoiceServiceClient) GetInvoiceCorrection(ctx context.Context, req *pb.GetInvoiceCorrectionRequest) (*pb.InvoiceCorrectionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceCorrection(ctx, req)
}

// CreateInvoiceComplaint （端點 13）
func (c *InvoiceServiceClient) CreateInvoiceComplaint(ctx context.Context, req *pb.CreateInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.CreateInvoiceComplaint(ctx, req)
}

// ListOrderInvoiceComplaints （端點 14）
func (c *InvoiceServiceClient) ListOrderInvoiceComplaints(ctx context.Context, req *pb.ListOrderInvoiceComplaintsRequest) (*pb.ListInvoiceComplaintsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListOrderInvoiceComplaints(ctx, req)
}

// GetInvoiceComplaint （端點 15）
func (c *InvoiceServiceClient) GetInvoiceComplaint(ctx context.Context, req *pb.GetInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceComplaint(ctx, req)
}

// ==================== 賣家端 ====================

// GetSellerInvoiceOverview （端點 16）
func (c *InvoiceServiceClient) GetSellerInvoiceOverview(ctx context.Context, req *pb.GetSellerInvoiceOverviewRequest) (*pb.GetSellerInvoiceOverviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerInvoiceOverview(ctx, req)
}

// GetSellerTaxProfile （端點 17）
func (c *InvoiceServiceClient) GetSellerTaxProfile(ctx context.Context, req *pb.GetSellerTaxProfileRequest) (*pb.SellerTaxProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerTaxProfile(ctx, req)
}

// SaveSellerTaxProfileDraft （端點 18）
func (c *InvoiceServiceClient) SaveSellerTaxProfileDraft(ctx context.Context, req *pb.SaveSellerTaxProfileDraftRequest) (*pb.SellerTaxProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.SaveSellerTaxProfileDraft(ctx, req)
}

// SubmitSellerTaxProfile （端點 19）
func (c *InvoiceServiceClient) SubmitSellerTaxProfile(ctx context.Context, req *pb.SubmitSellerTaxProfileRequest) (*pb.SellerTaxProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.SubmitSellerTaxProfile(ctx, req)
}

// RequestSellerTaxExtension （端點 20）
func (c *InvoiceServiceClient) RequestSellerTaxExtension(ctx context.Context, req *pb.RequestSellerTaxExtensionRequest) (*pb.SellerTaxProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RequestSellerTaxExtension(ctx, req)
}

// GetSelfIssueAttestation （端點 21）
func (c *InvoiceServiceClient) GetSelfIssueAttestation(ctx context.Context, req *pb.GetSelfIssueAttestationRequest) (*pb.SelfIssueAttestationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSelfIssueAttestation(ctx, req)
}

// AcceptSelfIssueAttestation （端點 22）
func (c *InvoiceServiceClient) AcceptSelfIssueAttestation(ctx context.Context, req *pb.AcceptSelfIssueAttestationRequest) (*pb.SelfIssueAttestationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.AcceptSelfIssueAttestation(ctx, req)
}

// GetSellerProviderAccount （端點 23）
func (c *InvoiceServiceClient) GetSellerProviderAccount(ctx context.Context, req *pb.GetSellerProviderAccountRequest) (*pb.SellerProviderAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerProviderAccount(ctx, req)
}

// PutSellerProviderCredentials （端點 24）
func (c *InvoiceServiceClient) PutSellerProviderCredentials(ctx context.Context, req *pb.PutSellerProviderCredentialsRequest) (*pb.SellerProviderAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.PutSellerProviderCredentials(ctx, req)
}

// VerifySellerProviderAccount （端點 25；連線測試 deadline）
func (c *InvoiceServiceClient) VerifySellerProviderAccount(ctx context.Context, req *pb.VerifySellerProviderAccountRequest) (*pb.SellerProviderAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceCredentialTestDeadline)
	defer cancel()
	return c.client.VerifySellerProviderAccount(ctx, req)
}

// ActivateSellerProviderAccount （端點 26）
func (c *InvoiceServiceClient) ActivateSellerProviderAccount(ctx context.Context, req *pb.ActivateSellerProviderAccountRequest) (*pb.SellerProviderAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ActivateSellerProviderAccount(ctx, req)
}

// RotateSellerProviderCredentials （端點 27）
func (c *InvoiceServiceClient) RotateSellerProviderCredentials(ctx context.Context, req *pb.RotateSellerProviderCredentialsRequest) (*pb.SellerProviderAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RotateSellerProviderCredentials(ctx, req)
}

// GetSellerReceiptAuthorization （端點 28）
func (c *InvoiceServiceClient) GetSellerReceiptAuthorization(ctx context.Context, req *pb.GetSellerReceiptAuthorizationRequest) (*pb.SellerReceiptAuthorizationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerReceiptAuthorization(ctx, req)
}

// ConfirmSellerReceiptAuthorization （端點 29）
func (c *InvoiceServiceClient) ConfirmSellerReceiptAuthorization(ctx context.Context, req *pb.ConfirmSellerReceiptAuthorizationRequest) (*pb.SellerReceiptAuthorizationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ConfirmSellerReceiptAuthorization(ctx, req)
}

// ListSellerInvoiceDocuments （端點 30）
func (c *InvoiceServiceClient) ListSellerInvoiceDocuments(ctx context.Context, req *pb.ListSellerInvoiceDocumentsRequest) (*pb.ListSellerInvoiceDocumentsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListSellerInvoiceDocuments(ctx, req)
}

// GetSellerInvoiceDocument （端點 31）
func (c *InvoiceServiceClient) GetSellerInvoiceDocument(ctx context.Context, req *pb.GetSellerInvoiceDocumentRequest) (*pb.GetSellerInvoiceDocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerInvoiceDocument(ctx, req)
}

// ListSellerReceivedInvoiceDocuments （端點 72）
func (c *InvoiceServiceClient) ListSellerReceivedInvoiceDocuments(ctx context.Context, req *pb.ListSellerReceivedInvoiceDocumentsRequest) (*pb.ListSellerReceivedInvoiceDocumentsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListSellerReceivedInvoiceDocuments(ctx, req)
}

// GetSellerInvoiceJob （端點 33）
func (c *InvoiceServiceClient) GetSellerInvoiceJob(ctx context.Context, req *pb.GetSellerInvoiceJobRequest) (*pb.GetSellerInvoiceJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerInvoiceJob(ctx, req)
}

// ListSellerInvoiceComplaints （端點 34）
func (c *InvoiceServiceClient) ListSellerInvoiceComplaints(ctx context.Context, req *pb.ListSellerInvoiceComplaintsRequest) (*pb.ListInvoiceComplaintsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListSellerInvoiceComplaints(ctx, req)
}

// GetSellerInvoiceComplaint （端點 35）
func (c *InvoiceServiceClient) GetSellerInvoiceComplaint(ctx context.Context, req *pb.GetSellerInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetSellerInvoiceComplaint(ctx, req)
}

// RespondSellerInvoiceComplaint （端點 36）
func (c *InvoiceServiceClient) RespondSellerInvoiceComplaint(ctx context.Context, req *pb.RespondSellerInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RespondSellerInvoiceComplaint(ctx, req)
}

// ==================== ADM ====================

// GetInvoiceOverviewForAdmin （端點 37）
func (c *InvoiceServiceClient) GetInvoiceOverviewForAdmin(ctx context.Context, req *pb.GetInvoiceOverviewForAdminRequest) (*pb.GetInvoiceOverviewForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceOverviewForAdmin(ctx, req)
}

// ListTaxReviewsForAdmin （端點 38）
func (c *InvoiceServiceClient) ListTaxReviewsForAdmin(ctx context.Context, req *pb.ListTaxReviewsForAdminRequest) (*pb.ListTaxReviewsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListTaxReviewsForAdmin(ctx, req)
}

// GetTaxReviewForAdmin （端點 39）
func (c *InvoiceServiceClient) GetTaxReviewForAdmin(ctx context.Context, req *pb.GetTaxReviewForAdminRequest) (*pb.GetTaxReviewForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetTaxReviewForAdmin(ctx, req)
}

// DecideTaxReviewForAdmin （端點 40）
func (c *InvoiceServiceClient) DecideTaxReviewForAdmin(ctx context.Context, req *pb.DecideTaxReviewForAdminRequest) (*pb.DecideTaxReviewForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.DecideTaxReviewForAdmin(ctx, req)
}

// DecideTaxExtensionForAdmin （端點 41）
func (c *InvoiceServiceClient) DecideTaxExtensionForAdmin(ctx context.Context, req *pb.DecideTaxExtensionForAdminRequest) (*pb.DecideTaxReviewForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.DecideTaxExtensionForAdmin(ctx, req)
}

// ListProviderAccountsForAdmin （端點 42）
func (c *InvoiceServiceClient) ListProviderAccountsForAdmin(ctx context.Context, req *pb.ListProviderAccountsForAdminRequest) (*pb.ListProviderAccountsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListProviderAccountsForAdmin(ctx, req)
}

// GetProviderAccountForAdmin （端點 43）
func (c *InvoiceServiceClient) GetProviderAccountForAdmin(ctx context.Context, req *pb.GetProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetProviderAccountForAdmin(ctx, req)
}

// HealthCheckProviderAccountForAdmin （端點 44；連線測試 deadline）
func (c *InvoiceServiceClient) HealthCheckProviderAccountForAdmin(ctx context.Context, req *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceCredentialTestDeadline)
	defer cancel()
	return c.client.HealthCheckProviderAccountForAdmin(ctx, req)
}

// DeactivateProviderAccountForAdmin （端點 45）
func (c *InvoiceServiceClient) DeactivateProviderAccountForAdmin(ctx context.Context, req *pb.DeactivateProviderAccountForAdminRequest) (*pb.DeactivateProviderAccountForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.DeactivateProviderAccountForAdmin(ctx, req)
}

// ListProviderCutoversForAdmin （端點 46）
func (c *InvoiceServiceClient) ListProviderCutoversForAdmin(ctx context.Context, req *pb.ListProviderCutoversForAdminRequest) (*pb.ListProviderCutoversForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListProviderCutoversForAdmin(ctx, req)
}

// ListInvoiceDocumentsForAdmin （端點 47）
func (c *InvoiceServiceClient) ListInvoiceDocumentsForAdmin(ctx context.Context, req *pb.ListInvoiceDocumentsForAdminRequest) (*pb.ListInvoiceDocumentsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListInvoiceDocumentsForAdmin(ctx, req)
}

// GetInvoiceDocumentForAdmin （端點 48）
func (c *InvoiceServiceClient) GetInvoiceDocumentForAdmin(ctx context.Context, req *pb.GetInvoiceDocumentForAdminRequest) (*pb.GetInvoiceDocumentForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceDocumentForAdmin(ctx, req)
}

// ResendInvoiceNotificationForAdmin （端點 49）
func (c *InvoiceServiceClient) ResendInvoiceNotificationForAdmin(ctx context.Context, req *pb.ResendInvoiceNotificationForAdminRequest) (*pb.ResendInvoiceNotificationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ResendInvoiceNotificationForAdmin(ctx, req)
}

// ListInvoiceJobsForAdmin （端點 50）
func (c *InvoiceServiceClient) ListInvoiceJobsForAdmin(ctx context.Context, req *pb.ListInvoiceJobsForAdminRequest) (*pb.ListInvoiceJobsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListInvoiceJobsForAdmin(ctx, req)
}

// GetInvoiceJobForAdmin （端點 51）
func (c *InvoiceServiceClient) GetInvoiceJobForAdmin(ctx context.Context, req *pb.GetInvoiceJobForAdminRequest) (*pb.GetInvoiceJobForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceJobForAdmin(ctx, req)
}

// RetryInvoiceJobForAdmin （端點 52）
func (c *InvoiceServiceClient) RetryInvoiceJobForAdmin(ctx context.Context, req *pb.RetryInvoiceJobForAdminRequest) (*pb.InvoiceJobMutationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RetryInvoiceJobForAdmin(ctx, req)
}

// ResumeInvoiceDocumentForAdmin （端點 73）
func (c *InvoiceServiceClient) ResumeInvoiceDocumentForAdmin(ctx context.Context, req *pb.ResumeInvoiceDocumentForAdminRequest) (*pb.InvoiceJobMutationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ResumeInvoiceDocumentForAdmin(ctx, req)
}

// CancelInvoiceJobForAdmin （端點 53）
func (c *InvoiceServiceClient) CancelInvoiceJobForAdmin(ctx context.Context, req *pb.CancelInvoiceJobForAdminRequest) (*pb.InvoiceJobMutationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.CancelInvoiceJobForAdmin(ctx, req)
}

// RequestManualUnlockForAdmin （端點 54）
func (c *InvoiceServiceClient) RequestManualUnlockForAdmin(ctx context.Context, req *pb.RequestManualUnlockForAdminRequest) (*pb.ManualUnlockResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RequestManualUnlockForAdmin(ctx, req)
}

// ApproveManualUnlockForAdmin （端點 55）
func (c *InvoiceServiceClient) ApproveManualUnlockForAdmin(ctx context.Context, req *pb.ApproveManualUnlockForAdminRequest) (*pb.ManualUnlockResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ApproveManualUnlockForAdmin(ctx, req)
}

// ReportManualResultForAdmin （端點 56）
func (c *InvoiceServiceClient) ReportManualResultForAdmin(ctx context.Context, req *pb.ReportManualResultForAdminRequest) (*pb.InvoiceJobMutationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ReportManualResultForAdmin(ctx, req)
}

// CreateManualIssuanceForAdmin （端點 57）
func (c *InvoiceServiceClient) CreateManualIssuanceForAdmin(ctx context.Context, req *pb.CreateManualIssuanceForAdminRequest) (*pb.CreateManualIssuanceForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.CreateManualIssuanceForAdmin(ctx, req)
}

// ListInvoiceComplaintsForAdmin （端點 58）
func (c *InvoiceServiceClient) ListInvoiceComplaintsForAdmin(ctx context.Context, req *pb.ListInvoiceComplaintsForAdminRequest) (*pb.ListInvoiceComplaintsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListInvoiceComplaintsForAdmin(ctx, req)
}

// GetInvoiceComplaintForAdmin （端點 59）
func (c *InvoiceServiceClient) GetInvoiceComplaintForAdmin(ctx context.Context, req *pb.GetInvoiceComplaintForAdminRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.GetInvoiceComplaintForAdmin(ctx, req)
}

// ActOnInvoiceComplaintForAdmin （端點 60）
func (c *InvoiceServiceClient) ActOnInvoiceComplaintForAdmin(ctx context.Context, req *pb.ActOnInvoiceComplaintForAdminRequest) (*pb.ActOnInvoiceComplaintForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ActOnInvoiceComplaintForAdmin(ctx, req)
}

// ListInvoiceAuditLogsForAdmin （端點 61）
func (c *InvoiceServiceClient) ListInvoiceAuditLogsForAdmin(ctx context.Context, req *pb.ListInvoiceAuditLogsForAdminRequest) (*pb.ListInvoiceAuditLogsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListInvoiceAuditLogsForAdmin(ctx, req)
}

// ==================== 賣家端 ====================

// ReportExternalIssuance （端點 62）
func (c *InvoiceServiceClient) ReportExternalIssuance(ctx context.Context, req *pb.ReportExternalIssuanceRequest) (*pb.GetSellerInvoiceJobResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ReportExternalIssuance(ctx, req)
}

// ==================== 買家端 ====================

// RejectInvoiceComplaintResponse （端點 63）
func (c *InvoiceServiceClient) RejectInvoiceComplaintResponse(ctx context.Context, req *pb.RejectInvoiceComplaintResponseRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RejectInvoiceComplaintResponse(ctx, req)
}

// WithdrawInvoiceComplaint （端點 64）
func (c *InvoiceServiceClient) WithdrawInvoiceComplaint(ctx context.Context, req *pb.WithdrawInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.WithdrawInvoiceComplaint(ctx, req)
}

// ReopenInvoiceComplaint （端點 65）
func (c *InvoiceServiceClient) ReopenInvoiceComplaint(ctx context.Context, req *pb.ReopenInvoiceComplaintRequest) (*pb.InvoiceComplaintResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ReopenInvoiceComplaint(ctx, req)
}

// ==================== ADM ====================

// ReactivateProviderAccountForAdmin （端點 66；連線測試 deadline）
func (c *InvoiceServiceClient) ReactivateProviderAccountForAdmin(ctx context.Context, req *pb.ReactivateProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceCredentialTestDeadline)
	defer cancel()
	return c.client.ReactivateProviderAccountForAdmin(ctx, req)
}

// RevokeProviderAccountForAdmin （端點 67）
func (c *InvoiceServiceClient) RevokeProviderAccountForAdmin(ctx context.Context, req *pb.RevokeProviderAccountForAdminRequest) (*pb.RevokeProviderAccountForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.RevokeProviderAccountForAdmin(ctx, req)
}

// ==================== 賣家端 ====================

// ListOrderExternalIssuances （端點 62R）
func (c *InvoiceServiceClient) ListOrderExternalIssuances(ctx context.Context, req *pb.ListOrderExternalIssuancesRequest) (*pb.ListOrderExternalIssuancesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, invoiceDeadline)
	defer cancel()
	return c.client.ListOrderExternalIssuances(ctx, req)
}
