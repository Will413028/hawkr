package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(NewContentServer),
	fx.Provide(newHealthController),
	fx.Invoke(registerSocialService),
	fx.Invoke(markSocialReady),
)

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.SocialService_ServiceDesc.ServiceName)
}

// registerSocialService runs during fx graph construction, before the shared
// gRPC server starts accepting connections.
func registerSocialService(server *grpclib.Server, handler *ContentServer) {
	pb.RegisterSocialServiceServer(server, handler)
}

func markSocialReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpclib.Server,
	_ *ContentServer,
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
