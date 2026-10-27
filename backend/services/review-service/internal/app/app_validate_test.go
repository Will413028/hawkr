package app

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/review-service/internal/client"
	"github.com/Will413028/hawkr/review-service/internal/config"
	reviewgrpc "github.com/Will413028/hawkr/review-service/internal/grpc"
	"github.com/Will413028/hawkr/review-service/internal/infrastructure"
	"github.com/Will413028/hawkr/review-service/internal/mq"
	"github.com/Will413028/hawkr/review-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"go.uber.org/fx"
)

// TestFxGraph_Valid asserts the complete application graph is resolvable. The
// explicit final health and shared gRPC params invokes keep the generated
// service registration and readiness contract visible to fx.ValidateApp.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		mq.Module,
		usecase.Module,
		reviewgrpc.Module,
		fx.Invoke(func(*server.HealthController) {}),
		fx.Invoke(func(server.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
