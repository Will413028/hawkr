package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// MediaClient is the gRPC client for media service
type MediaClient struct {
	conn   *grpc.ClientConn
	client pb.MediaServiceClient
}

// NewMediaClient creates a new media gRPC client
func NewMediaClient() (*MediaClient, error) {
	host := getEnvOrDefault("MEDIA_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("MEDIA_SERVICE_GRPC_PORT", "9937")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to media service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to media gRPC service at %s", target)

	return &MediaClient{
		conn:   conn,
		client: pb.NewMediaServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *MediaClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ========== 上傳會話相關 ==========

// CreateUploadSession 建立上傳會話
func (c *MediaClient) CreateUploadSession(ctx context.Context, filename, contentType string, fileSize string, ownerID int64, isSecure bool) (*pb.CreateUploadSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.CreateUploadSessionRequest{
		Filename:    filename,
		ContentType: contentType,
		FileSize:    fileSize,
		IsSecure:    isSecure,
		AccountId:   ownerID,
	}

	return c.client.CreateUploadSession(ctx, req)
}

// CompleteUploadSession 完成上傳會話
func (c *MediaClient) CompleteUploadSession(ctx context.Context, sessionID string, ownerID int64) (*pb.CompleteUploadSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.CompleteUploadSessionRequest{
		SessionId: sessionID,
		AccountId: ownerID,
	}

	return c.client.CompleteUploadSession(ctx, req)
}

// ========== 資產管理相關 ==========

// FinalizeImages 轉正圖片
func (c *MediaClient) FinalizeImages(ctx context.Context, stagedTokens []string, refType string) (*pb.FinalizeImagesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req := &pb.FinalizeImagesRequest{
		StagedTokens: stagedTokens,
		RefType:      refType,
	}

	return c.client.FinalizeImages(ctx, req)
}

// LinkImages 建立資產引用
func (c *MediaClient) LinkImages(ctx context.Context, refType, refID string, assetIDs []string) (*pb.LinkImagesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.LinkImagesRequest{
		RefType:  refType,
		RefId:    refID,
		AssetIds: assetIDs,
	}

	return c.client.LinkImages(ctx, req)
}

// UnlinkImages 解除資產引用
func (c *MediaClient) UnlinkImages(ctx context.Context, refType, refID string, assetIDs []string) (*pb.UnlinkImagesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.UnlinkImagesRequest{
		RefType:  refType,
		RefId:    refID,
		AssetIds: assetIDs,
	}

	return c.client.UnlinkImages(ctx, req)
}
