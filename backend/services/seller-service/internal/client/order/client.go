package order

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.OrderServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.OrderService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order-service at %s: %w", cfg.OrderService.Addr, err)
	}
	return &Client{client: pb.NewOrderServiceClient(conn)}, nil
}

// GetActiveMerchantOrderCount returns the number of active (non-completed) orders for a merchant.
// TODO: implement via order-service gRPC once proto is available.
func (c *Client) GetActiveMerchantOrderCount(_ context.Context, _ int64) (int, error) {
	// Stub: unimplemented; returns 0 so deletion is never blocked by this check until proto lands.
	return 0, nil
}

func (c *Client) GetTopSellersByRevenue(ctx context.Context, days, limit int32) ([]usecase.SellerRevenue, error) {
	resp, err := c.client.GetTopSellersByRevenue(ctx, &pb.GetTopSellersByRevenueRequest{
		Days:  days,
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("GetTopSellersByRevenue gRPC failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("GetTopSellersByRevenue error: code=%d msg=%s", resp.Code, resp.Message)
	}

	result := make([]usecase.SellerRevenue, 0, len(resp.Sellers))
	for _, s := range resp.Sellers {
		storeID, err := strconv.ParseInt(s.StoreId, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid store_id %q: %w", s.StoreId, err)
		}
		result = append(result, usecase.SellerRevenue{
			StoreID:      storeID,
			TotalRevenue: s.TotalRevenue,
			OrderCount:   s.OrderCount,
		})
	}
	return result, nil
}
