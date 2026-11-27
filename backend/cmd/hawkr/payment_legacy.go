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

type paymentOrdersClient interface {
	OrderForPayment(ctx context.Context, orderNo string) (orderservice.PaymentOrder, error)
}

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

func subscribePaymentRelay(bus *events.Bus, queue jobs.Enqueuer) {
	events.Subscribe(bus, paymentrelay.OnPaymentChanged(queue))
	events.Subscribe(bus, paymentrelay.OnRefundChanged(queue))
}
