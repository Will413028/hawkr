package media

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.MediaServiceClient
}

func NewClient(cfg *config.Config) (usecase.MediaClient, error) {
	conn, err := grpcclient.Dial(cfg.MediaService.Addr(),
		// 內部身分 HMAC 簽章 + 轉發使用者身分（#1366，取代原 internalServiceInterceptor）。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("review-service", true)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("media grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewMediaServiceClient(conn)}, nil
}

// FinalizeAndLink 委派給 pkg/media 的共用實作，轉正並建立 asset_reference。
func (c *Client) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (usecase.AssetInfo, error) {
	info, err := pkgmedia.FinalizeAndLink(ctx, c.client, stagedToken, refType, refID)
	if err != nil {
		return usecase.AssetInfo{}, err
	}
	return usecase.AssetInfo{
		AssetId:   info.AssetId,
		ObjectKey: info.ObjectKey,
	}, nil
}
