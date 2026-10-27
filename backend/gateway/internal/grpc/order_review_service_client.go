package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// OrderReviewServiceClient 連到 review-service（訂單商品評論）
type OrderReviewServiceClient struct {
	conn   *grpc.ClientConn
	client pb.ReviewServiceClient
}

func NewOrderReviewServiceClient() (*OrderReviewServiceClient, error) {
	host := getEnvOrDefault("REVIEW_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("REVIEW_SERVICE_GRPC_PORT", "10033")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to review-service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to review-service gRPC at %s", target)

	return &OrderReviewServiceClient{
		conn:   conn,
		client: pb.NewReviewServiceClient(conn),
	}, nil
}

func (c *OrderReviewServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *OrderReviewServiceClient) SubmitOrderReview(ctx context.Context, req *pb.SubmitOrderReviewRequest) (*pb.SubmitOrderReviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SubmitOrderReview(ctx, req)
}

func (c *OrderReviewServiceClient) ListMyOrderReviews(ctx context.Context, req *pb.ListMyOrderReviewsRequest) (*pb.ListMyOrderReviewsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListMyOrderReviews(ctx, req)
}

func (c *OrderReviewServiceClient) GetSellerOrderReviews(ctx context.Context, req *pb.GetSellerOrderReviewsRequest) (*pb.GetSellerOrderReviewsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerOrderReviews(ctx, req)
}

func (c *OrderReviewServiceClient) GetSellerOrderReviewStats(ctx context.Context, req *pb.GetSellerOrderReviewStatsRequest) (*pb.GetSellerOrderReviewStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerOrderReviewStats(ctx, req)
}

// ReplyReview 賣家回覆評論（#1174）。
func (c *OrderReviewServiceClient) ReplyReview(ctx context.Context, req *pb.ReplyReviewRequest) (*pb.ReviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ReplyReview(ctx, req)
}
