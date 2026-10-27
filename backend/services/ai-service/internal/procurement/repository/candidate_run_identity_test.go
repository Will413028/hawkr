//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 本檔守的是設計書 57 §1.2-3：「runId 不得同時代表 candidateSetId、requestId 或 Temporal
// Workflow ID」。#2446 凍結 golden fixtures 之前必須先分開，否則相等會被凍成消費端可以
// 依賴的契約。
//
// ⚠️ 這一波（expand）只做到「兩把身分在 DB 與查詢層真的分開、且兩種值都解析得到」。
// **對外 runId 仍然回 candidate set id**，翻轉在下一波（見 grpc/procurement_handler.go
// 的 wireRunID）。所以本檔驗的是 repository 層，不是 wire。

// setupRunIdentityFixture 建一個 REQUIREMENT_READY 的 request 與一個 candidate set。
func setupRunIdentityFixture(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID) (Request, CandidateSet) {
	t.Helper()
	req, rev, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	require.NoError(t, repo.SetRequestStatus(ctx, req.ID, "REQUIREMENT_READY"))

	set, err := repo.CreateCandidateSet(ctx, CreateCandidateSetInput{
		RequestID: req.ID, RequestRevisionID: rev.ID, BuyerID: buyerID,
		PolicyVersion: 1, LeaseOwner: "worker-1", LeaseTTL: 30 * time.Second,
	})
	require.NoError(t, err)
	return req, set
}

func TestCreateCandidateSet_MintsRunIDDistinctFromCandidateSetID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	require.NotZero(t, set.RunID, "run_id 必須被指派——零值代表 CreateCandidateSet 沒有配號")
	require.NotEqual(t, set.ID, set.RunID,
		"run 身分與 candidate set 身分必須是不同的值（設計書 57 §1.2-3）")

	// 回傳的 struct 對，不代表 DB 對。這個違規就是「兩個對外欄位讀同一個 DB 欄位」造成的，
	// 所以要直接驗 DB 那一列。
	var gotRunID *int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT run_id FROM procurement_candidate_sets WHERE id = $1`, set.ID,
	).Scan(&gotRunID))
	require.NotNil(t, gotRunID, "run_id 必須真的寫進 DB，不只是回傳的 struct")
	require.Equal(t, set.RunID, *gotRunID)
}

func TestGetCandidateSetRunWithCandidates_ResolvesByRunID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	got, _, err := repo.GetCandidateSetRunWithCandidates(ctx, set.RunID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, set.ID, got.ID)
	require.Equal(t, set.RunID, got.RunID)
}

// expand 階段的相容路徑：部署當下正在輪詢的 browser 手上拿的是舊的 candidate set id，
// 以及 migration 之前建立的列 run_id 為 NULL。兩者都必須仍然查得到，否則翻轉那一波
// 會讓 surge 視窗內的輪詢直接 404。
func TestGetCandidateSetRunWithCandidates_FallsBackToCandidateSetID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	got, _, err := repo.GetCandidateSetRunWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, set.ID, got.ID)
}

// migration 之前建立的列：run_id IS NULL。PublicRunID() 必須退回 candidate set id，
// 否則那些列對外的 runId 會變成空的 "0"。
func TestGetCandidateSetRunWithCandidates_LegacyNullRunIDRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	req, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	// 直接把 run_id 清成 NULL，模擬 20260822061013 之前建立的列。
	_, err := pool.Exec(ctx, `UPDATE procurement_candidate_sets SET run_id = NULL WHERE id = $1`, set.ID)
	require.NoError(t, err)

	got, _, err := repo.GetCandidateSetRunWithCandidates(ctx, set.ID, req.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, set.ID, got.ID)
	require.Zero(t, got.RunID, "NULL 應該對到零值")
	require.Equal(t, set.ID, got.PublicRunID(),
		"舊列沒有 run_id，對外 runId 必須退回 candidate set id，不能變成 0")
}

// 換一把查詢鍵不是放寬 ownership 的理由。
func TestGetCandidateSetRunWithCandidates_RejectsOtherBuyer(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	owner := identity.BuyerID(1)
	intruder := identity.BuyerID(2)
	req, set := setupRunIdentityFixture(ctx, t, repo, owner)

	_, _, err := repo.GetCandidateSetRunWithCandidates(ctx, set.RunID, req.ID, intruder)
	require.ErrorIs(t, err, ErrNotFound, "別的買家用 run 身分查也必須是 404，不是洩漏")

	// legacy fallback 也不能是後門。
	_, _, err = repo.GetCandidateSetRunWithCandidates(ctx, set.ID, req.ID, intruder)
	require.ErrorIs(t, err, ErrNotFound, "legacy 分支同樣要守 ownership")
}

// 重試沿用同一列 candidate_set，但**換一個 run 身分**：one run = one immutable
// execution attempt。
//
// 本測試取代舊的 TestRetryCandidateSet_KeepsRunIDStableInThisWave。那一版守的是「本波
// 先不輪替 run_id」，它列出的兩個理由都已經不成立：
//
//   - 「需要 run 歷史表才能讓被取代的 runId 仍然解析得到」——`procurement_agent_runs`
//     就是那張表（Slice F 已落地），解析走 GetProcurementCandidateSetBySupersededRunID，
//     由 TestGetCandidateSetRunWithCandidates_ResolvesSupersededAttempt 守著。
//   - 「StartSearch 併發重試的落敗方會把已被取代的 runId 交給前端」——落敗分支現在會
//     重讀一次再回覆（usecase/candidate_search.go），由該層的測試守著。
//
// 反過來，維持穩定 run_id 的代價已經確定：第二次嘗試的終態寫不進 ledger（終態守衛是
// 「現在還不是終態」，而那一列在第一次嘗試就終態化了），串流端永遠等不到收線訊號。
func TestRetryCandidateSet_MintsNewRunIDForTheNewAttempt(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(1)
	_, set := setupRunIdentityFixture(ctx, t, repo, buyerID)

	_, err := pool.Exec(ctx,
		`UPDATE procurement_candidate_sets SET status = 'FAILED', failure_reason = 'WORKER_LOST' WHERE id = $1`, set.ID)
	require.NoError(t, err)

	retried, err := repo.RetryCandidateSet(ctx, set.ID, "worker-2", 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, set.ID, retried.ID, "重試沿用同一列 candidate_set")
	require.NotEqual(t, set.RunID, retried.RunID, "但這是一次新的執行，必須配一個新的 run 身分")
	require.NotEqual(t, retried.ID, retried.RunID, "重試後兩把身分仍然不同值")
}
