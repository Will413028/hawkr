//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 本檔守的是 Task 10（doc 57 §563）：procurement_candidates 的兩條 partial UNIQUE
// （uq_procurement_candidates_set_store／uq_procurement_candidates_set_prospect）
// 是應用層去重失效時的防線，不是去重機制本身（PLATFORM 依 store 選代表、EXTERNAL
// 靠上游一列即一業者仍是機制）。這裡驗的是防線本身真的擋得住，以及它的範圍是
// 「同一批候選內」而非全表。
//
// isUniqueViolation 沿用 revision.go 既有的偵測函式（同一個 package），不新增第二份。

// platformCandidate／externalCandidate 是兩個最小候選；只填約束會看到的欄位。
// FactsAsOf 刻意帶上非零值——facts_as_of 是 NOT NULL，零值 time.Time 經
// timestamptzParam 轉換後 Valid 會是 false（即 SQL NULL），INSERT 會在還沒碰到
// UNIQUE 之前就先被 23502（not_null_violation）擋下，這兩個 helper 若省略它，
// 下面三個測試驗的就不是 UNIQUE 約束，而是一個完全不同的錯誤。
func platformCandidate(storeID identity.StoreID) Candidate {
	return Candidate{
		Lane: "PLATFORM", SourceID: "p1", DisplayName: "商品 A",
		MatchedFactsJSON: []byte("[]"), UnverifiableFactsJSON: []byte("[]"),
		FactsAsOf: refTime(), StoreID: storeID,
	}
}

func externalCandidate(prospectID int64) Candidate {
	return Candidate{
		Lane: "EXTERNAL", SourceID: "e1", DisplayName: "業者 A",
		MatchedFactsJSON: []byte("[]"), UnverifiableFactsJSON: []byte("[]"),
		FactsAsOf: refTime(), ProspectID: &prospectID,
	}
}

func TestCandidateUniqueness_SameStoreTwiceInOneSetIsRejected(t *testing.T) {
	// PLATFORM 一店一筆。應用層已在 lane 內去重（#2880 的 seen[StoreID]），這條驗證的
	// 是防線本身有效——機制失效時 DB 擋得下來。
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, set := setupRunIdentityFixture(ctx, t, repo, identity.BuyerID(9001))
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	c := platformCandidate(identity.StoreID(555))
	_, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: set.RequestID,
		RequestRevisionID: set.RequestRevisionID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{c, c}},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
	})
	constraintName, ok := isUniqueViolation(err)
	require.True(t, ok,
		"同一批候選內同一個 store 出現兩次必須被 uq_procurement_candidates_set_store 擋下，got %v", err)
	require.Equal(t, "uq_procurement_candidates_set_store", constraintName)
}

func TestCandidateUniqueness_SameProspectTwiceInOneSetIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, set := setupRunIdentityFixture(ctx, t, repo, identity.BuyerID(9002))
	markRunning(t, ctx, repo, set.ID, set.FencingToken)

	c := externalCandidate(7001)
	_, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
		CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: set.RequestID,
		RequestRevisionID: set.RequestRevisionID,
		Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
		External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{c, c}},
	})
	constraintName, ok := isUniqueViolation(err)
	require.True(t, ok,
		"同一批候選內同一個業者出現兩次必須被 uq_procurement_candidates_set_prospect 擋下，got %v", err)
	require.Equal(t, "uq_procurement_candidates_set_prospect", constraintName)
}

func TestCandidateUniqueness_DifferentSetsMayRepeatSameStore(t *testing.T) {
	// 約束範圍是「同一批候選內」，不是全表——不同 run 當然可以再出現同一家，
	// 否則一個買家搜過的店，別的買家就再也搜不到了。
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	c := platformCandidate(identity.StoreID(556))

	for _, buyerID := range []identity.BuyerID{9003, 9004} {
		_, set := setupRunIdentityFixture(ctx, t, repo, buyerID)
		markRunning(t, ctx, repo, set.ID, set.FencingToken)
		wrote, _, _, err := repo.FinalizeCandidateSet(ctx, FinalizeCandidateSetInput{
			CandidateSetID: set.ID, FencingToken: set.FencingToken, RequestID: set.RequestID,
			RequestRevisionID: set.RequestRevisionID,
			Platform:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value, Candidates: []Candidate{c}},
			External:          LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
		})
		require.NoError(t, err)
		require.True(t, wrote)
	}
}
