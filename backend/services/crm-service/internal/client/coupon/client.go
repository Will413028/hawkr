package coupon

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.CouponServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.CouponService.Addr,
		// 內部身分 HMAC 簽章（#1366，取代原 internalServiceInterceptor）；不轉發使用者身分（保留原語義）。
		// 缺此簽章＝enforcing 後 coupon-service 對 crm 呼叫回 UNAUTHENTICATED（原 Tier 2 audit 教訓的升級版）。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("crm-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to coupon-service at %s: %w", cfg.CouponService.Addr, err)
	}
	return &Client{client: pb.NewCouponServiceClient(conn)}, nil
}

func (c *Client) GetBuyerCouponCount(ctx context.Context, storeID, buyerID int64) (int32, error) {
	resp, err := c.client.ListBuyerCouponsForSeller(ctx, &pb.ListBuyerCouponsForSellerRequest{
		StoreId:  storeID,
		BuyerId:  strconv.FormatInt(buyerID, 10),
		Page:     "1",
		PageSize: "1",
	})
	if err != nil {
		return 0, fmt.Errorf("list buyer coupons: %w", err)
	}
	total, err := strconv.ParseInt(resp.Total, 10, 32)
	if err != nil {
		return 0, nil
	}
	return int32(total), nil
}
