package otel

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	prometheusbridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

type metricExporterFactory func(context.Context, string) (sdkmetric.Exporter, error)

// InitMeterProvider installs the process-wide OpenTelemetry MeterProvider.
//
// Metrics use the signal-specific endpoint when configured, otherwise the
// common OTLP endpoint. If neither is configured, the provider is installed
// without a reader so instrumentation remains safe but no localhost exporter
// is created. Exporter construction is fail-soft, matching InitTracer: a
// telemetry configuration problem must not prevent the service from starting.
// The returned shutdown function flushes and releases the provider.
func InitMeterProvider(serviceName string, logger *zap.Logger) (*sdkmetric.MeterProvider, func(context.Context) error) {
	provider := newMeterProvider(
		context.Background(),
		serviceName,
		configuredOTLPEndpoint(),
		logger,
		newOTLPMetricExporter,
	)
	otel.SetMeterProvider(provider)
	return provider, provider.Shutdown
}

// ValidateMetricsConfiguration enforces the production OTLP metrics contract.
// Development/local environments may intentionally run without a Collector;
// production must declare a usable OTLP HTTP(S) endpoint so removing the
// application /metrics surface cannot silently remove all metrics.
func ValidateMetricsConfiguration(env string) error {
	endpoint := configuredOTLPEndpoint()
	if endpoint == "" {
		if isProductionEnv(env) {
			return fmt.Errorf("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT is required in production")
		}
		return nil
	}

	normalized := normalizeOTLPEndpoint(endpoint)
	parsed, err := url.Parse(normalized)
	if err != nil {
		return fmt.Errorf("invalid OTLP metrics endpoint %q: %w", endpoint, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid OTLP metrics endpoint %q: scheme must be http or https", endpoint)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("invalid OTLP metrics endpoint %q: host is required", endpoint)
	}
	return nil
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func newMeterProvider(
	ctx context.Context,
	serviceName string,
	endpoint string,
	logger *zap.Logger,
	exporterFactory metricExporterFactory,
) *sdkmetric.MeterProvider {
	options := []sdkmetric.Option{
		sdkmetric.WithResource(newServiceResource(serviceName)),
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		exporter, err := exporterFactory(ctx, normalizeOTLPEndpoint(endpoint))
		if err != nil {
			logger.Warn("Failed to create OTLP metrics exporter, metrics will not be exported",
				zap.Error(err))
		} else {
			options = append(options, sdkmetric.WithReader(
				sdkmetric.NewPeriodicReader(exporter,
					sdkmetric.WithProducer(prometheusbridge.NewMetricProducer()),
				),
			))
			logger.Info("OTLP metrics exporter enabled",
				zap.String("endpoint", normalizeOTLPEndpoint(endpoint)),
				zap.Bool("prometheus_bridge", true))
		}
	}

	return sdkmetric.NewMeterProvider(options...)
}

func newOTLPMetricExporter(ctx context.Context, endpoint string) (sdkmetric.Exporter, error) {
	return otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
}

func configuredOTLPEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
}

func normalizeOTLPEndpoint(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	return "http://" + endpoint
}
