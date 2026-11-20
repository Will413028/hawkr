package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// fakeSellerAuth 模擬已登入且具賣家權限的 middleware（測試用）
// 從 X-Account-Id header 取得 accountId，並將 accountType 設為 2（SELLER）
func fakeSellerAuth() gin.HandlerFunc {
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
		c.Set("accountType", 2) // SELLER
		c.Next()
	}
}

// ==================== GetSellerProducts ====================

// TestGetSellerProducts_LogoCanonicalFields proves /public/sellers/:id/products
// exposes the canonical logo_key/logo_url fields (issue #1782/B-238) on
// records[].seller — this construction site was missed by the original Task 7
// sweep (final whole-branch review finding) and only set the deprecated
// camelCase logoKey/logoUrl pair.
func TestGetSellerProducts_LogoCanonicalFields(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			if in.StoreId != 777 {
				t.Errorf("StoreId = %d, want 777 (resolveStorefrontSellerID PK lookup)", in.StoreId)
			}
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: "777", Name: "老王鮮魚",
			}}, nil
		},
	}
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "測試商品", StoreId: "777",
						Seller: &productpb.SellerInfo{
							StoreId: "777",
							Name:    "老王鮮魚",
							LogoKey: "public/seller/777/logo.png",
						},
					},
				},
				Total: "1", Page: "1", PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Seller:  grpcclient.NewSellerClientForTest(sellerMock),
		Product: grpcclient.NewProductClientForTest(productMock),
	}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id/products", h.GetSellerProducts)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777/products", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	if len(records) != 1 {
		t.Fatalf("records count = %d, want 1", len(records))
	}
	seller := records[0].(map[string]interface{})["seller"].(map[string]interface{})

	// Exact-value key check
	if got := seller["logoKey"]; got != "public/seller/777/logo.png" {
		t.Errorf("records[0].seller.logoKey = %v, want %q", got, "public/seller/777/logo.png")
	}

	// Non-empty check for the canonical camelCase URL (49 §6.1)
	logoURL := seller["logoUrl"]
	if logoURL == nil || logoURL == "" {
		t.Errorf("records[0].seller.logoUrl = %v, want a non-empty built URL", logoURL)
	}

	// Deprecated camelCase fields must still exist (additive-only)
	if seller["logoKey"] == nil {
		t.Errorf("records[0].seller.logoKey (deprecated) should still exist")
	}
	if seller["logoUrl"] == nil {
		t.Errorf("records[0].seller.logoUrl (deprecated) should still exist")
	}
}

// TestGetSellerProducts_CarriesStoreCode pins the seller display code onto
// /public/sellers/:id/products records[].seller (web#365 storefront
// canonicalization). This handler calls product-service GetProductList without
// owner_store_id, i.e. the public branch that already enriches store_code — the
// same RPC/message that feeds /public/products — so an absent field here is a
// silent drop inside this handler, and the frontend link helper then falls back
// to the numeric store id, costing one 308 redirect per storefront link.
func TestGetSellerProducts_CarriesStoreCode(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: "777", StoreCode: "2612345678", Name: "老王鮮魚",
			}}, nil
		},
	}
	productMock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			if in.OwnerStoreId != 0 {
				t.Errorf("OwnerStoreId = %d, want 0 (public branch, the one that enriches store_code)", in.OwnerStoreId)
			}
			return &productpb.GetProductListResponse{
				Code: 0,
				Products: []*productpb.ProductListItem{
					{
						Id: "prod-1", Name: "測試商品", StoreId: "777",
						Seller: &productpb.SellerInfo{
							StoreId:   "777",
							StoreCode: "2612345678",
							Name:      "老王鮮魚",
							LogoKey:   "public/seller/777/logo.png",
						},
					},
				},
				Total: "1", Page: "1", PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		Seller:  grpcclient.NewSellerClientForTest(sellerMock),
		Product: grpcclient.NewProductClientForTest(productMock),
	}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id/products", h.GetSellerProducts)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777/products", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	if len(records) != 1 {
		t.Fatalf("records count = %d, want 1", len(records))
	}
	seller, ok := records[0].(map[string]interface{})["seller"].(map[string]interface{})
	if !ok {
		t.Fatalf("records[0].seller missing, got %#v", records[0])
	}
	if got := seller["storeCode"]; got != "2612345678" {
		t.Errorf("records[0].seller.storeCode = %v, want %q", got, "2612345678")
	}
}
