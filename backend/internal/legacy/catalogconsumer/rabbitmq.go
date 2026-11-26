package catalogconsumer

import (
	"context"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
)

// Run 訂閱 catalog 的 legacy 事件。使用單體自有的 queue 名稱（consumer 自己宣告，不在 topology.yaml，與 couponconsumer 相同）。
func (c Consumer) Run(ctx context.Context, url string, log *slog.Logger) {
	rabbitmq.Run(ctx, url, []rabbitmq.Binding{
		{Queue: "hawkr.monolith.catalog.order.cancelled", Exchange: "hawkr.order", Key: "order.cancelled", Handle: c.OrderCancelled},
		{Queue: "hawkr.monolith.catalog.order.sales.accumulated", Exchange: "hawkr.order", Key: "order.sales.accumulated", Handle: c.OrderSalesAccumulated},
	}, isPermanent, log)
}
