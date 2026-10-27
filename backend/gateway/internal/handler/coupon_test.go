package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	pb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock CouponServiceClient ====================

type couponMockCouponSvcClient struct {
	ListMyCouponsFn                func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error)
	ListAvailableCouponsFn         func(ctx context.Context, in *pb.ListAvailableCouponsRequest, opts ...grpc.CallOption) (*pb.ListAvailableCouponsResponse, error)
	SellerCreateTemplateFn         func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error)
	SellerListTemplatesFn          func(ctx context.Context, in *pb.SellerListTemplatesRequest, opts ...grpc.CallOption) (*pb.SellerListTemplatesResponse, error)
	SellerGetTemplateDetailFn      func(ctx context.Context, in *pb.SellerGetTemplateDetailRequest, opts ...grpc.CallOption) (*pb.SellerGetTemplateDetailResponse, error)
	SellerRevokeTemplateFn         func(ctx context.Context, in *pb.SellerRevokeTemplateRequest, opts ...grpc.CallOption) (*pb.SellerRevokeTemplateResponse, error)
	ListBuyerCouponsForSellerFn    func(ctx context.Context, in *pb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*pb.ListBuyerCouponsForSellerResponse, error)
	BuyerClaimCouponFn             func(ctx context.Context, in *pb.BuyerClaimCouponRequest, opts ...grpc.CallOption) (*pb.BuyerClaimCouponResponse, error)
	AdminGrantCompensationCouponFn func(ctx context.Context, in *pb.AdminGrantCompensationCouponRequest, opts ...grpc.CallOption) (*pb.AdminGrantCompensationCouponResponse, error)
	AdminIssueCouponsFn            func(ctx context.Context, in *pb.AdminIssueCouponsRequest, opts ...grpc.CallOption) (*pb.AdminIssueCouponsResponse, error)
	AdminCreateTemplateFn          func(ctx context.Context, in *pb.AdminCreateTemplateRequest, opts ...grpc.CallOption) (*pb.AdminCreateTemplateResponse, error)
}

func (m *couponMockCouponSvcClient) ListMyCoupons(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
	if m.ListMyCouponsFn != nil {
		return m.ListMyCouponsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) ListAvailableCoupons(ctx context.Context, in *pb.ListAvailableCouponsRequest, opts ...grpc.CallOption) (*pb.ListAvailableCouponsResponse, error) {
	if m.ListAvailableCouponsFn != nil {
		return m.ListAvailableCouponsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) ValidateCoupon(ctx context.Context, in *pb.ValidateCouponRequest, opts ...grpc.CallOption) (*pb.ValidateCouponResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) ReserveCoupon(ctx context.Context, in *pb.ReserveCouponRequest, opts ...grpc.CallOption) (*pb.ReserveCouponResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) ConsumeCoupon(ctx context.Context, in *pb.ConsumeCouponRequest, opts ...grpc.CallOption) (*pb.ConsumeCouponResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) ReleaseCoupon(ctx context.Context, in *pb.ReleaseCouponRequest, opts ...grpc.CallOption) (*pb.ReleaseCouponResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) SellerCreateTemplate(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
	if m.SellerCreateTemplateFn != nil {
		return m.SellerCreateTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) SellerListTemplates(ctx context.Context, in *pb.SellerListTemplatesRequest, opts ...grpc.CallOption) (*pb.SellerListTemplatesResponse, error) {
	if m.SellerListTemplatesFn != nil {
		return m.SellerListTemplatesFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) SellerGetTemplateDetail(ctx context.Context, in *pb.SellerGetTemplateDetailRequest, opts ...grpc.CallOption) (*pb.SellerGetTemplateDetailResponse, error) {
	if m.SellerGetTemplateDetailFn != nil {
		return m.SellerGetTemplateDetailFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) SellerRevokeTemplate(ctx context.Context, in *pb.SellerRevokeTemplateRequest, opts ...grpc.CallOption) (*pb.SellerRevokeTemplateResponse, error) {
	if m.SellerRevokeTemplateFn != nil {
		return m.SellerRevokeTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) ListBuyerCouponsForSeller(ctx context.Context, in *pb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*pb.ListBuyerCouponsForSellerResponse, error) {
	if m.ListBuyerCouponsForSellerFn != nil {
		return m.ListBuyerCouponsForSellerFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminCreateTemplate(ctx context.Context, in *pb.AdminCreateTemplateRequest, opts ...grpc.CallOption) (*pb.AdminCreateTemplateResponse, error) {
	if m.AdminCreateTemplateFn != nil {
		return m.AdminCreateTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminPublishTemplate(ctx context.Context, in *pb.AdminPublishTemplateRequest, opts ...grpc.CallOption) (*pb.AdminPublishTemplateResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminDisableTemplate(ctx context.Context, in *pb.AdminDisableTemplateRequest, opts ...grpc.CallOption) (*pb.AdminDisableTemplateResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminIssueCoupons(ctx context.Context, in *pb.AdminIssueCouponsRequest, opts ...grpc.CallOption) (*pb.AdminIssueCouponsResponse, error) {
	if m.AdminIssueCouponsFn != nil {
		return m.AdminIssueCouponsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) BuyerClaimCoupon(ctx context.Context, in *pb.BuyerClaimCouponRequest, opts ...grpc.CallOption) (*pb.BuyerClaimCouponResponse, error) {
	if m.BuyerClaimCouponFn != nil {
		return m.BuyerClaimCouponFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminDistributeTemplate(ctx context.Context, in *pb.AdminDistributeTemplateRequest, opts ...grpc.CallOption) (*pb.AdminDistributeTemplateResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminGrantCompensationCoupon(ctx context.Context, in *pb.AdminGrantCompensationCouponRequest, opts ...grpc.CallOption) (*pb.AdminGrantCompensationCouponResponse, error) {
	if m.AdminGrantCompensationCouponFn != nil {
		return m.AdminGrantCompensationCouponFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminVoidInstance(ctx context.Context, in *pb.AdminVoidInstanceRequest, opts ...grpc.CallOption) (*pb.AdminVoidInstanceResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) AdminUpdateSellerQuota(ctx context.Context, in *pb.AdminUpdateSellerQuotaRequest, opts ...grpc.CallOption) (*pb.AdminUpdateSellerQuotaResponse, error) {
	return nil, nil
}
func (m *couponMockCouponSvcClient) GrantSellerCompensationCoupon(ctx context.Context, in *pb.GrantSellerCompensationCouponRequest, opts ...grpc.CallOption) (*pb.GrantSellerCompensationCouponResponse, error) {
	return nil, nil
}

// ==================== Fake Auth Middleware ====================

// fakeBuyerAuth sets accountId and accountType=1 (BUYER)
func fakeBuyerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.GetHeader("X-Account-Id")
		if idStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "msg": "missing token"})
			return
		}
		var accountID int64
		for _, ch := range idStr {
			accountID = accountID*10 + int64(ch-'0')
		}
		c.Set("accountId", accountID)
		c.Set("accountType", 1) // BUYER
		c.Next()
	}
}

// fakeAdminAuth 模擬已登入的 admin caller（測試用）。AdminGrantCompensation/
// AdminIssueCouponsHandler 只讀 accountId，角色把關在 route 層 middleware，
// handler 本身不檢查 accountType，故沿用跟 fakeBuyerAuth 相同的 accountId 抽取
// 邏輯即可。
func fakeAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.GetHeader("X-Account-Id")
		if idStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "msg": "missing token"})
			return
		}
		var accountID int64
		for _, ch := range idStr {
			accountID = accountID*10 + int64(ch-'0')
		}
		c.Set("accountId", accountID)
		c.Set("accountType", 4) // ADMIN
		c.Next()
	}
}

// ==================== Buyer Resolver Test Helpers ====================
//
// buyerResolverForTest / buyerGrandfatherResolverForTest 定義於
// resolve_seller_test.go；noBuyerResolverForTest 定義於 resolve_buyer_test.go
// （② 買家 store-stint re-key W2 新增的共用 helper，多個 sibling
// gateway-wiring task 都會用到；package 內已存在，這裡不重複定義）。這裡只補
// 這個測試檔特有的 offsetBuyerResolverForTest。

// offsetBuyerResolverForTest 讓每個 accountID 解析出不同的 surrogate
// （accountID+offset），用於證明批次解析（resolveTargetBuyerIDs）逐筆
// 呼叫 resolver、不是整批共用同一個值。
func offsetBuyerResolverForTest(offset int64) *buyerid.Resolver {
	return buyerid.New(buyerid.GetterFunc(func(_ context.Context, accountID int64) (*buyerpb.BuyerInfo, error) {
		return &buyerpb.BuyerInfo{
			Id:        strconv.FormatInt(accountID+offset, 10),
			AccountId: strconv.FormatInt(accountID, 10),
		}, nil
	}))
}

// ==================== Router Setup ====================

func setupCouponRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeBuyerAuth())
	{
		app.GET("/coupons", h.GetCoupons)
		app.GET("/coupons/available", h.ListAvailableCoupons)
		app.POST("/coupons/:template_id/claim", h.ClaimCoupon)
	}
	return r
}

// setupAdminCouponRouter 建立 admin 優惠券路由（AdminGrantCompensation /
// AdminIssueCouponsHandler）的測試 Router。
func setupAdminCouponRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	admin := v1.Group("/admin")
	admin.Use(fakeAdminAuth())
	{
		admin.POST("/coupons/compensations", h.AdminGrantCompensation)
		admin.POST("/coupon_templates", h.AdminCreateCouponTemplate)
		admin.POST("/coupon_templates/:template_id/issue", h.AdminIssueCouponsHandler)
	}
	return r
}

func TestAdminCreateCouponTemplate_ResolvesBuyerAccountIds(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		AdminCreateTemplateFn: func(ctx context.Context, in *pb.AdminCreateTemplateRequest, opts ...grpc.CallOption) (*pb.AdminCreateTemplateResponse, error) {
			want := []string{"1200", "1201"}
			if len(in.BuyerIds) != len(want) {
				t.Fatalf("BuyerIds = %v, want %v", in.BuyerIds, want)
			}
			for i, id := range in.BuyerIds {
				if id != want[i] {
					t.Errorf("BuyerIds[%d] = %q, want %q (resolved buyer_id, not raw account_id)", i, id, want[i])
				}
			}
			return &pb.AdminCreateTemplateResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL200"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":           "管理員指定券",
		"type":            "CASH",
		"targetType":      "DESIGNATED",
		"discountAmount":  "50",
		"startAt":         "2026-04-01T00:00:00Z",
		"endAt":           "2026-04-07T23:59:59Z",
		"buyerAccountIds": []string{"200", "201"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

// ==================== GetCoupons (ListMyCoupons) ====================

func TestGetCoupons_Success(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &pb.ListMyCouponsResponse{
				Code: 0,
				Records: []*pb.TemplateInfo{
					{
						TemplateId:     "TPL001",
						Title:          "滿500折50",
						Type:           "CASH",
						DiscountAmount: "50",
						Status:         "ACTIVE",
					},
					{
						TemplateId:      "TPL002",
						Title:           "全站9折券",
						Type:            "PERCENT",
						DiscountPercent: "10",
						Status:          "ACTIVE",
					},
				},
				UsedRecords: []*pb.InstanceInfo{
					{
						InstanceId: "INS001",
						TemplateId: "TPL001",
						Title:      "滿500折50",
						Type:       "CASH",
						Status:     "USED",
					},
				},
				Total:    "2",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	records, ok := data["records"].([]interface{})
	if !ok {
		t.Fatal("missing records field")
	}
	if len(records) != 2 {
		t.Errorf("records count = %d, want 2", len(records))
	}
	usedRecords, ok := data["usedRecords"].([]interface{})
	if !ok {
		t.Fatal("missing usedRecords field")
	}
	if len(usedRecords) != 1 {
		t.Errorf("usedRecords count = %d, want 1", len(usedRecords))
	}
	if data["total"] != "2" {
		t.Errorf("total = %v, want 2", data["total"])
	}
}

func TestGetCoupons_IssuerStoreId_PassThrough(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			return &pb.ListMyCouponsResponse{
				Code: 0,
				Records: []*pb.TemplateInfo{
					{
						TemplateId:      "TPL001",
						Title:           "週末優惠",
						Type:            "CASH",
						DiscountAmount:  "50",
						Status:          "ACTIVE",
						IssuerType:      "2",
						IssuerStoreId:   "555",
						IssuerAccountId: "",
					},
				},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatal("expected exactly 1 record")
	}
	record := records[0].(map[string]interface{})
	if record["issuerStoreId"] != "555" {
		t.Errorf("issuerStoreId = %v, want 555", record["issuerStoreId"])
	}
	if record["issuerAccountId"] != "" {
		t.Errorf("issuerAccountId = %v, want empty string", record["issuerAccountId"])
	}
}

func TestGetCoupons_GrpcError(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetCoupons_Unauthorized(t *testing.T) {
	h := &Holder{}
	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

// ==================== ListAvailableCoupons ====================

func TestListAvailableCoupons_Success(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListAvailableCouponsFn: func(ctx context.Context, in *pb.ListAvailableCouponsRequest, opts ...grpc.CallOption) (*pb.ListAvailableCouponsResponse, error) {
			if in.BuyerId != 100 {
				t.Errorf("BuyerId = %d, want 100", in.BuyerId)
			}
			return &pb.ListAvailableCouponsResponse{
				Code: 0,
				Records: []*pb.AvailableCouponInfo{
					{
						Template: &pb.TemplateInfo{
							TemplateId:     "TPL001",
							Title:          "新客滿300折30",
							Type:           "CASH",
							DiscountAmount: "30",
							Status:         "ACTIVE",
						},
					},
				},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons/available?store_id=200&order_amount=500", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	records, ok := data["records"].([]interface{})
	if !ok {
		t.Fatal("missing records field")
	}
	if len(records) != 1 {
		t.Errorf("records count = %d, want 1", len(records))
	}
}

func TestListAvailableCoupons_WithSellerFilter(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListAvailableCouponsFn: func(ctx context.Context, in *pb.ListAvailableCouponsRequest, opts ...grpc.CallOption) (*pb.ListAvailableCouponsResponse, error) {
			if in.StoreId != "200" {
				t.Errorf("StoreId = %q, want 200", in.StoreId)
			}
			return &pb.ListAvailableCouponsResponse{
				Code:     0,
				Records:  []*pb.AvailableCouponInfo{},
				Total:    "0",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons/available?store_id=200&order_amount=500", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

// ==================== CreateCouponTemplate (Seller) ====================

func setupSellerCouponRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeSellerAuth())
	{
		app.POST("/coupon_templates", h.CreateCouponTemplate)
		app.GET("/coupon_templates", h.ListCouponTemplates)
		app.GET("/coupon_templates/:template_id", h.GetCouponTemplateDetail)
		app.POST("/coupon_templates/:template_id/revoke", h.RevokeCouponTemplate)
	}
	return r
}

func TestCreateCouponTemplate_Success(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerCreateTemplateFn: func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
			if in.StoreId != 555 {
				t.Errorf("StoreId = %d, want 555 (resolved store_id, not raw X-Account-Id 100)", in.StoreId)
			}
			if in.Title != "週末限定折扣" {
				t.Errorf("Title = %q, want 週末限定折扣", in.Title)
			}
			if in.TargetType != "DESIGNATED" {
				t.Errorf("TargetType = %q, want DESIGNATED", in.TargetType)
			}
			return &pb.SellerCreateTemplateResponse{
				Code: 0,
				Data: &pb.TemplateInfo{TemplateId: "TPL100"},
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Store:   storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":           "週末限定折扣",
		"type":            "CASH",
		"targetType":      "DESIGNATED",
		"discountAmount":  "100",
		"startAt":         "2026-04-01T00:00:00Z",
		"endAt":           "2026-04-07T23:59:59Z",
		"buyerAccountIds": []string{"200", "201", "202"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["templateId"] != "TPL100" {
		t.Errorf("templateId = %v, want TPL100", data["templateId"])
	}
}

func TestCreateCouponTemplate_BadRequest_MissingTitle(t *testing.T) {
	// SellerID must be set: CreateCouponTemplate now resolves store_id before
	// binding the request body, so a nil resolver would panic before reaching
	// the missing-title validation this test actually checks.
	h := &Holder{Store: grandfatherResolverForTest()}
	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"type":            "CASH",
		"targetType":      "DESIGNATED",
		"startAt":         "2026-04-01T00:00:00Z",
		"endAt":           "2026-04-07T23:59:59Z",
		"buyerAccountIds": []string{"200"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

// ==================== ListCouponTemplates (Seller) ====================

func TestListCouponTemplates_Success(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerListTemplatesFn: func(ctx context.Context, in *pb.SellerListTemplatesRequest, opts ...grpc.CallOption) (*pb.SellerListTemplatesResponse, error) {
			if in.StoreId != 555 {
				t.Errorf("StoreId = %d, want 555 (resolved store_id, not raw X-Account-Id 100)", in.StoreId)
			}
			return &pb.SellerListTemplatesResponse{
				Code: 0,
				Records: []*pb.TemplateInfo{
					{
						TemplateId:     "TPL001",
						Title:          "新春優惠",
						Type:           "CASH",
						DiscountAmount: "100",
						Status:         "ACTIVE",
					},
				},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Coupon: grpcclient.NewCouponClientForTest(couponMock),
		Store:  storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupon_templates", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	records, ok := data["records"].([]interface{})
	if !ok {
		t.Fatal("missing records field")
	}
	if len(records) != 1 {
		t.Errorf("records count = %d, want 1", len(records))
	}
}

// ==================== GetCouponTemplateDetail (Seller) ====================

func TestGetCouponTemplateDetail_ResolvesStoreId(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerGetTemplateDetailFn: func(ctx context.Context, in *pb.SellerGetTemplateDetailRequest, opts ...grpc.CallOption) (*pb.SellerGetTemplateDetailResponse, error) {
			if in.StoreId != 555 {
				t.Errorf("StoreId = %d, want 555 (resolved store_id, not raw X-Account-Id 100)", in.StoreId)
			}
			if in.TemplateId != "TPL001" {
				t.Errorf("TemplateId = %q, want TPL001", in.TemplateId)
			}
			return &pb.SellerGetTemplateDetailResponse{
				Code:          0,
				Data:          &pb.TemplateInfo{TemplateId: "TPL001"},
				UsedCount:     "2",
				ReservedCount: "1",
			}, nil
		},
	}
	h := &Holder{
		Coupon: grpcclient.NewCouponClientForTest(couponMock),
		Store:  storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupon_templates/TPL001", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

// ==================== RevokeCouponTemplate (Seller) ====================

func TestRevokeCouponTemplate_ResolvesStoreId(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerRevokeTemplateFn: func(ctx context.Context, in *pb.SellerRevokeTemplateRequest, opts ...grpc.CallOption) (*pb.SellerRevokeTemplateResponse, error) {
			if in.StoreId != 555 {
				t.Errorf("StoreId = %d, want 555 (resolved store_id, not raw X-Account-Id 100)", in.StoreId)
			}
			if in.TemplateId != "TPL001" {
				t.Errorf("TemplateId = %q, want TPL001", in.TemplateId)
			}
			return &pb.SellerRevokeTemplateResponse{Code: 0, RevokedInstanceCount: "3"}, nil
		},
	}
	h := &Holder{
		Coupon: grpcclient.NewCouponClientForTest(couponMock),
		Store:  storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{"reason": "測試收回"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates/TPL001/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

// ==================== sellerAvatarUrl 欄位透傳 ====================

func TestGetCoupons_SellerAvatarKey_PassThrough(t *testing.T) {
	const avatarURL = "https://cdn.example.com/seller/avatar.jpg"
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			return &pb.ListMyCouponsResponse{
				Code: 0,
				Records: []*pb.TemplateInfo{
					{
						TemplateId:        "TPL001",
						Title:             "商家券",
						Type:              "CASH",
						DiscountAmount:    "50",
						Status:            "ACTIVE",
						ApplicableStoreId: "9999",
						SellerName:        "測試商家",
						SellerAvatarKey:   avatarURL,
					},
				},
				UsedRecords: []*pb.InstanceInfo{
					{
						InstanceId:        "INS001",
						TemplateId:        "TPL001",
						Title:             "商家券",
						Type:              "CASH",
						Status:            "USED",
						ApplicableStoreId: "9999",
						SellerName:        "測試商家",
						SellerAvatarKey:   avatarURL,
					},
				},
				Total: "1", Page: "1", PageSize: "20",
			}, nil
		},
	}
	h := &Holder{Coupon: grpcclient.NewCouponClientForTest(couponMock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			Records []struct {
				SellerAvatarKey string `json:"logoUrl"`
			} `json:"records"`
			UsedRecords []struct {
				SellerAvatarKey string `json:"logoUrl"`
			} `json:"usedRecords"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body.Data.Records[0].SellerAvatarKey; got != avatarURL {
		t.Errorf("records[0].logoUrl = %q, want %q", got, avatarURL)
	}
	if got := body.Data.UsedRecords[0].SellerAvatarKey; got != avatarURL {
		t.Errorf("usedRecords[0].logoUrl = %q, want %q", got, avatarURL)
	}
}

func TestGetCoupons_SellerAvatarKey_EmptyForPlatformCoupon(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			return &pb.ListMyCouponsResponse{
				Code: 0,
				Records: []*pb.TemplateInfo{
					{TemplateId: "TPL001", Title: "全平台券", Type: "CASH", DiscountAmount: "30", Status: "ACTIVE"},
				},
				UsedRecords: []*pb.InstanceInfo{},
				Total:       "1", Page: "1", PageSize: "20",
			}, nil
		},
	}
	h := &Holder{Coupon: grpcclient.NewCouponClientForTest(couponMock), BuyerID: buyerGrandfatherResolverForTest()}
	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	var body struct {
		Data struct {
			Records []struct {
				SellerAvatarKey string `json:"logoUrl"`
			} `json:"records"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body.Data.Records[0].SellerAvatarKey; got != "" {
		t.Errorf("records[0].logoUrl = %q, want empty for platform coupon", got)
	}
}

// ==================== ② buyer store-stint re-key W2/W3: PR1 gateway wiring ====================
//
// gateway/internal/buyerid.Resolver 已在 W1（PR #1571）建好骨架，但只接進
// notification.go/notification_inbox.go/resolve_seller.go 三支 handler。以下
// 補 coupon-service 買家相關 call site（設計書
// docs/superpowers/specs/2026-07-16-buyer-store-stint-rekey-w2w3-design.md §3.1）。

// ---- handleListMyCoupons / GetCoupons（呼叫者自身身分）----

func TestGetCoupons_ResolvesBuyerID(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *pb.ListMyCouponsRequest, opts ...grpc.CallOption) (*pb.ListMyCouponsResponse, error) {
			if in.BuyerId != 9999 {
				t.Errorf("BuyerId = %d, want 9999 (resolved buyer_id, not raw account_id 100)", in.BuyerId)
			}
			if in.AccountId != 100 {
				t.Errorf("AccountId = %d, want 100 (raw JWT account_id, not resolved buyer_id 9999)", in.AccountId)
			}
			return &pb.ListMyCouponsResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestGetCoupons_NoCurrentBuyer_Returns403(t *testing.T) {
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(&couponMockCouponSvcClient{}),
		BuyerID: noBuyerResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 403)
	if resp["message"] != "buyer access required" {
		t.Errorf("message = %v, want %q", resp["message"], "buyer access required")
	}
}

// ---- ListAvailableCoupons（呼叫者自身身分）----

func TestListAvailableCoupons_ResolvesBuyerID(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListAvailableCouponsFn: func(ctx context.Context, in *pb.ListAvailableCouponsRequest, opts ...grpc.CallOption) (*pb.ListAvailableCouponsResponse, error) {
			if in.BuyerId != 9999 {
				t.Errorf("BuyerId = %d, want 9999 (resolved buyer_id, not raw account_id 100)", in.BuyerId)
			}
			if in.AccountId != 100 {
				t.Errorf("AccountId = %d, want 100 (raw JWT account_id, not resolved buyer_id 9999)", in.AccountId)
			}
			return &pb.ListAvailableCouponsResponse{Code: 0, Records: []*pb.AvailableCouponInfo{}, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons/available?store_id=200&order_amount=500", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

// ---- ClaimCoupon（呼叫者自身身分；先前無測試覆蓋）----

func TestClaimCoupon_ResolvesBuyerID(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		BuyerClaimCouponFn: func(ctx context.Context, in *pb.BuyerClaimCouponRequest, opts ...grpc.CallOption) (*pb.BuyerClaimCouponResponse, error) {
			if in.BuyerId != 9999 {
				t.Errorf("BuyerId = %d, want 9999 (resolved buyer_id, not raw account_id 100)", in.BuyerId)
			}
			if in.AccountId != 100 {
				t.Errorf("AccountId = %d, want 100 (raw JWT account_id, not resolved buyer_id 9999)", in.AccountId)
			}
			if in.TemplateId != "TPL001" {
				t.Errorf("TemplateId = %q, want TPL001", in.TemplateId)
			}
			return &pb.BuyerClaimCouponResponse{Code: 0, Success: true, ClaimedAt: "2026-07-16T00:00:00Z"}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupons/TPL001/claim", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

func TestClaimCoupon_NoCurrentBuyer_Returns403(t *testing.T) {
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(&couponMockCouponSvcClient{}),
		BuyerID: noBuyerResolverForTest(),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupons/TPL001/claim", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 403)
}

// ---- handleListBuyerCouponsForSeller（第三方目標買家：query param）----

func TestGetCoupons_SellerView_RejectsLegacyBuyerAccountIDQuery(t *testing.T) {
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(&couponMockCouponSvcClient{}),
		BuyerID: mustNotResolveBuyer(t),
		Store:   storeResolverForTest(555),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons?view=seller&buyer_account_id=500", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

func TestGetCoupons_SellerView_BuyerIDDirect_NoDoubleResolve(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(ctx context.Context, in *pb.ListBuyerCouponsForSellerRequest, opts ...grpc.CallOption) (*pb.ListBuyerCouponsForSellerResponse, error) {
			if in.BuyerId != "9999" {
				t.Errorf("BuyerId = %q, want 9999 (buyer_id passed straight through, not re-resolved)", in.BuyerId)
			}
			return &pb.ListBuyerCouponsForSellerResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: mustNotResolveBuyer(t),
		Store:   storeResolverForTest(555),
	}

	router := setupCouponRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/coupons?view=seller&buyer_id=9999", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

// ---- AdminGrantCompensation（第三方目標買家：body 欄位）----

func TestAdminGrantCompensation_ResolvesTargetBuyerID(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		AdminGrantCompensationCouponFn: func(ctx context.Context, in *pb.AdminGrantCompensationCouponRequest, opts ...grpc.CallOption) (*pb.AdminGrantCompensationCouponResponse, error) {
			if in.BuyerId != "9999" {
				t.Errorf("BuyerId = %q, want 9999 (resolved buyer_id, not raw account_id 500)", in.BuyerId)
			}
			if in.ApplicableStoreId != "2685914682" {
				t.Errorf("ApplicableStoreId = %q, want 2685914682 (原樣轉送，不得遺失)", in.ApplicableStoreId)
			}
			return &pb.AdminGrantCompensationCouponResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL_COMP"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"buyerAccountId":    "500",
		"amount":            "100",
		"applicableStoreId": "2685914682",
		"reason":            "退款補償",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons/compensations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

func TestAdminGrantCompensation_TargetNoCurrentBuyer_ReturnsBadRequest(t *testing.T) {
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(&couponMockCouponSvcClient{}),
		BuyerID: noBuyerResolverForTest(),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"buyerAccountId":    "500",
		"amount":            "100",
		"applicableStoreId": "2685914682",
		"reason":            "退款補償",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons/compensations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// 缺 applicableStoreId 曾經回 200 並建出「全平台任何商店都能用」的補償券——
// 一個打錯的欄位名就能把限店券放大成全平台券，且無任何錯誤訊號。這兩個 case
// 釘住：範圍必須是顯式指定的，缺值一律 400 且不得打到下游。
func TestAdminGrantCompensation_MissingApplicableStoreID_ReturnsBadRequest(t *testing.T) {
	called := false
	couponMock := &couponMockCouponSvcClient{
		AdminGrantCompensationCouponFn: func(ctx context.Context, in *pb.AdminGrantCompensationCouponRequest, opts ...grpc.CallOption) (*pb.AdminGrantCompensationCouponResponse, error) {
			called = true
			return &pb.AdminGrantCompensationCouponResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL_COMP"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"buyerAccountId": "500",
		"amount":         "100",
		"reason":         "退款補償",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons/compensations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if called {
		t.Error("缺 applicableStoreId 不得打到 coupon-service（會建出全平台補償券）")
	}
}

// 舊鍵 applicableSellerAccountId（已於 3db384ae2 正名）送上來時，Gin 會忽略它 →
// 新鍵仍為空 → 400。注意判定理由是「新鍵缺失」，gateway 不認得也不該認得舊鍵。
func TestAdminGrantCompensation_LegacySellerAccountIDKey_ReturnsBadRequest(t *testing.T) {
	called := false
	couponMock := &couponMockCouponSvcClient{
		AdminGrantCompensationCouponFn: func(ctx context.Context, in *pb.AdminGrantCompensationCouponRequest, opts ...grpc.CallOption) (*pb.AdminGrantCompensationCouponResponse, error) {
			called = true
			return &pb.AdminGrantCompensationCouponResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL_COMP"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerResolverForTest(9999),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"buyerAccountId":            "500",
		"amount":                    "100",
		"applicableSellerAccountId": "2685914682",
		"reason":                    "退款補償",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons/compensations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if called {
		t.Error("舊鍵不算指定範圍，不得打到 coupon-service")
	}
}

// ---- CreateCouponTemplate（第三方目標買家清單：body 陣列）----

func TestCreateCouponTemplate_ResolvesBuyerAccountIds(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerCreateTemplateFn: func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
			want := []string{"1200", "1201", "1202"}
			if len(in.BuyerIds) != len(want) {
				t.Fatalf("BuyerIds = %v, want %v", in.BuyerIds, want)
			}
			for i, id := range in.BuyerIds {
				if id != want[i] {
					t.Errorf("BuyerIds[%d] = %q, want %q (resolved buyer_id, not raw account_id)", i, id, want[i])
				}
			}
			return &pb.SellerCreateTemplateResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL100"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
		Store:   storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":           "週末限定折扣",
		"type":            "CASH",
		"targetType":      "DESIGNATED",
		"discountAmount":  "100",
		"startAt":         "2026-04-01T00:00:00Z",
		"endAt":           "2026-04-07T23:59:59Z",
		"buyerAccountIds": []string{"200", "201", "202"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

// ---- CreateCouponTemplate 的 buyerIds（已解析 buyer_id，跟 buyerAccountIds
// 並存不互斥；CRM 顧客列表/標籤搜尋只吐得出 buyer_id，見 2026-07-30 事故）----

func TestCreateCouponTemplate_AcceptsAlreadyResolvedBuyerIds(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerCreateTemplateFn: func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
			want := []string{"9999"}
			if len(in.BuyerIds) != len(want) {
				t.Fatalf("BuyerIds = %v, want %v", in.BuyerIds, want)
			}
			for i, id := range in.BuyerIds {
				if id != want[i] {
					t.Errorf("BuyerIds[%d] = %q, want %q (passed through verbatim, not re-resolved)", i, id, want[i])
				}
			}
			return &pb.SellerCreateTemplateResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL100"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
		Store:   storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":          "已解析 buyer_id 發券",
		"type":           "CASH",
		"targetType":     "DESIGNATED",
		"discountAmount": "100",
		"startAt":        "2026-04-01T00:00:00Z",
		"endAt":          "2026-04-07T23:59:59Z",
		"buyerIds":       []string{"9999"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestCreateCouponTemplate_MergesBuyerAccountIdsAndBuyerIds(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerCreateTemplateFn: func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
			want := []string{"1200", "9999"}
			if len(in.BuyerIds) != len(want) {
				t.Fatalf("BuyerIds = %v, want %v", in.BuyerIds, want)
			}
			for i, id := range in.BuyerIds {
				if id != want[i] {
					t.Errorf("BuyerIds[%d] = %q, want %q", i, id, want[i])
				}
			}
			return &pb.SellerCreateTemplateResponse{Code: 0, Data: &pb.TemplateInfo{TemplateId: "TPL100"}}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
		Store:   storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":           "混合來源發券",
		"type":            "CASH",
		"targetType":      "DESIGNATED",
		"discountAmount":  "100",
		"startAt":         "2026-04-01T00:00:00Z",
		"endAt":           "2026-04-07T23:59:59Z",
		"buyerAccountIds": []string{"200"},
		"buyerIds":        []string{"9999"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestCreateCouponTemplate_InvalidBuyerIds_BadRequest(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		SellerCreateTemplateFn: func(ctx context.Context, in *pb.SellerCreateTemplateRequest, opts ...grpc.CallOption) (*pb.SellerCreateTemplateResponse, error) {
			t.Fatal("SellerCreateTemplate should not be called for an invalid buyerIds entry")
			return nil, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
		Store:   storeResolverForTest(555),
	}

	router := setupSellerCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"title":          "不合法 buyerIds",
		"type":           "CASH",
		"targetType":     "DESIGNATED",
		"discountAmount": "100",
		"startAt":        "2026-04-01T00:00:00Z",
		"endAt":          "2026-04-07T23:59:59Z",
		"buyerIds":       []string{"not-a-number"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/coupon_templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// ---- AdminIssueCouponsHandler（第三方目標買家清單：body 陣列；先前無測試覆蓋）----

func TestAdminIssueCoupons_ResolvesBuyerAccountIds(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		AdminIssueCouponsFn: func(ctx context.Context, in *pb.AdminIssueCouponsRequest, opts ...grpc.CallOption) (*pb.AdminIssueCouponsResponse, error) {
			want := []string{"1200", "1201"}
			if len(in.BuyerIds) != len(want) {
				t.Fatalf("BuyerIds = %v, want %v", in.BuyerIds, want)
			}
			for i, id := range in.BuyerIds {
				if id != want[i] {
					t.Errorf("BuyerIds[%d] = %q, want %q (resolved buyer_id, not raw account_id)", i, id, want[i])
				}
			}
			if in.TemplateId != "TPL100" {
				t.Errorf("TemplateId = %q, want TPL100", in.TemplateId)
			}
			return &pb.AdminIssueCouponsResponse{Code: 0, IssuedCount: "2"}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: offsetBuyerResolverForTest(1000),
	}

	router := setupAdminCouponRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"buyerAccountIds": []string{"200", "201"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupon_templates/TPL100/issue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}
