//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

// 管理後台貼文清單的狀態篩選，對真實 SQL 驗一次。
//
// 這一組的正確性全在述詞裡，Go 端看不出來：AdminList 收到的 status 字串原樣送進 SQL，
// 呼叫鏈上沒有任何一層會過濾。述詞若退回只問 hidden_at IS NULL（那一欄只記平台強制
// 下架，草稿與賣家自行下架都不寫它），下面前兩個 case 會把從未發布與剛被撤下的貼文
// 照樣算進管理員的「上架中」分頁——而那張列表上的每一列看起來都正常。
//
// 賣家側的對照組是 TestListMyPostsFilterFollowsPublishStatus，兩支必須同時成立：同一個
// root cause 曾只修了賣家側。
func TestAdminListPostsFilterFollowsPublishStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)
	const sellerID int64 = 2685914682
	const (
		publishedID   int64 = 9300000000000001
		draftID       int64 = 9300000000000002
		unpublishedID int64 = 9300000000000003
		deletedID     int64 = 9300000000000004
	)

	_, err := repo.Create(ctx, publishedID, sellerID, "已發布", "", nil, nil, domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, draftID, sellerID, "從未發布", "", nil, nil, domain.PostPublishStatusDraft, nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, unpublishedID, sellerID, "待下架", "", nil, nil, domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, deletedID, sellerID, "待刪除", "", nil, nil, domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)

	// listIDs 同時驗 AdminList 與 AdminCount：兩支述詞逐字一致是分頁的前提，各驗各的
	// 會讓「內容對了但總數多算」這種形狀溜過去（管理員看到 4 筆卻分成 2 頁）。
	listIDs := func(t *testing.T, status string) []int64 {
		t.Helper()
		rows, lerr := repo.AdminList(ctx, status, 50, 0)
		require.NoError(t, lerr)
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		total, cerr := repo.AdminCount(ctx, status)
		require.NoError(t, cerr)
		require.EqualValues(t, len(ids), total,
			"AdminListPosts 與 AdminCountPosts 的述詞必須逐字一致，否則分頁總數與內容對不上")
		return ids
	}

	t.Run("published excludes a post that was never published", func(t *testing.T) {
		ids := listIDs(t, "published")
		require.Contains(t, ids, publishedID)
		require.NotContains(t, ids, draftID,
			"草稿的 hidden_at 本來就是 NULL——用它當發布判準會讓管理員的「上架中」分頁列出未發布貼文，id=%d", draftID)
	})

	t.Run("published excludes a post the seller just unpublished", func(t *testing.T) {
		require.NoError(t, repo.SetPublishStatus(ctx, unpublishedID,
			domain.PostPublishStatusUnpublished, time.Now().UTC(), nil, nil))

		ids := listIDs(t, "published")
		require.NotContains(t, ids, unpublishedID,
			"賣家自行下架不寫 hidden_at（那是平台強制下架），述詞必須問 publish_status，id=%d", unpublishedID)
		require.Contains(t, ids, publishedID)
	})

	t.Run("published excludes a soft-deleted post", func(t *testing.T) {
		_, err := repo.SoftDelete(ctx, deletedID, sellerID)
		require.NoError(t, err)

		require.NotContains(t, listIDs(t, "published"), deletedID)
		require.Equal(t, []int64{deletedID}, listIDs(t, "deleted"))
	})

	t.Run("hidden still means platform takedown only", func(t *testing.T) {
		require.Empty(t, listIDs(t, "hidden"),
			"賣家自行下架（4403）不得被算成平台強制下架，兩者的申訴管道不同")

		require.NoError(t, repo.HidePost(ctx, publishedID, "違規"))
		require.Equal(t, []int64{publishedID}, listIDs(t, "hidden"))
		require.NotContains(t, listIDs(t, "published"), publishedID,
			"平台下架後不得再算進「上架中」")
	})

	t.Run("the unfiltered list still carries every state", func(t *testing.T) {
		require.ElementsMatch(t,
			[]int64{publishedID, draftID, unpublishedID, deletedID},
			listIDs(t, ""),
			"管理員的「全部」分頁是治理入口，草稿、已下架與已刪除都必須看得到")
	})
}
