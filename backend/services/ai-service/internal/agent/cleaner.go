package agent

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sessionCleanerInterval is the cadence at which expired session data is purged.
const sessionCleanerInterval = 1 * time.Hour

// SessionCleaner 定時清理超過 24 小時的 session 資料
type SessionCleaner struct {
	pool      *pgxpool.Pool
	scheduler *outbox.Scheduler
}

// newSessionCleaner 收注入的共用 pool，不再自建連線——pool 的建立與 Close 統一由
// composition root（internal/infrastructure/db.go 的 ProvidePool，fx 具名
// "ai_pool"）負責。cleaner 只借用它，Stop() 不再關閉它。
func newSessionCleaner(pool *pgxpool.Pool) *SessionCleaner {
	return &SessionCleaner{pool: pool}
}

// Start begins the periodic cleanup scheduler, isolated behind a panic
// guard so a bug in any single pass can NEVER bring down the whole
// ai-service process. Mirrors seller-service's SellerDeletedOutboxScheduler
// (services/seller-service/internal/worker/seller_deleted_outbox_scheduler.go).
// WithInitialDelay(0) preserves the existing "啟動時先清一次" behavior.
func (c *SessionCleaner) Start() {
	c.scheduler = outbox.NewScheduler(sessionCleanerInterval,
		func(_ context.Context) error {
			c.cleanup()
			return nil
		},
		func(_ context.Context, r any) {
			slog.Error("session cleaner 發生 panic",
				"recovered", r,
				"stack", string(debug.Stack()))
		},
		nil,
		outbox.WithInitialDelay(0),
	)
	c.scheduler.Start(context.Background())
}

// Stop halts the periodic scheduler. It must NOT close c.pool: the pool is
// injected (fx-named "ai_pool"), shared with Style/Quota/Transform/
// CandidateSearch usecases that keep running after this cleaner stops.
// Ownership — and Close — belongs solely to whoever provides it
// (internal/infrastructure/db.go's ProvidePool, via its own OnStop hook).
func (c *SessionCleaner) Stop() {
	if c.scheduler != nil {
		c.scheduler.Stop()
	}
}

// procurementAppName 是 procurement graph 在 ADK session store 裡的 app_name，
// session key 為 (app_name, buyerId, requestId)——與 usecase 那側寫出來的必須一致。
//
// 取自同 package 的 AppNameProcurement，不自己寫一份字面值：usecase.ProcurementAppName
// 也是從它來的（見 graph_session.go），所以兩邊同源，漂移在型別層面就不可能發生。
// cleaner 仍然不 import procurement/usecase——宿主層的清理排程不該依賴單一 app 的
// usecase，而且那個方向會造成 import cycle（usecase → agent 已經成立）。
const procurementAppName = string(AppNameProcurement)

func (c *SessionCleaner) cleanup() {
	ctx := context.Background()
	cutoff := time.Now().Add(-24 * time.Hour)

	// ⛔ eligibility 與刪除在同一個 transaction：分兩次查會讓「檢查時還沒有
	// interrupt、刪除時已經有了」的窗口把剛掛起的對話清掉。
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		slog.Error("清理 session 開啟交易失敗", "error", err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 先刪 events（外鍵關聯）。
	//
	// 這道 DELETE 走自己的 timestamp，刻意與 sessions 分開：長時間持續被更新的
	// session 底下會不斷堆積舊事件，那些要照 24 小時保有期掃掉（doc 15 §7.5；
	// cleaner_integration_test 的 e-old-under-fresh-session 釘住這條）。
	//
	// ⛔ 例外只有一個：還掛著 PENDING interrupt 的 procurement session。events 就是
	// ADK 的 checkpoint 歷史，少了它 resume 會退化成 fresh invocation——上游 node
	// 重跑、再開一個 interrupt，而買家的回答落不到原本那個掛起點上
	// （usecase/graph_session.go 的不變式：session 必須跨越掛起與回答兩個
	// HTTP request）。只保住 sessions 那一列不算修好。
	if _, err := tx.Exec(ctx, `
		DELETE FROM events e
		WHERE e."timestamp" < $1
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_agent_run_interrupts i
			JOIN procurement_agent_runs r ON r.id = i.run_id
			JOIN procurement_conversations c ON c.id = r.conversation_id
			WHERE i.status = 'PENDING'
			  AND i.resolved_by_child_run_id IS NULL
			  AND e.app_name = $2
			  AND e.user_id = c.buyer_id::text
			  AND e.session_id = c.request_id::text
		  )`, cutoff, procurementAppName); err != nil {
		slog.Error("清理過期 events 失敗", "error", err)
		return
	}

	// 再刪 sessions，同一個豁免條件。
	tag, err := tx.Exec(ctx, `
		DELETE FROM sessions s
		WHERE s.update_time < $1
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_agent_run_interrupts i
			JOIN procurement_agent_runs r ON r.id = i.run_id
			JOIN procurement_conversations c ON c.id = r.conversation_id
			WHERE i.status = 'PENDING'
			  AND i.resolved_by_child_run_id IS NULL
			  AND s.app_name = $2
			  AND s.user_id = c.buyer_id::text
			  AND s.id = c.request_id::text
		  )`, cutoff, procurementAppName)
	if err != nil {
		slog.Error("清理過期 sessions 失敗", "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("清理 session 提交失敗", "error", err)
		return
	}

	if tag.RowsAffected() > 0 {
		slog.Info("已清理過期 sessions", "count", tag.RowsAffected())
	}
}
