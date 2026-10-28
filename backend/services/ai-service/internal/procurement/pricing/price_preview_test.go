package pricing

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func newPreviewUC(repo *testkit.FakePricePreviewRepo, slots domain.Slots, revisionID int64) *PreviewRfqPriceUseCase {
	return NewPreviewRfqPriceUseCase(repo, testkit.FakeRevisionReader{Slots: slots, RevisionID: revisionID})
}

func validRange() *BuyerRequestedRange {
	return &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "650", UpperBoundAmount: "850"}
}

func previewInput(clientRequestID string) PreviewRfqPriceInput {
	return PreviewRfqPriceInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: clientRequestID,
		ExpectedRevisionID: 1, DeliveryCity: "桃園市", BuyerRequestedRange: validRange(),
	}
}

func TestPreviewRfqPrice_BuyerEnteredRange_IsEchoedWithServerFilledBasis(t *testing.T) {
	for _, tc := range []struct {
		unit, wantQty string
	}{
		{"KG", "1"},
		{"CATTY", "1"},
		{"GRAM", "1000"},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			uc := newPreviewUC(&testkit.FakePricePreviewRepo{}, domain.Slots{Unit: testkit.Ptr(tc.unit)}, 1)

			got, err := uc.PreviewRfqPrice(context.Background(), previewInput("preview-basis-"+tc.unit))
			if err != nil {
				t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
			}
			r := got.BuyerRequestedRange
			if r.LowerBoundAmount != "650" || r.UpperBoundAmount != "850" || r.Currency != "TWD" {
				t.Errorf("BuyerRequestedRange = %+v，買家給的範圍必須原樣回顯", r)
			}
			if r.BasisQuantity != tc.wantQty || r.BasisUnit != tc.unit {
				t.Errorf("BasisQuantity/BasisUnit = %q/%q, want %q/%q", r.BasisQuantity, r.BasisUnit, tc.wantQty, tc.unit)
			}
		})
	}
}

func TestPreviewRfqPrice_NilBuyerRequestedRange_RejectsBeforeAnyWrite(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	uc := newPreviewUC(repo, domain.Slots{Unit: testkit.Ptr("KG")}, 1)

	in := previewInput("preview-nil-range")
	in.BuyerRequestedRange = nil
	_, err := uc.PreviewRfqPrice(context.Background(), in)
	if !errors.Is(err, ErrBuyerRequestedRangeRequired) {
		t.Errorf("err = %v, want ErrBuyerRequestedRangeRequired", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect，nil buyerRequestedRange 不得被持久化）", repo.InsertCalls)
	}
}

func TestPreviewRfqPrice_InvalidBuyerRequestedRange_RejectsBeforeAnyWrite(t *testing.T) {
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
			repo := &testkit.FakePricePreviewRepo{}
			uc := newPreviewUC(repo, domain.Slots{Unit: testkit.Ptr("KG")}, 1)

			bound := tc.input
			in := previewInput("preview-invalid-range-" + tc.name)
			in.BuyerRequestedRange = &bound
			_, err := uc.PreviewRfqPrice(context.Background(), in)
			if !errors.Is(err, ErrBuyerRequestedRangeRequired) {
				t.Errorf("err = %v, want ErrBuyerRequestedRangeRequired", err)
			}
			if repo.InsertCalls != 0 {
				t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.InsertCalls)
			}
		})
	}
}

func TestPreviewRfqPrice_StaleRevision_ReturnsError(t *testing.T) {
	uc := newPreviewUC(&testkit.FakePricePreviewRepo{}, domain.Slots{Unit: testkit.Ptr("KG")}, 2)

	_, err := uc.PreviewRfqPrice(context.Background(), previewInput("preview-stale"))
	if !errors.Is(err, ErrStalePricePreviewRevision) {
		t.Errorf("err = %v, want ErrStalePricePreviewRevision", err)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_IdenticalInputReplaysCleanly(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	uc := newPreviewUC(repo, domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(50)}, 1)

	in := previewInput("preview-identical-replay")
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
		t.Errorf("insertCalls = %d, want 1（第二次必須是冪等回放，不重寫）", repo.InsertCalls)
	}
	if first.QuoteTotalRange == nil || second.QuoteTotalRange == nil || *first.QuoteTotalRange != *second.QuoteTotalRange {
		t.Errorf("QuoteTotalRange first=%+v second=%+v, want identical", first.QuoteTotalRange, second.QuoteTotalRange)
	}
	if first.BuyerRequestedRange != second.BuyerRequestedRange {
		t.Errorf("BuyerRequestedRange first=%+v second=%+v, want identical", first.BuyerRequestedRange, second.BuyerRequestedRange)
	}
}

func TestPreviewRfqPrice_IdempotentReplay_DifferentInputSameClientRequestIDReturnsConflict(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	uc := newPreviewUC(repo, domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")}, 1)

	first := previewInput("preview-conflict")
	if _, err := uc.PreviewRfqPrice(context.Background(), first); err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}

	second := first
	second.BuyerRequestedRange = &BuyerRequestedRange{Currency: "TWD", LowerBoundAmount: "700", UpperBoundAmount: "900"}
	_, err := uc.PreviewRfqPrice(context.Background(), second)
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("err = %v, want domain.ErrIdempotencyKeyConflict", err)
	}
	if repo.InsertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1（conflict 不得二次寫入）", repo.InsertCalls)
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
			uc := newPreviewUC(&testkit.FakePricePreviewRepo{},
				domain.Slots{Unit: testkit.Ptr(tc.unit), DeliveryCity: testkit.Ptr("桃園市"), Quantity: testkit.Ptr(tc.quantity)}, 1)

			got, err := uc.PreviewRfqPrice(context.Background(), previewInput("preview-total-"+tc.name))
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
	uc := newPreviewUC(&testkit.FakePricePreviewRepo{}, domain.Slots{Unit: testkit.Ptr("KG"), DeliveryCity: testkit.Ptr("桃園市")}, 1)

	got, err := uc.PreviewRfqPrice(context.Background(), previewInput("preview-no-qty"))
	if err != nil {
		t.Fatalf("PreviewRfqPrice() error = %v, want nil", err)
	}
	if got.QuoteTotalRange != nil {
		t.Errorf("QuoteTotalRange = %+v, want nil", got.QuoteTotalRange)
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

func TestPreviewRfqPrice_QuantityTimesAmountOverflow_RejectsAsUnconvertible(t *testing.T) {
	repo := &testkit.FakePricePreviewRepo{}
	uc := newPreviewUC(repo, domain.Slots{Unit: testkit.Ptr("KG"), Quantity: testkit.Ptr(1 << 30)}, 1)

	in := previewInput("preview-overflow")
	in.BuyerRequestedRange = &BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "9000000000000000000", UpperBoundAmount: "9000000000000000001"}
	_, err := uc.PreviewRfqPrice(context.Background(), in)
	if !errors.Is(err, ErrQuantityUnitUnconvertible) {
		t.Errorf("err = %v, want ErrQuantityUnitUnconvertible", err)
	}
	if repo.InsertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0", repo.InsertCalls)
	}
}
