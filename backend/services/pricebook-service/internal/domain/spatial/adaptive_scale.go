package spatial

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Adaptive P50 scale policy versions (issue #3102). Unlike PriceScale's
// fixed CSV breaks (never recomputed per request — issue #2248), these are
// derived from the request's own SpatialStatistics every time; determinism
// instead comes from the request's inputs themselves being deterministic
// (same selection/target date/horizon/model snapshot always yields the same
// statistics, see TestSpatialSurface_Get_IsDeterministicAcrossRuns).
const (
	AdaptivePolicyV1             = "selection-p50-v1"
	AdaptivePolicyV1FlatFallback = "selection-p50-v1-flat-fallback"
)

// adaptiveScaleFractions are 5 fixed quantile-style fractions of the
// [Minimum, Maximum] domain, piecewise-linearly interpolated through the
// three known anchors (0, Minimum), (0.5, Median), (1, Maximum) — 0.5
// always lands exactly on Median. Candidates collapse toward each other
// when the distribution sits close to one side of the median, which is how
// the surviving break count varies between 2 and 5 without a second,
// separately-tuned "how many bands" rule.
var adaptiveScaleFractions = []float64{0.20, 0.35, 0.50, 0.65, 0.80}

// AdaptiveScaleDomain is the same three summary statistics
// SpatialStatistics.{minimum_p50,p50,maximum_p50} expose, so the response's
// own displayed range always bounds the response's own breaks (issue #3102
// acceptance criteria). Minimum <= Median <= Maximum is a structural
// guarantee from weightedMedianP50 (the usecase's only producer of this
// domain), not something re-validated here.
type AdaptiveScaleDomain struct {
	Minimum float64
	Median  float64
	Maximum float64
}

// AdaptiveScaleIdentity is every input the deterministic scale ID binds, so
// the same selection/date/horizon/model/domain snapshot always reproduces
// the same ID (issue #3102). BasePolicyID is the reviewed fixed scale's
// ScaleID from reference/price_scale.v1.csv — #2248's approval gate stays
// the eligibility/provenance check (no approved row, no surface at all),
// it just no longer supplies the breaks themselves.
type AdaptiveScaleIdentity struct {
	SelectionKind string
	SelectionCode string
	TargetDate    string
	HorizonDays   int
	ModelVersion  string
	BasePolicyID  string
}

// DeriveAdaptiveScale computes issue #3102's selection-specific adaptive P50
// scale. Each of adaptiveScaleFractions is rounded to 4 decimal places
// (matching the cell/county price quantization already used elsewhere) and
// kept only if strictly greater than the previously kept value and strictly
// less than domain.Maximum — duplicates collapse naturally when the
// distribution sits close to one side of the median. Fewer than 2 surviving
// breaks means the domain cannot support a meaningful multi-band scale, so
// the result is AdaptivePolicyV1FlatFallback with an empty Breaks slice
// instead of inventing a fake difference.
func DeriveAdaptiveScale(domain AdaptiveScaleDomain, identity AdaptiveScaleIdentity) PriceScale {
	breaks := make([]float64, 0, len(adaptiveScaleFractions))
	prev := domain.Minimum
	for _, f := range adaptiveScaleFractions {
		var v float64
		if f <= 0.5 {
			v = domain.Minimum + (f/0.5)*(domain.Median-domain.Minimum)
		} else {
			v = domain.Median + ((f-0.5)/0.5)*(domain.Maximum-domain.Median)
		}
		v = RoundTo(v, 4)
		if v > prev && v < domain.Maximum {
			breaks = append(breaks, v)
			prev = v
		}
	}

	policy := AdaptivePolicyV1
	if len(breaks) < 2 {
		policy = AdaptivePolicyV1FlatFallback
		breaks = nil
	}

	return PriceScale{
		ScaleID:       adaptiveScaleID(identity, domain, policy, breaks),
		PolicyVersion: policy,
		Breaks:        breaks,
	}
}

// adaptiveScaleID hashes every binding input into a short, stable suffix on
// the reviewed base policy's own ScaleID, so the adaptive ID still traces
// back to which #2248 approval row made this selection eligible at all.
func adaptiveScaleID(identity AdaptiveScaleIdentity, domain AdaptiveScaleDomain, policy string, breaks []float64) string {
	breakStrs := make([]string, 0, len(breaks))
	for _, b := range breaks {
		breakStrs = append(breakStrs, strconv.FormatFloat(b, 'f', 4, 64))
	}
	raw := fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s|%.4f|%.4f|%.4f|%s",
		identity.SelectionKind, identity.SelectionCode, identity.TargetDate, identity.HorizonDays,
		identity.ModelVersion, identity.BasePolicyID, policy,
		domain.Minimum, domain.Median, domain.Maximum, strings.Join(breakStrs, ","))
	sum := sha256.Sum256([]byte(raw))
	return identity.BasePolicyID + "-adaptive-" + hex.EncodeToString(sum[:])[:8]
}
