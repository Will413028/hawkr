package product

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.ProductServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.ProductService.Addr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to product-service at %s: %w", cfg.ProductService.Addr, err)
	}
	return &Client{client: pb.NewProductServiceClient(conn)}, nil
}

func (c *Client) GetProductsByIDs(ctx context.Context, productIDs []int64) ([]*usecase.ProductInfo, error) {
	ids := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		ids = append(ids, strconv.FormatInt(id, 10))
	}

	resp, err := c.client.GetProductsByIDs(ctx, &pb.GetProductsByIDsRequest{ProductIds: ids})
	if err != nil {
		return nil, fmt.Errorf("GetProductsByIDs gRPC failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("GetProductsByIDs error: code=%d msg=%s", resp.Code, resp.Message)
	}

	result := make([]*usecase.ProductInfo, 0, len(resp.Products))
	for _, p := range resp.Products {
		pid, _ := strconv.ParseInt(p.GetId(), 10, 64)
		sid, _ := strconv.ParseInt(p.GetStoreId(), 10, 64)
		result = append(result, &usecase.ProductInfo{ID: pid, SellerID: sid})
	}
	return result, nil
}

// ListStoreIDsWithPublicProducts returns the store ids that currently have at
// least one publicly listed product (product-service 02 §5.13c).
//
// It sits on the storefront's cache-miss path (recommended-sellers rail and
// /public/marketplace), so it carries its own deadline: the caller fails open on
// error, and "fails open" is only useful if it happens quickly.
//
// A malformed store_id is an error rather than a silently dropped element —
// dropping it would quietly hide that seller from the carousel with no signal.
func (c *Client) ListStoreIDsWithPublicProducts(ctx context.Context) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout3s)
	defer cancel()

	resp, err := c.client.ListStoreIDsWithPublicProductsInternal(ctx,
		&pb.ListStoreIDsWithPublicProductsInternalRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListStoreIDsWithPublicProductsInternal gRPC failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("ListStoreIDsWithPublicProductsInternal error: code=%d msg=%s", resp.Code, resp.Message)
	}

	result := make([]int64, 0, len(resp.StoreIds))
	for _, raw := range resp.StoreIds {
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return nil, fmt.Errorf("ListStoreIDsWithPublicProductsInternal: invalid store_id %q: %w", raw, perr)
		}
		result = append(result, id)
	}
	return result, nil
}
