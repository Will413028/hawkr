//go:build integration

package agent

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestSessionCleaner_Cleanup_DeletesExpiredKeepsFresh drives cleanup() against
// a real database. It locks in that each table is swept independently on its
// own timestamp column: sessions.update_time for sessions, events.timestamp
// for events.
//
// events.session_id has ON DELETE CASCADE onto sessions (see
// db/schema/schema.sql's fk_sessions_events), so an expired session's own
// events would disappear even if the events DELETE were a no-op — "e-old"
// alone cannot tell the two apart. "e-old-under-fresh-session" closes that
// gap: it is seeded under the still-fresh session, so cascade cannot be what
// removes it — only the events statement's own timestamp filter can. That is
// also the actual reason this statement exists as a separate DELETE: a
// long-running session that keeps getting updated while old events pile up
// underneath it. (A fresh event under an expiring session is a real case too
// — cascade removes it alongside its parent session — but that is existing
// product behavior this migration doesn't change, not something this test
// needs to exercise.)
func TestSessionCleaner_Cleanup_DeletesExpiredKeepsFresh(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAgentTestDB(ctx, t)
	defer cleanup()

	now := time.Now().UTC()
	old := now.Add(-25 * time.Hour)
	fresh := now.Add(-1 * time.Hour)

	seedSession(ctx, t, pool, "old", old)
	seedEvent(ctx, t, pool, "e-old", "old", old)
	seedSession(ctx, t, pool, "fresh", fresh)
	seedEvent(ctx, t, pool, "e-fresh", "fresh", fresh)
	seedEvent(ctx, t, pool, "e-old-under-fresh-session", "fresh", old)

	c := &SessionCleaner{pool: pool}
	c.cleanup()

	require.False(t, sessionExists(ctx, t, pool, "old"), "expired session should have been deleted")
	require.False(t, eventExists(ctx, t, pool, "e-old"), "expired event should have been deleted")
	require.True(t, sessionExists(ctx, t, pool, "fresh"), "fresh session should still exist")
	require.True(t, eventExists(ctx, t, pool, "e-fresh"), "fresh event should still exist")
	require.False(t, eventExists(ctx, t, pool, "e-old-under-fresh-session"),
		"expired event under a still-fresh session should have been deleted by the events sweep, independent of any session-cascade")
}

func seedSession(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id string, updateTime time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO sessions (app_name, user_id, id, update_time) VALUES ($1, $2, $3, $4)`,
		"hawkr_ai", "u1", id, updateTime)
	require.NoError(t, err)
}

func seedEvent(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id, sessionID string, timestamp time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO events (id, app_name, user_id, session_id, "timestamp") VALUES ($1, $2, $3, $4, $5)`,
		id, "hawkr_ai", "u1", sessionID, timestamp)
	require.NoError(t, err)
}

func sessionExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM sessions WHERE app_name = $1 AND user_id = $2 AND id = $3)`,
		"hawkr_ai", "u1", id).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func eventExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM events WHERE id = $1)`, id).Scan(&exists)
	require.NoError(t, err)
	return exists
}

// procurementAppName 必須與 usecase.ProcurementSessionID／ProcurementAppName 寫出
// 來的鍵一致。刻意不 import 那個 package（cleaner 是宿主層、不該依賴任一個 app 的
// usecase），改由 usecase 側的 pinning test 守住兩邊相同。
const procurementAppNameForTest = "hawkr_procurement"

// seedPendingProcurementSession 種出一份「買家被問了一題、還沒回答」的真實狀態：
// request → conversation → run → PENDING interrupt，以及對應的 ADK session/events。
// session 鍵照 production 的算法：app_name 固定、user_id 是 buyerId、id 是 requestId。
func seedPendingProcurementSession(
	ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	requestID, buyerID, conversationID, runID int64,
	interruptID, interruptStatus string, at time.Time,
) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO procurement_requests (id, buyer_id, status) VALUES ($1, $2, 'CLARIFYING')`,
		requestID, buyerID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO procurement_conversations (id, buyer_id, request_id) VALUES ($1, $2, $3)`,
		conversationID, buyerID, requestID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO procurement_agent_runs (id, buyer_id, conversation_id, run_type, status)
		 VALUES ($1, $2, $3, 'TURN', 'WAITING_FOR_INPUT')`,
		runID, buyerID, conversationID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO procurement_agent_run_interrupts
		   (id, run_id, interrupt_kind, response_schema_version, status)
		 VALUES ($1, $2, 'CLARIFICATION', 'v1', $3)`,
		interruptID, runID, interruptStatus)
	require.NoError(t, err)

	sessionID := strconv.FormatInt(requestID, 10)
	userID := strconv.FormatInt(buyerID, 10)
	_, err = pool.Exec(ctx,
		`INSERT INTO sessions (app_name, user_id, id, update_time) VALUES ($1, $2, $3, $4)`,
		procurementAppNameForTest, userID, sessionID, at)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO events (id, app_name, user_id, session_id, "timestamp") VALUES ($1, $2, $3, $4, $5)`,
		"e-"+interruptID, procurementAppNameForTest, userID, sessionID, at)
	require.NoError(t, err)
}

func procurementSessionExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, requestID, buyerID int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM sessions WHERE app_name = $1 AND user_id = $2 AND id = $3)`,
		procurementAppNameForTest,
		strconv.FormatInt(buyerID, 10), strconv.FormatInt(requestID, 10)).Scan(&exists))
	return exists
}

// TestSessionCleaner_ProcurementPending 釘住 retention 的真實依據：一個還掛著
// PENDING interrupt 的 procurement session，連同它 resume 所需的完整 event 歷史，
// 不得因為超過 24 小時就被清掉——CropGate 的掛起與買家回答分屬兩個獨立 HTTP
// request（usecase/graph_session.go 的不變式），清掉等於買家回來時答案無處可落。
//
// ⛔ 只保住 sessions 那一列不算修好：events 就是 ADK 的 checkpoint 歷史，
// 少了它 resume 會退化成 fresh invocation（上游 node 重跑、再開一個 interrupt）。
//
// 已解決的 interrupt 與其他 app 的既有政策不變——後兩者是負控制，少了它們
// 「乾脆不清理」也會讓這條測試變綠。
func TestSessionCleaner_ProcurementPending(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAgentTestDB(ctx, t)
	defer cleanup()

	now := time.Now().UTC()
	old := now.Add(-25 * time.Hour)
	fresh := now.Add(-1 * time.Hour)

	seedPendingProcurementSession(ctx, t, pool, 9001, 501, 8001, 7001, "int-pending", "PENDING", old)
	seedPendingProcurementSession(ctx, t, pool, 9002, 502, 8002, 7002, "int-resolved", "APPROVED", old)
	seedSession(ctx, t, pool, "other-app-old", old)
	seedEvent(ctx, t, pool, "e-other-app-old", "other-app-old", old)
	seedSession(ctx, t, pool, "other-app-fresh", fresh)
	seedEvent(ctx, t, pool, "e-old-under-fresh", "other-app-fresh", old)

	c := &SessionCleaner{pool: pool}
	c.cleanup()

	require.True(t, procurementSessionExists(ctx, t, pool, 9001, 501),
		"仍掛著 PENDING interrupt 的 session 不得被清掉")
	require.True(t, eventExists(ctx, t, pool, "e-int-pending"),
		"resume 需要的 event 歷史必須整套保留，只留 sessions 那一列不算修好")

	require.False(t, procurementSessionExists(ctx, t, pool, 9002, 502),
		"interrupt 已解決就回到既有的 24 小時政策")
	require.False(t, eventExists(ctx, t, pool, "e-int-resolved"))

	require.False(t, sessionExists(ctx, t, pool, "other-app-old"),
		"其他 app 的既有政策不變")
	require.False(t, eventExists(ctx, t, pool, "e-other-app-old"))
	require.True(t, sessionExists(ctx, t, pool, "other-app-fresh"))
	require.False(t, eventExists(ctx, t, pool, "e-old-under-fresh"),
		"沒有被保護的 session 底下，舊 events 仍照既有決定掃掉（cleaner_integration_test 的 e-old-under-fresh-session 已釘住這條）")
}
