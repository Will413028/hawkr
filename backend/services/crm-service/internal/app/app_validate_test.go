package app

import (
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/client"
	"github.com/Will413028/hawkr/crm-service/internal/config"
	crmgrpc "github.com/Will413028/hawkr/crm-service/internal/grpc"
	"github.com/Will413028/hawkr/crm-service/internal/infrastructure"
	"github.com/Will413028/hawkr/crm-service/internal/repository"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/crm-service/internal/worker"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
)

// TestFxGraph_Valid makes the complete CRM graph observable to fx.ValidateApp,
// including the final health controller, telemetry, and shared gRPC bootstrap
// consumers. A missing final-mode dependency must fail here instead of only
// surfacing after deployment.
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		infrastructure.Module,
		repository.Module,
		client.Module,
		usecase.Module,
		worker.Module,
		crmgrpc.Module,
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
