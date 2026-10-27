package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"google.golang.org/grpc"
)

func TestForecastComparisonMissingP50IsNullAndOthersUnchanged(t *testing.T) {
	zero := 0.0
	mock := &mockPricebookServiceClient{GetForecastCurveFn: func(_ context.Context, req *pb.GetForecastCurveRequest, _ ...grpc.CallOption) (*pb.GetForecastCurveResponse, error) {
		if req.GetCropCode() != "LA1" || req.MarketCode != nil {
			t.Fatalf("all-market request changed: %v", req)
		}
		return &pb.GetForecastCurveResponse{
			Status: "ready", ModelVersion: "test-model",
			Points: []*pb.ForecastCurvePoint{{TargetDate: "2026-09-09", HorizonDays: 1, Status: "market_closed"}},
			MarketComparison: []*pb.MarketComparisonEntry{
				{MarketCode: "109", MarketName: "台北", Status: "unavailable"},
				{MarketCode: "104", MarketName: "台北二", P50: &zero, Status: "ready"},
			},
		}, nil
	}}
	r := setupPricebookRouter(&Holder{Pricebook: grpcclient.NewPricebookClientForTest(mock)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/app/sellers/pricebook/forecast-curve?cropCode=LA1", nil))
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data["marketComparison"], &entries); err != nil {
		t.Fatal(err)
	}
	if string(entries[0]["p50"]) != "null" {
		t.Fatalf("missing p50 must be explicit null: %s", w.Body.String())
	}
	if string(entries[1]["p50"]) != "0" {
		t.Fatalf("real zero changed: %s", w.Body.String())
	}
	if string(entries[0]["status"]) != `"unavailable"` {
		t.Fatalf("status lost: %s", w.Body.String())
	}
	var points []map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data["points"], &points); err != nil {
		t.Fatal(err)
	}
	if _, exists := points[0]["p50"]; exists {
		t.Fatalf("must not change global optional scalar emission: %s", w.Body.String())
	}
}
