package infrastructure

import (
	"context"
	"errors"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("infrastructure",
	fx.Provide(func(cfg *config.Config) pkgserver.ServerConfig {
		return pkgserver.ServerConfig{
			ServerConfig: pkgconfig.ServerConfig{
				GRPCPort: cfg.Server.GRPCPort,
			},
			Env: cfg.Env,
			// EnableStreamInterceptors left false (zero value): GetObservedPrice
			// is unary-only, same as the former grpc.go comment this replaces.
		}
	}),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Provide(newTelemetry),
)

func newTelemetry(lc fx.Lifecycle, logger *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("pricebook-service", logger)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("pricebook-service", logger)
	if err := pkgotel.StartRuntimeMetrics(meterProvider); err != nil {
		logger.Warn("Go runtime metrics initialization failed; continuing without runtime instruments", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down telemetry providers")
			return errors.Join(meterShutdown(ctx), tracerShutdown(ctx))
		},
	})
	return meterProvider
}
