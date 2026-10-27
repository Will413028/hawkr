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
	cartpb "github.com/Will413028/hawkr/go-pkg/proto/cart"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock CartServiceClient ====================

type mockCartServiceClient struct {
	AddItemFn         func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error)
	GetCartFn         func(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error)
	SetItemQuantityFn func(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error)
	MergeCartFn       func(ctx context.Context, in *cartpb.MergeCartRequest, opts ...grpc.CallOption) (*cartpb.MergeCartResponse, error)
}

func (m *mockCartServiceClient) AddItem(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
	if m.AddItemFn != nil {
		return m.AddItemFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockCartServiceClient) GetCart(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error) {
	if m.GetCartFn != nil {
		return m.GetCartFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockCartServiceClient) SetItemQuantity(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error) {
	if m.SetItemQuantityFn != nil {
		return m.SetItemQuantityFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockCartServiceClient) MergeCart(ctx context.Context, in *cartpb.MergeCartRequest, opts ...grpc.CallOption) (*cartpb.MergeCartResponse, error) {
	if m.MergeCartFn != nil {
		return m.MergeCartFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockCartServiceClient) CheckoutLock(ctx context.Context, in *cartpb.CheckoutLockRequest, opts ...grpc.CallOption) (*cartpb.CheckoutLockResponse, error) {
	return nil, nil
}
func (m *mockCartServiceClient) CheckoutUnlock(ctx context.Context, in *cartpb.CheckoutUnlockRequest, opts ...grpc.CallOption) (*cartpb.CheckoutUnlockResponse, error) {
	return nil, nil
}
func (m *mockCartServiceClient) RemoveItemsByCheckoutId(ctx context.Context, in *cartpb.RemoveItemsByCheckoutIdRequest, opts ...grpc.CallOption) (*cartpb.RemoveItemsByCheckoutIdResponse, error) {
	return nil, nil
}

// ==================== Router Setup ====================

// setupCartRouter creates routes for cart handlers.
// Guest routes (no auth middleware) and auth routes (with fakeJWTAuth).
func setupCartRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")

	// Public cart routes (optional auth via getOptionalAccountID)
	pub := v1.Group("/public/carts")
	{
		pub.GET("", h.GetCartList)
		pub.POST("/items", h.AddToCart)
		pub.PUT("/items/:sku_id/quantity", h.SetItemQuantity)
		pub.DELETE("/items/:sku_id", h.DeleteCartItem)
	}

	// Auth-required cart routes
	app := v1.Group("/app/carts")
	app.Use(fakeJWTAuth())
	{
		app.POST("/merge", h.MergeCart)
	}

	// Public cart routes with optional auth (logged-in user)
	pubAuth := v1.Group("/public/carts-auth")
	pubAuth.Use(fakeJWTAuth())
	{
		pubAuth.GET("", h.GetCartList)
		pubAuth.POST("/items", h.AddToCart)
		pubAuth.PUT("/items/:sku_id/quantity", h.SetItemQuantity)
		pubAuth.DELETE("/items/:sku_id", h.DeleteCartItem)
	}

	return r
}

// ==================== GetCartList ====================

func TestGetCartList_Success(t *testing.T) {
	mock := &mockCartServiceClient{
		GetCartFn: func(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error) {
			return &cartpb.GetCartResponse{
				Code:                 0,
				EstimatedTotalAmount: "500",
				TotalItems:           "2",
				Groups: []*cartpb.CartSellerGroup{
					{
						StoreId:        "100",
						SellerName:     "Test Store",
						LogoKey:        "public/seller/logo.png",
						SellerCity:     "台北市",
						SellerDistrict: "中山區",
						Items: []*cartpb.CartItemView{
							{
								SkuId:         "sku-1",
								Quantity:      "2",
								Name:          "Apple",
								Price:         "250",
								Status:        "NORMAL",
								ProductId:     "prod-1",
								SpecName:      "Large",
								ImageKey:      "public/product/apple.jpg",
								StockQty:      "10",
								DeliveryTemps: []string{"AMBIENT"},
							},
						},
					},
				},
			}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/carts?guest_id=guest-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	if data["totalItems"] != "2" {
		t.Errorf("totalItems = %v, want 2", data["totalItems"])
	}
	groups, ok := data["groups"].([]interface{})
	if !ok || len(groups) != 1 {
		t.Fatalf("groups length = %d, want 1", len(groups))
	}
}

func TestGetCartList_LogoURLField(t *testing.T) {
	mock := &mockCartServiceClient{
		GetCartFn: func(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error) {
			return &cartpb.GetCartResponse{
				Code: 0, Groups: []*cartpb.CartSellerGroup{
					{StoreId: "100", SellerName: "Test Store", LogoKey: "public/seller/logo.png"},
				},
			}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/carts?guest_id=guest-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	data := resp["data"].(map[string]interface{})
	groups := data["groups"].([]interface{})
	first := groups[0].(map[string]interface{})
	if got := first["logoUrl"]; got == nil || got == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", got)
	}
	if got := first["logoKey"]; got != "public/seller/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/logo.png")
	}
}

func TestGetCartList_GrpcError(t *testing.T) {
	mock := &mockCartServiceClient{
		GetCartFn: func(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/carts?guest_id=guest-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== AddToCart ====================

func TestAddToCart_Success(t *testing.T) {
	mock := &mockCartServiceClient{
		AddItemFn: func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
			if in.SkuId != "sku-1" {
				t.Errorf("SkuId = %q, want sku-1", in.SkuId)
			}
			if in.Quantity != "1" {
				t.Errorf("Quantity = %q, want 1", in.Quantity)
			}
			return &cartpb.AddItemResponse{
				Code:       0,
				TotalItems: "3",
			}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId":  "guest-123",
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

func TestAddToCart_BadRequest_MissingFields(t *testing.T) {
	h := &Holder{}
	router := setupCartRouter(h)

	// Missing skuId and quantity
	body, _ := json.Marshal(map[string]interface{}{
		"guestId": "guest-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

func TestAddToCart_BadRequest_GuestNoGuestId(t *testing.T) {
	h := &Holder{}
	router := setupCartRouter(h)

	// Guest user without guestId
	body, _ := json.Marshal(map[string]interface{}{
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

func TestAddToCart_GrpcError(t *testing.T) {
	mock := &mockCartServiceClient{
		AddItemFn: func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId":  "guest-123",
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== SetItemQuantity ====================

func TestSetItemQuantity_Success(t *testing.T) {
	mock := &mockCartServiceClient{
		SetItemQuantityFn: func(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error) {
			if in.SkuId != "sku-1" {
				t.Errorf("SkuId = %q, want sku-1", in.SkuId)
			}
			if in.Quantity != "5" {
				t.Errorf("Quantity = %q, want 5", in.Quantity)
			}
			return &cartpb.SetItemQuantityResponse{
				Code:       0,
				TotalItems: "5",
			}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId":  "guest-123",
		"quantity": "5",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/public/carts/items/sku-1/quantity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

func TestSetItemQuantity_BadRequest(t *testing.T) {
	h := &Holder{}
	router := setupCartRouter(h)

	// Missing quantity
	body, _ := json.Marshal(map[string]interface{}{
		"guestId": "guest-123",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/public/carts/items/sku-1/quantity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

// ==================== DeleteCartItem ====================

func TestDeleteCartItem_Success(t *testing.T) {
	mock := &mockCartServiceClient{
		SetItemQuantityFn: func(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error) {
			if in.SkuId != "sku-1" {
				t.Errorf("SkuId = %q, want sku-1", in.SkuId)
			}
			if in.Quantity != "0" {
				t.Errorf("Quantity = %q, want 0 (delete)", in.Quantity)
			}
			return &cartpb.SetItemQuantityResponse{
				Code:       0,
				TotalItems: "1",
			}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)}
	router := setupCartRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/public/carts/items/sku-1?guest_id=guest-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

func TestDeleteCartItem_GuestNoGuestId(t *testing.T) {
	h := &Holder{}
	router := setupCartRouter(h)

	// Guest user without guest_id query param
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/public/carts/items/sku-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 400)
}

// ==================== MergeCart ====================

func TestMergeCart_Success(t *testing.T) {
	mock := &mockCartServiceClient{
		MergeCartFn: func(ctx context.Context, in *cartpb.MergeCartRequest, opts ...grpc.CallOption) (*cartpb.MergeCartResponse, error) {
			if in.GuestId != "guest-123" {
				t.Errorf("GuestId = %q, want guest-123", in.GuestId)
			}
			return &cartpb.MergeCartResponse{
				Code: 0,
			}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId": "guest-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/carts/merge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)
}

// ==================== Buyer ID resolution (W2 buyer-store-stint rekey) ====================
//
// gateway/internal/buyerid.Resolver wiring (design doc
// docs/superpowers/specs/2026-07-16-buyer-store-stint-rekey-w2w3-design.md
// §3.1, D3): every cart call site must forward the CALLER'S CURRENT buyer_id
// (stint surrogate), not the raw JWT, so a re-registered buyer
// (new stint) doesn't inherit a stale cart. Guest requests (accountID==0) and
// logged-in accounts without a buyer profile yet (ErrNoCurrentBuyer — e.g. a
// seller-only account browsing the storefront) must NOT be blocked; cart has
// no FK/join dependency on the identity value (D3), so both fall back to the
// pre-resolution behaviour of forwarding accountID unchanged.

// capturedXAccountID extracts the resolved identity CartClient's
// ctxWithAccount stamped onto the context for this call (see
// gateway/internal/grpc/cart_client.go) — the only way to observe, from the
// mock's ctx, which identity the handler actually resolved and forwarded.
//
// Reads the CtxKeyAccountID context value (via WithAuthContext), not
// outgoing gRPC metadata: NewCartClientForTest injects the mock proto client
// directly (see internal/grpc/testing.go), bypassing the real
// grpc.ClientConn/DefaultDialOpts — so AuthMetadataInterceptor, which is what
// actually turns this context value into "x-account-id" wire metadata in
// production, never runs here. That interceptor behavior (including the
// regression it once had — a second, stale x-account-id value silently
// winning over this one, see cart_client_test.go) is covered separately at
// the internal/grpc layer; this helper only needs to confirm the handler
// resolved and handed off the right identity.
func capturedXAccountID(ctx context.Context) string {
	accountID, ok := ctx.Value(grpcclient.CtxKeyAccountID).(int64)
	if !ok {
		return ""
	}
	return strconv.FormatInt(accountID, 10)
}

func TestAddToCart_ResolvesBuyerID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		AddItemFn: func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
			captured = capturedXAccountID(ctx)
			return &cartpb.AddItemResponse{Code: 0, TotalItems: "1"}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerResolverForTest(9009),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts-auth/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "9009" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (resolved buyer_id, not raw JWT account_id 42)", captured, "9009")
	}
}

func TestGetCartList_ResolvesBuyerID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		GetCartFn: func(ctx context.Context, in *cartpb.GetCartRequest, opts ...grpc.CallOption) (*cartpb.GetCartResponse, error) {
			captured = capturedXAccountID(ctx)
			return &cartpb.GetCartResponse{Code: 0, TotalItems: "0"}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerResolverForTest(9009),
	}
	router := setupCartRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/carts-auth", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "9009" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (resolved buyer_id, not raw JWT account_id 42)", captured, "9009")
	}
}

func TestSetItemQuantity_ResolvesBuyerID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		SetItemQuantityFn: func(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error) {
			captured = capturedXAccountID(ctx)
			return &cartpb.SetItemQuantityResponse{Code: 0, TotalItems: "5"}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerResolverForTest(9009),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"quantity": "5",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/public/carts-auth/items/sku-1/quantity", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "9009" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (resolved buyer_id, not raw JWT account_id 42)", captured, "9009")
	}
}

func TestDeleteCartItem_ResolvesBuyerID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		SetItemQuantityFn: func(ctx context.Context, in *cartpb.SetItemQuantityRequest, opts ...grpc.CallOption) (*cartpb.SetItemQuantityResponse, error) {
			if in.Quantity != "0" {
				t.Errorf("Quantity = %q, want 0 (delete)", in.Quantity)
			}
			captured = capturedXAccountID(ctx)
			return &cartpb.SetItemQuantityResponse{Code: 0, TotalItems: "0"}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerResolverForTest(9009),
	}
	router := setupCartRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/public/carts-auth/items/sku-1", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "9009" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (resolved buyer_id, not raw JWT account_id 42)", captured, "9009")
	}
}

func TestMergeCart_ResolvesBuyerID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		MergeCartFn: func(ctx context.Context, in *cartpb.MergeCartRequest, opts ...grpc.CallOption) (*cartpb.MergeCartResponse, error) {
			captured = capturedXAccountID(ctx)
			return &cartpb.MergeCartResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: buyerResolverForTest(9009),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId": "guest-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/carts/merge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "9009" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (resolved buyer_id, not raw JWT account_id 42)", captured, "9009")
	}
}

func TestAddToCart_NoBuyerProfileFallsBackToRawAccountID(t *testing.T) {
	var captured string
	mock := &mockCartServiceClient{
		AddItemFn: func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
			captured = capturedXAccountID(ctx)
			return &cartpb.AddItemResponse{Code: 0, TotalItems: "1"}, nil
		},
	}
	h := &Holder{
		Cart:    grpcclient.NewCartClientForTest(mock),
		BuyerID: noBuyerResolverForTest(),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts-auth/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	if captured != "42" {
		t.Errorf("x-account-id forwarded to cart-service = %q, want %q (fallback to raw account_id when buyer profile missing)", captured, "42")
	}
}

func TestAddToCart_BuyerResolverErrorReturns503(t *testing.T) {
	h := &Holder{
		Cart: grpcclient.NewCartClientForTest(&mockCartServiceClient{}),
		BuyerID: buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
			return nil, fmt.Errorf("buyer-service down")
		})),
	}
	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts-auth/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestAddToCart_GuestSkipsBuyerResolution(t *testing.T) {
	// accountID==0（訪客）必須完全略過 resolver——h.BuyerID 甚至可以是 nil。
	// 這條斷言鎖住 resolveCartBuyerID 的 accountID==0 短路分支：不能因為新增
	// 的呼叫而動到訪客路徑（含 h.BuyerID 未注入的情況，例如某些較舊的訪客
	// 專用 wiring）。
	mock := &mockCartServiceClient{
		AddItemFn: func(ctx context.Context, in *cartpb.AddItemRequest, opts ...grpc.CallOption) (*cartpb.AddItemResponse, error) {
			if got := capturedXAccountID(ctx); got != "" {
				t.Errorf("guest request must not carry x-account-id metadata, got %q", got)
			}
			return &cartpb.AddItemResponse{Code: 0, TotalItems: "1"}, nil
		},
	}
	h := &Holder{Cart: grpcclient.NewCartClientForTest(mock)} // BuyerID intentionally nil

	router := setupCartRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"guestId":  "guest-123",
		"skuId":    "sku-1",
		"quantity": "1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/carts/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}
