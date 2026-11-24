package main

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/paymentrelay"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// paymentOrdersClient 是 paymentOrders 用到的 orderservice.Client 的方法。
type paymentOrdersClient interface {
	OrderForPayment(ctx context.Context, orderNo string) (orderservice.PaymentOrder, error)
}

// paymentOrders 以還在舊服務的 order-service（GetOrderForPayment）實作 payment 的訂單 port（B2 步驟 4）：訂單不存在回 NotFound，其他錯誤回 Unavailable
// （訂單一方暫時問不到，不是訂單不存在）。ordering 搬進單體（B8）後換成 in-process 呼叫。
type paymentOrders struct{ client paymentOrdersClient }

func (o paymentOrders) OrderForPayment(ctx context.Context, orderNo string) (payment.PaymentOrder, error) {
	order, err := o.client.OrderForPayment(ctx, orderNo)
	if errors.Is(err, orderservice.ErrOrderNotFound) {
		return payment.PaymentOrder{}, apperr.NotFound("找不到這張訂單")
	}
	if err != nil {
		return payment.PaymentOrder{}, apperr.Unavailable("訂單服務暫時無法使用：%v", err)
	}
	return payment.PaymentOrder{
		OrderNo: order.OrderNo, BuyerID: order.BuyerID, StoreID: order.StoreID, Amount: order.Amount, Payable: order.Payable, Description: order.ProductName,
	}, nil
}

// subscribePaymentRelay 把 payment 的付款與退款事件轉給還在 RabbitMQ 上的消費者（Java order、aftersales）：每個事件在發布的 transaction 內排入一個送出的 job。
// serve 與 worker 兩個角色都要訂閱：建立付款的請求內（serve）與同步、退款 job（worker）都會發布事件。不轉送的事件與原因見 paymentrelay 的套件說明。
// B8、B9 搬進單體後刪除。
func subscribePaymentRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, paymentrelay.OnPaymentChanged(queue))
	events.Subscribe(bus, paymentrelay.OnRefundChanged(queue))
}
