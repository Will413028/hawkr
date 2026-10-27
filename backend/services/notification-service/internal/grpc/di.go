package grpc

import (
	"context"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// newGRPCServerConfig adapts notification-service's own *config.Config into
// the package-agnostic pkgserver.ServerConfig that pkgserver.NewGRPCServer
// needs — see pkg/server/grpc.go for the shared interceptor chain this
// replaces. No streaming RPCs, so EnableStreamInterceptors stays false.
// notification's final runtime only has a gRPC port, so copy the service-local
// value into the package-agnostic embedded config explicitly.
func newGRPCServerConfig(cfg *config.Config) pkgserver.ServerConfig {
	return pkgserver.ServerConfig{
		ServerConfig: pkgconfig.ServerConfig{
			GRPCPort: cfg.Server.GRPCPort,
		},
		Env: cfg.Env,
	}
}

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.NotificationService_ServiceDesc.ServiceName)
}

// registerNotificationService runs as an fx.Invoke during fx.New()'s graph
// construction — strictly before app.Start() fires pkgserver.NewGRPCServer's
// OnStart hook (which calls server.Serve), so registration always completes
// before the listener accepts a connection. Same pattern as ai-service's
// registerAIProcurementService.
func registerNotificationService(s *grpc.Server, handler *Server) {
	pb.RegisterNotificationServiceServer(s, handler)
}

func markNotificationReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpc.Server,
	_ *Server,
) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			health.MarkReady()
			return nil
		},
		OnStop: func(context.Context) error {
			health.MarkNotReady()
			return nil
		},
	})
}

var Module = fx.Module("grpc",
	fx.Provide(NewServer),
	fx.Provide(newGRPCServerConfig),
	fx.Provide(newHealthController),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Invoke(registerNotificationService),
	fx.Invoke(markNotificationReady),
)
