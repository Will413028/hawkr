package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// testingTB is the slice of testing.TB this file needs; importing "testing"
// from a non-test file would link its flags into the service binary.
type testingTB interface {
	Helper()
	Cleanup(func())
	Fatalf(format string, args ...any)
}

// InstallManualReader routes business metrics to a ManualReader for the
// test's lifetime and returns a collector that snapshots everything recorded
// so far. Tests in other packages use it to assert on series by name.
func InstallManualReader(tb testingTB) func() metricdata.ResourceMetrics {
	tb.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	restore := InstallMeterProvider(provider)
	tb.Cleanup(func() {
		restore()
		_ = provider.Shutdown(context.Background())
	})
	return func() metricdata.ResourceMetrics {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			tb.Fatalf("collect metrics: %v", err)
		}
		return rm
	}
}

// CounterValue returns the value of the int64 sum data point of name whose
// attributes contain every pair in attrs, requiring exactly one match;
// ambiguity (zero or more than one matching data point) is treated as not found.
func CounterValue(rm metricdata.ResourceMetrics, name string, attrs map[string]string) (int64, bool) {
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			var value int64
			matches := 0
			for _, p := range sum.DataPoints {
				if attrsContain(p.Attributes, attrs) {
					value = p.Value
					matches++
				}
			}
			if matches != 1 {
				continue
			}
			return value, true
		}
	}
	return 0, false
}

// HistogramCount returns the observation count of the float64 histogram data
// point of name whose attributes contain every pair in attrs, requiring
// exactly one match; ambiguity (zero or more than one matching data point) is
// treated as not found.
func HistogramCount(rm metricdata.ResourceMetrics, name string, attrs map[string]string) (uint64, bool) {
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			h, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				continue
			}
			var count uint64
			matches := 0
			for _, p := range h.DataPoints {
				if attrsContain(p.Attributes, attrs) {
					count = p.Count
					matches++
				}
			}
			if matches != 1 {
				continue
			}
			return count, true
		}
	}
	return 0, false
}

func attrsContain(set attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := set.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}
