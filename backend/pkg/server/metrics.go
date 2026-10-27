package server

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

const serverMeterName = "github.com/Will413028/hawkr/go-pkg/server"

type lifecycleMetrics struct {
	started  metric.Int64Counter
	shutdown metric.Int64Counter
}

func newLifecycleMetrics(provider *sdkmetric.MeterProvider, logger *zap.Logger) lifecycleMetrics {
	var meter metric.Meter
	if provider == nil {
		meter = otel.Meter(serverMeterName)
	} else {
		meter = provider.Meter(serverMeterName)
	}
	started, err := meter.Int64Counter(
		"server_start_total",
		metric.WithDescription("Number of successful shared server starts."),
	)
	if err != nil {
		logger.Warn("Failed to create server start metric", zap.Error(err))
	}

	shutdown, err := meter.Int64Counter(
		"server_shutdown_total",
		metric.WithDescription("Number of shared server shutdowns completed."),
	)
	if err != nil {
		logger.Warn("Failed to create server shutdown metric", zap.Error(err))
	}

	return lifecycleMetrics{started: started, shutdown: shutdown}
}

func (m lifecycleMetrics) recordStart(ctx context.Context, protocol string) {
	if m.started == nil {
		return
	}
	m.started.Add(ctx, 1, metric.WithAttributes(attribute.String("protocol", protocol)))
}

func (m lifecycleMetrics) recordShutdown(ctx context.Context, protocol string) {
	if m.shutdown == nil {
		return
	}
	m.shutdown.Add(ctx, 1, metric.WithAttributes(attribute.String("protocol", protocol)))
}
