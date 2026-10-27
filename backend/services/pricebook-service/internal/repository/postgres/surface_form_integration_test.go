//go:build integration

package postgres_test

// 這支守的是單元測試看不見的三件事：generated column 真的算對、trigram LIKE 真的
// 命中、backfill migration 真的把既有實體與 seed 搬進來。fake 不經過 driver，
// 對這三件事零鑑別力（2026-09-14 jsonb override 的教訓）。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func TestSurfaceForm_GeneratedNormAndTiers(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	repo := postgres.NewSurfaceFormRepository(pool)

	_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{ID: 700000000000000301, GroupCode: "G1", Name: "甘藍"})
	require.NoError(t, err)

	require.NoError(t, postgres.RegisterSurfaceForms(ctx, q, []domain.SurfaceFormRegistration{
		{EntityKind: domain.EntityKindCropGroup, EntityCode: "G1", FormRaw: "甘　藍", FormKind: domain.FormKindCanonical},
		{EntityKind: domain.EntityKindCropGroup, EntityCode: "G1", FormRaw: "臺灣甘藍", FormKind: domain.FormKindAlias},
	}))

	// generated column：全形空白被去掉、臺→台
	rows, err := pool.Query(ctx, `SELECT form_norm FROM priceable_surface_form WHERE entity_code = 'G1' ORDER BY form_norm`)
	require.NoError(t, err)
	var norms []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		norms = append(norms, n)
	}
	rows.Close()
	require.Equal(t, []string{"台灣甘藍", "甘藍"}, norms)

	// tier 0：查詢也被同一段運算式正規化（「甘 藍」→「甘藍」）
	hits, err := repo.Search(ctx, "甘 藍", 10)
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	require.Equal(t, int32(0), hits[0].Tier)
	require.Equal(t, "甘藍", hits[0].GroupName, "LEFT JOIN 要把大類名帶出來")

	// tier 2：子串
	hits, err = repo.Search(ctx, "灣甘", 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, int32(2), hits[0].Tier)
	require.Equal(t, "臺灣甘藍", hits[0].FormRaw)

	// 空查詢零列（usecase 另走 ListAliases）
	hits, err = repo.Search(ctx, "   ", 10)
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestSurfaceForm_RegisterReplacesSameKind(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	q := gen.New(pool)
	repo := postgres.NewSurfaceFormRepository(pool)

	_, err := pool.Exec(ctx, `INSERT INTO commodity (id, source, commodity_code, name, price_layer, unit) VALUES (7, 'S', 'C', '舊名', 'retail', 'kg')`)
	require.NoError(t, err)

	reg := func(name string) error {
		return postgres.RegisterSurfaceForms(ctx, q, []domain.SurfaceFormRegistration{
			{EntityKind: domain.EntityKindCommodity, EntityCode: "7", FormRaw: name, FormKind: domain.FormKindCanonical},
		})
	}
	require.NoError(t, reg("舊名"))
	// 模擬 ingest 的改名：先 upsert 實體本體，再同交易重登記 surface form
	_, err = pool.Exec(ctx, `UPDATE commodity SET name = '新名' WHERE id = 7`)
	require.NoError(t, err)
	require.NoError(t, reg("新名"))

	hits, err := repo.Search(ctx, "舊名", 10)
	require.NoError(t, err)
	require.Empty(t, hits, "改名後舊名不該還搜得到")
	hits, err = repo.Search(ctx, "新名", 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "新名", hits[0].CommodityName)
}

func backfillMigrationPath(t *testing.T) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("..", "..", "..", "db", "migrations", "*_backfill_priceable_surface_form.sql"))
	require.NoError(t, err)
	require.Len(t, matches, 1, "backfill migration 應恰好一支")
	return matches[0]
}

func TestSurfaceForm_BackfillMigrationPopulatesExistingEntitiesAndSeed(t *testing.T) {
	// setupTestDB 會 apply 全部 migrations，但 backfill 在空表上什麼都不做。
	// 所以：先塞實體，再重跑那一支 backfill 檔（它可重跑）。
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx, `
		INSERT INTO crop_group (id, group_code, name) VALUES (1, 'G-CAB', '甘藍'), (2, 'G-DUP', '芥菜'), (3, 'G-DUP2', '芥菜');
		INSERT INTO crop (id, crop_code, group_id, name, variant_name) VALUES (10, 'LA1', 1, '甘藍-初秋', '初秋');
		INSERT INTO commodity (id, source, commodity_code, name, price_layer, unit) VALUES (7, 'MOA_PORK', 'SpecPig', '毛豬（規格豬）', 'live_animal', 'kg');
	`)
	require.NoError(t, err)
	sql, err := os.ReadFile(backfillMigrationPath(t))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)

	repo := postgres.NewSurfaceFormRepository(pool)
	mustHit := func(query string, kind domain.EntityKind, code string, formKind domain.FormKind) {
		t.Helper()
		hits, err := repo.Search(ctx, query, 10)
		require.NoError(t, err)
		for _, h := range hits {
			if h.EntityKind == kind && h.EntityCode == code && h.FormKind == formKind {
				return
			}
		}
		t.Fatalf("%q 沒有命中 %s/%s（%s）：%+v", query, kind, code, formKind, hits)
	}
	mustHit("甘藍", domain.EntityKindCropGroup, "G-CAB", domain.FormKindCanonical)
	mustHit("甘藍-初秋", domain.EntityKindCrop, "LA1", domain.FormKindCanonical)
	mustHit("初秋", domain.EntityKindCrop, "LA1", domain.FormKindVariant)
	mustHit("毛豬", domain.EntityKindCommodity, "7", domain.FormKindCanonical)
	mustHit("高麗菜", domain.EntityKindCropGroup, "G-CAB", domain.FormKindAlias) // seed CSV 第一列

	// 同名大類（芥菜 ×2）：兩個 CANONICAL 都在（ILIKE 舊路徑也回兩個，契約不變）
	hits, err := repo.Search(ctx, "芥菜", 10)
	require.NoError(t, err)
	require.Len(t, hits, 2)

	// 可重跑：第二次執行不得報錯、不得增列
	var before, after int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM priceable_surface_form`).Scan(&before))
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM priceable_surface_form`).Scan(&after))
	require.Equal(t, before, after)
}
