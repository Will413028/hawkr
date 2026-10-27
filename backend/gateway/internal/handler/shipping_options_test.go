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

func setupShippingQuotesRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.POST("/shipping_quotes", h.CalculateShippingOptions)
	return r
}

const shippingQuoteBody = `{"length":"10","width":"10","height":"10","weight":"1","weightUnit":"KG"}`

func TestCalculateShippingOptions_SendsResolvedStoreSurrogate(t *testing.T) {
	var captured *productpb.CalculateShippingOptionsRequest
	mock := &mockProductServiceClient{
		CalculateShippingOptionsFn: func(ctx context.Context, in *productpb.CalculateShippingOptionsRequest, opts ...grpc.CallOption) (*productpb.CalculateShippingOptionsResponse, error) {
			captured = in
			return &productpb.CalculateShippingOptionsResponse{
				Code: 0,
				Data: &productpb.ShippingCalculationData{},
			}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}
	r := setupShippingQuotesRouter(h)

	req := httptest.NewRequest("POST", "/api/v1/app/shipping_quotes", strings.NewReader(shippingQuoteBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("product-service was not called")
	}
	if captured.StoreId != 7001 {
		t.Errorf("StoreId = %d, want 7001 (resolved store surrogate, not login account_id 99)", captured.StoreId)
	}
}

// 無現任賣家時必須 403（與 requireSeller 同語意），且不得帶著未解析的 id 打下游。
func TestCalculateShippingOptions_NoCurrentSeller403(t *testing.T) {
	mock := &mockProductServiceClient{
		CalculateShippingOptionsFn: func(ctx context.Context, in *productpb.CalculateShippingOptionsRequest, opts ...grpc.CallOption) (*productpb.CalculateShippingOptionsResponse, error) {
			t.Fatal("product-service must not be called without a current seller")
			return nil, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: noStoreResolverForTest()}
	r := setupShippingQuotesRouter(h)

	req := httptest.NewRequest("POST", "/api/v1/app/shipping_quotes", strings.NewReader(shippingQuoteBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	assertCode(t, parseJSON(t, w.Body), 403)
}
