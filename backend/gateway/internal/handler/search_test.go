package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Router Setup ====================

func setupSearchRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public/search")
	{
		pub.GET("/products", h.SearchProductsViaES)
		pub.GET("/suggest", h.GetSearchSuggestions)
	}
	return r
}

// ==================== SearchProductsViaES ====================

// TestSearchProductsViaES_SellerLogoURLField proves the nested seller object
// in /public/search/products responses exposes the seller logo as the canonical
// camelCase pair logoKey/logoUrl (B-238 / issue #1782; 49 §6.1) — search
// forwards to product-service.GetPublicProductList, the same shared
// SellerInfoItem type populated by /public/products. snake_case
// logo_key/logo_url is never emitted.
func TestSearchProductsViaES_SellerLogoURLField(t *testing.T) {
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
	router := setupSearchRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/search/products?q=蕃茄", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	products, ok := data["records"].([]interface{})
	if !ok || len(products) != 1 {
		t.Fatalf("products length = %d, want 1", len(products))
	}
	seller, ok := products[0].(map[string]interface{})["seller"].(map[string]interface{})
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

// TestSearchProductsViaES_CarriesStoreCode pins the seller display code onto
// /public/search/products (web#365 storefront canonicalization). Search forwards
// to the very same product-service GetProductList RPC that already feeds
// /public/products' storeCode, so the field is enriched upstream and any absence
// here is a silent drop inside buildSearchResult — the frontend link helper then
// falls back to the numeric store id and every seller link from a search result
// costs one 308 redirect. A shape-only assertion (seller object present) would
// not catch it, hence the exact-value check.
func TestSearchProductsViaES_CarriesStoreCode(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "1",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{
						Id:      "prod-1",
						Name:    "蕃茄",
						StoreId: "42",
						Seller: &productpb.SellerInfo{
							StoreId:   "42",
							StoreCode: "2612345678",
							Name:      "老王鮮魚",
							LogoKey:   "public/seller/42/logo.png",
							City:      "台北市",
							District:  "大安區",
						},
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupSearchRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/search/products?q=蕃茄", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	products, ok := data["records"].([]interface{})
	if !ok || len(products) != 1 {
		t.Fatalf("products length = %d, want 1", len(products))
	}
	seller, ok := products[0].(map[string]interface{})["seller"].(map[string]interface{})
	if !ok {
		t.Fatal("missing seller field")
	}
	if got := seller["storeCode"]; got != "2612345678" {
		t.Errorf("products[0].seller.storeCode = %v, want %q", got, "2612345678")
	}
}

// TestSearchProductsViaES_JSONKeysAreCamelCase pins §2 的 JSON body 命名（camelCase）
// 在 /public/search/products 上。
//
// 這支端點是全站唯一長期吐 snake_case body key 的地方（category_id / price_min /
// store_id / page_size …），而它的設計意圖明寫著「與 /public/products 相同的
// deserialization」——同一張商品卡在兩支端點得用兩套 key 名讀。
//
// 斷言分兩面，缺一不可：正面確認 camelCase 有值（漏映射會被抓），反面掃描整個
// data 物件確認舊 snake key 一個都不剩（只驗正面的話，同時吐新舊兩套 key 的
// 相容層照樣通過，而本專案禁止相容層）。
func TestSearchProductsViaES_JSONKeysAreCamelCase(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return &productpb.GetProductListResponse{
				Code:     0,
				Total:    "1",
				Page:     "1",
				PageSize: "20",
				Products: []*productpb.ProductListItem{
					{
						Id:            "prod-1",
						Name:          "蕃茄",
						StoreId:       "42",
						CategoryId:    "7",
						CategoryName:  "蔬果",
						PriceMin:      "30",
						PriceMax:      "80",
						CreatedAt:     "2026-01-02T03:04:05Z",
						ReviewCount:   "9",
						StockQty:      "11",
						SalesCount:    "13",
						FavoriteCount: "17",
						DeliveryTemp:  []string{"3201"},
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	router := setupSearchRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/search/products?q=蕃茄", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	products, ok := data["records"].([]interface{})
	if !ok || len(products) != 1 {
		t.Fatalf("products length = %d, want 1", len(products))
	}
	hit, ok := products[0].(map[string]interface{})
	if !ok {
		t.Fatal("products[0] is not an object")
	}

	// 正面：每個 camelCase key 都要帶到值（值互不相同，欄位對調會在這裡失敗）。
	for key, want := range map[string]interface{}{
		"categoryId":    "7",
		"categoryName":  "蔬果",
		"priceMin":      "30",
		"priceMax":      "80",
		"createdAt":     "2026-01-02T03:04:05Z",
		"reviewCount":   "9",
		"storeId":       "42",
		"stock":         "11",
		"salesCount":    "13",
		"favoriteCount": "17",
	} {
		if got := hit[key]; got != want {
			t.Errorf("products[0][%q] = %v, want %v", key, got, want)
		}
	}
	if got, ok := hit["deliveryTemp"].([]interface{}); !ok || len(got) != 1 || got[0] != "3201" {
		t.Errorf("products[0][\"deliveryTemp\"] = %v, want [\"3201\"]", hit["deliveryTemp"])
	}
	// 分頁四欄一律 string（§4），型別斷言直接寫 string——數字化回歸會轉紅。
	for field, want := range map[string]string{
		"total": "1", "page": "1", "pageSize": "20", "totalPages": "1",
	} {
		got, ok := data[field].(string)
		if !ok || got != want {
			t.Errorf("data[%q] = %v (%T), want string %q", field, data[field], data[field], want)
		}
	}
	// 分頁 key 用 records（§16），舊的 products 必須零命中。
	if v, present := data["products"]; present {
		t.Errorf("data 仍帶舊分頁 key \"products\" = %v；已改為 records", v)
	}

	// 反面：舊的 snake_case key 在 hit 與分頁層都必須零命中。
	for _, stale := range []string{
		"category_id", "category_name", "price_min", "price_max", "created_at",
		"review_count", "store_id", "account_id", "sales_count", "favorite_count",
		"delivery_temp", "shipping_methods",
	} {
		if v, present := hit[stale]; present {
			t.Errorf("products[0] still carries snake_case key %q = %v", stale, v)
		}
	}
	for _, stale := range []string{"page_size", "total_pages"} {
		if v, present := data[stale]; present {
			t.Errorf("data still carries snake_case key %q = %v", stale, v)
		}
	}
}

// TestSearchProductsViaES_GalleryKeepsObjectKeyWithoutAssetID 釘住公開搜尋結果的 gallery 形狀
// （02 §2.2.2）：只藏 assetId（媒體資產 TSID＝內部識別碼），objectKey 必須留下。
//
// 為什麼 objectKey 一定要留：同一筆結果的 imageKeys 本來就列出全部 objectKey，藏 gallery 那一份
// 保護不到任何東西；但少了 key，前端就只能拿 imageUrl（/i/original/）渲染縮圖，
// 搜尋結果一整頁都被迫載原圖。
func TestSearchProductsViaES_GalleryKeepsObjectKeyWithoutAssetID(t *testing.T) {
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
						Gallery: []*productpb.GalleryItem{
							{AssetId: "asset-1", ObjectKey: "public/p/1.jpg"},
						},
					},
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), MediaBaseURL: "https://media.test"}
	router := setupSearchRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/search/products?q=蕃茄", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	data := resp["data"].(map[string]interface{})
	products, ok := data["records"].([]interface{})
	if !ok || len(products) != 1 {
		t.Fatalf("products length = %d, want 1", len(products))
	}
	gallery, ok := products[0].(map[string]interface{})["gallery"].([]interface{})
	if !ok || len(gallery) != 1 {
		t.Fatalf("gallery length = %d, want 1", len(gallery))
	}
	item := gallery[0].(map[string]interface{})
	if got := item["imageKey"]; got != "public/p/1.jpg" {
		t.Errorf("gallery[0].imageKey = %v, want %q", got, "public/p/1.jpg")
	}
	if got := item["imageUrl"]; got != "https://media.test/i/original/public/p/1.jpg" {
		t.Errorf("gallery[0].imageUrl = %v, want the Gateway-built original URL", got)
	}
	for _, stale := range []string{"objectKey", "url"} {
		if v, present := item[stale]; present {
			t.Errorf("gallery[0] 不得再出現舊欄名 %q, got %v", stale, v)
		}
	}
	if v, present := item["assetId"]; present {
		t.Errorf("public search gallery must not expose assetId, got %v", v)
	}
}
