package review

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.ReviewServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.ReviewService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to review-service at %s: %w", cfg.ReviewService.Addr, err)
	}
	return &Client{client: pb.NewReviewServiceClient(conn)}, nil
}

func (c *Client) GetSellerReviewStats(ctx context.Context, storeID int64) (*usecase.ReviewStats, error) {
	resp, err := c.client.GetSellerOrderReviewStats(ctx, &pb.GetSellerOrderReviewStatsRequest{
		StoreId: strconv.FormatInt(storeID, 10),
	})
	if err != nil {
		return nil, fmt.Errorf("GetSellerOrderReviewStats gRPC failed: %w", err)
	}

	totalCount, _ := strconv.ParseInt(resp.TotalCount, 10, 32)
	avgRating, _ := strconv.ParseFloat(resp.AvgRating, 64)

	return &usecase.ReviewStats{
		TotalCount: int32(totalCount),
		AvgRating:  avgRating,
	}, nil
}
