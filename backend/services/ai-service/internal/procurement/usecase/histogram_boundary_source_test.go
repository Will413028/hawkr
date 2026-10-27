package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
)

// These five tests are the drift guard for issue #3519's histogram fix.
//
// internal/observability/metrics.go derives each procurement histogram's
// bucket boundaries from that operation's real timeout — and, for the two
// operations that retry, from the real retry attempt count too, since the
// worst *observed* duration is attempts × per-attempt timeout, not the
// per-attempt timeout alone (retryingExtractor and retryingSurfaceRealizer
// both time the whole call including any retry, with no backoff between
// attempts). observability cannot read ExtractTimeout / maxExtractAttempts /
// RealizeTimeout / maxRealizeAttempts / narrationTimeout directly: this
// package (usecase) already imports observability to report metrics, so the
// reverse import would be a cycle. metrics.go instead keeps a hand-written
// mirror of each value ("…ForBuckets" constants). These tests are what keeps
// those mirrors honest: change a timeout or an attempt count here without
// updating its mirror, and one of these fails.
//
// If a failure sends you here: the fix is almost always to update the
// ForBuckets constant in metrics.go to match the value you just changed —
// not to change the value here back. The mirrors exist so the histogram's
// bucket boundaries keep covering the real worst case (the bug this closed
// was procurement_extract_duration_seconds's top bucket sitting at 15 while
// a retried extract call could take up to 60s, so anything past 15s silently
// collapsed into +Inf).

func TestExtractTimeout_MatchesHistogramBucketMirror(t *testing.T) {
	if ExtractTimeout != observability.ExtractPerAttemptTimeoutForBuckets {
		t.Fatalf("usecase.ExtractTimeout = %v but observability.ExtractPerAttemptTimeoutForBuckets = %v: "+
			"procurement_extract_duration_seconds's histogram buckets are derived from the "+
			"mirror in metrics.go, not from ExtractTimeout directly (observability cannot "+
			"import this package — see the comment above this test). Update "+
			"observability.ExtractPerAttemptTimeoutForBuckets to match ExtractTimeout; do not "+
			"lower ExtractTimeout to match the mirror.",
			ExtractTimeout, observability.ExtractPerAttemptTimeoutForBuckets)
	}
}

func TestExtractAttempts_MatchesHistogramBucketMirror(t *testing.T) {
	if maxExtractAttempts != observability.ExtractAttemptsForBuckets {
		t.Fatalf("usecase.maxExtractAttempts = %v but observability.ExtractAttemptsForBuckets = %v: "+
			"procurement_extract_duration_seconds's histogram top boundary is derived from "+
			"attempts × per-attempt timeout so a retried call lands in a real bucket, not +Inf "+
			"(observability cannot import this package — see the comment above this test). "+
			"Update observability.ExtractAttemptsForBuckets to match maxExtractAttempts; do not "+
			"change maxExtractAttempts to match the mirror.",
			maxExtractAttempts, observability.ExtractAttemptsForBuckets)
	}
}

func TestRealizeTimeout_MatchesHistogramBucketMirror(t *testing.T) {
	if RealizeTimeout != observability.RealizePerAttemptTimeoutForBuckets {
		t.Fatalf("usecase.RealizeTimeout = %v but observability.RealizePerAttemptTimeoutForBuckets = %v: "+
			"procurement_surface_realize_duration_seconds's histogram buckets are derived "+
			"from the mirror in metrics.go, not from RealizeTimeout directly (observability "+
			"cannot import this package — see the comment above this test). Update "+
			"observability.RealizePerAttemptTimeoutForBuckets to match RealizeTimeout; do not "+
			"change RealizeTimeout to match the mirror.",
			RealizeTimeout, observability.RealizePerAttemptTimeoutForBuckets)
	}
}

func TestRealizeAttempts_MatchesHistogramBucketMirror(t *testing.T) {
	if maxRealizeAttempts != observability.RealizeAttemptsForBuckets {
		t.Fatalf("usecase.maxRealizeAttempts = %v but observability.RealizeAttemptsForBuckets = %v: "+
			"procurement_surface_realize_duration_seconds's histogram top boundary is derived "+
			"from attempts × per-attempt timeout so a retried call lands in a real bucket, not "+
			"+Inf (observability cannot import this package — see the comment above this "+
			"test). Update observability.RealizeAttemptsForBuckets to match maxRealizeAttempts; "+
			"do not change maxRealizeAttempts to match the mirror.",
			maxRealizeAttempts, observability.RealizeAttemptsForBuckets)
	}
}

func TestNarrationTimeout_MatchesHistogramBucketMirror(t *testing.T) {
	if narrationTimeout != observability.NarrationTimeoutForBuckets {
		t.Fatalf("usecase.narrationTimeout = %v but observability.NarrationTimeoutForBuckets = %v: "+
			"procurement_narration_duration_seconds's histogram buckets are derived from the "+
			"mirror in metrics.go, not from narrationTimeout directly (observability cannot "+
			"import this package — see the comment above this test). Update "+
			"observability.NarrationTimeoutForBuckets to match narrationTimeout; do not change "+
			"narrationTimeout to match the mirror.",
			narrationTimeout, observability.NarrationTimeoutForBuckets)
	}
}
