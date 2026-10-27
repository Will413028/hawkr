package postgres

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type surfaceFormRepo struct {
	pool *pgxpool.Pool
}

func NewSurfaceFormRepository(pool *pgxpool.Pool) domain.SurfaceFormRepository {
	return &surfaceFormRepo{pool: pool}
}

// RegisterSurfaceForms 與 ingest 的 upsert 共用同一個 q（同交易）。先刪同實體同 kind 的
// 舊列（改名不留舊名），再依 kind 權威度由高到低寫入，讓 ON CONFLICT DO NOTHING 保住
// 最權威的 kind。ALIAS 不經此路（那是 seed／人工資料，不隨 ingest 重寫）。
func RegisterSurfaceForms(ctx context.Context, q *gen.Queries, regs []domain.SurfaceFormRegistration) error {
	type key struct {
		kind domain.EntityKind
		code string
		form domain.FormKind
	}
	seen := map[key]bool{}
	for _, reg := range regs {
		k := key{reg.EntityKind, reg.EntityCode, reg.FormKind}
		if seen[k] || reg.FormKind == domain.FormKindAlias {
			continue
		}
		seen[k] = true
		if err := q.DeleteSurfaceFormsByKind(ctx, gen.DeleteSurfaceFormsByKindParams{
			EntityKind: string(reg.EntityKind), EntityCode: reg.EntityCode, FormKind: string(reg.FormKind),
		}); err != nil {
			return fmt.Errorf("RegisterSurfaceForms: delete %s/%s/%s: %w", reg.EntityKind, reg.EntityCode, reg.FormKind, err)
		}
	}
	ordered := append([]domain.SurfaceFormRegistration(nil), regs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return domain.FormKindRank(ordered[i].FormKind) < domain.FormKindRank(ordered[j].FormKind)
	})
	for _, reg := range ordered {
		if strings.TrimSpace(reg.FormRaw) == "" {
			continue
		}
		if err := q.UpsertSurfaceForm(ctx, gen.UpsertSurfaceFormParams{
			EntityKind: string(reg.EntityKind), EntityCode: reg.EntityCode,
			FormRaw: reg.FormRaw, FormKind: string(reg.FormKind),
		}); err != nil {
			return fmt.Errorf("RegisterSurfaceForms: upsert %s/%s %q: %w", reg.EntityKind, reg.EntityCode, reg.FormRaw, err)
		}
	}
	return nil
}

func (r *surfaceFormRepo) Search(ctx context.Context, query string, limit int32) ([]domain.SurfaceFormHit, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.SearchSurfaceForms(ctx, gen.SearchSurfaceFormsParams{Query: query, RowLimit: limit})
	if err != nil {
		return nil, fmt.Errorf("surfaceFormRepo.Search: %w", err)
	}
	out := make([]domain.SurfaceFormHit, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SurfaceFormHit{
			EntityKind: domain.EntityKind(row.EntityKind), EntityCode: row.EntityCode,
			FormRaw: row.FormRaw, FormKind: domain.FormKind(row.FormKind), Tier: row.Tier,
			GroupCode: row.GroupCode, GroupName: row.GroupName,
			CropCode: row.CropCode, CropName: row.CropName, VariantName: row.VariantName,
			CommodityName: row.CommodityName,
		})
	}
	return out, nil
}

func (r *surfaceFormRepo) ListAliases(ctx context.Context, limit int32) ([]domain.SurfaceFormHit, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListAliasSurfaceForms(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("surfaceFormRepo.ListAliases: %w", err)
	}
	out := make([]domain.SurfaceFormHit, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SurfaceFormHit{
			EntityKind: domain.EntityKind(row.EntityKind), EntityCode: row.EntityCode,
			FormRaw: row.FormRaw, FormKind: domain.FormKindAlias,
			GroupCode: row.GroupCode, GroupName: row.GroupName,
			CropCode: row.CropCode, CropName: row.CropName, VariantName: row.VariantName,
		})
	}
	return out, nil
}

func (r *surfaceFormRepo) CategoryIDs(ctx context.Context, hits []domain.SurfaceFormHit) (map[string][]string, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	var groups, crops []string
	var commodities []int64
	for _, h := range hits {
		switch h.EntityKind {
		case domain.EntityKindCropGroup:
			groups = append(groups, h.EntityCode)
		case domain.EntityKindCrop:
			crops = append(crops, h.EntityCode)
		case domain.EntityKindCommodity:
			id, err := strconv.ParseInt(h.EntityCode, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("surfaceFormRepo.CategoryIDs: commodity code %q is not an id: %w", h.EntityCode, err)
			}
			commodities = append(commodities, id)
		}
	}
	out := map[string][]string{}
	add := func(kind domain.EntityKind, code, categoryID string) {
		k := domain.SurfaceFormHit{EntityKind: kind, EntityCode: code}.EntityKey()
		out[k] = append(out[k], categoryID)
	}
	if len(groups) > 0 {
		rows, err := q.CategoryIDsByGroupCodes(ctx, groups)
		if err != nil {
			return nil, fmt.Errorf("surfaceFormRepo.CategoryIDs groups: %w", err)
		}
		for _, row := range rows {
			add(domain.EntityKindCropGroup, row.GroupCode, row.CategoryID)
		}
	}
	if len(crops) > 0 {
		rows, err := q.CategoryIDsByCropCodes(ctx, crops)
		if err != nil {
			return nil, fmt.Errorf("surfaceFormRepo.CategoryIDs crops: %w", err)
		}
		for _, row := range rows {
			add(domain.EntityKindCrop, row.CropCode, row.CategoryID)
		}
	}
	if len(commodities) > 0 {
		rows, err := q.CategoryIDsByCommodityIDs(ctx, commodities)
		if err != nil {
			return nil, fmt.Errorf("surfaceFormRepo.CategoryIDs commodities: %w", err)
		}
		for _, row := range rows {
			add(domain.EntityKindCommodity, strconv.FormatInt(row.CommodityID, 10), row.CategoryID)
		}
	}
	return out, nil
}
