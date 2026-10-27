package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupSellerProductRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.GET("/products", h.ListSellerProducts)
	app.POST("/products", h.CreateSellerProduct)
	return r
}

// ② owner column 灌注點：CreateProduct 的 AccountId 必須是 resolver 解析出的
// surrogate — 這個值就是 product-service 落 owner 欄位的來源。
func TestCreateSellerProduct_StampsResolvedSurrogate(t *testing.T) {
	var captured *productpb.CreateProductRequest
	mock := &mockProductServiceClient{
		CreateProductFn: func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
			captured = in
			return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupSellerProductRouter(h)

	// 建立商品必填至少一個 SKU，且每個 SKU 欄位齊全（CreateProductRequest.Skus 的
	// `required,dive` + CreateSkuInput 的 binding tag）。本測驗的是 surrogate 灌注，
	// SKU 只是讓請求通過驗證的背景資料。
	body := `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1","skus":[` +
		`{"skuName":"s1","unit":"3301","price":"100","stockQty":"5",` +
		`"weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}]}`
	req := httptest.NewRequest("POST", "/api/v1/app/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.StoreId != 7001 {
		t.Errorf("StoreId = %+v, want 7001 (resolved surrogate, not login id 99)", captured)
	}
}

func TestCreateSellerProduct_NoCurrentSeller403(t *testing.T) {
	mock := &mockProductServiceClient{
		CreateProductFn: func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
			t.Fatal("product-service must not be called without a current seller")
			return nil, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: noStoreResolverForTest()}
	r := setupSellerProductRouter(h)

	body := `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1"}`
	req := httptest.NewRequest("POST", "/api/v1/app/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	assertCode(t, parseJSON(t, w.Body), 403)
}

// ② owner filter：賣家自己的商品列表必須以 surrogate 過濾。
func TestListSellerProducts_FiltersByResolvedSurrogate(t *testing.T) {
	var captured *productpb.GetProductListRequest
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			captured = in
			return &productpb.GetProductListResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupSellerProductRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/products", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.OwnerStoreId != 7001 {
		t.Errorf("owner filter OwnerStoreId = %+v, want 7001 (resolved surrogate)", captured)
	}
}
