// Package order wraps the order-service gRPC client for the IM-side seller
// suspension order-binding logic (14 號 §6.5.1, 46 號 §4.4). Both calls here are
// internal-only RPCs (x-internal-service, no permission check on the order side).
package order

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"google.golang.org/grpc"
)

// Client wraps the order-service gRPC client.
type Client struct {
	client pb.OrderServiceClient
}

// NewClient creates a new order-service gRPC client.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.OrderServiceAddr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order-service at %s: %w", cfg.OrderServiceAddr, err)
	}
	return &Client{client: pb.NewOrderServiceClient(conn)}, nil
}

// OrderParties holds the buyer/store identity and status of an order,
// used to validate the three-way match for order-context conversation binding
// (14 號 §6.5.1 路徑①).
type OrderParties struct {
	BuyerID int64
	StoreID int64
	Status  string
}

// GetOrderParties fetches the buyer/seller/status of an order by order_no, reusing
// the existing internal GetOrderForAfterSales RPC (already x-internal-service,
// no-permission-check, and already returns exactly the three fields needed here —
// no new RPC was warranted per 07 方案設計 §6.4's explicit "HasOrderBetween 為新 RPC"
// scoping). Returns an error if the order does not exist or the response cannot be
// parsed; callers must treat any error as "cannot verify" (do not bind).
func (c *Client) GetOrderParties(ctx context.Context, orderNo string) (OrderParties, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetOrderForAfterSales(ctx, &pb.GetOrderForAfterSalesRequest{OrderNo: orderNo})
	if err != nil {
		return OrderParties{}, fmt.Errorf("get order parties for %s: %w", orderNo, err)
	}
	if resp.GetData() == nil {
		return OrderParties{}, fmt.Errorf("order %s not found", orderNo)
	}
	buyerID, err := strconv.ParseInt(resp.GetData().GetBuyerId(), 10, 64)
	if err != nil {
		return OrderParties{}, fmt.Errorf("parse buyer_id for %s: %w", orderNo, err)
	}
	sellerID, err := strconv.ParseInt(resp.GetData().GetStoreId(), 10, 64)
	if err != nil {
		return OrderParties{}, fmt.Errorf("parse store_id for %s: %w", orderNo, err)
	}
	return OrderParties{
		BuyerID: buyerID,
		StoreID: sellerID,
		Status:  resp.GetData().GetStatus(),
	}, nil
}

// HasOrderBetween reverse-looks-up whether a "related" order (status NOT IN
// (0,80), 14 號 §6.5.2) exists between the given buyer and seller, used by the
// existing-conversation backfill path (14 號 §6.5.1 路徑②). Returns the RPC error
// unwrapped so callers can distinguish "no order" (false, nil) from "could not
// verify" (error) — §6.5.3 requires the latter to deny (回填失敗 → 擋), not fail-open.
func (c *Client) HasOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.HasOrderBetween(ctx, &pb.HasOrderBetweenRequest{
		BuyerId: buyerID,
		StoreId: storeID,
	})
	if err != nil {
		return false, "", fmt.Errorf("has order between buyer=%d seller=%d: %w", buyerID, storeID, err)
	}
	return resp.GetHasOrder(), resp.GetLatestOrderNo(), nil
}
