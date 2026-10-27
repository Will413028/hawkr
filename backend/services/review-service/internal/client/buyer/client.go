package buyer

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
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

func (c *Client) BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, buyerIDs []int64) (map[int64]*usecase.BuyerMeta, error) {
	resp, err := c.client.BatchGetBuyerDisplayByBuyerIDs(ctx, &pb.BatchGetBuyerDisplayByBuyerIDsRequest{
		BuyerIds: buyerIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("BatchGetBuyerDisplayByBuyerIDs: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("BatchGetBuyerDisplayByBuyerIDs: code=%d msg=%s", resp.Code, resp.Message)
	}

	result := make(map[int64]*usecase.BuyerMeta, len(resp.Buyers))
	for buyerID, d := range resp.Buyers {
		result[buyerID] = &usecase.BuyerMeta{
			Nickname:  d.DisplayName, // 已註銷者已是墓碑字串，可直接顯示
			AvatarURL: d.AvatarKey,   // 49 §6.6：buyer 回 objectKey；批 4 組 URL
			Deleted:   d.Deleted,
		}
	}
	return result, nil
}
