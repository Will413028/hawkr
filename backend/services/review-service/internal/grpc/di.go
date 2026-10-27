package grpc

import (
	"context"

	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(NewReviewServer),
	fx.Provide(newHealthController),
	fx.Invoke(registerReviewService),
	fx.Invoke(markReviewReady),
)

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.ReviewService_ServiceDesc.ServiceName)
}

// registerReviewService runs during FX graph construction, before the shared
// gRPC server's OnStart hook can accept connections.
func registerReviewService(server *grpclib.Server, handler *ReviewServer) {
	pb.RegisterReviewServiceServer(server, handler)
}

func markReviewReady(
	lc fx.Lifecycle,
	health *pkgserver.HealthController,
	_ *grpclib.Server,
	_ *ReviewServer,
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
