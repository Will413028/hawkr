//go:build integration

package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestStartSearch_Integration_CompletesAsynchronously(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := repository.New(db, database.NewTxManager(db), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	// facts_as_of／matched_facts_json／unverifiable_facts_json 在 schema 上都是 NOT NULL
	// 無 DEFAULT（見 db/schema/procurement.sql 的 procurement_candidates）；真正的 Task 6/7
	// lane searcher 一律會填這三欄（candidate_platform_lane.go／candidate_external_lane.go
	// 的 Search 都設 FactsAsOf/MatchedFactsJSON/UnverifiableFactsJSON），這裡的 stub candidate
	// 補齊同樣的欄位，才不會讓 FinalizeCandidateSet 內的 INSERT 因 NOT NULL 違反而整筆
	// transaction rollback——run() 對 FinalizeCandidateSet 的錯誤是 fire-and-forget 地吞掉
	// （見 candidate_search.go run() 的設計），少了這些欄位時失敗會是完全無聲的，
	// 只會讓 candidate_set 卡在 RUNNING 直到 lease 過期，不會在測試以外的路徑上報錯。
	now := time.Now().UTC()
	emptyFacts := json.RawMessage(`[]`)
	uc := NewCandidateSearchUseCase(repo,
		stubPlatformSearcher{candidates: []repository.Candidate{{
			Lane: "PLATFORM", SourceID: "1", DisplayName: "candidate A",
			MatchedFactsJSON: emptyFacts, UnverifiableFactsJSON: emptyFacts, FactsAsOf: now,
		}}},
		stubExternalSearcher{candidates: []repository.Candidate{{
			Lane: "EXTERNAL", SourceID: "2", DisplayName: "candidate B",
			MatchedFactsJSON: emptyFacts, UnverifiableFactsJSON: emptyFacts, FactsAsOf: now,
		}}},
		// Activity 步驟列的 row id 產生器。這支測試不斷言 rail 內容，但 run()
		// 會真的把步驟列寫進 procurement_agent_run_events，row id 必須逐列
		// 相異，否則消費端的 merge-by-rowId 會把四列疊成一列。沿用本套件
		// 既有的遞增替身（同 repository.New 收到的那一個）。
		testIDGen(),
	)

	res, err := uc.StartSearch(ctx, StartSearchInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	require.NoError(t, err)
	require.NotEqual(t, res.CandidateSetID, res.RunID,
		"StartSearch 回的是 run 身分不是 candidate set 身分（設計書 57 §1.2-3）")

	deadline := time.Now().Add(3 * time.Second)
	var set repository.CandidateSet
	for time.Now().Before(deadline) {
		set, _, err = repo.GetCandidateSetRunWithCandidates(ctx, res.RunID, req.ID, buyerID)
		require.NoError(t, err)
		if set.Status == codemeta.ProcurementRunStatusSucceeded.Value || set.Status == codemeta.ProcurementRunStatusFailed.Value {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if set.Status != codemeta.ProcurementRunStatusSucceeded.Value {
		t.Fatalf("Status = %q, want COMPLETED within 3s", set.Status)
	}

	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	if gotReq.Status != "CANDIDATES_READY" {
		t.Errorf("request status = %q, want CANDIDATES_READY", gotReq.Status)
	}
}

// TestStartSearch_Integration_RetriesDeadRun exercises RetryProcurementCandidateSet's actual
// SQL against a real Postgres — the fake-based unit test (TestStartSearch_RetriesDeadRunWhenIdempotencyKeyHitsFailed)
// only proves the usecase calls RetryCandidateSet with the right arguments; it can't catch a
// bug in the hand-written UPDATE...RETURNING itself. This drives a run to a real FAILED state,
// then calls StartSearch again on the same (request_revision_id, policy_version) idempotency
// key and confirms it gets a *working* run back on the *same* candidate_set.id, not a second
// candidate_set row and not the stale FAILED snapshot forever (Design Decision #7).
//
// 身分模型（B9）：同一列 candidate_set，但**每次嘗試各配一個新的 run 身分**。尾段因此
// 一併斷言兩次嘗試在 ledger 上各自留下自己的終態事件——那是沿用同一個 run_id 時寫不出來
// 的東西，也是這次改動真正要買的結果。
func TestStartSearch_Integration_RetriesDeadRun(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := repository.New(db, database.NewTxManager(db), testIDGen())

	buyerID := identity.BuyerID(1)
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	// 第一次：兩條 lane 都失敗，讓 run 真的走到 FAILED 終態（非 reap，直接由
	// FinalizeCandidateSet 判定 status='FAILED'，request 退回 REQUIREMENT_READY）。
	failingUC := NewCandidateSearchUseCase(repo,
		stubPlatformSearcher{err: context.DeadlineExceeded},
		stubExternalSearcher{err: context.DeadlineExceeded},
		testIDGen(),
	)
	firstRun, err := failingUC.StartSearch(ctx, StartSearchInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	var failedSet repository.CandidateSet
	for time.Now().Before(deadline) {
		failedSet, _, err = repo.GetCandidateSetRunWithCandidates(ctx, firstRun.RunID, req.ID, buyerID)
		require.NoError(t, err)
		if failedSet.Status == "FAILED" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if failedSet.Status != "FAILED" {
		t.Fatalf("first run Status = %q, want FAILED within 3s (test setup precondition not met)", failedSet.Status)
	}
	gotReq, _, err := repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	if gotReq.Status != "REQUIREMENT_READY" {
		t.Fatalf("request status after FAILED run = %q, want REQUIREMENT_READY (test setup precondition not met)", gotReq.Status)
	}

	// 第二次：同一個 (request_revision_id, policy_version) 冪等鍵，這次 lane searcher
	// 會成功——驗證 StartSearch 真的呼叫了 RetryProcurementCandidateSet 把同一列從
	// FAILED 救回 QUEUED，而不是永遠回傳第一次那筆死掉的紀錄。
	now := time.Now().UTC()
	emptyFacts := json.RawMessage(`[]`)
	workingUC := NewCandidateSearchUseCase(repo,
		stubPlatformSearcher{candidates: []repository.Candidate{{
			Lane: "PLATFORM", SourceID: "1", DisplayName: "candidate A",
			MatchedFactsJSON: emptyFacts, UnverifiableFactsJSON: emptyFacts, FactsAsOf: now,
		}}},
		stubExternalSearcher{candidates: []repository.Candidate{{
			Lane: "EXTERNAL", SourceID: "2", DisplayName: "candidate B",
			MatchedFactsJSON: emptyFacts, UnverifiableFactsJSON: emptyFacts, FactsAsOf: now,
		}}},
		testIDGen(),
	)
	secondRun, err := workingUC.StartSearch(ctx, StartSearchInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		RequirementReady: true, Slots: procurementdomain.Slots{},
	})
	require.NoError(t, err)
	if secondRun.CandidateSetID != firstRun.CandidateSetID {
		t.Fatalf("second StartSearch CandidateSetID = %d, want the same candidate_set.id as the first run (%d) — retry must reuse the row, not create a second one", secondRun.CandidateSetID, firstRun.CandidateSetID)
	}
	if secondRun.RunID == firstRun.RunID {
		t.Fatalf("second StartSearch RunID = %d, want a *new* run id (first attempt was %d) — one run = one immutable execution attempt", secondRun.RunID, firstRun.RunID)
	}
	if secondRun.Status != "QUEUED" {
		t.Errorf("second StartSearch Status = %q, want QUEUED (retried, re-dispatched), not the stale FAILED snapshot", secondRun.Status)
	}

	deadline = time.Now().Add(3 * time.Second)
	var retriedSet repository.CandidateSet
	for time.Now().Before(deadline) {
		retriedSet, _, err = repo.GetCandidateSetRunWithCandidates(ctx, secondRun.RunID, req.ID, buyerID)
		require.NoError(t, err)
		if retriedSet.Status == codemeta.ProcurementRunStatusSucceeded.Value || retriedSet.Status == codemeta.ProcurementRunStatusFailed.Value {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if retriedSet.Status != codemeta.ProcurementRunStatusSucceeded.Value {
		t.Fatalf("retried run Status = %q, want COMPLETED within 3s", retriedSet.Status)
	}
	if retriedSet.FencingToken != failedSet.FencingToken+1 {
		t.Errorf("retried run FencingToken = %d, want %d (bumped by exactly one RetryCandidateSet call)", retriedSet.FencingToken, failedSet.FencingToken+1)
	}

	gotReq, _, err = repo.GetSnapshot(ctx, req.ID, buyerID)
	require.NoError(t, err)
	if gotReq.Status != "CANDIDATES_READY" {
		t.Errorf("request status after retried run = %q, want CANDIDATES_READY", gotReq.Status)
	}

	// 這一段是 B9 真正要買的東西：**兩次嘗試各自都有終態 ledger event**。
	//
	// 舊模型（兩次嘗試共用一個 run_id）下這裡必紅：那一列在第一次嘗試就被寫成
	// FAILED，第二次嘗試的 RUN_FINISHED 會被 TerminalizeProcurementAgentRunStatus 的
	// 「現在還不是終態」守衛整段丟掉，只留一行 Warn——買家的第二次搜尋其實成功了，
	// 串流端卻永遠等不到收線訊號，連線開到 gateway 的 deadline 為止。
	assertRunTerminalEvent(ctx, t, db, firstRun.RunID, codemeta.ProcurementLedgerEventTypeRunFailed.Value)
	assertRunTerminalEvent(ctx, t, db, secondRun.RunID, codemeta.ProcurementLedgerEventTypeRunFinished.Value)

	// 兩列 ledger run 都用 domain_ref 指回同一批候選——1:N 的多對一那一邊。
	var runsForSet int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_runs
		  WHERE domain_ref_type = 'candidateSetId' AND domain_ref_id = $1`,
		strconv.FormatInt(secondRun.CandidateSetID, 10),
	).Scan(&runsForSet))
	if runsForSet != 2 {
		t.Errorf("agent_runs rows for candidate set %d = %d, want 2 (one per attempt)", secondRun.CandidateSetID, runsForSet)
	}
}

// assertRunTerminalEvent 等到某個 run 的 ledger 最後一筆是指定的終態事件，且 run row
// 本身也已經是終態。兩者要一起看：doc57 §5.1 的 terminal transaction 保證它們同時寫入，
// 只驗其中一個會漏掉「狀態寫了但事件被丟掉」這種正是本波要修的形狀。
//
// **必須輪詢，不能讀一次就斷言**：ledger 終態投影排在 FinalizeCandidateSet **之後**
// （見 candidate_search.go 的 finalize），所以 candidate_set 已經是 SUCCEEDED 的那一刻，
// 這個 run 在 ledger 上通常還停在最後一筆 AGENT_ACTIVITY_SNAPSHOT。直接斷言會得到一支
// 只有在機器夠閒時才綠的測試——實測整包跑就紅。
func assertRunTerminalEvent(ctx context.Context, t *testing.T, db *pgxpool.Pool, runID int64, wantEventType string) {
	t.Helper()

	var lastEventType, status string
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := db.QueryRow(ctx,
			`SELECT event_type FROM procurement_agent_run_events
			  WHERE run_id = $1 ORDER BY seq DESC LIMIT 1`, runID,
		).Scan(&lastEventType)
		require.NoError(t, err, "run %d 在 ledger 上必須有事件", runID)
		require.NoError(t, db.QueryRow(ctx,
			`SELECT status FROM procurement_agent_runs WHERE id = $1`, runID).Scan(&status))

		if lastEventType == wantEventType && procurementdomain.IsTerminalRunStatus(status) {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if lastEventType != wantEventType {
		t.Errorf("run %d last ledger event = %q, want %q within 3s", runID, lastEventType, wantEventType)
	}
	if !procurementdomain.IsTerminalRunStatus(status) {
		t.Errorf("run %d status = %q, want a terminal status within 3s", runID, status)
	}
}
