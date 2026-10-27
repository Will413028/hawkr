package app

import (
	"github.com/Will413028/hawkr/seller-service/internal/app/cron"
	"github.com/Will413028/hawkr/seller-service/internal/client"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	sellergrpc "github.com/Will413028/hawkr/seller-service/internal/grpc"
	"github.com/Will413028/hawkr/seller-service/internal/infrastructure"
	"github.com/Will413028/hawkr/seller-service/internal/mq"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/Will413028/hawkr/seller-service/internal/worker"
	"go.uber.org/fx"
)

// New assembles all fx.Modules into the application.
func New() *fx.App {
	return fx.New(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		usecase.Module,
		worker.Module,
		mq.Module,
		sellergrpc.Module,
		cron.Module,
	)
}
