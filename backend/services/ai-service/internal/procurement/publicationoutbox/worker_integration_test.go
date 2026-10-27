//go:build integration

package publicationoutbox

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
)

// 自己的 shared-container-per-package／own-database-per-test harness：不能與
// internal/procurement/repository 或 internal/procurement/outbox 共用（那兩個
// pg 都是 package-scoped 的未匯出變數），所以 template DB 名字也要各自不同，
// 併發跑時容器／migration 才不會撞在一起。理由全文見 repository/dbtest_test.go。
var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "procurement_publication_outbox_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

// testMetrics 只建一次。outbox.NewMetrics 走 promauto，同一個 outbox_name 在同一
// 個程序註冊第二次會 panic；名字刻意不是 production 的 "rfq_publication"，避免與
// di.go 的 newMetrics 在任何情況下對撞。
var testMetrics = outbox.NewMetrics("rfq_publication_integration_test")

func setupTestDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	db, drop := pg.Clone(ctx, t)
	// 走 database.NewPool（production 用的同一條路），pool 的 QueryExecMode 才與
	// production 一致（pgx.QueryExecModeExec，為了 PgBouncer 相容）。
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		drop()
	}
}

// recordingPublisher 記下每一次外送，讓測試看得見「發了幾次、發了什麼」。
type recordingPublisher struct {
	calls []publishCall
	err   error
}

type publishCall struct {
	eventType string
	payload   []byte
}

func (p *recordingPublisher) publish(_ context.Context, eventType string, payload []byte) error {
	p.calls = append(p.calls, publishCall{eventType: eventType, payload: append([]byte(nil), payload...)})
	return p.err
}

type outboxState struct {
	status       string
	attemptCount int32
	lastError    *string
	completedAt  *time.Time
}

func readOutbox(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) outboxState {
	t.Helper()
	var got outboxState
	err := pool.QueryRow(ctx,
		`SELECT status, attempt_count, last_error, completed_at FROM procurement_rfq_publication_outbox WHERE id = $1`, id,
	).Scan(&got.status, &got.attemptCount, &got.lastError, &got.completedAt)
	require.NoError(t, err)
	return got
}

func readInviteState(ctx context.Context, t *testing.T, repo *repository.Repo, publicationID int64) string {
	t.Helper()
	invites, err := repo.ListPublicationInvites(ctx, publicationID)
	require.NoError(t, err)
	require.Len(t, invites, 1)
	return invites[0].DeliveryState
}

type fixture struct {
	pool          *pgxpool.Pool
	inner         *repository.Repo
	repo          *Repository
	publicationID int64
	inviteID      int64
	outboxID      int64
	payload       []byte
}

// seed 建一張 PLATFORM invite（起始 QUEUED）與它唯一的 outbox 命令，形狀與
// publication_execute 在它自己的交易內寫下的完全相同（payload 三個 camelCase
// 字串欄位）。
func seed(ctx context.Context, t *testing.T, pool *pgxpool.Pool, storeID int64) *fixture {
	t.Helper()
	var n int64
	inner := repository.New(pool, database.NewTxManager(pool), func() int64 { n++; return n })

	// 排在過去，Worker 這一輪就撈得到（FetchPendingDue 的述詞是 next_attempt_at <= now）。
	due := time.Now().UTC().Add(-time.Minute)
	publicationID := int64(770000 + storeID)
	invites, err := inner.InsertPublicationInvites(ctx, repository.InsertPublicationInvitesInput{
		PublicationID: publicationID,
		BuyerID:       identity.BuyerID(9401),
		Recipients:    []domain.PublicationRecipient{domain.PlatformRecipient(storeID)},
		Now:           due,
	})
	require.NoError(t, err)
	require.Equal(t, repository.InviteDeliveryStateQueued, invites[0].DeliveryState)

	payload := []byte(`{"inviteId":"1","publicationId":"2","storeId":"3"}`)
	require.NoError(t, inner.InsertPublicationOutbox(ctx, invites[0].ID,
		repository.PublicationOutboxEventInviteCreated, payload, due))

	repo := NewRepository(pool, database.NewTxManager(pool), inner)
	rows, err := repo.FetchPendingDue(ctx, time.Now().UTC(), 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, invites[0].ID, rows[0].InviteID)

	return &fixture{
		pool: pool, inner: inner, repo: repo,
		publicationID: publicationID,
		inviteID:      invites[0].ID,
		outboxID:      rows[0].ID,
		payload:       payload,
	}
}

// ① 成功路徑：發 MQ → invite 轉 SENT → outbox 轉 done。
func TestWorker_Success_MarksInviteSentAndOutboxDone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, 5301)
	pub := &recordingPublisher{}
	worker := NewWorker(f.repo, pub.publish, testMetrics, zaptest.NewLogger(t))

	require.NoError(t, worker.RunOnce(ctx))

	require.Len(t, pub.calls, 1)
	require.Equal(t, repository.PublicationOutboxEventInviteCreated, pub.calls[0].eventType)
	require.JSONEq(t, string(f.payload), string(pub.calls[0].payload),
		"payload 原樣過線：已提交的 bytes 就是上線的 bytes")

	require.Equal(t, repository.InviteDeliveryStateSent, readInviteState(ctx, t, f.inner, f.publicationID))

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "done", got.status)
	require.NotNil(t, got.completedAt)
	require.EqualValues(t, 0, got.attemptCount)
}

// ② 重入：同一列被處理兩次（MarkDone 沒寫成 ⇒ 列仍 pending，下一輪重跑）。
//
// 第二輪時 invite 已經是 SENT，MarkInviteSent 的 CAS 影響 0 列。斷言的重點不是
// 「invite 還是 SENT」而已——`status='done' 且 attempt_count 仍為 0` 才是「0 列沒
// 被當成錯誤」的證據：若它回錯，pkg/outbox.Worker 會走 bumpAttempt，這一列會變回
// pending 且 attempt_count=1。
func TestWorker_Reentry_ZeroRowCASIsNotAnError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, 5302)
	pub := &recordingPublisher{}
	worker := NewWorker(f.repo, pub.publish, testMetrics, zaptest.NewLogger(t))

	require.NoError(t, worker.RunOnce(ctx))
	require.Len(t, pub.calls, 1)

	// 把 MarkDone 那一次寫入「收回去」——這正是 publish 成功之後、MarkDone 之前
	// 崩潰（或 lease 到期）留下的狀態。
	_, err := pool.Exec(ctx,
		`UPDATE procurement_rfq_publication_outbox
		 SET status = 'pending', completed_at = NULL, next_attempt_at = $2 WHERE id = $1`,
		f.outboxID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)

	require.NoError(t, worker.RunOnce(ctx))

	require.Len(t, pub.calls, 2,
		"重入必然重發；重複由 notification-service 的 dedup 唯一索引吸收（reference_id 綁 inviteId）")
	require.Equal(t, repository.InviteDeliveryStateSent, readInviteState(ctx, t, f.inner, f.publicationID),
		"invite 終態一致：第二輪的 CAS 影響 0 列，狀態不動")

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "done", got.status)
	require.EqualValues(t, 0, got.attemptCount,
		"CAS 0 列若被當成錯誤，這裡會是 pending/attempt_count=1")
	require.Nil(t, got.lastError)
}

// ③a MarkFailed 一次呼叫之後，outbox 是 failed **且** invite 是 FAILED。
func TestRepository_MarkFailed_MovesOutboxAndInviteTogether(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, 5303)
	now := time.Now().UTC()

	require.NoError(t, f.repo.MarkFailed(ctx, f.outboxID, "publish: broker unreachable", now))

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "failed", got.status)
	require.NotNil(t, got.lastError)
	require.Equal(t, "publish: broker unreachable", *got.lastError)
	require.NotNil(t, got.completedAt)

	require.Equal(t, repository.InviteDeliveryStateFailed, readInviteState(ctx, t, f.inner, f.publicationID))
}

// ③b MarkFailed 的**原子性**證明。
//
// 「呼叫完兩者都變了」只證明兩次寫入都發生過，不證明它們同生共死。這支反過來
// 打：用一個 BEFORE UPDATE trigger 讓 invite 那一句必定失敗，然後檢查 outbox 那
// 一句有沒有留下來。
//
//   - 現在的實作（兩句在同一個 pgx.Tx 上）：整段 ROLLBACK，outbox 仍是 pending。
//   - 任何非原子的實作（兩句各自 autocommit，或改用 Hooks 在 MarkFailed 之後補
//     invite）：outbox 已經是 failed 而 invite 還在 QUEUED——正是設計書 §4.4 要
//     排除的窗口，這支測試會紅。
func TestRepository_MarkFailed_InviteWriteFailureRollsBackOutboxWrite(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, 5304)

	_, err := pool.Exec(ctx, `CREATE FUNCTION prpi_block_update() RETURNS trigger LANGUAGE plpgsql AS $fn$
		BEGIN RAISE EXCEPTION 'invite update blocked by test'; END $fn$`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TRIGGER prpi_block_update_trg BEFORE UPDATE ON procurement_rfq_publication_invites
		FOR EACH ROW EXECUTE FUNCTION prpi_block_update()`)
	require.NoError(t, err)

	markErr := f.repo.MarkFailed(ctx, f.outboxID, "publish: broker unreachable", time.Now().UTC())
	require.Error(t, markErr, "invite 那一句失敗必須讓整個 MarkFailed 失敗")

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "pending", got.status,
		"invite 寫不進去時 outbox 那一句必須一起回滾——否則就留下「outbox 已 failed、invite 仍 QUEUED」的窗口")
	require.Nil(t, got.completedAt)
	require.Nil(t, got.lastError)

	_, err = pool.Exec(ctx, `DROP TRIGGER prpi_block_update_trg ON procurement_rfq_publication_invites`)
	require.NoError(t, err)
	require.Equal(t, repository.InviteDeliveryStateQueued, readInviteState(ctx, t, f.inner, f.publicationID))
}

// ④ 順序保證：publish 失敗時 invite **不得**被轉 SENT。
//
// 這是本條線存在的理由本身：先轉 SENT 再發送，一旦發送失敗就留下「說已送出但沒
// 送」。這支測試釘住相反方向的最壞後果——invite 停在 QUEUED，而 outbox 排下一次
// 重試（低估，不是說謊）。
func TestWorker_PublishFailure_LeavesInviteQueuedAndRetriesOutbox(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	f := seed(ctx, t, pool, 5305)
	pub := &recordingPublisher{err: errors.New("broker unreachable")}
	worker := NewWorker(f.repo, pub.publish, testMetrics, zaptest.NewLogger(t))

	require.NoError(t, worker.RunOnce(ctx), "單列失敗不讓整個 tick 失敗")
	require.Len(t, pub.calls, 1)

	require.Equal(t, repository.InviteDeliveryStateQueued, readInviteState(ctx, t, f.inner, f.publicationID),
		"沒送出去就不准說已送出")

	got := readOutbox(ctx, t, pool, f.outboxID)
	require.Equal(t, "pending", got.status)
	require.EqualValues(t, 1, got.attemptCount)
	require.NotNil(t, got.lastError)
}

// ⑤ 未知 event_type 不得被靜默送出去。
func TestNewPublishFunc_UnknownEventTypeIsRejected(t *testing.T) {
	raw := &recordingRawPublisher{}
	err := NewPublishFunc(raw)(context.Background(), "NOT_A_REAL_EVENT", []byte(`{}`))
	require.Error(t, err)
	require.Empty(t, raw.calls)
}

// ⑥ 已知 event_type 走 procurement.events ＋ 本條線自己的 routing key。
func TestNewPublishFunc_MapsEventTypeToRoutingKey(t *testing.T) {
	raw := &recordingRawPublisher{}
	body := []byte(`{"inviteId":"1","publicationId":"2","storeId":"3"}`)
	require.NoError(t, NewPublishFunc(raw)(context.Background(),
		repository.PublicationOutboxEventInviteCreated, body))

	require.Len(t, raw.calls, 1)
	require.Equal(t, mq.ExchangeProcurement, raw.calls[0].exchange)
	require.Equal(t, mq.RoutingKeyProcurementRfqPublicationInviteCreated, raw.calls[0].routingKey)
	require.JSONEq(t, string(body), string(raw.calls[0].body))
}

type recordingRawPublisher struct {
	calls []rawCall
}

type rawCall struct {
	exchange   string
	routingKey string
	body       []byte
}

func (p *recordingRawPublisher) PublishRaw(_ context.Context, exchange, routingKey string, body []byte) error {
	p.calls = append(p.calls, rawCall{exchange: exchange, routingKey: routingKey, body: append([]byte(nil), body...)})
	return nil
}
