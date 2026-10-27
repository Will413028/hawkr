package buyer

import (
	"context"
	"fmt"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (c *Client) GetSubscriberCount(ctx context.Context, storeID int64) (int64, error) {
	resp, err := c.client.GetSubscriberDashboardStats(ctx, &pb.GetSubscriberDashboardStatsRequest{
		StoreId: storeID,
	})
	if err != nil {
		return 0, fmt.Errorf("GetSubscriberDashboardStats gRPC failed: %w", err)
	}
	return resp.Total, nil
}

func (c *Client) BatchGetSubscriberCounts(ctx context.Context, sellerIDs []int64) (map[int64]int64, error) {
	resp, err := c.client.BatchGetSubscriberCounts(ctx, &pb.BatchGetSubscriberCountsRequest{
		StoreIds: sellerIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("BatchGetSubscriberCounts gRPC failed: %w", err)
	}
	return resp.Counts, nil
}

func (c *Client) HasActiveBuyer(ctx context.Context, accountID int64) (bool, error) {
	// 這條查詢坐在使用者可見的刪除路徑與 outbox worker 的 tick loop 上，
	// 對側卡住不能拖著整個 drain 一起停。
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout3s)
	defer cancel()

	resp, err := c.client.GetBuyerProfile(ctx, &pb.GetBuyerProfileRequest{AccountId: accountID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("seller-service: GetBuyerProfile rpc: %w", err)
	}
	if resp.Code != 0 {
		// buyer-service 的 GetBuyerProfile 直接透傳底層 DomainError code
		// （grpc/handler.go），buyer 不存在時就是 CodeNotFound。
		if resp.Code == int32(pkgerrors.CodeNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("seller-service: GetBuyerProfile returned code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.Data != nil, nil
}
