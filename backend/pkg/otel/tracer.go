package otel

import (
	"context"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// InitTracer initializes the global TracerProvider with OTLP gRPC exporter.
// Returns a shutdown function. If OTEL_EXPORTER_OTLP_ENDPOINT is not set,
// traces are still created (for context propagation) but not exported.
func InitTracer(serviceName string, logger *zap.Logger) func(context.Context) error {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(newServiceResource(serviceName)),
		sdktrace.WithSampler(buildSampler()),
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint != "" {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		endpoint = strings.TrimPrefix(endpoint, "http://")
		exporter, err := otlptracegrpc.New(context.Background(),
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			logger.Warn("Failed to create OTLP exporter, traces will not be exported",
				zap.Error(err))
		} else {
			opts = append(opts, sdktrace.WithBatcher(exporter))
			logger.Info("OTLP trace exporter enabled",
				zap.String("endpoint", endpoint))
		}
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown
}

func buildSampler() sdktrace.Sampler {
	ratio := 1.0
	if v := os.Getenv("TRACING_SAMPLING"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed >= 0 && parsed <= 1 {
			ratio = parsed
		}
	}
	root := sdktrace.TraceIDRatioBased(ratio)
	return sdktrace.ParentBased(root, sdktrace.WithRemoteParentNotSampled(root))
}
