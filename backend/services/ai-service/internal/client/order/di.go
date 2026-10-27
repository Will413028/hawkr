package order

import (
	"github.com/Will413028/hawkr/ai-service/internal/procurement/orderprovisioning"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"go.uber.org/fx"
)

// Module 提供 order gRPC client，並把同一個連線綁成兩個**不同消費端**的窄介面：
// usecase.OrderClient 是 agent tool 的讀取面，orderprovisioning.RfqOrderCreator 是
// outbox worker 的建單面。⛔ 不要把兩者合成一個大介面——那會讓 agent tool 的 fake
// 必須實作建單、worker 的 fake 必須實作四支查詢。
var Module = fx.Module("order_client",
	fx.Provide(NewOrderClient),
	fx.Provide(func(c *Client) usecase.OrderClient { return c }),
	fx.Provide(func(c *Client) orderprovisioning.RfqOrderCreator { return c }),
)
