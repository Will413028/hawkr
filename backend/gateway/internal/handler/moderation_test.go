package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// setupModerationRouter mirrors registerModerationRoutes — admin role check is
// enforced at the router level in production, so handler-level tests bypass it.
func setupModerationRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")

	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	{
		app.POST("/reports", h.SubmitReport)
		app.GET("/reports/me", h.ListMyReports)
		app.POST("/blocks/:userId", h.BlockUser)
		app.DELETE("/blocks/:userId", h.UnblockUser)
		app.GET("/blocks", h.ListBlocks)
	}

	admin := v1.Group("/admin")
	admin.Use(fakeJWTAuth())
	{
		admin.GET("/reports/:id", h.AdminGetReport)
		admin.POST("/reports/:id/resolve", h.AdminResolveReport)
	}
	return r
}

// newModerationHolder models today's production reality on BOTH axes: every
// stint is still grandfathered onto account_id (store_id == buyer_id ==
// account_id). resolveModerationUserID consults the other axis whenever the
// active one reports no stint (#2867), so a holder that left Store nil would
// nil-panic the moment a test exercises that arm — and, worse, would hide the
// fact that under grandfather both axes answer with the same value.
func newModerationHolder(mc *mockSocialServiceClient) *Holder {
	return &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		BuyerID: buyerGrandfatherResolverForTest(),
		Store:   grandfatherResolverForTest(),
	}
}

func doModerationReq(r *gin.Engine, method, path, accountID, loginRole string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if accountID != "" {
		req.Header.Set("X-Account-Id", accountID)
	}
	if loginRole != "" {
		req.Header.Set("X-Login-Role", loginRole)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== ListMyReports ====================

func TestListMyReports_HappyWithPaging(t *testing.T) {
	var captured *socialpb.ListMyReportsRequest
	mc := &mockSocialServiceClient{
		ListMyReportsFn: func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
			captured = in
			return &socialpb.ListMyReportsResponse{Reports: []*socialpb.Report{{Id: "1"}, {Id: "2"}}}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/app/reports/me?page=3&page_size=50", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	if captured == nil || captured.ReporterId != 42 || captured.Page != 3 || captured.PageSize != 50 {
		t.Errorf("captured = %+v", captured)
	}
}

func TestListMyReports_DefaultPaging(t *testing.T) {
	var captured *socialpb.ListMyReportsRequest
	mc := &mockSocialServiceClient{
		ListMyReportsFn: func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
			captured = in
			return &socialpb.ListMyReportsResponse{Reports: nil}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/app/reports/me", "7", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.Page != 1 || captured.PageSize != 20 {
		t.Errorf("default paging = page %d size %d, want 1/20", captured.Page, captured.PageSize)
	}
}

func TestListMyReports_GrpcError(t *testing.T) {
	mc := &mockSocialServiceClient{
		ListMyReportsFn: func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "social down")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/app/reports/me", "1", "1", nil)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestListBlocks_UsesDTOAndReturnsEmptyArray(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mc := &mockSocialServiceClient{
			ListBlocksFn: func(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error) {
				return &socialpb.ListBlocksResponse{}, nil
			},
		}
		r := setupModerationRouter(newModerationHolder(mc))
		w := doModerationReq(r, http.MethodGet, "/api/v1/app/blocks", "42", "1", nil)
		body := parseJSON(t, w.Body)
		data := body["data"].(map[string]any)
		if records, ok := data["records"].([]any); !ok || len(records) != 0 {
			t.Fatalf("records = %#v, want []", data["records"])
		}
	})

	t.Run("camelCase and int64 string", func(t *testing.T) {
		mc := &mockSocialServiceClient{
			ListBlocksFn: func(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error) {
				return &socialpb.ListBlocksResponse{Blocks: []*socialpb.UserBlock{{
					Id: "1", BlockedId: "2", CreateTimeMs: 1785900000000,
				}}}, nil
			},
		}
		r := setupModerationRouter(newModerationHolder(mc))
		w := doModerationReq(r, http.MethodGet, "/api/v1/app/blocks", "42", "1", nil)
		body := parseJSON(t, w.Body)
		record := body["data"].(map[string]any)["records"].([]any)[0].(map[string]any)
		if record["blockedId"] != "2" || record["createTimeMs"] != "1785900000000" {
			t.Fatalf("record = %#v, want camelCase DTO with string timestamp", record)
		}
		if _, leaked := record["blocked_id"]; leaked {
			t.Fatalf("record leaked proto key blocked_id: %#v", record)
		}
	})
}

// ==================== BlockUser ====================

func TestBlockUser_Happy(t *testing.T) {
	var captured *socialpb.BlockUserRequest
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			captured = in
			return &socialpb.BlockUserResponse{}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "POST", "/api/v1/app/blocks/857045288972403572", "10", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 10 || captured.BlockedId != "857045288972403572" {
		t.Errorf("captured = %+v, want blocker=10 blocked=857045288972403572", captured)
	}
}

func TestBlockUser_InvalidUserId(t *testing.T) {
	// path id 改 string 透傳後 gateway 不再做數字格式驗證；非法 id 由 social-service
	// 判定（parseInt64("abc")=0 → blocked_id required InvalidArgument）。驗證 gateway
	// 正確將下游 InvalidArgument 轉為 HTTP 400。
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			return nil, grpcstatus.Error(grpccodes.InvalidArgument, "blocked_id is required")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "POST", "/api/v1/app/blocks/abc", "1", "1", nil)
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestBlockUser_GrpcError(t *testing.T) {
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "social down")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "POST", "/api/v1/app/blocks/77", "1", "1", nil)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== UnblockUser ====================

func TestUnblockUser_Happy(t *testing.T) {
	var captured *socialpb.UnblockUserRequest
	mc := &mockSocialServiceClient{
		UnblockUserFn: func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
			captured = in
			return &socialpb.UnblockUserResponse{}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "DELETE", "/api/v1/app/blocks/857045288972403572", "10", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 10 || captured.BlockedId != "857045288972403572" {
		t.Errorf("captured = %+v, want blocker=10 blocked=857045288972403572", captured)
	}
}

func TestUnblockUser_InvalidUserId(t *testing.T) {
	// 同 TestBlockUser_InvalidUserId：非法 id 由 social-service 判定，gateway 轉 400。
	mc := &mockSocialServiceClient{
		UnblockUserFn: func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
			return nil, grpcstatus.Error(grpccodes.InvalidArgument, "blocked_id is required")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "DELETE", "/api/v1/app/blocks/bad", "1", "1", nil)
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestUnblockUser_GrpcError(t *testing.T) {
	mc := &mockSocialServiceClient{
		UnblockUserFn: func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Internal, "boom")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "DELETE", "/api/v1/app/blocks/77", "1", "1", nil)
	assertHTTPStatus(t, w, http.StatusInternalServerError)
}

// ==================== AdminGetReport ====================

func TestAdminGetReport_Happy(t *testing.T) {
	var captured *socialpb.AdminGetReportRequest
	mc := &mockSocialServiceClient{
		AdminGetReportFn: func(ctx context.Context, in *socialpb.AdminGetReportRequest, opts ...grpc.CallOption) (*socialpb.AdminGetReportResponse, error) {
			captured = in
			return &socialpb.AdminGetReportResponse{Report: &socialpb.Report{Id: "42", Status: "pending"}}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/admin/reports/857045288972403572", "1", "", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReportId != "857045288972403572" {
		t.Errorf("captured = %+v, want ReportId=857045288972403572", captured)
	}
}

func TestAdminGetReport_InvalidId(t *testing.T) {
	// 非法 report_id 由 social-service 判定（parseInt64=0 → InvalidArgument），gateway 轉 400。
	mc := &mockSocialServiceClient{
		AdminGetReportFn: func(ctx context.Context, in *socialpb.AdminGetReportRequest, opts ...grpc.CallOption) (*socialpb.AdminGetReportResponse, error) {
			return nil, grpcstatus.Error(grpccodes.InvalidArgument, "report_id is required")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/admin/reports/notanint", "1", "", nil)
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestAdminGetReport_GrpcNotFound(t *testing.T) {
	mc := &mockSocialServiceClient{
		AdminGetReportFn: func(ctx context.Context, in *socialpb.AdminGetReportRequest, opts ...grpc.CallOption) (*socialpb.AdminGetReportResponse, error) {
			return nil, grpcstatus.Error(grpccodes.NotFound, "report not found")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "GET", "/api/v1/admin/reports/999", "1", "", nil)
	assertHTTPStatus(t, w, http.StatusNotFound)
}

// ==================== AdminResolveReport ====================

func TestAdminResolveReport_Happy(t *testing.T) {
	var captured *socialpb.AdminResolveReportRequest
	mc := &mockSocialServiceClient{
		AdminResolveReportFn: func(ctx context.Context, in *socialpb.AdminResolveReportRequest, opts ...grpc.CallOption) (*socialpb.AdminResolveReportResponse, error) {
			captured = in
			return &socialpb.AdminResolveReportResponse{Report: &socialpb.Report{Id: "42", Status: "resolved"}}, nil
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "POST", "/api/v1/admin/reports/857045288972403572/resolve", "100", "", map[string]string{
		"action": "HIDE_TARGET",
		"note":   "test note",
	})
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReportId != "857045288972403572" || captured.AdminId != 100 || captured.Action != "HIDE_TARGET" {
		t.Errorf("captured = %+v", captured)
	}
}

func TestAdminResolveReport_InvalidId(t *testing.T) {
	// 非法 report_id 由 social-service 判定（parseInt64=0 → InvalidArgument），gateway 轉 400。
	mc := &mockSocialServiceClient{
		AdminResolveReportFn: func(ctx context.Context, in *socialpb.AdminResolveReportRequest, opts ...grpc.CallOption) (*socialpb.AdminResolveReportResponse, error) {
			return nil, grpcstatus.Error(grpccodes.InvalidArgument, "report_id is required")
		},
	}
	r := setupModerationRouter(newModerationHolder(mc))
	w := doModerationReq(r, "POST", "/api/v1/admin/reports/abc/resolve", "1", "", map[string]string{"action": "DISMISS"})
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestAdminResolveReport_MissingAction(t *testing.T) {
	r := setupModerationRouter(newModerationHolder(&mockSocialServiceClient{}))
	w := doModerationReq(r, "POST", "/api/v1/admin/reports/42/resolve", "1", "", map[string]string{"note": "no action"})
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestSubmitReport_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.SubmitReportRequest
	mc := &mockSocialServiceClient{
		SubmitReportFn: func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
			captured = in
			return &socialpb.SubmitReportResponse{ReportId: "1", Status: "pending"}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupModerationRouter(h)
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	w := doModerationReq(r, "POST", "/api/v1/app/reports", "42", "1", body)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 9999 {
		t.Errorf("captured.ReporterId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// submitReportCaptor 回傳一個把 SubmitReportRequest 存下來的 mock。下面同族的幾支
// 只差在「兩軸各回什麼」，共用同一個 captor 才看得出差異在哪。
func submitReportCaptor() (*mockSocialServiceClient, func() *socialpb.SubmitReportRequest) {
	var captured *socialpb.SubmitReportRequest
	mc := &mockSocialServiceClient{
		SubmitReportFn: func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
			captured = in
			return &socialpb.SubmitReportResponse{ReportId: "1", Status: "pending"}, nil
		},
	}
	return mc, func() *socialpb.SubmitReportRequest { return captured }
}

func submitReportAs(t *testing.T, h *Holder, accountID, loginRole string) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	return doModerationReq(setupModerationRouter(h), "POST", "/api/v1/app/reports", accountID, loginRole, body)
}

// reporter_id 與 blocker_id 是同一個 resolveModerationUserID 的產物，所以檢舉這一側
// 只釘住「買家軸無 stint → 改問賣家軸」與「兩軸皆無 → 回原值」兩個端點；為什麼是
// 這兩個、以及 grandfather／re-key 的差別，完整說明在下面 TestBlockUser_ 同族。
func TestSubmitReport_NoCurrentBuyerResolvesCurrentStoreID(t *testing.T) {
	mc, captured := submitReportCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = storeResolverForTest(8001)
	w := submitReportAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 8001 {
		t.Errorf("captured.ReporterId = %v, want 8001 (current store_id, not raw account_id 42)", got)
	}
}

func TestSubmitReport_NoStintOnEitherAxisFallsBackToRaw(t *testing.T) {
	mc, captured := submitReportCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = noStoreResolverForTest()
	w := submitReportAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 42 {
		t.Errorf("captured.ReporterId = %v, want 42 (no live stint on either axis: raw account_id, still NOT 403)", got)
	}
}

// TestSubmitReport_BuyerServiceErrorPropagates：真正的後端錯誤（非 ErrNoCurrentBuyer）
// 仍照既有錯誤回應映射（503），不被 fallback 吞掉。
func TestSubmitReport_BuyerServiceErrorPropagates(t *testing.T) {
	h := newModerationHolder(&mockSocialServiceClient{})
	h.BuyerID = buyerServiceDownResolverForTest()
	r := setupModerationRouter(h)
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	w := doModerationReq(r, "POST", "/api/v1/app/reports", "42", "1", body)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestListMyReports_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.ListMyReportsRequest
	mc := &mockSocialServiceClient{
		ListMyReportsFn: func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
			captured = in
			return &socialpb.ListMyReportsResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "GET", "/api/v1/app/reports/me", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 9999 {
		t.Errorf("captured.ReporterId = %v, want 9999", captured)
	}
}

func TestBlockUser_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.BlockUserRequest
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			captured = in
			return &socialpb.BlockUserResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "POST", "/api/v1/app/blocks/857045288972403572", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 9999 {
		t.Errorf("captured.BlockerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

// blockUserCaptor 回傳一個把 BlockUserRequest 存下來的 mock，讓下面同族的幾支不必
// 各抄一份 mock —— 它們之間唯一的差別就是「兩軸各回什麼」。
func blockUserCaptor() (*mockSocialServiceClient, func() *socialpb.BlockUserRequest) {
	var captured *socialpb.BlockUserRequest
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			captured = in
			return &socialpb.BlockUserResponse{}, nil
		},
	}
	return mc, func() *socialpb.BlockUserRequest { return captured }
}

func blockUserAs(t *testing.T, h *Holder, accountID, loginRole string) *httptest.ResponseRecorder {
	t.Helper()
	return doModerationReq(setupModerationRouter(h), "POST", "/api/v1/app/blocks/857045288972403572", accountID, loginRole, nil)
}

// ==================== 買家軸無 stint：改問賣家軸，而不是送原始 account_id ====================
//
// 這一族取代了原本的 TestBlockUser_NoCurrentBuyerFallsBackToRaw（#2867）。它守的
// 不變式換了一個：
//
//	舊：買家軸查無 stint → 送 raw account_id，重點只在「不要 403」。
//	新：買家軸查無 stint → 改問賣家軸，拿到的是現任 store_id；仍然不 403。
//
// 為什麼要換：blocker_id 的值域是「活著的 store_id 或活著的 buyer_id」，而
// social-service 自 #2789 起對兩軸都認不得的 id 是 fail-closed 的。raw account_id
// 今天之所以能用，只是因為 grandfather 讓 store_id == buyer_id == account_id；
// 那個帳號一旦 re-key（設計書 44 W2/W3），同一個值兩軸都查不到 ⇒ 這個人一個都封鎖
// 不了。舊測試「送 42」在 grandfather 下永遠成立，所以它看不見這件事。
//
// 三支各自守一半：
//   - _ResolvesCurrentStoreID：re-key 之後（store_id 8001 ≠ account_id 42）送的是
//     8001。這支才是本票的實質——舊寫法在這裡會送 42。
//   - _GrandfatheredStoreSendsSameValue：grandfather 下（store_id == account_id）
//     新舊寫法送的是同一個值，證明今天的行為沒有被改動。
//   - _NoStintOnEitherAxis / _SellerAxisDown：兩個退場條件，都仍然是 200 不是 403。
func TestBlockUser_NoCurrentBuyerResolvesCurrentStoreID(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = storeResolverForTest(8001)
	w := blockUserAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 8001 {
		t.Errorf("captured.BlockerId = %v, want 8001 (current store_id; raw account_id 42 resolves on neither axis after re-key)", got)
	}
}

func TestBlockUser_NoCurrentBuyerGrandfatheredStoreSendsSameValue(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = grandfatherResolverForTest() // store_id == account_id，即今天 prod 的狀態
	w := blockUserAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 42 {
		t.Errorf("captured.BlockerId = %v, want 42 (grandfathered store_id == account_id: 走 resolver 與舊的 raw fallback 同值)", got)
	}
}

func TestBlockUser_NoStintOnEitherAxisFallsBackToRaw(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = noStoreResolverForTest()
	w := blockUserAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 42 {
		t.Errorf("captured.BlockerId = %v, want 42 (no live stint on either axis: raw account_id, still NOT 403)", got)
	}
}

// 次要查詢是 best-effort：主軸已經回答「查無 stint」，這時賣家軸掛掉不該讓這條路徑
// 多出一個它本來沒有的失敗模式（503）。真正的後端錯誤只有發生在**主軸**時才傳播，
// 見 TestSubmitReport_BuyerServiceErrorPropagates。
func TestBlockUser_NoCurrentBuyerSellerAxisDownStillFallsBackToRaw(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.BuyerID = noBuyerResolverForTest()
	h.Store = storeServiceDownResolverForTest()
	w := blockUserAs(t, h, "42", "1")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 42 {
		t.Errorf("captured.BlockerId = %v, want 42 (secondary lookup is best-effort, must not 503)", got)
	}
}

func TestUnblockUser_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.UnblockUserRequest
	mc := &mockSocialServiceClient{
		UnblockUserFn: func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
			captured = in
			return &socialpb.UnblockUserResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "DELETE", "/api/v1/app/blocks/857045288972403572", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 9999 {
		t.Errorf("captured.BlockerId = %v, want 9999", captured)
	}
}

func TestListBlocks_ResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.ListBlocksRequest
	mc := &mockSocialServiceClient{
		ListBlocksFn: func(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error) {
			captured = in
			return &socialpb.ListBlocksResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "GET", "/api/v1/app/blocks", "42", "1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 9999 {
		t.Errorf("captured.BlockerId = %v, want 9999", captured)
	}
}

// ==================== loginRole gate: seller resolves via store_id, unset never resolves ====================
//
// resolveModerationUserID gates on the caller's active role (loginRole,
// moderation.go). loginRole==2 (seller)
// resolves to the CURRENT store_id (2026-08-01 ADR) — the
// *_SellerRoleResolvesCurrentStoreID tests below prove the seller resolver
// IS consulted and its result used, and mustNotResolveBuyerSeller in those
// same tests proves the BUYER resolver is not touched **while the seller axis
// answers**. That last clause is narrower than it used to be: since #2867 a
// seller-active caller whose seller axis reports no current store does fall
// through to the buyer axis (see TestBlockUser_NoCurrentSellerResolvesCurrentBuyerID),
// because the alternative — sending a raw account_id — resolves on neither
// axis once stints are re-keyed. Only an unresolved/other active role
// (loginRole=="" below) skips both resolvers entirely — one representative
// endpoint suffices for that arm since every endpoint shares the same gate.

// This file reuses storeResolverForTest / noStoreResolverForTest /
// storeServiceDownResolverForTest from resolve_seller_test.go directly (same
// package, no redefinition needed).

func TestSubmitReport_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.SubmitReportRequest
	mc := &mockSocialServiceClient{
		SubmitReportFn: func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
			captured = in
			return &socialpb.SubmitReportResponse{ReportId: "1", Status: "pending"}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeResolverForTest(8001)
	r := setupModerationRouter(h)
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	w := doModerationReq(r, "POST", "/api/v1/app/reports", "42", "2", body)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 8001 {
		t.Errorf("captured.ReporterId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

// 賣家軸無 stint 的鏡像：改問買家軸，兩軸皆無才回原值。
func TestSubmitReport_NoCurrentSellerResolvesCurrentBuyerID(t *testing.T) {
	mc, captured := submitReportCaptor()
	h := newModerationHolder(mc)
	h.Store = noStoreResolverForTest()
	h.BuyerID = buyerResolverForTest(9001)
	w := submitReportAs(t, h, "42", "2")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 9001 {
		t.Errorf("captured.ReporterId = %v, want 9001 (current buyer_id, not raw account_id 42)", got)
	}
}

func TestSubmitReport_SellerRoleNoStintOnEitherAxisFallsBackToRaw(t *testing.T) {
	mc, captured := submitReportCaptor()
	h := newModerationHolder(mc)
	h.Store = noStoreResolverForTest()
	h.BuyerID = noBuyerResolverForTest()
	w := submitReportAs(t, h, "42", "2")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.ReporterId != 42 {
		t.Errorf("captured.ReporterId = %v, want 42 (no live stint on either axis: raw account_id, still NOT 403)", got)
	}
}

// TestSubmitReport_SellerServiceErrorPropagates：真正的後端錯誤（非
// ErrNoCurrentStore）仍照既有錯誤回應映射（503），不被 fallback 吞掉。
func TestSubmitReport_SellerServiceErrorPropagates(t *testing.T) {
	h := newModerationHolder(&mockSocialServiceClient{})
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeServiceDownResolverForTest()
	r := setupModerationRouter(h)
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	w := doModerationReq(r, "POST", "/api/v1/app/reports", "42", "2", body)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestSubmitReport_UnresolvedLoginRoleRejected(t *testing.T) {
	mc := &mockSocialServiceClient{
		SubmitReportFn: func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
			t.Fatal("SubmitReport should not be called when loginRole is unresolved")
			return nil, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = mustNotResolveStore(t)
	r := setupModerationRouter(h)
	body := map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM"}
	w := doModerationReq(r, "POST", "/api/v1/app/reports", "42", "", body)
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestListMyReports_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.ListMyReportsRequest
	mc := &mockSocialServiceClient{
		ListMyReportsFn: func(ctx context.Context, in *socialpb.ListMyReportsRequest, opts ...grpc.CallOption) (*socialpb.ListMyReportsResponse, error) {
			captured = in
			return &socialpb.ListMyReportsResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeResolverForTest(8001)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "GET", "/api/v1/app/reports/me", "42", "2", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.ReporterId != 8001 {
		t.Errorf("captured.ReporterId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

func TestBlockUser_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.BlockUserRequest
	mc := &mockSocialServiceClient{
		BlockUserFn: func(ctx context.Context, in *socialpb.BlockUserRequest, opts ...grpc.CallOption) (*socialpb.BlockUserResponse, error) {
			captured = in
			return &socialpb.BlockUserResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeResolverForTest(8001)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "POST", "/api/v1/app/blocks/857045288972403572", "42", "2", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 8001 {
		t.Errorf("captured.BlockerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

// TestBlockUser_NoCurrentSeller*：同 TestSubmitReport_NoCurrentSeller* 的封鎖側。
func TestBlockUser_NoCurrentSellerResolvesCurrentBuyerID(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.Store = noStoreResolverForTest()
	h.BuyerID = buyerResolverForTest(9001)
	w := blockUserAs(t, h, "42", "2")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 9001 {
		t.Errorf("captured.BlockerId = %v, want 9001 (current buyer_id, not raw account_id 42)", got)
	}
}

func TestBlockUser_SellerRoleNoStintOnEitherAxisFallsBackToRaw(t *testing.T) {
	mc, captured := blockUserCaptor()
	h := newModerationHolder(mc)
	h.Store = noStoreResolverForTest()
	h.BuyerID = noBuyerResolverForTest()
	w := blockUserAs(t, h, "42", "2")
	assertHTTPStatus(t, w, http.StatusOK)
	if got := captured(); got == nil || got.BlockerId != 42 {
		t.Errorf("captured.BlockerId = %v, want 42 (no live stint on either axis: raw account_id, still NOT 403)", got)
	}
}

func TestUnblockUser_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.UnblockUserRequest
	mc := &mockSocialServiceClient{
		UnblockUserFn: func(ctx context.Context, in *socialpb.UnblockUserRequest, opts ...grpc.CallOption) (*socialpb.UnblockUserResponse, error) {
			captured = in
			return &socialpb.UnblockUserResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeResolverForTest(8001)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "DELETE", "/api/v1/app/blocks/857045288972403572", "42", "2", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 8001 {
		t.Errorf("captured.BlockerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

func TestListBlocks_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	var captured *socialpb.ListBlocksRequest
	mc := &mockSocialServiceClient{
		ListBlocksFn: func(ctx context.Context, in *socialpb.ListBlocksRequest, opts ...grpc.CallOption) (*socialpb.ListBlocksResponse, error) {
			captured = in
			return &socialpb.ListBlocksResponse{}, nil
		},
	}
	h := newModerationHolder(mc)
	h.BuyerID = mustNotResolveBuyerSeller(t)
	h.Store = storeResolverForTest(8001)
	r := setupModerationRouter(h)
	w := doModerationReq(r, "GET", "/api/v1/app/blocks", "42", "2", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BlockerId != 8001 {
		t.Errorf("captured.BlockerId = %v, want 8001 (current store_id, not raw account_id 42)", captured)
	}
}

// ==================== C3：evidenceKeys 契約（欄位名不再說謊）====================

// POST /app/reports 的證據欄過去叫 evidence_urls，但下游 proto 欄位就是 evidence_keys、
// 值也一直是裸 objectKey（social-service 直接 json.Marshal 進 reports.evidence）。
// 欄位名收斂為 evidenceKeys 之後，這裡釘住三件事：
//  1. 新欄位名真的被讀到並原樣傳給 social-service；
//  2. 舊欄位名 evidence_urls 不再被接受（送了等於沒送，不是靜默改讀別的欄位）；
//  3. 前端可控寫入路徑只收 objectKey（10 檔 §25）——本平台顯示 URL 會被剝殼成裸 key，
//     外來域名與非 public/ 前綴一律 400，不得落庫。
func TestSubmitReport_EvidenceKeysContract(t *testing.T) {
	const mediaBase = "https://test-media.example.com"
	cases := []struct {
		name     string
		body     map[string]interface{}
		wantHTTP int
		wantKeys []string
	}{
		{
			name:     "裸 objectKey 原樣傳遞",
			body:     map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM", "evidenceKeys": []string{"public/report/1/a.jpg"}},
			wantHTTP: http.StatusOK,
			wantKeys: []string{"public/report/1/a.jpg"},
		},
		{
			name:     "本平台顯示 URL 剝殼還原成 objectKey",
			body:     map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM", "evidenceKeys": []string{mediaBase + "/i/original/public/report/1/a.jpg"}},
			wantHTTP: http.StatusOK,
			wantKeys: []string{"public/report/1/a.jpg"},
		},
		{
			name:     "外來域名一律 400，不得落庫",
			body:     map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM", "evidenceKeys": []string{"https://evil.example.com/a.jpg"}},
			wantHTTP: http.StatusBadRequest,
		},
		{
			name:     "非 public/ 前綴一律 400",
			body:     map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM", "evidenceKeys": []string{"secure/kyc/1/id.jpg"}},
			wantHTTP: http.StatusBadRequest,
		},
		{
			name:     "舊欄位名 evidence_urls 不再被接受",
			body:     map[string]interface{}{"targetType": "POST", "targetId": "1", "reasonCode": "SPAM", "evidence_urls": []string{"public/report/1/a.jpg"}},
			wantHTTP: http.StatusOK,
			wantKeys: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured *socialpb.SubmitReportRequest
			mc := &mockSocialServiceClient{
				SubmitReportFn: func(ctx context.Context, in *socialpb.SubmitReportRequest, opts ...grpc.CallOption) (*socialpb.SubmitReportResponse, error) {
					captured = in
					return &socialpb.SubmitReportResponse{ReportId: "1", Status: "pending"}, nil
				},
			}
			h := newModerationHolder(mc)
			h.MediaBaseURL = mediaBase
			w := doModerationReq(setupModerationRouter(h), "POST", "/api/v1/app/reports", "42", "1", tc.body)
			assertHTTPStatus(t, w, tc.wantHTTP)
			if tc.wantHTTP != http.StatusOK {
				if captured != nil {
					t.Fatalf("被拒的輸入不得抵達 social-service；captured = %v", captured)
				}
				return
			}
			if captured == nil {
				t.Fatal("social-service 未被呼叫")
			}
			if len(captured.EvidenceKeys) != len(tc.wantKeys) {
				t.Fatalf("EvidenceKeys = %v, want %v", captured.EvidenceKeys, tc.wantKeys)
			}
			for i, want := range tc.wantKeys {
				if captured.EvidenceKeys[i] != want {
					t.Errorf("EvidenceKeys[%d] = %q, want %q", i, captured.EvidenceKeys[i], want)
				}
			}
		})
	}
}
