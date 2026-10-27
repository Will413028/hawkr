package observability

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/genai"
)

func TestMetrics_RecordFunctionsExportOTLP(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	restore := InstallMeterProvider(provider)
	t.Cleanup(func() {
		restore()
		_ = provider.Shutdown(context.Background())
	})

	ctx := context.Background()
	RecordAITransform(ctx, "success")
	RecordAITransformRetry(ctx, "NO_IMAGE", "success")
	RecordProcurementRun(ctx, "SUCCEEDED")
	RecordProcurementCandidateZero(ctx, "PLATFORM")
	RecordProcurementExtract(ctx, "success")
	RecordProcurementDegraded(ctx)
	RecordProcurementExtractDuration(ctx, 0.5)
	RecordProcurementDialoguePlan(ctx, "ACK_AND_ASK_REQUIRED")
	RecordProcurementSurfaceRealize(ctx, "model", "success")
	RecordProcurementSurfaceRealizeDuration(ctx, 0.5)
	RecordProcurementSurfaceValidationReject(ctx, "claim_violation")
	RecordProcurementSurfaceFallback(ctx, "ACK_AND_ASK_REQUIRED")
	RecordProcurementNarration(ctx, "VALIDATE_SLOTS", "success")
	RecordProcurementNarrationDuration(ctx, "VALIDATE_SLOTS", 0.4)
	RecordProcurementCropSuggest(ctx, "GROUP", "rejected")
	RecordProcurementCropSuggestDuration(ctx, "GROUP", 1.2)
	RecordProcurementLLMUsage(ctx, "extract", &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount: 120, CandidatesTokenCount: 15, ThoughtsTokenCount: 0, CachedContentTokenCount: 30,
	})

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		t.Fatalf("collect AI business metrics: %v", err)
	}

	assertCounter(t, metrics, "ai_transform_total", map[string]string{"result": "success"}, 1)
	assertCounter(t, metrics, "ai_transform_retry_total", map[string]string{"reason": "NO_IMAGE", "result": "success"}, 1)
	assertCounter(t, metrics, "procurement_run_total", map[string]string{"status": "SUCCEEDED"}, 1)
	assertCounter(t, metrics, "procurement_candidate_zero_total", map[string]string{"lane": "PLATFORM"}, 1)
	assertCounter(t, metrics, "procurement_extract_total", map[string]string{"result": "success"}, 1)
	assertCounter(t, metrics, "procurement_degraded_total", nil, 1)
	assertHistogram(t, metrics, "procurement_extract_duration_seconds", 1)
	assertCounter(t, metrics, "procurement_dialogue_plan_total", map[string]string{"act": "ACK_AND_ASK_REQUIRED"}, 1)
	assertCounter(t, metrics, "procurement_surface_realize_total", map[string]string{"source": "model", "result": "success"}, 1)
	assertHistogram(t, metrics, "procurement_surface_realize_duration_seconds", 1)
	assertCounter(t, metrics, "procurement_surface_validation_reject_total", map[string]string{"reason": "claim_violation"}, 1)
	assertCounter(t, metrics, "procurement_surface_fallback_total", map[string]string{"act": "ACK_AND_ASK_REQUIRED"}, 1)
	assertCounter(t, metrics, "procurement_narration_total", map[string]string{"code": "VALIDATE_SLOTS", "result": "success"}, 1)
	assertHistogram(t, metrics, "procurement_narration_duration_seconds", 1)
	assertCounter(t, metrics, "procurement_crop_suggest_total", map[string]string{"stage": "GROUP", "result": "rejected"}, 1)
	assertHistogram(t, metrics, "procurement_crop_suggest_duration_seconds", 1)
	assertCounter(t, metrics, "procurement_llm_tokens_total", map[string]string{"role": "extract", "kind": "prompt"}, 120)
	assertCounter(t, metrics, "procurement_llm_tokens_total", map[string]string{"role": "extract", "kind": "completion"}, 15)
	assertCounter(t, metrics, "procurement_llm_tokens_total", map[string]string{"role": "extract", "kind": "cached"}, 30)
}

func assertCounter(t *testing.T, metrics metricdata.ResourceMetrics, name string, wantAttrs map[string]string, want int64) {
	t.Helper()
	got, ok := CounterValue(metrics, name, wantAttrs)
	if !ok || got != want {
		t.Fatalf("metric %q with attributes %v did not have value %d: %v", name, wantAttrs, want, metrics)
	}
}

func assertHistogram(t *testing.T, metrics metricdata.ResourceMetrics, name string, wantCount uint64) {
	t.Helper()
	got, ok := HistogramCount(metrics, name, nil)
	if !ok || got != wantCount {
		t.Fatalf("metric %q did not have one data point with count %d: %v", name, wantCount, metrics)
	}
}

func TestRecordProcurementLLMUsage_SkipsNilAndZero(t *testing.T) {
	collect := InstallManualReader(t)
	ctx := context.Background()

	RecordProcurementLLMUsage(ctx, "narrate", nil)
	RecordProcurementLLMUsage(ctx, "narrate", &genai.GenerateContentResponseUsageMetadata{})

	rm := collect()
	if _, ok := CounterValue(rm, "procurement_llm_tokens_total", map[string]string{"role": "narrate"}); ok {
		t.Fatalf("nil or all-zero usage must not create a token series: %v", rm)
	}
}

func TestInstallManualReader_HelpersReadBackValues(t *testing.T) {
	collect := InstallManualReader(t)
	ctx := context.Background()

	RecordProcurementNarration(ctx, "CROP_GATE", "error")
	RecordProcurementNarrationDuration(ctx, "CROP_GATE", 2.5)

	rm := collect()
	v, ok := CounterValue(rm, "procurement_narration_total", map[string]string{"code": "CROP_GATE", "result": "error"})
	if !ok || v != 1 {
		t.Fatalf("counter = (%d, %v), want (1, true)", v, ok)
	}
	c, ok := HistogramCount(rm, "procurement_narration_duration_seconds", map[string]string{"code": "CROP_GATE"})
	if !ok || c != 1 {
		t.Fatalf("histogram count = (%d, %v), want (1, true)", c, ok)
	}
	if _, ok := HistogramCount(rm, "procurement_narration_duration_seconds", map[string]string{"code": "VALIDATE_SLOTS"}); ok {
		t.Fatal("histogram lookup must be attribute-scoped")
	}
}

func TestCounterValueAndHistogramCount_AmbiguousMatchIsNotFound(t *testing.T) {
	collect := InstallManualReader(t)
	ctx := context.Background()

	RecordProcurementNarration(ctx, "A", "success")
	RecordProcurementNarration(ctx, "B", "success")
	RecordProcurementNarrationDuration(ctx, "A", 1)
	RecordProcurementNarrationDuration(ctx, "B", 2)

	rm := collect()
	if _, ok := CounterValue(rm, "procurement_narration_total", map[string]string{"result": "success"}); ok {
		t.Fatal("counter lookup matching two data points must return not-found")
	}
	if v, ok := CounterValue(rm, "procurement_narration_total", map[string]string{"code": "A", "result": "success"}); !ok || v != 1 {
		t.Fatalf("counter = (%d, %v), want (1, true)", v, ok)
	}
	if _, ok := HistogramCount(rm, "procurement_narration_duration_seconds", nil); ok {
		t.Fatal("histogram lookup matching two data points must return not-found")
	}
	if c, ok := HistogramCount(rm, "procurement_narration_duration_seconds", map[string]string{"code": "A"}); !ok || c != 1 {
		t.Fatalf("histogram count = (%d, %v), want (1, true)", c, ok)
	}
}

func TestCounterValueAndHistogramCount_SkipTypeMismatchInOtherScope(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	restore := InstallMeterProvider(provider)
	t.Cleanup(func() {
		restore()
		_ = provider.Shutdown(context.Background())
	})
	ctx := context.Background()

	other := provider.Meter("other")
	h, err := other.Float64Histogram("procurement_narration_total")
	if err != nil {
		t.Fatalf("create colliding instrument: %v", err)
	}
	h.Record(ctx, 1)
	c, err := other.Int64Counter("procurement_narration_duration_seconds")
	if err != nil {
		t.Fatalf("create colliding instrument: %v", err)
	}
	c.Add(ctx, 1)

	RecordProcurementNarration(ctx, "CROP_GATE", "success")
	RecordProcurementNarrationDuration(ctx, "CROP_GATE", 1.5)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(rm.ScopeMetrics) < 2 {
		t.Fatalf("want at least two scopes, got %d: %v", len(rm.ScopeMetrics), rm)
	}
	if v, ok := CounterValue(rm, "procurement_narration_total", map[string]string{"code": "CROP_GATE", "result": "success"}); !ok || v != 1 {
		t.Fatalf("counter = (%d, %v), want (1, true)", v, ok)
	}
	if cnt, ok := HistogramCount(rm, "procurement_narration_duration_seconds", map[string]string{"code": "CROP_GATE"}); !ok || cnt != 1 {
		t.Fatalf("histogram count = (%d, %v), want (1, true)", cnt, ok)
	}
}
