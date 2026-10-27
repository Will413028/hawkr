package spatial

import "testing"

func TestDeriveAdaptiveScale_NormalDomain_ProducesFiveBreaks(t *testing.T) {
	domain := AdaptiveScaleDomain{
		Minimum: 25.838686427923463,
		Median:  30.994506115067747,
		Maximum: 35.905374525801236,
	}
	identity := AdaptiveScaleIdentity{
		SelectionKind: "crop", SelectionCode: "A1",
		TargetDate: "2026-08-09", HorizonDays: 1,
		ModelVersion: "model-v1", BasePolicyID: "N05-price-scale-v1",
	}

	got := DeriveAdaptiveScale(domain, identity)

	if got.PolicyVersion != AdaptivePolicyV1 {
		t.Fatalf("PolicyVersion = %q, want %q", got.PolicyVersion, AdaptivePolicyV1)
	}
	want := []float64{27.901, 29.4478, 30.9945, 32.4678, 33.941}
	if len(got.Breaks) != len(want) {
		t.Fatalf("Breaks = %v, want %v", got.Breaks, want)
	}
	for i, w := range want {
		if got.Breaks[i] != w {
			t.Fatalf("Breaks[%d] = %v, want %v", i, got.Breaks[i], w)
		}
	}
	for i := 1; i < len(got.Breaks); i++ {
		if got.Breaks[i] <= got.Breaks[i-1] {
			t.Fatalf("Breaks not strictly increasing: %v", got.Breaks)
		}
	}
	if got.Breaks[0] <= domain.Minimum || got.Breaks[len(got.Breaks)-1] >= domain.Maximum {
		t.Fatalf("Breaks %v not strictly inside (%v, %v)", got.Breaks, domain.Minimum, domain.Maximum)
	}
	if got.ScaleID == "" || got.ScaleID == identity.BasePolicyID {
		t.Fatalf("ScaleID = %q, want a derived id distinct from BasePolicyID", got.ScaleID)
	}
}

func TestDeriveAdaptiveScale_FlatDomain_ReturnsFlatFallback(t *testing.T) {
	domain := AdaptiveScaleDomain{Minimum: 30, Median: 30, Maximum: 30}
	identity := AdaptiveScaleIdentity{
		SelectionKind: "crop", SelectionCode: "A1",
		TargetDate: "2026-08-09", HorizonDays: 1,
		ModelVersion: "model-v1", BasePolicyID: "N05-price-scale-v1",
	}

	got := DeriveAdaptiveScale(domain, identity)

	if got.PolicyVersion != AdaptivePolicyV1FlatFallback {
		t.Fatalf("PolicyVersion = %q, want %q", got.PolicyVersion, AdaptivePolicyV1FlatFallback)
	}
	if len(got.Breaks) != 0 {
		t.Fatalf("Breaks = %v, want empty", got.Breaks)
	}
}

// TestDeriveAdaptiveScale_ExactlyTwoSurvivors_DoesNotFlatFallback pins the
// minimum surviving-break count that must NOT trigger flat-fallback.
// Median == Minimum collapses every f<=0.5 candidate to exactly Minimum
// (rejected by the strict v > prev check), leaving only the f=0.65 and
// f=0.80 candidates, which land at 3 and 6 — verified by running the
// algorithm, not hand arithmetic.
func TestDeriveAdaptiveScale_ExactlyTwoSurvivors_DoesNotFlatFallback(t *testing.T) {
	domain := AdaptiveScaleDomain{Minimum: 0, Median: 0, Maximum: 10}
	identity := AdaptiveScaleIdentity{
		SelectionKind: "crop", SelectionCode: "A1",
		TargetDate: "2026-08-09", HorizonDays: 1,
		ModelVersion: "model-v1", BasePolicyID: "N05-price-scale-v1",
	}

	got := DeriveAdaptiveScale(domain, identity)

	if got.PolicyVersion != AdaptivePolicyV1 {
		t.Fatalf("PolicyVersion = %q, want %q", got.PolicyVersion, AdaptivePolicyV1)
	}
	want := []float64{3, 6}
	if len(got.Breaks) != len(want) {
		t.Fatalf("Breaks = %v, want %v", got.Breaks, want)
	}
	for i, w := range want {
		if got.Breaks[i] != w {
			t.Fatalf("Breaks[%d] = %v, want %v", i, got.Breaks[i], w)
		}
	}
}

// TestDeriveAdaptiveScale_QuantizationCollapse_ResetsToFlatFallback exercises
// the `breaks = nil` reset line on a domain that is NOT fully flat
// (Minimum != Maximum) but whose candidates still quantization-collapse
// after 4-decimal rounding to a single raw survivor (30.0001, from f=0.65
// and f=0.80 both rounding to the same value) — one survivor short of the
// 2 required, so the reset must fire. Verified by running the algorithm:
// today only the fully-flat 0-survivor case is covered.
func TestDeriveAdaptiveScale_QuantizationCollapse_ResetsToFlatFallback(t *testing.T) {
	domain := AdaptiveScaleDomain{Minimum: 30, Median: 30, Maximum: 30.0002}
	identity := AdaptiveScaleIdentity{
		SelectionKind: "crop", SelectionCode: "A1",
		TargetDate: "2026-08-09", HorizonDays: 1,
		ModelVersion: "model-v1", BasePolicyID: "N05-price-scale-v1",
	}

	got := DeriveAdaptiveScale(domain, identity)

	if got.PolicyVersion != AdaptivePolicyV1FlatFallback {
		t.Fatalf("PolicyVersion = %q, want %q", got.PolicyVersion, AdaptivePolicyV1FlatFallback)
	}
	if len(got.Breaks) != 0 {
		t.Fatalf("Breaks = %v, want empty (reset to nil, not a 1-element slice)", got.Breaks)
	}
}

func TestDeriveAdaptiveScale_IsDeterministic(t *testing.T) {
	domain := AdaptiveScaleDomain{Minimum: 10, Median: 20, Maximum: 40}
	identity := AdaptiveScaleIdentity{
		SelectionKind: "crop", SelectionCode: "A1",
		TargetDate: "2026-08-09", HorizonDays: 1,
		ModelVersion: "model-v1", BasePolicyID: "N05-price-scale-v1",
	}

	first := DeriveAdaptiveScale(domain, identity)
	second := DeriveAdaptiveScale(domain, identity)
	if first.ScaleID != second.ScaleID {
		t.Fatalf("ScaleID not deterministic: %q vs %q", first.ScaleID, second.ScaleID)
	}

	identity.TargetDate = "2026-08-10"
	third := DeriveAdaptiveScale(domain, identity)
	if third.ScaleID == first.ScaleID {
		t.Fatalf("ScaleID must differ when TargetDate differs, got same %q", first.ScaleID)
	}
}
