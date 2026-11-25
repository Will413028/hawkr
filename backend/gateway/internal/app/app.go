package app

import (
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/infrastructure"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		config.Module,
		grpcclient.Module,
		handler.Module,
		infrastructure.Module,
	)
}
