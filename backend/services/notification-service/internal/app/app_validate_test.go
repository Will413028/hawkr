package app

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	notifclient "github.com/Will413028/hawkr/notification-service/internal/client"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	notifgrpc "github.com/Will413028/hawkr/notification-service/internal/grpc"
	"github.com/Will413028/hawkr/notification-service/internal/infrastructure"
	"github.com/Will413028/hawkr/notification-service/internal/mq"
	"github.com/Will413028/hawkr/notification-service/internal/repository"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"github.com/Will413028/hawkr/notification-service/internal/worker"
	"go.uber.org/fx"
)

// TestFxGraph_Valid makes the complete notification graph observable to
// fx.ValidateApp, including the final health, telemetry, and shared gRPC
// bootstrap consumers. It catches a missing named/optional dependency before
// a deployment can hide it behind a readiness failure.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		repository.Module,
		mq.Module,
		notifclient.Module,
		usecase.Module,
		notifgrpc.Module,
		worker.Module,
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
