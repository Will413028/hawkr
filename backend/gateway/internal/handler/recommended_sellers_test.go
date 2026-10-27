package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupRecommendedSellersRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public")
	pub.GET("/sellers/recommended", h.GetRecommendedSellers)
	return r
}

func TestGetRecommendedSellers_Success(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			if in.Limit != 10 {
				t.Errorf("Limit = %d, want 10", in.Limit)
			}
			return &sellerpb.GetRecommendedSellersResponse{
				Code:    0,
				Message: "success",
				Sellers: []*sellerpb.RecommendedSeller{
					{StoreId: "1001", Name: "Store A", LogoKey: "logo_a.jpg", CoverKey: "cover_a.jpg", Description: "Desc A", Rating: 4.8, OrderCount: 10},
					{StoreId: "1002", Name: "Store B", LogoKey: "logo_b.jpg", CoverKey: "cover_b.jpg", Description: "Desc B", Rating: 4.5, OrderCount: 5},
				},
			}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}

	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	first := records[0].(map[string]interface{})
	if first["storeName"] != "Store A" {
		t.Errorf("first store_name = %q, want %q", first["storeName"], "Store A")
	}
	second := records[1].(map[string]interface{})
	if second["storeName"] != "Store B" {
		t.Errorf("second store_name = %q, want %q", second["storeName"], "Store B")
	}
}

func TestGetRecommendedSellers_EmptyResults(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return &sellerpb.GetRecommendedSellersResponse{
				Code:    0,
				Message: "success",
				Sellers: []*sellerpb.RecommendedSeller{},
			}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}

	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestGetRecommendedSellers_LimitCap(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			if in.Limit != 50 {
				t.Errorf("Limit = %d, want 50 (capped from 100)", in.Limit)
			}
			return &sellerpb.GetRecommendedSellersResponse{Code: 0, Sellers: []*sellerpb.RecommendedSeller{}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}

	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended?limit=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
}

func TestGetRecommendedSellers_SellerServiceDown(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}

	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

// TestGetRecommendedSellers_LogoFields is a regression guard proving this
// endpoint's records[] already carry the canonical logo_key/logo_url pair
// (B-238 / issue #1782) — no production code change in this test's task,
// only a Swagger-annotation fix; this test locks in the currently-correct
// runtime behavior so it can't silently regress later.
func TestGetRecommendedSellers_LogoFields(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return &sellerpb.GetRecommendedSellersResponse{
				Code: 0, Sellers: []*sellerpb.RecommendedSeller{
					{StoreId: "1001", Name: "Store A", LogoKey: "public/seller/1001/logo.png"},
				},
			}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}
	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	first := records[0].(map[string]interface{})
	if got := first["logoKey"]; got != "public/seller/1001/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/1001/logo.png")
	}
	if got := first["logoUrl"]; got == nil || got == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", got)
	}
}

// TestGetRecommendedSellers_CarriesStoreCode pins store_code onto the standalone
// recommendation endpoint (web#365). The storefront links each rail card to a
// seller by display code; with the field absent the frontend helper falls back to
// the numeric store id and the link resolves through a 308 redirect. seller-service
// carries the code on RecommendedSeller, so any regression would be a silent drop
// in this handler's gin.H assembly.
func TestGetRecommendedSellers_CarriesStoreCode(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetRecommendedSellersFn: func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
			return &sellerpb.GetRecommendedSellersResponse{
				Code: 0, Sellers: []*sellerpb.RecommendedSeller{
					{StoreId: "1001", StoreCode: "2612345678", Name: "Store A"},
				},
			}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}
	router := setupRecommendedSellersRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/sellers/recommended", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	records := data["records"].([]interface{})
	if len(records) != 1 {
		t.Fatalf("records count = %d, want 1", len(records))
	}
	if got := records[0].(map[string]interface{})["storeCode"]; got != "2612345678" {
		t.Errorf("records[0].store_code = %v, want %q", got, "2612345678")
	}
}
