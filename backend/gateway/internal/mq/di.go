package mq

import (
	"context"
	"os"

	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module wires the gateway-side MQ consumer for the SSE realtime platform。
//
// §5.0 啟動策略：
//   - HTTP / gRPC server 同步啟動
//   - SSE Hub 同步啟動（純 in-memory）
//   - MQ consumer **async** 啟動（goroutine）；連線失敗指數退避重連；不阻塞
//
// Run 永不 return（除非 ctx cancel）— 任何一次連線失敗都在內部退避重試，
// HTTP API 仍可服務。
var Module = fx.Module("gateway-mq",
	fx.Provide(NewGatewayConsumer),
	fx.Invoke(startConsumer),
)

// NewGatewayConsumer reads RABBITMQ_URL and wires the realtime registry。
func NewGatewayConsumer(reg *realtime.Registry, log *zap.Logger) *Consumer {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	return NewConsumer(url, reg, log)
}

// startConsumer launches Run in a background goroutine on OnStart and cancels
// the context on OnStop。Per §5.0 we do NOT block app startup on MQ connectivity。
func startConsumer(lc fx.Lifecycle, c *Consumer, log *zap.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				log.Info("SSE.mq.consumer.spawning")
				c.Run(ctx)
				log.Info("SSE.mq.consumer.exited")
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}
