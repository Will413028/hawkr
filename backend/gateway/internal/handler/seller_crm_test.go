package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	couponpb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// couponMockCouponSvcClient is defined in coupon_test.go (shared across test files)

// ==================== resolveCustomerBuyerID ====================

// TestResolveCustomerBuyerID_TakesBuyerIDDirectly 釘住新語意：
// /customers/{buyerId} 收的就是 buyer_id（列表 / lookup 回的那個值）
//
// 舊行為的兩個故障：
//
//	① 分身買家（account != buyer_id）拿列表值打 detail → 404 buyer not found
//	② stint 輪替後舊 crm 列掛舊 buyer_id，卻被解析成現任 → 查無列、歷史歸零
//
// resolver 這裡故意設成「任何輸入都翻成別的值」——新實作根本不該碰它。
func TestResolveCustomerBuyerID_TakesBuyerIDDirectly(t *testing.T) {
	const staleStintBuyerID = int64(2619321233) // 舊 stint，非現任
	h := &Holder{
		BuyerID: buyerResolverForTest(7484139444629864823),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "buyerId", Value: strconv.FormatInt(staleStintBuyerID, 10)}}
	c.Request = httptest.NewRequest("GET", "/", nil)

	got, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		t.Fatalf("resolveCustomerBuyerID failed, body=%s", w.Body.String())
	}
	if int64(got) != staleStintBuyerID {
		t.Fatalf("got %d, want the path param verbatim %d (must NOT resolve to current stint)",
			int64(got), staleStintBuyerID)
	}
}

func TestResolveCustomerBuyerID_RejectsNonNumeric(t *testing.T) {
	h := &Holder{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "buyerId", Value: "abc"}}
	c.Request = httptest.NewRequest("GET", "/", nil)

	if _, ok := h.resolveCustomerBuyerID(c); ok {
		t.Fatal("expected failure on non-numeric buyerId")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// buyer_id 是 TSID，恆為正。0 / 負數是壞輸入，必須在 gateway 擋掉而不是讓它
// 一路打到 crm / order —— 下游對 buyer_id=0 只會回空結果，看起來像「這位顧客
// 沒有任何紀錄」，把壞請求偽裝成合法的空狀態。
func TestResolveCustomerBuyerID_RejectsNonPositive(t *testing.T) {
	for _, raw := range []string{"0", "-1"} {
		t.Run(raw, func(t *testing.T) {
			h := &Holder{}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "buyerId", Value: raw}}
			c.Request = httptest.NewRequest("GET", "/", nil)

			if _, ok := h.resolveCustomerBuyerID(c); ok {
				t.Fatalf("expected failure on buyerId=%q", raw)
			}
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

// ==================== router ====================

func setupCrmRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	{
		v1.GET("/customers/:buyerId/orders", h.GetCustomerOrders)
		v1.GET("/customers/:buyerId/coupons", h.GetCustomerCoupons)
	}
	return r
}

// ==================== GetCustomerOrders tests ====================

func TestGetCustomerOrders_Success(t *testing.T) {
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			if in.BuyerId != 2688657200 {
				t.Errorf("BuyerId = %d, want 2688657200", in.BuyerId)
			}
			if in.StoreId != 99 {
				t.Errorf("StoreId = %d, want 99", in.StoreId)
			}
			return &orderpb.GetBuyerOrdersForSellerResponse{
				Code:    0,
				Message: "ok",
				Records: []*orderpb.BuyerSellerOrderRecord{
					{OrderNo: "ORD001", CreateTime: "1710000000000", ActualAmount: "350", DiscountAmount: "50", RefundAmount: "120"},
					{OrderNo: "ORD002", CreateTime: "1710100000000", ActualAmount: "200", DiscountAmount: "0", RefundAmount: "0"},
				},
				Total:    "2",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders?page=1&page_size=20", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	records, ok := data["records"].([]interface{})
	if !ok {
		t.Fatal("records is not an array")
	}
	if len(records) != 2 {
		t.Errorf("len(records) = %d, want 2", len(records))
	}
	if total := data["total"].(string); total != "2" {
		t.Errorf("total = %s, want 2", total)
	}
	// 這條清單自 2026-08-05 起含 PARTIALLY_REFUNDED(60) 的訂單，refundAmount 必須逐筆
	// 透傳（含 "0"，不得被吞）——否則賣家看到部分退款單只會看到原始成交額。
	for i, want := range []string{"120", "0"} {
		row, ok := records[i].(map[string]interface{})
		if !ok {
			t.Fatalf("records[%d] 不是物件；got %#v", i, records[i])
		}
		got, present := row["refundAmount"]
		if !present {
			t.Errorf("records[%d] 缺少 refundAmount；row = %v", i, row)
			continue
		}
		if got != want {
			t.Errorf("records[%d].refundAmount = %#v, want %q", i, got, want)
		}
	}
}

func TestGetCustomerOrders_InvalidBuyerId(t *testing.T) {
	orderMock := &mockOrderServiceClient{}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/abc/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestGetCustomerOrders_GrpcError(t *testing.T) {
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			return nil, fmt.Errorf("order connection refused")
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetCustomerOrders_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestGetCustomerOrders_DefaultPagination(t *testing.T) {
	var capturedPage, capturedSize string
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			capturedPage = in.Page
			capturedSize = in.Size
			return &orderpb.GetBuyerOrdersForSellerResponse{
				Code: 0, Total: "0", Page: "1", PageSize: "20",
			}, nil
		},
	}

	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if capturedPage != "1" {
		t.Errorf("Page = %q, want \"1\"", capturedPage)
	}
	if capturedSize != "10" {
		t.Errorf("Size = %q, want \"10\"", capturedSize)
	}
}

// ==================== GetCustomerCoupons tests ====================

func TestGetCustomerCoupons_Success(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			if in.StoreId != 99 {
				t.Errorf("StoreId = %d, want 99", in.StoreId)
			}
			if in.BuyerId != "2688657200" {
				t.Errorf("BuyerId = %s, want 2688657200", in.BuyerId)
			}
			return &couponpb.ListBuyerCouponsForSellerResponse{
				Code:    0,
				Message: "ok",
				Records: []*couponpb.TemplateInfo{
					{TemplateId: "tpl-1", Title: "滿200折50", Status: "ACTIVE"},
				},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}

	h := &Holder{Coupon: grpcclient.NewCouponClientForTest(couponMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/coupons?page=1&page_size=20", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	records, ok := data["records"].([]interface{})
	if !ok {
		t.Fatal("records is not an array")
	}
	if len(records) != 1 {
		t.Errorf("len(records) = %d, want 1", len(records))
	}
	if total := data["total"].(string); total != "1" {
		t.Errorf("total = %s, want 1", total)
	}
}

func TestGetCustomerCoupons_WithStatusFilter(t *testing.T) {
	var capturedStatus string
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			capturedStatus = in.Status
			return &couponpb.ListBuyerCouponsForSellerResponse{
				Code: 0, Total: "0", Page: "1", PageSize: "20",
			}, nil
		},
	}

	h := &Holder{Coupon: grpcclient.NewCouponClientForTest(couponMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/coupons?status=available", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if capturedStatus != "available" {
		t.Errorf("Status = %q, want \"available\"", capturedStatus)
	}
}

func TestGetCustomerCoupons_GrpcError(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			return nil, fmt.Errorf("coupon connection refused")
		},
	}

	h := &Holder{Coupon: grpcclient.NewCouponClientForTest(couponMock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/coupons", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetCustomerCoupons_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/coupons", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestGetCustomerOrders_UsesResolvedSurrogate(t *testing.T) {
	var captured *orderpb.GetBuyerOrdersForSellerRequest
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			captured = in
			return &orderpb.GetBuyerOrdersForSellerResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: storeResolverForTest(7001), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != 7001 {
		t.Errorf("StoreId = %+v, want 7001 (resolved surrogate, not login id 99)", captured)
	}
}

// ② coupon 站點（GetCustomerCoupons → ListBuyerCouponsForSellerRequest.StoreId）
// 的 surrogate 回歸釘：handler 已於 R7 轉 resolver，這裡釘住 wire 值防回退。
func TestGetCustomerCoupons_UsesResolvedSurrogate(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			if in.StoreId != 777 {
				t.Errorf("StoreId = %d, want resolved surrogate 777, not JWT 99", in.StoreId)
			}
			return &couponpb.ListBuyerCouponsForSellerResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		Store:   storeResolverForTest(777),
		BuyerID: buyerGrandfatherResolverForTest(),
	}
	r := gin.New()
	r.GET("/api/v1/app/sellers/me/customers/:buyerId/coupons", fakeSellerAuth(), h.GetCustomerCoupons)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/123/coupons", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
}

// ② 第三方目標買家（賣家指定的 buyerId path 參數）就是 buyer_id 本身，原值
// 送進 order-service，不再翻成現任 stint。
func TestGetCustomerOrders_ForwardsPathBuyerIDVerbatim(t *testing.T) {
	var captured *orderpb.GetBuyerOrdersForSellerRequest
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			captured = in
			return &orderpb.GetBuyerOrdersForSellerResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	// resolver 會把任何輸入翻成 8001——handler 不該碰它。
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest(), BuyerID: buyerResolverForTest(8001)}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.BuyerId != 2688657200 {
		t.Errorf("BuyerId = %+v, want path param 2688657200 verbatim (must not be resolved to 8001)", captured)
	}
}

// 買家沒有現任 stint 曾經是 404 的來源（resolver 回 ErrNoCurrentBuyer）。新語意
// 下 gateway 根本不查 stint：舊 stint 的 buyer_id 是合法輸入，order-service 照
// 樣要被問。這支就是釘「resolver 完全不在路徑上」——若有人把解析加回來，
// noBuyerResolverForTest 會讓它變成 404 而被抓到。
func TestGetCustomerOrders_NoCurrentStint_StillQueriesOrderService(t *testing.T) {
	called := false
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			called = true
			if in.BuyerId != 2688657200 {
				t.Errorf("BuyerId = %d, want path param 2688657200 verbatim", in.BuyerId)
			}
			return &orderpb.GetBuyerOrdersForSellerResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderMock), Store: grandfatherResolverForTest(), BuyerID: noBuyerResolverForTest()}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/2688657200/orders", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if !called {
		t.Error("order-service must still be queried — the path buyer_id is used directly, no stint lookup involved")
	}
}

func TestGetCustomerCoupons_ForwardsPathBuyerIDVerbatim(t *testing.T) {
	const (
		staleStintBuyerID      = "55555" // 舊 stint 的 buyer_id，crm 列上掛的就是它
		currentButWrongBuyerID = int64(9999)
	)
	var captured string
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			captured = in.BuyerId
			return &couponpb.ListBuyerCouponsForSellerResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Coupon: grpcclient.NewCouponClientForTest(couponMock),
		Store:  grandfatherResolverForTest(),
		// 任何輸入都會被翻成 9999——新語意下 handler 不該呼叫 resolver
		BuyerID: buyerResolverForTest(currentButWrongBuyerID),
	}

	r := setupCrmRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/customers/"+staleStintBuyerID+"/coupons", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != staleStintBuyerID {
		t.Errorf("BuyerId = %q, want path value verbatim %q (must not be resolved to %d)", captured, staleStintBuyerID, currentButWrongBuyerID)
	}
}

func TestGetCustomerList_PageParamParsing(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPage     int32
		wantPageSize int32
	}{
		{"合法數字", "page=3&page_size=5", 3, 5},
		{"非數字字串 fallback 到 default", "page=abc&page_size=xyz", 1, 10},
		{"空值 fallback 到 default", "", 1, 10},
		{"負數 fallback 到 default", "page=-1&page_size=-5", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured *crmpb.GetCustomerListRequest
			mock := &shapeCrmMock{
				GetCustomerListFn: func(ctx context.Context, in *crmpb.GetCustomerListRequest, opts ...grpc.CallOption) (*crmpb.CustomerListResponse, error) {
					captured = in
					return &crmpb.CustomerListResponse{Code: 0, Data: &crmpb.CustomerListData{}}, nil
				},
			}
			h := &Holder{Crm: grpcclient.NewCrmClientForTest(mock), Store: grandfatherResolverForTest()}
			r := gin.New()
			v1 := r.Group("/api/v1/app/sellers/me")
			v1.Use(fakeSellerAuth())
			v1.GET("/customers", h.GetCustomerList)

			url := "/api/v1/app/sellers/me/customers"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Account-Id", "99")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusOK)
			if captured == nil {
				t.Fatal("GetCustomerList was not called")
			}
			if captured.Page != tt.wantPage || captured.PageSize != tt.wantPageSize {
				t.Errorf("Page/PageSize = %d/%d, want %d/%d", captured.Page, captured.PageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}
