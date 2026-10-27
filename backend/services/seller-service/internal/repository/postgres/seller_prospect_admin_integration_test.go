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

// 未指定 status 時預設只回 ACTIVE：admin 的日常工作面是營運中的名單，
// 封存與已入駐要明確要求才出現。
func TestListForAdminDefaultsToActive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	active := newTestSellerProspect("在營運業者", "水產", "高雄")
	_, err := repo.Create(ctx, active)
	require.NoError(t, err)

	archived := newTestSellerProspect("已封存業者", "水產", "高雄")
	archived.Status = "ARCHIVED"
	_, err = repo.Create(ctx, archived)
	require.NoError(t, err)

	rows, total, err := repo.ListForAdmin(ctx, domain.ProspectAdminListParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, "在營運業者", rows[0].DisplayName)
}

func TestListForAdminFiltersAndPaginates(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	for _, name := range []string{"甲水產", "乙水產", "丙農產"} {
		p := newTestSellerProspect(name, "水產", "高雄")
		if name == "丙農產" {
			p.Category = "農產"
		}
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	rows, total, err := repo.ListForAdmin(ctx, domain.ProspectAdminListParams{
		Category: "水產", Page: 1, PageSize: 1,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "total 是符合條件的總數，不是本頁筆數")
	require.Len(t, rows, 1)

	rows, _, err = repo.ListForAdmin(ctx, domain.ProspectAdminListParams{
		Keyword: "農產", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "丙農產", rows[0].DisplayName)
}

// total 靠 COUNT(*) OVER() 搭本頁結果的便車：翻到超出範圍的頁碼時
// LIMIT／OFFSET 讓查詢回零列，窗口函式沒有列可附著。這裡驗證 repository
// 在那種情況下仍回真正符合條件的筆數，不是讓 total 隨著空頁跌回 0——
// 那會讓前端的 pager 誤判成「其實沒有資料」而收合。
func TestListForAdminReportsTotalOnOutOfRangePage(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	for _, name := range []string{"甲水產", "乙水產"} {
		_, err := repo.Create(ctx, newTestSellerProspect(name, "水產", "高雄"))
		require.NoError(t, err)
	}

	rows, total, err := repo.ListForAdmin(ctx, domain.ProspectAdminListParams{
		Page: 99, PageSize: 20,
	})
	require.NoError(t, err)
	require.Empty(t, rows, "第 99 頁超出範圍，本頁沒有資料")
	require.EqualValues(t, 2, total, "total 仍要回真正符合條件的筆數，不能因為本頁是空的就變成 0")
}

// status 傳明確值才會走 SQL 的 CASE WHEN ELSE 分支——這是 admin 唯一看得到
// ARCHIVED 列（含每一筆被合併掉的列）的路徑；空字串預設只回 ACTIVE 的
// THEN 分支已經由 TestListForAdminDefaultsToActive 覆蓋。
func TestListForAdminExplicitStatusFiltersArchived(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	active := newTestSellerProspect("在營運業者", "水產", "高雄")
	_, err := repo.Create(ctx, active)
	require.NoError(t, err)

	archived := newTestSellerProspect("已封存業者", "水產", "高雄")
	archived.Status = "ARCHIVED"
	_, err = repo.Create(ctx, archived)
	require.NoError(t, err)

	rows, total, err := repo.ListForAdmin(ctx, domain.ProspectAdminListParams{
		Status: "ARCHIVED", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, "已封存業者", rows[0].DisplayName)
}

// 更新聯絡方式必須連帶重算 match keys。舊 keys 留著會讓比對鍵與顯示值
// 長期分岔——重複清單與建檔查重都會據此判斷「是不是同一家」。
func TestUpdateRecomputesMatchKeys(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	p := newTestSellerProspect("改信箱業者", "水產", "高雄")
	p.PublicEmail = "OLD@Example.com "
	p.MatchKeys = domain.BuildProspectMatchKeys(domain.ProspectContactInput{Email: p.PublicEmail})
	created, err := repo.Create(ctx, p)
	require.NoError(t, err)
	require.Equal(t, "old@example.com", created.MatchKeys.Email)

	created.PublicEmail = "NEW@Example.com"
	created.MatchKeys = domain.BuildProspectMatchKeys(domain.ProspectContactInput{Email: created.PublicEmail})
	updated, err := repo.Update(ctx, created)
	require.NoError(t, err)
	require.Equal(t, "new@example.com", updated.MatchKeys.Email)
}

// 帶 merged_into 的列不可還原成 ACTIVE：那要求同時撤銷一次合併
// （包含把邀請搬回去），本輪不提供合併的反向操作。
func TestSetArchivedRejectsUnarchivingMergedRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winner, err := repo.Create(ctx, newTestSellerProspect("贏家", "水產", "高雄"))
	require.NoError(t, err)
	loser, err := repo.Create(ctx, newTestSellerProspect("輸家", "水產", "高雄"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET status = 'ARCHIVED', merged_into_prospect_id = $1 WHERE id = $2`,
		winner.ID, loser.ID)
	require.NoError(t, err)

	rows, err := repo.SetArchived(ctx, loser.ID, false, nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, rows, "被合併的列不得被還原")
}
