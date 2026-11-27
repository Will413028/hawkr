package app

import (
	"github.com/Will413028/hawkr/invoice-service/internal/client"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	invoicegrpc "github.com/Will413028/hawkr/invoice-service/internal/grpc"
	"github.com/Will413028/hawkr/invoice-service/internal/infrastructure"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"github.com/Will413028/hawkr/invoice-service/internal/worker"
	"go.uber.org/fx"
)

// New creates the Fx application with all modules wired.
//
// 分層順序是 58 號設計書 §4.2.2 的定版，不要重排：後面的層只能依賴前面的層。
func New() *fx.App { return fx.New(modules()...) }

func modules() []fx.Option {
	return []fx.Option{
		config.Module,
		infrastructure.Module,
		postgres.Module,
		client.Module,
		invoicemq.Module,
		usecase.Module,
		worker.Module,
		invoicegrpc.Module,
	}
}
