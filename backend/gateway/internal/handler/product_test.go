package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// favoriteTestAccountID / favoriteTestBuyerID are deliberately DIFFERENT values:
// the product overlay must send the resolved buyer_id downstream, never the raw
// account_id, and equal fixtures would hide that mistake.
const (
	favoriteTestAccountID int64 = 42
	favoriteTestBuyerID   int64 = 777
)

// ==================== Router Setup ====================

func setupProductRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public/products")
	{
		pub.GET("", h.GetProductList)
		pub.GET("/:id", h.GetProductDetailByID)
		pub.GET("/:id/rating", h.GetProductRating)
	}
	v1.DELETE("/admin/products/cache", h.PurgeProductCache)
	return r
}

// fakeOptionalAuth 模擬 middleware.OptionalJWTAuth()：X-Account-Id/X-Account-Type
// header 有帶才寫入 context，缺席時直接放行（公開路由匿名可訪問，不像 fakeJWTAuth 那樣 401）。
func fakeOptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if idStr := c.GetHeader("X-Account-Id"); idStr != "" {
			var accountID int64
			for _, ch := range idStr {
				accountID = accountID*10 + int64(ch-'0')
			}
			c.Set("accountId", accountID)
		}
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

func setupProductRouterWithAuth(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public/products")
	pub.Use(fakeOptionalAuth())
	{
		pub.GET("", h.GetProductList)
		pub.GET("/:id", h.GetProductDetailByID)
	}
	return r
}

// ==================== GetProductList ====================

func TestGetProductList_Success(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			if in.Page != "1" {
				t.Errorf("Page = %q, want 1", in.Page)
			}
			if in.PageSize != "20" {
				t.Errorf("PageSize = %q, want 20", in.PageSize)
			}
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "2",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{
						Id:       "prod-1",
						Name:     "Apple",
						Status:   "ACTIVE",
						PriceMin: "100",
						PriceMax: "200",
						Rating:   "4.5",
						ImageKey: "public/product/apple.jpg",
						Seller: &productpb.SellerInfo{
							StoreId: "100",
							Name:    "Test Store",
						},
					},
					{
						Id:       "prod-2",
						Name:     "Banana",
						Status:   "ACTIVE",
						PriceMin: "50",
						PriceMax: "80",
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products?page=1&page_size=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["total"] != "2" {
		t.Errorf("total = %v, want 2", data["total"])
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 2 {
		t.Fatalf("records length = %d, want 2", len(records))
	}
}

func TestGetProductList_PassesSellerCityCodeToProductService(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			if in.SellerCityCode != "TW-TPE" {
				t.Fatalf("seller_city_code = %q, want TW-TPE", in.SellerCityCode)
			}
			if !reflect.DeepEqual(in.StoreIds, []string{"102", "999"}) {
				t.Fatalf("store_ids = %v, want unchanged [102 999]", in.StoreIds)
			}
			return &productpb.GetProductListResponse{Code: 0, Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(productMock)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products?seller_city_code=TW-TPE&store_ids=102,999", nil)
	w := httptest.NewRecorder()
	setupProductRouter(h).ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
}

func TestGetProductList_GrpcError(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 503)
}

// ==================== GetProductList followed_* gating (ViewerBuyerId) ====================

// TestGetProductList_FollowedFilterGating pins the behavior change reviewed
// in phase 2: h.resolveBuyerID(c) is only consulted when followed_product_only
// or followed_seller_only is active, and the resolved buyer_id (never the raw
// account_id) is what reaches GetProductListRequest.ViewerBuyerId. Uses
// favoriteTestAccountID/favoriteTestBuyerID (defined in favorite_test.go,
// deliberately different values) so a regression that forwards the raw
// account_id instead of the resolved buyer_id is actually caught.
func TestGetProductList_FollowedFilterGating(t *testing.T) {
	t.Run("NoFilter_ViewerBuyerIdZero_ResolverNotCalled", func(t *testing.T) {
		productMock := &mockProductServiceClient{
			GetProductListFn: func(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
				if in.ViewerBuyerId != 0 {
					t.Errorf("ViewerBuyerId = %d, want 0 when no followed_* filter is active", in.ViewerBuyerId)
				}
				return &productpb.GetProductListResponse{Code: 0, Page: "1", PageSize: "20"}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			BuyerID: mustNotResolveBuyer(t),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		resp := parseJSON(t, w.Body)
		assertCode(t, resp, 0)
	})

	t.Run("FollowedProductOnly_NoCurrentBuyerStint_Returns403", func(t *testing.T) {
		called := false
		productMock := &mockProductServiceClient{
			GetProductListFn: func(_ context.Context, _ *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
				called = true
				return &productpb.GetProductListResponse{Code: 0, Page: "1", PageSize: "20"}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			BuyerID: noBuyerResolverForTest(),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products?followed_product_only=true", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusForbidden)
		resp := parseJSON(t, w.Body)
		assertCode(t, resp, 403)
		if called {
			t.Error("GetProductListFn must not be called when the account has no current buyer stint")
		}
	})

	t.Run("FollowedProductOnly_ForwardsResolvedBuyerID_NotAccountID", func(t *testing.T) {
		productMock := &mockProductServiceClient{
			GetProductListFn: func(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
				if in.ViewerBuyerId != favoriteTestBuyerID {
					t.Errorf("ViewerBuyerId = %d, want resolved buyer_id %d (account_id %d must not be forwarded)", in.ViewerBuyerId, favoriteTestBuyerID, favoriteTestAccountID)
				}
				if !in.FollowedProductOnly {
					t.Error("FollowedProductOnly = false, want true")
				}
				return &productpb.GetProductListResponse{Code: 0, Page: "1", PageSize: "20"}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			BuyerID: buyerResolverForTest(favoriteTestBuyerID),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products?followed_product_only=true", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		resp := parseJSON(t, w.Body)
		assertCode(t, resp, 0)
	})
}

// ==================== GetProductList favorites aggregation (buyer_id soft-fail) ====================

// TestGetProductList_FavoritesAggregation proves the isFavorited aggregation
// in GetProductList resolves account_id -> buyer_id via the soft-fail path
// (h.BuyerID.CurrentBuyerID, never h.resolveBuyerID which would write 401/403
// to c) before calling BatchCheckFavorites, and that the raw account_id is
// never forwarded downstream. favoriteTestAccountID/favoriteTestBuyerID
// (defined in favorite_test.go) are deliberately different values so a
// grandfather resolver (buyer_id == account_id) could not accidentally pass
// this assertion — same rationale as TestGetFavoriteList_Success.
//
// It also proves an account with no current buyer stint (ErrNoCurrentBuyer)
// degrades cleanly: the list still returns 200 with every item's
// isFavorited=false and BatchCheckFavorites is never called, instead of
// panicking or writing an error response.
func TestGetProductList_FavoritesAggregation(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(_ context.Context, _ *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "2",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{Id: "prod-1", Name: "Apple", Status: "ACTIVE", PriceMin: "100", PriceMax: "200"},
					{Id: "prod-2", Name: "Banana", Status: "ACTIVE", PriceMin: "50", PriceMax: "80"},
				},
			}, nil
		},
	}

	t.Run("ResolvedBuyerIDForwarded", func(t *testing.T) {
		buyerMock := &mockBuyerServiceClient{
			BatchCheckFavoritesFn: func(_ context.Context, in *buyerpb.BatchCheckFavoritesRequest, _ ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error) {
				if in.BuyerId != favoriteTestBuyerID {
					t.Errorf("BatchCheckFavoritesRequest.BuyerId = %d, want resolved buyer_id %d (account_id %d must not be forwarded)", in.BuyerId, favoriteTestBuyerID, favoriteTestAccountID)
				}
				return &buyerpb.BatchCheckFavoritesResponse{Code: 0, FavoritedProductIds: []string{"prod-1"}}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: buyerResolverForTest(favoriteTestBuyerID),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		records := pageRecords(t, body)
		if len(records) != 2 {
			t.Fatalf("len(records) = %d, want 2", len(records))
		}
		for _, rec := range records {
			item := rec.(map[string]interface{})
			want := item["id"] == "prod-1"
			if item["isFavorited"] != want {
				t.Errorf("product %v isFavorited = %v, want %v", item["id"], item["isFavorited"], want)
			}
		}
	})

	t.Run("NoCurrentBuyer_SkipsGracefully", func(t *testing.T) {
		called := false
		buyerMock := &mockBuyerServiceClient{
			BatchCheckFavoritesFn: func(context.Context, *buyerpb.BatchCheckFavoritesRequest, ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error) {
				called = true
				return &buyerpb.BatchCheckFavoritesResponse{Code: 0}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: noBuyerResolverForTest(),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		records := pageRecords(t, body)
		if len(records) != 2 {
			t.Fatalf("len(records) = %d, want 2", len(records))
		}
		for _, rec := range records {
			item := rec.(map[string]interface{})
			if item["isFavorited"] != false {
				t.Errorf("product %v isFavorited = %v, want false (no current buyer stint)", item["id"], item["isFavorited"])
			}
		}
		if called {
			t.Error("BatchCheckFavorites must not be called when the account has no current buyer stint")
		}
	})
}

// TestGetProductList_RestockSubscribedAggregation mirrors
// TestGetProductList_FavoritesAggregation for the isRestockSubscribed
// aggregation (BatchCheckRestockSubscriptions is now buyer_id-first too,
// sharing the same soft-fail buyerID resolved for favorites — see
// product.go's comment on the shared `buyerID, berr :=` above both
// aggregations).
func TestGetProductList_RestockSubscribedAggregation(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetProductListFn: func(_ context.Context, _ *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "2",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{Id: "prod-1", Name: "Apple", Status: "ACTIVE", PriceMin: "100", PriceMax: "200"},
					{Id: "prod-2", Name: "Banana", Status: "ACTIVE", PriceMin: "50", PriceMax: "80"},
				},
			}, nil
		},
	}

	t.Run("ResolvedBuyerIDForwarded", func(t *testing.T) {
		buyerMock := &mockBuyerServiceClient{
			// GetProductList 現在共用同一個 buyerID 做收藏+補貨訂閱兩個聚合，即使本
			// 測試只關心補貨訂閱，收藏那條路徑仍會被觸發，要給一個有效回應（否則
			// mock 預設的 (nil, nil) 會讓 BuyerClient.BatchCheckFavorites 對 nil
			// resp 解參考 panic）。
			BatchCheckFavoritesFn: func(context.Context, *buyerpb.BatchCheckFavoritesRequest, ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error) {
				return &buyerpb.BatchCheckFavoritesResponse{Code: 0}, nil
			},
			BatchCheckRestockSubscriptionsFn: func(_ context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, _ ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
				if in.BuyerId != favoriteTestBuyerID {
					t.Errorf("BatchCheckRestockSubscriptionsRequest.BuyerId = %d, want resolved buyer_id %d (account_id %d must not be forwarded)", in.BuyerId, favoriteTestBuyerID, favoriteTestAccountID)
				}
				return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0, Subscribed: map[string]bool{"prod-1": true}}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: buyerResolverForTest(favoriteTestBuyerID),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		records := pageRecords(t, body)
		if len(records) != 2 {
			t.Fatalf("len(records) = %d, want 2", len(records))
		}
		for _, rec := range records {
			item := rec.(map[string]interface{})
			want := item["id"] == "prod-1"
			if item["isRestockSubscribed"] != want {
				t.Errorf("product %v isRestockSubscribed = %v, want %v", item["id"], item["isRestockSubscribed"], want)
			}
		}
	})

	t.Run("NoCurrentBuyer_SkipsGracefully", func(t *testing.T) {
		called := false
		buyerMock := &mockBuyerServiceClient{
			BatchCheckRestockSubscriptionsFn: func(context.Context, *buyerpb.BatchCheckRestockSubscriptionsRequest, ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
				called = true
				return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: noBuyerResolverForTest(),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		records := pageRecords(t, body)
		if len(records) != 2 {
			t.Fatalf("len(records) = %d, want 2", len(records))
		}
		for _, rec := range records {
			item := rec.(map[string]interface{})
			if item["isRestockSubscribed"] != false {
				t.Errorf("product %v isRestockSubscribed = %v, want false (no current buyer stint)", item["id"], item["isRestockSubscribed"])
			}
		}
		if called {
			t.Error("BatchCheckRestockSubscriptions must not be called when the account has no current buyer stint")
		}
	})
}

// ==================== GetProductDetailByID ====================

func TestGetProductDetailByID_Success(t *testing.T) {
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			if in.ProductId != "prod-1" {
				t.Errorf("ProductId = %q, want prod-1", in.ProductId)
			}
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:       "prod-1",
					Name:     "Apple",
					Status:   "ACTIVE",
					Rating:   "4.5",
					PriceMin: "100",
					PriceMax: "200",
					Seller: &productpb.SellerInfo{
						StoreId: "100",
						Name:    "Test Store",
						LogoKey: "public/seller/logo.png",
						Address: "台北市中山區",
					},
					ReviewCount:      "10",
					PublicationState: "4012",
					Purchasable:      true,
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["name"] != "Apple" {
		t.Errorf("name = %v, want Apple", data["name"])
	}
	if data["publicationState"] != "4012" || data["purchasable"] != true {
		t.Errorf("publication contract = (%v, %v), want (4012, true)", data["publicationState"], data["purchasable"])
	}
	seller, ok := data["seller"].(map[string]interface{})
	if !ok {
		t.Fatal("missing seller field")
	}
	if seller["storeId"] != "100" {
		t.Errorf("storeId = %v, want 100", seller["storeId"])
	}
}

// ==================== Seller nested logoKey/logoUrl (B-238 / issue #1782) ====================

// TestGetProductList_SellerLogoURLField proves the nested seller object in
// /public/products list responses exposes the seller logo as the canonical
// camelCase pair logoKey/logoUrl (49 §6.1). snake_case logo_key/logo_url is
// never emitted.
func TestGetProductList_SellerLogoURLField(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "1",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{
						Id:   "prod-1",
						Name: "蕃茄",
						Seller: &productpb.SellerInfo{
							StoreId: "42",
							Name:    "老王鮮魚",
							LogoKey: "public/seller/42/logo.png",
						},
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
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
		t.Fatalf("records length = %d, want 1", len(records))
	}
	seller, ok := records[0].(map[string]interface{})["seller"].(map[string]interface{})
	if !ok {
		t.Fatal("missing seller field")
	}
	if got := seller["logoKey"]; got != "public/seller/42/logo.png" {
		t.Errorf("seller.logoKey = %v, want %q", got, "public/seller/42/logo.png")
	}
	if got := seller["logoUrl"]; got == nil || got == "" {
		t.Errorf("seller.logoUrl = %v, want a non-empty built URL", got)
	}
}

// TestGetProductDetailByID_SellerLogoURLField proves the nested seller object
// in /public/products/:id detail responses exposes the seller logo as the
// canonical camelCase pair logoKey/logoUrl (49 §6.1). snake_case
// logo_key/logo_url is never emitted.
func TestGetProductDetailByID_SellerLogoURLField(t *testing.T) {
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:     "prod-1",
					Name:   "Apple",
					Status: "ACTIVE",
					Seller: &productpb.SellerInfo{
						StoreId: "100",
						Name:    "Test Store",
						LogoKey: "public/seller/logo.png",
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	seller, ok := data["seller"].(map[string]interface{})
	if !ok {
		t.Fatal("missing seller field")
	}
	if got := seller["logoKey"]; got != "public/seller/logo.png" {
		t.Errorf("seller.logoKey = %v, want %q", got, "public/seller/logo.png")
	}
	if got := seller["logoUrl"]; got == nil || got == "" {
		t.Errorf("seller.logoUrl = %v, want a non-empty built URL", got)
	}
}

// TestGalleryAssetIDVisibility 驗證 gallery / description 內部資產識別碼在各端的可見性紅線（02 §2.2.2）：
// 賣家 / Admin（convertToProductDetail）保留 assetId + objectKey 供編輯 round-trip；
// 公開（convertToPublicProductDetail）兩處都換成不含 assetId 的公開型別（PublicGalleryItem /
// PublicDescriptionBlock），objectKey 與 url 一律保留——
// objectKey 是 public/ 前綴的公開路徑，同一份回應的 imageKeys 早已列出全部 gallery objectKey，
// 清掉它保護不到任何東西，卻讓前端（尤其 objectKey 不在 imageKeys 內的描述內嵌圖）只能載原圖。
func TestGalleryAssetIDVisibility(t *testing.T) {
	// 49 §6.6：gallery/description url 改由 Gateway 從 objectKey 組（proto Url 不再由服務側填、Gateway 忽略之）。
	const base = "https://media.test"
	galleryURL := mediaurl.BuildURL(base, "public/p/1.jpg")
	blockURL := mediaurl.BuildURL(base, "public/d/1.jpg")
	data := &productpb.ProductDetail{
		Id:   "prod-1",
		Name: "Apple",
		Gallery: []*productpb.GalleryItem{
			{AssetId: "asset-1", ObjectKey: "public/p/1.jpg"},
		},
		Description: &productpb.ProductDescription{
			Version: "1",
			Blocks: []*productpb.DescriptionBlock{
				{Type: "image", AssetId: "asset-desc-1", ObjectKey: "public/d/1.jpg"},
			},
		},
	}

	// 賣家 / Admin 端：保留 assetId / objectKey；url 由 Gateway 組
	seller := convertToProductDetail(base, data)
	if len(seller.Gallery) != 1 || seller.Gallery[0].AssetID != "asset-1" || seller.Gallery[0].ImageKey != "public/p/1.jpg" {
		t.Fatalf("seller gallery should keep assetId/objectKey, got %+v", seller.Gallery)
	}
	if seller.Gallery[0].ImageURL != galleryURL {
		t.Errorf("seller gallery url = %q, want %q", seller.Gallery[0].ImageURL, galleryURL)
	}
	if seller.Description == nil || len(seller.Description.Blocks) != 1 ||
		seller.Description.Blocks[0].AssetID != "asset-desc-1" || seller.Description.Blocks[0].ImageKey != "public/d/1.jpg" {
		t.Fatalf("seller description should keep assetId/objectKey, got %+v", seller.Description)
	}
	// 賣家 / Admin 端 round-trip 的 wire 層證據：assetId 必須真的序列化出去（gallery 與描述
	// 區塊各一個），否則賣家只調圖片順序時就會丟圖（既有圖靠 assetId 認）。
	sellerBlob, err := json.Marshal(seller)
	if err != nil {
		t.Fatal(err)
	}
	sellerJS := string(sellerBlob)
	if !strings.Contains(sellerJS, `"assetId":"asset-1"`) {
		t.Errorf("seller JSON must keep gallery assetId: %s", sellerJS)
	}
	if !strings.Contains(sellerJS, `"assetId":"asset-desc-1"`) {
		t.Errorf("seller JSON must keep description assetId: %s", sellerJS)
	}

	// 公開端：gallery 走 PublicGalleryItem、description.blocks 走 PublicDescriptionBlock，
	// 兩者型別上就沒有 assetId；objectKey / url 保留
	pub := convertToPublicProductDetail(base, data)
	if len(pub.Gallery) != 1 {
		t.Fatalf("public gallery length = %d, want 1", len(pub.Gallery))
	}
	if pub.Gallery[0].ImageKey != "public/p/1.jpg" {
		t.Errorf("public gallery objectKey must be kept, got %q want %q", pub.Gallery[0].ImageKey, "public/p/1.jpg")
	}
	if pub.Gallery[0].ImageURL != galleryURL {
		t.Errorf("public gallery url must be kept, got %q want %q", pub.Gallery[0].ImageURL, galleryURL)
	}
	if pub.Description == nil || len(pub.Description.Blocks) != 1 {
		t.Fatalf("public description should have exactly 1 block, got %+v", pub.Description)
	}
	if pub.Description.Blocks[0].ImageKey != "public/d/1.jpg" {
		t.Errorf("public description objectKey must be kept, got %q want %q", pub.Description.Blocks[0].ImageKey, "public/d/1.jpg")
	}
	if pub.Description.Blocks[0].ImageURL != blockURL {
		t.Errorf("public description url must be kept, got %q want %q", pub.Description.Blocks[0].ImageURL, blockURL)
	}

	// 序列化層級：assetId key 完全不得出現；gallery / description 的 objectKey 必須在
	blob, err := json.Marshal(pub)
	if err != nil {
		t.Fatal(err)
	}
	js := string(blob)
	if strings.Contains(js, `"assetId"`) {
		t.Errorf("public JSON must not contain assetId key: %s", js)
	}
	if !strings.Contains(js, `"imageKey":"public/p/1.jpg"`) {
		t.Errorf("public JSON must keep gallery imageKey: %s", js)
	}
	if !strings.Contains(js, `"imageKey":"public/d/1.jpg"`) {
		t.Errorf("public JSON must keep description imageKey: %s", js)
	}
	// 舊欄名不得殘留——只斷言新名存在的話，同時吐兩組也會綠。
	for _, stale := range []string{`"objectKey"`, `"url":`} {
		if strings.Contains(js, stale) {
			t.Errorf("public JSON 不得再出現舊欄名 %s: %s", stale, js)
		}
	}
	if !strings.Contains(js, galleryURL) {
		t.Errorf("public JSON must keep gallery url: %s", js)
	}
	// 遮蔽驗證：內嵌 ProductDetailItem 的 Gallery / Description（含 assetId 的賣家形狀）
	// 不得漏進公開 JSON。這兩行釘住的是 encoding/json 的同名欄位取較淺層規則——若哪天有人
	// 把外層欄位拿掉，內嵌欄位會立刻接手序列化並把 assetId 一起吐出去。
	if strings.Count(js, `"gallery"`) != 1 {
		t.Errorf("public JSON must contain exactly one gallery key: %s", js)
	}
	if strings.Count(js, `"description"`) != 1 {
		t.Errorf("public JSON must contain exactly one description key: %s", js)
	}
}

// TestPublicDTOFieldParity 釘死「公開型別＝賣家型別減掉 assetId」這條不變式。
//
// 型別分離（而非執行期清欄位）讓 swagger 宣告等於實際輸出，代價是新增欄位得改兩處。
// 這個測試讓漏掉的那一處當場轉紅，而不是等前端發現公開端少一個欄位——那正是 C5 一開始
// 反過來出事的形狀（宣告有、輸出沒有），只是方向相反。
func TestPublicDTOFieldParity(t *testing.T) {
	cases := []struct {
		name        string
		sellerShape interface{}
		publicShape interface{}
		omitted     []string // 公開端刻意不帶的 json key
	}{
		{"gallery", GalleryItem{}, PublicGalleryItem{}, []string{"assetId"}},
		{"descriptionBlock", DescriptionBlockResponse{}, PublicDescriptionBlock{}, []string{"assetId"}},
		// 容器層也要釘：在賣家側的 description 容器加欄位（而非 block 內）同樣會漂移，
		// 只是高一層。公開端與賣家端的容器 key 應完全相同——差別只在 blocks 的元素型別。
		{"descriptionContainer", ProductDescriptionResponse{}, PublicProductDescription{}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sellerKeys := jsonFieldKeys(t, tc.sellerShape)
			publicKeys := jsonFieldKeys(t, tc.publicShape)

			// 先證明取得出欄位——空集合對空集合會無聲通過，那是假綠。
			if len(sellerKeys) == 0 || len(publicKeys) == 0 {
				t.Fatalf("json field keys must be non-empty: seller=%v public=%v", sellerKeys, publicKeys)
			}

			want := make([]string, 0, len(sellerKeys))
			for _, k := range sellerKeys {
				if !slices.Contains(tc.omitted, k) {
					want = append(want, k)
				}
			}
			if len(want) != len(sellerKeys)-len(tc.omitted) {
				t.Fatalf("omitted keys %v not all present in seller shape %v", tc.omitted, sellerKeys)
			}
			if !slices.Equal(publicKeys, want) {
				t.Errorf("public shape json keys = %v, want %v (seller shape minus %v)", publicKeys, want, tc.omitted)
			}
			for _, k := range tc.omitted {
				if slices.Contains(publicKeys, k) {
					t.Errorf("public shape must not declare %q, got %v", k, publicKeys)
				}
			}
		})
	}
}

// jsonFieldKeys 取出 struct 每個匯出欄位的 json key（去掉 ,omitempty 之類的選項），依宣告順序。
func jsonFieldKeys(t *testing.T, v interface{}) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		t.Fatalf("jsonFieldKeys expects a struct, got %s", rt.Kind())
	}
	keys := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" { // 未匯出欄位不會序列化
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		keys = append(keys, name)
	}
	return keys
}

func TestGetProductDetailByID_SellerRoleResolvesToCurrentSurrogate(t *testing.T) {
	var gotAccountID int64
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			gotAccountID = in.StoreId
			return &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: in.ProductId, Status: "INACTIVE"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	req.Header.Set("X-Account-Id", "2600000001")
	req.Header.Set("X-Account-Type", "2") // SELLER
	w := httptest.NewRecorder()
	setupProductRouterWithAuth(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountID != rekeyTestSurrogate {
		t.Errorf("GetProductRequest.StoreId = %d, want resolved surrogate %d", gotAccountID, rekeyTestSurrogate)
	}
}

func TestGetProductDetailByID_BuyerViewer_UsesPublicPublicationContract(t *testing.T) {
	publicCalled := false
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(_ context.Context, in *productpb.GetPublicProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			publicCalled = true
			return &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{
				Id: in.ProductId, Status: "4003", PublicationState: "4013", Purchasable: false,
			}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: resolverThatFailsTest(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	req.Header.Set("X-Account-Id", "3600000002")
	req.Header.Set("X-Account-Type", "1") // BUYER
	w := httptest.NewRecorder()
	setupProductRouterWithAuth(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if !publicCalled {
		t.Fatal("buyer public PDP must call GetPublicProduct")
	}
}

func TestGetProductDetailByID_SellerWithNoCurrentStore_FallsBackToRawAccountID(t *testing.T) {
	var gotAccountID int64
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			gotAccountID = in.StoreId
			return &productpb.GetProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: in.ProductId, Status: "ACTIVE"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: noStoreResolverForTest()}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	req.Header.Set("X-Account-Id", "2600000001")
	req.Header.Set("X-Account-Type", "2") // SELLER，但已刪店
	w := httptest.NewRecorder()
	setupProductRouterWithAuth(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountID != 2600000001 {
		t.Errorf("GetProductRequest.StoreId = %d, want raw fallback 2600000001", gotAccountID)
	}
}

// ==================== GetProductDetailByID favorite aggregation (buyer_id soft-fail) ====================

// TestGetProductDetailByID_FavoriteAggregation proves the isFavorited and
// isRestockSubscribed aggregations in the PDP share a single soft-fail
// buyer_id resolution (h.BuyerID.CurrentBuyerID, never h.resolveBuyerID which
// would write 401/403 to c) before calling CheckFavorite / BatchCheckRestockSubscriptions,
// and that the raw account_id is never forwarded downstream to either. It
// also proves an account with no current buyer stint (ErrNoCurrentBuyer)
// degrades cleanly: detail.isFavorited and detail.isRestockSubscribed both
// stay false and neither sibling goroutine's RPC is called, while the PDP
// still returns a normal 200 response (no error written).
func TestGetProductDetailByID_FavoriteAggregation(t *testing.T) {
	const pdpProductID = "9001"
	productMock := &mockProductServiceClient{
		GetPublicProductFn: func(_ context.Context, in *productpb.GetPublicProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{Id: pdpProductID, Status: "ACTIVE", PublicationState: "4012", Purchasable: true},
			}, nil
		},
	}

	t.Run("ResolvedBuyerIDForwarded", func(t *testing.T) {
		buyerMock := &mockBuyerServiceClient{
			CheckFavoriteFn: func(_ context.Context, in *buyerpb.CheckFavoriteRequest, _ ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error) {
				if in.BuyerId != favoriteTestBuyerID {
					t.Errorf("CheckFavoriteRequest.BuyerId = %d, want resolved buyer_id %d (account_id %d must not be forwarded)", in.BuyerId, favoriteTestBuyerID, favoriteTestAccountID)
				}
				return &buyerpb.CheckFavoriteResponse{Code: 0, Favorited: true}, nil
			},
			BatchCheckRestockSubscriptionsFn: func(_ context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, _ ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
				if in.BuyerId != favoriteTestBuyerID {
					t.Errorf("BatchCheckRestockSubscriptionsRequest.BuyerId = %d, want resolved buyer_id %d (account_id %d must not be forwarded)", in.BuyerId, favoriteTestBuyerID, favoriteTestAccountID)
				}
				return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0, Subscribed: map[string]bool{pdpProductID: true}}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: buyerResolverForTest(favoriteTestBuyerID),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/"+pdpProductID, nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatal("missing data field")
		}
		if data["isFavorited"] != true {
			t.Errorf("isFavorited = %v, want true", data["isFavorited"])
		}
		if data["isRestockSubscribed"] != true {
			t.Errorf("isRestockSubscribed = %v, want true (sibling goroutine unaffected)", data["isRestockSubscribed"])
		}
	})

	t.Run("NoCurrentBuyer_SkipsBothGracefully", func(t *testing.T) {
		favoriteCalled := false
		restockCalled := false
		buyerMock := &mockBuyerServiceClient{
			CheckFavoriteFn: func(context.Context, *buyerpb.CheckFavoriteRequest, ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error) {
				favoriteCalled = true
				return &buyerpb.CheckFavoriteResponse{Code: 0, Favorited: true}, nil
			},
			BatchCheckRestockSubscriptionsFn: func(_ context.Context, _ *buyerpb.BatchCheckRestockSubscriptionsRequest, _ ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
				restockCalled = true
				return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0, Subscribed: map[string]bool{pdpProductID: true}}, nil
			},
		}
		h := &Holder{
			Product: grpcclient.NewProductClientForTest(productMock),
			Buyer:   grpcclient.NewBuyerClientForTest(buyerMock),
			BuyerID: noBuyerResolverForTest(),
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/"+pdpProductID, nil)
		req.Header.Set("X-Account-Id", strconv.FormatInt(favoriteTestAccountID, 10))
		w := httptest.NewRecorder()
		setupProductRouterWithAuth(h).ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 0)
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatal("missing data field")
		}
		if data["isFavorited"] != false {
			t.Errorf("isFavorited = %v, want false (no current buyer stint)", data["isFavorited"])
		}
		if favoriteCalled {
			t.Error("CheckFavorite must not be called when the account has no current buyer stint")
		}
		if data["isRestockSubscribed"] != false {
			t.Errorf("isRestockSubscribed = %v, want false (no current buyer stint) — restock now shares the same resolved buyer_id as favorites", data["isRestockSubscribed"])
		}
		if restockCalled {
			t.Error("BatchCheckRestockSubscriptions must not be called when the account has no current buyer stint — it now shares the favorites resolution instead of resolving independently")
		}
	})
}

func TestGetProductDetailByID_NotFound(t *testing.T) {
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code:    404,
				Message: "product not found",
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 404)
}

func TestGetProductDetailByID_GrpcError(t *testing.T) {
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 503)
}

// ==================== GetProductRating ====================

func TestGetProductRating_Success(t *testing.T) {
	mock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:          "prod-1",
					Name:        "Apple",
					Status:      "ACTIVE",
					Rating:      "4.5",
					ReviewCount: "42",
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupProductRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/prod-1/rating", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["rating"] != "4.5" {
		t.Errorf("rating = %v, want 4.5", data["rating"])
	}
	if data["reviewCount"] != "42" {
		t.Errorf("reviewCount = %v, want 42", data["reviewCount"])
	}
}

// ==================== PurgeProductCache ====================

func purgeCacheRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/cache", strings.NewReader(body))
}

func TestPurgeProductCache_ListScope_ResolvesAccountIDToCurrentSurrogate(t *testing.T) {
	var gotStoreID int64
	mock := &mockProductServiceClient{
		PurgeProductCacheFn: func(_ context.Context, in *productpb.PurgeProductCacheRequest, _ ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error) {
			gotStoreID = in.StoreId
			return &productpb.PurgeProductCacheResponse{Code: 0, Data: &productpb.PurgeCacheResult{Scope: in.Scope, PurgedKeys: "1"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	router := setupProductRouter(h)

	req := purgeCacheRequest(t, `{"scope":"list","accountId":2600000001}`)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
	if gotStoreID != rekeyTestSurrogate {
		t.Errorf("PurgeProductCacheRequest.StoreId = %d, want resolved surrogate %d", gotStoreID, rekeyTestSurrogate)
	}
}

// scope=list、accountId=0（不篩選特定帳號，清全部列表快取）：不該呼叫 resolver，
// 也要維持 accountId=0 直接送給 product-service。
func TestPurgeProductCache_ListScope_ZeroAccountID_SkipsResolver(t *testing.T) {
	var called bool
	mock := &mockProductServiceClient{
		PurgeProductCacheFn: func(_ context.Context, in *productpb.PurgeProductCacheRequest, _ ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error) {
			called = true
			if in.StoreId != 0 {
				t.Errorf("StoreId = %d, want 0", in.StoreId)
			}
			return &productpb.PurgeProductCacheResponse{Code: 0, Data: &productpb.PurgeCacheResult{Scope: in.Scope, PurgedKeys: "5"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: resolverThatFailsTest(t)}
	router := setupProductRouter(h)

	req := purgeCacheRequest(t, `{"scope":"list"}`)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if !called {
		t.Error("product-service was never called")
	}
}

// scope=detail：accountId 語意跟 list 無關，不該觸發 resolver。
func TestPurgeProductCache_DetailScope_SkipsResolver(t *testing.T) {
	var gotProductID string
	mock := &mockProductServiceClient{
		PurgeProductCacheFn: func(_ context.Context, in *productpb.PurgeProductCacheRequest, _ ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error) {
			gotProductID = in.ProductId
			return &productpb.PurgeProductCacheResponse{Code: 0, Data: &productpb.PurgeCacheResult{Scope: in.Scope, PurgedKeys: "1"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: resolverThatFailsTest(t)}
	router := setupProductRouter(h)

	req := purgeCacheRequest(t, `{"scope":"detail","productId":"prod-1"}`)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotProductID != "prod-1" {
		t.Errorf("PurgeProductCacheRequest.ProductId = %q, want prod-1", gotProductID)
	}
}

// 該帳號目前沒有現任店（已刪店未重開）→ 沒有對應的 list bucket 可清，回成功
// 0 筆，呼應 SearchAdminProducts 對同一情境回空清單的處理；product-service
// 不該被呼叫（沒有 store_id 可送）。
func TestPurgeProductCache_ListScope_NoCurrentSeller_ReturnsZeroPurged(t *testing.T) {
	mock := &mockProductServiceClient{
		PurgeProductCacheFn: func(context.Context, *productpb.PurgeProductCacheRequest, ...grpc.CallOption) (*productpb.PurgeProductCacheResponse, error) {
			t.Fatal("product-service must not be called when account has no current seller")
			return nil, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: noStoreResolverForTest()}
	router := setupProductRouter(h)

	req := purgeCacheRequest(t, `{"scope":"list","accountId":2600000001}`)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["purgedKeys"] != "0" {
		t.Errorf("purgedKeys = %v, want 0", data["purgedKeys"])
	}
}

// ==================== VerifyCertification：cert_no 字元白名單 ====================

// setupCertificationRouter 只掛 VerifyCertification 一條路由。真實路由（routes_product.go）
// 前面還有 JWT 與 requireSeller，但這裡要釘的是「非法字元有沒有在送進 gRPC 之前被擋下」，
// 與身分無關，掛裸 handler 才不會讓 auth 的 401 蓋掉要觀察的訊號。
func setupCertificationRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/app/products/certifications/verify", h.VerifyCertification)
	return r
}

func certificationRequest(certNo string) *http.Request {
	return certificationRequestWithType("3701", certNo)
}

func certificationRequestWithType(certType, certNo string) *http.Request {
	u := "/api/v1/app/products/certifications/verify?type=" + url.QueryEscape(certType) +
		"&cert_no=" + url.QueryEscape(certNo)
	return httptest.NewRequest(http.MethodGet, u, nil)
}

// TestVerifyCertification_RejectsNonWhitelistedCertNo
//
// cert_no 一路從這裡傳到 product-service 的 GovCertificationService.buildGovApiUrl，在那裡
// 原字未編碼串進農業部 API 的查詢字串，後面緊接著 &api_key=<真實金鑰>。修這條之前，
// 入口只擋空字串，因此任何人送一個含 URI 保留字元的 cert_no 就能讓上游的 URI 解析失敗、
// 把整條含金鑰的 URL 逼進例外訊息——不需要任何外部服務先壞掉，可反覆觸發。
//
// 斷言「gRPC 沒有被呼叫」是重點：只看 HTTP 狀態碼的話，「先送出去再回 400」的實作照樣會過，
// 而外洩發生在送出去的那一刻。
//
// 這些案例一律用可線上驗證的 type=3701——白名單只在那三型生效（cert_no 只有那三型會被串進
// 政府 API URL）。不可線上驗證的三型見 TestVerifyCertification_NonVerifiableTypeSkipsWhitelist。
func TestVerifyCertification_RejectsNonWhitelistedCertNo(t *testing.T) {
	cases := []struct {
		name   string
		certNo string
	}{
		{"空白", "1-008 131005"},
		{"角括號", "1-008-131005<x>"},
		{"大括號", "abc{}"},
		{"直線", "abc|def"},
		{"caret", "abc^def"},
		{"裸百分號", "%41"},
		{"多塞查詢參數", "abc&api_key=stolen"},
		{"井字號", "abc#frag"},
		{"路徑分隔", "abc/def"},
		{"非 ASCII", "有機認證"},
		{"超出長度上限", strings.Repeat("A", certNoMaxLength+1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			mock := &mockProductServiceClient{
				VerifyCertificationFn: func(context.Context, *productpb.VerifyCertificationRequest, ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error) {
					called = true
					return &productpb.VerifyCertificationResponse{Code: 0, Exists: true}, nil
				},
			}
			h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
			w := httptest.NewRecorder()
			setupCertificationRouter(h).ServeHTTP(w, certificationRequest(tc.certNo))

			if called {
				t.Fatalf("cert_no %q 被送進 gRPC：外洩就發生在這一刻，回幾號都無關", tc.certNo)
			}
			assertHTTPStatus(t, w, http.StatusBadRequest)
			resp := parseJSON(t, w.Body)
			msg, _ := resp["message"].(string)
			if !strings.Contains(msg, "認證編號") {
				t.Errorf("message = %q, 應指出是認證編號這一欄", msg)
			}
		})
	}
}

func TestVerifyCertification_RejectsEmptyCertNo(t *testing.T) {
	called := false
	mock := &mockProductServiceClient{
		VerifyCertificationFn: func(context.Context, *productpb.VerifyCertificationRequest, ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error) {
			called = true
			return nil, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	w := httptest.NewRecorder()
	setupCertificationRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/app/products/certifications/verify?type=3701&cert_no=", nil))

	if called {
		t.Fatal("空 cert_no 不該送進 gRPC")
	}
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// TestVerifyCertification_AcceptsWhitelistedCertNo 白名單不得過度收緊：
// 樣本取自設計書 19 §3/§7.1 與 infra/seed-data/04-product.sql 的實際認證編號。
func TestVerifyCertification_AcceptsWhitelistedCertNo(t *testing.T) {
	for _, certNo := range []string{
		"1-008-131005",                       // 有機
		"1-009-300022",                       // 有機（設計書時序圖樣本）
		"TAP-2025-0032",                      // 產銷履歷
		"CAS-2025-1234",                      // CAS
		"ALI-TEA-2026-015",                   // seed data 產地證明
		"abc_123",                            // 底線
		"A",                                  // 單字元下界
		strings.Repeat("A", certNoMaxLength), // 長度上界
	} {
		t.Run(certNo, func(t *testing.T) {
			var got string
			mock := &mockProductServiceClient{
				VerifyCertificationFn: func(_ context.Context, in *productpb.VerifyCertificationRequest, _ ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error) {
					got = in.CertNo
					return &productpb.VerifyCertificationResponse{Code: 0, Exists: true}, nil
				},
			}
			h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
			w := httptest.NewRecorder()
			setupCertificationRouter(h).ServeHTTP(w, certificationRequest(certNo))

			assertHTTPStatus(t, w, http.StatusOK)
			if got != certNo {
				t.Errorf("送進 gRPC 的 cert_no = %q, want %q", got, certNo)
			}
			resp := parseJSON(t, w.Body)
			assertCode(t, resp, 0)
		})
	}
}

// TestVerifyCertification_NonVerifiableTypeSkipsWhitelist 白名單不得溢出到不可線上驗證的三型。
//
// HACCP/ISO/GMP（3704/3705/3706）的 cert_no 永遠不會被 product-service 串進農業部 API 的
// 查詢字串——那三型在 product-service 端先短路成 3003「此認證類型暫不支援線上驗證」，
// cert_no 根本不離開系統。對它們套白名單零安全效益，卻會把 `ISO 22000:2018` 這種該類型的
// 常見合法編號從 3003 誤判成 400，等於無故改掉設計書 19 §13 測試計畫第 4 條的對外契約。
//
// 這裡斷言「請求有送到 gRPC、且 cert_no 逐字不變」：把判斷權留給 product-service 才是正解，
// gateway 不該替不會外洩的類型代擋。
func TestVerifyCertification_NonVerifiableTypeSkipsWhitelist(t *testing.T) {
	cases := []struct {
		name     string
		certType string
		certNo   string
	}{
		{"HACCP 帶空白", "3704", "HACCP 2026 DP 0047"},
		{"ISO 帶冒號", "3705", "ISO 22000:2018"},
		{"GMP 帶斜線", "3706", "GMP/2026/0031"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			mock := &mockProductServiceClient{
				VerifyCertificationFn: func(_ context.Context, in *productpb.VerifyCertificationRequest, _ ...grpc.CallOption) (*productpb.VerifyCertificationResponse, error) {
					got = in.CertNo
					return &productpb.VerifyCertificationResponse{
						Code:    bizcode.CertTypeNotVerifiable,
						Message: "此認證類型暫不支援線上驗證",
					}, nil
				},
			}
			h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
			w := httptest.NewRecorder()
			setupCertificationRouter(h).ServeHTTP(w, certificationRequestWithType(tc.certType, tc.certNo))

			if got != tc.certNo {
				t.Fatalf("送進 gRPC 的 cert_no = %q, want %q（白名單不該擋下不可線上驗證的類型）", got, tc.certNo)
			}
			resp := parseJSON(t, w.Body)
			assertCode(t, resp, int(bizcode.CertTypeNotVerifiable))
			msg, _ := resp["message"].(string)
			if strings.Contains(msg, "格式不正確") {
				t.Errorf("message = %q, 不該是白名單的格式錯誤", msg)
			}
		})
	}
}
