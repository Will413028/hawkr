package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
)

type orderClient interface {
	MyOrders(ctx context.Context, buyerID int64, statusFilter string, page, pageSize int) ([]orderservice.BuyerOrder, error)
	BuyerOrderDetail(ctx context.Context, buyerID int64, orderNo string) (orderservice.BuyerOrderDetail, error)
}

type assistantOrders struct{ client orderClient }

var _ assistant.OrderReader = assistantOrders{}

func (a assistantOrders) MyOrders(ctx context.Context, buyerID int64, statusFilter string, page, pageSize int) ([]assistant.BuyerOrder, error) {
	rows, err := a.client.MyOrders(ctx, buyerID, statusFilter, page, pageSize)
	if err != nil {
		return nil, err
	}
	out := make([]assistant.BuyerOrder, 0, len(rows))
	for _, r := range rows {
		out = append(out, assistant.BuyerOrder{OrderNo: r.OrderNo, Status: r.Status, SellerName: r.SellerName, Amount: r.Amount, CreateTime: r.CreateTime, ItemCount: r.ItemCount})
	}
	return out, nil
}

func (a assistantOrders) OrderDetail(ctx context.Context, buyerID int64, orderNo string) (assistant.BuyerOrderDetail, error) {
	d, err := a.client.BuyerOrderDetail(ctx, buyerID, orderNo)
	if err != nil {
		if isOrderNotFound(err) {
			return assistant.BuyerOrderDetail{}, fmt.Errorf("%w: %w", assistant.ErrOrderNotFound, err)
		}
		return assistant.BuyerOrderDetail{}, err
	}
	return assistant.BuyerOrderDetail{
		OrderNo: d.OrderNo, Status: d.Status, SellerName: d.SellerName, ShippingMethod: d.ShippingMethod, ShippingStatus: d.ShippingStatus,
		RecipientName: d.RecipientName, PaymentMethod: d.PaymentMethod, Amount: d.Amount, CreateTime: d.CreateTime, ShippingFee: d.ShippingFee,
	}, nil
}

func isOrderNotFound(err error) bool { return errors.Is(err, orderservice.ErrOrderNotFound) }
