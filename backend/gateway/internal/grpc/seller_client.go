package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// SellerClient is the gRPC client for seller service
type SellerClient struct {
	conn   *grpc.ClientConn
	client pb.SellerServiceClient // Reusing SellerServiceClient as proto is shared
}

// NewSellerClient creates a new seller gRPC client
func NewSellerClient() (*SellerClient, error) {
	host := getEnvOrDefault("SELLER_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("SELLER_SERVICE_GRPC_PORT", "9941")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to seller service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to seller gRPC service at %s", target)

	return &SellerClient{
		conn:   conn,
		client: pb.NewSellerServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *SellerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetSellerByAccountID gets seller info by account ID.
func (c *SellerClient) GetSellerByAccountID(ctx context.Context, accountID int64) (*pb.SellerResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetSeller(ctx, &pb.GetSellerRequest{
		AccountId: accountID,
	})
}

func (c *SellerClient) CurrentStoreID(ctx context.Context, accountID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.client.GetCurrentStoreID(ctx, &pb.GetCurrentStoreIDRequest{AccountId: accountID})
	if err != nil {
		return 0, err
	}
	return resp.StoreId, nil
}

// GetSellerByStoreID resolves a store by its PK surrogate（② public namespace）。
func (c *SellerClient) GetSellerByStoreID(ctx context.Context, storeID int64) (*pb.SellerResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSeller(ctx, &pb.GetSellerRequest{StoreId: storeID})
}

// GetSellerStatsByStoreID：② public /sellers/:id/stats|rating PK 變體。
func (c *SellerClient) GetSellerStatsByStoreID(ctx context.Context, storeID int64) (*pb.SellerStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetSellerStats(ctx, &pb.GetSellerStatsRequest{StoreId: storeID})
}

// GetSellerBatch gets seller info for multiple store IDs.
func (c *SellerClient) GetSellerBatch(ctx context.Context, storeIDs []int64) (*pb.SellerBatchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetSellerBatch(ctx, &pb.GetSellerBatchRequest{
		StoreIds: storeIDs,
	})
}

// BatchGetSellerMeta 讀店家此刻的可曝光與曾公開過的旗標（公開評價的父資源檢查、買家聊天對象檢查）。
func (c *SellerClient) BatchGetSellerMeta(ctx context.Context, storeIDs []int64) (*pb.BatchGetSellerMetaResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.BatchGetSellerMeta(ctx, &pb.BatchGetSellerMetaRequest{StoreIds: storeIDs})
}

// ==================== Delete ====================

// ==================== Store Visitors ====================

// RecordStoreVisit records a buyer visiting a seller's store page (fire-and-forget 呼叫，錯誤可忽略)
func (c *SellerClient) RecordStoreVisit(ctx context.Context, storeID, visitorBuyerID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := c.client.RecordStoreVisit(ctx, &pb.RecordStoreVisitRequest{
		StoreId:        storeID,
		VisitorBuyerId: visitorBuyerID,
	})
	return err
}

// ==================== Recommended Sellers ====================

// GetRecommendedSellers gets recommended sellers from seller service
func (c *SellerClient) GetRecommendedSellers(ctx context.Context, limit int32) (*pb.GetRecommendedSellersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetRecommendedSellers(ctx, &pb.GetRecommendedSellersRequest{
		Limit: limit,
	})
}

// SuggestAddress 地址模糊搜尋（proxy 至 seller 模組的 geocode ACL）。
func (c *SellerClient) SuggestAddress(ctx context.Context, county, district, query string) (*pb.SuggestAddressResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SuggestAddress(ctx, &pb.SuggestAddressRequest{County: county, District: district, Query: query})
}

// ValidateAddress 地址嚴格驗證（proxy 至 seller 模組的 geocode ACL）。
func (c *SellerClient) ValidateAddress(ctx context.Context, address string) (*pb.ValidateAddressResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ValidateAddress(ctx, &pb.ValidateAddressRequest{Address: address})
}
