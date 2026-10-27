//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

const testBuyerID = identity.BuyerID(1234)

func TestRepo_CreateRequest_MakesInitialRevision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	require.NotZero(t, req.ID)
	require.Equal(t, testBuyerID, req.BuyerID)
	require.Equal(t, 1, rev.RevisionNo)
	require.Equal(t, "ACTIVE", rev.Status)
	require.Equal(t, rev.ID, req.ActiveRevisionID)
	require.Equal(t, rev.ID, rev.BranchID, "首個 revision 自己就是 branch 根")
}

// TestRepo_CreateRequest_CreatesConversationRow 驗證 CreateRequest 建 request
// 的同一個交易內，procurement_conversations 也要有一列對應的 row，初始
// title_source 是 AUTO（見 conversations.sql 的 INSERT 預設值）。
func TestRepo_CreateRequest_CreatesConversationRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	var (
		buyerID     int64
		requestID   int64
		titleSource string
	)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT buyer_id, request_id, title_source FROM procurement_conversations WHERE request_id = $1`, req.ID,
	).Scan(&buyerID, &requestID, &titleSource))
	require.Equal(t, int64(testBuyerID), buyerID)
	require.Equal(t, req.ID, requestID)
	require.Equal(t, "AUTO", titleSource)
}

// TestRepo_AppendRevision_AutoTitleThenPreservedAfterUserRename 驗證
// AppendRevision 三條回傳路徑之一（正常寫入路徑）：第一輪 slots 齊全（item + delivery_city）時
// 自動生成 title；手動改名（title_source 變 USER）之後，即使第二輪 slots 又帶了
// 足以組出 auto title 的值，UPDATE ... WHERE title_source = 'AUTO' 的守衛必須讓
// 那句更新影響 0 列——title 維持使用者取的名字不被蓋掉，但 last_message_preview
// 沒有這個守衛，仍要跟著每一輪更新。
func TestRepo_AppendRevision_AutoTitleThenPreservedAfterUserRename(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	slots1, _ := domain.Slots{}.Apply(domain.SlotPatch{
		Item: ptr("寶島梨禮盒"), DeliveryCity: ptr("台北市"),
	}, refTime())
	rev1, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots1,
		Source: "LLM_EXTRACT", ClientRequestID: "conv-title-1", PayloadJSON: []byte(`{"reply":"第一輪回覆"}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	require.NotZero(t, rev1.ConversationID, "AppendRevision 正常寫入路徑必須補上 ConversationID")

	var title, titleSource, preview string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title, title_source, last_message_preview FROM procurement_conversations WHERE id = $1`, rev1.ConversationID,
	).Scan(&title, &titleSource, &preview))
	require.Equal(t, "寶島梨禮盒｜台北市", title)
	require.Equal(t, "AUTO", titleSource)
	require.Equal(t, "第一輪回覆", preview)

	found, hit, record, err := repo.RenameConversation(ctx, rev1.ConversationID, testBuyerID, "手動改名", "rename-for-title-test")
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, hit)
	require.Equal(t, "手動改名", record.Title)

	// 第二輪 slots 也齊全（item + delivery_city 都有值），autoTitleOf 一定會算出
	// 非空字串，藉此確認 DB 端 WHERE title_source = 'AUTO' 的守衛真的擋住了覆寫,
	// 而不是恰好因為 autoTitleOf 回空字串才「看起來」沒被蓋掉。
	slots2, _ := domain.Slots{}.Apply(domain.SlotPatch{
		Item: ptr("茂谷柑"), DeliveryCity: ptr("台中市"),
	}, refTime())
	rev2, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots2,
		Source: "LLM_EXTRACT", ClientRequestID: "conv-title-2", PayloadJSON: []byte(`{"reply":"第二輪回覆"}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	require.Equal(t, rev1.ConversationID, rev2.ConversationID, "同一個 request 只會有一個 conversation")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT title, title_source, last_message_preview FROM procurement_conversations WHERE id = $1`, rev1.ConversationID,
	).Scan(&title, &titleSource, &preview))
	require.Equal(t, "手動改名", title, "USER 改過的標題不該被 AUTO 覆寫")
	require.Equal(t, "USER", titleSource)
	require.Equal(t, "第二輪回覆", preview, "preview 沒有 title_source 那種守衛，仍要跟著每輪更新")
}

func TestRepo_AppendRevision_SupersedesPrevious(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, rev1, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	// 只填 Item，六個必填欄位還缺五個，狀態應維持 CLARIFYING——跟 usecase.Handle
	// 實際算出來的值一致（見 turn.go），這裡手動代入是因為本測試直接打 repository
	// 層，不經過 usecase 那段算式。
	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("寶島梨禮盒")}, refTime())
	rev2, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "cr-1", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	require.Equal(t, 2, rev2.RevisionNo)
	require.Equal(t, rev1.ID, rev2.ParentRevisionID)
	require.Equal(t, rev1.BranchID, rev2.BranchID)

	gotReq, gotRev, err := repo.GetSnapshot(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, rev2.ID, gotReq.ActiveRevisionID)
	require.Equal(t, "ACTIVE", gotRev.Status)
	require.Equal(t, rev2.ID, gotRev.ID)
	require.Equal(t, "CLARIFYING", gotReq.Status,
		"request 的狀態必須是 AppendRevision 傳進去的 RequestStatus，不是空字串")

	var supersededStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_request_revisions WHERE id = $1`, rev1.ID,
	).Scan(&supersededStatus))
	require.Equal(t, "SUPERSEDED", supersededStatus)
}

func TestRepo_AppendRevision_IdempotentByClientRequestID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("梨")}, refTime())

	in := AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "same-key", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING", // 只填 Item，必填欄位還缺，見上一個測試的註解
	}
	first, err := repo.AppendRevision(ctx, in)
	require.NoError(t, err)
	second, err := repo.AppendRevision(ctx, in)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID, "同 client_request_id 重送不得產生第二個版本")
	// 第二次呼叫走「交易內冪等命中提早 return」這條路徑（三條回傳路徑之一），
	// 驗證它也有補上 ConversationID，不是只有正常寫入路徑才有。
	require.NotZero(t, second.ConversationID)
	require.Equal(t, first.ConversationID, second.ConversationID)

	var count int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_request_revisions WHERE request_id = $1`, req.ID,
	).Scan(&count))
	require.Equal(t, int64(2), count, "初始版本 + 一次 append")
}

func TestRepo_GetSnapshot_RejectsOtherBuyer(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	_, _, err = repo.GetSnapshot(ctx, req.ID, identity.BuyerID(9999))
	require.ErrorIs(t, err, ErrNotFound, "跨買家讀取必須看起來像不存在，不得洩漏存在性")
}

// TestRepo_GetSnapshot_DoesNotPopulateConversationID 釘住 review round 2
// Minor #3 的裁決：GetSnapshot（SnapshotReader 這個共用讀取介面唯一的方法，
// 11 個呼叫點只有 internal/grpc 的 GetSnapshot RPC handler 真的需要
// conversationId）不得再像 round 1 那樣無條件多查一次 procurement_conversations
// ——那個成本會落在另外 10 個完全用不到這個值的呼叫端上（含經由 GetSlots
// 間接呼叫的路徑）。conversationId 改由唯一需要它的呼叫端直接呼叫
// GetConversationIDForRequest 取得，見該方法與下面的測試。
func TestRepo_GetSnapshot_DoesNotPopulateConversationID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	_, gotRev, err := repo.GetSnapshot(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Zero(t, gotRev.ConversationID,
		"GetSnapshot 不該再解析 conversationId——這是 round 2 Minor #3 移除的行為，回歸代表又把成本疊回共用 primitive 上了")
}

// TestRepo_GetConversationIDForRequest_ResolvesRealConversation 是
// GetConversationIDForRequest 本身（round 2 Minor #3 新增的匯出方法）的
// dbtest：一個真的存在的 request 必須解出它早已建好的 conversation id。
func TestRepo_GetConversationIDForRequest_ResolvesRealConversation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	got, err := repo.GetConversationIDForRequest(ctx, req.ID)
	require.NoError(t, err)
	require.NotZero(t, got)

	var wantConvID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, req.ID,
	).Scan(&wantConvID))
	require.Equal(t, wantConvID, got)
}

// TestRepo_GetConversationIDForRequest_UnknownRequest_ReturnsZeroNotError
// 驗證「查不到」的語意：這支方法對一個沒有 conversation row 的 requestID
// （這裡直接用一個從未建立過的 id 模擬）回 0, nil，不是 error——呼叫端
// （GetSnapshot handler）把它當「這次還解析不出來」處理，不當硬錯誤。
func TestRepo_GetConversationIDForRequest_UnknownRequest_ReturnsZeroNotError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	got, err := repo.GetConversationIDForRequest(ctx, 999999999)
	require.NoError(t, err)
	require.Zero(t, got)
}

// TestRepo_AppendRevision_SlotsJSONRoundTripsThroughJSONB 是 Fix round 1 item 3
// （models.go 從 gorm.io/datatypes.JSON 換成 encoding/json.RawMessage；jsonb 型別
// 現由 db/sqlc.yaml 的 go_type override 決定，已無 gorm tag）
// 的實測：不能只信「編譯得過」，要證明資料真的以 jsonb 型別存進 DB、且內容原封不動讀得回來。
func TestRepo_AppendRevision_SlotsJSONRoundTripsThroughJSONB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	slots, rejected := domain.Slots{}.Apply(domain.SlotPatch{
		Item: ptr("寶島梨禮盒"), Quantity: ptr(50), Unit: ptr("KG"),
		BudgetPerUnit: ptr(300), DeliveryCity: ptr("台北市"), DeliveryDate: ptr("2026-09-01"),
	}, refTime())
	require.Empty(t, rejected)

	// 六個必填欄位全部填齊，對應 RequestStatus 應該是 REQUIREMENT_READY（見
	// turn.go 實際算式）；連同下面的 request-level status 讀回，補上「AppendRevision
	// 真的把 RequestStatus 寫進 procurement_requests.status」這一段 repository 層
	// 原本缺的覆蓋（先前只驗證過 Revision.Status，沒驗證過 Request.Status）。
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "jsonb-roundtrip", PayloadJSON: []byte(`{}`),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	var gotRequestStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM procurement_requests WHERE id = $1`, req.ID,
	).Scan(&gotRequestStatus))
	require.Equal(t, "REQUIREMENT_READY", gotRequestStatus,
		"request 的狀態必須是 AppendRevision 傳進去的 RequestStatus，不是空字串")

	// 走 repo 的公開讀取路徑：unmarshal 回 domain.Slots 後內容要完全一致。
	gotSlots, err := repo.GetSlots(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, slots, gotSlots)

	// 直接用 Postgres 的 jsonb 運算子（->>）讀欄位——如果欄位其實被塞成 text/bytea
	// 而非真正的 jsonb，這個運算子會直接報錯，不會安靜地矇混過去。
	var itemFromJSONB string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT slots_json->>'item' FROM procurement_request_revisions WHERE request_id = $1 AND status = 'ACTIVE'`,
		req.ID,
	).Scan(&itemFromJSONB))
	require.Equal(t, "寶島梨禮盒", itemFromJSONB)

	var colType string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'procurement_request_revisions' AND column_name = 'slots_json'`,
	).Scan(&colType))
	require.Equal(t, "jsonb", colType, "schema 定義本來就是 jsonb；這裡驗證的是寫入時沒有繞過型別")
}

// TestRepo_AppendRevision_SelfHealsMissingConversationRow 釘住 rolling deploy
// 視窗這個 landmine：DB migration 先跑完、但舊 pod 還在用「不建 conversation
// row」的 CreateRequest 服務流量，那段期間建立的 request 永遠沒有對應的
// conversation row（Task 2 的 backfill migration 只涵蓋 migration 執行前就存在
// 的列，救不到 rollout 期間新生的）。修正前 AppendRevision 尾端是一句純 SELECT，
// 會吃到沒被對映的 pgx.ErrNoRows，讓那些 request 之後每一輪都固定 500 且無法
// 自我修復。這裡用「直接刪掉 conversation row」重現那個狀態。
func TestRepo_AppendRevision_SelfHealsMissingConversationRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	// 重現舊 pod 的 CreateRequest：request 有了，conversation row 沒有。
	_, err = pool.Exec(ctx, `DELETE FROM procurement_conversations WHERE request_id = $1`, req.ID)
	require.NoError(t, err)

	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("梨")}, refTime())
	rev, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "self-heal-1",
		PayloadJSON: []byte(`{"reply":"r"}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err, "缺 conversation row 不得讓這一輪 500")
	require.NotZero(t, rev.ConversationID)

	var persisted int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, req.ID).Scan(&persisted))
	require.Equal(t, persisted, rev.ConversationID, "補建的那一列就是回傳的 conversationId")

	// 第二輪（不同 client_request_id）必須沿用同一列，不是每輪各建一列——
	// uq_procurement_conversations_request 也擋，但這裡要驗證的是 ON CONFLICT
	// 走的是「拿既有列」而不是報錯。
	slots2, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("蘋果")}, refTime())
	rev2, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots2,
		Source: "LLM_EXTRACT", ClientRequestID: "self-heal-2",
		PayloadJSON: []byte(`{"reply":"r2"}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	require.Equal(t, rev.ConversationID, rev2.ConversationID)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_conversations WHERE request_id = $1`, req.ID).Scan(&count))
	require.Equal(t, 1, count, "自我修復不得每輪多插一列")
}

// TestRepo_FindByIdempotencyKey_SelfHealsMissingConversationRow 是同一個
// landmine 在命令層冪等預檢那一側：修正前這裡把缺列對映成 ErrNotFound，讓
// rolling deploy 視窗內建立的 request 的合法重送被誤判成「這個命令沒做過」，
// 於是整條 turn 被重跑一次（重複扣資源／重複回覆），而不是回放前次結果。
func TestRepo_FindByIdempotencyKey_SelfHealsMissingConversationRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	slots, _ := domain.Slots{}.Apply(domain.SlotPatch{Item: ptr("梨")}, refTime())
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: slots,
		Source: "LLM_EXTRACT", ClientRequestID: "find-self-heal-1",
		PayloadJSON: []byte(`{"reply":"r"}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM procurement_conversations WHERE request_id = $1`, req.ID)
	require.NoError(t, err)

	got, err := repo.FindByIdempotencyKey(ctx, testBuyerID, "find-self-heal-1")
	require.NoError(t, err, "缺 conversation row 不得讓合法重送被當成沒做過")
	require.Equal(t, req.ID, got.RequestID)
	require.NotZero(t, got.ConversationID)

	var persisted int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM procurement_conversations WHERE request_id = $1`, req.ID).Scan(&persisted))
	require.Equal(t, persisted, got.ConversationID)
}
