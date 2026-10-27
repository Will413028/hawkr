package config

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("config",
	fx.Provide(Load),
	fx.Provide(func(cfg *Config) *zap.Logger { return logger.NewLogger(cfg.Env) }),
	fx.Invoke(func(l *zap.Logger) { zap.ReplaceGlobals(l) }),
	fx.Provide(initOTel),
)

func initOTel(lc fx.Lifecycle, logger *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("im-service", logger)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("im-service", logger)
	if err := pkgotel.StartRuntimeMetrics(meterProvider); err != nil {
		logger.Warn("Go runtime metrics initialization failed; continuing without runtime instruments", zap.Error(err))
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down tracer provider")
			tracerErr := tracerShutdown(ctx)
			logger.Info("Shutting down meter provider")
			meterErr := meterShutdown(ctx)
			return errors.Join(tracerErr, meterErr)
		},
	})
	return meterProvider
}
