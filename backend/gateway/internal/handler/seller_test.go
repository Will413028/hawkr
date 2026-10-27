package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// ==================== GetSellerInfo (public, PK lookup) ====================
func TestGetSellerInfo_PublicIDIsPKLookup(t *testing.T) {
	mock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			if in.StoreId != 777 {
				t.Errorf("StoreId = %d, want 777 (PK lookup)", in.StoreId)
			}
			if in.AccountId != 0 {
				t.Errorf("AccountId = %d, want 0 (public path must not send login id)", in.AccountId)
			}
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: "777", Name: "老王鮮魚",
			}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if got := data["storeId"]; got != "777" {
		t.Errorf("SellerInfo.store_id = %v, want \"777\" (surrogate, not login id 42)", got)
	}
}

func TestGetSellerInfo_StoreCodeExposed(t *testing.T) {
	mock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: "777", StoreCode: "3012345678", Name: "老王鮮魚",
			}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if got := data["storeCode"]; got != "3012345678" {
		t.Errorf("SellerInfo.store_code = %v, want \"3012345678\"", got)
	}
}

// TestGetSellerInfo_LogoURLField proves /public/sellers/:id exposes the
// canonical `logo_url` field (B-238 / issue #1782) alongside the existing
// deprecated `logo` field, both built from the same LogoKey.
func TestGetSellerInfo_LogoURLField(t *testing.T) {
	mock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: "777", Name: "老王鮮魚", LogoKey: "public/seller/777/logo.png",
			}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if got := data["logoKey"]; got != "public/seller/777/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/777/logo.png")
	}
	if got := data["logoUrl"]; got == nil || got == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", got)
	}
}

// ==================== GetSellerInfo store-visitor tracking ====================
// GetSellerInfo 掛 OptionalJWTAuth，"訪客" 語意限定買家——這裡的 fixture 刻意讓
// raw accountId（9)、resolved buyer_id（surrogate）、store_id（TSID storeID）三個
// 全部取不同數字，確保斷言「傳的是解析後的值」真的有鑑別力（identity-resolution-
// consistency.md Rule 4）。

func TestGetSellerInfo_RecordsVisitWithResolvedBuyerID(t *testing.T) {
	captured := make(chan *sellerpb.RecordStoreVisitRequest, 1)
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 500, StoreId: "777123456789012345", Name: "老王鮮魚",
			}}, nil
		},
		RecordStoreVisitFn: func(ctx context.Context, in *sellerpb.RecordStoreVisitRequest, opts ...grpc.CallOption) (*sellerpb.RecordStoreVisitResponse, error) {
			captured <- in
			return &sellerpb.RecordStoreVisitResponse{Code: 0}, nil
		},
	}
	// raw accountId 9 解析出的現任 buyer surrogate 是 90001——刻意跟 9 不同量級，
	// 斷言才能區分「傳了解析後的值」還是「原封不動傳了 raw accountId」。
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock), BuyerID: buyerResolverForTest(90001)}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", fakeOptionalAuth(), h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	req.Header.Set("X-Account-Id", "9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)

	select {
	case got := <-captured:
		if got.VisitorBuyerId != 90001 {
			t.Errorf("VisitorBuyerId = %d, want 90001 (resolved buyer surrogate, not raw accountId 9)", got.VisitorBuyerId)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RecordStoreVisit was not called within timeout")
	}
}

func TestGetSellerInfo_SkipsVisitForNonBuyerAccount(t *testing.T) {
	called := make(chan struct{}, 1)
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 500, StoreId: "777123456789012345", Name: "老王鮮魚",
			}}, nil
		},
		RecordStoreVisitFn: func(ctx context.Context, in *sellerpb.RecordStoreVisitRequest, opts ...grpc.CallOption) (*sellerpb.RecordStoreVisitResponse, error) {
			called <- struct{}{}
			return &sellerpb.RecordStoreVisitResponse{Code: 0}, nil
		},
	}
	// 一個沒有現任買家 stint 的登入身分（例如純賣家帳號逛別家商店）——
	// noBuyerResolverForTest 模擬 buyer-service 404 → ErrNoCurrentBuyer。
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock), BuyerID: noBuyerResolverForTest()}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", fakeOptionalAuth(), h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	req.Header.Set("X-Account-Id", "9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)

	select {
	case <-called:
		t.Fatal("RecordStoreVisit was called for a non-buyer account (no current buyer stint) — should be skipped")
	case <-time.After(300 * time.Millisecond):
		// 沒呼叫，符合預期。
	}
}

func TestGetSellerInfo_SkipsSelfVisitForRekeyedStore(t *testing.T) {
	called := make(chan struct{}, 1)
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			// 店主 raw account_id=9，但商店已重鍵過，store_id（TSID）是完全不同量級的
			// 數字——舊寫法 vid != storeID 對這種商店恆真，等於從沒排除過店主自己。
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 9, StoreId: "777123456789012345", Name: "老王鮮魚",
			}}, nil
		},
		RecordStoreVisitFn: func(ctx context.Context, in *sellerpb.RecordStoreVisitRequest, opts ...grpc.CallOption) (*sellerpb.RecordStoreVisitResponse, error) {
			called <- struct{}{}
			return &sellerpb.RecordStoreVisitResponse{Code: 0}, nil
		},
	}
	// 店主自己也剛好有現任買家身分（常見：既是賣家也是一般消費者）——若自訪排除
	// 沒生效，這裡會被誤記一次訪客。
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock), BuyerID: buyerResolverForTest(90001)}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", fakeOptionalAuth(), h.GetSellerInfo)
	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	req.Header.Set("X-Account-Id", "9") // 跟店主 AccountId 相同
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)

	select {
	case <-called:
		t.Fatal("RecordStoreVisit was called for the store owner viewing their own (rekeyed) store — self-exclusion should have skipped it")
	case <-time.After(300 * time.Millisecond):
		// 沒呼叫，符合預期。
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
