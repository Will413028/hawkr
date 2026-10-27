package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(NewSellerServer),
	fx.Provide(newHealthController),
	fx.Invoke(registerSellerService),
	fx.Invoke(markSellerReady),
)

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.SellerService_ServiceDesc.ServiceName)
}

// registerSellerService runs during fx graph construction, before the shared
// gRPC server starts accepting connections.
func registerSellerService(server *grpclib.Server, handler *SellerServer) {
	pb.RegisterSellerServiceServer(server, handler)
}

func markSellerReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpclib.Server,
	_ *SellerServer,
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
