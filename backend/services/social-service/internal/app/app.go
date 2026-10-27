package app

import (
	"github.com/Will413028/hawkr/social-service/internal/client"
	"github.com/Will413028/hawkr/social-service/internal/config"
	contentgrpc "github.com/Will413028/hawkr/social-service/internal/grpc"
	"github.com/Will413028/hawkr/social-service/internal/infrastructure"
	socialmq "github.com/Will413028/hawkr/social-service/internal/mq"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"github.com/Will413028/hawkr/social-service/internal/worker"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		socialmq.Module,
		usecase.Module,
		contentgrpc.Module,
		worker.Module,
	)
}
