package app

import (
	"github.com/Will413028/hawkr/crm-service/internal/client"
	"github.com/Will413028/hawkr/crm-service/internal/config"
	crmgrpc "github.com/Will413028/hawkr/crm-service/internal/grpc"
	"github.com/Will413028/hawkr/crm-service/internal/infrastructure"
	"github.com/Will413028/hawkr/crm-service/internal/repository"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/crm-service/internal/worker"
	"go.uber.org/fx"
)

// New assembles all fx.Modules into the application.
func New() *fx.App {
	return fx.New(
		config.Module,
		infrastructure.Module,
		repository.Module,
		client.Module,
		usecase.Module,
		worker.Module,
		crmgrpc.Module,
	)
}
