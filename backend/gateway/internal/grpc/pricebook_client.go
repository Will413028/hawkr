// gateway/internal/grpc/pricebook_client.go
package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// PricebookClient is the gRPC client for pricebook-service.
type PricebookClient struct {
	conn   *grpc.ClientConn
	client pb.PricebookServiceClient
}

// NewPricebookClient creates a new pricebook gRPC client.
func NewPricebookClient() (*PricebookClient, error) {
	host := getEnvOrDefault("PRICEBOOK_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("PRICEBOOK_SERVICE_GRPC_PORT", "10039")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pricebook service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to pricebook gRPC service at %s", target)

	return &PricebookClient{conn: conn, client: pb.NewPricebookServiceClient(conn)}, nil
}

// Close closes the gRPC connection.
func (c *PricebookClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetObservedPrice fetches historical observed prices for a resolved crop
// or crop group. history=true returns the full observed_window_days window
// unreduced (for trend charts), instead of collapsing to the latest row
// per (market, crop).
func (c *PricebookClient) GetObservedPrice(ctx context.Context, cropName, marketCode, cropCode, groupCode string, history bool) (*pb.GetObservedPriceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := &pb.GetObservedPriceRequest{CropName: cropName, History: history}
	if marketCode != "" {
		req.MarketCode = &marketCode
	}
	if cropCode != "" {
		req.CropCode = &cropCode
	}
	if groupCode != "" {
		req.GroupCode = &groupCode
	}
	return c.client.GetObservedPrice(ctx, req)
}

// GetForecastCurve fetches the full 7-day forecast curve for a resolved
// crop or crop group.
func (c *PricebookClient) GetForecastCurve(ctx context.Context, cropName, marketCode, cropCode, groupCode string) (*pb.GetForecastCurveResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := &pb.GetForecastCurveRequest{CropName: cropName}
	if marketCode != "" {
		req.MarketCode = &marketCode
	}
	if cropCode != "" {
		req.CropCode = &cropCode
	}
	if groupCode != "" {
		req.GroupCode = &groupCode
	}
	return c.client.GetForecastCurve(ctx, req)
}

// GetTrendSummary fetches a day/week trend series (+ optional WoW/YoY364
// comparison) for a resolved crop (single market, marketCode required) or
// crop group (marketCode forbidden).
func (c *PricebookClient) GetTrendSummary(ctx context.Context, cropCode, groupCode, marketCode, from, to, granularity string, comparisons []string) (*pb.GetTrendSummaryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := &pb.GetTrendSummaryRequest{From: from, To: to, Granularity: granularity, Comparisons: comparisons}
	if cropCode != "" {
		req.CropCode = &cropCode
	}
	if groupCode != "" {
		req.GroupCode = &groupCode
	}
	if marketCode != "" {
		req.MarketCode = &marketCode
	}
	return c.client.GetTrendSummary(ctx, req)
}

// SearchCropCatalog searches the full crop/crop_group catalog.
func (c *PricebookClient) SearchCropCatalog(ctx context.Context, query string, limit int32) (*pb.SearchCropCatalogResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.SearchCropCatalog(ctx, &pb.SearchCropCatalogRequest{Query: query, Limit: limit})
}

// GetSpatialSurface fetches the nationwide price surface for a crop or crop
// group at a single horizon (issue #2248).
func (c *PricebookClient) GetSpatialSurface(ctx context.Context, cropCode, groupCode string, horizonDays int32) (*pb.GetSpatialSurfaceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := &pb.GetSpatialSurfaceRequest{HorizonDays: horizonDays}
	if cropCode != "" {
		req.CropCode = &cropCode
	}
	if groupCode != "" {
		req.GroupCode = &groupCode
	}
	return c.client.GetSpatialSurface(ctx, req)
}

// GetRegionalComparison fetches the per-county price comparison table for
// a crop or crop group at a single horizon (issue #2249 Slice C).
func (c *PricebookClient) GetRegionalComparison(ctx context.Context, cropCode, groupCode string, horizonDays int32) (*pb.GetRegionalComparisonResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := &pb.GetRegionalComparisonRequest{HorizonDays: horizonDays}
	if cropCode != "" {
		req.CropCode = &cropCode
	}
	if groupCode != "" {
		req.GroupCode = &groupCode
	}
	return c.client.GetRegionalComparison(ctx, req)
}

// ResolveCategoryMapping resolves a product-service categoryId to its
// authoritative Pricebook crop/group mapping (issue #2786).
func (c *PricebookClient) ResolveCategoryMapping(ctx context.Context, categoryID string) (*pb.ResolveCategoryMappingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ResolveCategoryMapping(ctx, &pb.ResolveCategoryMappingRequest{CategoryId: categoryID})
}

// UpsertCategoryMapping writes an admin-curated categoryId→crop/group
// mapping (issue #2786).
func (c *PricebookClient) UpsertCategoryMapping(ctx context.Context, req *pb.UpsertCategoryMappingRequest) (*pb.UpsertCategoryMappingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.UpsertCategoryMapping(ctx, req)
}

// ListCategoryMappingWorklist lists categories still needing an admin
// mapping decision (issue #2786).
func (c *PricebookClient) ListCategoryMappingWorklist(ctx context.Context, statusFilter string) (*pb.ListCategoryMappingWorklistResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListCategoryMappingWorklist(ctx, &pb.ListCategoryMappingWorklistRequest{Status: statusFilter})
}

// GetCategoryPriceRollup fetches the daily-computed price snapshot for a
// product-service category at any tree level (L1/L2/L3) (Task 10).
func (c *PricebookClient) GetCategoryPriceRollup(ctx context.Context, categoryID string) (*pb.GetCategoryPriceRollupResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetCategoryPriceRollup(ctx, &pb.GetCategoryPriceRollupRequest{CategoryId: categoryID})
}
