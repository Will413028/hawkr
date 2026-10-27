package media

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"google.golang.org/grpc"
)

// Client wraps the Media Service gRPC client.
type Client struct {
	client pb.MediaServiceClient
}

// NewClient creates a new Media Service gRPC client.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.MediaServiceAddr,
		// 內部身分 HMAC 簽章 + 轉發使用者身分（#1366，取代原 authPropagationInterceptor）。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("im-service", true)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect media-service at %s: %w", cfg.MediaServiceAddr, err)
	}
	return &Client{client: pb.NewMediaServiceClient(conn)}, nil
}

// FinalizeAndLink 將暫存圖片轉正並建立引用關係，防止 GC 清除。
func (c *Client) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (usecase.AssetInfo, error) {
	return pkgmedia.FinalizeAndLink(ctx, c.client, stagedToken, refType, refID)
}

// ReleaseChatReferences releases chat-image asset_reference rows by message ID, in
// batches. Fail-closed: any RPC error or non-zero code is returned so the
// account.deleted consumer retries.
func (c *Client) ReleaseChatReferences(ctx context.Context, refIDs []string) error {
	const batchSize = 500
	for start := 0; start < len(refIDs); start += batchSize {
		end := start + batchSize
		if end > len(refIDs) {
			end = len(refIDs)
		}
		cctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout10s)
		resp, err := c.client.ReleaseReferences(cctx, &pb.ReleaseReferencesRequest{
			RefType: pkgmedia.RefTypeChatMessage,
			RefIds:  refIDs[start:end],
		})
		cancel()
		if err != nil {
			return fmt.Errorf("release chat references: %w", err)
		}
		if resp.GetCode() != 0 {
			return fmt.Errorf("release chat references failed (code=%d): %s", resp.GetCode(), resp.GetMessage())
		}
	}
	return nil
}
