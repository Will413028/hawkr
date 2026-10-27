//go:build integration

package worker

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "social_worker_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

func setupWorkerDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	db, drop := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		drop()
	}
}

func newLivePublisher(pool *pgxpool.Pool, audit *fakeAuditClient, owner string) *postSchedulePublisher {
	hashtags := usecase.NewHashtagUseCase(postgres.NewHashtagRepository(pool), func() int64 { return 1 })
	p := newPostSchedulePublisher(testConfig(), postgres.NewScheduledPostRepository(pool),
		hashtags, audit, nil, zap.NewNop())
	p.owner = owner
	return p
}

func seedScheduled(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64, scheduledAt time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO cms_post (id, seller_id, content, publish_status, scheduled_publish_at,
		                      schedule_state, scheduled_by_session_id)
		VALUES ($1, 2685914682, '排程貼文', 4401, $2, 'pending', 7700000000000001)`, id, scheduledAt)
	require.NoError(t, err)
}

type postState struct {
	publishStatus int16
	scheduleState string
	leaseOwner    *string
	publishedAt   *time.Time
}

func readState(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) postState {
	t.Helper()
	var s postState
	err := pool.QueryRow(ctx, `
		SELECT publish_status, COALESCE(schedule_state, ''), schedule_lease_owner, published_at
		  FROM cms_post WHERE id = $1`, id).
		Scan(&s.publishStatus, &s.scheduleState, &s.leaseOwner, &s.publishedAt)
	require.NoError(t, err)
	return s
}

// 一個 tick：到點的 pending 貼文變成已發布，稽核走完 intent → confirm。
func TestWorkerTick_PublishesDuePost(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWorkerDB(ctx, t)
	defer cleanup()

	const postID int64 = 9200000000000001
	seedScheduled(ctx, t, pool, postID, time.Now().UTC().Add(-time.Minute))

	audit := &fakeAuditClient{}
	newLivePublisher(pool, audit, "pod-a").tick(ctx)

	s := readState(ctx, t, pool, postID)
	require.EqualValues(t, domain.PostPublishStatusPublished, s.publishStatus)
	require.Equal(t, domain.ScheduleStateDone, s.scheduleState)
	require.Nil(t, s.leaseOwner, "發布之後租約必須清掉")
	require.NotNil(t, s.publishedAt)

	require.Len(t, audit.intents, 1)
	require.Equal(t, domain.DelegationOriginScheduled, audit.intents[0].Origin)
	require.Len(t, audit.confirms, 1)
	require.Equal(t, "SUCCEEDED", audit.confirms[0].Result)
}

// 等待期間被賣家刪除／被平台強制下架：一律不發布，且兩者留下不同的終態值。
func TestWorkerTick_CancelledPostsAreNotPublished(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWorkerDB(ctx, t)
	defer cleanup()

	const deletedID int64 = 9200000000000011
	const hiddenID int64 = 9200000000000012
	scheduledAt := time.Now().UTC().Add(-time.Minute)
	seedScheduled(ctx, t, pool, deletedID, scheduledAt)
	seedScheduled(ctx, t, pool, hiddenID, scheduledAt)

	_, err := pool.Exec(ctx, `UPDATE cms_post SET deleted_at = NOW() WHERE id = $1`, deletedID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE cms_post SET hidden_at = NOW() WHERE id = $1`, hiddenID)
	require.NoError(t, err)

	audit := &fakeAuditClient{}
	newLivePublisher(pool, audit, "pod-a").tick(ctx)

	d := readState(ctx, t, pool, deletedID)
	require.Equal(t, domain.ScheduleStateCancelledByDeletion, d.scheduleState)
	require.EqualValues(t, domain.PostPublishStatusDraft, d.publishStatus)

	h := readState(ctx, t, pool, hiddenID)
	require.Equal(t, domain.ScheduleStateCancelledByGovernance, h.scheduleState)
	require.EqualValues(t, domain.PostPublishStatusDraft, h.publishStatus)

	require.Empty(t, audit.intents, "取消一律不寫代行稽核、不通知賣家")
	require.Empty(t, audit.confirms)
}

// 兩個 worker 併發跑同一個 tick：同一筆只發一次，稽核也只確認一次。
func TestWorkerTick_ConcurrentWorkersPublishOnce(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWorkerDB(ctx, t)
	defer cleanup()

	const postID int64 = 9200000000000021
	seedScheduled(ctx, t, pool, postID, time.Now().UTC().Add(-time.Minute))

	auditA, auditB := &fakeAuditClient{}, &fakeAuditClient{}
	a := newLivePublisher(pool, auditA, "pod-a")
	b := newLivePublisher(pool, auditB, "pod-b")

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, p := range []*postSchedulePublisher{a, b} {
		wg.Add(1)
		go func(p *postSchedulePublisher) {
			defer wg.Done()
			<-start
			p.tick(ctx)
		}(p)
	}
	close(start)
	wg.Wait()

	require.Equal(t, 1, len(auditA.confirms)+len(auditB.confirms),
		"兩個 pod 各發一次 ⇒ 賣家收到兩則通知、稽核兩列 SUCCEEDED，而貼文只有一則")
	require.Equal(t, 1, len(auditA.intents)+len(auditB.intents),
		"沒搶到租約的 pod 連 intent 都不該寫——那會留下一列永遠不會被 confirm 的 PENDING")

	s := readState(ctx, t, pool, postID)
	require.EqualValues(t, domain.PostPublishStatusPublished, s.publishStatus)
	require.Equal(t, domain.ScheduleStateDone, s.scheduleState)
}

// 空表跑一輪 tick 不得報錯：上線初期沒有任何 pending 列是預期狀態。
func TestWorkerTick_EmptyTable(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWorkerDB(ctx, t)
	defer cleanup()

	audit := &fakeAuditClient{}
	newLivePublisher(pool, audit, "pod-a").tick(ctx)

	require.Empty(t, audit.intents)
	require.Empty(t, audit.confirms)
}

// 賣家自己發的既有貼文（schedule_state 為 NULL）完全不受 worker 影響。
func TestWorkerTick_LeavesUnscheduledPostsAlone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWorkerDB(ctx, t)
	defer cleanup()

	const postID int64 = 9200000000000031
	_, err := pool.Exec(ctx, `
		INSERT INTO cms_post (id, seller_id, content) VALUES ($1, 2685914682, '賣家自己發的')`, postID)
	require.NoError(t, err)

	newLivePublisher(pool, &fakeAuditClient{}, "pod-a").tick(ctx)

	s := readState(ctx, t, pool, postID)
	require.EqualValues(t, domain.PostPublishStatusPublished, s.publishStatus, "DB 預設即已發布")
	require.Equal(t, "", s.scheduleState, "NULL ＝ 從未被排程，worker 不得寫任何值進去")
	require.Nil(t, s.publishedAt, "worker 不得碰它")
}
