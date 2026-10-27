package category

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"google.golang.org/grpc"
)

// Client wraps the gRPC CategoryServiceClient.
type Client struct {
	client categorypb.CategoryServiceClient
}

// NewCategoryClient 建立連線至 product-service gRPC 的 CategoryClient。
func NewCategoryClient(cfg *config.Config) (*Client, error) {
	target := fmt.Sprintf("%s:%s", cfg.ProductServiceHost, cfg.ProductServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to category service at %s: %w", target, err)
	}
	return &Client{client: categorypb.NewCategoryServiceClient(conn)}, nil
}

// GetCategoryTree 取得分類樹，status="active" 只回傳啟用分類。
func (c *Client) GetCategoryTree(ctx context.Context, status string) (*categorypb.GetCategoryTreeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout10s)
	defer cancel()
	return c.client.GetCategoryTree(ctx, &categorypb.GetCategoryTreeRequest{
		Status: status,
	})
}
