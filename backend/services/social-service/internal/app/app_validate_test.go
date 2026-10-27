package app

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/social-service/internal/client"
	"github.com/Will413028/hawkr/social-service/internal/config"
	socialgrpc "github.com/Will413028/hawkr/social-service/internal/grpc"
	"github.com/Will413028/hawkr/social-service/internal/infrastructure"
	socialmq "github.com/Will413028/hawkr/social-service/internal/mq"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"github.com/Will413028/hawkr/social-service/internal/worker"
	"go.uber.org/fx"
)

// TestFxGraph_Valid asserts the complete application graph is resolvable. The
// explicit final-mode health and gRPC params invokes keep the readiness
// controller and shared bootstrap contract visible to fx.ValidateApp.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		socialmq.Module,
		usecase.Module,
		socialgrpc.Module,
		worker.Module,
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
