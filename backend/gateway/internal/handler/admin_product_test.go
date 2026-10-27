package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// resolverThatFailsTest 若被呼叫就讓測試失敗——用來斷言 owner_id=0（不篩選擁有者）
// 時 handler 完全不該去解析 resolver。
func resolverThatFailsTest(t *testing.T) *storeid.Resolver {
	t.Helper()
	return storeid.New(storeid.GetterFunc(func(context.Context, int64) (int64, error) {
		t.Fatal("resolver must not be called when owner_id is 0")
		return 0, nil
	}))
}

func setupAdminProductRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/admin/products", h.SearchAdminProducts)
	return r
}

func TestSearchAdminProducts_ResolvesOwnerIDToCurrentSurrogate(t *testing.T) {
	var gotOwnerID int64
	mock := &mockProductServiceClient{
		GetProductListFn: func(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			gotOwnerID = in.OwnerStoreId
			return &productpb.GetProductListResponse{Code: 0, Total: "0", Page: "1", PageSize: "10"}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/products?owner_id=2600000001", nil)
	setupAdminProductRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotOwnerID != rekeyTestSurrogate {
		t.Errorf("GetProductListRequest.OwnerStoreId = %d, want resolved surrogate %d", gotOwnerID, rekeyTestSurrogate)
	}
}

// 該帳號目前沒有現任賣家（已刪店未重開）→ 回空清單，不是 403 或錯誤（admin 查詢
// 容許查到「沒店」的帳號，不該讓管理員以為 API 壞了）。
func TestSearchAdminProducts_NoCurrentSeller_ReturnsEmptyList(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductListFn: func(context.Context, *productpb.GetProductListRequest, ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			t.Fatal("product-service must not be called when owner has no current seller")
			return nil, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: noStoreResolverForTest()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/products?owner_id=2600000001", nil)
	setupAdminProductRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data field")
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 0 {
		t.Errorf("records = %v, want empty list", data["records"])
	}
}

// owner_id 沒帶（=0）：不篩選擁有者，不呼叫 resolver，維持既有行為。
func TestSearchAdminProducts_NoOwnerFilter_SkipsResolver(t *testing.T) {
	var called bool
	mock := &mockProductServiceClient{
		GetProductListFn: func(_ context.Context, in *productpb.GetProductListRequest, _ ...grpc.CallOption) (*productpb.GetProductListResponse, error) {
			called = true
			if in.OwnerStoreId != 0 {
				t.Errorf("OwnerStoreId = %d, want 0 (no owner filter)", in.OwnerStoreId)
			}
			return &productpb.GetProductListResponse{Code: 0, Total: "0", Page: "1", PageSize: "10"}, nil
		},
	}
	h := &Holder{
		Product: grpcclient.NewProductClientForTest(mock),
		Store:   resolverThatFailsTest(t),
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/products", nil)
	setupAdminProductRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if !called {
		t.Error("product-service was never called")
	}
}
