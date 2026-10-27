package media

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// authPropagationInterceptor forwards x-account-id and x-account-type from
// incoming gRPC metadata (set by gateway) to outgoing gRPC metadata,
// so downstream services (e.g. media-service) can authenticate the caller.
func authPropagationInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			pairs := make([]string, 0, 4)
			if vals := md.Get("x-account-id"); len(vals) > 0 {
				pairs = append(pairs, "x-account-id", vals[0])
			}
			if vals := md.Get("x-account-type"); len(vals) > 0 {
				pairs = append(pairs, "x-account-type", vals[0])
			}
			if len(pairs) > 0 {
				outMD := metadata.Pairs(pairs...)
				if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
					outMD = metadata.Join(existingMD, outMD)
				}
				ctx = metadata.NewOutgoingContext(ctx, outMD)
			}
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// Client wraps the Media Service gRPC client for FinalizeImages and LinkImages.
type Client struct {
	client pb.MediaServiceClient
}

// NewClient creates a new Media Service gRPC client.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(
		cfg.MediaService.Addr,
		grpc.WithUnaryInterceptor(authPropagationInterceptor()),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect media-service at %s: %w", cfg.MediaService.Addr, err)
	}
	return &Client{client: pb.NewMediaServiceClient(conn)}, nil
}

// FinalizeImages converts staged tokens to asset IDs.
func (c *Client) FinalizeImages(ctx context.Context, stagedTokens []string, refType string) ([]usecase.AssetInfo, error) {
	resp, err := c.client.FinalizeImages(ctx, &pb.FinalizeImagesRequest{StagedTokens: stagedTokens, RefType: refType})
	if err != nil {
		return nil, fmt.Errorf("finalize images: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("finalize images failed (code=%d): %s", resp.Code, resp.Message)
	}

	assets := make([]usecase.AssetInfo, len(resp.Assets))
	for i, a := range resp.Assets {
		assets[i] = usecase.AssetInfo{
			AssetId:     a.AssetId,
			StagedToken: a.StagedToken,
			ObjectKey:   a.ObjectKey,
			IsSecure:    a.IsSecure,
		}
	}
	return assets, nil
}

// LinkImages creates references between assets and a business entity.
// 委派 pkg/media 的共用實作（含冪等重試）；呼叫端必須把錯誤當致命並在寫業務列之前呼叫，
// 順序契約見 pkgmedia.LinkImages。
func (c *Client) LinkImages(ctx context.Context, refType, refID string, assetIDs []string) error {
	return pkgmedia.LinkImages(ctx, c.client, refType, refID, assetIDs)
}

// ReleaseReferences 移除 (refType, refIDs) 的 asset_reference，keepAssetIDs 內的資產除外
// （#1489 / #2222）。keepAssetIDs 為空 = 釋放全部（實體或帳號刪除）；非空 = 換圖收斂，
// media-service 在單一交易內刪舊留新。呼叫端 best-effort，且必須排在業務列寫入之後。
func (c *Client) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	return pkgmedia.ReleaseReferences(ctx, c.client, refType, refIDs, keepAssetIDs)
}
