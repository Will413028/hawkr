package couponconsumer

import (
	"context"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
)

// Run 訂閱 coupon 的 legacy 事件。使用單體自有的 queue 名稱（consumer 自己宣告，不在 topology.yaml，與 crmconsumer、buyerconsumer 相同）。
func (c Consumer) Run(ctx context.Context, url string, log *slog.Logger) {
	rabbitmq.Run(ctx, url, []rabbitmq.Binding{
		{Queue: "hawkr.monolith.coupon.order.completed", Exchange: "hawkr.order", Key: "order.completed", Handle: c.OrderCompleted},
	}, isPermanent, log)
}
