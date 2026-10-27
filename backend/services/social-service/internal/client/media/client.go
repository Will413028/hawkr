package media

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.MediaServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.Media.Addr(),
		// 內部身分 HMAC 簽章 + 轉發使用者身分（#1366，取代原 authPropagationInterceptor）。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("social-service", true)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("media grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewMediaServiceClient(conn)}, nil
}

// FinalizeAndLink 將暫存圖片轉正並建立引用關係，防止 GC 清除。
func (c *Client) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (usecase.AssetInfo, error) {
	return pkgmedia.FinalizeAndLink(ctx, c.client, stagedToken, refType, refID)
}

// ReleaseReferences 移除 (refType, refIDs) 的 asset_reference，keepAssetIDs 內的資產除外
// （#1489 / #2222）。keepAssetIDs 為空 = 釋放全部（實體或帳號刪除）；非空 = 換圖收斂，
// media-service 在單一交易內刪舊留新。呼叫端 best-effort，且必須排在業務列寫入之後。
func (c *Client) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	return pkgmedia.ReleaseReferences(ctx, c.client, refType, refIDs, keepAssetIDs)
}
