package app

import (
	"github.com/Will413028/hawkr/gateway/internal/config"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/infrastructure"
	gatewaymq "github.com/Will413028/hawkr/gateway/internal/mq"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/sse"
	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		config.Module,
		grpcclient.Module,
		sse.Module,
		realtime.Module,
		handler.Module,
		gatewaymq.Module,
		infrastructure.Module,
	)
}
