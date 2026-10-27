package infrastructure

import (
	"context"
	"errors"

	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides infrastructure dependencies.
var Module = fx.Module("infrastructure",
	fx.Provide(NewFirebaseNotifier),
	fx.Provide(NewTokenDeactivationPolicy),
	fx.Provide(newTelemetry),
)

func newTelemetry(lc fx.Lifecycle, l *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("notification-service", l)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("notification-service", l)
	if err := pkgotel.StartRuntimeMetrics(meterProvider); err != nil {
		l.Warn("Go runtime metrics initialization failed; continuing without runtime instruments", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			l.Info("Shutting down telemetry providers")
			return errors.Join(meterShutdown(ctx), tracerShutdown(ctx))
		},
	})
	return meterProvider
}
