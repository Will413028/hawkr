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

// 商品描述 block 的 imageKey（對外欄名；proto 側仍叫 object_key）是前端可控寫入面
// （49 號 §25 / 10 檔 §25）。
// product-service 只在 stagedToken finalize 成功時覆寫該欄位，因此任何未經正規化
// 的值會直接落進 products.description，並隨「載入→再送出」自我複製。
// 以下測試釘住 Gateway 邊界的四種輸入行為：裸 key 原樣、我方 URL 剝殼、外來域名 400、
// 非白名單前綴 400。

const descKeyTestMediaBase = "https://test-media.example.com"

func setupSellerProductDescRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.POST("/products", h.CreateSellerProduct)
	app.PUT("/products/:id", h.UpdateSellerProduct)
	return r
}

func descHolderWithCapture(t *testing.T) (*Holder, func() *productpb.ProductDescription, func() bool) {
	t.Helper()
	var createDesc, updateDesc *productpb.ProductDescription
	called := false
	mock := &mockProductServiceClient{
		CreateProductFn: func(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
			called = true
			createDesc = in.Description
			return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
		},
		UpdateProductFn: func(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
			called = true
			updateDesc = in.Description
			return &productpb.UpdateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
		},
	}
	h := &Holder{
		Product:      grpcclient.NewProductClientForTest(mock),
		Store:        storeResolverForTest(7001),
		MediaBaseURL: descKeyTestMediaBase,
	}
	captured := func() *productpb.ProductDescription {
		if createDesc != nil {
			return createDesc
		}
		return updateDesc
	}
	return h, captured, func() bool { return called }
}

// minimalSku 是通過 CreateProductRequest 驗證所需的最小 SKU（建立商品必填至少一個 SKU，
// 且每個 SKU 的 skuName/unit/price/stockQty/weight/材積都必填——見 CreateSkuInput）。
// 本檔測的是 description objectKey 的處理，SKU 只是讓請求合法的背景資料。
const minimalSku = `{"skuName":"s1","unit":"3301","price":"100","stockQty":"5",` +
	`"weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}`

func descBody(objectKey string) string {
	return `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1","skus":[` + minimalSku + `],` +
		`"description":{"version":"1","blocks":[` +
		`{"type":"text","id":"b0","content":"說明"},` +
		`{"type":"image","id":"b1","imageKey":"` + objectKey + `","alt":"圖"}]}}`
}

func doDescRequest(t *testing.T, h *Holder, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := setupSellerProductDescRouter(h)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 裸 objectKey：原樣透傳（不得被誤改）。
func TestSellerProductDescription_BareKey_PassThrough(t *testing.T) {
	const bare = "public/product/20260805/abc.png"
	for _, tc := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/v1/app/products"},
		{"update", http.MethodPut, "/api/v1/app/products/7312345678901234567"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, captured, called := descHolderWithCapture(t)
			w := doDescRequest(t, h, tc.method, tc.path, descBody(bare))
			assertHTTPStatus(t, w, http.StatusOK)
			if !called() {
				t.Fatal("product-service 未被呼叫")
			}
			desc := captured()
			if desc == nil || len(desc.Blocks) != 2 {
				t.Fatalf("description 未正確轉換: %+v", desc)
			}
			if got := desc.Blocks[1].ObjectKey; got != bare {
				t.Errorf("objectKey = %q, want %q", got, bare)
			}
		})
	}
}

// 我方 media host 的完整顯示 URL（website 實測落庫的髒值形狀）：必須剝殼成裸 key。
func TestSellerProductDescription_OwnHostURL_StrippedToBareKey(t *testing.T) {
	const full = "https://test-media.example.com/i/original/public/product/20260805/abc.png"
	const bare = "public/product/20260805/abc.png"
	for _, tc := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/v1/app/products"},
		{"update", http.MethodPut, "/api/v1/app/products/7312345678901234567"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, captured, called := descHolderWithCapture(t)
			w := doDescRequest(t, h, tc.method, tc.path, descBody(full))
			assertHTTPStatus(t, w, http.StatusOK)
			if !called() {
				t.Fatal("product-service 未被呼叫")
			}
			desc := captured()
			if desc == nil || len(desc.Blocks) != 2 {
				t.Fatalf("description 未正確轉換: %+v", desc)
			}
			if got := desc.Blocks[1].ObjectKey; got != bare {
				t.Errorf("objectKey = %q, want 剝殼後的裸 key %q", got, bare)
			}
		})
	}
}

// 外來域名：400，且不得呼叫 product-service（不落庫）。
func TestSellerProductDescription_ForeignDomain_Rejected(t *testing.T) {
	for _, tc := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/v1/app/products"},
		{"update", http.MethodPut, "/api/v1/app/products/7312345678901234567"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, called := descHolderWithCapture(t)
			w := doDescRequest(t, h, tc.method, tc.path, descBody("https://evil.example.com/x.png"))
			assertHTTPStatus(t, w, http.StatusBadRequest)
			if called() {
				t.Error("外來域名應被擋，不得呼叫 product-service")
			}
		})
	}
}

// 非白名單前綴（機敏區 secure/）：400，且不得呼叫 product-service。
func TestSellerProductDescription_NonWhitelistedPrefix_Rejected(t *testing.T) {
	for _, tc := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/api/v1/app/products"},
		{"update", http.MethodPut, "/api/v1/app/products/7312345678901234567"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, called := descHolderWithCapture(t)
			w := doDescRequest(t, h, tc.method, tc.path, descBody("secure/kyc/7001/id-card-front.jpg"))
			assertHTTPStatus(t, w, http.StatusBadRequest)
			if called() {
				t.Error("非白名單前綴應被擋，不得呼叫 product-service")
			}
		})
	}
}

// 路徑穿越：400（ValidateObjectKey 的 .. 檢查在描述面同樣生效）。
func TestSellerProductDescription_PathTraversal_Rejected(t *testing.T) {
	h, _, called := descHolderWithCapture(t)
	w := doDescRequest(t, h, http.MethodPost, "/api/v1/app/products", descBody("public/../secure/x.png"))
	assertHTTPStatus(t, w, http.StatusBadRequest)
	if called() {
		t.Error("路徑穿越應被擋，不得呼叫 product-service")
	}
}

// staged token 新上傳分支：objectKey 為空，正規化必須 no-op（不得誤擋）。
func TestSellerProductDescription_StagedToken_NotBlocked(t *testing.T) {
	h, captured, called := descHolderWithCapture(t)
	body := `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1","skus":[` + minimalSku + `],"description":{"version":"1","blocks":[` +
		`{"type":"image","id":"b1","stagedToken":"tok-a"}]}}`
	w := doDescRequest(t, h, http.MethodPost, "/api/v1/app/products", body)
	assertHTTPStatus(t, w, http.StatusOK)
	if !called() {
		t.Fatal("staged token 分支不得被擋")
	}
	desc := captured()
	if desc == nil || len(desc.Blocks) != 1 {
		t.Fatalf("description 未正確轉換: %+v", desc)
	}
	if desc.Blocks[0].ObjectKey != "" {
		t.Errorf("objectKey = %q, want 空字串", desc.Blocks[0].ObjectKey)
	}
	if desc.Blocks[0].StagedToken != "tok-a" {
		t.Errorf("stagedToken 未透傳: %q", desc.Blocks[0].StagedToken)
	}
}
