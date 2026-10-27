package account

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/go-pkg/rolestate/rolestatepb"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
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

func (c *Client) GetEmail(ctx context.Context, accountID int64) (string, error) {
	resp, err := c.client.GetAccountInfo(ctx, &pb.GetAccountInfoRequest{AccountId: accountID})
	if err != nil {
		return "", fmt.Errorf("get account info failed: %w", err)
	}
	if resp.Data == nil {
		return "", fmt.Errorf("account %d not found", accountID)
	}
	return resp.Data.Email, nil
}

// GetAccountInfo retrieves full account information including email, phone, account_type.
func (c *Client) GetAccountInfo(ctx context.Context, accountID int64) (*usecase.AccountInfo, error) {
	resp, err := c.client.GetAccountInfo(ctx, &pb.GetAccountInfoRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("get account info failed: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("account %d not found", accountID)
	}
	return &usecase.AccountInfo{
		Email:       resp.Data.Email,
		Phone:       resp.Data.Phone,
		AccountType: resp.Data.AccountType,
	}, nil
}

// BindSellerRole calls account-service BindSeller, attributing the grant to the
// stint identified by storeID (issue #3189). Idempotent: account-service no-ops
// when the role is already held, but still re-points the existing grant at
// storeID — which is what stops a previous stint's pending deletion from
// revoking it.
func (c *Client) BindSellerRole(ctx context.Context, accountID, storeID int64) error {
	resp, err := c.client.BindSeller(ctx, &pb.BindSellerRequest{
		AccountId: accountID,
		SourceId:  storeID,
	})
	if err != nil {
		return fmt.Errorf("seller-service: BindSeller rpc: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("seller-service: BindSeller returned code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}

// RemoveSellerRole calls account-service RemoveRole with Role_ROLE_SELLER,
// passing along seller-service's judgment on whether the account's buyer
// role is still active. If this is the last active role on the account AND
// other==rolestate.OtherRoleAbsent, account-service performs full
// soft-deletion (Apple revoke + Redis blacklist + MQ).
func (c *Client) RemoveSellerRole(ctx context.Context, accountID int64, other rolestate.OtherRoleState, storeID int64) error {
	resp, err := c.client.RemoveRole(ctx, &pb.RemoveRoleRequest{
		AccountId:      accountID,
		Role:           pb.Role_ROLE_SELLER,
		OtherRoleState: rolestatepb.ToProto(other),
		SourceId:       storeID,
	})
	if err != nil {
		return fmt.Errorf("seller-service: RemoveRole rpc: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("seller-service: RemoveRole returned code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}
