package social

import (
	"context"
	"fmt"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.SocialServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.SocialService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to social-service at %s: %w", cfg.SocialService.Addr, err)
	}
	return &Client{client: pb.NewSocialServiceClient(conn)}, nil
}

func (c *Client) HasPublishedPost(ctx context.Context, storeID int64) (bool, error) {
	resp, err := c.client.GetPostList(ctx, &pb.GetPostListRequest{
		SellerId: strconv.FormatInt(storeID, 10),
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		return false, pkgerrors.Wrap(pkgerrors.CodeUnavailable, "服務暫時無法取得，請稍後再試", err)
	}
	if resp.Code != 0 {
		return false, pkgerrors.NewUnavailable(resp.Message)
	}
	return resp.Total > 0, nil
}
