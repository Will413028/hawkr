package infrastructure

import (
	"context"
	"errors"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	svcconfig "github.com/Will413028/hawkr/seller-service/internal/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// newGRPCServerConfig adapts seller-service's own *config.Config into the
// package-agnostic pkgserver.ServerConfig that pkgserver.NewGRPCServer needs —
// see pkg/server/grpc.go for the shared interceptor chain this replaces.
// Only GRPCPort is copied intentionally: seller-service is a final-mode
// gRPC-only process and must not acquire a management HTTP listener through
// the shared config shape.
func newGRPCServerConfig(cfg *svcconfig.Config) pkgserver.ServerConfig {
	return pkgserver.ServerConfig{
		ServerConfig:             pkgconfig.ServerConfig{GRPCPort: cfg.Server.GRPCPort},
		Env:                      cfg.Env,
		EnableStreamInterceptors: true,
	}
}

var Module = fx.Module("infrastructure",
	fx.Provide(newGRPCServerConfig),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Provide(newTelemetry),
	// B-169 — Redis client + public violation cache (D8)
	fx.Provide(NewRedisClient),
	fx.Provide(NewPublicViolationCache),
	fx.Provide(NewBusinessRegistrationCache),
	fx.Provide(NewGeocodeCache),
	// 32 §13 / 46 §4.2 — admin manual suspension state propagation
	fx.Provide(NewSellerSuspensionRedis),
)

func newTelemetry(lc fx.Lifecycle, l *zap.Logger) *sdkmetric.MeterProvider {
	tracerShutdown := pkgotel.InitTracer("seller-service", l)
	meterProvider, meterShutdown := pkgotel.InitMeterProvider("seller-service", l)
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
