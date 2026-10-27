package pricebook

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"google.golang.org/grpc"
)

// fakePricebookServiceClient lets tests control the raw gRPC response without a live server.
type fakePricebookServiceClient struct {
	pricebookpb.PricebookServiceClient // embedded nil: any unstubbed method panics if called

	gotReq *pricebookpb.GetObservedPriceRequest
	resp   *pricebookpb.GetObservedPriceResponse
	err    error

	gotRegionReq *pricebookpb.GetRegionalComparisonRequest
	regionResp   *pricebookpb.GetRegionalComparisonResponse
	regionErr    error
}

func (f *fakePricebookServiceClient) GetObservedPrice(_ context.Context, in *pricebookpb.GetObservedPriceRequest, _ ...grpc.CallOption) (*pricebookpb.GetObservedPriceResponse, error) {
	f.gotReq = in
	return f.resp, f.err
}

func (f *fakePricebookServiceClient) GetRegionalComparison(_ context.Context, in *pricebookpb.GetRegionalComparisonRequest, _ ...grpc.CallOption) (*pricebookpb.GetRegionalComparisonResponse, error) {
	f.gotRegionReq = in
	return f.regionResp, f.regionErr
}

func ptr(f float64) *float64 { return &f }

func strPtr(s string) *string { return &s }

// TestGetObservedPrice_FieldMapping 驗證 pb.PriceObservation → usecase.ObservedPrice
// 逐欄位轉換不漏欄位、不把 nil（該市場當天未回報）誤轉成 0。
func TestGetObservedPrice_FieldMapping(t *testing.T) {
	fake := &fakePricebookServiceClient{
		resp: &pricebookpb.GetObservedPriceResponse{
			Observations: []*pricebookpb.PriceObservation{
				{
					MarketCode:  "108",
					CropCode:    "LA1",
					TradeDate:   "2026-07-30",
					AvgPrice:    ptr(25.5),
					UpperPrice:  ptr(30.0),
					MiddlePrice: ptr(25.0),
					LowerPrice:  ptr(20.0),
					VolumeKg:    ptr(1200.0),
				},
				{
					// 該市場當天只回報均價，其餘欄位保持 nil——轉換不得補成 0。
					MarketCode: "241",
					CropCode:   "LA1",
					TradeDate:  "2026-07-30",
					AvgPrice:   ptr(26.0),
				},
			},
		},
	}
	c := &pricebookClient{client: fake}

	got, err := c.GetObservedPrice(context.Background(), "高麗菜")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotReq.GetCropName() != "高麗菜" {
		t.Errorf("crop_name = %q, want 高麗菜", fake.gotReq.GetCropName())
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	first := got[0]
	if first.MarketCode != "108" || first.CropCode != "LA1" || first.TradeDate != "2026-07-30" {
		t.Errorf("first observation string fields wrong: %+v", first)
	}
	for name, got := range map[string]*float64{
		"AvgPrice": first.AvgPrice, "UpperPrice": first.UpperPrice,
		"MiddlePrice": first.MiddlePrice, "LowerPrice": first.LowerPrice, "VolumeKg": first.VolumeKg,
	} {
		if got == nil {
			t.Errorf("first.%s = nil, want non-nil", name)
		}
	}
	if *first.AvgPrice != 25.5 || *first.VolumeKg != 1200.0 {
		t.Errorf("first observation price values wrong: avg=%v volume=%v", first.AvgPrice, first.VolumeKg)
	}

	second := got[1]
	if second.AvgPrice == nil || *second.AvgPrice != 26.0 {
		t.Errorf("second.AvgPrice = %v, want 26.0", second.AvgPrice)
	}
	if second.UpperPrice != nil || second.MiddlePrice != nil || second.LowerPrice != nil || second.VolumeKg != nil {
		t.Errorf("second observation unset fields should stay nil, got: %+v", second)
	}
}

// TestGetObservedPrice_Empty 驗證空 observations 回傳空 slice、無 error
// （pricebook-service 用這個形狀同時表達「作物名無法解析」與「已解析但無成交資料」）。
func TestGetObservedPrice_Empty(t *testing.T) {
	fake := &fakePricebookServiceClient{resp: &pricebookpb.GetObservedPriceResponse{}}
	c := &pricebookClient{client: fake}

	got, err := c.GetObservedPrice(context.Background(), "不存在的作物")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

// TestGetObservedPrice_RPCError 驗證 gRPC 層錯誤原樣往上拋，不被吞掉。
func TestGetObservedPrice_RPCError(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &fakePricebookServiceClient{err: wantErr}
	c := &pricebookClient{client: fake}

	_, err := c.GetObservedPrice(context.Background(), "高麗菜")
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

// TestGetRegionalComparison_MapsFreshnessStatusAndQualityReasonCodes 驗證
// pb.RegionalComparisonRegion → usecase.RegionalComparisonRegion 轉換沒有
// 漏掉 FreshnessStatus/QualityReasonCodes 這兩個欄位——這兩個欄位在
// pricebook-service 端早就存在，本次改動之前 adapter 完全沒讀它們。
func TestGetRegionalComparison_MapsFreshnessStatusAndQualityReasonCodes(t *testing.T) {
	p50 := 42.5
	fake := &fakePricebookServiceClient{
		regionResp: &pricebookpb.GetRegionalComparisonResponse{
			Regions: []*pricebookpb.RegionalComparisonRegion{
				{
					RegionCode: "68000", RegionName: "桃園市", Status: "ready", P50: &p50,
					FreshnessStatus:    "stale",
					QualityReasonCodes: []string{"no_markets_in_region"},
				},
			},
		},
	}
	c := &pricebookClient{client: fake}

	got, err := c.GetRegionalComparison(context.Background(), usecase.RegionalComparisonInput{GroupCode: strPtr("N04")}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Regions) != 1 {
		t.Fatalf("len(got.Regions) = %d, want 1", len(got.Regions))
	}
	if got.Regions[0].FreshnessStatus != "stale" {
		t.Errorf("FreshnessStatus = %q, want stale", got.Regions[0].FreshnessStatus)
	}
	if len(got.Regions[0].QualityReasonCodes) != 1 || got.Regions[0].QualityReasonCodes[0] != "no_markets_in_region" {
		t.Errorf("QualityReasonCodes = %v, want [no_markets_in_region]", got.Regions[0].QualityReasonCodes)
	}
}

func TestGetRegionalComparison_MapsP10P90SupportingMarketCountAggregationMethodAndExcludedMarketCodes(t *testing.T) {
	p10 := 30.25
	p50 := 35.5
	p90 := 41.75
	aggMethod := "market_volume_weighted_quantile_average/v1"
	fake := &fakePricebookServiceClient{
		regionResp: &pricebookpb.GetRegionalComparisonResponse{
			Regions: []*pricebookpb.RegionalComparisonRegion{
				{
					RegionCode: "68000", RegionName: "桃園市", Status: "ready",
					P10: &p10, P50: &p50, P90: &p90,
					SupportingMarketCount: 4,
					ExcludedMarketCodes:   []string{"M001", "M002"},
					AggregationMethod:     &aggMethod,
					FreshnessStatus:       "fresh",
				},
			},
		},
	}
	c := &pricebookClient{client: fake}

	got, err := c.GetRegionalComparison(context.Background(), usecase.RegionalComparisonInput{GroupCode: strPtr("N04")}, 1)
	if err != nil {
		t.Fatalf("GetRegionalComparison() error = %v, want nil", err)
	}
	if len(got.Regions) != 1 {
		t.Fatalf("len(got.Regions) = %d, want 1", len(got.Regions))
	}

	for name, gotVal := range map[string]*float64{"P10": got.Regions[0].P10, "P90": got.Regions[0].P90} {
		wantVal := map[string]*float64{"P10": &p10, "P90": &p90}[name]
		if gotVal == nil || *gotVal != *wantVal {
			t.Errorf("%s = %v, want %v", name, gotVal, *wantVal)
		}
	}
	if got.Regions[0].SupportingMarketCount != 4 {
		t.Errorf("SupportingMarketCount = %v, want 4", got.Regions[0].SupportingMarketCount)
	}
	if got.Regions[0].AggregationMethod == nil || *got.Regions[0].AggregationMethod != aggMethod {
		t.Errorf("AggregationMethod = %v, want %v", got.Regions[0].AggregationMethod, aggMethod)
	}
	if len(got.Regions[0].ExcludedMarketCodes) != 2 || got.Regions[0].ExcludedMarketCodes[0] != "M001" || got.Regions[0].ExcludedMarketCodes[1] != "M002" {
		t.Errorf("ExcludedMarketCodes = %v, want [M001 M002]", got.Regions[0].ExcludedMarketCodes)
	}
}

// TestGetRegionalComparison_MapsLineageSourceDataAsOf 釘住本次新增的 lineage
// 搬運：source_data_as_of 掛在 response 級的 lineage 上、不在每個 region 上，
// 修前 adapter 的回傳型別是裸 []RegionalComparisonRegion，整個 lineage 區塊
// 在這裡就被丟掉，呼叫端拿不到「這份行情資料涵蓋到哪一天」。
func TestGetRegionalComparison_MapsLineageSourceDataAsOf(t *testing.T) {
	p50 := 42.5
	asOf := "2026-08-14T15:59:59Z"
	fake := &fakePricebookServiceClient{
		regionResp: &pricebookpb.GetRegionalComparisonResponse{
			Regions: []*pricebookpb.RegionalComparisonRegion{
				{RegionCode: "68000", RegionName: "桃園市", Status: "ready", P50: &p50, FreshnessStatus: "fresh"},
			},
			Lineage: &pricebookpb.SpatialLineage{SourceDataAsOf: &asOf},
		},
	}
	c := &pricebookClient{client: fake}

	got, err := c.GetRegionalComparison(context.Background(), usecase.RegionalComparisonInput{GroupCode: strPtr("N04")}, 1)
	if err != nil {
		t.Fatalf("GetRegionalComparison() error = %v, want nil", err)
	}
	if got.SourceDataAsOf == nil || *got.SourceDataAsOf != asOf {
		t.Errorf("SourceDataAsOf = %v, want %q", got.SourceDataAsOf, asOf)
	}
}

// TestGetRegionalComparison_NilLineage_LeavesSourceDataAsOfNil 證明沒有 lineage
// 時不會 panic，也不會被補成空字串——proto 端 source_data_as_of 是 optional
// （pricebook-service 在沒有底層資料時本來就會省略），nil 必須原樣傳下去，
// 否則下游會把「不知道資料截至哪天」誤顯示成一個看起來正常的空值。
func TestGetRegionalComparison_NilLineage_LeavesSourceDataAsOfNil(t *testing.T) {
	p50 := 42.5
	fake := &fakePricebookServiceClient{
		regionResp: &pricebookpb.GetRegionalComparisonResponse{
			Regions: []*pricebookpb.RegionalComparisonRegion{
				{RegionCode: "68000", RegionName: "桃園市", Status: "ready", P50: &p50, FreshnessStatus: "fresh"},
			},
		},
	}
	c := &pricebookClient{client: fake}

	got, err := c.GetRegionalComparison(context.Background(), usecase.RegionalComparisonInput{GroupCode: strPtr("N04")}, 1)
	if err != nil {
		t.Fatalf("GetRegionalComparison() error = %v, want nil", err)
	}
	if got.SourceDataAsOf != nil {
		t.Errorf("SourceDataAsOf = %v, want nil", *got.SourceDataAsOf)
	}
}
