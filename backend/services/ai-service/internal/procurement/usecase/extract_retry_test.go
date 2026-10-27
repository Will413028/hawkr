package usecase

import (
	"context"
	"errors"
	"testing"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// scriptedExtractor 依序回傳固定的 (patch, err)，呼叫次數超過腳本長度時重複回傳
// 最後一筆——用來模擬 retryingExtractor 面對「首次失敗、重試成功／仍失敗」等組合。
type scriptedExtractor struct {
	results []scriptedResult
	calls   int
}

type scriptedResult struct {
	patch       domain.SlotPatch
	shortIntent string
	err         error
}

func (s *scriptedExtractor) Extract(_ context.Context, _ string, _ domain.Slots) (ExtractResult, error) {
	i := s.calls
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	s.calls++
	r := s.results[i]
	return ExtractResult{Patch: r.patch, ShortIntent: r.shortIntent}, r.err
}

func installTestMeterProvider(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	restore := observability.InstallMeterProvider(provider)
	t.Cleanup(func() {
		restore()
		_ = provider.Shutdown(context.Background())
	})
	return reader
}

func histogramSampleCount(t *testing.T, reader *sdkmetric.ManualReader) uint64 {
	t.Helper()
	var metrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &metrics))
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "procurement_extract_duration_seconds" {
				continue
			}
			histogram, ok := metric.Data.(metricdata.Histogram[float64])
			if !ok || len(histogram.DataPoints) == 0 {
				return 0
			}
			return histogram.DataPoints[0].Count
		}
	}
	return 0
}

func counterValue(t *testing.T, reader *sdkmetric.ManualReader, name string, labels ...string) int64 {
	t.Helper()
	if len(labels)%2 != 0 {
		t.Fatalf("counterValue labels must be key/value pairs, got %v", labels)
	}
	var metrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &metrics))
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %q data type = %T, want metricdata.Sum[int64]", name, metric.Data)
			}
			for _, point := range sum.DataPoints {
				matches := true
				for i := 0; i < len(labels); i += 2 {
					got, ok := point.Attributes.Value(attribute.Key(labels[i]))
					if !ok || got.AsString() != labels[i+1] {
						matches = false
						break
					}
				}
				if matches {
					return point.Value
				}
			}
		}
	}
	return 0
}

func TestRetryingExtractor_SucceedsFirstTry_NoRetry(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{patch: domain.SlotPatch{Item: ptr("寶島梨禮盒")}},
	}}
	re := &retryingExtractor{inner: inner}

	result, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 1, inner.calls, "第一次就成功不該多打一次")
}

func TestRetryingExtractor_RetriesOnceOnProviderUnavailableError_ThenSucceeds(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{err: &aidomain.ProviderUnavailableError{Err: errors.New("upstream 503")}},
		{patch: domain.SlotPatch{Item: ptr("寶島梨禮盒")}},
	}}
	re := &retryingExtractor{inner: inner}

	result, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.NoError(t, err)
	require.Equal(t, "寶島梨禮盒", *result.Patch.Item)
	require.Equal(t, 2, inner.calls, "基礎設施性失敗要補打一次")
}

func TestRetryingExtractor_DoesNotRetryNonRetryableError(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{err: errors.New("400 invalid argument")},
	}}
	re := &retryingExtractor{inner: inner}

	_, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.Error(t, err)
	require.Equal(t, 1, inner.calls, "非可重試錯誤不該補打，浪費使用者等待時間")
}

func TestRetryingExtractor_RetriesAtMostOnce(t *testing.T) {
	inner := &scriptedExtractor{results: []scriptedResult{
		{err: &aidomain.ProviderUnavailableError{Err: errors.New("upstream 503")}},
		{err: &aidomain.ProviderUnavailableError{Err: errors.New("still 503")}},
	}}
	re := &retryingExtractor{inner: inner}

	_, err := re.Extract(context.Background(), "msg", domain.Slots{})

	require.Error(t, err)
	require.Equal(t, 2, inner.calls,
		"最多重試一次——沿用 callGeminiWithRetry(transform_usecase.go) 同一條 PayUni 錯誤收斂規範 §8.1")
}

func TestRetryingExtractor_RecordsSuccessMetric(t *testing.T) {
	reader := installTestMeterProvider(t)
	before := counterValue(t, reader, "procurement_extract_total", "result", "success")

	inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{}}}}
	re := &retryingExtractor{inner: inner}
	_, err := re.Extract(context.Background(), "msg", domain.Slots{})
	require.NoError(t, err)

	require.Equal(t, before+1, counterValue(t, reader, "procurement_extract_total", "result", "success"))
}

func TestRetryingExtractor_RecordsErrorMetric(t *testing.T) {
	reader := installTestMeterProvider(t)
	before := counterValue(t, reader, "procurement_extract_total", "result", "error")

	inner := &scriptedExtractor{results: []scriptedResult{{err: errors.New("400 invalid argument")}}}
	re := &retryingExtractor{inner: inner}
	_, err := re.Extract(context.Background(), "msg", domain.Slots{})
	require.Error(t, err)

	require.Equal(t, before+1, counterValue(t, reader, "procurement_extract_total", "result", "error"))
}

func TestRetryingExtractor_RecordsTimeoutMetric(t *testing.T) {
	reader := installTestMeterProvider(t)
	before := counterValue(t, reader, "procurement_extract_total", "result", "timeout")

	inner := &scriptedExtractor{results: []scriptedResult{
		{err: &aidomain.ProviderUnavailableError{Err: context.DeadlineExceeded}},
		{err: &aidomain.ProviderUnavailableError{Err: context.DeadlineExceeded}},
	}}
	re := &retryingExtractor{inner: inner}
	_, err := re.Extract(context.Background(), "msg", domain.Slots{})
	require.Error(t, err)

	require.Equal(t, before+1, counterValue(t, reader, "procurement_extract_total", "result", "timeout"))
}

func TestRetryingExtractor_ObservesDuration(t *testing.T) {
	reader := installTestMeterProvider(t)
	before := histogramSampleCount(t, reader)

	inner := &scriptedExtractor{results: []scriptedResult{{patch: domain.SlotPatch{}}}}
	re := &retryingExtractor{inner: inner}
	_, err := re.Extract(context.Background(), "msg", domain.Slots{})
	require.NoError(t, err)

	require.Equal(t, before+1, histogramSampleCount(t, reader))
}
