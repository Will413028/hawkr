//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// TestListBranchRevisions_WalksParentChainRootToTip 驗證 ListBranchRevisions
// 依 parent_revision_id 從根走到 tip，順序是根→尾（不是 DB 回傳的任意順序），
// 且每個非根節點都配對到它自己的 answer_patch。
func TestListBranchRevisions_WalksParentChainRootToTip(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	item := "水果禮盒"
	rev1, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-1",
		PayloadJSON: []byte(`{"message":"我要水果禮盒"}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	city := "桃園市"
	rev2, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "cr-2",
		PayloadJSON: []byte(`{"message":"桃園市"}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	chain, err := repo.ListBranchRevisions(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Len(t, chain, 3, "根 revision + 兩次 AppendRevision")

	require.Equal(t, int64(0), chain[0].ParentRevisionID, "第一筆是根版本")
	require.Empty(t, chain[0].PatchPayloadJSON, "根版本沒有 answer_patch")
	require.Equal(t, rev1.ID, chain[1].ID)
	require.Equal(t, "LLM_EXTRACT", chain[1].PatchSource)
	require.Equal(t, rev2.ID, chain[2].ID)
	require.Equal(t, "ANSWER", chain[2].PatchSource)
}

// TestListBranchRevisions_RootOnly_ReturnsSingleRootRevision 驗證只呼叫過
// CreateRequest、還沒有任何 AppendRevision 的 request（唯一版本就是根版本本身）
// 回傳長度 1 的鏈，且該筆沒有配對到任何 answer_patch。
func TestListBranchRevisions_RootOnly_ReturnsSingleRootRevision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, rootRev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	chain, err := repo.ListBranchRevisions(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Len(t, chain, 1, "還沒 AppendRevision 過，鏈上只有根版本")
	require.Equal(t, rootRev.ID, chain[0].ID)
	require.Equal(t, int64(0), chain[0].ParentRevisionID)
	require.Empty(t, chain[0].PatchSource)
	require.Empty(t, chain[0].PatchPayloadJSON)
}

// TestListBranchRevisions_UnknownRequest_ReturnsErrNotFound 驗證找不到 request
// （或不屬於這個買家）時回 ErrNotFound——與 GetSnapshot 一致，不洩漏存在性。
func TestListBranchRevisions_UnknownRequest_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.ListBranchRevisions(ctx, 999999999, testBuyerID)
	require.ErrorIs(t, err, ErrNotFound)
}
