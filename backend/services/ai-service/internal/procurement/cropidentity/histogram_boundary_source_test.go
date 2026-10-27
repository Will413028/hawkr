package cropidentity

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
)

// TestCropCategoryModelTimeout_MatchesHistogramBucketMirror is the drift
// guard for issue #3519's histogram fix, for this package's timeout.
//
// internal/observability/metrics.go derives
// procurement_crop_suggest_duration_seconds's bucket boundaries from
// cropCategoryModelTimeout — the real deadline wrapped around the LLM call
// RecordProcurementCropSuggestDuration measures — but observability cannot
// import this package to read it directly: this package already imports
// observability to report metrics, so the reverse import would be a cycle.
// metrics.go instead keeps a hand-written mirror
// (CropSuggestTimeoutForBuckets). This test is what keeps that mirror
// honest: change cropCategoryModelTimeout without updating its mirror, and
// this fails.
//
// If this fails: update observability.CropSuggestTimeoutForBuckets in
// metrics.go to match cropCategoryModelTimeout — do not change
// cropCategoryModelTimeout to match the mirror. The bug this guards against
// is procurement_extract_duration_seconds's top bucket sitting at 15 while
// its own timeout was 30s, silently collapsing 15-30s into +Inf; the same
// silent drift is possible here if the mirror is left stale.
func TestCropCategoryModelTimeout_MatchesHistogramBucketMirror(t *testing.T) {
	if cropCategoryModelTimeout != observability.CropSuggestTimeoutForBuckets {
		t.Fatalf("cropCategoryModelTimeout = %v but observability.CropSuggestTimeoutForBuckets = %v: "+
			"procurement_crop_suggest_duration_seconds's histogram buckets are derived from the "+
			"mirror in metrics.go, not from cropCategoryModelTimeout directly (observability "+
			"cannot import this package — see the comment above this test). Update "+
			"observability.CropSuggestTimeoutForBuckets to match cropCategoryModelTimeout; do "+
			"not change cropCategoryModelTimeout to match the mirror.",
			cropCategoryModelTimeout, observability.CropSuggestTimeoutForBuckets)
	}
}
