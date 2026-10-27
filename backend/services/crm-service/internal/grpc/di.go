package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(NewCrmServer),
	fx.Provide(newHealthController),
	fx.Invoke(registerCrmService),
	fx.Invoke(markCrmReady),
)

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.CrmService_ServiceDesc.ServiceName)
}

// registerCrmService runs during fx graph construction, before the shared
// gRPC server's OnStart hook can accept connections.
func registerCrmService(server *grpclib.Server, handler *CrmServer) {
	pb.RegisterCrmServiceServer(server, handler)
}

func markCrmReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpclib.Server,
	_ *CrmServer,
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
