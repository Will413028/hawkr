//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// TestRewindTo_ForksNewBranchFromTarget 驗證 RewindTo 從 target revision 開一個新
// 分支：parent_revision_id 指向 target、branch_id 是新版本自身 ID（比照 root revision
// 的既定慣例）、revision_no 接續「目前 ACTIVE tip」而不是 target 的 revision_no，且
// GetSlots 讀回的內容真的是 target 當下的 slots（delivery_city 這個 target 之後才
// 出現的欄位要連帶消失）。
func TestRewindTo_ForksNewBranchFromTarget(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, rootRev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	item := "水果禮盒"
	rev1, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-1",
		PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	city := "桃園市"
	rev2, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "cr-2",
		PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	// 回溯到 item 剛設定完、delivery_city 還沒填之前——target 應該是 rev1。
	rewound, err := repo.RewindTo(ctx, RewindInput{
		RequestID: req.ID, BuyerID: testBuyerID, TargetRevisionID: rev1.ID,
		ClientRequestID: "cr-3", PayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	require.NotEqual(t, rev2.ID, rewound.ID)
	require.Equal(t, rev1.ID, rewound.ParentRevisionID)
	require.NotEqual(t, rootRev.BranchID, rewound.BranchID, "rewind 必須開新分支")
	require.Equal(t, rewound.ID, rewound.BranchID, "新分支以自身 id 為 branch_id")
	require.Equal(t, rev2.RevisionNo+1, rewound.RevisionNo, "revision_no 接續原本 ACTIVE tip，不是 target")

	slots, err := repo.GetSlots(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Nil(t, slots.DeliveryCity, "delivery_city 應該連帶回退消失")
	require.Equal(t, item, *slots.Item)
}

// TestRewindTo_Idempotent 驗證同一個 client_request_id 重送 RewindTo 不會二度開分支
// ——跟 AppendRevision 同一套 uq_procurement_answer_patches_idempotency 冪等鍵。
func TestRewindTo_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, rootRev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	in := RewindInput{
		RequestID: req.ID, BuyerID: testBuyerID, TargetRevisionID: rootRev.ID,
		ClientRequestID: "cr-dup", PayloadJSON: []byte(`{}`),
	}
	first, err := repo.RewindTo(ctx, in)
	require.NoError(t, err)
	second, err := repo.RewindTo(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "同一個 clientRequestId 重送必須回同一個 revision")
}

// TestRewindTo_RejectsTargetFromDifferentRequest 釘住 RewindTo 與 AppendRevision
// 在 target/parent 解析上唯一的結構差異：AppendRevision 的 parent 是用
// GetActiveProcurementRevision(txCtx, in.RequestID) 取得，天生被 request_id 限定；
// RewindTo 改用裸 ID 查 target（GetProcurementRevisionByID 沒有 WHERE request_id），
// 若不額外驗證 targetRow.RequestID == in.RequestID，呼叫端傳入別的 request（甚至別的
// 買家）的 revision id 就會被接受，把別人的 slots 內容當成這個 request 的新版本寫入
// 並可被 GetSlots 讀出——跨租戶資料外洩。這裡驗證：request A 用 request B 的 revision
// id 當 target 必須被拒絕（ErrNotFound，不洩漏「這個 id 存在，只是不屬於你」），且
// request A 的狀態完全不受影響（沒有新版本、active revision 沒變）。
func TestRewindTo_RejectsTargetFromDifferentRequest(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	reqA, rootA, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	otherBuyerID := testBuyerID + 1
	reqB, _, err := repo.CreateRequest(ctx, otherBuyerID)
	require.NoError(t, err)
	secretItem := "別人的採購項目"
	revB1, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: reqB.ID, BuyerID: otherBuyerID, Slots: domain.Slots{Item: &secretItem},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-b-1",
		PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	_, err = repo.RewindTo(ctx, RewindInput{
		RequestID: reqA.ID, BuyerID: testBuyerID, TargetRevisionID: revB1.ID,
		ClientRequestID: "cr-cross-request", PayloadJSON: []byte(`{}`),
	})
	require.ErrorIs(t, err, ErrNotFound, "target 屬於別的 request 必須被當成不存在拒絕")

	gotReq, gotRev, err := repo.GetSnapshot(ctx, reqA.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, rootA.ID, gotReq.ActiveRevisionID, "被拒絕的 rewind 不得改動 request A 的 active revision")
	require.Equal(t, rootA.ID, gotRev.ID)

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_request_revisions WHERE request_id = $1`, reqA.ID,
	).Scan(&count))
	require.Equal(t, int64(1), count, "被拒絕的 rewind 不得在 request A 底下多插一個版本")
}
