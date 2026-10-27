// Package crmservice 呼叫遷移期的 crm-service。
package crmservice

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	crmpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/crm"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/crm/crmconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

// Client 讀取買家在店家的 VIP 狀態。
type Client struct{ rpc crmconnect.CrmServiceClient }

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("crmservice：缺少位址")
	}
	return &Client{rpc: crmconnect.NewCrmServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// VIPStatuses 批次查詢買家在各店的會員狀態。
func (c *Client) VIPStatuses(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error) {
	if len(storeIDs) == 0 {
		return map[int64]bool{}, nil
	}
	ids := make([]string, len(storeIDs))
	for i, id := range storeIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rpc.GetBuyerVipStatuses(ctx, connect.NewRequest(&crmpb.GetBuyerVipStatusesRequest{BuyerId: strconv.FormatInt(buyerID, 10), StoreIds: ids}))
	if err != nil {
		return nil, fmt.Errorf("crmservice：GetBuyerVipStatuses：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("crmservice：GetBuyerVipStatuses code=%d", resp.Msg.GetCode())
	}
	out := make(map[int64]bool, len(resp.Msg.GetData()))
	for _, value := range resp.Msg.GetData() {
		id, err := strconv.ParseInt(value.GetStoreId(), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("crmservice：店家 ID 不合法")
		}
		out[id] = value.GetCustomerType() == "vip"
	}
	return out, nil
}
