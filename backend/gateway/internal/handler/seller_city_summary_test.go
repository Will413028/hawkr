package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func TestGetSellerCitiesSummary_MapsCompleteCountsAndCachePolicy(t *testing.T) {
	mock := &mockProductServiceClient{GetPublicSellerCitiesSummaryFn: func(_ context.Context, _ *productpb.GetPublicSellerCitiesSummaryRequest, _ ...grpc.CallOption) (*productpb.GetPublicSellerCitiesSummaryResponse, error) {
		return &productpb.GetPublicSellerCitiesSummaryResponse{Code: 0, Message: "success", Records: []*productpb.SellerCitySummaryItem{{
			CityCode: "TW-TPE", CityName: "台北市", ProductCount: "115", SellerCount: "7",
			PreviewStoreIds: []string{"101", "102", "103", "104", "105", "106"},
		}}}, nil
	}}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock)}
	r := gin.New()
	r.GET("/api/v1/public/seller-cities/summary", h.GetSellerCitiesSummary)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/seller-cities/summary", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=300, stale-while-revalidate=3600" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var body struct {
		Data struct {
			Records []SellerCitySummaryItem `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Records) != 1 || body.Data.Records[0].ProductCount != "115" || body.Data.Records[0].SellerCount != "7" {
		t.Fatalf("body = %+v", body)
	}
}
