//go:build integration

package repository

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// TestClaimRun_ConcurrentSameKey_UniqueViolationRecovered is Fix round 1
// (code review)'s target: the original TestClaimRun_ConcurrentSameKey_
// OnlyOneRunCreated below only launches N goroutines close together —
// verified empirically (8 goroutines, a start-gate barrier, 5 repeated
// runs) that this NEVER actually forces the DB-level race even against
// the pre-fix code: connection-establishment jitter (the pool has
// MinConns=0, so N concurrent pool.Begin() calls each dial a fresh
// connection) dwarfs the few sub-millisecond round trips inside
// ClaimRun's transaction, so in practice one goroutine's whole
// transaction commits before the others even finish dialing — every
// later caller just hits the clean "existing key found" path and the
// race window described below is never entered. A test that only
// probabilistically covers a bug is not a regression guard.
//
// This test instead forces the exact race deterministically, with no
// reliance on goroutine-scheduling luck: it manually drives a "winner"
// transaction, via raw gen.Queries on its own pgx.Tx, to the exact point
// ClaimRun itself would reach mid-transaction — run row inserted, run
// command row inserted for this idempotency key — and deliberately does
// NOT commit yet. It then calls the real repo.ClaimRun concurrently for
// the SAME (buyer_id, client_request_id); ClaimRun's own SELECT (running
// under READ COMMITTED) cannot see the winner's uncommitted insert, so it
// takes the "not found" branch and tries to INSERT the same idempotency
// key — this is real Postgres unique-index "value locking" behavior: the
// INSERT blocks on the winner's uncommitted row instead of erroring
// immediately. The test polls pg_stat_activity (not a fixed sleep) until
// it observes ClaimRun's backend genuinely parked on that lock, THEN
// commits the winner — only now does Postgres re-check the unique index,
// find the winner's row committed, and raise 23505 on ClaimRun's blocked
// INSERT exactly as review described. ClaimRun's own error handling must
// turn that into a clean recovered result, not propagate the raw error.
func TestClaimRun_ConcurrentSameKey_UniqueViolationRecovered(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	in := ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-race", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	}

	// Manually drive the "winner" side of the race on its own transaction,
	// stopping right before COMMIT — this reproduces exactly the mid-
	// transaction state a real concurrent ClaimRun call races against.
	winnerTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = winnerTx.Rollback(ctx) }) // no-op once committed below
	winnerQ := gen.New(winnerTx)

	winnerRunID := repo.newID()
	_, err = winnerQ.CreateProcurementAgentRun(ctx, gen.CreateProcurementAgentRunParams{
		ID: winnerRunID, BuyerID: int64(buyerID),
		RunType: string(in.RunType), Status: domain.RunStatusQueued(), LastEventSeq: 1,
		ControllerKind: string(in.ControllerKind),
	})
	require.NoError(t, err)
	_, err = winnerQ.CreateProcurementAgentRunCommand(ctx, gen.CreateProcurementAgentRunCommandParams{
		ID: repo.newID(), BuyerID: int64(buyerID), CommandKind: in.CommandKind,
		ClientRequestID: in.ClientRequestID, PayloadHash: in.PayloadHash,
		RunID: winnerRunID, Status: "ACCEPTED",
	})
	require.NoError(t, err) // winner's idempotency-key row exists but is NOT committed yet

	type claimResult struct {
		run   Run
		isNew bool
		err   error
	}
	resultCh := make(chan claimResult, 1)
	go func() {
		run, isNew, claimErr := repo.ClaimRun(ctx, in)
		resultCh <- claimResult{run, isNew, claimErr}
	}()

	// Poll (not sleep) until ClaimRun's own INSERT is observed genuinely
	// blocked on the winner's uncommitted row — confirms the race window
	// is actually open before we commit, instead of guessing a duration.
	require.Eventually(t, func() bool {
		var blocked int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND state = 'active'
			  AND query ILIKE '%procurement_agent_run_commands%'
		`).Scan(&blocked); err != nil {
			return false
		}
		return blocked > 0
	}, 5*time.Second, 5*time.Millisecond,
		"ClaimRun 的 goroutine 必須真的卡在等 winner tx 的 row lock 上，這個測試才有意義")

	require.NoError(t, winnerTx.Commit(ctx))

	res := <-resultCh
	require.NoError(t, res.err,
		"撞到 23505 之後必須被 recoverClaimedRun 轉成乾淨結果，不能把裸 DB 錯誤丟給呼叫端")
	require.Equal(t, winnerRunID, res.run.ID, "recover 出來的 run 必須跟贏家 commit 的那個一致")
	require.False(t, res.isNew, "這次呼叫沒有真的建立新 run——run 是贏家那筆交易建的")

	var cmdCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(buyerID), in.ClientRequestID,
	).Scan(&cmdCount))
	require.Equal(t, int64(1), cmdCount, "輸家的交易必須完整 ROLLBACK，不留下第二筆 run command")

	var runCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE buyer_id = $1`, int64(buyerID),
	).Scan(&runCount))
	require.Equal(t, int64(1), runCount, "輸家的交易必須完整 ROLLBACK，不留下它自己建的那筆 run")
}

// TestClaimRun_ConcurrentSameKey_OnlyOneRunCreated is a broader (if only
// probabilistic — see the deterministic test above for the actual
// regression guard) concurrency smoke test: many real ClaimRun calls
// racing the same idempotency key through the public API, gated on a
// shared start signal so they launch as close together as Go's scheduler
// allows, must still converge on exactly one run with no caller ever
// seeing a raw DB error.
func TestClaimRun_ConcurrentSameKey_OnlyOneRunCreated(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	in := ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	}

	const n = 8
	var startWG sync.WaitGroup // 確認每個 goroutine 都已經跑到 gate 前
	var doneWG sync.WaitGroup
	start := make(chan struct{})
	results := make([]Run, n)
	isNews := make([]bool, n)
	errs := make([]error, n)

	startWG.Add(n)
	for i := 0; i < n; i++ {
		doneWG.Add(1)
		go func(idx int) {
			defer doneWG.Done()
			startWG.Done()
			<-start // 全部一次放行，讓各自的第一個 SELECT 盡量同時發生
			results[idx], isNews[idx], errs[idx] = repo.ClaimRun(ctx, in)
		}(i)
	}
	startWG.Wait() // 確認 n 個 goroutine 都已經卡在 gate 上，才開閘
	close(start)
	doneWG.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "goroutine %d 不得看到裸 DB 錯誤（23505 unique_violation 必須被轉成乾淨的冪等回放）", i)
	}

	newCount := 0
	for i := 1; i < n; i++ {
		require.Equal(t, results[0].ID, results[i].ID, "%d 個併發 claim 必須拿到同一個 run", n)
	}
	for _, isNewFlag := range isNews {
		if isNewFlag {
			newCount++
		}
	}
	require.Equal(t, 1, newCount, "%d 個併發 claim 中只能有一個是真正新建", n)

	var cmdCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(buyerID), "req-1",
	).Scan(&cmdCount))
	require.Equal(t, int64(1), cmdCount, "無論哪個先寫贏，DB 只能留下一筆 run command")

	var runCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE buyer_id = $1`, int64(buyerID),
	).Scan(&runCount))
	require.Equal(t, int64(1), runCount, "無論哪個先寫贏，DB 只能留下一筆 run——輸家的交易必須完整 ROLLBACK")
}

func TestClaimRun_SameKeyDifferentHash_ConflictError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	_, _, err = repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-2", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

// 同 key、**同 hash**、只有 command kind 不同：必須衝突，不能把 SUBMIT_TURN
// 那一次的 run 回放給 SUBMIT_ANSWER。這條今天靠三支 canonical hash 的 struct
// 形狀互不相同而附帶成立，但那是巧合——ClaimRun 要自己擋，判準跟 ResumeRun
// 的 existingCmd.CommandKind 檢查一致。
func TestClaimRun_SameKeyDifferentCommandKind_ConflictError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	first, isNew, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.True(t, isNew)

	_, _, err = repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-1", CommandKind: "SUBMIT_ANSWER",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	var cmdCount, runCount int64
	var storedKind string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = 'req-1'`, int64(buyerID),
	).Scan(&cmdCount))
	require.Equal(t, int64(1), cmdCount, "衝突那次不得新增第二筆 command")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE buyer_id = $1`, int64(buyerID),
	).Scan(&runCount))
	require.Equal(t, int64(1), runCount, "衝突那次不得建立第二個 run（零 mutation）")
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT command_kind FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = 'req-1'`, int64(buyerID),
	).Scan(&storedKind))
	require.Equal(t, "SUBMIT_TURN", storedKind, "既有 command 的 kind 不得被後來者改寫")
	require.NotZero(t, first.ID)
}

func TestAppendRunEvent_SeqMonotonic(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "req-2", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	ev2, err := repo.AppendRunEvent(ctx, AppendRunEventInput{
		RunID: run.ID, BuyerID: identity.BuyerID(1), EventType: "RUN_STARTED",
		SchemaVersion: "v1", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), ev2.Seq, "第一筆 event（RUN_ACCEPTED）已經是 seq=1，第二筆必須是 2")
}

// TestTerminalizeRun_Success_WritesTerminalEventAndStatus 驗證 CAS 命中時
// TerminalizeRun 在同一交易內把 status／version 更新與 terminal event append
// 一起寫下去——version 必須 +1（doc57 §6.2 v1 CAS 欄位），last_event_seq 必須
// bump 到新寫入的 terminal event 的 seq。
func TestTerminalizeRun_Success_WritesTerminalEventAndStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-3", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), run.Version, "DB default，ClaimRun 新建的 run 起始 version 必須是 1")

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version,
		Status: domain.RunStatusSucceeded(), TerminalEventType: "RUN_SUCCEEDED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote, "version 相符時 CAS 必須成功")

	var status string
	var version, lastEventSeq int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, version, last_event_seq FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&status, &version, &lastEventSeq))
	require.Equal(t, domain.RunStatusSucceeded(), status)
	require.Equal(t, int64(2), version, "CAS 成功後 version 必須 +1")
	require.Equal(t, int64(2), lastEventSeq, "terminal event（接在 seq=1 的 RUN_ACCEPTED 之後）必須 bump last_event_seq")
}

// TestTerminalizeRun_AlreadyTerminal_NoWriteNoMutation 取代舊的
// TestTerminalizeRun_VersionMismatch_NoWriteNoMutation（2026-09-07 決策：
// terminalize 不對 version 做 CAS，改用狀態機 guard）。
//
// 守住的保護意圖完全不變——terminalize **不得覆寫一個已經終態的 run**，且被擋下
// 時零 mutation（status 沒被改、沒有第二筆 terminal event、last_event_seq 沒被
// bump）。變的只是觸發條件：從「呼叫端手上的 version 對不上」換成「這個 run 已經
// 是終態」。version 對不上不再是拒絕的理由，見
// TestTerminalizeRun_AfterCancelBumpedVersion_StillWritesTerminalEvent；而
// cancel／resume 那條「拿著快照要改它」的路徑仍然保留 version CAS，見
// TestCancelRun_StaleExpectedVersion_NonTerminalRun_ReturnsVersionConflict。
func TestTerminalizeRun_AlreadyTerminal_NoWriteNoMutation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-4", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID,
		Status: domain.RunStatusSucceeded(), TerminalEventType: "RUN_SUCCEEDED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)

	var seqAfterFirst int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_event_seq FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&seqAfterFirst))

	// 第二次終結：run 已經是終態，狀態守衛必須擋下。
	wrote, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID,
		Status: domain.RunStatusFailed(), TerminalEventType: "RUN_FAILED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err, "被狀態守衛擋下不是錯誤，只是 wrote=false")
	require.False(t, wrote)

	var status string
	var lastEventSeq int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, last_event_seq FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&status, &lastEventSeq))
	require.Equal(t, domain.RunStatusSucceeded(), status, "已終態的 run 的 status 不得被覆寫")
	require.Equal(t, seqAfterFirst, lastEventSeq, "不得寫入第二筆 terminal event 或 bump seq")
}

// TestTerminalizeRun_AfterCancelBumpedVersion_StillWritesTerminalEvent 守住
// 2026-09-07 決策：terminalize 不對 version 做 CAS，改用狀態機 guard。
//
// CancelRun 走的是跟 TerminalizeRun 同一支 UpdateProcurementAgentRunStatus，
// 而那支 query 是 `version = version + 1`——所以買家一按取消，worker 手上那份
// 建立當下的 version 就過期了。舊版 TerminalizeRun 拿它做 CAS，於是終態整個
// 寫不進去，且因為 wrote=false 不是錯誤，呼叫端只記一行 Warn：**該 run 永遠
// 不會有終態 ledger event**，串流消費端沒有收線訊號，連線開到 gateway 的
// 120 秒 deadline。
//
// worker 宣告自己的執行結果不是 read-modify-write，正確的守衛是狀態轉移合法性
// （run 還不是終態），不是呼叫端手上的版本號。CANCEL_REQUESTED 不在終態集合裡，
// 所以這次 terminalize 必須成立。
func TestTerminalizeRun_AfterCancelBumpedVersion_StillWritesTerminalEvent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-terminalize-after-cancel", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), run.Version)

	// 買家按下取消：run 仍非終態（CANCEL_REQUESTED 不在終態集合），但 version
	// 已被推進，worker 手上那份快照就此過期。
	_, disposition, err := repo.CancelRun(ctx, CancelRunInput{
		RunID: run.ID, BuyerID: buyerID, ExpectedVersion: run.Version,
	})
	require.NoError(t, err)
	require.Equal(t, CancelDispositionRequested, disposition)

	// worker 這時才跑完，帶著建立當下的 version 來終結自己。
	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version,
		Status: domain.RunStatusSucceeded(), TerminalEventType: "RUN_FINISHED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote, "run 仍非終態，terminalize 必須成立——守衛是狀態轉移合法性，不是 version")

	events, err := repo.ListRunEventsAfterSeq(ctx, run.ID, 0)
	require.NoError(t, err)
	types := make([]string, 0, len(events))
	for _, ev := range events {
		types = append(types, ev.EventType)
	}
	require.Contains(t, types, "RUN_FINISHED",
		"終態 ledger event 必須存在，否則串流消費端永遠等不到收線訊號")
}

// TestTerminalizeRun_ParkIntoWaiting_MustNotEraseCancel 是實作 2026-09-07 那個決策時
// 發現的反例探針：TerminalizeRun 除了寫真正的終態，也被 answer.go／graph_dispatch.go
// ／rewind.go 用來把 run 停進 WAITING_FOR_INPUT——那**不是終態**，run 還活著。
// 對這條分支，「worker 只是在陳述已發生的事實」不成立：一個併發的 cancel 是真實的
// 競爭意圖，不得被 park 覆寫掉。
func TestTerminalizeRun_ParkIntoWaiting_MustNotEraseCancel(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-park-vs-cancel", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	_, disposition, err := repo.CancelRun(ctx, CancelRunInput{
		RunID: run.ID, BuyerID: buyerID, ExpectedVersion: run.Version,
	})
	require.NoError(t, err)
	require.Equal(t, CancelDispositionRequested, disposition)

	// worker 這時才跑完該輪，想把 run 停進 WAITING 等買家回覆 interrupt。
	_, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version,
		Status: domain.RunStatusWaiting(), TerminalEventType: "RUN_FINISHED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&status))
	require.Equal(t, domain.RunStatusCancelReq(), status,
		"買家的取消不得被 park into WAITING 覆寫掉")
}

// TestCreateLedgerRunForCandidateSearch_ProjectsIntoRunLedger 是 Task 10 的核心
// repository 層驗證：對一個真實建立的 candidate_set 呼叫這支函式，
// procurement_agent_runs 要真的多一列——id 用 set.PublicRunID()（不是
// set.ID，兩者刻意不同值，見設計書 57 §1.2-3），run_type 是 CANDIDATE_SEARCH，
// domain_ref 指向 candidateSetId，且首筆 RUN_ACCEPTED event 已經在同一交易內
// 寫進去（last_event_seq=1）。最後驗證這筆 ledger run 確實能被既有的
// TerminalizeRun（Task 3-4 的既有介面，Task 7-9 也在用）收斂終態。⚠️ 這裡**不再**
// 是在釘「version=1 當 CAS 依據」——終態分支已改成狀態機 guard，candidate_search.go
// 的 run() 也不再帶 Version（見 TestTerminalizeRun_AfterCancelBumpedVersion_
// StillWritesTerminalEvent）。本測試現在釘的是「這筆 ledger run 建得起來、而且終態
// 收斂得掉」，值得在真實 DB 上驗而不是只靠 mock 相信它。
func TestCreateLedgerRunForCandidateSearch_ProjectsIntoRunLedger(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(42)
	req, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, set))

	var runType, domainRefType, domainRefID, status string
	var runBuyerID, lastEventSeq, version int64
	var conversationID pgtype.Int8
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT run_type, domain_ref_type, domain_ref_id, status, buyer_id, last_event_seq, version, conversation_id
		 FROM procurement_agent_runs WHERE id = $1`, set.PublicRunID(),
	).Scan(&runType, &domainRefType, &domainRefID, &status, &runBuyerID, &lastEventSeq, &version, &conversationID))

	require.Equal(t, "CANDIDATE_SEARCH", runType)
	require.Equal(t, "candidateSetId", domainRefType)
	require.Equal(t, strconv.FormatInt(set.ID, 10), domainRefID, "domain_ref_id 必須指向 candidateSetId（set.ID），不是 run 自己的 id（set.PublicRunID()）")
	require.Equal(t, domain.RunStatusQueued(), status)
	require.Equal(t, int64(buyerID), runBuyerID)
	require.Equal(t, int64(1), lastEventSeq)
	require.Equal(t, int64(1), version, "DB DEFAULT：新建的 ledger run 起始 version 必須是 1（終態轉移已不依賴這個值，但 cancel／resume 的 CAS 仍然依賴）")

	// Task 15 write-path 修復：候選搜尋 run 跟 TURN／ANSWER／REWIND 一樣屬於某個
	// conversation，這裡驗證 CreateLedgerRunForCandidateSearch 真的把
	// set.RequestID 解析出來的 conversation_id 寫進去了，不是留 NULL。
	require.True(t, conversationID.Valid, "候選搜尋 run 也該有 conversation_id，讓它出現在 runtime-snapshot 的 runs[] 裡")
	var wantConvID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, req.ID,
	).Scan(&wantConvID))
	require.Equal(t, wantConvID, conversationID.Int64)

	var eventSeq int64
	var eventType string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT seq, event_type FROM procurement_agent_run_events WHERE run_id = $1`, set.PublicRunID(),
	).Scan(&eventSeq, &eventType))
	require.Equal(t, int64(1), eventSeq)
	require.Equal(t, "RUN_ACCEPTED", eventType)

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: set.PublicRunID(), BuyerID: buyerID,
		Status: domain.RunStatusSucceeded(), TerminalEventType: "RUN_FINISHED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote, "這筆 ledger run 建得起來、而且終態收斂得掉——終態守衛是狀態轉移合法性，不帶 Version")
}

// TestCreateLedgerRunForCandidateSearch_DuplicateRunID_ReturnsUniqueViolation
// 驗證重複對同一個 run_id 呼叫這支函式會撞 procurement_agent_runs 的 primary
// key 而失敗，不是靜默成功或覆寫——這是「StartSearch 只在新建 candidate_set
// 那條路徑呼叫一次，重試路徑刻意不呼叫」這個設計決策的正確性前提（見
// candidate_search.go StartSearch 的呼叫點註解）。
func TestCreateLedgerRunForCandidateSearch_DuplicateRunID_ReturnsUniqueViolation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, set))

	err := repo.CreateLedgerRunForCandidateSearch(ctx, set)
	require.Error(t, err, "第二次對同一個 run_id 呼叫必須失敗（primary key 衝突），不能靜默通過")
}

// TestCancelRun_TerminalRun_ReturnsTooLate 驗證 doc57 §5.4：對已經 SUCCEEDED
// 的 run 呼叫 CancelRun 必須回 TOO_LATE，不得謊報取消成功或改動 run status。
func TestCancelRun_TerminalRun_ReturnsTooLate(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "req-3", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: identity.BuyerID(1), Version: run.Version,
		Status: domain.RunStatusSucceeded(), TerminalEventType: "RUN_FINISHED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)

	_, disposition, err := repo.CancelRun(ctx, CancelRunInput{RunID: run.ID, BuyerID: identity.BuyerID(1), ExpectedVersion: run.Version + 1})
	require.NoError(t, err)
	require.Equal(t, CancelDispositionTooLate, disposition)
}

// TestCancelRun_StaleExpectedVersion_NonTerminalRun_ReturnsVersionConflict is
// the regression guard for a code-review-found bug: the run genuinely
// transitioned (QUEUED -> RUNNING, same version-CAS machinery
// TerminalizeRun/CancelRun both use) so its version moved on from what the
// caller still has, but its status is very much NOT terminal. The old
// CAS-miss branch in CancelRun unconditionally reported
// CancelDispositionAlreadyTerminal here — lying to the caller that a
// still-running run had already reached a final state. CancelRun must
// instead recognize the run is genuinely non-terminal/non-CANCEL_REQUESTED
// and refuse to report any of doc57 §5.4's four dispositions for it, since
// none of them are true; it returns ErrCancelVersionConflict and performs
// zero mutation.
func TestCancelRun_StaleExpectedVersion_NonTerminalRun_ReturnsVersionConflict(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "req-6", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	staleVersion := run.Version // 呼叫端手上這份快照，接下來馬上就會變舊

	// 讓 run 真的往前轉一次狀態（QUEUED -> RUNNING，走跟 CancelRun／
	// TerminalizeRun 同一支 UpdateProcurementAgentRunStatus CAS）——這一步之後
	// version 往前推進，但 status 仍然是非終態。
	affected, err := repo.q(ctx).UpdateProcurementAgentRunStatus(ctx, gen.UpdateProcurementAgentRunStatusParams{
		ID: run.ID, Version: run.Version, Status: domain.RunStatusRunning(),
		FailureReason: "", ResultRevisionID: pointerInt64(0),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	_, _, err = repo.CancelRun(ctx, CancelRunInput{RunID: run.ID, BuyerID: identity.BuyerID(1), ExpectedVersion: staleVersion})
	require.ErrorIs(t, err, ErrCancelVersionConflict,
		"run 仍非終態，CAS 沒過必須回版本衝突，不能謊報成 ALREADY_TERMINAL 或任何一個 doc57 §5.4 的四值 disposition")

	var status string
	var version int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, version FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&status, &version))
	require.Equal(t, domain.RunStatusRunning(), status, "CancelRun 的版本衝突分支不得 mutate run status")
	require.Equal(t, int64(2), version, "CancelRun 的版本衝突分支不得再次 bump version（零 mutation）")
}

// resumeReadyParentFixture 造一個 ResumeRun 的 T0 guard（Task 14b §5.2 步驟
// 6-8）真正要求的前置狀態：一個 WAITING_FOR_INPUT、帶著真實
// result_revision_id 的 parent run，掛一筆 PENDING interrupt。跟真實環境
// （executeAnswerPipeline／executeTurnPipeline 的 terminalStatus 賦值順序）
// 同一種組法——先 CreateRequest 拿到一個真的 revision，再用公開的
// ClaimRun／TerminalizeRun／CreateInterrupt API 組出這個狀態，不手寫 SQL
// 繞過應用層邏輯。
func resumeReadyParentFixture(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID, clientRequestID string) (parent Run, interrupt Interrupt, revisionID int64) {
	t.Helper()
	parent, interrupt, revisionID, _ = resumeReadyParentFixtureWithRequest(ctx, t, repo, buyerID, clientRequestID)
	return parent, interrupt, revisionID
}

func resumeReadyParentFixtureWithRequest(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID, clientRequestID string) (parent Run, interrupt Interrupt, revisionID, requestID int64) {
	t.Helper()
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	// RequestID 帶進 ClaimRun（Task 15 write-path 修復）：這個 fixture 模擬的是
	// 續接既有 request 的 TURN run，跟 answer.go/rewind.go 的真實呼叫端一樣
	// RequestID 恆非零，所以 parent.ConversationID 現在會是真的解析出來的值
	// （見 TestResumeRun_ChildInheritsParentConversationID），不再是零值。
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: clientRequestID, CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-seed-" + clientRequestID, RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusWaiting(),
		ResultRevisionID: rev.ID, TerminalEventType: "CONFIRMATION_REQUIRED", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)

	interrupt, err = repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
	})
	require.NoError(t, err)

	parent, err = repo.GetRun(ctx, run.ID, buyerID)
	require.NoError(t, err)
	return parent, interrupt, rev.ID, req.ID
}

// fixedResolveInput 是 dbtest 共用的 ResolveInput 替身：只驗證它真的在 T0
// 交易內被呼叫過（見各測試對 InputRefJSON／InputSchemaVersion 的斷言），不
// 重新測 usecase 層 ClaimAndDispatchResume 那個真正的 kind／payload 解碼
// 邏輯（那在 internal/procurement/usecase/resume_test.go 測）。
func fixedResolveInput() func(context.Context, ResumeResolveContext) (ResolvedRunInput, error) {
	return func(_ context.Context, rc ResumeResolveContext) (ResolvedRunInput, error) {
		return ResolvedRunInput{
			InputRefJSON:       []byte(`{"optionId":"crop-a"}`),
			InputSchemaVersion: rc.ResponseSchemaVersion,
			RetentionClass:     "BUYER_INPUT",
		}, nil
	}
}

// TestResumeRun_SameInterruptTwice_OnlyOneChild 驗證 doc57 §5.5：同一
// interrupt 用同一組 idempotency key 重送 ResumeRun 兩次，必須回同一個 child
// run，且不能因為第二次呼叫又建出第二個 child。
func TestResumeRun_SameInterruptTwice_OnlyOneChild(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, interrupt, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-4")

	out1, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-1", PayloadHash: "hash-a", ResponsePayload: []byte(`{"optionId":"crop-a"}`),
		ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out1.IsNew)

	out2, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-1", PayloadHash: "hash-a", ResponsePayload: []byte(`{"optionId":"crop-a"}`),
		ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.False(t, out2.IsNew)
	require.Equal(t, out1.Child.ID, out2.Child.ID, "同 key 重送必須回同一個 child run")
}

// TestResumeRun_ParentResultRevisionIDNotWiped 是 Task 14b C1 的 dbtest 釘子：
// resume 把 parent 轉 RESUMED 之後，parent 原本的 result_revision_id
// （replayExistingRun 的就緒判準）必須維持不變，不能被抹成字面值 0（v1 bug：
// UpdateProcurementAgentRunStatus 無條件 SET result_revision_id = $5，
// ResumeRun 舊實作硬傳 sqlutil.Int8(0)）。
func TestResumeRun_ParentResultRevisionIDNotWiped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, revID := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-c1-resume")

	_, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-c1", PayloadHash: "hash-c1", ResponsePayload: []byte(`{"optionId":"crop-a"}`),
		ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)

	var status string
	var resultRevisionID pgtype.Int8
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, result_revision_id FROM procurement_agent_runs WHERE id = $1`, parent.ID,
	).Scan(&status, &resultRevisionID))
	require.Equal(t, domain.RunStatusResumed(), status)
	require.True(t, resultRevisionID.Valid, "resume 不得把 parent 的 result_revision_id 抹成 NULL——replayExistingRun 的錨點必須保留")
	require.Equal(t, revID, resultRevisionID.Int64)
}

// TestCancelRun_ParkedRun_RejectsWithInterruptPending_ZeroMutation supersedes
// the old TestCancelRun_ParkedRun_ResultRevisionIDNotWiped (Task 14b C1),
// which asserted the very assumption this task's guard closes: that
// cancelling a WAITING_FOR_INPUT run was an allowed transition and only
// needed to preserve result_revision_id across it. That transition orphans
// the run's interrupt (nothing ever closes it afterwards), which is exactly
// the Blocking 3 decision this guard implements — CancelRun must now reject
// outright with ErrCancelInterruptPending, before touching
// classifyCancelDisposition or any status update, and leave the run's
// status/version/result_revision_id genuinely untouched (zero mutation, not
// merely "the revision id survived a mutation that did happen").
func TestCancelRun_ParkedRun_RejectsWithInterruptPending_ZeroMutation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, _, revID := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-guard-cancel-parked")

	_, _, err := repo.CancelRun(ctx, CancelRunInput{RunID: parent.ID, BuyerID: buyerID, ExpectedVersion: parent.Version})
	require.ErrorIs(t, err, ErrCancelInterruptPending)

	var status string
	var version int64
	var resultRevisionID pgtype.Int8
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, version, result_revision_id FROM procurement_agent_runs WHERE id = $1`, parent.ID,
	).Scan(&status, &version, &resultRevisionID))
	require.Equal(t, domain.RunStatusWaiting(), status, "guard 擋下時 run status 不得被動到")
	require.Equal(t, parent.Version, version, "guard 擋下時 version 不得被 bump（零 mutation）")
	require.True(t, resultRevisionID.Valid)
	require.Equal(t, revID, resultRevisionID.Int64, "guard 擋下時 result_revision_id 不得被動到")
}

// TestResumeRun_SameKeyDifferentInterrupt_ReturnsIdempotencyConflict 釘住
// Task 14b M4 的第二層防線：即使兩次呼叫剛好帶著逐位元組相同的 PayloadHash
// 字面值，但分別指向兩個不同的 interrupt，repository 步驟 2 的一致性斷言
// （existingCmd.RunID == interruptRow.ResolvedByChildRunID.Int64）也要擋下
// 第二次呼叫——不能讓「key 重用到另一個 interrupt」被誤判成冪等重送。
func TestResumeRun_SameKeyDifferentInterrupt_ReturnsIdempotencyConflict(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, interrupt1, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-m4-1")
	_, interrupt2, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-m4-2")

	out1, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt1.ID, BuyerID: buyerID,
		ClientRequestID: "cr-m4-shared", PayloadHash: "hash-shared", ResponsePayload: []byte(`{"optionId":"crop-a"}`),
		ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out1.IsNew)

	_, err = repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt2.ID, BuyerID: buyerID,
		ClientRequestID: "cr-m4-shared", PayloadHash: "hash-shared", ResponsePayload: []byte(`{"optionId":"crop-b"}`),
		ResolveInput: fixedResolveInput(),
	})
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_agent_run_interrupts WHERE id = $1`, interrupt2.ID,
	).Scan(&status))
	require.Equal(t, "PENDING", status, "interrupt2 必須完全沒被動到——零 mutation")
}

// TestResumeRun_ResolveInputFails_ZeroMutation 釘住 Task 14b M5：ResolveInput
// 在任何 mutation 之前於交易內執行，失敗必須完整 ROLLBACK——parent 仍是
// WAITING_FOR_INPUT、interrupt 仍是 PENDING、沒有 child、沒有 command row。
func TestResumeRun_ResolveInputFails_ZeroMutation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-m5")

	boom := errors.New("unsupported interrupt kind")
	_, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-m5", PayloadHash: "hash-m5", ResponsePayload: []byte(`{}`),
		ResolveInput: func(context.Context, ResumeResolveContext) (ResolvedRunInput, error) {
			return ResolvedRunInput{}, boom
		},
	})
	require.ErrorIs(t, err, boom)

	var parentStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_agent_runs WHERE id = $1`, parent.ID,
	).Scan(&parentStatus))
	require.Equal(t, domain.RunStatusWaiting(), parentStatus, "ResolveInput 失敗必須零 mutation，parent 狀態不得被改動")

	var interruptStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_agent_run_interrupts WHERE id = $1`, interrupt.ID,
	).Scan(&interruptStatus))
	require.Equal(t, "PENDING", interruptStatus)

	var childCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE parent_run_id = $1`, parent.ID,
	).Scan(&childCount))
	require.Equal(t, int64(0), childCount, "ResolveInput 失敗不得建立 child run")
}

// TestResumeRun_ParentNotWaiting_ReturnsErrRunNotResumable 釘住步驟 7 的
// guard：interrupt 本身仍是 PENDING，但它所屬的 parent 不處於
// WAITING_FOR_INPUT（這裡直接測 guard 本身的行為，不依賴
// 「CreateInterrupt 只在 terminalize 成 WAITING 那筆交易內被呼叫」這個
// production 不變式）。
func TestResumeRun_ParentNotWaiting_ReturnsErrRunNotResumable(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "req-notwaiting", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	interrupt, err := repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
	})
	require.NoError(t, err)

	_, err = repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-notwaiting", PayloadHash: "hash-nw", ResponsePayload: []byte(`{}`),
		ResolveInput: fixedResolveInput(),
	})
	require.ErrorIs(t, err, ErrRunNotResumable, "parent 仍是 QUEUED（不是 WAITING_FOR_INPUT），resume 必須被 guard 擋下")
}

// ---- FindOpenInterruptForRequest（Task 14b B3，M-1(b) 補測）----
//
// 這支 query 是 /answers 與 /crop-disambiguation 寫入前判斷「這個 request 是否
// 被一個尚未回覆的 interrupt 佔用」的唯一依據，在這輪之前完全沒有對真實
// Postgres 跑過——sqlc 只驗證得出型別/欄位對不對，驗不出語意，而
// `r.status = 'WAITING_FOR_INPUT'` 這個 predicate（design §7.3：讓孤兒
// interrupt 不會鎖死 legacy 路徑）恰好是唯一沒辦法從型別檢查看出對錯的部分。

// openInterruptFixture 是 resumeReadyParentFixture 的姊妹版：多回傳
// FindOpenInterruptForRequest 需要的 requestID。刻意不擴充
// resumeReadyParentFixture 本身的回傳形狀——那個既有 helper 已有 7 個呼叫點，
// 只有這組新測試需要 requestID，用同一套組法另開一個 helper比改動既有簽章更小。
func openInterruptFixture(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID, clientRequestID string) (requestID int64, parent Run, interrupt Interrupt) {
	t.Helper()
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: clientRequestID, CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-seed-" + clientRequestID, RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusWaiting(),
		ResultRevisionID: rev.ID, TerminalEventType: "CONFIRMATION_REQUIRED", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)

	interrupt, err = repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
	})
	require.NoError(t, err)

	parent, err = repo.GetRun(ctx, run.ID, buyerID)
	require.NoError(t, err)
	return req.ID, parent, interrupt
}

// TestFindOpenInterruptForRequest_OpenInterrupt_Found is the query's one
// true-positive case: a PENDING interrupt on a WAITING_FOR_INPUT parent must
// be found for the owning buyer.
func TestFindOpenInterruptForRequest_OpenInterrupt_Found(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	requestID, _, interrupt := openInterruptFixture(ctx, t, repo, buyerID, "req-find-open")

	found, err := repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.NoError(t, err)
	require.Equal(t, interrupt.ID, found.ID)
}

// TestFindOpenInterruptForRequest_OrphanInterrupt_NotFound is the guard's
// design-mandated failure-direction choice (design §7.3): if the parent left
// WAITING_FOR_INPUT some other way while its interrupt is still PENDING, the
// query must NOT find it — an orphaned interrupt must never permanently 409
// the buyer's legacy /answers or /crop-disambiguation path. This is the one
// predicate in the query sqlc's type-checking cannot verify, and the one
// this task's review flagged as never having run against real Postgres.
//
// This fixture used to drive the transition via repo.CancelRun — but that is
// exactly the bug Task 14b Blocking 3's CancelRun guard (repository.go's
// ErrCancelInterruptPending) now closes: CancelRun on a WAITING_FOR_INPUT run
// rejects instead of transitioning it away. So this test now forces the
// transition directly via UpdateProcurementAgentRunStatus (same technique
// TestCancelRun_StaleExpectedVersion_NonTerminalRun_ReturnsVersionConflict
// already uses to simulate an out-of-band status move), standing in for
// whatever non-CancelRun path could still produce this state. The invariant
// under test — FindOpenInterruptForRequest must fail-open once the parent is
// no longer WAITING, regardless of how it got there — must hold independent
// of which path caused the transition.
func TestFindOpenInterruptForRequest_OrphanInterrupt_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	requestID, parent, _ := openInterruptFixture(ctx, t, repo, buyerID, "req-find-orphan")

	affected, err := repo.q(ctx).UpdateProcurementAgentRunStatus(ctx, gen.UpdateProcurementAgentRunStatusParams{
		ID: parent.ID, Version: parent.Version, Status: domain.RunStatusCancelReq(),
		FailureReason: "", ResultRevisionID: nil,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected, "前提：這次直接的狀態轉移必須真的命中")

	var parentStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_agent_runs WHERE id = $1`, parent.ID,
	).Scan(&parentStatus))
	require.NotEqual(t, domain.RunStatusWaiting(), parentStatus, "前提：parent 必須真的離開 WAITING_FOR_INPUT")

	_, err = repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.ErrorIs(t, err, ErrNotFound, "interrupt 仍是 PENDING 但 parent 已離開 WAITING_FOR_INPUT，不得回傳它")
}

// TestFindOpenInterruptForRequest_ResolvedInterrupt_NotFound covers the
// other terminal state: once resume has genuinely resolved the interrupt
// (resolved_by_child_run_id set), the query must stop finding it — otherwise
// a buyer who already answered would stay 409'd on the legacy paths forever.
func TestFindOpenInterruptForRequest_ResolvedInterrupt_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	requestID, _, interrupt := openInterruptFixture(ctx, t, repo, buyerID, "req-find-resolved")

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-find-resolved", PayloadHash: "hash-find-resolved", ResponsePayload: []byte(`{}`),
		ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out.IsNew)

	_, err = repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.ErrorIs(t, err, ErrNotFound, "interrupt 已被 resume 解決，不得再被視為 open")
}

// TestFindOpenInterruptForRequest_OtherBuyer_NotFound covers cross-tenant
// scoping: the query must never let one buyer's open interrupt leak into
// another buyer's guard check, even for the same requestID value (request
// IDs are TSIDs, not guessable, but the query's own buyer_id predicate is
// the real boundary and deserves its own direct assertion).
func TestFindOpenInterruptForRequest_OtherBuyer_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	otherBuyerID := identity.BuyerID(2)
	requestID, _, _ := openInterruptFixture(ctx, t, repo, buyerID, "req-find-otherbuyer")

	_, err := repo.FindOpenInterruptForRequest(ctx, requestID, otherBuyerID)
	require.ErrorIs(t, err, ErrNotFound, "別的買家不得看到這個 request 的 open interrupt")
}

// ---- Task 15 write-path 修復：procurement_agent_runs.conversation_id ----
//
// 修復前，run 建立時（ClaimRun／CreateLedgerRunForCandidateSearch）從未寫過
// conversation_id——見 runs.sql FindOpenProcurementInterruptForRequest 既有
// 註解記載的這個既知缺口。以下測試釘住修復後的行為：ClaimRun 對續接既有對話
// 的呼叫（RequestID 非零）能在 claim transaction 內就解出 conversation_id；
// 對全新對話（RequestID 零）誠實地留 NULL，交給 SetRunConversationID 事後
// 回填；ListRunsByConversation／GetOpenInterruptForRun 這兩支 Task 15 新讀取
// 方法本身也各自有測試。

// TestClaimRun_ContinuingConversationRequestID_ResolvesConversationID 驗證
// ClaimRun 對非零 RequestID（ANSWER／REWIND 恆如此、TURN 續接既有對話時也是）
// 真的在同一個 claim transaction 內解出這個 request 早已存在的 conversation_id，
// 不留 NULL。
func TestClaimRun_ContinuingConversationRequestID_ResolvesConversationID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	var wantConvID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, req.ID,
	).Scan(&wantConvID))

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-continuing-1", CommandKind: "SUBMIT_ANSWER",
		PayloadHash: "hash-continuing-1", RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Equal(t, wantConvID, run.ConversationID, "ClaimRun 回傳值本身就要帶著解出來的 conversation_id")

	var persistedConvID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT conversation_id FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&persistedConvID))
	require.Equal(t, wantConvID, persistedConvID)
}

// TestClaimRun_NewConversation_LeavesConversationIDNull 是上一個測試的另一
// 半：RequestID==0（全新對話，TURN 專屬情境）時 claim 當下還沒有 request／
// conversation 可解，conversation_id 必須誠實地留 NULL——不是字面值 0（那會
// 被 idx_procurement_agent_runs_conversation 的 partial index 誤收，也會讓
// ListRunsByConversation 用 conversationID=0 查到不相干的列，見該函式文件）。
func TestClaimRun_NewConversation_LeavesConversationIDNull(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "cr-new-conv-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-new-conv-1", RunType: domain.RunTypeTurn, // RequestID 刻意留零值
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Zero(t, run.ConversationID)

	var conversationID pgtype.Int8
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT conversation_id FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&conversationID))
	require.False(t, conversationID.Valid, "全新對話的 run 建立時 conversation_id 必須是 NULL，不是字面值 0")
}

// TestSetRunConversationID_BackfillsThenIsNoOp 驗證 SetRunConversationID 的
// `conversation_id IS NULL` guard：第一次呼叫真的寫進去，對同一個 run 再呼叫
// 第二次（模擬呼叫端重複觸發回填，或誤用在一個已經有值的 run 上）必須是
// no-op，不覆寫已經正確的值。
func TestSetRunConversationID_BackfillsThenIsNoOp(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "cr-backfill-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-backfill-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Zero(t, run.ConversationID, "前置條件：全新對話，claim 當下還沒有 conversation_id")

	require.NoError(t, repo.SetRunConversationID(ctx, run.ID, 555))
	var got int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT conversation_id FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&got))
	require.Equal(t, int64(555), got)

	// 第二次呼叫帶不同的值——guard 必須擋下，維持第一次寫入的 555，不是 666。
	require.NoError(t, repo.SetRunConversationID(ctx, run.ID, 666))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT conversation_id FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&got))
	require.Equal(t, int64(555), got, "conversation_id 已經有值，第二次呼叫必須是 no-op")
}

// ---- ListRunsByConversation（Task 15）----

// TestListRunsByConversation_ReturnsNewestFirstScopedToBuyer 同時驗證排序
// （newest-first）與租戶邊界（buyer_id）：同一個 conversation 上的兩個 run
// 依 created_at DESC 排列，別的買家（即使碰巧知道同一個 conversationID）查不到。
func TestListRunsByConversation_ReturnsNewestFirstScopedToBuyer(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	turnRun, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-list-turn-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-list-turn-1", RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.NotZero(t, turnRun.ConversationID)

	answerRun, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-list-answer-1", CommandKind: "SUBMIT_ANSWER",
		PayloadHash: "hash-list-answer-1", RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)
	require.Equal(t, turnRun.ConversationID, answerRun.ConversationID, "同一個 request 只會有一個 conversation")

	got, err := repo.ListRunsByConversation(ctx, turnRun.ConversationID, buyerID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, answerRun.ID, got[0].ID, "後建立的 run 排最前面（created_at DESC）")
	require.Equal(t, turnRun.ID, got[1].ID)

	gotOtherBuyer, err := repo.ListRunsByConversation(ctx, turnRun.ConversationID, identity.BuyerID(9999))
	require.NoError(t, err)
	require.Empty(t, gotOtherBuyer, "別的買家不得看到這個 conversation 底下的 run")
}

// TestListRunsByConversation_ZeroConversationID_ReturnsEmpty 是 GetSnapshot
// handler 那條「conversation 目前還解不出來」防禦路徑的正確性依據：SQL 的
// `conversation_id = NULL` 永不為真，所以呼叫端傳 0（int64OrNil 轉出的
// nil）必須安全地查到空清單，不是意外撈出所有 conversation_id IS NULL 的列
// （那會洩漏其他買家尚未歸戶的 run）。
func TestListRunsByConversation_ZeroConversationID_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-zero-conv-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-zero-conv-1", RunType: domain.RunTypeTurn, // 全新對話，conversation_id 留 NULL
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	got, err := repo.ListRunsByConversation(ctx, 0, buyerID)
	require.NoError(t, err)
	require.Empty(t, got, "conversationID=0 必須查到空清單，不是所有 conversation_id IS NULL 的列")
}

// ---- GetOpenInterruptForRun（Task 15）----

// TestGetOpenInterruptForRun_Open_Found 是查詢的一個真陽性案例：PENDING
// 且未被 resume 解決的 interrupt 必須查得到。
func TestGetOpenInterruptForRun_Open_Found(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, parent, interrupt := openInterruptFixture(ctx, t, repo, buyerID, "req-getopen-1")

	got, ok, err := repo.GetOpenInterruptForRun(ctx, parent.ID, buyerID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, interrupt.ID, got.ID)
}

// TestGetOpenInterruptForRun_OtherBuyer_NotFound 是 review round 2 Minor #4
// 補上的租戶邊界測試：即使 runID 正確指向一個真的有未解 interrupt 的 run，
// 別的買家一樣查不到——同 FindOpenInterruptForRequest 的既有邊界保證，這裡是
// GetOpenInterruptForRun 自己的版本（修復前這支查詢沒有 buyer_id 條件，這個
// 案例在修復前會誤判為 ok=true）。
func TestGetOpenInterruptForRun_OtherBuyer_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	otherBuyerID := identity.BuyerID(2)
	_, parent, _ := openInterruptFixture(ctx, t, repo, buyerID, "req-getopen-otherbuyer-1")

	_, ok, err := repo.GetOpenInterruptForRun(ctx, parent.ID, otherBuyerID)
	require.NoError(t, err)
	require.False(t, ok, "別的買家不得看到這個 run 的 open interrupt")
}

// TestGetOpenInterruptForRun_NoInterrupt_NotFoundIsNotError 驗證一個從未
// park 過的 run（沒有任何 interrupt row）回 ok=false 而不是 error——GetSnapshot
// handler 只在 ok=true 時才組 openConfirmation，這裡是那個判準的另一半前提。
func TestGetOpenInterruptForRun_NoInterrupt_NotFoundIsNotError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-getopen-none-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-getopen-none-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	got, ok, err := repo.GetOpenInterruptForRun(ctx, run.ID, buyerID)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, Interrupt{}, got)
}

// TestGetOpenInterruptForRun_ResolvedInterrupt_NotFound 驗證一個已經被
// resume 解決的 interrupt（resolved_by_child_run_id 非 NULL）不再算「open」——
// 買家已經回答過，不該再看到 openConfirmation 卡在畫面上。
func TestGetOpenInterruptForRun_ResolvedInterrupt_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, parent, interrupt := openInterruptFixture(ctx, t, repo, buyerID, "req-getopen-resolved-1")

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-getopen-resolved-resume-1", PayloadHash: "hash-getopen-resolved-1",
		ResponsePayload: []byte(`{}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out.IsNew)

	_, ok, err := repo.GetOpenInterruptForRun(ctx, parent.ID, buyerID)
	require.NoError(t, err)
	require.False(t, ok, "interrupt 已被 resume 解決，不得再被視為 open")
}

// TestResumeRun_ChildInheritsParentConversationID 是 Task 15 ruling 明確要求
// 用測試而非假設確認的一點：resumeReadyParentFixture 現在會用真實 RequestID
// claim（見該函式文件），所以 parent.ConversationID 是非零值；child run 建立
// 時直接沿用 parentRow.ConversationID（run.go ResumeRun 步驟 9，本身沒有另外
// 改動）——這裡證明那個既有寫法在 parent 的 conversation_id 真的被正確解析出
// 來之後，child 也會自動帶對同一個值，不需要 Task 15 額外改 ResumeRun 本身。
func TestResumeRun_ChildInheritsParentConversationID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-child-conv-1")
	require.NotZero(t, parent.ConversationID, "前置條件：parent 必須真的解出了 conversation_id")

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-child-conv-resume-1", PayloadHash: "hash-child-conv-1",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out.IsNew)
	require.Equal(t, parent.ConversationID, out.Child.ConversationID, "child run 必須繼承 parent 的 conversation_id")

	var persistedConvID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT conversation_id FROM procurement_agent_runs WHERE id = $1`, out.Child.ID,
	).Scan(&persistedConvID))
	require.Equal(t, parent.ConversationID, persistedConvID)
}

// TestResumeRun_ChildInheritsParentControllerKind proves the invariant
// ClaimAndDispatchResume (Sub-project 3b, Task 8) depends on: a child run's
// controller_kind always equals its parent's, so a chain of resumes (a
// graph-parked run resumed, itself pausing on a second interrupt, resumed
// again) keeps routing to the graph at every step — not just the first one.
func TestResumeRun_ChildInheritsParentControllerKind(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-controllerkind-inherit-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-controllerkind-inherit-1", RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindGraph,
	})
	require.NoError(t, err)
	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version, Status: domain.RunStatusWaiting(),
		ResultRevisionID: rev.ID, TerminalEventType: "CONFIRMATION_REQUIRED", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)
	interrupt, err := repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
	})
	require.NoError(t, err)

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "cr-controllerkind-inherit-resume-1", PayloadHash: "hash-inherit-resume-1",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out.IsNew)
	require.Equal(t, domain.ControllerKindGraph, out.Child.ControllerKind,
		"child of a GRAPH-claimed parent must itself be marked GRAPH")

	var persisted string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT controller_kind FROM procurement_agent_runs WHERE id = $1`, out.Child.ID,
	).Scan(&persisted))
	require.Equal(t, "GRAPH", persisted)
}

// TestClaimRun_WritesControllerKind proves ClaimRun persists whatever
// ControllerKind the caller passes — the field ClaimAndDispatchResume
// (Task 8) will later read off the parent run to decide graph-vs-durable
// resume routing (spec §4.1).
func TestClaimRun_WritesControllerKind(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	run, isNew, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-controller-kind-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-controller-kind-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindGraph,
	})
	require.NoError(t, err)
	require.True(t, isNew)
	require.Equal(t, domain.ControllerKindGraph, run.ControllerKind)

	var persisted string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT controller_kind FROM procurement_agent_runs WHERE id = $1`, run.ID,
	).Scan(&persisted))
	require.Equal(t, "GRAPH", persisted)
}

// TestCreateLedgerRunForCandidateSearch_WritesControllerKindDurable proves
// the candidate-search ledger path (unrelated to Turn/Answer/Rewind/Resume
// dispatch) writes a well-defined, harmless ControllerKind — CANDIDATE_SEARCH
// runs are never read by the new ClaimAndDispatchResume branch (Task 8),
// but the column is NOT NULL so every INSERT must supply an explicit value.
func TestCreateLedgerRunForCandidateSearch_WritesControllerKindDurable(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	set := CandidateSet{ID: repo.newID(), BuyerID: buyerID, RequestID: req.ID}
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, set))

	var persisted string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT controller_kind FROM procurement_agent_runs WHERE id = $1`, set.PublicRunID(),
	).Scan(&persisted))
	require.Equal(t, "DURABLE", persisted)
}

// TestLedgerEvents_CarryTraceIDFromContext 守住「一列 ledger 事件必須能接回
// log／trace」這個不變式。
//
// ProcurementRunFailureReason 的 CodeMeta 定義刻意讓 failure_reason 維持粗粒度
// （TURN_PIPELINE_ERROR 一個值涵蓋整條 pipeline 的所有失敗），並明文把細節指向
// log／trace。procurement_agent_run_events.trace_id 是那句話唯一的落地方式。
//
// 2026-09-03 稽核發現這條路整條是斷的：dev 上 7 天 1076 筆事件的 trace_id 全部
// 是空字串——7 個寫入點裡 6 個連參數都沒帶，剩下那個的 AppendRunEventInput.
// TraceID 零個呼叫端填過。一筆 FAILED 的 run 因此完全無法回溯根因。
//
// 三條寫入路徑（ClaimRun 的 RUN_ACCEPTED、AppendRunEvent、TerminalizeRun 的
// 終態 event）一起驗，因為漏掉任何一條都會在 replay 時留下斷點。
func TestLedgerEvents_CarryTraceIDFromContext(t *testing.T) {
	base := context.Background()
	pool, cleanup := setupProcurementTestDB(base, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	ctx := trace.ContextWithSpanContext(base, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	buyerID := identity.BuyerID(1)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "trace-req-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	_, err = repo.AppendRunEvent(ctx, AppendRunEventInput{
		RunID: run.ID, BuyerID: buyerID, EventType: "RUN_STARTED",
		SchemaVersion: "v1", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	wrote, err := repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: buyerID, Version: run.Version,
		Status: domain.RunStatusFailed(), TerminalEventType: "RUN_FAILED",
		FailureReason: "TURN_PIPELINE_ERROR", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, wrote)

	rows, err := pool.Query(ctx,
		`SELECT seq, event_type, trace_id FROM procurement_agent_run_events
		 WHERE run_id = $1 ORDER BY seq`, run.ID)
	require.NoError(t, err)
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var seq int64
		var eventType, gotTrace string
		require.NoError(t, rows.Scan(&seq, &eventType, &gotTrace))
		require.Equal(t, traceID.String(), gotTrace,
			"seq=%d (%s) 的 trace_id 必須是 ctx 上的 trace id，否則這一列無法接回 log／trace",
			seq, eventType)
		seen++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 3, seen, "RUN_ACCEPTED／RUN_STARTED／RUN_FAILED 三筆都要被檢查到")
}

// TestLedgerEvents_NoSpan_LeavesTraceIDEmpty 確認沒有 active span 時寫入空字串
// 而不是垃圾值——trace_id 是 NOT NULL DEFAULT ”，空字串是「這一列沒有可用的
// trace」的正確表示，不能塞一個查不到的假 id。
func TestLedgerEvents_NoSpan_LeavesTraceIDEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: identity.BuyerID(1), ClientRequestID: "trace-req-2", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindDurable,
	})
	require.NoError(t, err)

	var gotTrace string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT trace_id FROM procurement_agent_run_events WHERE run_id = $1 AND seq = 1`, run.ID,
	).Scan(&gotTrace))
	require.Empty(t, gotTrace)
}

func TestRestoreInterruptAfterFailedResume_MakesTheCardAnswerableAgain(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, revID, requestID := resumeReadyParentFixtureWithRequest(ctx, t, repo, buyerID, "req-restore")

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-that-fails", PayloadHash: "hash-a",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)
	require.True(t, out.IsNew)

	_, err = repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.ErrorIs(t, err, ErrNotFound, "T0 之後這張卡對 open-interrupt guard 是不存在的——這正是 /answers 會放行並回 422 的原因")

	restored, err := repo.RestoreInterruptAfterFailedResume(ctx, RestoreInterruptAfterFailedResumeInput{
		InterruptID: interrupt.ID, BuyerID: buyerID, ChildRunID: out.Child.ID,
	})
	require.NoError(t, err)
	require.True(t, restored)

	p, err := repo.GetRun(ctx, parent.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, domain.RunStatusWaiting(), p.Status)
	require.Equal(t, revID, p.ResultRevisionID, "result_revision_id 是下一次 resume 的樂觀鎖錨點，不得被補償抹掉")

	got, err := repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.NoError(t, err)
	require.Equal(t, interrupt.ID, got.ID)

	out2, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-retry", PayloadHash: "hash-b",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err, "補償之後買家必須能再答一次，這是整個修正唯一要證明的事")
	require.True(t, out2.IsNew)
	require.NotEqual(t, out.Child.ID, out2.Child.ID)
}

func TestRestoreInterruptAfterFailedResume_ForeignChildRunID_IsNoOp(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, _, _ := resumeReadyParentFixtureWithRequest(ctx, t, repo, buyerID, "req-restore-foreign")

	out, err := repo.ResumeRun(ctx, ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-foreign", PayloadHash: "hash-a",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	})
	require.NoError(t, err)

	restored, err := repo.RestoreInterruptAfterFailedResume(ctx, RestoreInterruptAfterFailedResumeInput{
		InterruptID: interrupt.ID, BuyerID: buyerID, ChildRunID: out.Child.ID + 777,
	})
	require.NoError(t, err)
	require.False(t, restored)

	p, err := repo.GetRun(ctx, parent.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, domain.RunStatusResumed(), p.Status)
}

// TestResumeCommand_ReplayAfterRestore：resume 失敗後走補償
// （RestoreInterruptAfterFailedResume 把 resolved_by_child_run_id 清成 NULL），
// 買家的 client 用**同一把鍵、同一份內容**重送。doc 57 §5.2／§11 的契約是
// 「同鍵同內容是成功的 replay 路徑」，只有同鍵不同 payload 才是
// IDEMPOTENCY_KEY_CONFLICT。回放判定因此不能建立在那個會被補償清掉的可變指標上。
func TestResumeCommand_ReplayAfterRestore(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent, interrupt, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-replay-restore")

	in := ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-restored", PayloadHash: "same-body",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	}
	first, err := repo.ResumeRun(ctx, in)
	require.NoError(t, err)
	require.True(t, first.IsNew)

	_, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: first.Child.ID, BuyerID: buyerID, Status: domain.RunStatusFailed(),
		FailureReason: "TURN_PIPELINE_ERROR", TerminalEventType: "RUN_FAILED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	restored, err := repo.RestoreInterruptAfterFailedResume(ctx, RestoreInterruptAfterFailedResumeInput{
		InterruptID: interrupt.ID, BuyerID: buyerID, ChildRunID: first.Child.ID,
	})
	require.NoError(t, err)
	require.True(t, restored)

	replay, err := repo.ResumeRun(ctx, in)
	require.NoError(t, err, "同鍵同內容必須回放，不是 IDEMPOTENCY_KEY_CONFLICT")
	require.False(t, replay.IsNew)
	require.Equal(t, first.Child.ID, replay.Child.ID, "回放要回原本那個 child，不是新建一個")

	var childCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE parent_run_id = $1`,
		parent.ID).Scan(&childCount))
	require.Equal(t, 1, childCount, "回放不得建立第二個 child")
}

// TestResumeCommand_ReplayAfterRestore_DifferentBodyStillConflicts 是上一條的
// 負控制：補償之後換了內容還沿用同一把鍵，仍然必須是 conflict。
func TestResumeCommand_ReplayAfterRestore_DifferentBodyStillConflicts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, interrupt, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-replay-conflict")

	in := ResumeRunInput{
		InterruptID: interrupt.ID, BuyerID: buyerID,
		ClientRequestID: "resume-restored", PayloadHash: "body-a",
		ResponsePayload: []byte(`{"optionId":"crop-a"}`), ResolveInput: fixedResolveInput(),
	}
	first, err := repo.ResumeRun(ctx, in)
	require.NoError(t, err)

	_, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: first.Child.ID, BuyerID: buyerID, Status: domain.RunStatusFailed(),
		FailureReason: "TURN_PIPELINE_ERROR", TerminalEventType: "RUN_FAILED",
		PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = repo.RestoreInterruptAfterFailedResume(ctx, RestoreInterruptAfterFailedResumeInput{
		InterruptID: interrupt.ID, BuyerID: buyerID, ChildRunID: first.Child.ID,
	})
	require.NoError(t, err)

	changed := in
	changed.PayloadHash = "body-b"
	changed.ResponsePayload = []byte(`{"optionId":"crop-b"}`)
	_, err = repo.ResumeRun(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

// TestResumeCommand_ConcurrentDifferentInterrupts：兩個**不同** interrupt 的
// resume 同時進來，帶著同一個 (buyer, clientRequestId)。步驟 1 的 FOR UPDATE 只
// 序列化同一個 interrupt，兩者不會互相擋，所以只剩步驟 16 的唯一索引能分勝負——
// 而那個 INSERT 是 ON CONFLICT DO UPDATE，撞鍵不拋錯。呼叫端不讀 RETURNING 的
// id，敗方就不知道自己輸了，於是把自己的 child run／事件／revision 一起 commit。
// 一把命令鍵只能留下一個 committed child。
func TestResumeCommand_ConcurrentDifferentInterrupts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	parent1, interrupt1, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-conc-1")
	parent2, interrupt2, _ := resumeReadyParentFixture(ctx, t, repo, buyerID, "req-conc-2")

	// barrier：兩邊都進到交易內的 ResolveInput 才放行，確保步驟 2 的冪等查詢都已
	// 經跑過（彼此都還沒 commit，所以都查不到對方）。
	// ⛔ 有 timeout 且不作為 green 前提：修好之後正確的早期認領本來就可能讓其中
	// 一邊根本走不到這裡。
	var arrived sync.WaitGroup
	arrived.Add(2)
	barrier := func() func(context.Context, ResumeResolveContext) (ResolvedRunInput, error) {
		inner := fixedResolveInput()
		return func(c context.Context, rc ResumeResolveContext) (ResolvedRunInput, error) {
			arrived.Done()
			done := make(chan struct{})
			go func() { arrived.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
			return inner(c, rc)
		}
	}

	type outcome struct {
		out ResumeRunOutput
		err error
	}
	results := make(chan outcome, 2)
	for _, interruptID := range []string{interrupt1.ID, interrupt2.ID} {
		go func(id string) {
			out, err := repo.ResumeRun(ctx, ResumeRunInput{
				InterruptID: id, BuyerID: buyerID,
				ClientRequestID: "cr-concurrent", PayloadHash: "hash-shared",
				ResponsePayload: []byte(`{"optionId":"crop-a"}`),
				ResolveInput:    barrier(),
			})
			results <- outcome{out, err}
		}(interruptID)
	}
	first, second := <-results, <-results

	successes := 0
	for _, r := range []outcome{first, second} {
		if r.err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "一把命令鍵只能有一個 transaction 成功")

	var childCount, commandCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs WHERE parent_run_id = ANY($1)`,
		[]int64{parent1.ID, parent2.ID}).Scan(&childCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(buyerID), "cr-concurrent").Scan(&commandCount))
	require.Equal(t, 1, commandCount, "唯一索引本來就只會留一列")
	require.Equal(t, 1, childCount, "敗方的 child run 必須隨 rollback 退掉")

	// 敗方那個 interrupt 必須完全沒被動到。
	var pending int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_interrupts
		 WHERE id = ANY($1) AND status = 'PENDING' AND resolved_by_child_run_id IS NULL`,
		[]string{interrupt1.ID, interrupt2.ID}).Scan(&pending))
	require.Equal(t, 1, pending, "敗方的 interrupt 必須仍是 PENDING、零 mutation")
}
