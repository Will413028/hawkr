package main

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

// rfqOrderLane 是 CreateRfqOrder 的 lane：WISH_POOL 刪除後只剩平台的店（A9-D44），賣方就是被接受的店。
const rfqOrderLane = "PLATFORM"

// procurementOrders 把舊 order-service 的 CreateRfqOrder 轉成建單的 port；ordering 搬進單體時整段刪除。
type procurementOrders struct{ client rfqOrderCreator }

type rfqOrderCreator interface {
	CreateRfqOrder(ctx context.Context, cmd orderservice.RfqOrderCommand) (orderservice.RfqOrderResult, error)
}

var _ procurement.OrderCreator = procurementOrders{}

func (p procurementOrders) CreateOrder(ctx context.Context, cmd procurement.OrderCommand) (procurement.OrderResult, error) {
	res, err := p.client.CreateRfqOrder(ctx, orderservice.RfqOrderCommand{
		AcceptanceID: cmd.OrderRef, BuyerID: cmd.BuyerID, StoreID: cmd.StoreID, TotalAmount: cmd.TotalAmount,
		ShippingFee: cmd.ShippingFee, UnitPrice: cmd.UnitPrice, SkuName: cmd.ItemName, Lane: rfqOrderLane,
	})
	if err != nil {
		return procurement.OrderResult{}, err
	}
	return procurement.OrderResult{Code: res.Code, Message: res.Message, OrderNo: res.OrderNo, PaymentDeadline: res.PaymentDeadline}, nil
}
