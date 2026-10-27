package pricebook

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
)

// ResolveCategoryMapping uses the same authenticated gRPC client as all other
// Pricebook operations. This optional capability leaves forecast-only callers
// and test doubles on their existing PricebookClient interface.
func (c *pricebookClient) ResolveCategoryMapping(ctx context.Context, categoryID string) (usecase.PricebookCategoryMapping, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	resp, err := c.client.ResolveCategoryMapping(ctx, &pricebookpb.ResolveCategoryMappingRequest{CategoryId: categoryID})
	if err != nil {
		return usecase.PricebookCategoryMapping{}, err
	}
	if resp == nil {
		return usecase.PricebookCategoryMapping{}, nil
	}
	out := usecase.PricebookCategoryMapping{
		CategoryID: resp.GetCategoryId(), Status: resp.GetStatus(),
		GroupCode: resp.GroupCode, CommodityCode: resp.CommodityCode,
		Label:          resp.Label,
		MappingVersion: resp.GetMappingVersion(),
	}
	for _, v := range resp.GetCropVarieties() {
		label := v.GetVariantName()
		if strings.TrimSpace(label) == "" {
			label = v.GetName()
		}
		out.CropVarieties = append(out.CropVarieties, usecase.PricebookCropVariety{
			CropCode: v.GetCropCode(), Label: label,
		})
	}
	return out, nil
}
