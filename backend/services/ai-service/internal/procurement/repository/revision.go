package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrNotFound 同時涵蓋「不存在」與「不屬於這個買家」——對外不區分兩者，
// 否則會洩漏他人 request 的存在性。
var ErrNotFound = errors.New("procurement: request not found")

// ErrMissingRequestStatus 是 AppendRevision 的輸入驗證：procurement_requests.status
// 欄位是 TEXT NOT NULL DEFAULT 'CLARIFYING'，沒有 CHECK 限制合法值，
// SetProcurementRequestStatus 又是一句不做額外檢查的 UPDATE ... SET status = $2。
// 若呼叫端忘記算好 in.RequestStatus 就送進來，這句 UPDATE 會用空字串把欄位靜默
// 洗掉——不報錯、不 panic，只留下一筆看起來合法、實際錯誤的資料，比直接噴錯難抓
// 得多。寧可在還沒進交易前就用一行檢查提早、大聲地擋下來。
var ErrMissingRequestStatus = errors.New("procurement: AppendInput.RequestStatus must not be empty")

// errLostIdempotencyRace 是「答案 patch 的冪等鍵被併發的另一次呼叫先寫走」的內部
// 哨兵。它不是 DB 錯誤——query 用 ON CONFLICT DO UPDATE，撞鍵不拋錯，交易仍然健康；
// 回傳它只是為了讓 WithTx 回捲這一輪已經寫下的 revision 與 supersede，之後在交易外
// 由 recoverIdempotentAppend 讀出贏家的結果。呼叫端看不到它。
var errLostIdempotencyRace = errors.New("procurement: lost answer patch idempotency race")

type Repo struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
	newID     func() int64
}

// New 的 ID 產生器由 fx 注入 tsid.NewGenerator（`github.com/Will413028/hawkr/go-pkg/tsid`，
// 簽名為 `func NewGenerator() func() int64`），與 seller-service 同一份實作。
// 不用 package 級全域：測試要能替換成可預期的序號。
//
// 交易在本 struct 的方法內部開（`r.txManager.WithTx`），不是由 usecase 層開——procurement
// 的 usecase 介面（RevisionStore.AppendRevision 等）宣告成「一次呼叫、內部保證原子性」的
// 契約，這個切法讓 usecase 層與其測試替身完全不受這次 GORM→sqlc 更換影響（見計畫
// Design Decisions #8）。
func New(pool *pgxpool.Pool, txManager database.TxManager, newID func() int64) *Repo {
	return &Repo{pool: pool, txManager: txManager, newID: newID}
}

func (r *Repo) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// AppendInput 是一次需求改動的完整輸入。RequestID 是資源 ID（不代表任何登入或
// 業務身分），維持裸 int64；BuyerID 是身分 ID，用 identity.BuyerID 讓
// 帳號／買家／賣場三種 ID 互相誤標這個 bug class 在編譯期就擋下來。
type AppendInput struct {
	RequestID       int64
	BuyerID         identity.BuyerID
	Slots           domain.Slots
	Source          string // LLM_EXTRACT | ANSWER | REWIND
	ClientRequestID string
	PayloadJSON     []byte
	// RequestStatus 是這次改動之後 request 應該落在的狀態（例如 CLARIFYING／
	// REQUIREMENT_READY）。跟著 revision 寫進同一個交易，不是事後另一次呼叫——
	// 呼叫端（usecase）在送進來之前就算好，repository 只負責原子寫入，理由見
	// AppendRevision 註解。
	RequestStatus string
	// QuotaReservationID／QuotaPeriodStart 把「這一輪開始前建立的額度預約」帶進
	// revision／answer_patch 持久化的同一個交易（doc 57 §7.5 point 3）——確認用量
	// 與寫入結果要嘛一起 commit、要嘛一起 rollback，不會出現「revision 寫成功但
	// 沒扣到額度」或「扣了額度但這一輪其實沒留下任何內容」。
	//
	// 身分軸沿用這個 struct 既有的 BuyerID（已經是 identity.BuyerID），不另開欄位：
	// 額度就是以買家為軸，跟其他欄位同一個身分。
	//
	// QuotaReservationID == 0 表示「這次呼叫沒有預約要確認」，AppendRevision 會
	// 整段跳過確認。正常的 turn 路徑永遠會帶值（usecase/turn.go），這個零值出口是
	// 給不經過額度管制的呼叫端用的（例如 RewindTo 那類後續擴充，以及尚未接上
	// 額度的單元測試替身）。
	QuotaReservationID int64
	QuotaPeriodStart   time.Time
}

// RewindInput 是一次回溯的完整輸入。TargetRevisionID 由呼叫端（usecase 層，見 Task 6
// 的 resolveRewindTarget）依 slotKey 解析出來，repository 只負責原子寫入。
type RewindInput struct {
	RequestID        int64
	BuyerID          identity.BuyerID
	TargetRevisionID int64
	ClientRequestID  string
	PayloadJSON      []byte
}

// CreateRequest 建立 request 與它的第一個（空）revision，並在同一交易內建立
// 這個 request 對應的 conversation row（title_source 初始為 AUTO）。
// conversationId 不透過這個方法對外露出——見下方 AppendRevision 的說明。
func (r *Repo) CreateRequest(ctx context.Context, buyerID identity.BuyerID) (Request, Revision, error) {
	var req Request
	var rev Revision

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		empty, _ := domain.Slots{}.Apply(domain.SlotPatch{}, nowUTC())
		slotsJSON, err := json.Marshal(empty)
		if err != nil {
			return err
		}

		reqRow, err := q.CreateProcurementRequest(txCtx, gen.CreateProcurementRequestParams{
			ID: r.newID(), BuyerID: int64(buyerID),
		})
		if err != nil {
			return err
		}

		revID := r.newID()
		revRow, err := q.CreateProcurementRevision(txCtx, gen.CreateProcurementRevisionParams{
			ID: revID, RequestID: reqRow.ID, RevisionNo: 1,
			ParentRevisionID: pointerInt64(0), // 首個 revision 沒有 parent；0 表示根（doc 55 §5.1）
			BranchID:         revID,           // 首個 revision 自己就是 branch 根
			SlotsJson:        slotsJSON, SlotsHash: empty.Hash(), Status: "ACTIVE",
		})
		if err != nil {
			return err
		}

		if err := q.SetProcurementRequestActiveRevision(txCtx, gen.SetProcurementRequestActiveRevisionParams{
			ID: reqRow.ID, ActiveRevisionID: pointerInt64(revID),
		}); err != nil {
			return err
		}

		convID := r.newID()
		if _, err := q.CreateProcurementConversation(txCtx, gen.CreateProcurementConversationParams{
			ID: convID, BuyerID: int64(buyerID), RequestID: reqRow.ID,
		}); err != nil {
			return err
		}

		req = toRequestEntity(reqRow)
		req.ActiveRevisionID = revID
		rev = toRevisionEntity(revRow)
		return nil
	})
	if err != nil {
		return Request{}, Revision{}, err
	}
	return req, rev, nil
}

// AppendRevision 在同一 branch 上追加一個版本，並把前一個 ACTIVE 標為 SUPERSEDED。
// 正常寫入路徑同時更新對應 conversation 的 last_message_preview，以及（僅當
// title_source 仍是 AUTO 時）自動標題。三條回傳路徑（交易內冪等命中、正常
// 寫入、交易外併發復原）最終都會補上回傳值的 ConversationID，見函式尾端統一
// 查詢那一段的註解。
//
// 冪等：同一買家同一 client_request_id 重送時，回傳前次產生的 revision，不建立
// 第二個版本。應用層「先查再寫」只在無併發時保證正確；真正併發撞上時，兩個
// goroutine 都可能查到「不存在」而同時往下寫，此時 DB 的
// uq_procurement_answer_patches_idempotency 唯一索引才是最終防線——輸的那個
// transaction 會在 INSERT answer_patch 時撞 23505 整條 ROLLBACK，下方在交易外
// 重新查一次就能拿到跟贏家一致的結果，而不是把裸的 DB 錯誤丟給呼叫端。
//
// ⛔ 「在交易外重新查一次」這句話只在**呼叫端自己沒有交易**時成立，所以交易要用
// r.withNestableTx 而不是裸的 WithTx：WithTx 巢狀時沿用外層交易，輸家撞 23505 打成
// aborted 的就是**呼叫端的**交易，而 recoverIdempotentAppend 用的 r.q(ctx) 正是那個
// 交易，於是拿到 25P02、回放失效，外層也再寫不進任何東西。交易內那道冪等預檢擋得住
// 序列重送（撞不到鍵），擋不住這條路徑——真正併發時兩邊都會預檢落空。哨兵：
// TestAppendRevision_ConcurrentLoserInsideCallerTransactionReplays。
//
// 對 procurement_requests row 做 SELECT ... FOR UPDATE：同一 request_id 的併發改動
// （即使 client_request_id 不同）在這裡序列化，避免兩者讀到同一個 parent revision、
// 算出同一個 revision_no，撞上 uq_procurement_request_revisions_no。
//
// in.RequestStatus 的寫入跟 revision／answer_patch 是同一個交易：早期版本讓呼叫端
// （usecase.Handle）在 AppendRevision commit 之後再另外呼叫一次 SetRequestStatus，
// 若那次呼叫失敗（斷線／timeout／DB 重啟），revision 已經 durable、但狀態沒更新，
// 而重送會在冪等預檢命中、走 replayResult 直接回成功——狀態永遠卡在錯的值，且
// 沒有任何路徑能自我修復。狀態寫入搬進本交易後，要嘛跟 revision 一起 commit，要嘛
// 一起 rollback（此時呼叫端會收到錯誤，可以重送，重送會重新算出同一個狀態再寫一次），
// 不會再有「revision 寫成功、狀態沒寫成功」這種中間態。
//
// in.QuotaReservationID 的確認（doc 59）也在同一個交易內、且是交易體的最後一步，
// 理由同上：額度用量與這一輪的內容必須同生共死。三條回傳路徑對它的效果分別是——
// 交易內冪等命中：直接 return，不確認（那次重送沒有產生新用量）；正常寫入：確認；
// 交易外併發復原：整個交易已經 ROLLBACK，確認也一併回滾，輸的那一邊的預約留待
// TTL 過期（贏的那一邊已經確認過它自己的那一筆，總數仍然正確）。
func (r *Repo) AppendRevision(ctx context.Context, in AppendInput) (Revision, error) {
	if in.RequestStatus == "" {
		return Revision{}, ErrMissingRequestStatus
	}

	var out Revision

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		existing, err := q.FindProcurementAnswerPatchByIdempotencyKey(txCtx, gen.FindProcurementAnswerPatchByIdempotencyKeyParams{
			BuyerID: int64(in.BuyerID), ClientRequestID: in.ClientRequestID,
		})
		if err == nil {
			row, err := q.GetProcurementRevisionByID(txCtx, existing.ResultRevisionID)
			if err != nil {
				return err
			}
			out = toRevisionEntity(row)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if _, err := q.GetProcurementRequestForUpdate(txCtx, gen.GetProcurementRequestForUpdateParams{
			ID: in.RequestID, BuyerID: int64(in.BuyerID),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		parentRow, err := q.GetActiveProcurementRevision(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		parent := toRevisionEntity(parentRow)

		slotsJSON, err := json.Marshal(in.Slots)
		if err != nil {
			return err
		}

		if err := q.SupersedeProcurementRevision(txCtx, parent.ID); err != nil {
			return err
		}

		revID := r.newID()
		revRow, err := q.CreateProcurementRevision(txCtx, gen.CreateProcurementRevisionParams{
			ID: revID, RequestID: in.RequestID, RevisionNo: int32(parent.RevisionNo + 1),
			ParentRevisionID: pointerInt64(parent.ID), BranchID: parent.BranchID,
			SlotsJson: slotsJSON, SlotsHash: in.Slots.Hash(), Status: "ACTIVE",
		})
		if err != nil {
			return err
		}
		out = toRevisionEntity(revRow)

		payload := in.PayloadJSON
		if len(payload) == 0 {
			payload = []byte(`{}`)
		}
		patchID := r.newID()
		patchRow, err := q.CreateProcurementAnswerPatch(txCtx, gen.CreateProcurementAnswerPatchParams{
			ID: patchID, RequestID: in.RequestID, BuyerID: int64(in.BuyerID),
			BaseRevisionID: pointerInt64(parent.ID), ResultRevisionID: out.ID,
			Source: in.Source, PayloadJson: payload, ClientRequestID: in.ClientRequestID,
		})
		if err != nil {
			return err
		}
		if patchRow.ID != patchID {
			return errLostIdempotencyRace
		}

		if err := q.BumpProcurementRequestRevision(txCtx, gen.BumpProcurementRequestRevisionParams{
			ID: in.RequestID, ActiveRevisionID: pointerInt64(out.ID),
		}); err != nil {
			return err
		}

		if err := q.UpdateProcurementConversationPreview(txCtx, gen.UpdateProcurementConversationPreviewParams{
			RequestID: in.RequestID, LastMessagePreview: previewOf(payload),
		}); err != nil {
			return err
		}
		if title := autoTitleOf(in.Slots); title != "" {
			if err := q.UpdateProcurementConversationAutoTitle(txCtx, gen.UpdateProcurementConversationAutoTitleParams{
				RequestID: in.RequestID, Title: title,
			}); err != nil {
				return err
			}
		}

		if err := q.SetProcurementRequestStatus(txCtx, gen.SetProcurementRequestStatusParams{
			ID: in.RequestID, Status: in.RequestStatus,
		}); err != nil {
			return err
		}

		// 額度確認：把這一輪開始前建立的預約轉成 durable 用量（doc 59）。
		//
		// 位置有三個必須成立的條件，缺一個就是靜默的計費錯誤：
		//  1. 必須在上面那個「冪等命中直接 return」的分支之後——那條路徑代表這次
		//     重送沒有真的寫入任何新版本，走到這裡會讓同一輪被計費第二次
		//     （confirmQuotaInTx 裡的 IncrementProcurementUsage 是無條件 +1）。
		//     這裡放在交易體最後，天然在它之後。
		//  2. 必須用這個 closure 自己的 q（上面 r.q(txCtx) 那一個），不能另外呼叫
		//     一次 r.q(...)——後者會綁到連線池而不是這個交易，advisory lock 會在
		//     那一句結束就釋放，DELETE／INCREMENT 也不在本交易內，整個原子性保證
		//     會消失，而且照樣編譯、照樣測試通過。
		//  3. 全函式只有這一個呼叫點，且在成功路徑上無條件執行。多個呼叫點會讓
		//     同一買家的併發 turn 以不同順序取得 advisory lock。放在最後也讓
		//     advisory lock 的持有時間最短（pg_advisory_xact_lock 持有到 commit）。
		//
		// 鎖順序：本交易是「先 procurement_requests 的 row lock
		// （GetProcurementRequestForUpdate），後 buyer advisory lock」，而
		// ReserveQuota／ReleaseQuota 只取 advisory lock、不碰 request row，
		// 因此不存在反向持有而互等的環。
		if in.QuotaReservationID == 0 {
			return nil // 這次呼叫不受額度管制，見 AppendInput 的說明
		}
		return r.confirmQuotaInTx(txCtx, q, in.QuotaReservationID, in.BuyerID, in.QuotaPeriodStart)
	})

	if err != nil {
		if !errors.Is(err, errLostIdempotencyRace) {
			return Revision{}, err
		}
		out, err = r.recoverIdempotentAppend(ctx, in.BuyerID, in.ClientRequestID)
		if err != nil {
			return Revision{}, err
		}
	}

	// 不管 out 是三條路徑中的哪一條產生的（交易內冪等命中、正常寫入、上面的
	// 交易外併發復原），到這裡都已經有一個確定的 RequestID，統一補查一次
	// conversation，讓呼叫端不必自己分辨走了哪條路徑。
	//
	// 用 get-or-create 而不是純查詢：CreateRequest 在自己的交易內建 conversation
	// row 這個保證，在 rolling deploy 的視窗內會被打破——DB migration 先跑完、
	// 但舊 pod 還在用沒有這段接線的 CreateRequest 服務流量，那段期間建立的
	// request 永遠不會有 conversation row（Task 2 的 backfill migration 只涵蓋
	// migration 執行前就存在的列，救不到 rollout 期間新生的）。純查詢會在這裡
	// 吃到 pgx.ErrNoRows 這個沒被對映的裸錯誤，讓那些 request 之後每一輪都固定
	// 500，且沒有任何自我修復路徑。
	convID := r.newID()
	conv, err := r.q(ctx).GetOrCreateProcurementConversation(ctx, gen.GetOrCreateProcurementConversationParams{
		ID: convID, BuyerID: int64(in.BuyerID), RequestID: out.RequestID,
	})
	if err != nil {
		return Revision{}, err
	}
	out.ConversationID = conv.ID
	return out, nil
}

// RewindTo 從 target 版本開一個新分支——supersede 目前 ACTIVE revision，以 target 的
// slots_json 為內容建新版本：parent_revision_id=target.ID、branch_id=新版本自身 ID
// （比照 root revision「以自身 id 為 branch_id」的既定慣例，開新分支）、
// revision_no=**目前 ACTIVE revision**（即將被 supersede 的那個，不是 target）的
// revision_no+1——沿用 AppendRevision 同一條規則，revision_no 是全域遞增序號，
// uq_procurement_request_revisions_no 對 (request_id, revision_no) 唯一、不分 branch。
//
// 冪等與交易外復原路徑跟 AppendRevision 完全同構：同一套
// uq_procurement_answer_patches_idempotency 唯一索引、同一個 recoverIdempotentAppend。
//
// in.TargetRevisionID 是外部輸入（設計上由 usecase 層的 resolveRewindTarget 依
// slotKey 在「這個 request 自己的」ListBranchRevisions 鏈上解析出來，理論上一定屬於
// in.RequestID），但這裡仍要驗證 targetRow.RequestID == in.RequestID 才能用——
// AppendRevision 的 parent 是用 GetActiveProcurementRevision(txCtx, in.RequestID)
// 取得、天生就被 request_id 限定住，RewindTo 換成用裸 ID 查（GetProcurementRevisionByID
// 沒有 WHERE request_id），若不額外驗證，一旦上層傳入別的 request（甚至別的買家）的
// revision id，這裡就會把別人的 slots 內容原封不動當成這個 request 的新版本內容寫入、
// 並讓 GetSlots 讀出來——等於跨租戶資料外洩。找不到或不屬於這個 request 都回
// ErrNotFound，理由同檔頭 ErrNotFound 的說明：不得讓錯誤型別洩漏「這個 revision id
// 存在，只是不屬於你」。
// ⛔ 交易用 r.withNestableTx 而不是裸的 WithTx，理由與 AppendRevision 完全相同（兩者
// 共用同一把冪等鍵與同一條復原路徑 recoverIdempotentAppend）。哨兵：
// TestRewindTo_ConcurrentLoserInsideCallerTransactionReplays。
func (r *Repo) RewindTo(ctx context.Context, in RewindInput) (Revision, error) {
	var out Revision

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		existing, err := q.FindProcurementAnswerPatchByIdempotencyKey(txCtx, gen.FindProcurementAnswerPatchByIdempotencyKeyParams{
			BuyerID: int64(in.BuyerID), ClientRequestID: in.ClientRequestID,
		})
		if err == nil {
			row, err := q.GetProcurementRevisionByID(txCtx, existing.ResultRevisionID)
			if err != nil {
				return err
			}
			out = toRevisionEntity(row)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if _, err := q.GetProcurementRequestForUpdate(txCtx, gen.GetProcurementRequestForUpdateParams{
			ID: in.RequestID, BuyerID: int64(in.BuyerID),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		targetRow, err := q.GetProcurementRevisionByID(txCtx, in.TargetRevisionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if targetRow.RequestID != in.RequestID {
			return ErrNotFound
		}

		activeRow, err := q.GetActiveProcurementRevision(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		active := toRevisionEntity(activeRow)

		if err := q.SupersedeProcurementRevision(txCtx, active.ID); err != nil {
			return err
		}

		revID := r.newID()
		revRow, err := q.CreateProcurementRevision(txCtx, gen.CreateProcurementRevisionParams{
			ID: revID, RequestID: in.RequestID, RevisionNo: int32(active.RevisionNo + 1),
			ParentRevisionID: pointerInt64(targetRow.ID), BranchID: revID,
			SlotsJson: targetRow.SlotsJson, SlotsHash: targetRow.SlotsHash, Status: "ACTIVE",
		})
		if err != nil {
			return err
		}
		out = toRevisionEntity(revRow)

		payload := in.PayloadJSON
		if len(payload) == 0 {
			payload = []byte(`{}`)
		}
		patchID := r.newID()
		patchRow, err := q.CreateProcurementAnswerPatch(txCtx, gen.CreateProcurementAnswerPatchParams{
			ID: patchID, RequestID: in.RequestID, BuyerID: int64(in.BuyerID),
			BaseRevisionID: pointerInt64(targetRow.ID), ResultRevisionID: out.ID,
			Source: "REWIND", PayloadJson: payload, ClientRequestID: in.ClientRequestID,
		})
		if err != nil {
			return err
		}
		if patchRow.ID != patchID {
			return errLostIdempotencyRace
		}

		if err := q.BumpProcurementRequestRevision(txCtx, gen.BumpProcurementRequestRevisionParams{
			ID: in.RequestID, ActiveRevisionID: pointerInt64(out.ID),
		}); err != nil {
			return err
		}

		if err := q.UpdateProcurementConversationPreview(txCtx, gen.UpdateProcurementConversationPreviewParams{
			RequestID: in.RequestID, LastMessagePreview: previewOf(payload),
		}); err != nil {
			return err
		}

		var slots domain.Slots
		if err := json.Unmarshal(targetRow.SlotsJson, &slots); err != nil {
			return err
		}
		if title := autoTitleOf(slots); title != "" {
			if err := q.UpdateProcurementConversationAutoTitle(txCtx, gen.UpdateProcurementConversationAutoTitleParams{
				RequestID: in.RequestID, Title: title,
			}); err != nil {
				return err
			}
		}

		status := "CLARIFYING"
		if len(slots.MissingRequired()) == 0 {
			status = "REQUIREMENT_READY"
		}
		return q.SetProcurementRequestStatus(txCtx, gen.SetProcurementRequestStatusParams{
			ID: in.RequestID, Status: status,
		})
	})

	if err != nil {
		if !errors.Is(err, errLostIdempotencyRace) {
			return Revision{}, err
		}
		out, err = r.recoverIdempotentAppend(ctx, in.BuyerID, in.ClientRequestID)
		if err != nil {
			return Revision{}, err
		}
	}

	convID := r.newID()
	conv, err := r.q(ctx).GetOrCreateProcurementConversation(ctx, gen.GetOrCreateProcurementConversationParams{
		ID: convID, BuyerID: int64(in.BuyerID), RequestID: out.RequestID,
	})
	if err != nil {
		return Revision{}, err
	}
	out.ConversationID = conv.ID
	return out, nil
}

// IdempotentResult 是一次「已經做過」的命令的完整結果，供命令層級的冪等重播用。
// PayloadJSON 是當初寫進 answer_patch 的原始 payload——它的內部形狀由寫入端
// （usecase）定義，repository 只負責原封不動搬運，不解讀。
type IdempotentResult struct {
	RequestID      int64
	RevisionID     int64
	Slots          domain.Slots
	PayloadJSON    json.RawMessage
	ConversationID int64
}

// FindByIdempotencyKey 依冪等鍵 (buyer_id, client_request_id) 找出既有的
// answer_patch，並一併取回它產生的那個 revision 的 slots。找不到回 ErrNotFound。
//
// 存在的理由：冪等只擋在 AppendRevision（整條 turn 的最後一步）時，前面的
// CreateRequest 已經先跑掉了——客戶端沒收到回應而重送同一個 client_request_id
// 時，會多生一個沒有任何內容的孤兒 request，而回應帶的 revision 卻屬於前一個
// request。呼叫端要能在做任何事之前先問「這個命令是不是已經做過」，才有辦法
// 讓整條命令冪等，而不是只有最後一步冪等。
//
// 兩次查詢刻意不包在交易裡：answer_patch 寫入後即不可變，revision 也是
// append-only（只有 status 會被後續版本改成 SUPERSEDED，slots_json 永不改動），
// 所以「先讀 patch 拿到 result_revision_id，再依那個確切 ID 讀 revision」不可能
// 讀到不自洽的組合——與 GetSnapshot 靠 ActiveRevisionID 直接取的理由同一個。
func (r *Repo) FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (IdempotentResult, error) {
	q := r.q(ctx)

	patch, err := q.FindProcurementAnswerPatchByIdempotencyKey(ctx, gen.FindProcurementAnswerPatchByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IdempotentResult{}, ErrNotFound
		}
		return IdempotentResult{}, err
	}

	revRow, err := q.GetProcurementRevisionByID(ctx, patch.ResultRevisionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IdempotentResult{}, ErrNotFound
		}
		return IdempotentResult{}, err
	}

	var slots domain.Slots
	if err := json.Unmarshal(revRow.SlotsJson, &slots); err != nil {
		return IdempotentResult{}, err
	}

	// 與 AppendRevision 尾端同一支 get-or-create（理由見該處註解）：兩個 call site
	// 用同一種自我修復方式，不對「conversation row 不存在」這件事做特例分辨。
	// 原本這裡把缺列對映成 ErrNotFound，會讓 rolling deploy 視窗內建立的 request
	// 的合法重送被誤判成「這個命令沒做過」而重跑一次整條 turn；INSERT ... ON
	// CONFLICT 不會像純 SELECT 那樣回 pgx.ErrNoRows，所以那個分支一併移除。
	convID := r.newID()
	conv, err := q.GetOrCreateProcurementConversation(ctx, gen.GetOrCreateProcurementConversationParams{
		ID: convID, BuyerID: int64(buyerID), RequestID: patch.RequestID,
	})
	if err != nil {
		return IdempotentResult{}, err
	}

	return IdempotentResult{
		RequestID: patch.RequestID, RevisionID: revRow.ID, Slots: slots,
		PayloadJSON: patch.PayloadJson, ConversationID: conv.ID,
	}, nil
}

// recoverIdempotentAppend 在 AppendRevision／RewindTo 因併發撞上 client_request_id
// 冪等鍵而回捲之後，重新讀出贏家寫入的 answer_patch，回傳它產生的 revision——跟循序
// 路徑命中冪等時回傳的東西一致。
//
// ⛔ 它跑在 ctx 上，也就是**呼叫端的**交易（如果有的話），不是「一定在交易外」。這樣
// 仍然成立，是因為撞鍵不再拋錯（answer patch 的 INSERT 用 ON CONFLICT DO UPDATE，見
// PostgreSQL 規範第十二章）：交易被 WithTx 主動回捲，不是被 25P02 打成 aborted。
func (r *Repo) recoverIdempotentAppend(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (Revision, error) {
	q := r.q(ctx)
	existing, err := q.FindProcurementAnswerPatchByIdempotencyKey(ctx, gen.FindProcurementAnswerPatchByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		return Revision{}, err
	}
	row, err := q.GetProcurementRevisionByID(ctx, existing.ResultRevisionID)
	if err != nil {
		return Revision{}, err
	}
	return toRevisionEntity(row), nil
}

// GetSnapshot 取 request 與它目前指向的 revision。買家不符一律回 ErrNotFound。
//
// 依 req.ActiveRevisionID 直接取那一個 revision（而非另外用 status='ACTIVE'
// 查詢）：revision 是 append-only、永不刪除，所以「先讀 request 拿到某個
// ActiveRevisionID，再依那個確切 ID 去讀 revision」這兩步之間即使有別的
// AppendRevision 插隊 commit，也不會讓回傳的組合失去自洽——回傳的 rev 永遠是
// req 讀取當下真正指向的那一個，req.ActiveRevisionID == rev.ID 這個不變式必成立。
// 用 status='ACTIVE' 查詢則沒有這個保證：兩次查詢之間若有新版本插入，會拿到
// 「req 指向舊版，但 rev 已經是新版」這種不自洽組合。
func (r *Repo) GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (Request, Revision, error) {
	q := r.q(ctx)

	reqRow, err := q.GetProcurementRequestSnapshot(ctx, gen.GetProcurementRequestSnapshotParams{
		ID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Request{}, Revision{}, ErrNotFound
		}
		return Request{}, Revision{}, err
	}
	req := toRequestEntity(reqRow)

	if req.ActiveRevisionID == 0 {
		return Request{}, Revision{}, ErrNotFound
	}

	revRow, err := q.GetProcurementRevisionByID(ctx, req.ActiveRevisionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Request{}, Revision{}, ErrNotFound
		}
		return Request{}, Revision{}, err
	}
	// ConversationID 刻意不在這裡補（review round 2 Minor #3，撤銷 round 1 的
	// 作法）：GetSnapshot 是 SnapshotReader 這個共用讀取介面唯一的方法，11 個
	// 呼叫點裡只有 internal/grpc 的 GetSnapshot RPC handler 真的需要
	// conversationId（組 runs[]／activeRunId／openConfirmation 用）——
	// selection.go／rewind.go／rfq_confirmation.go／price_preview.go／turn.go／
	// crop_disambiguation.go／answer.go 這些呼叫點（含經由 GetSlots 間接呼叫的
	// 路徑）完全用不到
	// 這個值，卻會因為疊在這支共用 primitive 上而無條件多付一次
	// procurement_conversations 查詢，還多背一個跟 slots／crop resolution 完全
	// 無關的新失敗模式。改由唯一需要的呼叫端（internal/grpc/
	// procurement_handler.go GetSnapshot）自己在拿到 requestID 後直接呼叫
	// conversationIDForRequest，把成本收斂到真正的消費端。
	return req, toRevisionEntity(revRow), nil
}

// GetSlots 是 GetSnapshot 的輕量版：只回傳目前 revision 的 slots，給只關心需求
// 內容（不需要 Request/Revision metadata）的呼叫端用。
func (r *Repo) GetSlots(ctx context.Context, requestID int64, buyerID identity.BuyerID) (domain.Slots, error) {
	_, rev, err := r.GetSnapshot(ctx, requestID, buyerID)
	if err != nil {
		return domain.Slots{}, err
	}
	var slots domain.Slots
	if err := json.Unmarshal(rev.SlotsJSON, &slots); err != nil {
		return domain.Slots{}, err
	}
	return slots, nil
}

// ListBranchRevisions 回傳目前 active branch 從根到 tip 的完整版本鏈——依實際
// parent_revision_id 鏈結走訪，不是依 branch_id 分組：rewind 產生的新分支其
// parent_revision_id 會指回舊分支上的某個祖先版本，branch_id 只是稽核用的分類
// 標籤，不是可靠的鏈結來源；唯一可靠的鏈結是 parent_revision_id。
//
// 用途：/rewinds 用它找某個 slotKey 最後一次被寫入之前的版本；pickNextSlot 用它
// 衍生「哪些選填欄位已被跳過」與「連續問了幾輪選填問題」（見 usecase 層的
// deriveQuestionHistory，Task 3）。
func (r *Repo) ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]RevisionWithPatch, error) {
	req, _, err := r.GetSnapshot(ctx, requestID, buyerID)
	if err != nil {
		return nil, err
	}

	q := r.q(ctx)
	revRows, err := q.ListProcurementRevisionsByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	patchRows, err := q.ListProcurementAnswerPatchesByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	patchByResultRevisionID := make(map[int64]gen.ProcurementAnswerPatch, len(patchRows))
	for _, p := range patchRows {
		patchByResultRevisionID[p.ResultRevisionID] = p
	}
	revByID := make(map[int64]Revision, len(revRows))
	for _, row := range revRows {
		rev := toRevisionEntity(row)
		revByID[rev.ID] = rev
	}

	var chain []RevisionWithPatch
	for id := req.ActiveRevisionID; id != 0; {
		rev, ok := revByID[id]
		if !ok {
			return nil, fmt.Errorf("procurement: revision %d missing from chain for request %d", id, requestID)
		}
		patch := patchByResultRevisionID[id]
		chain = append(chain, RevisionWithPatch{
			Revision: rev, PatchSource: patch.Source, PatchPayloadJSON: patch.PayloadJson,
		})
		id = rev.ParentRevisionID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// SetRequestStatus 更新 request 的狀態欄位。呼叫端負責判斷該轉去哪個狀態
// （純函式邏輯留在 usecase／domain，這裡只做寫入），因此不做狀態機合法性檢查。
func (r *Repo) SetRequestStatus(ctx context.Context, requestID int64, status string) error {
	return r.q(ctx).SetProcurementRequestStatus(ctx, gen.SetProcurementRequestStatusParams{
		ID: requestID, Status: status,
	})
}

func nowUTC() time.Time { return time.Now().UTC() }

// previewOf 從 turnPayload 的 JSON 取出 reply 當列表項預覽，截斷到合理長度——
// 不是給使用者看的完整回覆，只是列表一行摘要。60 字是暫定值，Task 落地後如果
// 跟實際 UI 寬度對不上，改這一個常數即可。
const conversationPreviewMaxRunes = 60

func previewOf(payloadJSON []byte) string {
	var p struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return ""
	}
	r := []rune(p.Reply)
	if len(r) > conversationPreviewMaxRunes {
		return string(r[:conversationPreviewMaxRunes])
	}
	return p.Reply
}

// autoTitleOf 用 {item}｜{delivery_city} 組自動標題，任一值缺席回空字串（呼叫端
// 看到空字串就不寫回去，維持現狀）。40 runes 截斷。
func autoTitleOf(s domain.Slots) string {
	if s.Item == nil || s.DeliveryCity == nil {
		return ""
	}
	title := *s.Item + "｜" + *s.DeliveryCity
	r := []rune(title)
	if len(r) > 40 {
		return string(r[:40])
	}
	return title
}
