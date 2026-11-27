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


const matrixMediaBase = "https://media.example.test"

const (
	matrixStoreID   = "777"
	matrixLogoKey   = "public/seller/777/logo.png"
	matrixStoreName = "老王鮮魚"
)

func matrixSellerInfo() *productpb.SellerInfo {
	return &productpb.SellerInfo{
		StoreId:   matrixStoreID,
		Name:      matrixStoreName,
		Rate:      "4.8",
		LogoKey:   matrixLogoKey,
		Address:   "台北市大安區",
		City:      "台北市",
		District:  "大安區",
		StoreCode: "2612345678",
	}
}

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

func matrixWantFlat() map[string]interface{} {
	return map[string]interface{}{
		"sellerName":    matrixStoreName,
		"sellerLogoKey": matrixLogoKey,
		"sellerLogoUrl": mediaurl.BuildURL(matrixMediaBase, matrixLogoKey),
	}
}


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


type assembly struct {
	endpoint string
	seller map[string]interface{}
	flat map[string]interface{}
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

func driveProductList(t *testing.T) assembly {
	t.Helper()
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupProductRouter(h), "/api/v1/public/products")
	rec := matrixFirstOf(t, data, "records")
	return assembly{endpoint: "GET /public/products", seller: matrixSellerOf(t, rec), flat: matrixFlatOf(rec), hasFlat: true}
}

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

func driveSearch(t *testing.T) assembly {
	t.Helper()
	h := &Holder{Product: grpcclient.NewProductClientForTest(matrixProductMock()), MediaBaseURL: matrixMediaBase}
	data := matrixDoGET(t, setupSearchRouter(h), "/api/v1/public/search/products?q=%E6%B8%AC%E8%A9%A6")
	rec := matrixFirstOf(t, data, "records")
	return assembly{endpoint: "GET /public/search/products", seller: matrixSellerOf(t, rec), flat: matrixFlatOf(rec), hasFlat: true}
}

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
