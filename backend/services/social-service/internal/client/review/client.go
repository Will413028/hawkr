package review

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"google.golang.org/grpc"
)

// Client is the social-service-internal gRPC client for review-service.
// Used by AdminResolveReport HIDE_TARGET dispatch (B-135).
type Client struct {
	conn   *grpc.ClientConn
	client pb.ReviewServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.Review.Addr())
	if err != nil {
		return nil, fmt.Errorf("review grpc.NewClient: %w", err)
	}
	return &Client{conn: conn, client: pb.NewReviewServiceClient(conn)}, nil
}

func (c *Client) HideReview(ctx context.Context, reviewID int64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	resp, err := c.client.HideReview(ctx, &pb.HideReviewRequest{ReviewId: reviewID, HiddenReason: reason})
	if err != nil {
		return fmt.Errorf("review.HideReview rpc: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("review.HideReview code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}
