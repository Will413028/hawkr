//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// 56 §5.1.5／§7.6.2 的述詞全部只在真實資料庫裡才成立：收斂語句的互斥性、候選 SELECT 的
// 六個條件、claim 與終寫兩個 CAS 的先到先贏，沒有一項是可以靠假件證明的。

const scheduleWindowHours = 24

func insertScheduledPost(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64, scheduledAt time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO cms_post (id, seller_id, content, publish_status, scheduled_publish_at,
		                      schedule_state, scheduled_by_session_id)
		VALUES ($1, 2685914682, '排程貼文', 4401, $2, 'pending', 7700000000000001)`, id, scheduledAt)
	require.NoError(t, err)
}

// readPost 走裸 SQL 而不是 gen.GetPost：後者過濾 deleted_at（「已刪除＝不存在」），
// 而本檔一半的斷言正是在檢查已刪除的列被收斂成什麼。
func readPost(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) gen.CmsPost {
	t.Helper()
	var row gen.CmsPost
	err := pool.QueryRow(ctx, `
		SELECT id, version, publish_status, published_at, schedule_state,
		       schedule_lease_owner, schedule_lease_expires_at, schedule_next_attempt_at,
		       schedule_retry_count
		  FROM cms_post WHERE id = $1`, id).
		Scan(&row.ID, &row.Version, &row.PublishStatus, &row.PublishedAt, &row.ScheduleState,
			&row.ScheduleLeaseOwner, &row.ScheduleLeaseExpiresAt, &row.ScheduleNextAttemptAt,
			&row.ScheduleRetryCount)
	require.NoError(t, err)
	return row
}

func TestScheduledPostPublish_HappyPath(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000001
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Minute))

	due, err := repo.ListDue(ctx, now, scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Equal(t, []int64{postID}, due)

	claimed, err := repo.Claim(ctx, postID, "pod-a", now, now.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.NotNil(t, claimed.ScheduleLeaseOwner)
	require.Equal(t, "pod-a", *claimed.ScheduleLeaseOwner)
	require.EqualValues(t, domain.PostPublishStatusDraft, claimed.PublishStatus,
		"claim 只取租約，不得改動業務欄位")

	var hookRan bool
	ok, err := repo.Publish(ctx, postID, "pod-a", domain.PostPublishStatusPublished, now,
		func(context.Context, gen.DBTX) error { hookRan = true; return nil })
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, hookRan, "CAS 贏了就必須在同一個交易內跑計數 hook")

	after := readPost(ctx, t, pool, postID)
	require.EqualValues(t, domain.PostPublishStatusPublished, after.PublishStatus)
	require.NotNil(t, after.PublishedAt)
	require.Equal(t, domain.ScheduleStateDone, *after.ScheduleState)
	require.Nil(t, after.ScheduleLeaseOwner, "終寫必須清掉租約")
	require.EqualValues(t, claimed.Version, after.Version,
		"排程不覆蓋內容，因此不得 bump version——bump 了會讓賣家手上的 expectedVersion 平白失效")

	// 已發布的列不再是候選。
	due, err = repo.ListDue(ctx, now, scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Empty(t, due)
}

// 賣家自刪與平台強制下架各自留下不同的終態值，且都不發布。
// 只寫一個籠統的 cancelled 會在申訴成功（hidden_at 被 RestorePost 清成 NULL）之後
// 反查不出取消原因。
func TestScheduledPostConverge_DeletionAndGovernanceAreDistinct(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()

	const deletedID int64 = 9100000000000011
	const hiddenID int64 = 9100000000000012
	const bothID int64 = 9100000000000013
	for _, id := range []int64{deletedID, hiddenID, bothID} {
		insertScheduledPost(ctx, t, pool, id, now.Add(-time.Minute))
	}
	_, err := pool.Exec(ctx, `UPDATE cms_post SET deleted_at = NOW() WHERE id = ANY($1)`,
		[]int64{deletedID, bothID})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE cms_post SET hidden_at = NOW() WHERE id = ANY($1)`,
		[]int64{hiddenID, bothID})
	require.NoError(t, err)

	// 候選 SELECT 已經看不到這三列了——但收斂仍不可省：被擋下的列會停在 pending 被每個
	// tick 重撈，最後由逾窗口那條轉 expired，讓逾期告警為一個完全預期的情形而響。
	due, err := repo.ListDue(ctx, now, scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Empty(t, due)

	n, err := repo.ConvergeDeleted(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 2, n, "已刪除的兩列（含同時被下架的那列）都歸刪除")

	n, err = repo.ConvergeHidden(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, n,
		"下架那條帶 deleted_at IS NULL，否則同一列會被兩條各記一次，OBS10 的 reason 分佈失真")

	require.Equal(t, domain.ScheduleStateCancelledByDeletion, *readPost(ctx, t, pool, deletedID).ScheduleState)
	require.Equal(t, domain.ScheduleStateCancelledByGovernance, *readPost(ctx, t, pool, hiddenID).ScheduleState)
	require.Equal(t, domain.ScheduleStateCancelledByDeletion, *readPost(ctx, t, pool, bothID).ScheduleState,
		"deleted_at ≻ hidden_at：既有程式碼對兩欄的排序如此，終態值必須跟著")

	for _, id := range []int64{deletedID, hiddenID, bothID} {
		require.EqualValues(t, domain.PostPublishStatusDraft, readPost(ctx, t, pool, id).PublishStatus,
			"取消一律不發布")
	}
}

// 收斂語句刻意不帶租約守衛：正被某個 pod 持有的失效列照樣要被推進終態，
// 否則它會停在 pending 等租約到期，而那個 pod 隨後照樣把它發出去。
func TestScheduledPostConverge_IgnoresHeldLease(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000021
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Minute))

	claimed, err := repo.Claim(ctx, postID, "pod-a", now, now.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)
	require.NotNil(t, claimed)

	_, err = pool.Exec(ctx, `UPDATE cms_post SET deleted_at = NOW() WHERE id = $1`, postID)
	require.NoError(t, err)

	n, err := repo.ConvergeDeleted(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, n, "租約在別人手上不是跳過收斂的理由")

	// 第四件讀時防禦：持有租約的 pod 到了終寫仍會因述詞不成立而拿到 rows = 0。
	ok, err := repo.Publish(ctx, postID, "pod-a", domain.PostPublishStatusPublished, now, nil)
	require.NoError(t, err)
	require.False(t, ok, "claim 到終寫之間被刪除的列必須放棄發布——這個窗口在單副本下同樣存在")
	require.EqualValues(t, domain.PostPublishStatusDraft, readPost(ctx, t, pool, postID).PublishStatus)
}

// claim 到終寫之間才被刪除／下架：前三件防禦都只防到 claim 當下，這一段只有終寫守得住。
func TestScheduledPostPublish_TerminalCASGuardsPostClaimDeletion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()

	for name, mutate := range map[string]string{
		"deleted": `UPDATE cms_post SET deleted_at = NOW() WHERE id = $1`,
		"hidden":  `UPDATE cms_post SET hidden_at = NOW() WHERE id = $1`,
	} {
		t.Run(name, func(t *testing.T) {
			id := int64(9100000000000030) + int64(len(name))
			insertScheduledPost(ctx, t, pool, id, now.Add(-time.Minute))

			claimed, err := repo.Claim(ctx, id, "pod-a", now, now.Add(120*time.Second), scheduleWindowHours)
			require.NoError(t, err)
			require.NotNil(t, claimed)

			_, err = pool.Exec(ctx, mutate, id)
			require.NoError(t, err)

			ok, err := repo.Publish(ctx, id, "pod-a", domain.PostPublishStatusPublished, now,
				func(context.Context, gen.DBTX) error {
					t.Fatal("CAS 輸掉時不得跑同 tx 的計數 hook")
					return nil
				})
			require.NoError(t, err)
			require.False(t, ok)
			require.EqualValues(t, domain.PostPublishStatusDraft, readPost(ctx, t, pool, id).PublishStatus)
		})
	}
}

// 搶佔：兩個 pod 同時 claim 同一列，只有一個拿得到；輸的那個連終寫都過不了。
func TestScheduledPostClaim_IsExclusive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000041
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Minute))

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners []string
	)
	start := make(chan struct{})
	for _, owner := range []string{"pod-a", "pod-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			<-start
			row, err := repo.Claim(ctx, postID, owner, time.Now().UTC(),
				time.Now().UTC().Add(120*time.Second), scheduleWindowHours)
			if err == nil && row != nil {
				mu.Lock()
				winners = append(winners, owner)
				mu.Unlock()
			}
		}(owner)
	}
	close(start)
	wg.Wait()

	require.Len(t, winners, 1, "單列 CAS 必須先到先贏——兩個都拿到租約就是重複發布")

	loser := "pod-a"
	if winners[0] == "pod-a" {
		loser = "pod-b"
	}
	ok, err := repo.Publish(ctx, postID, loser, domain.PostPublishStatusPublished, now, nil)
	require.NoError(t, err)
	require.False(t, ok, "沒拿到租約的 pod 不得寫得進終寫")

	ok, err = repo.Publish(ctx, postID, winners[0], domain.PostPublishStatusPublished, now, nil)
	require.NoError(t, err)
	require.True(t, ok)

	// 贏家發完之後，輸家就算重跑一次也發不出第二筆（schedule_state 已是 done）。
	ok, err = repo.Publish(ctx, postID, winners[0], domain.PostPublishStatusPublished, now, nil)
	require.NoError(t, err)
	require.False(t, ok, "終寫必須是一次性的，否則 published_at 會被後續的重跑覆蓋")
}

// 租約是冷卻租約不是已處理旗標：到期之後該列自動回到候選集合。
func TestScheduledPostLease_ExpiresBackIntoCandidates(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000051
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Hour))

	_, err := repo.Claim(ctx, postID, "crashed-pod", now, now.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)

	due, err := repo.ListDue(ctx, now.Add(time.Minute), scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Empty(t, due, "租約還在有效期內，別人不得撿走")

	later := now.Add(3 * time.Minute)
	due, err = repo.ListDue(ctx, later, scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Equal(t, []int64{postID}, due,
		"租約到期後該列必須回到候選集合——這正是它不能是永久旗標的理由")

	row, err := repo.Claim(ctx, postID, "pod-b", later, later.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)
	require.NotNil(t, row)
}

// 失敗釋放：帶自己的守衛，清租約、累加 retry_count、推遲下次嘗試。
func TestScheduledPostRelease_GuardedByOwner(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000061
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Minute))

	_, err := repo.Claim(ctx, postID, "pod-a", now, now.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)

	ok, err := repo.ReleaseLease(ctx, postID, "pod-b", now, now.Add(time.Minute))
	require.NoError(t, err)
	require.False(t, ok, "釋放語句必須帶守衛：動到別人的租約就是搶")
	require.Equal(t, "pod-a", *readPost(ctx, t, pool, postID).ScheduleLeaseOwner)

	next := now.Add(2 * time.Minute)
	ok, err = repo.ReleaseLease(ctx, postID, "pod-a", now, next)
	require.NoError(t, err)
	require.True(t, ok)

	after := readPost(ctx, t, pool, postID)
	require.Nil(t, after.ScheduleLeaseOwner)
	require.EqualValues(t, 1, after.ScheduleRetryCount)
	require.Equal(t, domain.ScheduleStatePending, *after.ScheduleState)

	// 退避還沒到，這一列不該被撈出來。
	due, err := repo.ListDue(ctx, now.Add(time.Minute), scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Empty(t, due)

	due, err = repo.ListDue(ctx, next.Add(time.Second), scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Equal(t, []int64{postID}, due)
}

// 候選窗口是參數不是字面值；逾窗口的列由第三條收斂轉 expired。
func TestScheduledPostConverge_WindowIsParameterised(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000071
	insertScheduledPost(ctx, t, pool, postID, now.Add(-30*time.Hour))

	due, err := repo.ListDue(ctx, now, scheduleWindowHours, 50)
	require.NoError(t, err)
	require.Empty(t, due, "30 小時前排定的貼文超出 24 小時窗口")

	// 換一個窗口值，同一列就回到候選——證明窗口真的走參數，不是寫死的 24。
	due, err = repo.ListDue(ctx, now, 48, 50)
	require.NoError(t, err)
	require.Equal(t, []int64{postID}, due)

	n, err := repo.ConvergeExpired(ctx, now, scheduleWindowHours)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	require.Equal(t, domain.ScheduleStateExpired, *readPost(ctx, t, pool, postID).ScheduleState)
}

func TestScheduledPostCancelByRevocation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewScheduledPostRepository(pool)
	now := time.Now().UTC()
	const postID int64 = 9100000000000081
	insertScheduledPost(ctx, t, pool, postID, now.Add(-time.Minute))

	_, err := repo.Claim(ctx, postID, "pod-a", now, now.Add(120*time.Second), scheduleWindowHours)
	require.NoError(t, err)

	ok, err := repo.CancelByRevocation(ctx, postID, "pod-b", now)
	require.NoError(t, err)
	require.False(t, ok, "取消也帶租約守衛")

	ok, err = repo.CancelByRevocation(ctx, postID, "pod-a", now)
	require.NoError(t, err)
	require.True(t, ok)

	after := readPost(ctx, t, pool, postID)
	require.Equal(t, domain.ScheduleStateCancelledByRevocation, *after.ScheduleState)
	require.EqualValues(t, domain.PostPublishStatusDraft, after.PublishStatus)
	require.Nil(t, after.ScheduleLeaseOwner)
}
