package app

import (
	"github.com/Will413028/hawkr/pricebook-service/internal/app/cron"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	pricebookgrpc "github.com/Will413028/hawkr/pricebook-service/internal/grpc"
	"github.com/Will413028/hawkr/pricebook-service/internal/infrastructure"
	pricebookpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"github.com/Will413028/hawkr/pricebook-service/internal/worker"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		config.Module,
		usecase.Module,
		infrastructure.Module,
		cron.Module,
		pricebookpostgres.Module,
		pricebookgrpc.Module,
		worker.Module,
	)
}
