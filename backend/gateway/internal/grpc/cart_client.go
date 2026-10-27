package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/cart"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// CartClient 是 cart-service 的 gRPC 用戶端
type CartClient struct {
	conn   *grpc.ClientConn
	client pb.CartServiceClient
}

// NewCartClient 建立新的 cart gRPC 用戶端
func NewCartClient() (*CartClient, error) {
	host := getEnvOrDefault("CART_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("CART_SERVICE_GRPC_PORT", "9927")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cart service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to cart gRPC service at %s", target)

	return &CartClient{
		conn:   conn,
		client: pb.NewCartServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *CartClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func ctxWithAccount(ctx context.Context, timeout time.Duration, userID int64) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	ctx = WithAuthContext(ctx, userID, "")
	return ctx, cancel
}

// ctxWithoutAccount 建立不含身分 metadata 的 context（訪客流程）
func ctxWithoutAccount(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// ==================== 對外 API ====================

// AddItem 加入購物車
func (c *CartClient) AddItem(ctx context.Context, userID int64, req *pb.AddItemRequest) (*pb.AddItemResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()
	return c.client.AddItem(rpcCtx, req)
}

// GetCart 查詢購物車
func (c *CartClient) GetCart(ctx context.Context, userID int64, req *pb.GetCartRequest) (*pb.GetCartResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()
	return c.client.GetCart(rpcCtx, req)
}

// SetItemQuantity 設定商品數量
func (c *CartClient) SetItemQuantity(ctx context.Context, userID int64, req *pb.SetItemQuantityRequest) (*pb.SetItemQuantityResponse, error) {
	var rpcCtx context.Context
	var cancel context.CancelFunc
	if userID > 0 {
		rpcCtx, cancel = ctxWithAccount(ctx, 30*time.Second, userID)
	} else {
		rpcCtx, cancel = ctxWithoutAccount(ctx, 30*time.Second)
	}
	defer cancel()
	return c.client.SetItemQuantity(rpcCtx, req)
}

// MergeCart 合併購物車（登入後）
func (c *CartClient) MergeCart(ctx context.Context, userID int64, req *pb.MergeCartRequest) (*pb.MergeCartResponse, error) {
	rpcCtx, cancel := ctxWithAccount(ctx, 30*time.Second, userID)
	defer cancel()
	return c.client.MergeCart(rpcCtx, req)
}
