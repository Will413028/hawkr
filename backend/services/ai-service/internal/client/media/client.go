package media

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"google.golang.org/grpc"
)

// API 是 AI 服務需要的 media 操作子集（縮圖 finalize + 引用釋放）。
// 以介面暴露方便 handler 層做單元測試（#1489）。
type API interface {
	FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (pkgmedia.AssetInfo, error)
	ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error
}

// Client 封裝 media service gRPC 客戶端。
type Client struct {
	client pb.MediaServiceClient
}

// NewClient 建立 media service gRPC 客戶端。
func NewClient(cfg *config.Config) (*Client, error) {
	target := fmt.Sprintf("%s:%s", cfg.MediaServiceHost, cfg.MediaServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
		// 內部身分 HMAC 簽章 + 轉發使用者身分（#1366，取代原 authPropagationInterceptor）。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("ai-service", true)),
	)
	if err != nil {
		return nil, fmt.Errorf("connect media-service at %s: %w", target, err)
	}
	return &Client{client: pb.NewMediaServiceClient(conn)}, nil
}

// FinalizeAndLink 將 staged token 轉換為正式資產並建立引用關係。
//
// 呼叫端 **fail-closed**（#2222）：grpc/handler.go 的 CreateAiStyle / UpdateAiStyle
// 收到 error 一律中止並回 Internal，不吞。吞掉的後果是 thumbnail_key 落庫但沒有
// asset_reference，7 天後 media GC 連同 R2 物件實體刪除。
// 對照：下方 ReleaseReferences 才是 best-effort，兩者不可混用。
func (c *Client) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (pkgmedia.AssetInfo, error) {
	return pkgmedia.FinalizeAndLink(ctx, c.client, stagedToken, refType, refID)
}

// ReleaseReferences 移除 (refType, refIDs) 的 asset_reference，keepAssetIDs 內的資產除外
// （#1489 / #2222）。keepAssetIDs 為空 = 釋放全部（實體或帳號刪除）；非空 = 換圖收斂，
// media-service 在單一交易內刪舊留新。呼叫端 best-effort，且必須排在業務列寫入之後。
func (c *Client) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	return pkgmedia.ReleaseReferences(ctx, c.client, refType, refIDs, keepAssetIDs)
}
