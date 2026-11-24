package orderservice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
)

// BuyerOrder 是買家訂單列表的一列。
type BuyerOrder struct {
	OrderNo    string
	Status     string
	SellerName string
	Amount     int64
	CreateTime time.Time
	ItemCount  int64
}

// BuyerOrderDetail 是買家看到的單筆訂單摘要。
type BuyerOrderDetail struct {
	OrderNo        string
	Status         string
	SellerName     string
	ShippingMethod string
	RecipientName  string
	PaymentMethod  string
	Amount         int64
	CreateTime     time.Time
	ShippingFee    int64
}

// RfqOrderCommand 是 RFQ 成交建單命令。AcceptanceID 是 order-service 的冪等鍵；明細是 non-catalog line，不帶 SKU ID
// 與配送溫層。
type RfqOrderCommand struct {
	AcceptanceID int64
	BuyerID      int64
	// StoreID 是訂單賣方的 store。
	StoreID int64
	// TotalAmount 是成交總額，等於 UnitPrice 加 ShippingFee。
	TotalAmount int64
	ShippingFee int64
	UnitPrice   int64
	// SkuName 是含數量與單位的真實成交品名。
	SkuName string
	Lane    string
}

// RfqOrderResult 是 order-service 的建單回應。Code 原樣帶回，由呼叫端分類終態拒絕與可重試；Code 為 0 時其餘欄位才有意義。
type RfqOrderResult struct {
	Code            int32
	Message         string
	OrderNo         string
	PaymentDeadline time.Time
	Replayed        bool
}

func accountHeader[T any](req *connect.Request[T], buyerID int64) *connect.Request[T] {
	req.Header().Set("x-account-id", strconv.FormatInt(buyerID, 10))
	return req
}

// orderTime 解析 order-service 的時間字串：ISO 8601（RFC 3339，含或不含小數秒），舊 adapter 也接受的 Unix 毫秒當後備。
func orderTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	ms, err := nonnegativeAmount(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("orderservice：時間不合法: %q", raw)
	}
	return time.UnixMilli(ms).UTC(), nil
}

// MyOrders 查詢買家的訂單；statusFilter 為空表示全部，多值以逗號分隔。buyerID 是目前的 buyer_id，不是登入帳號 ID。
func (c *Client) MyOrders(ctx context.Context, buyerID int64, statusFilter string, page, pageSize int) ([]BuyerOrder, error) {
	if buyerID <= 0 || page < 1 || pageSize < 1 {
		return nil, fmt.Errorf("orderservice：MyOrders 參數不合法")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req := accountHeader(connect.NewRequest(&orderpb.GetMyOrdersRequest{
		BuyerId:      buyerID,
		StatusFilter: statusFilter,
		Page:         &orderpb.PageRequest{Page: strconv.Itoa(page), Size: strconv.Itoa(pageSize)},
	}), buyerID)
	resp, err := c.rpc.GetMyOrders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("orderservice：GetMyOrders：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("orderservice：GetMyOrders code=%d", resp.Msg.GetCode())
	}
	out := make([]BuyerOrder, 0, len(resp.Msg.GetOrders()))
	for _, o := range resp.Msg.GetOrders() {
		if o.GetOrderNo() == "" {
			return nil, fmt.Errorf("orderservice：訂單號缺失")
		}
		amount, err := nonnegativeAmount(o.GetTotalActualAmount())
		if err != nil {
			return nil, err
		}
		created, err := orderTime(o.GetCreateTime())
		if err != nil {
			return nil, err
		}
		count, err := nonnegativeAmount(o.GetItemCount())
		if err != nil {
			return nil, err
		}
		out = append(out, BuyerOrder{OrderNo: o.GetOrderNo(), Status: o.GetStatus(), SellerName: o.GetSellerName(), Amount: amount, CreateTime: created, ItemCount: count})
	}
	return out, nil
}

// BuyerOrderDetail 查詢買家的單筆訂單；訂單不存在或不屬於該買家時回 ErrOrderNotFound。
func (c *Client) BuyerOrderDetail(ctx context.Context, buyerID int64, orderNo string) (BuyerOrderDetail, error) {
	if buyerID <= 0 || strings.TrimSpace(orderNo) == "" {
		return BuyerOrderDetail{}, fmt.Errorf("orderservice：BuyerOrderDetail 參數不合法")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetOrder(ctx, accountHeader(connect.NewRequest(&orderpb.GetOrderRequest{BuyerId: buyerID, OrderNo: orderNo}), buyerID))
	if err != nil {
		return BuyerOrderDetail{}, fmt.Errorf("orderservice：GetOrder：%w", err)
	}
	if resp.Msg.GetCode() == grpcNotFound {
		return BuyerOrderDetail{}, ErrOrderNotFound
	}
	if resp.Msg.GetCode() != 0 {
		return BuyerOrderDetail{}, fmt.Errorf("orderservice：GetOrder code=%d", resp.Msg.GetCode())
	}
	d := resp.Msg.GetData()
	if d == nil {
		return BuyerOrderDetail{}, ErrOrderNotFound
	}
	amount, err := nonnegativeAmount(d.GetTotalActualAmount())
	if err != nil {
		return BuyerOrderDetail{}, err
	}
	fee, err := nonnegativeAmount(d.GetShippingFee())
	if err != nil {
		return BuyerOrderDetail{}, err
	}
	created, err := orderTime(d.GetCreateTime())
	if err != nil {
		return BuyerOrderDetail{}, err
	}
	return BuyerOrderDetail{
		OrderNo: d.GetOrderNo(), Status: d.GetStatus(), SellerName: d.GetSellerName(),
		ShippingMethod: d.GetShippingMethod(),
		RecipientName:  d.GetRecipientName(), PaymentMethod: d.GetPaymentMethod(),
		Amount: amount, CreateTime: created, ShippingFee: fee,
	}, nil
}

// CreateRfqOrder 投遞 RFQ 成交建單命令，冪等鍵是 AcceptanceID。回應的非 0 code 不在這一層變成 error：
// 「冪等命中」與「被拒絕」要由呼叫端分辨。
func (c *Client) CreateRfqOrder(ctx context.Context, cmd RfqOrderCommand) (RfqOrderResult, error) {
	if cmd.AcceptanceID <= 0 || cmd.BuyerID <= 0 || cmd.StoreID <= 0 || cmd.TotalAmount < 0 || cmd.ShippingFee < 0 || cmd.UnitPrice < 0 || strings.TrimSpace(cmd.SkuName) == "" || cmd.Lane == "" {
		return RfqOrderResult{}, fmt.Errorf("orderservice：CreateRfqOrder 命令不完整")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.CreateRfqOrder(ctx, connect.NewRequest(&orderpb.CreateRfqOrderRequest{
		AcceptanceId: strconv.FormatInt(cmd.AcceptanceID, 10),
		BuyerId:      cmd.BuyerID,
		StoreId:      strconv.FormatInt(cmd.StoreID, 10),
		TotalAmount:  strconv.FormatInt(cmd.TotalAmount, 10),
		ShippingFee:  strconv.FormatInt(cmd.ShippingFee, 10),
		SkuName:      cmd.SkuName,
		UnitPrice:    strconv.FormatInt(cmd.UnitPrice, 10),
		Lane:         cmd.Lane,
	}))
	if err != nil {
		return RfqOrderResult{}, fmt.Errorf("orderservice：CreateRfqOrder：%w", err)
	}
	out := RfqOrderResult{Code: resp.Msg.GetCode(), Message: resp.Msg.GetMessage()}
	if out.Code != 0 {
		return out, nil
	}
	data := resp.Msg.GetData()
	if data.GetOrderNo() == "" {
		return RfqOrderResult{}, fmt.Errorf("orderservice：CreateRfqOrder 成功但沒有訂單號")
	}
	out.OrderNo, out.Replayed = data.GetOrderNo(), data.GetReplayed()
	if deadline := data.GetPaymentDeadline(); deadline != "" {
		parsed, err := time.Parse(time.RFC3339, deadline)
		if err != nil {
			return RfqOrderResult{}, fmt.Errorf("orderservice：付款期限不合法 %q：%w", deadline, err)
		}
		out.PaymentDeadline = parsed
	}
	return out, nil
}
