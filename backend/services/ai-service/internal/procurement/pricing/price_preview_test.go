package pricing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	pbusecase "github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestPreviewRfqPrice_PricebookGuided_ComputesReferenceRangeWithoutMarkup(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-1",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferencePriceRange == nil {
		t.Fatalf("ReferencePriceRange = nil, want non-nil")
	}
	if got.ReferencePriceRange.Lower == nil || got.ReferencePriceRange.Upper == nil {
		t.Fatalf("crop 路徑必須有三個分位：%+v", *got.ReferencePriceRange)
	}
	if *got.ReferencePriceRange.Lower != 30 || got.ReferencePriceRange.Midpoint != 36 ||
		*got.ReferencePriceRange.Upper != 42 {
		t.Errorf("ReferencePriceRange = %+v, want {30 36 42}（不得套用任何 markup，KG 單位換算係數為 1；金額經 math.Round 收斂成整數元）", got.ReferencePriceRange)
	}
}

func TestPreviewRfqPrice_PricebookGuided_FirstPreviewPrefillsFromReferenceBounds(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-2",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: nil,
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.BuyerRequestedRange.LowerBoundAmount != "30" || got.BuyerRequestedRange.UpperBoundAmount != "42" {
		t.Errorf("BuyerRequestedRange = %+v, want prefilled from rounded reference bounds {30, 42}", got.BuyerRequestedRange)
	}
	if got.BuyerRequestedRange.BasisQuantity != "1" || got.BuyerRequestedRange.BasisUnit != "KG" {
		t.Errorf("BasisQuantity/BasisUnit = %q/%q, want 1/KG", got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
	}
}

func TestPreviewRfqPrice_BuyerEnteredNoReference_RequiresAcknowledgementAndSetsTemperatureUnknown(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "unavailable", FreshnessStatus: "unavailable"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-3",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferenceAvailability != "UNAVAILABLE" || got.ReferencePriceRange != nil {
		t.Errorf("ReferenceAvailability/ReferencePriceRange = %v/%v, want UNAVAILABLE/nil", got.ReferenceAvailability, got.ReferencePriceRange)
	}
	if got.DealTemperature.Status != "UNKNOWN" {
		t.Errorf("DealTemperature.Status = %v, want UNKNOWN", got.DealTemperature.Status)
	}
}

func TestPreviewRfqPrice_GuidedMode_BuyerRangeSet_ReturnsCoverage(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(52.0), P50: testkit.Ptr(61.0), P90: testkit.Ptr(74.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-dt-1",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "58", UpperBoundAmount: "70",
		},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}

	dt := got.DealTemperature
	if dt.Status != "AVAILABLE" {
		t.Fatalf("Status = %q，預期 AVAILABLE（ReasonCode=%q）", dt.Status, dt.ReasonCode)
	}
	if dt.ProbabilityLowerBps != "4100" || dt.ProbabilityUpperBps != "7800" {
		t.Errorf("機率 = %s/%s，預期 4100/7800", dt.ProbabilityLowerBps, dt.ProbabilityUpperBps)
	}
	if dt.Band != "HOT" {
		t.Errorf("Band = %q，預期 HOT", dt.Band)
	}
	if dt.OutcomeDefinition != "BUYER_RANGE_VS_PRICEBOOK_FORECAST_COVERAGE" ||
		dt.OutcomeDefinitionVersion != "1" ||
		dt.PolicyVersion != "DEAL_TEMPERATURE_COVERAGE_V1" ||
		dt.BandVersion != "BANDS_V2" {
		t.Errorf("版本欄位不齊：%+v", dt)
	}
}

func TestPreviewRfqPrice_GuidedMode_BuyerRangeUnset_ReturnsBuyerRangeNotSet(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(52.0), P50: testkit.Ptr(61.0), P90: testkit.Ptr(74.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-dt-2",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: nil,
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.DealTemperature.Status != "UNKNOWN" ||
		got.DealTemperature.ReasonCode != "BUYER_RANGE_NOT_SET" {
		t.Fatalf("DealTemperature = %+v，預期 UNKNOWN/BUYER_RANGE_NOT_SET", got.DealTemperature)
	}
	if got.DealTemperature.ProbabilityLowerBps != "" || got.DealTemperature.Band != "" {
		t.Errorf("UNKNOWN 不得帶機率或 band：%+v", got.DealTemperature)
	}
}

func TestDealTemperatureFor_NilReference_ReturnsMappingMissing(t *testing.T) {
	got := dealTemperatureFor(nil, &BuyerRequestedRange{
		Currency: "TWD", PriceComponent: "GOODS_ONLY",
		LowerBoundAmount: "58", UpperBoundAmount: "70",
	})
	if got.Status != "UNKNOWN" || got.ReasonCode != "MAPPING_MISSING" {
		t.Fatalf("dealTemperatureFor(nil, ...) = %+v，預期 UNKNOWN/MAPPING_MISSING", got)
	}
	if got.ProbabilityLowerBps != "" || got.ProbabilityUpperBps != "" || got.Band != "" {
		t.Errorf("UNKNOWN 不得帶機率或 band：%+v", got)
	}
}

func TestPreviewRfqPrice_UnitCannotCanonicalize_ReturnsPriceBasisUnavailable(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-4",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if !errors.Is(err, ErrPriceBasisUnavailable) {
		t.Errorf("err = %v, want ErrPriceBasisUnavailable", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_BuyerEnteredNoReference_NilBuyerRequestedRange_RejectsBeforeAnyWrite(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "unavailable", FreshnessStatus: "unavailable"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-nil-range",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE", DeliveryCity: "桃園市",
		BuyerRequestedRange: nil,
	})
	if !errors.Is(err, ErrPriceBasisUnavailable) {
		t.Errorf("err = %v, want ErrPriceBasisUnavailable", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect，nil buyerRequestedRange 不得被持久化）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_BuyerEnteredNoReference_InvalidBuyerRequestedRange_RejectsBeforeAnyWrite(t *testing.T) {
	cases := []struct {
		name  string
		input BuyerRequestedRange
	}{
		{"非數字下界", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "not-a-number", UpperBoundAmount: "850"}},
		{"負數下界", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "-100", UpperBoundAmount: "850"}},
		{"零下界", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "0", UpperBoundAmount: "850"}},
		{"上界缺失", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: ""}},
		{"非整數（禁止浮點）", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650.5", UpperBoundAmount: "850"}},
		{"下界大於上界", BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "900", UpperBoundAmount: "850"}},
		{"幣別非 TWD", BuyerRequestedRange{Currency: "USD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
				{RegionName: "桃園市", Status: "unavailable", FreshnessStatus: "unavailable"},
			}}
			repo := &testkit.FakePricePreviewRepo{}
			uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
				Slots:      domain.Slots{Unit: testkit.Ptr("KG")},
				Resolution: testkit.GroupResolution("N04-001"),
				RevisionID: 1,
			})

			bound := tc.input
			_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
				RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-invalid-range-" + tc.name,
				ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE", DeliveryCity: "桃園市",
				BuyerRequestedRange: &bound,
			})
			if !errors.Is(err, ErrPriceBasisUnavailable) {
				t.Errorf("err = %v, want ErrPriceBasisUnavailable", err)
			}
			if repo.InsertCalls != 0 {
				t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.InsertCalls)
			}
		})
	}
}

func TestPreviewRfqPrice_StaleRevision_ReturnsError(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{Slots: domain.Slots{Unit: testkit.Ptr("KG")}, RevisionID: 2})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-5",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if !errors.Is(err, ErrStalePricePreviewRevision) {
		t.Errorf("err = %v, want ErrStalePricePreviewRevision", err)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_SameClientRequestIDReturnsExistingRow(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	in := PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-6",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	}
	first, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}
	second, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}
	if first.PricePreviewID != second.PricePreviewID {
		t.Errorf("PricePreviewID = %d then %d, want same", first.PricePreviewID, second.PricePreviewID)
	}
	if repo.InsertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_DifferentInputSameClientRequestIDReturnsConflict(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
		{RegionName: "台中市", Status: "ready", P10: testkit.Ptr(20.0), P50: testkit.Ptr(25.0), P90: testkit.Ptr(30.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	first := PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-conflict",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	}
	if _, err := uc.PreviewRfqPrice(context.Background(), first); err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}

	second := first
	second.DeliveryCity = "台中市"
	_, err := uc.PreviewRfqPrice(context.Background(), second)
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("err = %v, want domain.ErrIdempotencyKeyConflict", err)
	}
	if repo.InsertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1（conflict 不得二次寫入）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_IdenticalInputReplaysCleanly(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	in := PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-identical-replay",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	}
	first, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}
	second, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("second call (identical input) error = %v, want nil（不得誤判為 conflict）", err)
	}
	if first.PricePreviewID != second.PricePreviewID || first.PayloadHash != second.PayloadHash {
		t.Errorf("first=%+v second=%+v, want identical replay", first, second)
	}
	if repo.InsertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_NoResolvedCropOrGroupCode_SkipsPricebookAndReturnsBuyerEnteredNoReference(t *testing.T) {
	counting := &testkit.CountingPricebookClient{FakePricebookClient: &testkit.FakePricebookClient{
		Regions: []pbusecase.RegionalComparisonRegion{
			{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
		},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(counting, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG")},
		Resolution: nil,
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-7",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferenceAvailability != "UNAVAILABLE" || got.ReferencePriceRange != nil {
		t.Errorf("ReferenceAvailability/ReferencePriceRange = %v/%v, want UNAVAILABLE/nil", got.ReferenceAvailability, got.ReferencePriceRange)
	}
	if got.DealTemperature.Status != "UNKNOWN" || got.DealTemperature.ReasonCode != "MAPPING_MISSING" {
		t.Errorf("DealTemperature = %+v, want {UNKNOWN MAPPING_MISSING}", got.DealTemperature)
	}
	if counting.RegionCalls != 0 {
		t.Errorf("regionCalls = %d, want 0（品項未解析時不得呼叫 Pricebook）", counting.RegionCalls)
	}
}

func TestPreviewRfqPrice_RegionMatch_NormalizesTaiCityDualWriting(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "台北市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(40.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("臺北市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-tai-dual-writing",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil（Pricebook 回「台北市」，買家 slots 存「臺北市」，正規化後應相符）", err)
	}
	if got.ReferenceAvailability != "AVAILABLE" || got.ReferencePriceRange == nil {
		t.Fatalf("ReferenceAvailability/ReferencePriceRange = %v/%v, want AVAILABLE/non-nil", got.ReferenceAvailability, got.ReferencePriceRange)
	}
	if got.ReferencePriceRange.Midpoint != 35 {
		t.Errorf("Midpoint = %v, want 35", got.ReferencePriceRange.Midpoint)
	}
}

func TestPreviewRfqPrice_CattyUnit_RegressionFloatPrecisionArtifact_FormatsCleanIntegerAmounts(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(90.1), P50: testkit.Ptr(100.1), P90: testkit.Ptr(110.1), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("CATTY"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-catty-regress",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferencePriceRange.Midpoint != 60 {
		t.Errorf("Midpoint = %v, want 60（100.1*0.6 的浮點殘留必須被 math.Round 收斂）", got.ReferencePriceRange.Midpoint)
	}
	if got.BuyerRequestedRange.LowerBoundAmount != "54" || got.BuyerRequestedRange.UpperBoundAmount != "66" {
		t.Errorf("BuyerRequestedRange = %+v, want clean integer strings {54, 66}，不是 float-precision artifact（例如 \"60.059999999999995\"）", got.BuyerRequestedRange)
	}
	if got.BuyerRequestedRange.BasisQuantity != "1" || got.BuyerRequestedRange.BasisUnit != "CATTY" {
		t.Errorf("BasisQuantity/BasisUnit = %q/%q, want 1/CATTY", got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
	}
}

func TestPreviewRfqPrice_GramUnit_PopulatesThousandGramBasis(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(40.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("GRAM"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-gram-basis",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.BuyerRequestedRange.BasisQuantity != "1000" || got.BuyerRequestedRange.BasisUnit != "GRAM" {
		t.Errorf("BasisQuantity/BasisUnit = %q/%q, want 1000/GRAM", got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
	}
}

func TestPreviewRfqPrice_RegionMatch_UsesSlotsDeliveryCityNotRequestField(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(40.0), FreshnessStatus: "fresh"},
		{RegionName: "台中市", Status: "ready", P10: testkit.Ptr(20.0), P50: testkit.Ptr(25.0), P90: testkit.Ptr(30.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("台中市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-slots-vs-request-city",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferencePriceRange.Midpoint != 25 {
		t.Errorf("Midpoint = %v, want 25（台中市的 P50，來自 slots.DeliveryCity，不是 client 送的桃園市 35）", got.ReferencePriceRange.Midpoint)
	}
}

func TestPreviewRfqPricePricebookGuidedWithUnreliableMapping_RejectsEvenWithBuyerOverride(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "unavailable", FreshnessStatus: "unavailable"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-guided-unreliable-with-override",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"},
	})
	if !errors.Is(err, ErrMappingUnavailableForGuidedMode) {
		t.Errorf("err = %v, want ErrMappingUnavailableForGuidedMode", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（不得靜默成功並持久化誤導性的 pricing_mode='PRICEBOOK_GUIDED' 紀錄）", repo.InsertCalls)
	}
}

func TestPreviewRfqPricePricebookGuidedWithUnreliableMapping_NoOverride_RejectsWithMappingMissingNotPriceBasisUnavailable(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "unavailable", FreshnessStatus: "unavailable"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-guided-unreliable-no-override",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if !errors.Is(err, ErrMappingUnavailableForGuidedMode) {
		t.Errorf("err = %v, want ErrMappingUnavailableForGuidedMode", err)
	}
	if errors.Is(err, ErrPriceBasisUnavailable) {
		t.Errorf("err 不得是 ErrPriceBasisUnavailable——那個碼保留給 currency/quantity/unit/conversion basis 無法 canonicalize，不是 mapping 不可用")
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_BuyerEnteredNoReferenceWithReliableMapping_StaysUnavailable(t *testing.T) {
	counting := &testkit.CountingPricebookClient{FakePricebookClient: &testkit.FakePricebookClient{
		Regions: []pbusecase.RegionalComparisonRegion{
			{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.25), P50: testkit.Ptr(35.5), P90: testkit.Ptr(41.75), FreshnessStatus: "fresh"},
		},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(counting, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-no-reference-despite-reliable-mapping",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferenceAvailability != "UNAVAILABLE" || got.ReferencePriceRange != nil {
		t.Errorf("ReferenceAvailability/ReferencePriceRange = %v/%v, want UNAVAILABLE/nil（即使 mapping 剛好可靠，買家已明確選擇不參考，不可被悄悄升級成 AVAILABLE）", got.ReferenceAvailability, got.ReferencePriceRange)
	}
	if counting.RegionCalls != 0 {
		t.Errorf("regionCalls = %d, want 0（買家已明確選擇 BUYER_ENTERED_NO_REFERENCE，不該呼叫 Pricebook）", counting.RegionCalls)
	}
}

func TestPreviewRfqPrice_Available_ExposesMarketPriceBasis(t *testing.T) {
	asOf := "2026-08-14T15:59:59Z"
	fpc := &testkit.FakePricebookClient{
		Regions: []pbusecase.RegionalComparisonRegion{
			{
				RegionName: "桃園市", Status: "ready",
				P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(41.0),
				SupportingMarketCount: 3, FreshnessStatus: "fresh",
			},
		},
		SourceDataAsOf: &asOf,
	}
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-basis",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.MarketPriceBasis == nil {
		t.Fatal("MarketPriceBasis = nil, want non-nil on the AVAILABLE path")
	}
	if got.MarketPriceBasis.RegionName != "桃園市" {
		t.Errorf("RegionName = %q, want 桃園市", got.MarketPriceBasis.RegionName)
	}
	if got.MarketPriceBasis.SupportingMarketCount != 3 {
		t.Errorf("SupportingMarketCount = %d, want 3", got.MarketPriceBasis.SupportingMarketCount)
	}
	if got.MarketPriceBasis.SourceDataAsOf == nil || *got.MarketPriceBasis.SourceDataAsOf != asOf {
		t.Errorf("SourceDataAsOf = %v, want %q", got.MarketPriceBasis.SourceDataAsOf, asOf)
	}
	if got.MarketPriceBasis.FreshnessStatus != "fresh" {
		t.Errorf("FreshnessStatus = %q, want fresh", got.MarketPriceBasis.FreshnessStatus)
	}
}

func TestPreviewRfqPrice_MarketPriceBasis_ReportsMatchedRegionNotBuyerCity(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "台北市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(41.0), FreshnessStatus: "fresh"},
	}}
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("臺北市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-region",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.MarketPriceBasis == nil || got.MarketPriceBasis.RegionName != "台北市" {
		t.Errorf("RegionName = %v, want 台北市（Pricebook 實際命中的那一列）", got.MarketPriceBasis)
	}
}

func TestPreviewRfqPrice_StaleMarketData_StillReturnsReferenceAndDisclosesStaleness(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(41.0), FreshnessStatus: "stale"},
	}}
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-stale",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.ReferenceAvailability != "AVAILABLE" {
		t.Errorf("ReferenceAvailability = %q, want AVAILABLE（stale 不得擋掉參考價）", got.ReferenceAvailability)
	}
	if got.ReferencePriceRange == nil {
		t.Fatal("ReferencePriceRange = nil, want non-nil")
	}
	if got.MarketPriceBasis == nil || got.MarketPriceBasis.FreshnessStatus != "stale" {
		t.Errorf("FreshnessStatus = %v, want stale", got.MarketPriceBasis)
	}
}

func TestPreviewRfqPrice_BuyerEnteredNoReference_HasNoMarketPriceBasis(t *testing.T) {
	uc := NewPreviewRfqPriceUseCase(&testkit.FakePricebookClient{}, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(50)},
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-no-ref",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850",
		},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.MarketPriceBasis != nil {
		t.Errorf("MarketPriceBasis = %+v, want nil", got.MarketPriceBasis)
	}
}

func TestPreviewRfqPrice_ReferencePriceRangeCarriesItsOwnBasis(t *testing.T) {
	for _, tc := range []struct {
		unit          string
		wantQuantity  string
		wantBasisUnit string
	}{
		{unit: "KG", wantQuantity: "1", wantBasisUnit: "KG"},
		{unit: "CATTY", wantQuantity: "1", wantBasisUnit: "CATTY"},
		{unit: "GRAM", wantQuantity: "1000", wantBasisUnit: "GRAM"},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
				{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(41.0), FreshnessStatus: "fresh"},
			}}
			uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
				Slots:      domain.Slots{Unit: testkit.Ptr(tc.unit), DeliveryCity: testkit.Ptr("桃園市")},
				Resolution: testkit.GroupResolution("N04-001"),
				RevisionID: 1,
			})

			got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
				RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-" + tc.unit,
				ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
			})
			if err != nil {
				t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
			}
			if got.ReferencePriceRange == nil {
				t.Fatal("ReferencePriceRange = nil, want non-nil")
			}
			if got.ReferencePriceRange.BasisQuantity != tc.wantQuantity || got.ReferencePriceRange.BasisUnit != tc.wantBasisUnit {
				t.Errorf("reference basis = %q/%q, want %q/%q",
					got.ReferencePriceRange.BasisQuantity, got.ReferencePriceRange.BasisUnit,
					tc.wantQuantity, tc.wantBasisUnit)
			}
			if got.BuyerRequestedRange.BasisQuantity != tc.wantQuantity || got.BuyerRequestedRange.BasisUnit != tc.wantBasisUnit {
				t.Errorf("buyer range basis = %q/%q, want same as reference basis",
					got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
			}
		})
	}
}

func TestPreviewRfqPrice_BuyerSuppliedRange_StillGetsServerFilledBasis(t *testing.T) {
	for _, mode := range []string{"PRICEBOOK_GUIDED", "BUYER_ENTERED_NO_REFERENCE"} {
		t.Run(mode, func(t *testing.T) {
			fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
				{RegionName: "桃園市", Status: "ready", P10: testkit.Ptr(30.0), P50: testkit.Ptr(35.0), P90: testkit.Ptr(41.0), FreshnessStatus: "fresh"},
			}}
			uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
				Slots:      domain.Slots{Unit: testkit.Ptr("GRAM"), DeliveryCity: testkit.Ptr("桃園市")},
				Resolution: testkit.GroupResolution("N04-001"),
				RevisionID: 1,
			})

			got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
				RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-supplied-" + mode,
				ExpectedRevisionID: 1, PricingMode: mode,
				BuyerRequestedRange: &BuyerRequestedRange{
					Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850",
				},
			})
			if err != nil {
				t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
			}
			if got.BuyerRequestedRange.BasisQuantity != "1000" || got.BuyerRequestedRange.BasisUnit != "GRAM" {
				t.Errorf("basis = %q/%q, want 1000/GRAM（server 回填，不因買家自帶金額而留空）",
					got.BuyerRequestedRange.BasisQuantity, got.BuyerRequestedRange.BasisUnit)
			}
			if got.BuyerRequestedRange.LowerBoundAmount != "650" || got.BuyerRequestedRange.UpperBoundAmount != "850" {
				t.Errorf("bounds = %q/%q, want 650/850（原樣回顯）",
					got.BuyerRequestedRange.LowerBoundAmount, got.BuyerRequestedRange.UpperBoundAmount)
			}
		})
	}
}

func TestPreviewRfqPrice_QuoteTotalRange_DividesByBasisQuantity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		unit       string
		quantity   int
		wantLower  string
		wantUpper  string
		wantQtyStr string
	}{
		{name: "KG basis 1", unit: "KG", quantity: 50, wantLower: "32500", wantUpper: "42500", wantQtyStr: "50"},
		{name: "GRAM basis 1000", unit: "GRAM", quantity: 50000, wantLower: "32500", wantUpper: "42500", wantQtyStr: "50000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewPreviewRfqPriceUseCase(&testkit.FakePricebookClient{}, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
				Slots:      domain.Slots{Unit: testkit.Ptr(tc.unit), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(tc.quantity)},
				RevisionID: 1,
			})

			got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
				RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-total-" + tc.name,
				ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE",
				BuyerRequestedRange: &BuyerRequestedRange{
					Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850",
				},
			})
			if err != nil {
				t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
			}
			if got.QuoteTotalRange == nil {
				t.Fatal("QuoteTotalRange = nil, want non-nil")
			}
			if got.QuoteTotalRange.LowerBoundAmount != tc.wantLower || got.QuoteTotalRange.UpperBoundAmount != tc.wantUpper {
				t.Errorf("total = %q–%q, want %q–%q",
					got.QuoteTotalRange.LowerBoundAmount, got.QuoteTotalRange.UpperBoundAmount,
					tc.wantLower, tc.wantUpper)
			}
			if got.QuoteTotalRange.Quantity != tc.wantQtyStr || got.QuoteTotalRange.QuantityUnit != tc.unit {
				t.Errorf("quantity = %q %q, want %q %q",
					got.QuoteTotalRange.Quantity, got.QuoteTotalRange.QuantityUnit, tc.wantQtyStr, tc.unit)
			}
			if got.QuoteTotalRange.Currency != "TWD" {
				t.Errorf("Currency = %q, want TWD", got.QuoteTotalRange.Currency)
			}
		})
	}
}

func TestPreviewRfqPrice_QuoteTotalRange_NilWhenQuantityMissing(t *testing.T) {
	uc := NewPreviewRfqPriceUseCase(&testkit.FakePricebookClient{}, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-no-qty",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850",
		},
	})
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.QuoteTotalRange != nil {
		t.Errorf("QuoteTotalRange = %+v, want nil", got.QuoteTotalRange)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_PreservesV6Fields(t *testing.T) {
	asOf := "2026-08-14T15:59:59Z"
	fpc := &testkit.FakePricebookClient{
		Regions: []pbusecase.RegionalComparisonRegion{
			{
				RegionName: "桃園市", Status: "ready",
				P10: testkit.Ptr(600.0), P50: testkit.Ptr(700.0), P90: testkit.Ptr(800.0),
				SupportingMarketCount: 3, FreshnessStatus: "fresh",
			},
		},
		SourceDataAsOf: &asOf,
	}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(50)},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	in := PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-replay",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED",
	}
	first, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("first PreviewRfqPrice() error = %v, want nil", err)
	}
	if first.MarketPriceBasis == nil || first.QuoteTotalRange == nil {
		t.Fatalf("first call must produce both blocks, got basis=%+v total=%+v",
			first.MarketPriceBasis, first.QuoteTotalRange)
	}

	replay, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("replay PreviewRfqPrice() error = %v, want nil", err)
	}
	if repo.InsertCalls != 1 {
		t.Fatalf("insertCalls = %d, want 1（第二次必須是冪等回放，不重寫）", repo.InsertCalls)
	}
	if replay.MarketPriceBasis == nil {
		t.Fatal("replay MarketPriceBasis = nil, want the persisted block")
	}
	if replay.MarketPriceBasis.RegionName != first.MarketPriceBasis.RegionName ||
		replay.MarketPriceBasis.SupportingMarketCount != first.MarketPriceBasis.SupportingMarketCount ||
		replay.MarketPriceBasis.FreshnessStatus != first.MarketPriceBasis.FreshnessStatus {
		t.Errorf("replay MarketPriceBasis = %+v, want %+v", *replay.MarketPriceBasis, *first.MarketPriceBasis)
	}
	if replay.MarketPriceBasis.SourceDataAsOf == nil ||
		*replay.MarketPriceBasis.SourceDataAsOf != *first.MarketPriceBasis.SourceDataAsOf {
		t.Errorf("replay SourceDataAsOf = %v, want %q",
			replay.MarketPriceBasis.SourceDataAsOf, *first.MarketPriceBasis.SourceDataAsOf)
	}
	if replay.QuoteTotalRange == nil {
		t.Fatal("replay QuoteTotalRange = nil, want the persisted block")
	}
	if *replay.QuoteTotalRange != *first.QuoteTotalRange {
		t.Errorf("replay QuoteTotalRange = %+v, want %+v", *replay.QuoteTotalRange, *first.QuoteTotalRange)
	}
	if replay.ReferencePriceRange == nil ||
		replay.ReferencePriceRange.BasisQuantity != first.ReferencePriceRange.BasisQuantity ||
		replay.ReferencePriceRange.BasisUnit != first.ReferencePriceRange.BasisUnit {
		t.Errorf("replay reference basis = %+v, want %+v", replay.ReferencePriceRange, first.ReferencePriceRange)
	}
}

func TestQuoteTotalRange_ReturnsNilOnUncomputableInput(t *testing.T) {
	base := BuyerRequestedRange{
		Currency: "TWD", BasisQuantity: "1", BasisUnit: "KG",
		LowerBoundAmount: "650", UpperBoundAmount: "850",
	}
	for _, tc := range []struct {
		name     string
		mutate   func(r *BuyerRequestedRange)
		quantity *int
	}{
		{name: "quantity nil", quantity: nil},
		{name: "quantity zero", quantity: testkit.Ptr(0)},
		{name: "quantity negative", quantity: testkit.Ptr(-1)},
		{name: "basis quantity empty（unit 未填）", mutate: func(r *BuyerRequestedRange) { r.BasisQuantity = "" }, quantity: testkit.Ptr(50)},
		{name: "basis quantity zero", mutate: func(r *BuyerRequestedRange) { r.BasisQuantity = "0" }, quantity: testkit.Ptr(50)},
		{name: "lower not an integer", mutate: func(r *BuyerRequestedRange) { r.LowerBoundAmount = "650.5" }, quantity: testkit.Ptr(50)},
		{name: "upper not a number", mutate: func(r *BuyerRequestedRange) { r.UpperBoundAmount = "abc" }, quantity: testkit.Ptr(50)},
		{name: "lower zero", mutate: func(r *BuyerRequestedRange) { r.LowerBoundAmount = "0" }, quantity: testkit.Ptr(50)},
		{name: "overflow", mutate: func(r *BuyerRequestedRange) { r.UpperBoundAmount = "9223372036854775807" }, quantity: testkit.Ptr(1_000_000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			if tc.mutate != nil {
				tc.mutate(&r)
			}
			if got := quoteTotalRange(r, tc.quantity); got != nil {
				t.Errorf("quoteTotalRange() = %+v, want nil", got)
			}
		})
	}
}

func TestQuoteTotalRange_RoundsHalfUpWithoutFloat(t *testing.T) {
	got := quoteTotalRange(BuyerRequestedRange{
		Currency: "TWD", BasisQuantity: "1000", BasisUnit: "GRAM",
		LowerBoundAmount: "7", UpperBoundAmount: "9",
	}, testkit.Ptr(1500))
	if got == nil {
		t.Fatal("quoteTotalRange() = nil, want non-nil")
	}
	if got.LowerBoundAmount != "11" || got.UpperBoundAmount != "14" {
		t.Errorf("total = %q–%q, want 11–14", got.LowerBoundAmount, got.UpperBoundAmount)
	}
}

func TestPreviewRfqPrice_GuidedMode_MissingP10_RejectsInsteadOfPanicking(t *testing.T) {
	fpc := &testkit.FakePricebookClient{Regions: []pbusecase.RegionalComparisonRegion{
		{RegionName: "桃園市", Status: "ready", P10: nil, P50: testkit.Ptr(61.0), P90: testkit.Ptr(74.0), FreshnessStatus: "fresh"},
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	_, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-p10-nil",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
	if !errors.Is(err, ErrMappingUnavailableForGuidedMode) {
		t.Fatalf("err = %v, want ErrMappingUnavailableForGuidedMode", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_CarriesQualityKnowledgeState(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(&testkit.FakePricebookClient{}, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(50)},
		Resolution: testkit.GroupResolution("N04-001"),
		RevisionID: 1,
	})

	in := PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-quality",
		ExpectedRevisionID: 1, PricingMode: "BUYER_ENTERED_NO_REFERENCE",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "58", UpperBoundAmount: "70",
			BasisQuantity: "1", BasisUnit: "KG",
		},
	}

	first, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if first.QualityKnowledgeState != "BUYER_QUALITY_UNKNOWN" {
		t.Errorf("QualityKnowledgeState = %q, want BUYER_QUALITY_UNKNOWN", first.QualityKnowledgeState)
	}

	replay, err := uc.PreviewRfqPrice(context.Background(), in)
	if err != nil {
		t.Fatalf("replay PreviewRfqPrice() error = %v, want nil", err)
	}
	if replay.QualityKnowledgeState != first.QualityKnowledgeState {
		t.Errorf("replay QualityKnowledgeState = %q, want %q",
			replay.QualityKnowledgeState, first.QualityKnowledgeState)
	}
}

func TestPreviewRfqPrice_CommodityResolution_NoRangeAndCarriesLayer(t *testing.T) {
	p50 := 106.03
	client := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal", Unit: "kg",
		SourceDataAsOf: "2026-09-12", SupportingMarketCount: "19", QuantityUnit: "head",
	}}
	got := previewForCommodity(t, client)

	require.NotNil(t, got.ReferencePriceRange)
	require.Equal(t, float64(106), got.ReferencePriceRange.Midpoint)
	require.Nil(t, got.ReferencePriceRange.Lower, "來源只公布一個均價，不得造出下界")
	require.Nil(t, got.ReferencePriceRange.Upper, "來源只公布一個均價，不得造出上界")

	require.NotNil(t, got.MarketPriceBasis)
	require.Equal(t, MarketPriceScopeNational, got.MarketPriceBasis.Scope)
	require.Empty(t, got.MarketPriceBasis.RegionName, "全國行情沒有縣市")
	require.Equal(t, "live_animal", got.MarketPriceBasis.PriceLayer,
		"買家必須看得到這是活體拍賣價，否則會照著它出價")
	require.Equal(t, int32(19), got.MarketPriceBasis.SupportingMarketCount)
}

func TestPreviewRfqPrice_CommodityResolution_DealTemperatureIsPointEstimate(t *testing.T) {
	p50 := 106.03
	client := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal",
	}}
	got := previewForCommodityWithBuyerRange(t, client, "90", "120")

	require.Equal(t, "AVAILABLE", got.DealTemperature.Status)
	require.Equal(t, "10000", got.DealTemperature.ProbabilityLowerBps,
		"參考價 106 落在 90~120 內，退化分布下為 100%")
	for _, v := range []string{
		got.DealTemperature.ProbabilityLowerBps,
		got.DealTemperature.ProbabilityUpperBps,
	} {
		require.Contains(t, []string{"0", "10000"}, v, "單點估計不該有中間值")
	}
}

func TestPreviewRfqPrice_CommodityResolution_NoPriceRejectsGuidedMode(t *testing.T) {
	client := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "insufficient_data",
	}}
	_, err := previewForCommodityErr(t, client)
	require.ErrorIs(t, err, ErrMappingUnavailableForGuidedMode)
}

func previewForCommodityErr(t *testing.T, fpc *testkit.FakePricebookClient) (PricePreviewResult, error) {
	t.Helper()
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.CommodityResolution("700000000000000001"),
		RevisionID: 1,
	})
	return uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-commodity",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})
}

func previewForCommodity(t *testing.T, fpc *testkit.FakePricebookClient) PricePreviewResult {
	t.Helper()
	got, err := previewForCommodityErr(t, fpc)
	require.NoError(t, err)
	return got
}

func previewForCommodityWithBuyerRange(t *testing.T, fpc *testkit.FakePricebookClient, lo, hi string) PricePreviewResult {
	t.Helper()
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.CommodityResolution("700000000000000001"),
		RevisionID: 1,
	})
	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-commodity-range",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: lo, UpperBoundAmount: hi,
		},
	})
	require.NoError(t, err)
	return got
}

func TestPreviewRfqPrice_CommodityResolution_PersistsRow(t *testing.T) {
	p50 := 103.08
	fpc := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal", Unit: "kg",
		SourceDataAsOf: "2026-09-15", SupportingMarketCount: "19",
	}}
	repo := &testkit.FakePricePreviewRepo{}
	uc := NewPreviewRfqPriceUseCase(fpc, repo, testkit.FakeRevisionReader{
		Slots:      domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")},
		Resolution: testkit.CommodityResolution("700000000000000001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-commodity-persist",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.InsertCalls,
		"commodity 路徑沒持久化的話，買家拿到參考價卻確認不了，RFQ 發不出去")
	require.NotZero(t, got.PricePreviewID, "rfq-confirmations 要拿這個 ID 做 CAS")
	require.NotEmpty(t, got.PayloadHash, "rfq-confirmations 的 CAS 還要比對這個 hash")
	require.Equal(t, "BUYER_QUALITY_UNKNOWN", got.QualityKnowledgeState,
		"品質認知狀態與品項解析型別無關，兩條路徑一致")
}

func TestPreviewRfqPrice_CommodityResolution_QuoteTotalRangeFromBuyerRange(t *testing.T) {
	p50 := 103.08
	fpc := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal", Unit: "kg",
	}}
	quantity := 50
	uc := NewPreviewRfqPriceUseCase(fpc, &testkit.FakePricePreviewRepo{}, testkit.FakeRevisionReader{
		Slots: domain.Slots{
			Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市"), Quantity: &quantity,
		},
		Resolution: testkit.CommodityResolution("700000000000000001"),
		RevisionID: 1,
	})

	got, err := uc.PreviewRfqPrice(context.Background(), PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "preview-commodity-total",
		ExpectedRevisionID: 1, PricingMode: "PRICEBOOK_GUIDED", DeliveryCity: "桃園市",
		BuyerRequestedRange: &BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "90", UpperBoundAmount: "120",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, got.QuoteTotalRange, "50 KG × 90~120 元算得出總額，不該是 null")
	require.Equal(t, "4500", got.QuoteTotalRange.LowerBoundAmount)
	require.Equal(t, "6000", got.QuoteTotalRange.UpperBoundAmount)
}

func TestPreviewRfqPrice_CommodityResolution_DoesNotClaimAFreshnessItNeverComputed(t *testing.T) {
	p50 := 106.03
	client := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal", Unit: "kg",
		SourceDataAsOf: "2026-09-12", SupportingMarketCount: "19",
	}}

	got := previewForCommodity(t, client)

	require.NotNil(t, got.MarketPriceBasis)
	require.Empty(t, got.MarketPriceBasis.FreshnessStatus,
		"單點來源沒有新鮮度判斷。原本寫的是 cp.Status（PredictionStatus，值域 ready／partial／"+
			"market_closed／insufficient_data／not_predicted／stale／unavailable），而這個欄位的契約"+
			"詞彙是 FreshnessLabel（fresh／stale／unavailable）。兩套詞彙混用讓前端 safeParse 失敗，"+
			"買家卡在「目前無法完成價格確認」（2026-09-19 實測）")
	require.NotEmpty(t, got.MarketPriceBasis.SourceDataAsOf,
		"原始資料日期照送——買家要判斷資料多舊，材料仍在，少的只是我們沒算過的那個標籤")
}

func TestPreviewRfqPrice_CommodityResolution_UnsetBuyerRangeStaysUnset(t *testing.T) {
	p50 := 106.03
	client := &testkit.FakePricebookClient{CommodityPrice: pbusecase.CommodityPrice{
		Status: "ready", P50: &p50, PriceLayer: "live_animal",
	}}

	got := previewForCommodity(t, client)

	require.True(t, got.BuyerRequestedRange.IsUnset(),
		"買家略過預算時不得留下一個全空字串的區間物件；dealTemperature.reasonCode 已經寫了 BUYER_RANGE_NOT_SET")
}
