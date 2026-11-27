package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeInvoiceServer struct {
	pb.UnimplementedInvoiceServiceServer
	code    int32
	message string
	data    *pb.SellerProviderAccountData
	list    *pb.ListProviderAccountsForAdminData
}

func (s *fakeInvoiceServer) HealthCheckProviderAccountForAdmin(_ context.Context, _ *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{Code: s.code, Message: s.message, Data: s.data}, nil
}

func (s *fakeInvoiceServer) ReactivateProviderAccountForAdmin(_ context.Context, _ *pb.ReactivateProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{Code: s.code, Message: s.message, Data: s.data}, nil
}

func (s *fakeInvoiceServer) ListProviderAccountsForAdmin(_ context.Context, _ *pb.ListProviderAccountsForAdminRequest) (*pb.ListProviderAccountsForAdminResponse, error) {
	return &pb.ListProviderAccountsForAdminResponse{Code: s.code, Message: s.message, Data: s.list}, nil
}

func newInvoiceAdminRouter(t *testing.T, srv pb.InvoiceServiceServer) http.Handler {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	pb.RegisterInvoiceServiceServer(grpcSrv, srv)
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

	h := &Holder{Invoice: grpcclient.NewInvoiceServiceClientForTest(pb.NewInvoiceServiceClient(conn))}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	r.Use(func(c *gin.Context) { c.Set("accountId", int64(4242)); c.Next() })
	r.POST("/admin/invoices/provider_accounts/:store_id/health_checks", h.HealthCheckProviderAccountForAdmin)
	r.POST("/admin/invoices/provider_accounts/:store_id/reactivations", h.ReactivateProviderAccountForAdmin)
	r.GET("/admin/invoices/provider_accounts", h.ListProviderAccountsForAdmin)
	return r
}

func doInvoiceReq(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"reason":"e2e","confirmed":true,"expectedVersion":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "e2e-preservedata-0001")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func sampleProviderData() *pb.SellerProviderAccountData {
	return &pb.SellerProviderAccountData{
		Provider:       "AMEGO",
		Status:         "CREDENTIAL_INVALID",
		StatusNotice:   "憑證驗證失敗",
		LastRequestId:  "req-abc-123",
		LastVerifiedAt: "2026-08-31T00:00:00Z",
	}
}

// ① 已坐實缺陷的迴歸鎖：端點 44 遇 9331 必須回 code=9331 ＋ data，HTTP 200。
func TestHealthCheckProviderAccount_9331_NotSwallowedAsSuccess(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 9331, message: "憑證驗證失敗，請確認統一編號與 AppKey", data: sampleProviderData(),
	})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/health_checks")

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 9331)
	if body["message"] != "憑證驗證失敗，請確認統一編號與 AppKey" {
		t.Errorf("message = %v, want the provider failure text", body["message"])
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 必須原樣透傳，得到 %#v", body["data"])
	}
	if data["lastRequestId"] != "req-abc-123" {
		t.Errorf("data.lastRequestId = %v, want req-abc-123", data["lastRequestId"])
	}
	if data["status"] != "CREDENTIAL_INVALID" {
		t.Errorf("data.status = %v, want CREDENTIAL_INVALID", data["status"])
	}
}

// ① 之二：本來就寫對的端點 66 行為未變（同碼同斷言）——證明修復沒有改壞既有正確路徑。
func TestReactivateProviderAccount_9331_UnchangedBehaviour(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 9331, message: "憑證驗證失敗", data: sampleProviderData(),
	})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/reactivations")

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 9331)
	if _, ok := body["data"].(map[string]interface{}); !ok {
		t.Fatalf("data 必須原樣透傳，得到 %#v", body["data"])
	}
}

// ② 非 preserveData 碼必須仍走 error 路徑：9330 登錄為 NotFound ＋ preserveBizCode
//
//	⇒ HTTP 404 ＋ envelope code 9330。守門若把它也吞成 200 就是新的破口。
func TestHealthCheckProviderAccount_9330_StillErrorPath(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 9330, message: "找不到 AMEGO 帳號", data: sampleProviderData(),
	})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/health_checks")

	assertHTTPStatus(t, w, http.StatusNotFound)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 9330)
	if _, present := body["data"]; present {
		t.Errorf("非 preserveData 碼不應帶 data，得到 %#v", body["data"])
	}
}

// ② 之二：未登錄碼（9332 provider 不可用）走 fallback FailedPrecondition ⇒ HTTP 409。
func TestHealthCheckProviderAccount_9332_CollapsesTo409(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 9332, message: "AMEGO 服務暫時不可用", data: sampleProviderData(),
	})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/health_checks")

	assertHTTPStatus(t, w, http.StatusConflict)
	assertCode(t, parseJSON(t, w.Body), 9332)
}

// ③ 成功形狀逐欄未變：code=0 ⇒ {code:0, message:"success", data:{...}}，且無 traceId 之外的多餘欄位。
func TestHealthCheckProviderAccount_Success_ShapeUnchanged(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{code: 0, data: sampleProviderData()})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/health_checks")

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	if body["message"] != "success" {
		t.Errorf("成功回應 message 必須恆為 success，得到 %v", body["message"])
	}
	if _, ok := body["data"].(map[string]interface{}); !ok {
		t.Fatalf("data missing on success: %#v", body["data"])
	}
}

// ④ 分頁端點改寫後成功形狀未變：data.{records,total,page,pageSize}。
//
//	這是「61 支預防性守門」裡唯一動到既有成功出口寫法的一類，必須逐欄釘住。
func TestListProviderAccountsForAdmin_Success_PageShapeUnchanged(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 0,
		list: &pb.ListProviderAccountsForAdminData{
			Records:  []*pb.ProviderAccountItem{{StoreId: "2685914682", StoreName: "測試商店", Status: "CONFIG_ERROR"}},
			Total:    "1",
			Page:     "1",
			PageSize: "10",
		},
	})
	w := doInvoiceReq(t, router, http.MethodGet, "/admin/invoices/provider_accounts")

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data missing: %#v", body["data"])
	}
	for _, k := range []string{"records", "total", "page", "pageSize"} {
		if _, present := data[k]; !present {
			t.Errorf("分頁 data 少了 %q —— 前端讀通用 data.%s 會拿到 undefined", k, k)
		}
	}
	if data["total"] != "1" || data["page"] != "1" || data["pageSize"] != "10" {
		t.Errorf("分頁欄位值改變: %#v", data)
	}
	recs, ok := data["records"].([]interface{})
	if !ok || len(recs) != 1 {
		t.Fatalf("records 形狀改變: %#v", data["records"])
	}
}

// 一併釘住「envelope 只有這些頂層欄位」——業務資料一律在 data 之下（API 設計規範 §2）。
func TestInvoicePreserveData_EnvelopeTopLevelKeys(t *testing.T) {
	router := newInvoiceAdminRouter(t, &fakeInvoiceServer{
		code: 9331, message: "憑證驗證失敗", data: sampleProviderData(),
	})
	w := doInvoiceReq(t, router, http.MethodPost, "/admin/invoices/provider_accounts/2685914682/health_checks")

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	allowed := map[string]bool{"code": true, "message": true, "data": true, "traceId": true, "messageZh": true}
	for k := range raw {
		if !allowed[k] {
			t.Errorf("envelope 出現非契約頂層欄位 %q —— 業務資料必須在 data 之下", k)
		}
	}
}
