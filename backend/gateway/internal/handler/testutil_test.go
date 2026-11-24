package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ==================== Router Setup ====================

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeJWTAuth is a minimal middleware that extracts accountId and accountType from headers
// (set by tests), like middleware.JWTAuth trusting the monolith's identity headers.
func fakeJWTAuth() gin.HandlerFunc {
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
		if typeStr := c.GetHeader("X-Account-Type"); typeStr != "" {
			var accountType int
			for _, ch := range typeStr {
				accountType = accountType*10 + int(ch-'0')
			}
			c.Set("accountType", accountType)
		}
		if roleStr := c.GetHeader("X-Login-Role"); roleStr != "" {
			var loginRole int
			for _, ch := range roleStr {
				loginRole = loginRole*10 + int(ch-'0')
			}
			c.Set("loginRole", loginRole)
		}
		c.Next()
	}
}

// ==================== JSON Helpers ====================

func parseJSON(t *testing.T, body io.Reader) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	return m
}

func assertCode(t *testing.T, body map[string]interface{}, expected int) {
	t.Helper()
	got, ok := body["code"].(float64)
	if !ok {
		t.Fatalf("response missing 'code' field or not a number: %v", body)
	}
	if int(got) != expected {
		t.Errorf("code = %d, want %d; body = %v", int(got), expected, body)
	}
}

func assertHTTPStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("HTTP status = %d, want %d; body = %s", w.Code, expected, w.Body.String())
	}
}

// pageData 取分頁 envelope 的 data 物件，並斷言標準四欄（records/total/page/pageSize）
// 全都在 data 之下、且 total/page/pageSize 為字串（10 檔 §2 統一 Envelope ＋ §4 對外數字
// 一律 string）。頂層一旦出現任一分頁欄位即視為 Page-trap 回歸，直接 fail。
func pageData(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	for _, k := range []string{"records", "total", "page", "pageSize", "pageSize"} {
		if _, leaked := body[k]; leaked {
			t.Fatalf("envelope 頂層不得出現分頁欄位 %q（Page-trap 回歸）；body = %v", k, body)
		}
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object; body = %v", body)
	}
	for _, k := range []string{"total", "page", "pageSize"} {
		if _, ok := data[k].(string); !ok {
			t.Fatalf("data.%s 必須是字串（§4）；got %T = %v", k, data[k], data[k])
		}
	}
	return data
}

// pageRecords 取分頁 envelope 的 data.records 陣列（先過 pageData 的形狀斷言）。
func pageRecords(t *testing.T, body map[string]interface{}) []interface{} {
	t.Helper()
	data := pageData(t, body)
	rows, ok := data["records"].([]interface{})
	if !ok {
		t.Fatalf("data.records is not an array; data = %v", data)
	}
	return rows
}

// ==================== Mock SellerServiceClient ====================

type mockSellerServiceClient struct {
	MatchCertificateHolderFn func(context.Context, *sellerpb.MatchCertificateHolderRequest, ...grpc.CallOption) (*sellerpb.MatchCertificateHolderResponse, error)
	// SearchSellerProspects：Gateway 不呼叫，mock 只是為了滿足 SellerServiceClient 介面。
	SearchSellerProspectsFn    func(context.Context, *sellerpb.SearchSellerProspectsRequest, ...grpc.CallOption) (*sellerpb.SearchSellerProspectsResponse, error)
	GetSellerFn                func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error)
	GetSellerStatsFn           func(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error)
	BatchGetSellerMetaFn       func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error)
	GetSellerBatchFn           func(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error)
	GetCurrentStoreIDFn        func(ctx context.Context, in *sellerpb.GetCurrentStoreIDRequest, opts ...grpc.CallOption) (*sellerpb.GetCurrentStoreIDResponse, error)
	ListPublicStoreIDsByCityFn func(ctx context.Context, in *sellerpb.ListPublicStoreIDsByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoreIDsByCityResponse, error)
	ListPublicStoresByCityFn   func(ctx context.Context, in *sellerpb.ListPublicStoresByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoresByCityResponse, error)
}

func (m *mockSellerServiceClient) GetSeller(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	if m.GetSellerFn != nil {
		return m.GetSellerFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.NotFound, "seller not found")
}
func (m *mockSellerServiceClient) GetSellerBatch(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
	if m.GetSellerBatchFn != nil {
		return m.GetSellerBatchFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) ListPublicStoreIDsByCity(ctx context.Context, in *sellerpb.ListPublicStoreIDsByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoreIDsByCityResponse, error) {
	if m.ListPublicStoreIDsByCityFn != nil {
		return m.ListPublicStoreIDsByCityFn(ctx, in, opts...)
	}
	return &sellerpb.ListPublicStoreIDsByCityResponse{}, nil
}

func (m *mockSellerServiceClient) ListPublicStoresByCity(ctx context.Context, in *sellerpb.ListPublicStoresByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoresByCityResponse, error) {
	if m.ListPublicStoresByCityFn != nil {
		return m.ListPublicStoresByCityFn(ctx, in, opts...)
	}
	return &sellerpb.ListPublicStoresByCityResponse{}, nil
}
func (m *mockSellerServiceClient) GetSellerStats(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error) {
	if m.GetSellerStatsFn != nil {
		return m.GetSellerStatsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSellerServiceClient) ResolvePrincipals(ctx context.Context, in *sellerpb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListStoreIDsByAccount(ctx context.Context, in *sellerpb.ListStoreIDsByAccountRequest, opts ...grpc.CallOption) (*sellerpb.ListStoreIDsByAccountResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) BatchGetSellerMeta(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
	if m.BatchGetSellerMetaFn != nil {
		return m.BatchGetSellerMetaFn(ctx, in, opts...)
	}
	sellers := make(map[int64]*sellerpb.SellerMeta, len(in.StoreIds))
	for _, id := range in.StoreIds {
		sellers[id] = &sellerpb.SellerMeta{CanAcceptOrders: true, HasPublicCard: true}
	}
	return &sellerpb.BatchGetSellerMetaResponse{Sellers: sellers}, nil
}
func (m *mockSellerServiceClient) GetSellerShippingProfile(ctx context.Context, in *sellerpb.GetSellerShippingProfileRequest, opts ...grpc.CallOption) (*sellerpb.GetSellerShippingProfileResponse, error) {
	return nil, nil
}

// ---- B-169 governance / food-safety / appeal stubs (added with PR #727) ----
// 這 18 個 RPC 在 gateway test 目前沒被直接呼叫；補空 stub 讓 mock 滿足 interface 即可。
// 真實單元測試另外在 b169_test.go 用獨立 mock。

func (m *mockSellerServiceClient) ResolveSellerAccountID(ctx context.Context, in *sellerpb.ResolveSellerAccountIDRequest, opts ...grpc.CallOption) (*sellerpb.ResolveSellerAccountIDResponse, error) {
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "ResolveSellerAccountID not wired in test")
}
func (m *mockSellerServiceClient) GetCurrentStoreID(ctx context.Context, in *sellerpb.GetCurrentStoreIDRequest, opts ...grpc.CallOption) (*sellerpb.GetCurrentStoreIDResponse, error) {
	if m.GetCurrentStoreIDFn != nil {
		return m.GetCurrentStoreIDFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "GetCurrentStoreIDFn not wired in test")
}

// MatchCertificateHolder 供 19 號 §8.5.1 的業者名稱歸屬比對。Gateway 自己不呼叫它
// （product-service 才是消費者），mock 只是為了滿足 SellerServiceClient 介面。
func (m *mockSellerServiceClient) MatchCertificateHolder(ctx context.Context, in *sellerpb.MatchCertificateHolderRequest, opts ...grpc.CallOption) (*sellerpb.MatchCertificateHolderResponse, error) {
	if m.MatchCertificateHolderFn != nil {
		return m.MatchCertificateHolderFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "MatchCertificateHolderFn not wired in test")
}

// SearchSellerProspects Gateway 自己不呼叫，mock 只是為了滿足介面。
func (m *mockSellerServiceClient) SearchSellerProspects(ctx context.Context, in *sellerpb.SearchSellerProspectsRequest, opts ...grpc.CallOption) (*sellerpb.SearchSellerProspectsResponse, error) {
	if m.SearchSellerProspectsFn != nil {
		return m.SearchSellerProspectsFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "SearchSellerProspectsFn not wired in test")
}

// ==================== Mock BuyerServiceClient ====================

type mockBuyerServiceClient struct {
	GetBuyerProfileFn                func(ctx context.Context, in *buyerpb.GetBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error)
	BatchGetBuyerDisplayByBuyerIDsFn func(context.Context, *buyerpb.BatchGetBuyerDisplayByBuyerIDsRequest, ...grpc.CallOption) (*buyerpb.BatchGetBuyerDisplayByBuyerIDsResponse, error)
	GetAddressFn                     func(ctx context.Context, in *buyerpb.GetAddressRequest, opts ...grpc.CallOption) (*buyerpb.GetAddressResponse, error)
	CheckFavoriteFn                  func(ctx context.Context, in *buyerpb.CheckFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error)
	BatchCheckFavoritesFn            func(ctx context.Context, in *buyerpb.BatchCheckFavoritesRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error)
	BatchCheckRestockSubscriptionsFn func(ctx context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error)
	ValidateAddressFn                func(ctx context.Context, in *buyerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*buyerpb.ValidateAddressResponse, error)
	GetBuyerEntitlementFn            func(ctx context.Context, in *buyerpb.GetBuyerEntitlementRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error)
}

func (m *mockBuyerServiceClient) GetBuyerProfile(ctx context.Context, in *buyerpb.GetBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
	if m.GetBuyerProfileFn != nil {
		return m.GetBuyerProfileFn(ctx, in, opts...)
	}
	return &buyerpb.BuyerProfileResponse{Code: 0, Data: &buyerpb.BuyerInfo{Id: "1"}}, nil
}
func (m *mockBuyerServiceClient) GetAddress(ctx context.Context, in *buyerpb.GetAddressRequest, opts ...grpc.CallOption) (*buyerpb.GetAddressResponse, error) {
	if m.GetAddressFn != nil {
		return m.GetAddressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) CheckFavorite(ctx context.Context, in *buyerpb.CheckFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error) {
	if m.CheckFavoriteFn != nil {
		return m.CheckFavoriteFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) ListFavoritedProductIds(ctx context.Context, in *buyerpb.ListFavoritedProductIdsRequest, opts ...grpc.CallOption) (*buyerpb.ListFavoritedProductIdsResponse, error) {
	return nil, nil
}
func (m *mockBuyerServiceClient) ListSubscribedSellerIds(ctx context.Context, in *buyerpb.ListSubscribedSellerIdsRequest, opts ...grpc.CallOption) (*buyerpb.ListSubscribedSellerIdsResponse, error) {
	return nil, nil
}
func (m *mockBuyerServiceClient) ValidateAddress(ctx context.Context, in *buyerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*buyerpb.ValidateAddressResponse, error) {
	if m.ValidateAddressFn != nil {
		return m.ValidateAddressFn(ctx, in, opts...)
	}
	return &buyerpb.ValidateAddressResponse{Code: 0, Data: &buyerpb.AddressValidationData{}}, nil
}

func (m *mockBuyerServiceClient) BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, in *buyerpb.BatchGetBuyerDisplayByBuyerIDsRequest, opts ...grpc.CallOption) (*buyerpb.BatchGetBuyerDisplayByBuyerIDsResponse, error) {
	if m.BatchGetBuyerDisplayByBuyerIDsFn != nil {
		return m.BatchGetBuyerDisplayByBuyerIDsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) BatchCheckFavorites(ctx context.Context, in *buyerpb.BatchCheckFavoritesRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error) {
	if m.BatchCheckFavoritesFn != nil {
		return m.BatchCheckFavoritesFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Restock Subscription (#942) ====================

func (m *mockBuyerServiceClient) BatchCheckRestockSubscriptions(ctx context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
	if m.BatchCheckRestockSubscriptionsFn != nil {
		return m.BatchCheckRestockSubscriptionsFn(ctx, in, opts...)
	}
	return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0, Subscribed: map[string]bool{}}, nil
}

// ListBuyerIDsByAccount is an internal-only RPC (im-service PII-purge
// consumer, D8); the gateway never calls it, so this is a plain interface
// stub — no XxxFn field, matching the other never-called-from-gateway stubs
// above (BatchCheckRestockSubscriptions etc.).
func (m *mockBuyerServiceClient) ListBuyerIDsByAccount(_ context.Context, _ *buyerpb.ListBuyerIDsByAccountRequest, _ ...grpc.CallOption) (*buyerpb.ListBuyerIDsByAccountResponse, error) {
	return &buyerpb.ListBuyerIDsByAccountResponse{Code: 0}, nil
}

// ResolvePrincipals is an internal-only cross-service RPC (im-service
// self-message guard, ② buyer re-key W2/W3); the gateway never calls it, so
// this is a plain interface stub — no XxxFn field, same pattern as
// ListBuyerIDsByAccount above.
func (m *mockBuyerServiceClient) ResolvePrincipals(_ context.Context, _ *buyerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*buyerpb.ResolvePrincipalsResponse, error) {
	return &buyerpb.ResolvePrincipalsResponse{Code: 0}, nil
}

func (m *mockBuyerServiceClient) GetBuyerEntitlement(ctx context.Context, in *buyerpb.GetBuyerEntitlementRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error) {
	if m.GetBuyerEntitlementFn != nil {
		return m.GetBuyerEntitlementFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== mock clients ====================

type mockOrderServiceClient struct {
	orderpb.OrderServiceClient
	GetBuyerOrdersForSellerFn    func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error)
	GetMyOrdersFn                func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error)
	GetMyOrderStatusCountsFn     func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error)
	GetOrderFn                   func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error)
	GetSellerOrdersFn            func(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error)
	GetSellerOrderDetailFn       func(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error)
	CancelOrderFn                func(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error)
	ConfirmReceiptFn             func(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error)
	CheckoutValidateFn           func(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error)
	CheckoutPreviewFn            func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error)
	CheckoutConfirmFn            func(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error)
	CreateOrderFn                func(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error)
	SearchOrdersFn               func(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error)
	ListAbnormalOrdersForAdminFn func(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error)
	CheckoutShippingFeeFn        func(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error)
}

func (m *mockOrderServiceClient) GetBuyerOrdersForSeller(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
	if m.GetBuyerOrdersForSellerFn != nil {
		return m.GetBuyerOrdersForSellerFn(ctx, in, opts...)
	}
	return &orderpb.GetBuyerOrdersForSellerResponse{}, nil
}

func (m *mockOrderServiceClient) GetSellerOrders(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
	if m.GetSellerOrdersFn != nil {
		return m.GetSellerOrdersFn(ctx, in, opts...)
	}
	return &orderpb.GetSellerOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{}}, nil
}

func (m *mockOrderServiceClient) GetSellerOrderDetail(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error) {
	if m.GetSellerOrderDetailFn != nil {
		return m.GetSellerOrderDetailFn(ctx, in, opts...)
	}
	return &orderpb.GetSellerOrderDetailResponse{}, nil
}

func (m *mockOrderServiceClient) GetMyOrders(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
	if m.GetMyOrdersFn != nil {
		return m.GetMyOrdersFn(ctx, in, opts...)
	}
	return &orderpb.GetMyOrdersResponse{}, nil
}

func (m *mockOrderServiceClient) GetMyOrderStatusCounts(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
	if m.GetMyOrderStatusCountsFn != nil {
		return m.GetMyOrderStatusCountsFn(ctx, in, opts...)
	}
	return &orderpb.GetMyOrderStatusCountsResponse{}, nil
}

func (m *mockOrderServiceClient) GetOrder(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
	if m.GetOrderFn != nil {
		return m.GetOrderFn(ctx, in, opts...)
	}
	return &orderpb.GetOrderResponse{}, nil
}

func (m *mockOrderServiceClient) CancelOrder(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error) {
	if m.CancelOrderFn != nil {
		return m.CancelOrderFn(ctx, in, opts...)
	}
	return &orderpb.CancelOrderResponse{}, nil
}

func (m *mockOrderServiceClient) ConfirmReceipt(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error) {
	if m.ConfirmReceiptFn != nil {
		return m.ConfirmReceiptFn(ctx, in, opts...)
	}
	return &orderpb.ConfirmReceiptResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutValidate(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error) {
	if m.CheckoutValidateFn != nil {
		return m.CheckoutValidateFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutValidateResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutPreview(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
	if m.CheckoutPreviewFn != nil {
		return m.CheckoutPreviewFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutPreviewResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutConfirm(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error) {
	if m.CheckoutConfirmFn != nil {
		return m.CheckoutConfirmFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutConfirmResponse{}, nil
}

func (m *mockOrderServiceClient) CreateOrder(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error) {
	if m.CreateOrderFn != nil {
		return m.CreateOrderFn(ctx, in, opts...)
	}
	return &orderpb.CreateOrderResponse{}, nil
}

func (m *mockOrderServiceClient) SearchOrders(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error) {
	if m.SearchOrdersFn != nil {
		return m.SearchOrdersFn(ctx, in, opts...)
	}
	return &orderpb.SearchOrdersResponse{}, nil
}

func (m *mockOrderServiceClient) ListAbnormalOrdersForAdmin(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error) {
	if m.ListAbnormalOrdersForAdminFn != nil {
		return m.ListAbnormalOrdersForAdminFn(ctx, in, opts...)
	}
	return &orderpb.ListAbnormalOrdersForAdminResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutShippingFee(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error) {
	if m.CheckoutShippingFeeFn != nil {
		return m.CheckoutShippingFeeFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutShippingFeeResponse{}, nil
}
