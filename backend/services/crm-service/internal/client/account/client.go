package account

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.AccountServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.AccountService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to account-service at %s: %w", cfg.AccountService.Addr, err)
	}
	return &Client{client: pb.NewAccountServiceClient(conn)}, nil
}

func (c *Client) GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*usecase.CustomerAccountInfo, error) {
	if len(accountIDs) == 0 {
		return map[int64]*usecase.CustomerAccountInfo{}, nil
	}
	resp, err := c.client.GetAccountInfoBatch(ctx, &pb.GetAccountInfoBatchRequest{AccountIds: accountIDs})
	if err != nil {
		return nil, fmt.Errorf("get account info batch: %w", err)
	}
	result := make(map[int64]*usecase.CustomerAccountInfo, len(resp.Data))
	for _, info := range resp.Data {
		result[info.AccountId] = &usecase.CustomerAccountInfo{
			Nickname: info.Nickname,
			Avatar:   info.Avatar,
			Phone:    info.Phone,
			Email:    info.Email,
		}
	}
	return result, nil
}

func (c *Client) GetAccountInfo(ctx context.Context, accountID int64) (*usecase.CustomerAccountInfo, error) {
	resp, err := c.client.GetAccountInfo(ctx, &pb.GetAccountInfoRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("get account info: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("account %d not found", accountID)
	}
	return &usecase.CustomerAccountInfo{
		Nickname: resp.Data.Nickname,
		Avatar:   resp.Data.Avatar,
		Phone:    resp.Data.Phone,
		Email:    resp.Data.Email,
	}, nil
}
