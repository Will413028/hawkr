package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 十支認證端點的契約斷言（19 號設計書 §9.1 / §9.2 / §9.3）。
//
// 兩件事在這裡守：
//   ① 賣家 DTO 與 ADM DTO 是**兩個型別**，Admin-only 欄位不是靠執行期清空而是根本
//      不映射。靠清空的話 swagger 宣告會與實際輸出對不上，而且下一個複製這段 code
//      的人不會知道要清。
//   ② reviewer 身分一律取 auth context。由 request body 決定「誰批准的」＝稽核紀錄
//      可被偽造，而那正是這組端點存在的理由。

// certEndpointStub 記下每一支 RPC 收到的 request。
type certEndpointStub struct {
	productpb.ProductServiceClient
	lastList     *productpb.ListProductCertificationApplicationsRequest
	lastSubmit   *productpb.SubmitProductCertificationRequest
	lastWithdraw *productpb.WithdrawProductCertificationRequest
	lastReview   *productpb.ReviewProductCertificationForAdminRequest
	lastReverify *productpb.ReverifyProductCertificationForAdminRequest
	lastGetAdmin *productpb.GetProductCertificationApplicationForAdminRequest
	lastAudit    *productpb.ListProductCertificationAuditLogsForAdminRequest
	// code 非 0 時所有 RPC 回這個業務碼（驗 HTTP status 分流）。
	code int32
}

// fullApplication 是一筆「所有欄位都有值」的申請，用來證明賣家 DTO 真的沒把
// Admin-only 欄位帶出去——欄位是空的時候，漏映射與正確隔離看起來一模一樣。
func fullApplication() *productpb.ProductCertificationApplication {
	return &productpb.ProductCertificationApplication{
		Id:                      "843856384063471761",
		ProductId:               "843856384063471755",
		StoreId:                 "2685914682",
		CertificationType:       "3703",
		CertificateNo:           "010105",
		Status:                  "SUBMITTED",
		Source:                  "SELLER",
		RequiresManualReview:    true,
		GovVerifyState:          "VERIFIED",
		GovVerifiedAt:           "2026-08-26T04:12:09Z",
		SubmittedAt:             "2026-08-26T04:12:05Z",
		ReviewedAt:              "2026-08-26T05:00:00Z",
		ReviewedByAccountId:     "2701338845",
		ReasonCode:              "CERT_NOT_FOUND",
		PublicReason:            "政府資料庫查無此編號",
		Version:                 "2",
		SubmittedByAccountId:    "2685914682",
		ProductName:             "有機小松菜",
		ProductPublicationState: "4014", // 已移除：認證申請結構上會指向已軟刪商品（#2818）
		InternalNote:            "已致電賣家確認",
		GovVerifyPayload: &productpb.GovVerifyResult{
			CertType: "3703", CertNo: "010105", Found: true, Validity: "VALID",
			HolderName: "○○食品股份有限公司", MatchedCount: "173",
			QueriedAt: "2026-08-26T04:12:09Z", RawExcerptJson: `{"trimmed":true}`,
		},
		CurrentApproved: &productpb.ProductCertificationSummary{
			Id: "843856384063471760", CertificationType: "3701",
			CertificateNo: "1-009-300022", Status: "APPROVED",
			GovVerifyState: "VERIFIED", ReviewedAt: "2026-08-26T04:12:07Z",
		},
	}
}

func (s *certEndpointStub) ListProductCertificationApplications(_ context.Context, in *productpb.ListProductCertificationApplicationsRequest, _ ...grpc.CallOption) (*productpb.ListProductCertificationApplicationsResponse, error) {
	s.lastList = in
	return &productpb.ListProductCertificationApplicationsResponse{
		Code: s.code, Records: []*productpb.ProductCertificationApplication{fullApplication()},
		Total: "1", Page: "1", PageSize: "20",
	}, nil
}

func (s *certEndpointStub) SubmitProductCertification(_ context.Context, in *productpb.SubmitProductCertificationRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationResponse, error) {
	s.lastSubmit = in
	return &productpb.ProductCertificationApplicationResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) WithdrawProductCertification(_ context.Context, in *productpb.WithdrawProductCertificationRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationResponse, error) {
	s.lastWithdraw = in
	return &productpb.ProductCertificationApplicationResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) GetProductCertificationApplicationForAdmin(_ context.Context, in *productpb.GetProductCertificationApplicationForAdminRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	s.lastGetAdmin = in
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) ListProductCertificationApplicationsForAdmin(_ context.Context, _ *productpb.ListProductCertificationApplicationsForAdminRequest, _ ...grpc.CallOption) (*productpb.ListProductCertificationApplicationsForAdminResponse, error) {
	return &productpb.ListProductCertificationApplicationsForAdminResponse{
		Code: s.code, Records: []*productpb.ProductCertificationApplication{fullApplication()},
		Total: "1", Page: "1", PageSize: "20",
	}, nil
}

func (s *certEndpointStub) ListProductCertificationAuditLogsForAdmin(_ context.Context, in *productpb.ListProductCertificationAuditLogsForAdminRequest, _ ...grpc.CallOption) (*productpb.ListProductCertificationAuditLogsForAdminResponse, error) {
	s.lastAudit = in
	return &productpb.ListProductCertificationAuditLogsForAdminResponse{
		Code: s.code,
		Records: []*productpb.ProductCertificationReviewEvent{{
			Id: "1", ApplicationId: "843856384063471761", Action: "REJECT",
			FromStatus: "SUBMITTED", ToStatus: "REJECTED", ActorRole: "ADMIN",
			ActorAccountId: "2701338845", CertificationType: "3703", CertificateNo: "010105",
			ApplicationVersion: "2", CreatedAt: "2026-08-26T05:00:00Z",
		}},
		Total: "1", Page: "1", PageSize: "20",
	}, nil
}

func (s *certEndpointStub) ApproveProductCertificationForAdmin(_ context.Context, in *productpb.ReviewProductCertificationForAdminRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	s.lastReview = in
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) RejectProductCertificationForAdmin(_ context.Context, in *productpb.ReviewProductCertificationForAdminRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	s.lastReview = in
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) RevokeProductCertificationForAdmin(_ context.Context, in *productpb.ReviewProductCertificationForAdminRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	s.lastReview = in
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Data: fullApplication()}, nil
}

func (s *certEndpointStub) ReverifyProductCertificationForAdmin(_ context.Context, in *productpb.ReverifyProductCertificationForAdminRequest, _ ...grpc.CallOption) (*productpb.ProductCertificationApplicationAdminResponse, error) {
	s.lastReverify = in
	return &productpb.ProductCertificationApplicationAdminResponse{Code: s.code, Data: fullApplication()}, nil
}

func newCertEndpointRouter(stub *certEndpointStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Holder{Product: grpcclient.NewProductClientForTest(stub), Store: storeResolverForTest(2685914682)}

	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.GET("/products/:id/certification_applications", h.ListSellerProductCertificationApplications)
	app.POST("/products/:id/certification_applications", h.SubmitSellerProductCertification)
	app.POST("/products/:id/certification_applications/:application_id/withdraw", h.WithdrawSellerProductCertification)

	admin := r.Group("/api/v1/admin")
	admin.Use(fakeAdminAuth())
	admin.GET("/product_certification_applications", h.ListAdminProductCertificationApplications)
	admin.GET("/product_certification_applications/:id", h.GetAdminProductCertificationApplication)
	admin.GET("/product_certification_applications/:id/audit_logs", h.ListAdminProductCertificationAuditLogs)
	admin.POST("/product_certification_applications/:id/approve", h.ApproveAdminProductCertification)
	admin.POST("/product_certification_applications/:id/reject", h.RejectAdminProductCertification)
	admin.POST("/product_certification_applications/:id/revoke", h.RevokeAdminProductCertification)
	admin.POST("/product_certification_applications/:id/reverify", h.ReverifyAdminProductCertification)
	return r
}

func certCall(t *testing.T, r *gin.Engine, method, path, body, accountID string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", accountID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func certDataMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v（body=%s）", err, w.Body.String())
	}
	return env.Data
}

// 賣家 DTO 不得帶 ADM 內部註記與政府原始資料——即使上游把每一欄都填滿了。
func TestSellerCertificationDTO_HidesAdminOnlyFields(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodPost, "/api/v1/app/products/843856384063471755/certification_applications",
		`{"certificationType":"3703","certificateNo":"010105","idempotencyKey":"k1"}`, "2685914682")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	data := certDataMap(t, w)

	for _, forbidden := range []string{"internalNote", "govVerifyPayload", "submittedByAccountId", "productName", "currentApproved", "reviewedByAccountId", "storeId", "source", "requiresManualReview", "productPublicationState"} {
		if _, present := data[forbidden]; present {
			t.Errorf("賣家 DTO 不得含 %q（上游有填值 ⇒ 這是真的被帶出去了，不是碰巧為空）", forbidden)
		}
	}
	// 控制組：賣家真正需要的欄位必須在，否則上面的斷言可以靠「回一個空物件」通過。
	for _, want := range []string{"id", "productId", "certificationType", "certificateNo", "status", "govVerifyState", "publicReason", "version"} {
		if _, present := data[want]; !present {
			t.Errorf("賣家 DTO 缺少 %q", want)
		}
	}
}

// 送出／撤回／列表都必須把「登入 principal」與「當前 store surrogate」分開傳下去。
func TestSellerCertificationEndpoints_PassPrincipalAndStoreSeparately(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	certCall(t, r, http.MethodGet, "/api/v1/app/products/843856384063471755/certification_applications?status=SUBMITTED&page=2&page_size=5", "", "777")
	if stub.lastList == nil {
		t.Fatal("ListProductCertificationApplications 沒有被呼叫")
	}
	if stub.lastList.GetAccountId() != 777 {
		t.Errorf("account_id = %d, want 777（登入 principal）", stub.lastList.GetAccountId())
	}
	if stub.lastList.GetStoreId() != 2685914682 {
		t.Errorf("store_id = %d, want 2685914682（resolver 解析出的 surrogate，不得等於 account_id）", stub.lastList.GetStoreId())
	}
	if stub.lastList.GetProductId() != "843856384063471755" {
		t.Errorf("product_id = %q", stub.lastList.GetProductId())
	}
	if stub.lastList.GetStatus() != "SUBMITTED" || stub.lastList.GetPage() != "2" || stub.lastList.GetPageSize() != "5" {
		t.Errorf("查詢參數沒有透傳：status=%q page=%q pageSize=%q",
			stub.lastList.GetStatus(), stub.lastList.GetPage(), stub.lastList.GetPageSize())
	}

	certCall(t, r, http.MethodPost, "/api/v1/app/products/843856384063471755/certification_applications/843856384063471761/withdraw",
		`{"expectedVersion":"2","idempotencyKey":"k-wd"}`, "777")
	if stub.lastWithdraw == nil {
		t.Fatal("WithdrawProductCertification 沒有被呼叫")
	}
	if stub.lastWithdraw.GetApplicationId() != "843856384063471761" || stub.lastWithdraw.GetExpectedVersion() != "2" {
		t.Errorf("撤回參數沒有透傳：%+v", stub.lastWithdraw)
	}
	if stub.lastWithdraw.GetStoreId() != 2685914682 {
		t.Errorf("store_id = %d, want 2685914682", stub.lastWithdraw.GetStoreId())
	}
}

// 分頁三欄一律字串（19 §9.1 的 ⚠️：前端照數字宣告型別會全錯，且沉默到執行期）。
func TestSellerCertificationList_PaginationFieldsAreStrings(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodGet, "/api/v1/app/products/843856384063471755/certification_applications", "", "777")
	data := certDataMap(t, w)
	for _, key := range []string{"total", "page", "pageSize"} {
		if _, ok := data[key].(string); !ok {
			t.Errorf("data.%s = %#v，必須是字串", key, data[key])
		}
	}
	if _, ok := data["records"].([]any); !ok {
		t.Errorf("data.records = %#v，必須是陣列", data["records"])
	}
}

// 🔴 reviewer 一律取 auth context。body 就算塞了別的身分欄位也不得影響送下去的 account_id。
func TestAdminCertificationReview_ReviewerComesFromAuthContextNotBody(t *testing.T) {
	for _, action := range []string{"approve", "reject", "revoke"} {
		t.Run(action, func(t *testing.T) {
			stub := &certEndpointStub{}
			r := newCertEndpointRouter(stub)

			// body 刻意塞入偽造身分：accountId / reviewedByAccountId / adminId 都不該被讀。
			body := `{"idempotencyKey":"k-` + action + `","expectedVersion":"2",` +
				`"reasonCode":"CERT_NOT_FOUND","publicReason":"查無此編號","internalNote":"n",` +
				`"accountId":"1","reviewedByAccountId":"1","adminId":"1"}`
			w := certCall(t, r, http.MethodPost,
				"/api/v1/admin/product_certification_applications/843856384063471761/"+action, body, "2701338845")
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
			}
			if stub.lastReview == nil {
				t.Fatal("審核 RPC 沒有被呼叫，下面的斷言會空過")
			}
			if got := stub.lastReview.GetAccountId(); got != 2701338845 {
				t.Errorf("account_id = %d, want 2701338845（auth context），body 的偽造值不得生效", got)
			}
			if stub.lastReview.GetExpectedVersion() != "2" || stub.lastReview.GetReasonCode() != "CERT_NOT_FOUND" {
				t.Errorf("審核參數沒有透傳：%+v", stub.lastReview)
			}
			if stub.lastReview.GetInternalNote() != "n" {
				t.Error("internalNote 沒有透傳到下游")
			}
		})
	}
}

// reverify 不帶 expectedVersion（它不改 status），但一定要帶 idempotencyKey——
// 少了它，連點兩次就是打兩次政府 API，而那是有每日配額的外部資源。
func TestAdminCertificationReverify_RequiresIdempotencyKeyAndCarriesPrincipal(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodPost,
		"/api/v1/admin/product_certification_applications/843856384063471761/reverify", `{}`, "2701338845")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 idempotencyKey 應回 400，實際 %d；body = %s", w.Code, w.Body.String())
	}
	if stub.lastReverify != nil {
		t.Fatal("缺 idempotencyKey 的請求不得打到 product-service")
	}

	w = certCall(t, r, http.MethodPost,
		"/api/v1/admin/product_certification_applications/843856384063471761/reverify",
		`{"idempotencyKey":"k-rv"}`, "2701338845")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	if stub.lastReverify.GetAccountId() != 2701338845 || stub.lastReverify.GetIdempotencyKey() != "k-rv" {
		t.Errorf("reverify 參數沒有透傳：%+v", stub.lastReverify)
	}
}

// ADM 詳情必須看得到那三塊，且 rawExcerpt 是**物件**不是字串——回字串會逼前端再 parse
// 一次，而那個 parse 沒有人會寫。
func TestAdminCertificationDetail_ExposesAdminOnlyBlocks(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodGet,
		"/api/v1/admin/product_certification_applications/843856384063471761", "", "2701338845")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	data := certDataMap(t, w)
	for _, want := range []string{"govVerifyPayload", "internalNote", "currentApproved", "productName", "submittedByAccountId", "storeId", "source", "requiresManualReview", "productPublicationState"} {
		if v, present := data[want]; !present || v == nil {
			t.Errorf("ADM 詳情缺少 %q（實際 = %#v）", want, v)
		}
	}
	// 🔴 上面那條 `!present || v == nil` 對 productPublicationState **沒有鑑別力**：
	// 它是非指標 string 且無 omitempty，刪掉 handler 的映射那一行之後 JSON 仍會吐 ""，
	// present=true 且 v≠nil ⇒ 測試照樣綠。而「刪掉映射那一行」正是這個欄位唯一的失效模式，
	// 所以必須斷言值本身。fixture（fullApplication）設的是 "4014"。
	if got := data["productPublicationState"]; got != "4014" {
		t.Errorf("productPublicationState = %#v，want \"4014\"（上游有填值 ⇒ 空字串代表映射漏了）", got)
	}

	payload, _ := data["govVerifyPayload"].(map[string]any)
	if payload == nil {
		t.Fatal("govVerifyPayload 不是物件")
	}
	if _, ok := payload["rawExcerpt"].(map[string]any); !ok {
		t.Errorf("govVerifyPayload.rawExcerpt = %#v，必須是物件（proto 的 JSON 字串要在 Gateway 還原）", payload["rawExcerpt"])
	}
	if _, ok := payload["matchedCount"].(string); !ok {
		t.Errorf("govVerifyPayload.matchedCount = %#v，數字對外一律字串", payload["matchedCount"])
	}
	if stub.lastGetAdmin.GetAccountId() != 2701338845 {
		t.Errorf("詳情查詢沒有帶 auth context 的 account_id：%+v", stub.lastGetAdmin)
	}
}

// 審核事件流水的 actor 與快照欄位要完整帶出——它是這組端點唯一的稽核出口。
func TestAdminCertificationAuditLogs_CarryActorAndSnapshot(t *testing.T) {
	stub := &certEndpointStub{}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodGet,
		"/api/v1/admin/product_certification_applications/843856384063471761/audit_logs", "", "2701338845")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	data := certDataMap(t, w)
	records, _ := data["records"].([]any)
	if len(records) != 1 {
		t.Fatalf("records 筆數 = %d, want 1", len(records))
	}
	row, _ := records[0].(map[string]any)
	for _, want := range []string{"action", "fromStatus", "toStatus", "actorRole", "actorAccountId", "certificationType", "certificateNo", "applicationVersion", "createdAt"} {
		if _, present := row[want]; !present {
			t.Errorf("審核事件缺少 %q（實際 keys=%v）", want, row)
		}
	}
	if stub.lastAudit.GetApplicationId() != "843856384063471761" {
		t.Errorf("application_id 沒有透傳：%+v", stub.lastAudit)
	}
}

// 業務碼必須走 BizStatus 的登錄表分流：3025 是 404，不是 200 也不是塌縮的 409。
// 走 GrpcResult（HTTP 200 ＋ 非零 code）會讓 ADM 的「查不到」畫成一個空白詳情頁。
//
// ⚠️ **這支測試驗的是 handler 的 `resp.GetCode() != 0` 分支，而 production 走不到那裡。**
// certEndpointStub 是注進 NewProductClientForTest 的 mock proto client，那條路沒有
// *grpc.ClientConn ⇒ EnvelopeGuardInterceptor 不會跑 ⇒ {Code: 3025} 原樣抵達 handler。
// 真實路徑上 interceptor 會把它轉成 FailedPrecondition ＋ "[3025] " 前綴，status 由
// response.GrpcErr 查同一張登錄表決定 —— 本測試全綠而 dev 上 3025／3028 雙雙回 409，
// 就是這個落差（#2755）。**production 的保證在
// TestCertificationEndpoints_BizCodeStatusThroughEnvelopeGuard**（bufconn ＋ 真
// interceptor）；這支留著只證明「上游若真的把碼送到 handler，handler 不會吞掉它」。
func TestCertificationEndpoints_BusinessCodeMapsToHTTPStatus(t *testing.T) {
	stub := &certEndpointStub{code: 3025}
	r := newCertEndpointRouter(stub)

	w := certCall(t, r, http.MethodGet,
		"/api/v1/admin/product_certification_applications/843856384063471761", "", "2701338845")
	if w.Code != http.StatusNotFound {
		t.Errorf("3025 → HTTP %d, want 404；body = %s", w.Code, w.Body.String())
	}

	stub.code = 3026
	w = certCall(t, r, http.MethodPost,
		"/api/v1/admin/product_certification_applications/843856384063471761/approve",
		`{"idempotencyKey":"k","expectedVersion":"2"}`, "2701338845")
	if w.Code != http.StatusConflict {
		t.Errorf("3026 → HTTP %d, want 409；body = %s", w.Code, w.Body.String())
	}

	// 賣家側同樣要分流（非本人商品 → 3025 → 404，而不是一個空列表）。
	stub.code = 3025
	w = certCall(t, r, http.MethodGet,
		"/api/v1/app/products/843856384063471755/certification_applications", "", "777")
	if w.Code != http.StatusNotFound {
		t.Errorf("賣家列表 3025 → HTTP %d, want 404；body = %s", w.Code, w.Body.String())
	}
}
