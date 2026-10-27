package buyer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.BuyerServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.BuyerService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to buyer-service at %s: %w", cfg.BuyerService.Addr, err)
	}
	return &Client{client: pb.NewBuyerServiceClient(conn)}, nil
}

func (c *Client) GetBuyerMetaBatchByBuyerIDs(ctx context.Context, buyerIDs []int64) (map[int64]*usecase.BuyerMeta, error) {
	if len(buyerIDs) == 0 {
		return map[int64]*usecase.BuyerMeta{}, nil
	}
	resp, err := c.client.BatchGetBuyerMetaByBuyerIDs(ctx, &pb.BatchGetBuyerMetaByBuyerIDsRequest{BuyerIds: buyerIDs})
	if err != nil {
		return nil, fmt.Errorf("batch get buyer meta by buyer ids: %w", err)
	}
	// buyer-service 的業務錯誤是「gRPC 層無 error + Code 非 0 + 空 payload」
	// （handler.go 對 pkgerrors 的處理），不檢查 Code 的話上游故障會被誤讀成
	// 「這些買家都查無資料」。同 repo 先例：client/order/client.go、
	// social-service/internal/client/buyer/client.go。
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("batch get buyer meta by buyer ids: server error code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}
	result := make(map[int64]*usecase.BuyerMeta, len(resp.Buyers))
	for buyerID, meta := range resp.Buyers {
		result[buyerID] = &usecase.BuyerMeta{
			Nickname:  meta.Nickname,
			AvatarURL: meta.AvatarKey, // 49 §6.6 S9：buyer 回 objectKey，透傳；Gateway 組 avatarUrl
		}
	}
	return result, nil
}

func (c *Client) GetProfileByBuyerID(ctx context.Context, buyerID int64) (*usecase.BuyerProfile, error) {
	resp, err := c.client.GetBuyerProfileByBuyerID(ctx, &pb.GetBuyerProfileByBuyerIDRequest{BuyerId: buyerID})
	if err != nil {
		return nil, fmt.Errorf("get buyer profile by buyer id: %w", err)
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get buyer profile by buyer id %d: server error code=%d msg=%s", buyerID, resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if d == nil {
		return nil, nil
	}
	accountID, err := strconv.ParseInt(d.GetAccountId(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("buyer %d: parse account_id %q: %w", buyerID, d.GetAccountId(), err)
	}
	return &usecase.BuyerProfile{
		AccountID: accountID,
		Nickname:  d.GetNickname(),
		Phone:     d.GetPhone(),
		AvatarURL: d.GetAvatarKey(),
	}, nil
}

func (c *Client) GetDefaultAddress(ctx context.Context, buyerID int64) (string, error) {
	resp, err := c.client.GetAddressList(ctx, &pb.GetAddressListRequest{BuyerId: buyerID})
	if err != nil {
		return "", fmt.Errorf("get default address: %w", err)
	}
	if resp.GetCode() != 0 {
		return "", fmt.Errorf("get default address for buyer %d: server error code=%d msg=%s", buyerID, resp.GetCode(), resp.GetMessage())
	}
	for _, addr := range resp.Data {
		if addr.IsDefault {
			return addr.City + addr.District + addr.DetailAddress, nil
		}
	}
	return "", nil
}
