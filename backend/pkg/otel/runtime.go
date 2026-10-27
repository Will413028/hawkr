package otel

import (
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// StartRuntimeMetrics registers the standard Go runtime instruments with the
// supplied provider. A final gRPC-only service uses these in place of the
// process/go collectors that used to be exposed through /metrics.
func StartRuntimeMetrics(provider metric.MeterProvider) error {
	if provider == nil {
		provider = otel.GetMeterProvider()
	}
	return runtime.Start(runtime.WithMeterProvider(provider))
}
