package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type categoryMappingRepo struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewCategoryMappingRepository 建構 category_crop_mapping 表 + crop/crop_group
// active 驗證查詢的 repository 實作（issue #2786）。logger 只用在
// ListResolvedCategoryGroupMappings 的 nil-GroupCode 防禦性 skip（final
// whole-branch review finding 4）——這個 repository 目前沒有其他需要留痕跡的
// 分支。
func NewCategoryMappingRepository(pool *pgxpool.Pool, logger *zap.Logger) domain.CategoryMappingRepository {
	return &categoryMappingRepo{pool: pool, logger: logger}
}

func (r *categoryMappingRepo) Get(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCategoryCropMapping(ctx, categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m := toDomainCategoryMapping(row)
	return &m, nil
}

// Upsert 寫入或覆蓋一列，updated_at 由 DB now() 填（見 UpsertCategoryCropMapping
// 的 SQL），回傳寫入後的完整列。
func (r *categoryMappingRepo) Upsert(ctx context.Context, m domain.CategoryMapping) (*domain.CategoryMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.UpsertCategoryCropMapping(ctx, gen.UpsertCategoryCropMappingParams{
		CategoryID:     m.CategoryID,
		Status:         string(m.Status),
		GroupCode:      m.GroupCode,
		MappingVersion: m.MappingVersion,
		SourceNote:     m.SourceNote,
		UpdatedBy:      m.UpdatedBy,
	})
	if err != nil {
		return nil, err
	}
	saved := toDomainCategoryMapping(row)
	return &saved, nil
}

func (r *categoryMappingRepo) ListByStatuses(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	strStatuses := make([]string, 0, len(statuses))
	for _, s := range statuses {
		strStatuses = append(strStatuses, string(s))
	}
	rows, err := q.ListCategoryCropMappingsByStatuses(ctx, strStatuses)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CategoryMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainCategoryMapping(row))
	}
	return out, nil
}

func (r *categoryMappingRepo) ListAllCategoryIDs(ctx context.Context) ([]string, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.ListAllCategoryCropMappings(ctx)
}

func (r *categoryMappingRepo) ActiveGroupName(ctx context.Context, groupCode string) (*string, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.FindActiveCropGroupByCode(ctx, groupCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row.Name, nil
}

func (r *categoryMappingRepo) ActiveCommodityName(ctx context.Context, commodityID int64) (*string, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCommodityByID(ctx, commodityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row.Name, nil
}

func (r *categoryMappingRepo) ListResolvedCategoryGroupMappings(ctx context.Context) ([]domain.CategoryGroupMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListResolvedCategoryGroupMappings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CategoryGroupMapping, 0, len(rows))
	for _, row := range rows {
		// row.GroupCode is *string because the sqlc-inferred column type is
		// nullable (group_code has no column-level NOT NULL) — the query's
		// WHERE status = 'resolved' filter combined with the
		// category_crop_mapping_resolved_oneof_check CHECK constraint
		// (status = 'resolved' => group_code IS NOT NULL) guarantees non-nil
		// here; skip defensively rather than panic if that invariant is ever
		// violated. Warn so a production violation of this invariant leaves a
		// trace instead of silently dropping the row (final whole-branch
		// review finding 4).
		if row.GroupCode == nil {
			r.logger.Warn("category_mapping_repo: resolved 列的 group_code 為 nil，違反 CHECK constraint 不變量，已跳過",
				zap.String("category_id", row.CategoryID))
			continue
		}
		out = append(out, domain.CategoryGroupMapping{CategoryID: row.CategoryID, GroupCode: *row.GroupCode})
	}
	return out, nil
}

func (r *categoryMappingRepo) ListResolvedCategoryCommodityMappings(ctx context.Context) ([]domain.CategoryCommodityMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListResolvedCategoryCommodityMappings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CategoryCommodityMapping, 0, len(rows))
	for _, row := range rows {
		if row.CommodityID == nil {
			r.logger.Warn("category_mapping_repo: resolved 列的 commodity_id 為 nil，違反 CHECK constraint 不變量，已跳過",
				zap.String("category_id", row.CategoryID))
			continue
		}
		out = append(out, domain.CategoryCommodityMapping{CategoryID: row.CategoryID, CommodityID: *row.CommodityID})
	}
	return out, nil
}

// toDomainCategoryMapping 轉換 sqlc 產生的 gen.CategoryCropMapping。nullable
// TEXT 欄位使用 generated *string，直接沿用 NULL 語意，不在 repository boundary
// 重建 pgtype nullable wrapper。
func toDomainCategoryMapping(row gen.CategoryCropMapping) domain.CategoryMapping {
	return domain.CategoryMapping{
		CategoryID:     row.CategoryID,
		Status:         domain.CategoryMappingStatus(row.Status),
		GroupCode:      row.GroupCode,
		CommodityID:    row.CommodityID,
		MappingVersion: row.MappingVersion,
		SourceNote:     row.SourceNote,
		UpdatedBy:      row.UpdatedBy,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func (r *categoryMappingRepo) ListResolvedCategoryCropVarietyMappings(ctx context.Context) ([]domain.CategoryCropVarietyMapping, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListResolvedCategoryCropMappings(ctx)
	if err != nil {
		return nil, err
	}
	byCategory := make(map[string][]string, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, seen := byCategory[row.CategoryID]; !seen {
			order = append(order, row.CategoryID)
		}
		byCategory[row.CategoryID] = append(byCategory[row.CategoryID], row.CropCode)
	}
	out := make([]domain.CategoryCropVarietyMapping, 0, len(order))
	for _, categoryID := range order {
		out = append(out, domain.CategoryCropVarietyMapping{
			CategoryID: categoryID, CropCodes: byCategory[categoryID],
		})
	}
	return out, nil
}

func (r *categoryMappingRepo) ListCategoryCropVarieties(ctx context.Context, categoryID string) ([]domain.CropVariety, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListCategoryCropMappingCrops(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CropVariety, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CropVariety{
			CropCode: row.CropCode, Name: row.Name, VariantName: row.VariantName,
		})
	}
	return out, nil
}
