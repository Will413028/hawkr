//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func testSellerProspectID() func() int64 {
	var n int64 = 700_000_000
	return func() int64 {
		n++
		return n
	}
}

func newTestSellerProspect(displayName, category, region string) domain.SellerProspect {
	return domain.SellerProspect{
		DisplayName:    displayName,
		VendorType:     "business",
		Status:         "ACTIVE",
		OutreachStatus: "NOT_CONTACTED",
		Category:       category,
		Region:         region,
		ProductSummary: "測試商品摘要",
		Positioning:    "測試定位描述",
	}
}

func TestSellerProspectRepository_Search_OnlyReturnsActiveStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	active, err := repo.Create(ctx, newTestSellerProspect("活躍業者", "水果", "台南"))
	require.NoError(t, err)

	archived, err := repo.Create(ctx, newTestSellerProspect("已封存業者", "水果", "台南"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE seller_prospects SET status = 'ARCHIVED' WHERE id = $1`, archived.ID)
	require.NoError(t, err)

	converting, err := repo.Create(ctx, newTestSellerProspect("已入駐業者", "水果", "台南"))
	require.NoError(t, err)
	rows, err := repo.MarkConverted(ctx, converting.ID, 555001, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows, "MarkConverted 應該成功轉換這一筆")

	got, err := repo.Search(ctx, domain.ProspectSearchParams{Category: "水果", Region: "台南", Limit: 10})
	require.NoError(t, err)

	gotIDs := make([]int64, len(got))
	for i, hit := range got {
		gotIDs[i] = hit.Prospect.ID
	}
	require.ElementsMatch(t, []int64{active.ID}, gotIDs,
		"只有 ACTIVE 的業者該出現；ARCHIVED 與 CONVERTED 都必須被 status = 'ACTIVE' 擋下")
}

func TestSellerProspectRepository_Search_ZeroLimitDoesNotCapAtTen(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	for i := 0; i < 11; i++ {
		_, err := repo.Create(ctx, newTestSellerProspect(fmt.Sprintf("業者-%d", i), "水果", "台南"))
		require.NoError(t, err)
	}

	got, err := repo.Search(ctx, domain.ProspectSearchParams{Category: "水果", Limit: 0})
	require.NoError(t, err)
	require.Len(t, got, 11, "Limit<=0 的服務端預設必須遠高於 10，否則第 11 筆會被 DB 端砍掉，"+
		"永遠不會有機會被 ai-service 那端的排序看到")
}

func TestSellerProspectRepository_Search_WeightedOrNotStrictAnd(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	seed := newTestSellerProspect("水梨果園", "水果", "台南")

	seed.ProductSummary = "水梨禮盒"
	seed.Positioning = "在地小農直送"
	created, err := repo.Create(ctx, seed)
	require.NoError(t, err)

	got, err := repo.Search(ctx, domain.ProspectSearchParams{
		Category: "水果", Region: "台南", Keywords: "西瓜", Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, got, 1,
		"category 與 region 都命中，即使 keywords 沒有字面命中，加權 OR 下仍必須整筆入選")

	hit := got[0]
	require.Equal(t, created.ID, hit.Prospect.ID)
	require.True(t, hit.HitCategory, "category 命中")
	require.True(t, hit.HitRegion, "region 命中")
	require.False(t, hit.HitKeywords, "關鍵字沒有字面出現在任何欄位，hit_keywords 必須是 false")
}

func TestSellerProspectRepository_Search_RegionSubstringMatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	seed := newTestSellerProspect("東山龍眼農場", "水果", "台南東山")
	created, err := repo.Create(ctx, seed)
	require.NoError(t, err)

	got, err := repo.Search(ctx, domain.ProspectSearchParams{Region: "台南", Limit: 10})
	require.NoError(t, err)
	require.Len(t, got, 1, "泛稱地區「台南」必須命中細粒度值「台南東山」——ILIKE 部分比對，不是精確比對")
	require.Equal(t, created.ID, got[0].Prospect.ID)
	require.True(t, got[0].HitRegion)
}

func TestSellerProspectRepository_Search_CategorySubstringMatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	seed := newTestSellerProspect("禮盒水果行", "水果／禮盒", "台南")
	created, err := repo.Create(ctx, seed)
	require.NoError(t, err)

	got, err := repo.Search(ctx, domain.ProspectSearchParams{Category: "水果", Limit: 10})
	require.NoError(t, err)
	require.Len(t, got, 1, "單詞查詢「水果」必須命中複合詞分類「水果／禮盒」——ILIKE 部分比對，不是精確比對")
	require.Equal(t, created.ID, got[0].Prospect.ID)
	require.True(t, got[0].HitCategory)
}

func tagProspect(ctx context.Context, t *testing.T, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, prospectID int64, categoryIDs ...string) {
	t.Helper()
	for _, id := range categoryIDs {
		_, err := pool.Exec(ctx, `INSERT INTO seller_prospect_categories (prospect_id, category_id) VALUES ($1, $2)`, prospectID, id)
		require.NoError(t, err)
	}
}

func TestSellerProspectRepository_Search_CategoryIDsMatchStructuredTags(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	pork, err := repo.Create(ctx, newTestSellerProspect("究好豬", "豬肉", ""))
	require.NoError(t, err)
	tagProspect(ctx, t, pool, pork.ID, "198")

	meat, err := repo.Create(ctx, newTestSellerProspect("台畜", "肉品", ""))
	require.NoError(t, err)
	tagProspect(ctx, t, pool, meat.ID, "4")

	veg, err := repo.Create(ctx, newTestSellerProspect("臺灣九號", "蔬菜／毛豆", ""))
	require.NoError(t, err)
	tagProspect(ctx, t, pool, veg.ID, "1")

	hits, err := repo.Search(ctx, domain.ProspectSearchParams{CategoryIDs: []string{"2008", "198", "4"}})
	require.NoError(t, err)

	got := map[string]bool{}
	for _, h := range hits {
		got[h.Prospect.DisplayName] = h.HitCategoryIDs
	}
	require.True(t, got["究好豬"], "掛 L2 豬肉(198)，在祖先鏈裡")
	require.True(t, got["台畜"], "掛 L1 肉類(4)，也在祖先鏈裡")
	require.NotContains(t, got, "臺灣九號", "掛蔬菜(1) 不在祖先鏈裡，其他條件也沒給，不該命中")
}

func TestSellerProspectRepository_Search_EmptyCategoryIDsKeepsLegacyBehaviour(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	tagged, err := repo.Create(ctx, newTestSellerProspect("究好豬", "豬肉", ""))
	require.NoError(t, err)
	tagProspect(ctx, t, pool, tagged.ID, "198")

	_, err = repo.Create(ctx, newTestSellerProspect("臺灣九號", "蔬菜", ""))
	require.NoError(t, err)

	all, err := repo.Search(ctx, domain.ProspectSearchParams{})
	require.NoError(t, err)
	require.Len(t, all, 2, "四個條件皆空仍是全撈，加了 category_ids 不改這條")
	for _, h := range all {
		require.False(t, h.HitCategoryIDs, "沒給 category_ids 時這個旗標必須是 false")
	}

	byCategory, err := repo.Search(ctx, domain.ProspectSearchParams{Category: "豬肉"})
	require.NoError(t, err)
	require.Len(t, byCategory, 1)
	require.Equal(t, "究好豬", byCategory[0].Prospect.DisplayName)
	require.True(t, byCategory[0].HitCategory)
	require.False(t, byCategory[0].HitCategoryIDs)
}
