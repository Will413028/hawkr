package seller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

type sellerClient struct {
	client sellerpb.SellerServiceClient
}

func NewSellerClient(cfg *config.Config) (usecase.SellerClient, error) {
	target := fmt.Sprintf("%s:%s", cfg.SellerServiceHost, cfg.SellerServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to seller-service at %s: %w", target, err)
	}
	return &sellerClient{client: sellerpb.NewSellerServiceClient(conn)}, nil
}

func (c *sellerClient) GetSellerStats(ctx context.Context, storeID identity.StoreID) (*usecase.SellerStatsItem, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetSellerStats(ctx, &sellerpb.GetSellerStatsRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		return nil, err
	}

	d := resp.GetData()
	if d == nil {
		return nil, fmt.Errorf("seller stats not found for store %d", storeID)
	}

	return &usecase.SellerStatsItem{

		StoreID:       storeID,
		TotalProducts: d.GetTotalProducts(),
		TotalOrders:   d.GetTotalOrders(),
		TotalSales:    d.GetTotalSales(),
		Rating:        d.GetRating(),
		Followers:     d.GetFollowers(),
	}, nil
}

func (c *sellerClient) BatchGetSellerMeta(ctx context.Context, storeIDs []identity.StoreID) (map[identity.StoreID]*usecase.SellerMetaItem, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	raw := make([]int64, 0, len(storeIDs))
	for _, id := range storeIDs {
		raw = append(raw, int64(id))
	}
	resp, err := c.client.BatchGetSellerMeta(ctx, &sellerpb.BatchGetSellerMetaRequest{
		StoreIds: raw,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[identity.StoreID]*usecase.SellerMetaItem, len(resp.GetSellers()))
	for id, meta := range resp.GetSellers() {
		result[identity.StoreID(id)] = &usecase.SellerMetaItem{
			AccountID:  identity.AccountID(meta.GetAccountId()),
			SellerName: meta.GetSellerName(),
			LogoKey:    meta.GetLogoKey(),
			StoreCode:  meta.GetStoreCode(),
		}
	}
	return result, nil
}

func (c *sellerClient) SearchProspects(ctx context.Context, params usecase.ProspectSearchParams) ([]usecase.ProspectHit, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.SearchSellerProspects(ctx, &sellerpb.SearchSellerProspectsRequest{
		Category: params.Category, Region: params.Region,
		Keywords: params.Keywords, CategoryIds: params.CategoryIDs,
		Limit: int32(params.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]usecase.ProspectHit, 0, len(resp.GetData()))
	for _, h := range resp.GetData() {
		p := h.GetProspect()
		id, err := strconv.ParseInt(p.GetId(), 10, 64)
		if err != nil {

			continue
		}

		updatedAt, _ := time.Parse(time.RFC3339, p.GetUpdatedAt())
		out = append(out, usecase.ProspectHit{
			ID: id, DisplayName: p.GetDisplayName(),
			Category: p.GetCategory(), Region: p.GetRegion(),
			ProductSummary: p.GetProductSummary(), Positioning: p.GetPositioning(),
			ResearchSummary: p.GetResearchSummary(),
			PublicWebsite:   p.GetPublicWebsite(),
			PriorityLetter:  p.GetPriorityLetter(),

			PriorityStars: atoiOrZero(p.GetPriorityStars()),
			UpdatedAt:     updatedAt,
			LogoKey:       p.GetLogoKey(),
			CoverKey:      p.GetCoverKey(),
			HitCategory:   h.GetHitCategory(), HitRegion: h.GetHitRegion(), HitKeywords: h.GetHitKeywords(),
			HitCategoryIDs: h.GetHitCategoryIds(),
		})
	}
	return out, nil
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
