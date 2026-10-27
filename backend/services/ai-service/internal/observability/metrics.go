package observability

import (
	"context"
	"math"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"google.golang.org/genai"
)

const meterName = "github.com/Will413028/hawkr/ai-service/internal/observability"

// The constants below are MIRRORS, not sources of truth: each copies a value
// that lives in another package, so that newHistogram's bucket derivation
// (timeoutHistogramBoundaries, via retryWorstCaseTimeout) can cover the full
// observable range. They are exported and "…ForBuckets"-suffixed
// specifically so they never read like an ordinary constant that's safe to
// edit in place.
//
// observability cannot import the packages that own the real constants
// (procurement/usecase, procurement/cropidentity), because those packages
// already import observability to report metrics — importing them back here
// would be a cycle. So the sync runs the other way: each owning package has
// a test (named below) that imports observability and asserts its real
// constant equals its mirror here. Change a real value without updating its
// mirror, and that test fails — this is what makes "change the timeout,
// forget the buckets" a caught bug instead of a silent dashboard reshape.
//
// That claim covers all four *timeout* mirrors (ExtractPerAttemptTimeout…,
// RealizePerAttemptTimeout…, NarrationTimeout…, CropSuggestTimeout…): each
// really is the value applied at its call site (model_llm_extractor.go:37,
// surface_realizer.go:88, step_narrator.go:75, crop_category_model.go:62).
//
// It covers "the retry count" only for RealizeAttemptsForBuckets.
// retryingSurfaceRealizer.Realize (surface_realizer.go) drives its retry
// loop from maxRealizeAttempts itself, so changing that constant changes
// how many times Realize actually attempts the call — the mirror test
// (TestRealizeAttempts_MatchesHistogramBucketMirror) then genuinely catches
// "changed the retry count, forgot the bucket". ExtractAttemptsForBuckets is
// NOT the same: retryingExtractor.Extract (extractor_provider.go) still
// hardcodes exactly one retry as straight-line code — maxExtractAttempts is
// only read afterwards, to *label* result.MaxAttempts. Changing
// maxExtractAttempts today changes no behaviour at all; only its own mirror
// test would need to move, and only because both sides are hand-set to 2.
// This is a real, narrower gap than the one closed for Realize (2026-09-19
// final review Finding 4) and is not fixed here — flagged so nobody reads
// this comment and assumes both retry counts are equally load-bearing.
//
// Two of the four operations retry once on a provider-unavailable error
// (retryingExtractor.Extract in extractor_provider.go,
// retryingSurfaceRealizer.Realize in surface_realizer.go), and both time the
// whole call including any retry (time.Since(start) spans both attempts,
// with no sleep/backoff between them — confirmed by reading both call
// sites). So the worst *observable* duration for those two is
// attempts × per-attempt timeout, not the per-attempt timeout alone — a
// bucket top set to just the per-attempt timeout still lets the slowest real
// calls (the ones that retried) overflow into +Inf, which is the same defect
// this task removed, just at 2× the value. StepNarrator and the crop
// category suggester never retry (no retrying wrapper exists for either),
// so their mirror is used directly as the worst case.
//
// To change one of these: update the value here AND the real constant it
// mirrors together, in the same commit. Never edit only this side.
const (
	// ExtractPerAttemptTimeoutForBuckets mirrors ExtractTimeout in
	// internal/procurement/usecase/extract.go — the cap on *one* extract
	// attempt, not the worst case across retries. Pinned by
	// usecase.TestExtractTimeout_MatchesHistogramBucketMirror.
	ExtractPerAttemptTimeoutForBuckets = 30 * time.Second
	// ExtractAttemptsForBuckets mirrors the unexported maxExtractAttempts in
	// internal/procurement/usecase/extractor_provider.go: the worst-case
	// number of attempts (1 initial + retries) retryingExtractor.Extract
	// makes before giving up. Pinned by
	// usecase.TestExtractAttempts_MatchesHistogramBucketMirror.
	ExtractAttemptsForBuckets = 2
	// RealizePerAttemptTimeoutForBuckets mirrors RealizeTimeout in
	// internal/procurement/usecase/surface_realizer.go — the cap on *one*
	// realize attempt, not the worst case across retries. Pinned by
	// usecase.TestRealizeTimeout_MatchesHistogramBucketMirror.
	RealizePerAttemptTimeoutForBuckets = 4 * time.Second
	// RealizeAttemptsForBuckets mirrors the unexported maxRealizeAttempts in
	// internal/procurement/usecase/surface_realizer.go: the worst-case
	// number of attempts retryingSurfaceRealizer.Realize makes before giving
	// up. Pinned by usecase.TestRealizeAttempts_MatchesHistogramBucketMirror.
	RealizeAttemptsForBuckets = 2
	// NarrationTimeoutForBuckets mirrors the unexported narrationTimeout in
	// internal/procurement/usecase/step_narrator.go. StepNarrator never
	// retries, so this is already the worst case — no attempts multiplier.
	// Pinned by usecase.TestNarrationTimeout_MatchesHistogramBucketMirror.
	NarrationTimeoutForBuckets = 3 * time.Second
	// CropSuggestTimeoutForBuckets mirrors the unexported
	// cropCategoryModelTimeout in
	// internal/procurement/cropidentity/crop_category_model.go — the timeout
	// actually wrapped around the LLM call that
	// RecordProcurementCropSuggestDuration measures. The crop category
	// suggester never retries, so this is already the worst case — no
	// attempts multiplier. Pinned by
	// cropidentity.TestCropCategoryModelTimeout_MatchesHistogramBucketMirror.
	CropSuggestTimeoutForBuckets = 5 * time.Second
)

// retryWorstCaseTimeout returns the longest duration an operation that
// retries up to attempts times, each individually bounded by perAttempt and
// with no fixed backoff between attempts, can actually take to return. This
// — not perAttempt alone — is what newHistogram's bucket derivation needs
// for a retrying operation, so the top boundary covers what can actually be
// observed instead of only the fast path.
func retryWorstCaseTimeout(perAttempt time.Duration, attempts int) time.Duration {
	return perAttempt * time.Duration(attempts)
}

const (
	// histogramBoundaryCount is how many finite bucket boundaries
	// timeoutHistogramBoundaries produces, regardless of the timeout. Chosen
	// to land in the same 7–11 range the hand-written bucket lists occupied.
	histogramBoundaryCount = 10
	// histogramMaxBoundaryFactor pushes the top finite boundary past the
	// nominal timeout: a call that actually hits its deadline is observed a
	// little later than the timeout value itself (context-cancellation
	// propagation, retry unwind, measurement overhead), so the top boundary
	// must clear that margin or a timing-out call still lands in +Inf.
	histogramMaxBoundaryFactor = 1.2
	// histogramMinBoundaryFloor is the smallest finite boundary any derived
	// histogram uses (seconds), regardless of how small the timeout is:
	// sub-50ms resolution is noise for these LLM-call latencies, and every
	// extra boundary is Prometheus cardinality that isn't free.
	histogramMinBoundaryFloor = 0.05
)

// timeoutHistogramBoundaries derives Prometheus-compatible bucket boundaries
// (seconds) for a histogram whose measured calls are bounded by timeout. It
// is a pure function of timeout: boundaries are geometrically spaced
// (constant ratio between consecutive values) from histogramMinBoundaryFloor
// up to timeout*histogramMaxBoundaryFactor, spreading resolution evenly
// across orders of magnitude the way the hand-written lists did, without
// anyone having to hand-copy them again when a timeout changes.
func timeoutHistogramBoundaries(timeout time.Duration) []float64 {
	max := timeout.Seconds() * histogramMaxBoundaryFactor
	min := histogramMinBoundaryFloor
	if max <= min {
		max = min * 2
	}
	ratio := math.Pow(max/min, 1/float64(histogramBoundaryCount-1))
	boundaries := make([]float64, histogramBoundaryCount)
	v := min
	for i := range boundaries {
		// Round to 4 decimal places: purely cosmetic (keeps the `le` label
		// values readable), never collapses two boundaries into one at this
		// scale since consecutive raw values differ by far more than 0.0001.
		boundaries[i] = math.Round(v*10000) / 10000
		v *= ratio
	}
	return boundaries
}

type Metrics struct {
	aiTransformTotal       metric.Int64Counter
	aiTransformRetryTotal  metric.Int64Counter
	procurementRunTotal    metric.Int64Counter
	candidateZeroTotal     metric.Int64Counter
	extractTotal           metric.Int64Counter
	degradedTotal          metric.Int64Counter
	dialoguePlanTotal      metric.Int64Counter
	extractDuration        metric.Float64Histogram
	surfaceRealizeTotal    metric.Int64Counter
	validationRejectTotal  metric.Int64Counter
	surfaceFallbackTotal   metric.Int64Counter
	surfaceRealizeDuration metric.Float64Histogram
	narrationTotal         metric.Int64Counter
	narrationDuration      metric.Float64Histogram
	cropSuggestTotal       metric.Int64Counter
	cropSuggestDuration    metric.Float64Histogram
	cropResolverV2Total    metric.Int64Counter
	llmTokensTotal         metric.Int64Counter
}

var activeMetrics atomic.Pointer[Metrics]

func init() {
	activeMetrics.Store(newMetrics(noop.NewMeterProvider()))
}

func InstallMeterProvider(provider metric.MeterProvider) func() {
	if provider == nil {
		provider = noop.NewMeterProvider()
	}
	previous := activeMetrics.Load()
	activeMetrics.Store(newMetrics(provider))
	return func() { activeMetrics.Store(previous) }
}

func newMetrics(provider metric.MeterProvider) *Metrics {
	meter := provider.Meter(meterName)
	return &Metrics{
		aiTransformTotal:      newCounter(meter, "ai_transform_total", "Cumulative count of AI image transform attempts by outcome"),
		aiTransformRetryTotal: newCounter(meter, "ai_transform_retry_total", "Cumulative count of AI image transform retries by trigger reason and retry outcome"),
		procurementRunTotal:   newCounter(meter, "procurement_run_total", "Cumulative count of procurement candidate-search runs by terminal status"),
		candidateZeroTotal:    newCounter(meter, "procurement_candidate_zero_total", "Cumulative count of procurement candidate-search lane completions with zero candidates"),
		extractTotal:          newCounter(meter, "procurement_extract_total", "Cumulative count of procurement slot-extraction attempts by result (success/timeout/error)"),
		degradedTotal:         newCounter(meter, "procurement_degraded_total", "Cumulative count of procurement turns that degraded (LLM slot extraction unavailable)"),
		dialoguePlanTotal:     newCounter(meter, "procurement_dialogue_plan_total", "Count of DialoguePlanner act selections, labeled by act code."),
		extractDuration: newHistogram(meter, "procurement_extract_duration_seconds",
			"Procurement slot-extraction call duration in seconds, including any internal retry",
			// Worst case across all attempts, not just the first: retryingExtractor retries once.
			retryWorstCaseTimeout(ExtractPerAttemptTimeoutForBuckets, ExtractAttemptsForBuckets)),
		surfaceRealizeTotal:   newCounter(meter, "procurement_surface_realize_total", "Cumulative count of SurfaceRealizer attempts by source and result"),
		validationRejectTotal: newCounter(meter, "procurement_surface_validation_reject_total", "Cumulative count of SurfaceRealizer output rejections by reason"),
		surfaceFallbackTotal:  newCounter(meter, "procurement_surface_fallback_total", "Cumulative count of SurfaceRealizer fallbacks to deterministic template, by act"),
		surfaceRealizeDuration: newHistogram(meter, "procurement_surface_realize_duration_seconds",
			"SurfaceRealizer call duration in seconds",
			// Worst case across all attempts, not just the first: retryingSurfaceRealizer retries once.
			retryWorstCaseTimeout(RealizePerAttemptTimeoutForBuckets, RealizeAttemptsForBuckets)),
		narrationTotal: newCounter(meter, "procurement_narration_total",
			"Cumulative count of StepNarrator calls by step code and result (success/error/rejected/disabled)"),
		narrationDuration: newHistogram(meter, "procurement_narration_duration_seconds",
			"StepNarrator model call duration in seconds, by step code",
			// Single attempt — no retrying wrapper exists for StepNarrator.
			NarrationTimeoutForBuckets),
		cropSuggestTotal: newCounter(meter, "procurement_crop_suggest_total",
			"Cumulative count of crop category suggestion calls by stage and result (success/error/rejected/bound_exceeded)"),
		cropSuggestDuration: newHistogram(meter, "procurement_crop_suggest_duration_seconds",
			"Crop category suggestion model call duration in seconds, by stage",
			// Single attempt — no retrying wrapper exists in cropidentity.
			CropSuggestTimeoutForBuckets),
		cropResolverV2Total: newCounter(meter, "procurement_crop_resolver_v2_total",
			"Cumulative count of v2 (retrieve-then-rank) crop resolutions by outcome (resolved/ask/legacy/escape/error)"),
		llmTokensTotal: newCounter(meter, "procurement_llm_tokens_total",
			"Cumulative LLM token usage reported by the provider, by call role and token kind (prompt/completion/thoughts/cached)"),
	}
}

func newCounter(meter metric.Meter, name, description string) metric.Int64Counter {
	counter, err := meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		return nil
	}
	return counter
}

// newHistogram builds a duration histogram whose bucket boundaries are
// derived from the operation's own hard timeout (see
// timeoutHistogramBoundaries), instead of a hand-written []float64 that can
// silently fall out of sync with the timeout.
func newHistogram(meter metric.Meter, name, description string, timeout time.Duration) metric.Float64Histogram {
	histogram, err := meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(timeoutHistogramBoundaries(timeout)...),
	)
	if err != nil {
		return nil
	}
	return histogram
}

func recordCounter(ctx context.Context, counter metric.Int64Counter, value int64, attrs ...attribute.KeyValue) {
	if counter != nil {
		counter.Add(ctx, value, metric.WithAttributes(attrs...))
	}
}

func recordHistogram(ctx context.Context, histogram metric.Float64Histogram, value float64, attrs ...attribute.KeyValue) {
	if histogram != nil {
		histogram.Record(ctx, value, metric.WithAttributes(attrs...))
	}
}

func RecordAITransform(ctx context.Context, result string) {
	recordCounter(ctx, activeMetrics.Load().aiTransformTotal, 1, attribute.String("result", result))
}

func RecordAITransformRetry(ctx context.Context, reason, result string) {
	recordCounter(ctx, activeMetrics.Load().aiTransformRetryTotal, 1,
		attribute.String("reason", reason), attribute.String("result", result))
}

func RecordProcurementRun(ctx context.Context, status string) {
	recordCounter(ctx, activeMetrics.Load().procurementRunTotal, 1, attribute.String("status", status))
}

func RecordProcurementCandidateZero(ctx context.Context, lane string) {
	recordCounter(ctx, activeMetrics.Load().candidateZeroTotal, 1, attribute.String("lane", lane))
}

func RecordProcurementExtract(ctx context.Context, result string) {
	recordCounter(ctx, activeMetrics.Load().extractTotal, 1, attribute.String("result", result))
}

func RecordProcurementDegraded(ctx context.Context) {
	recordCounter(ctx, activeMetrics.Load().degradedTotal, 1)
}

func RecordProcurementExtractDuration(ctx context.Context, seconds float64) {
	recordHistogram(ctx, activeMetrics.Load().extractDuration, seconds)
}

func RecordProcurementDialoguePlan(ctx context.Context, act string) {
	recordCounter(ctx, activeMetrics.Load().dialoguePlanTotal, 1, attribute.String("act", act))
}

func RecordProcurementSurfaceRealize(ctx context.Context, source, result string) {
	recordCounter(ctx, activeMetrics.Load().surfaceRealizeTotal, 1,
		attribute.String("source", source), attribute.String("result", result))
}

func RecordProcurementSurfaceRealizeDuration(ctx context.Context, seconds float64) {
	recordHistogram(ctx, activeMetrics.Load().surfaceRealizeDuration, seconds)
}

func RecordProcurementSurfaceValidationReject(ctx context.Context, reason string) {
	recordCounter(ctx, activeMetrics.Load().validationRejectTotal, 1, attribute.String("reason", reason))
}

func RecordProcurementSurfaceFallback(ctx context.Context, act string) {
	recordCounter(ctx, activeMetrics.Load().surfaceFallbackTotal, 1, attribute.String("act", act))
}

func RecordProcurementNarration(ctx context.Context, code, result string) {
	recordCounter(ctx, activeMetrics.Load().narrationTotal, 1,
		attribute.String("code", code), attribute.String("result", result))
}

func RecordProcurementNarrationDuration(ctx context.Context, code string, seconds float64) {
	recordHistogram(ctx, activeMetrics.Load().narrationDuration, seconds, attribute.String("code", code))
}

func RecordProcurementCropSuggest(ctx context.Context, stage, result string) {
	recordCounter(ctx, activeMetrics.Load().cropSuggestTotal, 1,
		attribute.String("stage", stage), attribute.String("result", result))
}

func RecordProcurementCropSuggestDuration(ctx context.Context, stage string, seconds float64) {
	recordHistogram(ctx, activeMetrics.Load().cropSuggestDuration, seconds, attribute.String("stage", stage))
}

func RecordProcurementCropResolverV2(ctx context.Context, outcome string) {
	recordCounter(ctx, activeMetrics.Load().cropResolverV2Total, 1, attribute.String("outcome", outcome))
}

func RecordProcurementLLMUsage(ctx context.Context, role string, usage *genai.GenerateContentResponseUsageMetadata) {
	if usage == nil {
		return
	}
	counter := activeMetrics.Load().llmTokensTotal
	kinds := []struct {
		kind  string
		count int32
	}{
		{"prompt", usage.PromptTokenCount},
		{"completion", usage.CandidatesTokenCount},
		{"thoughts", usage.ThoughtsTokenCount},
		{"cached", usage.CachedContentTokenCount},
	}
	for _, k := range kinds {
		if k.count <= 0 {
			continue
		}
		recordCounter(ctx, counter, int64(k.count),
			attribute.String("role", role), attribute.String("kind", k.kind))
	}
}
