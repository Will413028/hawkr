//go:build integration

package repository

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// mustConversationIDForRequest 是測試用的小工具：CreateRequest 只回傳
// Request／Revision，不曝露它在同一交易內建立的 conversation ID（見
// revision.go CreateRequest 上方註解：conversationId 對外只透過
// AppendRevision 的回傳值曝露），所以想在測試裡拿到那個 ID 時直接查表，
// 跟 revision_test.go 既有的「直接打 pool.QueryRow 驗證」風格一致。
func mustConversationIDForRequest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, requestID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, requestID,
	).Scan(&id))
	return id
}

// mustAppendTrivialRevision 是測試用的小工具：對一個剛 CreateRequest 出來、
// 還沒有任何 answer_patch 的 request 補一筆最小可行的 AppendRevision，讓它
// 的 conversation 具備「至少一個 completed turn」——ListProcurementConversations
// 的 EXISTS(procurement_answer_patches) 過濾要求這個前提，否則測試建出來的
// conversation 全部會被新過濾條件擋在列表外。clientRequestID 由呼叫端傳入，
// 確保同一個 test 內多次呼叫不會撞冪等鍵。
func mustAppendTrivialRevision(t *testing.T, ctx context.Context, repo *Repo, requestID int64, buyerID identity.BuyerID, clientRequestID string) {
	t.Helper()
	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("測試品項")}, refTime())
	_, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: requestID, BuyerID: buyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: clientRequestID,
		PayloadJSON: []byte(`{"reply":"r"}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
}

func TestRepo_ListConversations_ReturnsLimitPlusOneAndExcludesSoftDeleted(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	convIDs := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		req, _, err := repo.CreateRequest(ctx, testBuyerID)
		require.NoError(t, err)
		// 補一筆 answer_patch：ListProcurementConversations 現在只回傳有
		// 至少一個 completed turn 的 conversation（見 conversations.sql 的
		// EXISTS 過濾），單純 CreateRequest 不會有 answer_patch。
		mustAppendTrivialRevision(t, ctx, repo, req.ID, testBuyerID, "list-test-seed-"+strconv.Itoa(i))
		convIDs = append(convIDs, mustConversationIDForRequest(t, ctx, pool, req.ID))
	}

	// limitPlusOne=3 模擬呼叫端要求 page_size=2：repository 老實回傳 limit+1 筆，
	// trim 掉多抓的那一筆是 usecase 層的責任，不是這裡。
	got, err := repo.ListConversations(ctx, testBuyerID, nil, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)

	found, alreadyDeleted, err := repo.DeleteConversation(ctx, convIDs[0], testBuyerID, "del-for-list-test")
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, alreadyDeleted)

	got2, err := repo.ListConversations(ctx, testBuyerID, nil, 3)
	require.NoError(t, err)
	require.Len(t, got2, 2, "軟刪一筆之後只剩兩筆未刪的")
	for _, c := range got2 {
		require.NotEqual(t, convIDs[0], c.ID, "軟刪的 conversation 不該出現在列表")
	}
}

func TestRepo_RenameConversation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	convID := mustConversationIDForRequest(t, ctx, pool, req.ID)

	found, hit, record, err := repo.RenameConversation(ctx, convID, testBuyerID, "新名稱", "rename-cr-1")
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, hit)
	require.Equal(t, "新名稱", record.Title)

	var titleSource string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title_source FROM procurement_conversations WHERE id = $1`, convID,
	).Scan(&titleSource))
	require.Equal(t, "USER", titleSource)

	// 同 clientRequestId 重送：即使這次傳了不同的 title，也要回放第一次的結果，
	// 不得真的再執行一次改名。
	found2, hit2, record2, err := repo.RenameConversation(ctx, convID, testBuyerID, "應該被忽略的名稱", "rename-cr-1")
	require.NoError(t, err)
	require.True(t, found2)
	require.True(t, hit2)
	require.Equal(t, "新名稱", record2.Title, "冪等重播回傳第一次的值，不是這次傳入的新 title")

	// 對不存在的 id：404。
	found3, hit3, _, err := repo.RenameConversation(ctx, convID+999_999_999, testBuyerID, "x", "rename-cr-missing")
	require.NoError(t, err)
	require.False(t, found3)
	require.False(t, hit3)
}

func TestRepo_DeleteConversation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	convID := mustConversationIDForRequest(t, ctx, pool, req.ID)

	found, alreadyDeleted, err := repo.DeleteConversation(ctx, convID, testBuyerID, "delete-cr-1")
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, alreadyDeleted)

	// 同 clientRequestId 重送：走冪等分支，found=true（這次呼叫本身一定成功過）。
	found2, alreadyDeleted2, err := repo.DeleteConversation(ctx, convID, testBuyerID, "delete-cr-1")
	require.NoError(t, err)
	require.True(t, found2)
	require.False(t, alreadyDeleted2)

	// 不同 clientRequestId，對已刪除的再刪一次：200 no-op，不是 404。
	found3, alreadyDeleted3, err := repo.DeleteConversation(ctx, convID, testBuyerID, "delete-cr-2")
	require.NoError(t, err)
	require.True(t, found3)
	require.True(t, alreadyDeleted3)

	// 對不存在的 id：404。
	found4, alreadyDeleted4, err := repo.DeleteConversation(ctx, convID+999_999_999, testBuyerID, "delete-cr-missing")
	require.NoError(t, err)
	require.False(t, found4)
	require.False(t, alreadyDeleted4)
}

func TestRepo_GetConversationForMessages(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	convID := mustConversationIDForRequest(t, ctx, pool, req.ID)

	conv, status, err := repo.GetConversationForMessages(ctx, convID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, convID, conv.ID)
	require.Equal(t, req.ID, conv.RequestID)
	require.Equal(t, "CLARIFYING", status, "procurement_requests 的初始狀態")

	_, _, err = repo.GetConversationForMessages(ctx, convID, identity.BuyerID(999_999))
	require.ErrorIs(t, err, ErrNotFound, "非本人必須看起來像不存在")

	found, _, err := repo.DeleteConversation(ctx, convID, testBuyerID, "delete-before-get")
	require.NoError(t, err)
	require.True(t, found)

	_, _, err = repo.GetConversationForMessages(ctx, convID, testBuyerID)
	require.ErrorIs(t, err, ErrNotFound, "已軟刪的 conversation 也視為不存在")
}

func TestRepo_ListAnswerPatchesByRequestID_ReturnsInTimeOrder(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	slots1, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("梨")}, refTime())
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots1,
		Source: "LLM_EXTRACT", ClientRequestID: "patch-1", PayloadJSON: []byte(`{"reply":"r1"}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	slots2, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("蘋果")}, refTime())
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots2,
		Source: "LLM_EXTRACT", ClientRequestID: "patch-2", PayloadJSON: []byte(`{"reply":"r2"}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	patches, err := repo.ListAnswerPatchesByRequestID(ctx, req.ID)
	require.NoError(t, err)
	require.Len(t, patches, 2)
	require.Equal(t, "patch-1", patches[0].ClientRequestID)
	require.Equal(t, "patch-2", patches[1].ClientRequestID)
}

// TestRepo_ListConversations_ExcludesConversationsWithoutAnyAnswerPatch 驗證
// conversations.sql 的 EXISTS(procurement_answer_patches) 過濾：一個 request
// 建立後若從未成功跑完一輪（真孤兒——例如 CreateRequest 之後、AppendRevision
// 之前當掉；或 legacy backfill 出來、title/last_message_preview 皆空但也
// 沒有任何 answer_patch 的 row），它的 conversation 不該出現在列表。
//
// 判準刻意不是「title/last_message_preview 是否為空字串」——那個 proxy 對
// legacy backfill 但確實有訊息歷史的 row 會是偽陽性（見同檔案上方
// mustAppendTrivialRevision 與 conversations.sql 的註解）。這裡直接控制
// 「有沒有 answer_patch」這個唯一正確訊號本身，用兩個 conversation
// 對照：一個零 patch（必須被擋在列表外），一個至少一 patch（必須出現），
// 確認過濾用的是這個訊號，不是碰巧因為欄位空白而被排除。
func TestRepo_ListConversations_ExcludesConversationsWithoutAnyAnswerPatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	// 零 patch：只 CreateRequest，不補 AppendRevision——模擬真孤兒 row。
	orphanReq, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	orphanConvID := mustConversationIDForRequest(t, ctx, pool, orphanReq.ID)

	// 至少一 patch：CreateRequest 後補一輪 AppendRevision——模擬有完成過
	// 至少一輪的正常 conversation（也涵蓋「legacy row 但有真實訊息歷史」
	// 這個情境本身要能通過過濾）。
	completedReq, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	mustAppendTrivialRevision(t, ctx, repo, completedReq.ID, testBuyerID, "exists-filter-completed")
	completedConvID := mustConversationIDForRequest(t, ctx, pool, completedReq.ID)

	got, err := repo.ListConversations(ctx, testBuyerID, nil, 10)
	require.NoError(t, err)

	gotIDs := make([]int64, 0, len(got))
	for _, c := range got {
		gotIDs = append(gotIDs, c.ID)
	}
	require.NotContains(t, gotIDs, orphanConvID, "零 answer_patch 的 conversation 不該出現在列表")
	require.Contains(t, gotIDs, completedConvID, "至少一個 answer_patch 的 conversation 必須出現在列表")
}

// setConversationUpdatedAt 直接改寫 updated_at。列表排序／游標測試需要確定的
// 時間差：靠連續呼叫 AppendRevision 讓 now() 自然拉開間距在快機器上可能撞到
// 同一微秒，斷言會偶發翻車（repo CLAUDE.md §19：fixture 必須 deterministic）。
func setConversationUpdatedAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, convID int64, at time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`UPDATE procurement_conversations SET updated_at = $2 WHERE id = $1`, convID, at)
	require.NoError(t, err)
}

// mustSeedConversation 建一個 request＋一輪 turn，回傳它的 conversation ID。
func mustSeedConversation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repo, clientRequestID string) int64 {
	t.Helper()
	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	mustAppendTrivialRevision(t, ctx, repo, req.ID, testBuyerID, clientRequestID)
	return mustConversationIDForRequest(t, ctx, pool, req.ID)
}

// TestRepo_ListConversations_OrdersByUpdatedAtDescNotID 釘住排序判準本身：
// 列表是「最近有動靜的排最上面」（updated_at DESC），不是「最近建立的排最上面」
// （id DESC）。刻意讓 id 順序與 updated_at 順序完全相反——若 SQL 退回 ORDER BY
// c.id DESC，回傳順序會整個顛倒，這個測試就會紅。
func TestRepo_ListConversations_OrdersByUpdatedAtDescNotID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	// testIDGen 遞增，所以 convA < convB < convC（建立順序＝id 順序）。
	convA := mustSeedConversation(t, ctx, pool, repo, "order-seed-a")
	convB := mustSeedConversation(t, ctx, pool, repo, "order-seed-b")
	convC := mustSeedConversation(t, ctx, pool, repo, "order-seed-c")
	require.Less(t, convA, convB)
	require.Less(t, convB, convC)

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	setConversationUpdatedAt(t, ctx, pool, convA, base.Add(3*time.Hour)) // 最新回覆
	setConversationUpdatedAt(t, ctx, pool, convB, base.Add(2*time.Hour))
	setConversationUpdatedAt(t, ctx, pool, convC, base.Add(1*time.Hour)) // 最舊回覆但 id 最大

	got, err := repo.ListConversations(ctx, testBuyerID, nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, []int64{convA, convB, convC}, []int64{got[0].ID, got[1].ID, got[2].ID},
		"排序依 updated_at DESC；id DESC 的話順序會是 convC/convB/convA")
}

// TestRepo_ListConversations_CompositeCursorPaginatesThroughRealSQL 是「翻第二頁」
// 打到真 SQL 的覆蓋——分支審查指出 cursor 那半條 WHERE 在這個 codebase 完全沒有
// 任何測試走過（handler／usecase 兩層的 fake 都是自己重寫的過濾邏輯，測不到真的
// row-wise comparison）。第二段刻意讓兩列 updated_at 完全相同，走的是「靠 id
// 決勝負」那條路徑——單欄 updated_at cursor 在這裡會漏列或重複列。
func TestRepo_ListConversations_CompositeCursorPaginatesThroughRealSQL(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	conv1 := mustSeedConversation(t, ctx, pool, repo, "cursor-seed-1")
	conv2 := mustSeedConversation(t, ctx, pool, repo, "cursor-seed-2")
	conv3 := mustSeedConversation(t, ctx, pool, repo, "cursor-seed-3")

	base := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	newest, middle, oldest := base.Add(3*time.Hour), base.Add(2*time.Hour), base.Add(time.Hour)
	setConversationUpdatedAt(t, ctx, pool, conv1, newest)
	setConversationUpdatedAt(t, ctx, pool, conv2, middle)
	setConversationUpdatedAt(t, ctx, pool, conv3, oldest)

	page1, err := repo.ListConversations(ctx, testBuyerID, nil, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, []int64{conv1, conv2}, []int64{page1[0].ID, page1[1].ID})

	// 第二頁：游標＝第一頁最後一筆的 (updated_at, id)。
	last := page1[len(page1)-1]
	page2, err := repo.ListConversations(ctx, testBuyerID, &ConversationCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1, "第二頁只剩最後一筆")
	require.Equal(t, conv3, page2[0].ID)
	require.NotEqual(t, last.ID, page2[0].ID, "游標那一列本身不得重複出現在下一頁")

	// updated_at 撞值：conv2 與 conv3 設成同一個時間，游標指到其中 id 較大者，
	// 下一頁必須剛好接到 id 較小的那一筆（不漏、不重複）。
	tie := base.Add(30 * time.Minute)
	setConversationUpdatedAt(t, ctx, pool, conv2, tie)
	setConversationUpdatedAt(t, ctx, pool, conv3, tie)

	all, err := repo.ListConversations(ctx, testBuyerID, nil, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{conv1, conv3, conv2}, []int64{all[0].ID, all[1].ID, all[2].ID},
		"撞值時以 id DESC 決勝負（conv3 > conv2）")

	tiePage, err := repo.ListConversations(ctx, testBuyerID, &ConversationCursor{UpdatedAt: tie, ID: conv3}, 10)
	require.NoError(t, err)
	require.Len(t, tiePage, 1)
	require.Equal(t, conv2, tiePage[0].ID, "撞值時游標必須接到 id 較小的那一筆，不是整批跳過")
}

// TestRepo_RenameConversation_RejectsReplayFromDifferentConversation 是 Finding 3
// 的核心證明：同一把 clientRequestId 被重用在「另一個」conversation 上時，冪等
// 命中不得被當成這次呼叫的合法重播。冪等鍵是 (buyer_id, client_request_id)，
// 不含 conversation_id，所以命中本身不代表「這個 conversation 改過名」。
func TestRepo_RenameConversation_RejectsReplayFromDifferentConversation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	convA := mustSeedConversation(t, ctx, pool, repo, "rename-cross-seed-a")
	convB := mustSeedConversation(t, ctx, pool, repo, "rename-cross-seed-b")

	const sharedKey = "reused-client-request-id"
	found, hit, rec, err := repo.RenameConversation(ctx, convA, testBuyerID, "A 的新名稱", sharedKey)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, hit)
	require.Equal(t, "A 的新名稱", rec.Title)

	// 同一把 key 打到 convB：必須回 404 語意（found=false），不得回報成功。
	found2, hit2, _, err := repo.RenameConversation(ctx, convB, testBuyerID, "B 想改的名稱", sharedKey)
	require.NoError(t, err)
	require.False(t, found2, "跨 conversation 的 key 重用不是這次呼叫的合法重播")
	require.False(t, hit2)

	require.Equal(t, "", mustConversationTitle(t, ctx, pool, convB),
		"convB 的標題必須完全沒被動過——回報成功卻什麼都沒改是這個 finding 的實際危害")
	require.Equal(t, "A 的新名稱", mustConversationTitle(t, ctx, pool, convA))
}

// TestRepo_DeleteConversation_RejectsReplayFromDifferentConversation 是 Finding 3
// 在 DELETE 這一側的證明：修正前，重用的 key 會讓第二次呼叫回 200「刪除成功」，
// 但 convB 一列都沒被刪——client 看到成功、對話卻還在列表上。
func TestRepo_DeleteConversation_RejectsReplayFromDifferentConversation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	convA := mustSeedConversation(t, ctx, pool, repo, "delete-cross-seed-a")
	convB := mustSeedConversation(t, ctx, pool, repo, "delete-cross-seed-b")

	const sharedKey = "reused-delete-client-request-id"
	found, alreadyDeleted, err := repo.DeleteConversation(ctx, convA, testBuyerID, sharedKey)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, alreadyDeleted)

	found2, alreadyDeleted2, err := repo.DeleteConversation(ctx, convB, testBuyerID, sharedKey)
	require.NoError(t, err)
	require.False(t, found2, "跨 conversation 的 key 重用不得被當成 convB 刪除成功")
	require.False(t, alreadyDeleted2)

	require.False(t, mustConversationDeleted(t, ctx, pool, convB),
		"convB 必須仍未刪除——這是「回報成功卻什麼都沒刪」的直接反證")
	require.True(t, mustConversationDeleted(t, ctx, pool, convA))

	// convB 仍應出現在列表上（從使用者角度看「刪不掉」這件事的最終症狀）。
	rows, err := repo.ListConversations(ctx, testBuyerID, nil, 10)
	require.NoError(t, err)
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	require.Contains(t, ids, convB)
	require.NotContains(t, ids, convA)
}

// TestRepo_ConversationCommand_RejectsReplayAcrossOperationTypes 驗證同一個
// conversation 上、同一把 key 被換成另一種操作時也不算重播（result_json 的
// op 欄位是判準）。這裡用 rename → delete：修正前 DELETE 會看到冪等命中就
// 直接回 found=true，實際上該對話從未被刪。
func TestRepo_ConversationCommand_RejectsReplayAcrossOperationTypes(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	conv := mustSeedConversation(t, ctx, pool, repo, "cross-op-seed")

	const sharedKey = "reused-across-ops"
	found, _, _, err := repo.RenameConversation(ctx, conv, testBuyerID, "先改名", sharedKey)
	require.NoError(t, err)
	require.True(t, found)

	found2, alreadyDeleted, err := repo.DeleteConversation(ctx, conv, testBuyerID, sharedKey)
	require.NoError(t, err)
	require.False(t, found2, "同一把 key 換成另一種操作，對 DELETE 而言不是重播")
	require.False(t, alreadyDeleted)
	require.False(t, mustConversationDeleted(t, ctx, pool, conv),
		"對話必須仍未刪除——不得因為 rename 用過同一把 key 就冒充刪除成功")
}

func mustConversationTitle(t *testing.T, ctx context.Context, pool *pgxpool.Pool, convID int64) string {
	t.Helper()
	var title string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title FROM procurement_conversations WHERE id = $1`, convID).Scan(&title))
	return title
}

func mustConversationDeleted(t *testing.T, ctx context.Context, pool *pgxpool.Pool, convID int64) bool {
	t.Helper()
	var deletedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT deleted_at FROM procurement_conversations WHERE id = $1`, convID).Scan(&deletedAt))
	return deletedAt != nil
}
