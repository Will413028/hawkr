//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// 這是整條查詢的存亡測試。541 筆「待確認」email 正規化後就是空字串；
// 每個 OR 分支若少了守衛，這 3 筆全空鍵的列就會互相配對成 3 對，634 筆
// 規模下則是 O(n²) 的假配對爆炸——與遷移時佔位值黑名單同一個地雷。守衛
// 本身（獨立成縮排區塊是因為 Go 的文件註解格式化會把行內相鄰的兩個單
// 引號吃成一個排版引號）：
//
//	<> ''
func TestListDuplicatePairsIgnoresEmptyMatchKeys(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	for _, name := range []string{"空鍵甲", "空鍵乙", "空鍵丙"} {
		p := newTestSellerProspect(name, "水產", "高雄")
		p.MatchKeys = domain.BuildProspectMatchKeys(domain.ProspectContactInput{Email: "待確認"})
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	pairs, err := repo.ListDuplicatePairs(ctx)
	require.NoError(t, err)
	require.Empty(t, pairs, "全空 match key 不得產生任何配對")
}

func TestListDuplicatePairsFindsSharedKeyOnce(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	shared := domain.BuildProspectMatchKeys(domain.ProspectContactInput{Email: "same@example.com"})
	for _, name := range []string{"漁夫鮮撈", "梓官區漁會"} {
		p := newTestSellerProspect(name, "水產", "高雄")
		p.MatchKeys = shared
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	pairs, err := repo.ListDuplicatePairs(ctx)
	require.NoError(t, err)
	require.Len(t, pairs, 1, "一對只能出現一次（a.id < b.id）")
	require.Less(t, pairs[0].Left.ID, pairs[0].Right.ID)
}

// 已封存或已入駐的列不該出現在待處理的重複清單上。
func TestListDuplicatePairsSkipsNonActive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	shared := domain.BuildProspectMatchKeys(domain.ProspectContactInput{Email: "same@example.com"})

	a := newTestSellerProspect("在營運", "水產", "高雄")
	a.MatchKeys = shared
	_, err := repo.Create(ctx, a)
	require.NoError(t, err)

	b := newTestSellerProspect("已封存", "水產", "高雄")
	b.MatchKeys = shared
	b.Status = "ARCHIVED"
	_, err = repo.Create(ctx, b)
	require.NoError(t, err)

	pairs, err := repo.ListDuplicatePairs(ctx)
	require.NoError(t, err)
	require.Empty(t, pairs)
}
