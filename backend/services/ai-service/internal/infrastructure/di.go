package infrastructure

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("infrastructure",
	fx.Provide(fx.Annotate(ProvidePool, fx.ResultTags(`name:"ai_pool"`))),
	fx.Provide(newTelemetry),
)

func newTelemetry(lc fx.Lifecycle, l *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("ai-service", l)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("ai-service", l)
	restoreMetrics := observability.InstallMeterProvider(meterProvider)
	if err := pkgotel.StartRuntimeMetrics(meterProvider); err != nil {
		l.Warn("Go runtime metrics initialization failed; continuing without runtime instruments", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			l.Info("Shutting down telemetry providers")
			restoreMetrics()
			return errors.Join(meterShutdown(ctx), tracerShutdown(ctx))
		},
	})
	return meterProvider
}
