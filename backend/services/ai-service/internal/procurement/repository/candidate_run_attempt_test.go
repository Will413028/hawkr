//go:build integration

package repository

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 本檔守的是「one run = one immutable execution attempt」：一次重試是一次**新的執行**，
// 必須配一個新的 run 身分，而不是把同一個 run_id 重新開機。
//
// 為什麼這件事非做不可，而不只是模型上比較漂亮：`procurement_agent_runs` 的
// `TerminalizeProcurementAgentRunStatus` 守衛是「現在還不是終態」。沿用同一個 run_id 時，
// 第一次嘗試已經把那一列寫成終態，第二次嘗試的終態於是被守衛擋掉、只留一行 Warn——**那個
// run 的第二次執行永遠沒有終態 ledger event**，串流端沒有收線訊號。這不是計數少一筆，是
// 一條使用者看得到的掛死路徑。
//
// 這也讓 `candidate_sets : agent_runs` 從 1:1 變成 1:N：candidate set 是「這批候選」，
// 每次嘗試各是一個 run。`procurement_candidate_sets.run_id` 因此變成**當前**嘗試的身分，
// 歷史嘗試只留在 `procurement_agent_runs`（由 `domain_ref_id` 指回同一個 candidate set）。

// setupRunAttemptFixture 建一個 request ＋ candidate set ＋ 它第一次嘗試的 ledger run。
// leaseTTL 傳負值可以造出一個「lease 已過期」的 run，讓 lazy reaper 在下一次讀取時觸發。
func setupRunAttemptFixture(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID, leaseTTL time.Duration) (Request, CandidateSet) {
	t.Helper()
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: leaseTTL,
	})
	require.NoError(t, err)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, set))
	return req, set
}

// ledgerEventTypes 回傳一個 run 在 ledger 上的事件型別序列（依 seq）。
func ledgerEventTypes(ctx context.Context, t *testing.T, repo *Repo, runID int64) []string {
	t.Helper()
	rows, err := repo.pool.Query(ctx,
		`SELECT event_type FROM procurement_agent_run_events WHERE run_id = $1 ORDER BY seq`, runID)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var eventType string
		require.NoError(t, rows.Scan(&eventType))
		out = append(out, eventType)
	}
	require.NoError(t, rows.Err())
	return out
}

// ledgerRunStatus 回傳一個 ledger run 的 (status, failure_reason)。
func ledgerRunStatus(ctx context.Context, t *testing.T, repo *Repo, runID int64) (string, string) {
	t.Helper()
	var status, failureReason string
	require.NoError(t, repo.pool.QueryRow(ctx,
		`SELECT status, failure_reason FROM procurement_agent_runs WHERE id = $1`, runID,
	).Scan(&status, &failureReason))
	return status, failureReason
}

// TestRetryCandidateSet_MintsNewRunIDPerAttempt 是 B9 的核心斷言：重試沿用同一列
// candidate_set（冪等鍵 `(request_revision_id, policy_version)` 上的 unique index 本來
// 就擋掉第二列），但**必須換一個 run_id**。
//
// 修正前這裡是紅的：`RetryProcurementCandidateSet` 的 SET 子句完全不碰 run_id。
func TestRetryCandidateSet_MintsNewRunIDPerAttempt(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	firstRunID := set.PublicRunID()

	// 讓 lazy reaper 把第一次嘗試收斂成 FAILED，重試才有東西可以救。
	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status, "test precondition: reap 必須真的發生")

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)

	require.Equal(t, set.ID, retried.ID,
		"重試必須沿用同一列 candidate_set——這批候選還是同一批")
	require.NotEqual(t, firstRunID, retried.PublicRunID(),
		"重試是一次**新的執行**，必須配一個新的 run 身分；沿用舊 run_id 會讓第二次嘗試的終態被"+
			"`TerminalizeProcurementAgentRunStatus` 的『現在還不是終態』守衛擋掉，該 run 永遠沒有收線訊號")
	require.NotZero(t, retried.RunID)

	// 回傳的 struct 對不代表 DB 對——身分欄位一律直接驗那一列。
	var gotRunID *int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT run_id FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&gotRunID))
	require.NotNil(t, gotRunID)
	require.Equal(t, retried.RunID, *gotRunID, "新的 run_id 必須真的寫進 DB")
}

// TestCreateLedgerRunForCandidateSearch_SecondAttempt_IsSecondRunForSameCandidateSet
// 是 1:N 本身的證明：同一個 candidate set 底下可以有兩列 agent_runs，兩列的
// `domain_ref_id` 都指回那個 candidate set。
//
// 修正前這裡是紅的：第二次嘗試沿用同一個 run_id，`CreateProcurementAgentRun` 會撞
// primary key（既有的 TestCreateLedgerRunForCandidateSearch_DuplicateRunID_ReturnsUniqueViolation
// 守的就是那個行為），根本建不出第二列。
func TestCreateLedgerRunForCandidateSearch_SecondAttempt_IsSecondRunForSameCandidateSet(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	firstRunID := set.PublicRunID()

	_, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, retried),
		"第二次嘗試必須建得出第二列 ledger run——那正是 1:N")

	var runCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs
		  WHERE domain_ref_type = 'candidateSetId' AND domain_ref_id = $1`, strconv.FormatInt(set.ID, 10),
	).Scan(&runCount))
	require.Equal(t, 2, runCount,
		"一個 candidate set 兩次嘗試＝兩列 agent_runs（1:N）")

	// 兩列都要有自己的首筆 RUN_ACCEPTED——一次新的執行從頭開始記，不接續上一次的 seq。
	require.Equal(t, []string{codemeta.ProcurementLedgerEventTypeRunAccepted.Value},
		ledgerEventTypes(ctx, t, repo, retried.PublicRunID())[:1],
		"第二次嘗試的 ledger 首筆必須是它自己的 RUN_ACCEPTED")
	require.NotEqual(t, firstRunID, retried.PublicRunID())
}

// TestGetCandidateSetRunWithCandidates_ResolvesSupersededAttempt 守的是 1:N 之後的解析
// 回歸：`procurement_candidate_sets.run_id` 只留**當前**嘗試，被取代的舊 run_id 在那張表
// 上再也查不到。
//
// 沒有這條 fallback 的話，一個在重試發生前就開始輪詢、手上拿著舊 runId 的 browser 會突然
// 開始收到 404——而它拿到的舊值不會隨部署恢復（問題出在 browser 手上的值）。這正是
// grpc/procurement_handler.go GetCandidateSearch 的 echo-back 註解描述過的那一族靜默失效，
// 只是觸發點從「兩波 rollout」換成「一次重試」。
//
// 解析走 `procurement_agent_runs.domain_ref_id` 指回 candidate set：那是 1:N 的多對一方向，
// 每一次嘗試都認得自己屬於哪一批候選。
func TestGetCandidateSetRunWithCandidates_ResolvesSupersededAttempt(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	firstRunID := set.PublicRunID()

	_, _, err := repo.GetCandidateSetRunWithCandidates(ctx, firstRunID, req.ID, buyerID)
	require.NoError(t, err, "test precondition: 第一次嘗試的 runId 本來就查得到")

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, retried))

	got, _, err := repo.GetCandidateSetRunWithCandidates(ctx, firstRunID, req.ID, buyerID)
	require.NoError(t, err,
		"被取代的舊 runId 仍必須解析得到它所屬的 candidate set，否則輪詢中的舊 client 直接掉進 404")
	require.Equal(t, set.ID, got.ID)
	require.Equal(t, retried.RunID, got.RunID,
		"解析回來的是 candidate set 目前的狀態（＝最新一次嘗試）；對外 runId 由 handler echo-back，不從這裡推導")
}

// TestGetCandidateSetRunWithCandidates_SupersededAttempt_StillEnforcesOwnership 守的是
// 新增解析路徑最容易漏掉的那一半：換一把查詢鍵**不是**放寬 ownership 的理由
// （GetProcurementCandidateSetByRunID 的既有註解就是這句）。
func TestGetCandidateSetRunWithCandidates_SupersededAttempt_StillEnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	firstRunID := set.PublicRunID()

	// 這次讀取順便觸發 lazy reap，把第一次嘗試收斂成 FAILED，重試才有東西可以救。
	_, _, err := repo.GetCandidateSetRunWithCandidates(ctx, firstRunID, req.ID, buyerID)
	require.NoError(t, err)

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, retried))

	_, _, err = repo.GetCandidateSetRunWithCandidates(ctx, firstRunID, req.ID, identity.BuyerID(999))
	require.ErrorIs(t, err, ErrNotFound, "別的買家不得經由被取代的 runId 讀到這批候選")

	_, _, err = repo.GetCandidateSetRunWithCandidates(ctx, firstRunID, req.ID+1, buyerID)
	require.ErrorIs(t, err, ErrNotFound, "requestId 對不上時同樣不得解析")
}

// ── B11：reap 補齊 ledger 終態 ────────────────────────────────────────────────
//
// reapCandidateSet 原本只寫 procurement_candidate_sets 與 procurement_requests 兩張表，
// 完全不碰 procurement_agent_runs、不寫任何 ledger event。淨結果是 WORKER_LOST 的 run
// 永遠停在 QUEUED／RUNNING，串流端沒有收線訊號，SSE 開到 gateway 的 120 秒 deadline；
// 買家唯一的失敗管道是輪詢讀回 failureReason，被前端原樣渲染成粗體機器碼 WORKER_LOST。
//
// 對照 LANE_ALL_FAILED（in-process 失敗路徑）：有 RUN_FAILED ledger event、有 artifact、
// 前端 onRunFailed 拿得到人話訊息。本波把 **ledger 層**補對稱；artifact 層刻意維持不對稱
// （見 TestReapCandidateSet_DoesNotProduceErrorRecoveryArtifact）。

// TestReapCandidateSet_WritesRunFailedTerminalEvent 是 B11 的核心斷言。
func TestReapCandidateSet_WritesRunFailedTerminalEvent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	runID := set.PublicRunID()

	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status, "test precondition: reap 必須真的發生")
	require.Equal(t, codemeta.ProcurementRunFailureReasonWorkerLost.Value, reaped.FailureReason)

	// ledger run 也必須被終態化——「終態必須寫進 ledger，不論由哪一條路徑產生」是硬約束。
	status, failureReason := ledgerRunStatus(ctx, t, repo, runID)
	require.Equal(t, codemeta.ProcurementRunStatusFailed.Value, status,
		"reap 是這個 run 的終態寫入，agent_runs 那一列不能還停在 QUEUED")
	require.Equal(t, codemeta.ProcurementRunFailureReasonWorkerLost.Value, failureReason)

	events := ledgerEventTypes(ctx, t, repo, runID)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunAccepted.Value, events[0])
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFailed.Value, events[len(events)-1],
		"ledger 最後一筆必須是終態事件——那是串流收線訊號的唯一來源")

	// payload 要帶得出 reason，前端 onRunFailed 才顯示得出人話而非機器碼。
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT public_payload_json FROM procurement_agent_run_events
		  WHERE run_id = $1 ORDER BY seq DESC LIMIT 1`, runID,
	).Scan(&payload))
	require.JSONEq(t, `{"failureReason":"WORKER_LOST"}`, string(payload))
}

// TestReapCandidateSet_TerminalizesCurrentAttemptOnly 是 B11 必須排在 B9 之後的原因：
// 「reap 要 terminalize 哪一個 attempt」的答案取決於身分模型。1:N 之後答案是唯一的——
// 掛掉的是 candidate_set 目前掛著的那一次嘗試，被取代的舊嘗試早就有自己的終態了，不能
// 被重複寫一次。
func TestReapCandidateSet_TerminalizesCurrentAttemptOnly(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)
	firstRunID := set.PublicRunID()

	_, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	firstEvents := ledgerEventTypes(ctx, t, repo, firstRunID)

	// 第二次嘗試同樣配一個立刻過期的 lease，讓它也被 reap。
	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", -1*time.Second)
	require.NoError(t, err)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, retried))
	secondRunID := retried.PublicRunID()

	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "FAILED", reaped.Status, "test precondition: 第二次 reap 必須真的發生")

	secondStatus, secondReason := ledgerRunStatus(ctx, t, repo, secondRunID)
	require.Equal(t, codemeta.ProcurementRunStatusFailed.Value, secondStatus,
		"第二次嘗試才是掛掉的那一個，它必須被終態化")
	require.Equal(t, codemeta.ProcurementRunFailureReasonWorkerLost.Value, secondReason)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		ledgerEventTypes(ctx, t, repo, secondRunID)[len(ledgerEventTypes(ctx, t, repo, secondRunID))-1])

	require.Equal(t, firstEvents, ledgerEventTypes(ctx, t, repo, firstRunID),
		"被取代的第一次嘗試已經有自己的終態，第二次 reap 不得再往它身上寫任何事件")
}

// TestReapCandidateSet_DoesNotProduceErrorRecoveryArtifact 守的是一個**刻意的不對稱**，
// 不是漏做的事（2026-09-07 決策，選項 A）。
//
// ErrorRecoveryData 只有 reason／message／retryable 三欄，而這三欄在 RUN_FAILED 的
// ledger payload 裡已經有了；artifact 額外提供的 version／checksum／可回放，對「worker
// 掛了、重試一次就好」這種瞬時基礎設施故障沒有消費端。要產它就得讓 producer 從 usecase
// 下沉到 repository 的交易、或讓 reap 上浮到 usecase，兩種都動分層，而 reap 是惰性的——
// 會讓買家的一次 GET 變成更多張表的寫入交易。
//
// 兩種失敗成因因此在 **ledger 層對稱、artifact 層刻意不對稱**。這條測試存在的理由是讓
// 之後看到不對稱的人知道那是決策而不是缺陷；要翻案請先改 ADR。
func TestReapCandidateSet_DoesNotProduceErrorRecoveryArtifact(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunAttemptFixture(ctx, t, repo, buyerID, -1*time.Second)

	_, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)

	var artifacts int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_artifacts WHERE run_id = $1`, set.PublicRunID(),
	).Scan(&artifacts))
	require.Zero(t, artifacts,
		"reap 只補 ledger 終態，不產 ERROR_RECOVERY artifact（2026-09-07 決策選項 A）")
}

// TestReapCandidateSet_WithoutLedgerRun_StillReapsCandidateSet 守的是「ledger 是投影不是
// SSOT」這條既有取捨在 reap 這一側也成立。
//
// 沒有對應 agent_runs 列的 candidate_set 真實存在：expand 階段之前建立的舊列（run_id 為
// NULL），以及 CreateLedgerRunForCandidateSearch 失敗那條只記 log 的路徑。終態化在那裡
// 影響 0 列，這**不是錯誤**——不能讓它把買家的一次 GET 變成 500，也不能讓 candidate_set
// 的 reap 跟著 rollback（那會讓那一列永遠卡在 QUEUED，沒有任何路徑能自我修復）。
func TestReapCandidateSet_WithoutLedgerRun_StillReapsCandidateSet(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	// 刻意不呼叫 CreateLedgerRunForCandidateSearch：這一列在 ledger 上不存在。
	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: -1 * time.Second,
	})
	require.NoError(t, err)

	reaped, _, err := repo.GetCandidateSetWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err, "ledger run 不存在不得讓買家的讀取失敗")
	require.Equal(t, "FAILED", reaped.Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonWorkerLost.Value, reaped.FailureReason)

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, "REQUIREMENT_READY", gotReq.Status,
		"reap 的另一半（request 狀態退回）同樣必須 commit，不能被缺席的 ledger 投影拖著 rollback")
}
