package usecase

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func declareCommodityCategories(
	ctx context.Context, q *gen.Queries,
	commodityID int64, categoryIDs []string, mappingVersion, note string,
) error {
	for _, categoryID := range categoryIDs {
		n := note
		if err := q.UpsertCategoryCommodityMapping(ctx, gen.UpsertCategoryCommodityMappingParams{
			CategoryID: categoryID, CommodityID: &commodityID,
			MappingVersion: mappingVersion, SourceNote: &n,

			UpdatedBy: 0,
		}); err != nil {
			return fmt.Errorf("宣告分類映射失敗（category=%s）: %w", categoryID, err)
		}
	}
	return nil
}
