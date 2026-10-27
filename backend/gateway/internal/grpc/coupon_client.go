package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// CouponClient 是 coupon-service 的 gRPC 用戶端
type CouponClient struct {
	conn   *grpc.ClientConn
	client pb.CouponServiceClient
}

// NewCouponClient 建立新的 coupon gRPC 用戶端
func NewCouponClient() (*CouponClient, error) {
	host := getEnvOrDefault("COUPON_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("COUPON_SERVICE_GRPC_PORT", "9951")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to coupon service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to coupon gRPC service at %s", target)

	return &CouponClient{
		conn:   conn,
		client: pb.NewCouponServiceClient(conn),
	}, nil
}

// Close 關閉連線
func (c *CouponClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== Buyer ====================

// ListMyCoupons 查詢買家優惠券列表
func (c *CouponClient) ListMyCoupons(ctx context.Context, req *pb.ListMyCouponsRequest) (*pb.ListMyCouponsResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListMyCoupons(rpcCtx, req)
}

// BuyerClaimCoupon 買家領取優惠券
func (c *CouponClient) BuyerClaimCoupon(ctx context.Context, req *pb.BuyerClaimCouponRequest) (*pb.BuyerClaimCouponResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.BuyerClaimCoupon(rpcCtx, req)
}

// ListAvailableCoupons 查詢可領取優惠券
func (c *CouponClient) ListAvailableCoupons(ctx context.Context, req *pb.ListAvailableCouponsRequest) (*pb.ListAvailableCouponsResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListAvailableCoupons(rpcCtx, req)
}

// ==================== Seller ====================

// SellerCreateTemplate 賣家建立優惠券
func (c *CouponClient) SellerCreateTemplate(ctx context.Context, req *pb.SellerCreateTemplateRequest) (*pb.SellerCreateTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SellerCreateTemplate(rpcCtx, req)
}

// SellerListTemplates 賣家模板列表
func (c *CouponClient) SellerListTemplates(ctx context.Context, req *pb.SellerListTemplatesRequest) (*pb.SellerListTemplatesResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SellerListTemplates(rpcCtx, req)
}

// SellerGetTemplateDetail 賣家模板詳情
func (c *CouponClient) SellerGetTemplateDetail(ctx context.Context, req *pb.SellerGetTemplateDetailRequest) (*pb.SellerGetTemplateDetailResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SellerGetTemplateDetail(rpcCtx, req)
}

// SellerRevokeTemplate 賣家收回模板
func (c *CouponClient) SellerRevokeTemplate(ctx context.Context, req *pb.SellerRevokeTemplateRequest) (*pb.SellerRevokeTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SellerRevokeTemplate(rpcCtx, req)
}

// ListBuyerCouponsForSeller 賣家查詢買家持券
func (c *CouponClient) ListBuyerCouponsForSeller(ctx context.Context, req *pb.ListBuyerCouponsForSellerRequest) (*pb.ListBuyerCouponsForSellerResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListBuyerCouponsForSeller(rpcCtx, req)
}

// ==================== Admin ====================

// AdminCreateTemplate 管理員建立模板
func (c *CouponClient) AdminCreateTemplate(ctx context.Context, req *pb.AdminCreateTemplateRequest) (*pb.AdminCreateTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminCreateTemplate(rpcCtx, req)
}

// AdminPublishTemplate 管理員上架模板
func (c *CouponClient) AdminPublishTemplate(ctx context.Context, req *pb.AdminPublishTemplateRequest) (*pb.AdminPublishTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminPublishTemplate(rpcCtx, req)
}

// AdminDisableTemplate 管理員停用模板
func (c *CouponClient) AdminDisableTemplate(ctx context.Context, req *pb.AdminDisableTemplateRequest) (*pb.AdminDisableTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminDisableTemplate(rpcCtx, req)
}

// AdminIssueCoupons 管理員發行券
func (c *CouponClient) AdminIssueCoupons(ctx context.Context, req *pb.AdminIssueCouponsRequest) (*pb.AdminIssueCouponsResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminIssueCoupons(rpcCtx, req)
}

// AdminDistributeTemplate 管理員代領模板
func (c *CouponClient) AdminDistributeTemplate(ctx context.Context, req *pb.AdminDistributeTemplateRequest) (*pb.AdminDistributeTemplateResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminDistributeTemplate(rpcCtx, req)
}

// AdminGrantCompensationCoupon 管理員發放補償券
func (c *CouponClient) AdminGrantCompensationCoupon(ctx context.Context, req *pb.AdminGrantCompensationCouponRequest) (*pb.AdminGrantCompensationCouponResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminGrantCompensationCoupon(rpcCtx, req)
}

// AdminVoidInstance 管理員作廢券
func (c *CouponClient) AdminVoidInstance(ctx context.Context, req *pb.AdminVoidInstanceRequest) (*pb.AdminVoidInstanceResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminVoidInstance(rpcCtx, req)
}

// AdminUpdateSellerQuota 管理員更新賣家配額
func (c *CouponClient) AdminUpdateSellerQuota(ctx context.Context, req *pb.AdminUpdateSellerQuotaRequest) (*pb.AdminUpdateSellerQuotaResponse, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.AdminUpdateSellerQuota(rpcCtx, req)
}
