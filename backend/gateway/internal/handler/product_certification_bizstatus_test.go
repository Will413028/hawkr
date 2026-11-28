package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type certBizStatusServer struct {
	productpb.UnimplementedProductServiceServer
	code         int32
	msg          string
	transportErr error
}

func (s *certBizStatusServer) ListProductCertificationApplications(_ context.Context, _ *productpb.ListProductCertificationApplicationsRequest) (*productpb.ListProductCertificationApplicationsResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &productpb.ListProductCertificationApplicationsResponse{Code: s.code, Message: s.msg}, nil
}

func (s *certBizStatusServer) WithdrawProductCertification(_ context.Context, _ *productpb.WithdrawProductCertificationRequest) (*productpb.ProductCertificationApplicationResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &productpb.ProductCertificationApplicationResponse{Code: s.code, Message: s.msg}, nil
}

func (s *certBizStatusServer) GetProductCertificationApplicationForAdmin(_ context.Context, _ *productpb.GetProductCertificationApplicationForAdminRequest) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Message: s.msg}, nil
}

func (s *certBizStatusServer) ApproveProductCertificationForAdmin(_ context.Context, _ *productpb.ReviewProductCertificationForAdminRequest) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Message: s.msg}, nil
}

func (s *certBizStatusServer) RejectProductCertificationForAdmin(_ context.Context, _ *productpb.ReviewProductCertificationForAdminRequest) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Message: s.msg}, nil
}

func newCertBizStatusRouter(t *testing.T, srv productpb.ProductServiceServer) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	productpb.RegisterProductServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		grpcSrv.Stop()
	})

	h := &Holder{
		Product: grpcclient.NewProductClientForTest(productpb.NewProductServiceClient(conn)),
		Store:   storeResolverForTest(2685914682),
	}

	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.GET("/products/:id/certification_applications", h.ListSellerProductCertificationApplications)
	app.POST("/products/:id/certification_applications/:application_id/withdraw", h.WithdrawSellerProductCertification)

	admin := r.Group("/api/v1/admin")
	admin.Use(fakeAdminAuth())
	admin.GET("/product_certification_applications/:id", h.GetAdminProductCertificationApplication)
	admin.POST("/product_certification_applications/:id/approve", h.ApproveAdminProductCertification)
	admin.POST("/product_certification_applications/:id/reject", h.RejectAdminProductCertification)
	return r
}

func certEnvelopeCode(t *testing.T, w *httptest.ResponseRecorder) int32 {
	t.Helper()
	var env struct {
		Code int32 `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v（body=%s）", err, w.Body.String())
	}
	return env.Code
}

func TestCertificationEndpoints_BizCodeStatusThroughEnvelopeGuard(t *testing.T) {
	const (
		adminDetail  = "/api/v1/admin/product_certification_applications/843856384063471761"
		adminApprove = adminDetail + "/approve"
		adminReject  = adminDetail + "/reject"
		sellerList   = "/api/v1/app/products/843856384063471755/certification_applications"
		sellerWd     = sellerList + "/843856384063471761/withdraw"
	)
	reviewBody := `{"idempotencyKey":"k","expectedVersion":"2","reasonCode":"CERT_NOT_FOUND"}`

	cases := []struct {
		name       string
		code       int32
		msg        string
		method     string
		path       string
		body       string
		accountID  string
		wantStatus int
	}{
		{
			name: "3025 認證申請不存在 → 404", code: bizcode.CertApplicationNotFound, msg: "認證申請不存在",
			method: http.MethodGet, path: adminDetail, accountID: "2701338845",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "3028 缺駁回原因 → 400", code: bizcode.CertReviewReasonRequired, msg: "請填寫駁回原因",
			method: http.MethodPost, path: adminReject, body: reviewBody, accountID: "2701338845",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "3026 版本衝突 → 409", code: bizcode.CertApplicationVersionConflict, msg: "版本已變更",
			method: http.MethodPost, path: adminApprove, body: reviewBody, accountID: "2701338845",
			wantStatus: http.StatusConflict,
		},
		{
			name: "3027 狀態不允許 → 409", code: bizcode.CertApplicationStateInvalid, msg: "狀態不允許此操作",
			method: http.MethodPost, path: sellerWd, body: `{"expectedVersion":"2","idempotencyKey":"k"}`, accountID: "777",
			wantStatus: http.StatusConflict,
		},
		{
			name: "賣家列表 3025 → 404", code: bizcode.CertApplicationNotFound, msg: "認證申請不存在",
			method: http.MethodGet, path: sellerList, accountID: "777",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "未登錄的 3024 仍塌縮 409", code: 3024, msg: "其他認證錯誤",
			method: http.MethodGet, path: adminDetail, accountID: "2701338845",
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newCertBizStatusRouter(t, &certBizStatusServer{code: tc.code, msg: tc.msg})
			w := certCall(t, r, tc.method, tc.path, tc.body, tc.accountID)
			if w.Code != tc.wantStatus {
				t.Errorf("HTTP %d, want %d（body=%s）", w.Code, tc.wantStatus, w.Body.String())
			}
			if got := certEnvelopeCode(t, w); got != tc.code {
				t.Errorf("envelope code = %d, want %d（body=%s）", got, tc.code, w.Body.String())
			}
		})
	}
}

// 傳輸層失敗不得被登錄表吃掉：product-service 真的不可用時仍是 503，而不是某個
// 業務碼的 status。少了這條，「把所有 err 都當成業務碼」也會讓上面全綠。
func TestCertificationEndpoints_TransportFailureStaysUnavailable(t *testing.T) {
	r := newCertBizStatusRouter(t, &certBizStatusServer{
		transportErr: grpcstatus.Error(grpccodes.Unavailable, "product service down"),
	})
	w := certCall(t, r, http.MethodGet,
		"/api/v1/admin/product_certification_applications/843856384063471761", "", "2701338845")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("HTTP %d, want 503（body=%s）", w.Code, w.Body.String())
	}
}
