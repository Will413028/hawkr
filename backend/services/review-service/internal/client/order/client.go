package order

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.OrderServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.OrderService.Addr, grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order-service at %s: %w", cfg.OrderService.Addr, err)
	}
	return &Client{client: pb.NewOrderServiceClient(conn)}, nil
}

func (c *Client) GetOrderForReview(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
	resp, err := c.client.GetOrderForReview(ctx, &pb.GetOrderForReviewRequest{OrderNo: orderNo})
	if err != nil {
		return nil, nil, fmt.Errorf("GetOrderForReview: %w", err)
	}
	if resp.Code != 0 || resp.Data == nil {
		return nil, nil, fmt.Errorf("GetOrderForReview: code=%d msg=%s", resp.Code, resp.Message)
	}

	buyerID, err := strconv.ParseInt(resp.Data.BuyerId, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid buyer_id %q: %w", resp.Data.BuyerId, err)
	}
	sellerID, err := strconv.ParseInt(resp.Data.StoreId, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid store_id %q: %w", resp.Data.StoreId, err)
	}

	completedAt, err := timeutil.ParseISO8601(resp.Data.CompletedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid completed_at %q: %w", resp.Data.CompletedAt, err)
	}

	eligibility := &domain.OrderEligibility{
		OrderNo:     resp.Data.OrderNo,
		BuyerID:     buyerID,
		StoreID:     sellerID,
		Status:      resp.Data.Status,
		CompletedAt: completedAt,
	}

	items := make([]*domain.OrderItemInfo, 0, len(resp.Items))
	for _, pbItem := range resp.Items {
		itemID, err := strconv.ParseInt(pbItem.OrderItemId, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid order_item_id %q: %w", pbItem.OrderItemId, err)
		}
		price, err := strconv.ParseInt(pbItem.Price, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid item price %q: %w", pbItem.Price, err)
		}
		var productID int64
		if pbItem.ProductId != "" {
			productID, err = strconv.ParseInt(pbItem.ProductId, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid product_id %q: %w", pbItem.ProductId, err)
			}
		}
		items = append(items, &domain.OrderItemInfo{
			OrderItemID:    itemID,
			SkuName:        pbItem.SkuName,
			ImageURL:       pbItem.ImageUrl,
			ImageObjectKey: pbItem.ImageObjectKey,
			Price:          price,
			ProductID:      productID,
		})
	}

	return eligibility, items, nil
}
