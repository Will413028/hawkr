package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ============================================================================
// SellerInfoItem 覆蓋矩陣
//
// 每個吐 `seller` 物件的端點都必須把 pb.SellerInfo 映射成**同一組**欄位。這在
// 過去是靠人手抄五份達成的，於是長期不一致：賣家店內列表與市集漏了
// city/district/address，store_code 則是分三次逐處補釘（commit a2419f271 的
// 說明就叫「2 missed SellerInfoItem sites」）。前端商品卡的產地是
// `origin || seller.city`，所以漏 city 不是美觀問題，是店內商品卡產地直接空白。
//
// 現在五處統一走 buildSellerInfoItem。本測試餵**同一份**上游 SellerInfo 給全部
// 端點，斷言每一端吐出的 seller 物件「鍵集合與值」完全相同——用 DeepEqual 比對
// 完整 map，所以少映一欄會紅，某一端偷偷多吐一欄也會紅。
//
// 這支測試是防「第六份手抄副本」的柵欄：日後 SellerInfoItem 新增欄位，只要有
// 任何一個端點沒跟上，這裡就會失敗。
//
// ---- 矩陣範圍：為什麼是 5 個端點而不是 7 個 ----
//
// 另有兩個端點回 ProductItem 但**整個 Seller 物件都不組**，且**刻意不納入**本矩陣：
//
//	GET /public/products/{id}/brief    （GetProductInfoByID）
//	GET /public/products/{id}/related  （GetRelatedProducts）
//
// 判準是「今天有沒有使用者看得見的缺陷」，不是「一致比較整齊」。2026-07-31 實測
// 四個前端 repo（website / admin / buyer app / seller app）：
//   - /related 零客戶端呼叫（網頁的「相關商品」是自己用 /public/search/products
//     併同分類補齊做的，不走這支）
//   - /brief 唯一呼叫者是 buyer app product_repository.dart:206 getSkuImageUrl()，
//     只讀 url / imageUrl / data，不讀任何 seller 欄位，且該方法無人呼叫（dead code）
//
// 兩者上游都帶得出 seller（product-service 公開分支已 batch 填 seller meta），
// 所以要補都只是兩行——但沒有消費者的情況下補，只是為零讀者放大回應體。
// 詳細理由與**重新評估的觸發條件**寫在兩支 handler 的呼叫點註解上（product.go）。
// 觸發條件成立時：補 buildSellerInfoItem + flatSellerFields，並把該端點加進
// matrixAssemblies()，即自動受本矩陣保護。
// ============================================================================

const matrixMediaBase = "https://media.example.test"

const (
	matrixStoreID   = "777"
	matrixLogoKey   = "public/seller/777/logo.png"
	matrixStoreName = "老王鮮魚"
)

// matrixSellerInfo 是餵給所有端點的唯一上游 fixture：pb.SellerInfo 的每個欄位
// 都填了值，好讓「某端漏映射」表現為缺鍵而不是剛好同為空值。
func matrixSellerInfo() *productpb.SellerInfo {
	return &productpb.SellerInfo{
		StoreId:   matrixStoreID,
		Name:      matrixStoreName,
		Rate:      "4.8", // 見下方 wantSeller 註解：全端一致不映射
		LogoKey:   matrixLogoKey,
		Address:   "台北市大安區",
		City:      "台北市",
		District:  "大安區",
		StoreCode: "2612345678",
	}
}

// matrixWantSeller 是 seller 物件的期望**完整**樣貌（鍵集合 + 值）。
//
// 刻意不含 `rate`：pb.SellerInfo.rate 存在，但 SellerInfoItem 沒有對應欄位，
// 五個端點一致不映射。這是全端一致的省略（不是分歧），故列為期望的一部分——
// 哪天有人只在單一端點把 rate 加上去，這裡會紅，逼他五處一起走 builder。
func matrixWantSeller() map[string]interface{} {
	return map[string]interface{}{
		"storeId":   matrixStoreID,
		"name":      matrixStoreName,
		"logoKey":   matrixLogoKey,
		"logoUrl":   mediaurl.BuildURL(matrixMediaBase, matrixLogoKey),
		"address":   "台北市大安區",
		"city":      "台北市",
		"district":  "大安區",
		"storeCode": "2612345678",
	}
}

// matrixWantFlat 是商品卡容器（ProductItem / ProductHitData）在 nested seller
// 之外另外攤平的三欄期望值。攤平值必須恆等於 nested 值。
func matrixWantFlat() map[string]interface{} {
	return map[string]interface{}{
		"sellerName":    matrixStoreName,
		"sellerLogoKey": matrixLogoKey,
		"sellerLogoUrl": mediaurl.BuildURL(matrixMediaBase, matrixLogoKey),
	}
}

// ==================== 共用小工具 ====================

func matrixListResponse() *productpb.GetProductListResponse {
	return &productpb.GetProductListResponse{
		Code: 0,
		Products: []*productpb.ProductListItem{
			{
				Id:      "4004",
				Name:    "測試商品",
				StoreId: matrixStoreID,
				Seller:  matrixSellerInfo(),
			},
		},
		Total: "1", Page: "1", PageSize: "20",
	}
}

func matrixProductMock() *mockProductServiceClient {
	return &mockProductServiceClient{
		GetProductListFn: func(ctx context.Context, in *productpb.GetProductListRequest, opts ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			return matrixListResponse(), nil
		},
		GetPublicProductFn: func(ctx context.Context, in *productpb.GetPublicProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:     "4004",
					Name:   "測試商品",
					Seller: matrixSellerInfo(),
				},
			}, nil
		},
	}
}

func matrixDoGET(t *testing.T, r *gin.Engine, path string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing data object: %v", body)
	}
	return data
}

// matrixFirstOf 取出清單型回應的第一筆記錄。
func matrixFirstOf(t *testing.T, data map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	arr, ok := data[key].([]interface{})
	if !ok {
		t.Fatalf("data.%s is not an array: %#v", key, data[key])
	}
	if len(arr) != 1 {
		t.Fatalf("data.%s length = %d, want 1", key, len(arr))
	}
	rec, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("data.%s[0] is not an object: %#v", key, arr[0])
	}
	return rec
}

func matrixSellerOf(t *testing.T, rec map[string]interface{}) map[string]interface{} {
	t.Helper()
	si, ok := rec["seller"].(map[string]interface{})
	if !ok {
		t.Fatalf("record has no nested seller object: %#v", rec)
	}
	return si
}

// matrixFlatOf 只取三個攤平欄，缺席時留空字串（omitempty 會讓漏映射的欄位消失，
// 正好讓比對顯示為空字串 vs 期望值）。
func matrixFlatOf(rec map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, k := range []string{"sellerName", "sellerLogoKey", "sellerLogoUrl"} {
		if v, ok := rec[k]; ok {
			out[k] = v
		} else {
			out[k] = ""
		}
	}
	return out
}

// ==================== 各端點 driver ====================

// assembly 描述一個 SellerInfoItem 組裝點的實測產出。
type assembly struct {
	// endpoint 是人看的名字，失敗訊息會指名道姓。
	endpoint string
	// seller 是該端點吐出的 nested seller 物件。
	seller map[string]interface{}
	// flat 是攤平便利欄；容器型別沒有這三欄時為 nil（見 hasFlat 說明）。
	flat map[string]interface{}
	// hasFlat 標記「這個端點的容器型別**應該**帶攤平欄」。
	// 唯一 false 的是商品詳情：PublicProductDetailItem 根本沒宣告這三個欄位，
	// 是容器型別的差異，不是漏抄——故列為預期例外而非失敗。
	hasFlat bool
}

func matrixAssemblies(t *testing.T) []assembly {
	t.Helper()
	return []assembly{
		driveProductList(t),
		driveProductDetail(t),
		driveSearch(t),
		driveSellerProducts(t),
	}
}

// ① /public/products —— product.go GetProductList
func driveProductList(t *testing.T) assembly {
	t.Helper()
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupProductRouter(h), "/api/v1/public/products")
	rec := matrixFirstOf(t, data, "records")
	return assembly{endpoint: "GET /public/products", seller: matrixSellerOf(t, rec), flat: matrixFlatOf(rec), hasFlat: true}
}

// ② /public/products/:id —— product.go convertToPublicProductDetail
func driveProductDetail(t *testing.T) assembly {
	t.Helper()
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupProductRouter(h), "/api/v1/public/products/4004")
	si, ok := data["seller"].(map[string]interface{})
	if !ok {
		t.Fatalf("product detail has no nested seller object: %#v", data)
	}
	return assembly{endpoint: "GET /public/products/:id", seller: si, hasFlat: false}
}

// ④ /public/search/products —— search.go buildSearchResult
func driveSearch(t *testing.T) assembly {
	t.Helper()
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupSearchRouter(h), "/api/v1/public/search/products?q=%E6%B8%AC%E8%A9%A6")
	rec := matrixFirstOf(t, data, "records")
	return assembly{endpoint: "GET /public/search/products", seller: matrixSellerOf(t, rec), flat: matrixFlatOf(rec), hasFlat: true}
}

// ⑤ /public/sellers/:id/products —— seller.go GetSellerProducts
func driveSellerProducts(t *testing.T) assembly {
	t.Helper()
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: &sellerpb.SellerInfo{
				AccountId: 42, StoreId: matrixStoreID, Name: matrixStoreName,
			}}, nil
		},
	}
	h := &Holder{
		Seller:       grpcclient.NewSellerClientForTest(sellerMock),
		Product:      grpcclient.NewProductClientForTest(matrixProductMock()),
		MediaBaseURL: matrixMediaBase,
	}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id/products", h.GetSellerProducts)
	data := matrixDoGET(t, r, "/api/v1/public/sellers/777/products")
	rec := matrixFirstOf(t, data, "records")
	return assembly{endpoint: "GET /public/sellers/:id/products", seller: matrixSellerOf(t, rec), flat: matrixFlatOf(rec), hasFlat: true}
}

// ==================== 矩陣斷言 ====================

// TestSellerInfoMatrix_NestedSellerIdenticalAcrossEndpoints 是本組的主斷言：
// 同一份上游 SellerInfo 餵進五個端點，吐出的 seller 物件必須逐鍵逐值相同。
func TestSellerInfoMatrix_NestedSellerIdenticalAcrossEndpoints(t *testing.T) {
	want := matrixWantSeller()
	for _, a := range matrixAssemblies(t) {
		if !reflect.DeepEqual(a.seller, want) {
			t.Errorf("%s: nested seller mismatch\n got = %#v\nwant = %#v\n"+
				"（所有組裝點都必須走 buildSellerInfoItem；出現分歧代表有人又手抄了一份）",
				a.endpoint, a.seller, want)
		}
	}
}

// TestSellerInfoMatrix_FlatColumnsMirrorNested 斷言攤平便利欄恆等於 nested 值，
// 且商品詳情的「沒有攤平欄」是**預期**而非漏抄。
func TestSellerInfoMatrix_FlatColumnsMirrorNested(t *testing.T) {
	wantFlat := matrixWantFlat()
	for _, a := range matrixAssemblies(t) {
		if !a.hasFlat {
			// 預期例外：PublicProductDetailItem 沒宣告攤平欄（見 flatSellerFields 註解）。
			continue
		}
		if !reflect.DeepEqual(a.flat, wantFlat) {
			t.Errorf("%s: flat seller columns mismatch\n got = %#v\nwant = %#v\n"+
				"（攤平欄必須等於 nested 值，兩者不得有兩套說法）",
				a.endpoint, a.flat, wantFlat)
		}
	}
}

// TestSellerInfoMatrix_DetailHasNoFlatColumns 把「詳情端點沒有攤平欄」這條預期
// 例外釘死。若哪天 PublicProductDetailItem 真的補上攤平欄，這裡會紅，提醒把它
// 一併納入 matrixWantFlat 的覆蓋，而不是讓例外清單默默失準。
func TestSellerInfoMatrix_DetailHasNoFlatColumns(t *testing.T) {
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupProductRouter(h), "/api/v1/public/products/4004")
	for _, k := range []string{"sellerName", "sellerLogoKey", "sellerLogoUrl"} {
		if v, ok := data[k]; ok {
			t.Errorf("product detail unexpectedly carries flat column %q = %v; "+
				"若這是刻意新增，請同步更新 assembly.hasFlat 與 matrixWantFlat", k, v)
		}
	}
}

// TestBuildSellerInfoItem_EmptyProtoFieldsCollapseToOmitted 釘住 builder 的空值
// 語意：proto 空字串收斂成 nil 指標，讓 omitempty 真的把欄位拿掉，而不是吐
// `"city": ""` 之類需要前端再判一次的雜訊。StoreID 是非指標欄位，恆存在。
func TestBuildSellerInfoItem_EmptyProtoFieldsCollapseToOmitted(t *testing.T) {
	si := buildSellerInfoItem(matrixMediaBase, &productpb.SellerInfo{StoreId: matrixStoreID})
	if si == nil {
		t.Fatal("buildSellerInfoItem returned nil for a non-nil proto")
	}
	if si.StoreID != matrixStoreID {
		t.Errorf("StoreID = %q, want %q", si.StoreID, matrixStoreID)
	}
	for name, got := range map[string]*string{
		"Name": si.Name, "LogoKey": si.LogoKey, "LogoUrl": si.LogoUrl,
		"Address": si.Address, "City": si.City, "District": si.District,
		"StoreCode": si.StoreCode,
	} {
		if got != nil {
			t.Errorf("%s = %q, want nil (empty proto string must collapse to an omitted field)", name, *got)
		}
	}
}

// TestBuildSellerInfoItem_NilProtoYieldsNil 釘住 nil 傳遞語意：上游沒有 seller
// meta 時回 nil，呼叫端直接指派即可讓 `seller` 欄位整個省略。
func TestBuildSellerInfoItem_NilProtoYieldsNil(t *testing.T) {
	if si := buildSellerInfoItem(matrixMediaBase, nil); si != nil {
		t.Errorf("buildSellerInfoItem(nil) = %#v, want nil", si)
	}
}
