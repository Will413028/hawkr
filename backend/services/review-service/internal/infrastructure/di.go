package infrastructure

import (
	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	svcconfig "github.com/Will413028/hawkr/review-service/internal/config"
	"go.uber.org/fx"
)

// newGRPCServerConfig adapts review-service's own *config.Config into the
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
)
