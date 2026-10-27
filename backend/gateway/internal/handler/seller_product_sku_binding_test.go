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

// CreateSkuInput 上的 binding:"required" 只有在父欄位帶 `dive` 時才會生效。
// 沒有 dive 的話那些標籤完全是裝飾——dev 實測（2026-08-05）省略 stockQty、送空
// skuName 都得到 200 created，真正的把關只在 Java 端，而讀 Gateway 這段程式碼的人
// 會以為這裡擋得住。本測試把「標籤真的有作用」釘住。
func TestCreateSellerProduct_NestedSkuValidationIsEnforced(t *testing.T) {
	const validSku = `{"skuName":"s1","unit":"3301","price":"100","stockQty":"5",` +
		`"weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}`

	cases := []struct {
		name       string
		skusJSON   string
		wantStatus int
	}{
		{"完整 SKU（控制組，證明其他斷言不是恆真）", `[` + validSku + `]`, http.StatusOK},
		{"省略 stockQty", `[{"skuName":"s1","unit":"3301","price":"100","weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}]`, http.StatusBadRequest},
		{"skuName 空字串", `[{"skuName":"","unit":"3301","price":"100","stockQty":"5","weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}]`, http.StatusBadRequest},
		{"省略 weight", `[{"skuName":"s1","unit":"3301","price":"100","stockQty":"5","lengthCm":"10","widthCm":"10","heightCm":"10"}]`, http.StatusBadRequest},
		{"省略 lengthCm", `[{"skuName":"s1","unit":"3301","price":"100","stockQty":"5","weight":"1","widthCm":"10","heightCm":"10"}]`, http.StatusBadRequest},
		// 零 SKU 商品會被建成「上架但不可購買」（priceMin=0 / stock=0）
		{"空陣列", `[]`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			mock := &mockProductServiceClient{
				CreateProductFn: func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
					called = true
					return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
				},
			}
			h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}

			r := gin.New()
			app := r.Group("/api/v1/app")
			app.Use(fakeSellerAuth())
			app.POST("/products", h.CreateSellerProduct)

			body := `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1","skus":` + tc.skusJSON + `}`
			req := httptest.NewRequest("POST", "/api/v1/app/products", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "99")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP status = %d, want %d; body = %s", w.Code, tc.wantStatus, w.Body.String())
			}
			// 被擋下時不得打到 product-service：驗證必須在 Gateway 就結束
			if tc.wantStatus == http.StatusBadRequest && called {
				t.Error("請求已被驗證擋下，product-service 不應被呼叫")
			}
		})
	}
}

// 更新走 PATCH 語意，必填集合與建立不同（dev 實測：只帶 id + 材積可成功更新），
// 因此 UpdateSkuInput 不得沿用 CreateSkuInput 的必填標籤。
func TestUpdateSellerProduct_SkuPatchSemantics(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			"只帶 id + 重量材積（省略 skuName/unit/price/stockQty）→ 允許",
			`{"idempotencyKey":"k1","skus":[{"id":"356","weight":"2","lengthCm":"11","widthCm":"11","heightCm":"11"}]}`,
			http.StatusOK,
		},
		{
			"完全不帶 skus（只改名稱）→ 允許，且不動 SKU",
			`{"idempotencyKey":"k1","name":"改名"}`,
			http.StatusOK,
		},
		{
			"帶了 SKU 卻省略 weight → 擋下（product-service 會讀成 0 並回 400）",
			`{"idempotencyKey":"k1","skus":[{"id":"356","lengthCm":"11","widthCm":"11","heightCm":"11"}]}`,
			http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockProductServiceClient{
				UpdateProductFn: func(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
					return &productpb.UpdateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
				},
			}
			h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}

			r := gin.New()
			app := r.Group("/api/v1/app")
			app.Use(fakeSellerAuth())
			app.PUT("/products/:id", h.UpdateSellerProduct)

			req := httptest.NewRequest("PUT", "/api/v1/app/products/7312345678901234567", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "99")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP status = %d, want %d; body = %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}
