package orderservice

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
)

// OrderParties 是訂單的買家、店家與狀態的已解析投影。只取這四個欄位：GetOrderForAfterSales 的回應還帶買家姓名、電話、地址，
// 不帶出這個轉接層（最小化）。
type OrderParties struct {
	OrderNo string
	BuyerID int64
	StoreID int64
	Status  string
}

// OrderParties 回傳訂單的買家、店家與狀態（呼叫 GetOrderForAfterSales）。訂單不存在（回應內 code 404）回 ErrOrderNotFound；
// 傳輸錯誤、其他 code、ID 不是正整數、單號不一致與狀態空白都回一般錯誤（fail-closed，不推定為不存在）。
func (c *Client) OrderParties(ctx context.Context, orderNo string) (OrderParties, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetOrderForAfterSales(ctx, connect.NewRequest(&orderpb.GetOrderForAfterSalesRequest{OrderNo: orderNo}))
	if err != nil {
		return OrderParties{}, fmt.Errorf("orderservice：GetOrderForAfterSales：%w", err)
	}
	switch code := resp.Msg.GetCode(); {
	case code == grpcNotFound:
		return OrderParties{}, ErrOrderNotFound
	case code != 0:
		return OrderParties{}, fmt.Errorf("orderservice：GetOrderForAfterSales code=%d", code)
	}
	data := resp.Msg.GetData()
	if data == nil {
		return OrderParties{}, fmt.Errorf("orderservice：GetOrderForAfterSales 缺少 data")
	}
	buyerID, err := positiveID(data.GetBuyerId())
	if err != nil {
		return OrderParties{}, fmt.Errorf("orderservice：買家 ID 不合法: %q", data.GetBuyerId())
	}
	storeID, err := positiveID(data.GetStoreId())
	if err != nil {
		return OrderParties{}, err
	}
	if data.GetOrderNo() != orderNo {
		return OrderParties{}, fmt.Errorf("orderservice：回應的訂單號 %q 不是請求的 %q", data.GetOrderNo(), orderNo)
	}
	if data.GetStatus() == "" {
		return OrderParties{}, fmt.Errorf("orderservice：訂單狀態缺失")
	}
	return OrderParties{OrderNo: orderNo, BuyerID: buyerID, StoreID: storeID, Status: data.GetStatus()}, nil
}

// HasOrderBetween 回報買家與店家之間是否有狀態不是 0（待付款）與 80（已取消）的訂單，以及最近一筆的單號（呼叫 HasOrderBetween）。
// 已完成與已退款的訂單也算有；只回最近一筆。傳輸錯誤、非 0 的 code、has 為真但單號是空的都回一般錯誤。
func (c *Client) HasOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, string, error) {
	if buyerID <= 0 || storeID <= 0 {
		return false, "", fmt.Errorf("orderservice：買家 ID %d 或店家 ID %d 不合法", buyerID, storeID)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.HasOrderBetween(ctx, connect.NewRequest(&orderpb.HasOrderBetweenRequest{BuyerId: buyerID, StoreId: storeID}))
	if err != nil {
		return false, "", fmt.Errorf("orderservice：HasOrderBetween：%w", err)
	}
	if code := resp.Msg.GetCode(); code != 0 {
		return false, "", fmt.Errorf("orderservice：HasOrderBetween code=%d", code)
	}
	if !resp.Msg.GetHasOrder() {
		return false, "", nil
	}
	if resp.Msg.GetLatestOrderNo() == "" {
		return false, "", fmt.Errorf("orderservice：HasOrderBetween 有訂單但單號是空的")
	}
	return true, resp.Msg.GetLatestOrderNo(), nil
}

var openOrderStatuses = map[string]bool{"10": true, "20": true, "30": true, "32": true, "35": true, "40": true}

// OpenOrderBetween 回報買家與店家之間最近一筆有關聯的訂單是否進行中（狀態 10、20、30、32、35、40）。
// 已知看不到「最近一筆已結案、較舊的一筆還在配送」，錯的方向是多擋；任何查詢錯誤（含訂單查無、買家或店家與訂單不符）都回錯誤，
// 呼叫端一律擋（fail-closed）。
func (c *Client) OpenOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, error) {
	has, orderNo, err := c.HasOrderBetween(ctx, buyerID, storeID)
	if err != nil || !has {
		return false, err
	}
	p, err := c.OrderParties(ctx, orderNo)
	if err != nil {
		return false, err
	}
	if p.BuyerID != buyerID || p.StoreID != storeID {
		return false, fmt.Errorf("orderservice：訂單 %s 的買家 %d、店家 %d 不是查詢的 %d、%d", orderNo, p.BuyerID, p.StoreID, buyerID, storeID)
	}
	return openOrderStatuses[p.Status], nil
}
