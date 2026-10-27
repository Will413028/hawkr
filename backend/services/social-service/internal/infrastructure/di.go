package infrastructure

import (
	"context"
	"errors"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	svcconfig "github.com/Will413028/hawkr/social-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// newGRPCServerConfig adapts social-service's own *config.Config into the
// package-agnostic pkgserver.ServerConfig that pkgserver.NewGRPCServer needs —
// see pkg/server/grpc.go for the shared interceptor chain this replaces.
func newGRPCServerConfig(cfg *svcconfig.Config) pkgserver.ServerConfig {
	return pkgserver.ServerConfig{
		ServerConfig:             pkgconfig.ServerConfig{GRPCPort: cfg.Server.GRPCPort},
		Env:                      cfg.Env,
		EnableStreamInterceptors: true,
	}
}

var Module = fx.Module("infrastructure",
	fx.Provide(NewRedisClient),
	fx.Provide(newGRPCServerConfig),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Provide(newTelemetry),
)

func newTelemetry(lc fx.Lifecycle, l *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("social-service", l)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("social-service", l)
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
