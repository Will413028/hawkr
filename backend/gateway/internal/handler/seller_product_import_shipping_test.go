package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	importpb "github.com/Will413028/hawkr/go-pkg/proto/product_import"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type importItemShippingMock struct {
	importpb.ProductImportServiceClient
}

func (importItemShippingMock) GetImportItem(_ context.Context, _ *importpb.GetImportItemRequest, _ ...grpc.CallOption) (*importpb.GetImportItemResponse, error) {
	return &importpb.GetImportItemResponse{
		Data: &importpb.ImportItemDetail{
			Skus: []*importpb.ImportItemSkuDto{{
				SkuId: "s1",
				ShippingMethods: []*importpb.ShippingMethodDto{
					{Method: "TAQBIN", Temperature: "CHILLED"},
				},
			}},
		},
	}, nil
}

func TestGetImportItem_ShippingMethodsCarryNoTemperature(t *testing.T) {
	h := &Holder{
		ProductImport: grpcclient.NewProductImportClientForTest(importItemShippingMock{}),
		Store:         storeResolverForTest(rekeyTestSurrogate),
	}
	r := gin.New()
	r.Group("/api/v1/app", fakeJWTAuth()).GET("/product_import_jobs/:import_job_id/items/:import_item_id", h.GetImportItem)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/app/product_import_jobs/j1/items/i1", nil)
	req.Header.Set("X-Account-Id", strconv.FormatInt(rekeyTestAccountID, 10))
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, 200)

	var body struct {
		Data struct {
			Skus []struct {
				ShippingMethods []map[string]any `json:"shippingMethods"`
			} `json:"skus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Skus) != 1 || len(body.Data.Skus[0].ShippingMethods) != 1 {
		t.Fatalf("unexpected shape: %s", w.Body.String())
	}
	got := body.Data.Skus[0].ShippingMethods[0]
	if _, has := got["temperature"]; has {
		t.Errorf("import shippingMethods must not expose temperature: %v", got)
	}
	if got["method"] != "TAQBIN" {
		t.Errorf("method = %v, want TAQBIN", got["method"])
	}
}
