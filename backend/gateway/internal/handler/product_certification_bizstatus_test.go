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

// 商品認證五個業務碼的 HTTP status 分流，**走 production 逐位元相同的那條路**（#2755）。
//
// ⚠️ 這支測試存在的理由是 TestCertificationEndpoints_BusinessCodeMapsToHTTPStatus
// 全綠而 dev 上 3025／3028 雙雙回 409。那支用 grpcclient.NewProductClientForTest(mock)
// 注入 mock proto client，那條路**沒有 *grpc.ClientConn**，於是
// EnvelopeGuardInterceptor 根本不會跑，mock 回的 {Code: 3025} 原樣抵達 handler 並走進
// `resp.GetCode() != 0` 那條分支 —— 而 production 走不到那裡：interceptor 會把
// 「gRPC OK ＋ 非零 envelope code」轉成 gRPC error（未登錄於 envelopeCodeToGRPC 的碼
// 一律 FailedPrecondition ＋ "[NNNN] " 前綴），業務碼是從 err 那一側回來的。
//
// 所以這裡起一台 bufconn 上的假 ProductService，掛真的 EnvelopeGuardInterceptor。
// 每個案例同時斷言 HTTP status **與** envelope code：只斷言 status 的話，「把所有錯誤
// 都回 404」也會綠。

type certBizStatusServer struct {
	productpb.UnimplementedProductServiceServer
	// code 非零時回 gRPC OK ＋ 該 envelope code，鏡射 product-service 的
	// `catch (ProductBusinessException e) { onNext(...setCode(e.code())) }`：
	// 業務拒絕從來不是 gRPC error。
	code int32
	msg  string
	// transportErr 非 nil 時直接回它，鏡射真正問不到答案的那一側。
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

// newCertBizStatusRouter 起 bufconn 假 server，並以掛真 EnvelopeGuardInterceptor 的
// 連線建 ProductClient —— 那正是 production DefaultDialOpts 中 envelope guard 的位置。
// 其餘 auth／breaker interceptor 與本主張無關故不掛（breaker 的 registry 以 cc.Target()
// 為鍵，掛上去會讓幾個 subtest 共用同一顆計數器）。
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
			// 這一條就是 dev 上實測回 409 的那個案例。
			name: "3025 認證申請不存在 → 404", code: bizcode.CertApplicationNotFound, msg: "認證申請不存在",
			method: http.MethodGet, path: adminDetail, accountID: "2701338845",
			wantStatus: http.StatusNotFound,
		},
		{
			// 這一條同上：駁回少帶 reasonCode 是請求本身寫錯了，不是版本衝突。
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
			// 賣家側同樣要分流：非本人商品／不存在 → 404，而不是一個空列表也不是 409。
			name: "賣家列表 3025 → 404", code: bizcode.CertApplicationNotFound, msg: "認證申請不存在",
			method: http.MethodGet, path: sellerList, accountID: "777",
			wantStatus: http.StatusNotFound,
		},
		{
			// 鑑別力控制組：未登錄的鄰居碼仍須塌縮 409。少了它，「把 FailedPrecondition
			// 整條改成 404」也會讓上面兩條綠。
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
			// status 對了還不夠：業務碼必須原樣送到前端，否則前端分不出同一個 status
			// 底下的不同原因（也會讓「所有錯誤都回 404」這種假修復通過）。
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
