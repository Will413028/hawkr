// Package infrastructure 是 Layer 1：三個獨立的監聽面。
//
//	10040 HTTP  內部運維面（health / metrics / pprof）—— 走 pkg/server 的共用 router，
//	            永不承載業務 API（pkg/server/http_test.go 的
//	            TestNewRouter_ExposesOnlyOperationalRoutes 把它釘死在那三條）
//	10041 gRPC  業務面，唯一 caller 是 Gateway
//	10042 HTTP  PDF 授權下載通道（§6.1.4）—— 獨立的 http.Server，見 file_server.go
package infrastructure

import (
	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"go.uber.org/fx"
)

// Module provides infrastructure dependencies.
var Module = fx.Module("infrastructure",
	fx.Provide(func(cfg *config.Config) pkgserver.ServerConfig {
		return pkgserver.ServerConfig{
			ServerConfig: pkgconfig.ServerConfig{
				GRPCPort: cfg.Server.GRPCPort,
				HTTPPort: cfg.Server.HTTPPort,
			},
			Env: cfg.Env,
		}
	}),
	fx.Provide(fx.Annotate(
		func() string { return "invoice-service" },
		fx.ResultTags(`name:"service_name"`),
	)),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Invoke(pkgserver.NewInternalHTTPServer),
	fx.Invoke(NewFileHTTPServer),
)
