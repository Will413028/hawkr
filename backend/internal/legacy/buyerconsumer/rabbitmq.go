package buyerconsumer

import (
	"context"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
)

// Run 訂閱買家模組的 legacy 事件。
func (c Consumer) Run(ctx context.Context, url string, log *slog.Logger) {
	rabbitmq.Run(ctx, url, []rabbitmq.Binding{
		{Queue: "hawkr.monolith.buyer.order.completed", Exchange: "hawkr.order", Key: "order.completed", Handle: c.OrderCompleted},
		{Queue: "hawkr.monolith.buyer.product.restock", Exchange: "hawkr.product", Key: "product.restock", Handle: c.ProductRestock},
	}, isPermanent, log)
}
