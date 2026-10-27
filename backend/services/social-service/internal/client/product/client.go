package product

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.ProductServiceClient
}

func NewClient(cfg *config.Config) (usecase.ProductClient, error) {
	conn, err := grpcclient.Dial(cfg.Product.Addr(),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("product grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewProductServiceClient(conn)}, nil
}

func (c *Client) GetProductsByIDs(ctx context.Context, productIDs []string) (map[string]*usecase.ProductBasicInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetProductsByIDs(ctx, &pb.GetProductsByIDsRequest{ProductIds: productIDs})
	if err != nil {
		return nil, fmt.Errorf("GetProductsByIDs: %w", err)
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("GetProductsByIDs failed (code=%d): %s", resp.GetCode(), resp.GetMessage())
	}

	result := make(map[string]*usecase.ProductBasicInfo, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		result[p.GetId()] = &usecase.ProductBasicInfo{
			ID:       p.GetId(),
			Name:     p.GetName(),
			ImageURL: p.GetImageKey(),
			Price:    p.GetPriceMin(),
		}
	}
	return result, nil
}
