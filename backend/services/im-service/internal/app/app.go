package app

import (
	"github.com/Will413028/hawkr/im-service/internal/client"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	imgrpc "github.com/Will413028/hawkr/im-service/internal/grpc"
	"github.com/Will413028/hawkr/im-service/internal/infrastructure"
	immq "github.com/Will413028/hawkr/im-service/internal/mq"
	"github.com/Will413028/hawkr/im-service/internal/repository"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"github.com/Will413028/hawkr/im-service/internal/worker"
	"go.uber.org/fx"
)

// New creates a new fx application with all modules assembled.
func New() *fx.App {
	return fx.New(
		config.Module,
		infrastructure.Module,
		client.Module,
		repository.Module,
		usecase.Module,
		immq.Module,
		worker.Module,
		imgrpc.Module,
		fx.Invoke(startHub),
	)
}

func startHub(hub domain.Hub) {
	go hub.Run()
}
