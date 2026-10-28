package grpc

import (
	"context"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
)

// SellerClient method wrappers for governance / food-safety / appeals (B-169) —
// gateway → seller 模組 gRPC.
// 配合 internal/handler/governance.go HTTP handler 使用。

const governanceRPCTimeout = 30 * time.Second

// ----- Reporter side -----

func (c *SellerClient) CreateViolationReport(ctx context.Context, req *pb.CreateViolationReportRequest) (*pb.CreateViolationReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, governanceRPCTimeout)
	defer cancel()
	return c.client.CreateViolationReport(ctx, req)
}

func (c *SellerClient) ListMyViolationReports(ctx context.Context, req *pb.ListMyViolationReportsRequest) (*pb.ListMyViolationReportsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, governanceRPCTimeout)
	defer cancel()
	return c.client.ListMyViolationReports(ctx, req)
}

func (c *SellerClient) CreateFoodSafetyReport(ctx context.Context, req *pb.CreateFoodSafetyReportRequest) (*pb.CreateFoodSafetyReportResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, governanceRPCTimeout)
	defer cancel()
	return c.client.CreateFoodSafetyReport(ctx, req)
}

// ----- Public seller-page violations -----

func (c *SellerClient) GetPublicSellerViolations(ctx context.Context, req *pb.GetPublicSellerViolationsRequest) (*pb.GetPublicSellerViolationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, governanceRPCTimeout)
	defer cancel()
	return c.client.GetPublicSellerViolations(ctx, req)
}
