package interceptor

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/stats"
)

// OTelServerStatsHandler returns a stats.Handler for OpenTelemetry server-side tracing.
// Use this with grpc.StatsHandler() option when creating a gRPC server.
// This is the recommended approach for OTel instrumentation (replaces interceptors).
//
// The default handler intentionally uses a no-op meter. The Collector currently
// derives the repository's gRPC RED metrics from spans via its spanmetrics
// connector; enabling otelgrpc's direct duration metric at the same time would
// create a second source with different names and aggregation semantics.
// Use OTelServerStatsHandlerWithMetrics explicitly when a service has completed
// that RED cutover.
func OTelServerStatsHandler() stats.Handler {
	return newOTelServerStatsHandler(metricnoop.NewMeterProvider())
}

// OTelServerStatsHandlerWithMetrics opts into otelgrpc's direct metrics. This
// is kept separate from OTelServerStatsHandler so a process-wide MeterProvider
// cannot silently change the repository's existing spanmetrics contract.
func OTelServerStatsHandlerWithMetrics() stats.Handler {
	return newOTelServerStatsHandler(otel.GetMeterProvider())
}

func newOTelServerStatsHandler(meterProvider metric.MeterProvider) stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
		otelgrpc.WithMeterProvider(meterProvider),
	)
}

// OTelClientStatsHandler returns a stats.Handler for OpenTelemetry client-side tracing.
// Use this with grpc.WithStatsHandler() option when creating a gRPC client connection.
func OTelClientStatsHandler() stats.Handler {
	return newOTelClientStatsHandler(metricnoop.NewMeterProvider())
}

// OTelClientStatsHandlerWithMetrics opts into otelgrpc's direct client metrics.
// It is the client-side counterpart to OTelServerStatsHandlerWithMetrics.
func OTelClientStatsHandlerWithMetrics() stats.Handler {
	return newOTelClientStatsHandler(otel.GetMeterProvider())
}

func newOTelClientStatsHandler(meterProvider metric.MeterProvider) stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
		otelgrpc.WithMeterProvider(meterProvider),
	)
}

// SetupOTelPropagator sets the global text map propagator for trace context propagation.
// Call this during application initialization.
func SetupOTelPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}
