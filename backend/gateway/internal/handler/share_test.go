package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func setupShareRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	app := v1.Group("/app")
	app.Use(fakeJWTAuth())
	app.GET("/share_url", h.GetShareURL)
	return r
}

func setupShareHolder(sellerMock *mockSellerServiceClient, productMock *mockProductServiceClient) *Holder {
	h := &Holder{}
	if sellerMock != nil {
		h.Seller = grpcclient.NewSellerClientForTest(sellerMock)
	}
	if productMock != nil {
		h.Product = grpcclient.NewProductClientForTest(productMock)
	}
	return h
}

func TestGetShareURL_Product(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:       "10001",
					Name:     "有機蘋果 1kg",
					PriceMin: "200",
					ImageKey: "https://test-media.example.com/apple.jpg",
					Seller: &productpb.SellerInfo{
						StoreId: "1001",
						Name:    "黑貓測試家",
					},
				},
			}, nil
		},
	}
	h := setupShareHolder(nil, productMock)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=product&id=10001", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	if data["url"] != "http://localhost:3000/products/10001" {
		t.Errorf("url = %q, want %q", data["url"], "http://localhost:3000/products/10001")
	}
	if data["title"] != "有機蘋果 1kg" {
		t.Errorf("title = %q, want %q", data["title"], "有機蘋果 1kg")
	}
	if data["description"] != "NT$200 - 黑貓測試家" {
		t.Errorf("description = %q, want %q", data["description"], "NT$200 - 黑貓測試家")
	}
	if data["imageUrl"] != "https://test-media.example.com/apple.jpg" {
		t.Errorf("imageUrl = %q, want %q", data["imageUrl"], "https://test-media.example.com/apple.jpg")
	}
}

func TestGetShareURL_MissingParams(t *testing.T) {
	h := setupShareHolder(nil, nil)
	router := setupShareRouter(h)

	tests := []struct {
		name string
		url  string
	}{
		{"missing type and id", "/api/v1/app/share_url"},
		{"missing id", "/api/v1/app/share_url?type=post"},
		{"missing type", "/api/v1/app/share_url?id=123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			req.Header.Set("X-Account-Id", "9999")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestGetShareURL_InvalidType(t *testing.T) {
	h := setupShareHolder(nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=unknown&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestGetShareURL_PostTypeIsInvalid(t *testing.T) {
	h := setupShareHolder(nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=post&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestGetShareURL_ProductNotFound(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return nil, grpcstatus.Error(grpccodes.NotFound, "product not found")
		},
	}
	router := setupShareRouter(setupShareHolder(nil, productMock))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=product&id=999", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
}

func TestGetShareURL_ServiceDown(t *testing.T) {
	productMock := &mockProductServiceClient{
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	router := setupShareRouter(setupShareHolder(nil, productMock))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=product&id=123", nil)
	req.Header.Set("X-Account-Id", "9999")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetShareURL_Unauthorized(t *testing.T) {
	h := setupShareHolder(nil, nil)
	router := setupShareRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/share_url?type=product&id=123", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}
