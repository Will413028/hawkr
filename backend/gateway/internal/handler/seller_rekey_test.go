package handler

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	importpb "github.com/Will413028/hawkr/go-pkg/proto/product_import"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock proto clients（介面 embed：未 stub 的方法呼叫即 nil-panic）====================

type mockProductImportServiceClient struct {
	importpb.ProductImportServiceClient
	ListImportJobsFn func(ctx context.Context, in *importpb.ListImportJobsRequest, opts ...grpc.CallOption) (*importpb.ListImportJobsResponse, error)
}

func (m *mockProductImportServiceClient) ListImportJobs(ctx context.Context, in *importpb.ListImportJobsRequest, opts ...grpc.CallOption) (*importpb.ListImportJobsResponse, error) {
	return m.ListImportJobsFn(ctx, in, opts...)
}

func setupSellerRekeyRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app", fakeJWTAuth())
	{
		app.GET("/products/:id", h.GetSellerProductDetail)
		app.DELETE("/products/:id", h.DeleteSellerProduct)
		app.GET("/products/:id/skus", h.GetSellerProductSkus)
		app.GET("/product_import_jobs", h.ListImportJobs)
		app.GET("/shipping_methods", h.ListSellerShippingMethods)
	}
	return r
}

const (
	rekeyTestAccountID = int64(2600000001)
	rekeyTestSurrogate = int64(7300000000000000123)
)

func doRekeyReq(t *testing.T, r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Account-Id", strconv.FormatInt(rekeyTestAccountID, 10))
	r.ServeHTTP(w, req)
	return w
}

// GetSellerProductDetail：request 欄位帶 surrogate；:630 guard 以 surrogate 比對 CreatedBy
func TestGetSellerProductDetail_ComparesCreatedByAgainstSurrogate(t *testing.T) {
	var gotReqAccountID int64
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			gotReqAccountID = in.StoreId
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:        in.ProductId,
					CreatedBy: strconv.FormatInt(rekeyTestSurrogate, 10),
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/products/7301111111111111111")

	assertHTTPStatus(t, w, 200)
	if gotReqAccountID != rekeyTestSurrogate {
		t.Errorf("GetProductRequest.StoreId = %d, want surrogate %d", gotReqAccountID, rekeyTestSurrogate)
	}
}

func TestGetSellerProductDetail_RawAccountIDDoesNotMatchOwner(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id: in.ProductId,
					// 別人的商品：owner surrogate ≠ 本帳號的 surrogate
					CreatedBy: strconv.FormatInt(rekeyTestSurrogate+1, 10),
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/products/7301111111111111111")

	assertHTTPStatus(t, w, 403)
	assertCode(t, parseJSON(t, w.Body), 403)
}

// GetSellerProductSkus：request 欄位帶 surrogate；owner guard 以 surrogate 比對 CreatedBy
// （#1558 跟進：ListSellerSkus 原本 /app 路由漏用 resolver，重開過店的賣家會鎖死自己非上架中商品的 SKU）
func TestGetSellerProductSkus_PassesSurrogate(t *testing.T) {
	var gotReqAccountID int64
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			gotReqAccountID = in.StoreId
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:        in.ProductId,
					CreatedBy: strconv.FormatInt(rekeyTestSurrogate, 10),
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/products/7301111111111111111/skus")

	assertHTTPStatus(t, w, 200)
	if gotReqAccountID != rekeyTestSurrogate {
		t.Errorf("GetProductRequest.StoreId = %d, want surrogate %d", gotReqAccountID, rekeyTestSurrogate)
	}
}

func TestGetSellerProductSkus_RawAccountIDDoesNotMatchOwner(t *testing.T) {
	mock := &mockProductServiceClient{
		GetProductFn: func(_ context.Context, in *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
			return &productpb.GetProductResponse{
				Code: 0,
				Data: &productpb.ProductDetail{
					Id:        in.ProductId,
					CreatedBy: strconv.FormatInt(rekeyTestSurrogate+1, 10),
				},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/products/7301111111111111111/skus")

	assertHTTPStatus(t, w, 403)
	assertCode(t, parseJSON(t, w.Body), 403)
}

func TestDeleteSellerProduct_PassesSurrogate(t *testing.T) {
	var gotReqAccountID int64
	mock := &mockProductServiceClient{
		DeleteProductFn: func(_ context.Context, in *productpb.DeleteProductRequest, _ ...grpc.CallOption) (*productpb.DeleteProductResponse, error) {
			gotReqAccountID = in.StoreId
			return &productpb.DeleteProductResponse{Code: 0}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "DELETE", "/api/v1/app/products/7301111111111111111")

	assertHTTPStatus(t, w, 200)
	if gotReqAccountID != rekeyTestSurrogate {
		t.Errorf("DeleteProductRequest.StoreId = %d, want surrogate %d", gotReqAccountID, rekeyTestSurrogate)
	}
}

func TestListImportJobs_PassesSurrogate(t *testing.T) {
	var gotReqAccountID int64
	mock := &mockProductImportServiceClient{
		ListImportJobsFn: func(_ context.Context, in *importpb.ListImportJobsRequest, _ ...grpc.CallOption) (*importpb.ListImportJobsResponse, error) {
			gotReqAccountID = in.StoreId
			return &importpb.ListImportJobsResponse{Code: 0}, nil
		},
	}
	h := &Holder{ProductImport: grpcclient.NewProductImportClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/product_import_jobs")

	assertHTTPStatus(t, w, 200)
	if gotReqAccountID != rekeyTestSurrogate {
		t.Errorf("ListImportJobsRequest.StoreId = %d, want surrogate %d", gotReqAccountID, rekeyTestSurrogate)
	}
}

func TestListSellerShippingMethods_PassesSurrogate(t *testing.T) {
	var gotReqAccountID int64
	mock := &mockShippingServiceClient{
		ListSellerShippingMethodsFn: func(_ context.Context, in *shippingpb.ListSellerShippingMethodsRequest, _ ...grpc.CallOption) (*shippingpb.ListSellerShippingMethodsResponse, error) {
			gotReqAccountID = in.StoreId
			return &shippingpb.ListSellerShippingMethodsResponse{Code: 0}, nil
		},
	}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(mock), Store: storeResolverForTest(rekeyTestSurrogate)}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "GET", "/api/v1/app/shipping_methods")

	assertHTTPStatus(t, w, 200)
	if gotReqAccountID != rekeyTestSurrogate {
		t.Errorf("ListSellerShippingMethodsRequest.StoreId = %d, want surrogate %d", gotReqAccountID, rekeyTestSurrogate)
	}
}

// 帳號目前非賣家（無 current stint）→ 403，沿用鄰近 handler 的 Forbidden 慣例
func TestSellerRekey_NoCurrentSeller_Forbidden(t *testing.T) {
	h := &Holder{
		Product: grpcclient.NewProductClientForTest(&mockProductServiceClient{}),
		Store:   noStoreResolverForTest(),
	}
	w := doRekeyReq(t, setupSellerRekeyRouter(h), "DELETE", "/api/v1/app/products/7301111111111111111")

	assertHTTPStatus(t, w, 403)
	assertCode(t, parseJSON(t, w.Body), 403)
}
