//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

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

func counterValue(t *testing.T, reader *sdkmetric.ManualReader, name, key, value string) int64 {
	t.Helper()
	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect metric %q: %v", name, err)
	}
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
				if got, ok := point.Attributes.Value(attribute.Key(key)); ok && got.AsString() == value {
					return point.Value
				}
			}
		}
	}
	return 0
}
