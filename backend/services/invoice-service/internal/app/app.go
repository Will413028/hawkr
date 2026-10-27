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

// modules 是分層清單的唯一來源，讓 New 與 app_test.go 的整圖驗證用同一份。
// 兩邊各寫一份的話，測試綠而開機炸的組合是可能的。
func modules() []fx.Option {
	return []fx.Option{
		config.Module,         // Layer 0:   Config / Logger / pool / TxManager / OTel / cipher
		infrastructure.Module, // Layer 1:   gRPC server（五件套內建）、內部 HTTP（health/metrics/pprof）、PDF 下載通道
		postgres.Module,       // Layer 2:   Repository
		client.Module,         // Layer 3:   AMEGO client ＋ peer gRPC client
		invoicemq.Module,      // Layer 3.5: MQ publisher / consumer
		usecase.Module,        // Layer 4:   業務邏輯
		worker.Module,         // Layer 4.5: outbox / recheck / track 排程
		invoicegrpc.Module,    // Layer 5:   gRPC handler 註冊
	}
}
