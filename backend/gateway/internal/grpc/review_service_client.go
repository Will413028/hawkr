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

// ReviewServiceClient is the gRPC client for review service
type ReviewServiceClient struct {
	conn   *grpc.ClientConn
	client pb.ReviewServiceClient
}

func NewReviewServiceClient() (*ReviewServiceClient, error) {
	host := getEnvOrDefault("REVIEW_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("REVIEW_SERVICE_GRPC_PORT", "10033")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to review service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to review gRPC service at %s", target)

	return &ReviewServiceClient{
		conn:   conn,
		client: pb.NewReviewServiceClient(conn),
	}, nil
}

func (c *ReviewServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *ReviewServiceClient) GetProductReviews(ctx context.Context, productID string, viewerID int64, rating int32, withImages bool, sortBy string, page, pageSize int32) (*pb.ProductReviewsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetProductReviews(ctx, &pb.GetProductReviewsRequest{
		ProductId:  productID,
		ViewerId:   viewerID,
		Rating:     rating,
		WithImages: withImages,
		SortBy:     sortBy,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (c *ReviewServiceClient) GetReview(ctx context.Context, reviewID string) (*pb.ReviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetReview(ctx, &pb.GetReviewRequest{ReviewId: reviewID})
}
