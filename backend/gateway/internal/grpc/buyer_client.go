package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// BuyerClient is the gRPC client for buyer service
type BuyerClient struct {
	conn   *grpc.ClientConn
	client pb.BuyerServiceClient
}

// NewBuyerClient creates a new buyer gRPC client
func NewBuyerClient() (*BuyerClient, error) {
	host := getEnvOrDefault("LEGACY_BUYER_GRPC_HOST", "127.0.0.1")
	port := getEnvOrDefault("LEGACY_BUYER_GRPC_PORT", "9941")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to buyer service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to buyer gRPC service at %s", target)

	return &BuyerClient{
		conn:   conn,
		client: pb.NewBuyerServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *BuyerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetProfile gets user's profile
func (c *BuyerClient) GetProfile(ctx context.Context, accountID int64) (*pb.BuyerInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetBuyerProfile(ctx, &pb.GetBuyerProfileRequest{AccountId: accountID})
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("buyer service error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.Data, nil
}

// CheckFavorite checks if a product is favorited
func (c *BuyerClient) CheckFavorite(ctx context.Context, buyerID int64, productID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.CheckFavorite(ctx, &pb.CheckFavoriteRequest{
		BuyerId:   buyerID,
		ProductId: productID,
	})
	if err != nil {
		return false, err
	}
	if resp.Code != 0 {
		return false, fmt.Errorf("buyer service error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.Favorited, nil
}

// BatchCheckFavorites checks which productIDs are favorited by the buyer.
func (c *BuyerClient) BatchCheckFavorites(ctx context.Context, buyerID int64, productIDs []string) ([]string, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.BatchCheckFavorites(ctx, &pb.BatchCheckFavoritesRequest{
		BuyerId:    buyerID,
		ProductIds: productIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("buyer service error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.FavoritedProductIds, nil
}


// BatchCheckRestockSubscriptions returns the set of product IDs the buyer has ACTIVE
// restock subscriptions for, keyed by product_id string.
func (c *BuyerClient) BatchCheckRestockSubscriptions(ctx context.Context, buyerID int64, productIDs []string) (map[string]bool, error) {
	if len(productIDs) == 0 {
		return map[string]bool{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.client.BatchCheckRestockSubscriptions(ctx, &pb.BatchCheckRestockSubscriptionsRequest{
		BuyerId:    buyerID,
		ProductIds: productIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("buyer service error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.Subscribed, nil
}
