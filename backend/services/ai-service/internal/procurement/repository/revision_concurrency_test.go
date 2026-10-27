//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/stretchr/testify/require"
)

// TestRepo_AppendRevision_ConcurrentSameClientRequestID_IsIdempotent is Fix
// round 1 item 1 (HIGH): two goroutines racing AppendRevision with the SAME
// client_request_id must both come back with nil error and the SAME revision
// ID — not one success and one raw "duplicate key value violates unique
// constraint" driver error. The application-layer check-then-act (SELECT
// answer_patches before INSERT) is not enough by itself under real
// concurrency; this test is what the fix (catching SQLSTATE 23505 on the
// answer_patch INSERT and recovering the winner's result outside the failed
// transaction) has to satisfy.
func TestRepo_AppendRevision_ConcurrentSameClientRequestID_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("race")}, refTime())

	in := AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "race-key", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING", // 只填 Item，必填欄位還缺
	}

	const n = 2
	var wg sync.WaitGroup
	errs := make([]error, n)
	revs := make([]Revision, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			revs[idx], errs[idx] = repo.AppendRevision(ctx, in)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "goroutine %d must not see a raw DB error under the idempotency race", i)
	}
	require.Equal(t, revs[0].ID, revs[1].ID, "同 client_request_id 併發呼叫必須回傳同一個 revision")
	require.NotZero(t, revs[0].ID)
	// 輸的那個 goroutine 走 recoverIdempotentAppend（三條回傳路徑之一，交易外復原），
	// 贏的那個走正常寫入路徑——兩者最後都要能算出同一個 ConversationID，不能只有
	// 贏家那條路徑有值。
	require.NotZero(t, revs[0].ConversationID)
	require.Equal(t, revs[0].ConversationID, revs[1].ConversationID,
		"recoverIdempotentAppend 復原出來的 ConversationID 必須跟正常寫入路徑一致")

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_answer_patches WHERE client_request_id = $1`, "race-key",
	).Scan(&count))
	require.Equal(t, int64(1), count, "無論哪個先寫贏，DB 只能留下一筆 answer_patch")
}

// TestRepo_AppendRevision_ConcurrentDifferentClientRequestID_Serializes is Fix
// round 1 item 2 (MED): two DIFFERENT client_request_id calls on the SAME
// request must not read the same parent revision and race to the same
// revision_no (which would trip uq_procurement_request_revisions_no and hand
// the loser a raw DB error). The fix takes a row lock (SELECT ... FOR UPDATE)
// on the request row so the two calls serialize instead of racing.
func TestRepo_AppendRevision_ConcurrentDifferentClientRequestID_Serializes(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	slotsA, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("甲需求")}, refTime())
	slotsB, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("乙需求")}, refTime())

	inputs := []AppendInput{
		// 兩個都只填 Item，必填欄位還缺，狀態應維持 CLARIFYING。
		{RequestID: req.ID, BuyerID: testBuyerID, Slots: slotsA,
			Source: "LLM_EXTRACT", ClientRequestID: "concurrent-a", PayloadJSON: []byte(`{}`),
			RequestStatus: "CLARIFYING"},
		{RequestID: req.ID, BuyerID: testBuyerID, Slots: slotsB,
			Source: "LLM_EXTRACT", ClientRequestID: "concurrent-b", PayloadJSON: []byte(`{}`),
			RequestStatus: "CLARIFYING"},
	}

	var wg sync.WaitGroup
	errs := make([]error, len(inputs))
	revs := make([]Revision, len(inputs))
	for i, in := range inputs {
		wg.Add(1)
		go func(idx int, in AppendInput) {
			defer wg.Done()
			revs[idx], errs[idx] = repo.AppendRevision(ctx, in)
		}(i, in)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "goroutine %d must not fail on revision_no collision", i)
	}
	require.NotEqual(t, revs[0].ID, revs[1].ID, "兩個不同改動必須各自成一個獨立版本")

	gotRevNos := map[int]bool{revs[0].RevisionNo: true, revs[1].RevisionNo: true}
	require.True(t, gotRevNos[2] && gotRevNos[3],
		"序列化後兩個 append 必須分別拿到 revision_no 2 與 3，得到 %v", gotRevNos)

	var activeCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_request_revisions WHERE request_id = $1 AND status = 'ACTIVE'`, req.ID,
	).Scan(&activeCount))
	require.Equal(t, int64(1), activeCount, "最終只能有一個 ACTIVE 版本")

	gotReq, gotRev, err := repo.GetSnapshot(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, gotReq.ActiveRevisionID, gotRev.ID, "GetSnapshot 回傳的 pair 必須自洽")
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"併發序列化下狀態寫入依然正確，不是空字串或被其中一次覆寫成別的值")
}
