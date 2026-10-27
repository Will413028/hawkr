package buyer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.BuyerServiceClient
}

func NewClient(cfg *config.Config) (usecase.BuyerClient, error) {
	conn, err := grpcclient.Dial(cfg.BuyerServiceGRPCAddr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("buyer grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewBuyerServiceClient(conn)}, nil
}

// ListIDsByAccount returns every buyer_id ever assigned to accountID
// (including closed stints), used solely by the chat-notification PII-purge
// path (buyer.deleted/seller.deleted/account.deleted). Mirrors
// im-service/internal/client/buyer.Client.ListIDsByAccount exactly (same
// RPC, same string-to-int64 parsing convention).
func (c *Client) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ListBuyerIDsByAccount(ctx, &pb.ListBuyerIDsByAccountRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("list buyer ids by account: %w", err)
	}
	ids := make([]int64, 0, len(resp.GetBuyerIds()))
	for _, s := range resp.GetBuyerIds() {
		id, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil {
			continue // defensive: buyer-service's TSID-as-string field should always parse
		}
		ids = append(ids, id)
	}
	return ids, nil
}
