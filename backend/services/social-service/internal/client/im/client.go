package im

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"google.golang.org/grpc"
)

// Client is the social-service-internal gRPC client for im-service.
// Used by AdminResolveReport HIDE_TARGET dispatch (B-135).
type Client struct {
	conn   *grpc.ClientConn
	client pb.IMServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.IM.Addr())
	if err != nil {
		return nil, fmt.Errorf("im grpc.NewClient: %w", err)
	}
	return &Client{conn: conn, client: pb.NewIMServiceClient(conn)}, nil
}

func (c *Client) HideMessage(ctx context.Context, messageID int64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	resp, err := c.client.HideMessage(ctx, &pb.HideMessageRequest{MessageId: messageID, HiddenReason: reason})
	if err != nil {
		return fmt.Errorf("im.HideMessage rpc: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("im.HideMessage code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}
