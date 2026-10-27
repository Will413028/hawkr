package app

import (
	"github.com/Will413028/hawkr/review-service/internal/client"
	"github.com/Will413028/hawkr/review-service/internal/config"
	reviewgrpc "github.com/Will413028/hawkr/review-service/internal/grpc"
	"github.com/Will413028/hawkr/review-service/internal/infrastructure"
	"github.com/Will413028/hawkr/review-service/internal/mq"
	"github.com/Will413028/hawkr/review-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		mq.Module,
		usecase.Module,
		reviewgrpc.Module,
	)
}
