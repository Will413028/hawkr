package orderservice

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
)

const statusPendingPayment = "0"

const maxPaymentOrderNoRunes = 64

// PaymentOrder 是 GetOrderForPayment 的已解析投影：付款依它寫成快照。
type PaymentOrder struct {
	OrderNo     string
	BuyerID     int64
	StoreID     int64
	Amount      int64
	Payable     bool
	ProductName string
}

// OrderForPayment 讀訂單的應付資訊。訂單不存在回 ErrOrderNotFound；傳輸錯誤、其他 code 與任何不合法的欄位都回一般錯誤（fail-closed，不推定為不存在）。
// Payable 只有待付款狀態為真；不認得的狀態值不是有效值，回錯誤而不是推定為不可付。
func (c *Client) OrderForPayment(ctx context.Context, orderNo string) (PaymentOrder, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetOrderForPayment(ctx, connect.NewRequest(&orderpb.GetOrderForPaymentRequest{OrderNo: orderNo}))
	if err != nil {
		return PaymentOrder{}, fmt.Errorf("orderservice：GetOrderForPayment：%w", err)
	}
	switch code := resp.Msg.GetCode(); {
	case code == grpcNotFound:
		return PaymentOrder{}, ErrOrderNotFound
	case code != 0:
		return PaymentOrder{}, fmt.Errorf("orderservice：GetOrderForPayment code=%d", code)
	}
	data := resp.Msg.GetData()
	if data == nil {
		return PaymentOrder{}, fmt.Errorf("orderservice：GetOrderForPayment 缺少 data")
	}
	if data.GetOrderNo() != orderNo {
		return PaymentOrder{}, fmt.Errorf("orderservice：回應的訂單號 %q 不是請求的 %q", data.GetOrderNo(), orderNo)
	}
	if n := utf8.RuneCountInString(orderNo); n < 1 || n > maxPaymentOrderNoRunes {
		return PaymentOrder{}, fmt.Errorf("orderservice：訂單號長度不合法")
	}
	buyerID, err := positiveID(data.GetBuyerId())
	if err != nil {
		return PaymentOrder{}, fmt.Errorf("orderservice：買家 ID 不合法: %q", data.GetBuyerId())
	}
	storeID, err := positiveID(data.GetStoreId())
	if err != nil {
		return PaymentOrder{}, err
	}
	amount, err := nonnegativeAmount(data.GetAmount())
	if err != nil {
		return PaymentOrder{}, err
	}
	status := data.GetStatus()
	if _, known := openStatuses[status]; !known {
		return PaymentOrder{}, fmt.Errorf("orderservice：未知訂單狀態 %q", status)
	}
	return PaymentOrder{
		OrderNo: orderNo, BuyerID: buyerID, StoreID: storeID, Amount: amount,
		Payable: status == statusPendingPayment, ProductName: strings.TrimSpace(data.GetProductName()),
	}, nil
}
