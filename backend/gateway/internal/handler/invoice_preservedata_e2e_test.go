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

// 58 號《電子發票》§6.1.0(13) preserveData 業務碼的端到端斷言。
//
// 為什麼要走 bufconn ＋ 真 EnvelopeGuardInterceptor，而不是注入 mock proto client：
// preserveData 這個概念**只存在於 interceptor 裡**（envelopeCodePreserveData 命中就
// `return nil`，非零 envelope code 於是以「err == nil ＋ resp.Code != 0」的形狀抵達
// handler）。注入 mock 的話**每個**非零碼都長這樣，preserveData 與非 preserveData
// 的差別整個消失 —— 那種測試無論 handler 有沒有檢查 resp.GetCode() 都會綠，
// 正是本檔要防的失效模式（同 testing.go 對 NewAccountClientForTest 的警語、#2050）。
//
// 覆蓋四件事：
//
//	① preserveData 碼（9331）不得被寫成 code=0    —— 已坐實缺陷的迴歸鎖
//	② 非 preserveData 碼仍要走 error 路徑          —— 證明守門沒有把錯誤吞掉
//	③ code=0 的成功形狀逐欄未變                    —— 證明「行為零改變」不是推理
//	④ 分頁端點改寫後 data.{records,total,page,pageSize} 未變
type fakeInvoiceServer struct {
	pb.UnimplementedInvoiceServiceServer
	code    int32
	message string
	data    *pb.SellerProviderAccountData
	list    *pb.ListProviderAccountsForAdminData
}

func (s *fakeInvoiceServer) HealthCheckProviderAccountForAdmin(_ context.Context, _ *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	// 鏡射 invoice-service 的 provider_account.go：gRPC OK ＋ envelope 非零碼 ＋ data 仍在。
	return &pb.GetProviderAccountForAdminResponse{Code: s.code, Message: s.message, Data: s.data}, nil
}

func (s *fakeInvoiceServer) ReactivateProviderAccountForAdmin(_ context.Context, _ *pb.ReactivateProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{Code: s.code, Message: s.message, Data: s.data}, nil
}

func (s *fakeInvoiceServer) ListProviderAccountsForAdmin(_ context.Context, _ *pb.ListProviderAccountsForAdminRequest) (*pb.ListProviderAccountsForAdminResponse, error) {
	return &pb.ListProviderAccountsForAdminResponse{Code: s.code, Message: s.message, Data: s.list}, nil
}

// newInvoiceAdminRouter 起 bufconn fake invoice server，掛上 production 的
// EnvelopeGuardInterceptor，回傳一個把 accountId 預先塞好的 gin router
// （授權由 requireAdmin 承擔，與本檔主張無關，故不掛）。
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
	// bizcarrier.Middleware 是 production 全域鏈的一員（infrastructure/di.go 的
	// newEngine），preserveData 業務碼的「不轉 error」特權由它承載：
	// EnvelopeGuardInterceptor 只在 Arm 成功時 return nil，Arm 拿不到 carrier 就
	// 撤銷特權、改走一般錯誤映射（碼還在、data 掉了）。
	//
	// ⚠️ 拿掉這一行，本檔的 9331 兩支會變成 HTTP 409 ＋ 無 data —— 那不是測試壞了，
	// 那正是 fail-closed 契約本身：沒有 carrier 就沒有 preserveData。
	r.Use(bizcarrier.Middleware())
	r.Use(func(c *gin.Context) { c.Set("accountId", int64(4242)); c.Next() })
	r.POST("/admin/invoices/provider_accounts/:store_id/health_checks", h.HealthCheckProviderAccountForAdmin)
	r.POST("/admin/invoices/provider_accounts/:store_id/reactivations", h.ReactivateProviderAccountForAdmin)
	r.GET("/admin/invoices/provider_accounts", h.ListProviderAccountsForAdmin)
	return r
}

func doInvoiceReq(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	// 帶一份合法 JSON body：端點 66 之類會先 ShouldBindJSON，空 body 會在打到
	// invoice client 之前就 400，那樣測到的是 binding 不是 envelope（探權限要送
	// 合法 payload 的同一條紀律）。
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

	// preserveData 碼一律 HTTP 200（業務碼不走 HTTP status 分流）。
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
	// data.providerRequestId 是 ADM 去光貿對帳的唯一線索（§6.1.8）。
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
