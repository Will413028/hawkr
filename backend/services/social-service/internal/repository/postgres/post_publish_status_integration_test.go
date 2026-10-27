//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

// 56 §6.1.9 的發布／下架命令與 §7.6.1 的 T0 排程建立，對真實 SQL 驗一次。
//
// 這一組的正確性全在述詞與 CASE 裡，Go 端看不出來：CAS 是否真的擋住、published_at 在
// 哪個方向被重寫、以及最要緊的——立即發布有沒有把還沒執行的排程一併結束。最後一項若漏
// 掉，worker 到點會再發一次（第二筆稽核、第二則賣家通知），而三條收斂語句一條都不會攔，
// 因為那則貼文既沒被刪也沒被下架。
func TestPostPublishStatusCommands(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)
	const sellerID int64 = 2685914682
	v := func(n int64) *int64 { return &n }

	t.Run("publish writes status, publish time and a new version", func(t *testing.T) {
		const id int64 = 9100000000000001
		_, err := repo.Create(ctx, id, sellerID, "草稿", "", nil, nil, domain.PostPublishStatusDraft, nil)
		require.NoError(t, err)

		now := time.Now().UTC().Truncate(time.Second)
		require.NoError(t, repo.SetPublishStatus(ctx, id, domain.PostPublishStatusPublished, now, v(0), nil))

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusPublished, post.PublishStatus)
		require.NotNil(t, post.PublishedAt, "發布必須寫下發布時點")
		require.EqualValues(t, 1, post.Version, "代行寫入必須 bump version，否則同一個 expectedVersion 可以無限次通過 CAS")
	})

	t.Run("a stale expectedVersion changes nothing", func(t *testing.T) {
		const id int64 = 9100000000000002
		_, err := repo.Create(ctx, id, sellerID, "草稿", "", nil, nil, domain.PostPublishStatusDraft, nil)
		require.NoError(t, err)

		err = repo.SetPublishStatus(ctx, id, domain.PostPublishStatusPublished,
			time.Now().UTC(), v(7), nil)
		require.ErrorIs(t, err, domain.ErrVersionConflict)

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusDraft, post.PublishStatus,
			"CAS 失敗必須是完全沒有寫入，不是寫一半")
		require.EqualValues(t, 0, post.Version)
	})

	t.Run("unpublish keeps the publish time", func(t *testing.T) {
		const id int64 = 9100000000000003
		_, err := repo.Create(ctx, id, sellerID, "已發布", "", nil, nil, domain.PostPublishStatusPublished, nil)
		require.NoError(t, err)
		before, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, before.PublishedAt)

		require.NoError(t, repo.SetPublishStatus(ctx, id, domain.PostPublishStatusUnpublished,
			time.Now().UTC().Add(time.Hour), v(0), nil))

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusUnpublished, post.PublishStatus)
		require.NotNil(t, post.PublishedAt, "下架不清 published_at")
		require.WithinDuration(t, *before.PublishedAt, *post.PublishedAt, time.Second,
			"published_at 記的是發布時點，下架不得改寫它——「卡住的排程」偵測述詞讀的就是這一欄")
	})

	t.Run("schedule writes the pending row without publishing it", func(t *testing.T) {
		const id int64 = 9100000000000004
		const sessionID int64 = 7100000000000001
		_, err := repo.Create(ctx, id, sellerID, "待排程", "", nil, nil, domain.PostPublishStatusDraft, nil)
		require.NoError(t, err)

		at := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
		require.NoError(t, repo.SchedulePublish(ctx, id, at, sessionID, time.Now().UTC(), v(0)))

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusDraft, post.PublishStatus,
			"排程不得改動 publish_status：pending 的列必須是尚未計數的狀態，worker 的終寫才敢無條件 +1 標籤計數")
		require.Equal(t, domain.ScheduleStatePending, *post.ScheduleState)
		require.NotNil(t, post.ScheduledPublishAt)
		require.WithinDuration(t, at, *post.ScheduledPublishAt, time.Second)
		require.EqualValues(t, sessionID, *post.ScheduledBySessionID,
			"worker 靠這一欄重建授權，缺了就是一列永遠發不出去的 pending")
		require.EqualValues(t, 0, post.ScheduleRetryCount)
	})

	t.Run("scheduling a published post is refused", func(t *testing.T) {
		const id int64 = 9100000000000005
		_, err := repo.Create(ctx, id, sellerID, "已發布", "", nil, nil, domain.PostPublishStatusPublished, nil)
		require.NoError(t, err)

		err = repo.SchedulePublish(ctx, id, time.Now().UTC().Add(2*time.Hour), 7100000000000001,
			time.Now().UTC(), v(0))
		require.ErrorIs(t, err, domain.ErrVersionConflict)

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.Nil(t, post.ScheduleState,
			"已發布的貼文不得進入 pending：worker 到點會對一則已計數的貼文再 +1 一次標籤計數")
	})

	// The discriminating case for the CASE clauses in SetPostPublishStatus:
	// without them the row keeps schedule_state='pending' and the worker publishes
	// it a second time — a second POST_PUBLISH audit row and a second seller
	// notification, with none of the three convergence statements catching it.
	t.Run("publishing now ends a pending schedule", func(t *testing.T) {
		const id int64 = 9100000000000006
		_, err := repo.Create(ctx, id, sellerID, "先排程再改立即發布", "", nil, nil, domain.PostPublishStatusDraft, nil)
		require.NoError(t, err)

		at := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
		require.NoError(t, repo.SchedulePublish(ctx, id, at, 7100000000000001, time.Now().UTC(), v(0)))
		require.NoError(t, repo.SetPublishStatus(ctx, id, domain.PostPublishStatusPublished,
			time.Now().UTC(), v(1), nil))

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, domain.ScheduleStateDone, *post.ScheduleState)
		require.Nil(t, post.ScheduleLeaseOwner)
		require.Nil(t, post.ScheduleLeaseExpiresAt)

		// Asserted through the worker's own candidate query rather than by reading
		// the column back: what matters is that the worker will not pick it up.
		sched := postgres.NewScheduledPostRepository(pool)
		due, err := sched.ListDue(ctx, at.Add(time.Minute), 24, 50)
		require.NoError(t, err)
		require.NotContains(t, due, id,
			"排程被立即發布取代之後，worker 到點仍會再發一次——賣家收到第二則通知，稽核多一列 POST_PUBLISH")
	})

	t.Run("the seller's own path writes without a version predicate", func(t *testing.T) {
		const id int64 = 9100000000000007
		_, err := repo.Create(ctx, id, sellerID, "賣家自己下架", "", nil, nil, domain.PostPublishStatusPublished, nil)
		require.NoError(t, err)
		bumpVersion(ctx, t, pool, id) // 版本已被別人推進，但本路徑沒有 expectedVersion

		require.NoError(t, repo.SetPublishStatus(ctx, id, domain.PostPublishStatusUnpublished,
			time.Now().UTC(), nil, nil))

		post, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusUnpublished, post.PublishStatus)
	})
}

// 賣家清單的篩選述詞與上面那組狀態寫入碰同一個 publish_status 欄，而它們各自的測試
// 都會綠：SetPublishStatus 的測試只讀回該欄，ListMy 的測試（若只餵已發布資料）也永遠
// 看不出差別。缺的是同時經過兩者的這一支——「賣家自己下架之後，篩選還看不看得到？」
//
// 述詞若退回只問 hidden_at IS NULL（那一欄記的是平台強制下架，賣家自行下架不寫它），
// 下面第二個 case 會把剛下架的貼文照樣算成「已發布」。
func TestListMyPostsFilterFollowsPublishStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)
	const sellerID int64 = 2685914682
	const (
		publishedID   int64 = 9200000000000001
		draftID       int64 = 9200000000000002
		unpublishedID int64 = 9200000000000003
	)

	_, err := repo.Create(ctx, publishedID, sellerID, "已發布", "", nil, nil, domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, draftID, sellerID, "從未發布", "", nil, nil, domain.PostPublishStatusDraft, nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, unpublishedID, sellerID, "待下架", "", nil, nil, domain.PostPublishStatusPublished, nil)
	require.NoError(t, err)

	listIDs := func(t *testing.T, status string) []int64 {
		t.Helper()
		rows, lerr := repo.ListMy(ctx, sellerID, status, 50, 0)
		require.NoError(t, lerr)
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		total, cerr := repo.CountMy(ctx, sellerID, status)
		require.NoError(t, cerr)
		require.EqualValues(t, len(ids), total,
			"ListMyPosts 與 CountMyPosts 的述詞必須逐字一致，否則分頁總數與內容對不上")
		return ids
	}

	t.Run("published excludes a post that was never published", func(t *testing.T) {
		ids := listIDs(t, "published")
		require.Contains(t, ids, publishedID)
		require.NotContains(t, ids, draftID,
			"草稿的 hidden_at 本來就是 NULL——用它當發布判準會把未發布貼文報成已發布")
	})

	t.Run("published excludes a post the seller just unpublished", func(t *testing.T) {
		require.NoError(t, repo.SetPublishStatus(ctx, unpublishedID,
			domain.PostPublishStatusUnpublished, time.Now().UTC(), nil, nil))

		ids := listIDs(t, "published")
		require.NotContains(t, ids, unpublishedID,
			"賣家自行下架不寫 hidden_at（那是平台強制下架），述詞必須問 publish_status")
		require.Contains(t, ids, publishedID)
	})

	t.Run("the unfiltered list still carries every state", func(t *testing.T) {
		ids := listIDs(t, "")
		require.ElementsMatch(t, []int64{publishedID, draftID, unpublishedID}, ids,
			"賣家看自己的清單不過濾發布狀態——草稿與已下架都必須看得到，否則他救不回自己的貼文")
	})

	t.Run("hidden still means platform takedown only", func(t *testing.T) {
		require.Empty(t, listIDs(t, "hidden"),
			"賣家自行下架不得被算成平台強制下架，兩者的申訴管道不同")

		require.NoError(t, repo.HidePost(ctx, publishedID, "違規"))
		require.Equal(t, []int64{publishedID}, listIDs(t, "hidden"))
	})
}

// bumpVersion moves a row's version the way any other writer would, so a test
// can set up "someone else changed this row" without going through a repository
// method that also changes what is under test.
func bumpVersion(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE cms_post SET version = version + 1 WHERE id = $1`, id)
	require.NoError(t, err)
}
