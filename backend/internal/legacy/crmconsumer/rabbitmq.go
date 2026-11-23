package crmconsumer

import (
	"context"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
)

// Run 訂閱 crm 的 legacy 事件。使用單體自有的 queue 名稱。
func (c Consumer) Run(ctx context.Context, url string, log *slog.Logger) {
	rabbitmq.Run(ctx, url, []rabbitmq.Binding{
		{Queue: "hawkr.monolith.crm.order.completed", Exchange: "hawkr.order", Key: "order.completed", Handle: c.OrderCompleted},
		{Queue: "hawkr.monolith.crm.refund.completed", Exchange: "aftersales.events", Key: "aftersales.refund.completed", Handle: c.RefundCompleted},
	}, isPermanent, log)
}
