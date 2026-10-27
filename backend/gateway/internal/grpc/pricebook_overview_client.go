package grpc

import (
	"context"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"time"
)

func (c *PricebookClient) GetMarketOverview(ctx context.Context, req *pb.GetMarketOverviewRequest) (*pb.GetMarketOverviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetMarketOverview(ctx, req)
}
func (c *PricebookClient) CreateMarketWatchTarget(ctx context.Context, req *pb.CreateMarketWatchTargetRequest) (*pb.CreateMarketWatchTargetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.CreateMarketWatchTarget(ctx, req)
}
func (c *PricebookClient) DeleteMarketWatchTarget(ctx context.Context, req *pb.DeleteMarketWatchTargetRequest) (*pb.DeleteMarketWatchTargetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.DeleteMarketWatchTarget(ctx, req)
}
