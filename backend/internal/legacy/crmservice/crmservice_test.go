package crmservice

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	crmpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/crm"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/crm/crmconnect"
)

type fakeCRM struct {
	crmconnect.CrmServiceClient
	response *crmpb.GetBuyerVipStatusesResponse
}

func (f fakeCRM) GetBuyerVipStatuses(ctx context.Context, req *connect.Request[crmpb.GetBuyerVipStatusesRequest]) (*connect.Response[crmpb.GetBuyerVipStatusesResponse], error) {
	if _, ok := ctx.Deadline(); !ok || req.Msg.GetBuyerId() != "4" || len(req.Msg.GetStoreIds()) != 2 {
		return nil, errors.New("bad request")
	}
	return connect.NewResponse(f.response), nil
}

func TestVIPStatusesBatch(t *testing.T) {
	c := &Client{rpc: fakeCRM{response: &crmpb.GetBuyerVipStatusesResponse{Data: []*crmpb.BuyerVipStatus{{StoreId: "7", CustomerType: "vip"}, {StoreId: "8", CustomerType: "normal"}}}}}
	status, err := c.VIPStatuses(context.Background(), 4, []int64{7, 8})
	if err != nil || !status[7] || status[8] {
		t.Fatalf("status=%v err=%v", status, err)
	}
}
