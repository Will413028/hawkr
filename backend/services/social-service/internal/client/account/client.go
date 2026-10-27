package account

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client        pb.AccountServiceClient
	delegationSvc pb.DelegationServiceClient
}

// NewClient returns the concrete type, not usecase.AccountClient, so fx can bind
// one instance to both AccountClient and usecase.DelegationAuditClient. Two
// providers would mean two grpc.ClientConn to the same downstream, and then the
// circuit breaker and connection pool each see half the traffic.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.Account.Addr(),
		// 內部身分 HMAC 簽章 + 轉發使用者身分（#1366，52 號設計書）。
		// 這一條是排程稽核的前提，不是可選的加固：account-service 只在
		// 「驗簽通過且 serviceName == social-service」時才接受
		// origin=scheduled（§7.6.3），沒有簽章那個判準永遠不成立，排程發布
		// 的稽核列會被降級成 interactive 或直接被拒。
		// 既有的 GetAccountInfo／GetAccountInfoBatch 也會開始帶簽章——這是
		// 預期且正確的方向，其餘 client（media）早已如此。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("social-service", true)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("account grpc.NewClient: %w", err)
	}
	return &Client{
		client:        pb.NewAccountServiceClient(conn),
		delegationSvc: pb.NewDelegationServiceClient(conn),
	}, nil
}

func (c *Client) GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*usecase.AccountInfo, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetAccountInfoBatch(ctx, &pb.GetAccountInfoBatchRequest{AccountIds: accountIDs})
	if err != nil || resp.GetCode() != 0 {
		// graceful fallback
		return make(map[int64]*usecase.AccountInfo), nil
	}
	result := make(map[int64]*usecase.AccountInfo, len(resp.GetData()))
	for _, d := range resp.GetData() {
		result[d.GetAccountId()] = &usecase.AccountInfo{
			Nickname: d.GetNickname(),
			Avatar:   d.GetAvatar(),
		}
	}
	return result, nil
}

func (c *Client) GetAccountInfo(ctx context.Context, accountID int64) (*usecase.AccountInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetAccountInfo(ctx, &pb.GetAccountInfoRequest{AccountId: accountID})
	if err != nil || resp.GetCode() != 0 || resp.GetData() == nil {
		// nil = 查不到（見 usecase.AccountInfo 的契約說明）。理由同 seller / buyer client：
		// 空物件與真實資料無法區分，會讓「以存在與否判斷」的 caller 系統性誤判。
		return nil, nil
	}
	data := resp.GetData()
	return &usecase.AccountInfo{
		Nickname: data.GetNickname(),
		Avatar:   data.GetAvatar(),
	}, nil
}

func (c *Client) SuspendAccount(ctx context.Context, accountID int64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	_, err := c.client.SuspendAccount(ctx, &pb.SuspendAccountRequest{AccountId: accountID, Reason: reason})
	return err
}
