package observability

import (
	"testing"
	"time"
)

// TestTimeoutHistogramBoundaries_PinnedPerRealTimeout pins the exact
// boundaries timeoutHistogramBoundaries produces for the four real
// worst-case durations newMetrics actually uses. Extract and SurfaceRealize
// retry once (retryingExtractor, retryingSurfaceRealizer both time the whole
// call including the retry), so their input is
// retryWorstCaseTimeout(perAttempt, attempts) — attempts × per-attempt
// timeout — not the per-attempt timeout alone; Narration and CropSuggest
// never retry, so their input is just their timeout mirror. If the
// derivation (or an attempts/timeout mirror) changes, this test fails and
// shows the new shape as a diff — the whole point being that reshaping every
// dashboard is something a reviewer sees, not something that happens
// silently because a hand-written bucket list got out of sync with a
// timeout or a retry count. See the doc comment above the "…ForBuckets"
// constants, and TestExtractTimeout_MatchesHistogramBucketMirror (and its
// four siblings) in the packages that own the real timeouts/attempt counts,
// for how each mirror is kept honest.
func TestTimeoutHistogramBoundaries_PinnedPerRealTimeout(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
		want    []float64
	}{
		{"extractDuration (mirrors usecase.ExtractTimeout × maxExtractAttempts — retries once)",
			retryWorstCaseTimeout(ExtractPerAttemptTimeoutForBuckets, ExtractAttemptsForBuckets),
			[]float64{0.05, 0.1122, 0.2517, 0.5646, 1.2667, 2.8419, 6.376, 14.3045, 32.0925, 72}},
		{"surfaceRealizeDuration (mirrors usecase.RealizeTimeout × maxRealizeAttempts — retries once)",
			retryWorstCaseTimeout(RealizePerAttemptTimeoutForBuckets, RealizeAttemptsForBuckets),
			[]float64{0.05, 0.0897, 0.1608, 0.2884, 0.5173, 0.9278, 1.6641, 2.9845, 5.3527, 9.6}},
		{"narrationDuration (mirrors usecase.narrationTimeout — never retries; unchanged by this task)", NarrationTimeoutForBuckets,
			[]float64{0.05, 0.0804, 0.1293, 0.208, 0.3345, 0.538, 0.8653, 1.3918, 2.2384, 3.6}},
		{"cropSuggestDuration (mirrors cropidentity.cropCategoryModelTimeout — never retries; unchanged by this task)", CropSuggestTimeoutForBuckets,
			[]float64{0.05, 0.0851, 0.1449, 0.2466, 0.4198, 0.7146, 1.2164, 2.0707, 3.5248, 6}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timeoutHistogramBoundaries(tc.timeout)
			if len(got) != len(tc.want) {
				t.Fatalf("boundary count = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("boundaries = %v, want %v", got, tc.want)
				}
			}

			// The largest finite boundary must clear the timeout itself, or a
			// call that actually times out collapses into +Inf — exactly the
			// procurement_extract_duration_seconds bug this replaces.
			largest := got[len(got)-1]
			if largest < tc.timeout.Seconds() {
				t.Fatalf("largest finite boundary %v < timeout %v", largest, tc.timeout)
			}

			// Boundaries must be strictly increasing: OTel/Prometheus bucket
			// boundaries are meaningless (and may be rejected) otherwise.
			for i := 1; i < len(got); i++ {
				if got[i] <= got[i-1] {
					t.Fatalf("boundaries not strictly increasing at index %d: %v", i, got)
				}
			}
		})
	}
}

// TestTimeoutHistogramBoundaries_CardinalitySane guards the general shape
// (not tied to any specific timeout): bucket count must stay in the same
// ballpark as the hand-written lists it replaced (7-11 boundaries), never
// grow into dozens regardless of how large a future timeout is.
func TestTimeoutHistogramBoundaries_CardinalitySane(t *testing.T) {
	for _, timeout := range []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second, 5 * time.Minute} {
		got := timeoutHistogramBoundaries(timeout)
		if len(got) < 7 || len(got) > 11 {
			t.Fatalf("timeout %v: boundary count = %d, want 7-11", timeout, len(got))
		}
	}
}
