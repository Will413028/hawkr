// conversation.go 是「Hawkr 幫找」對話容器（列表／transcript／改名／軟刪）的持久層。
// 建立與逐輪更新（title/last_message_preview）接在 revision.go 既有的
// CreateRequest／AppendRevision 交易內，見該檔案；本檔只放這四個端點自己專屬的
// 讀寫方法，不重複那兩個交易本身。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ConversationCursor 是列表分頁的複合游標。列表依 (updated_at, id) DESC 排序，
// 單靠 id 無法表達「排在這一列之後」——updated_at 撞值時會漏列或重複列，所以
// 兩個欄位都要帶。
type ConversationCursor struct {
	UpdatedAt time.Time
	ID        int64
}

// ListConversations 回傳比 limit 多一筆（呼叫端傳 limit+1）的原始列表，hasMore
// 與 trim 掉多抓那筆的邏輯留給呼叫端（usecase 層）——repository 只負責如實回傳
// query 結果，不在這裡藏一層「猜呼叫端要不要 trim」的邏輯。
// cursor=nil 代表第一頁。
func (r *Repo) ListConversations(ctx context.Context, buyerID identity.BuyerID, cursor *ConversationCursor, limitPlusOne int) ([]Conversation, error) {
	params := gen.ListProcurementConversationsParams{
		BuyerID: int64(buyerID), LimitPlusOne: int32(limitPlusOne),
	}
	if cursor != nil {
		params.HasCursor = true
		params.CursorUpdatedAt = timestamptzParam(cursor.UpdatedAt)
		params.CursorID = cursor.ID
	}
	rows, err := r.q(ctx).ListProcurementConversations(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Conversation, len(rows))
	for i, row := range rows {
		out[i] = toConversationEntity(row)
	}
	return out, nil
}

// GetConversationForMessages 是 GET .../messages 的存在性檢查——0 列回 ErrNotFound。
// 回傳的 status 來自 JOIN 進來的 procurement_requests.status，跟 conversation 本身
// 的欄位一次查詢拿齊，不是事後另一次查詢拼出來的（見 GetSnapshot 的既有教訓）。
func (r *Repo) GetConversationForMessages(ctx context.Context, conversationID int64, buyerID identity.BuyerID) (Conversation, string, error) {
	row, err := r.q(ctx).GetProcurementConversationForMessages(ctx, gen.GetProcurementConversationForMessagesParams{
		ID: conversationID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Conversation{}, "", ErrNotFound
		}
		return Conversation{}, "", err
	}
	return Conversation{ID: row.ID, RequestID: row.RequestID, Title: row.Title}, row.Status, nil
}

// ListAnswerPatchesByRequestID 依時間序回傳一個 request 的全部 answer_patch，
// 供 GET .../messages 展成 transcript。不分頁——單一採購需求的輪數天然有限。
func (r *Repo) ListAnswerPatchesByRequestID(ctx context.Context, requestID int64) ([]gen.ProcurementAnswerPatch, error) {
	return r.q(ctx).ListProcurementAnswerPatchesByRequestID(ctx, requestID)
}

// ConversationCommandResult 是冪等預檢的結果。Hit=false 時其餘欄位無意義。
//
// ConversationID 是當初那次命令實際作用的對話——冪等鍵只有 (buyer_id,
// client_request_id)，不含 conversation_id，所以命中不代表「這次這個
// conversation 的這個操作做過」。呼叫端必須拿它跟本次的目標比對，否則同一把
// clientRequestId 被重用在別的 conversation 上時，會回報成功卻什麼都沒做。
type ConversationCommandResult struct {
	Hit            bool
	ConversationID int64
	ResultJSON     json.RawMessage
}

func (r *Repo) findConversationCommand(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (ConversationCommandResult, error) {
	row, err := r.q(ctx).FindProcurementConversationCommandByIdempotencyKey(ctx, gen.FindProcurementConversationCommandByIdempotencyKeyParams{
		BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConversationCommandResult{}, nil
		}
		return ConversationCommandResult{}, err
	}
	return ConversationCommandResult{Hit: true, ConversationID: row.ConversationID, ResultJSON: row.ResultJson}, nil
}

func (r *Repo) recordConversationCommand(ctx context.Context, id int64, buyerID identity.BuyerID, clientRequestID string, conversationID int64, resultJSON json.RawMessage) error {
	_, err := r.q(ctx).CreateProcurementConversationCommand(ctx, gen.CreateProcurementConversationCommandParams{
		ID: id, BuyerID: int64(buyerID), ClientRequestID: clientRequestID,
		ConversationID: conversationID, ResultJson: resultJSON,
	})
	return err
}

// RenameRecord 是 PATCH 冪等重播／首次成功都要組出的形狀——usecase 層直接用這個
// 反序列化 result_json，不用另外定義一個 usecase 專屬的鏡射型別。
type RenameRecord struct {
	Op    string `json:"op"`
	Title string `json:"title"`
}

// DeleteRecord 是 DELETE 寫進 result_json 的形狀。只有一個 op 欄位，但仍用具名
// struct 而非裸 map[string]string——寫入端與重播檢查端要用同一個型別，才不會
// 一邊寫 "op" 另一邊讀別的 key 卻沒人發現（比照 RenameRecord）。
type DeleteRecord struct {
	Op string `json:"op"`
}

// RenameConversation 冪等改名。found=false 代表不存在／非本人／已刪除（呼叫端回
// 404）；found=true 時 hit 分辨這次是冪等重播還是真的執行了改名。
func (r *Repo) RenameConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID, title, clientRequestID string) (found bool, hit bool, record RenameRecord, err error) {
	existing, err := r.findConversationCommand(ctx, buyerID, clientRequestID)
	if err != nil {
		return false, false, RenameRecord{}, err
	}
	if existing.Hit {
		if existing.ConversationID != conversationID {
			// 這個 clientRequestId 曾經用在別的 conversation 上——不是這次呼叫的合法重播。
			// 視為「這個 conversation 底下沒有這個 clientRequestId 用過」，回 404，而不是
			// 誤判成這次改名成功過。不嘗試真的去寫，因為 unique index 一定會擋。
			return false, false, RenameRecord{}, nil
		}
		var rec RenameRecord
		if err := json.Unmarshal(existing.ResultJSON, &rec); err != nil {
			return false, false, RenameRecord{}, err
		}
		if rec.Op != "rename" {
			// 同一把 key 曾經用在不同種操作上（例如先 delete 後又拿同一把 key 打 rename）。
			// 同樣視為對「這次操作」而言不存在，不冒充成功。
			return false, false, RenameRecord{}, nil
		}
		return true, true, rec, nil
	}

	affected, err := r.q(ctx).RenameProcurementConversation(ctx, gen.RenameProcurementConversationParams{
		ID: conversationID, BuyerID: int64(buyerID), Title: title,
	})
	if err != nil {
		return false, false, RenameRecord{}, err
	}
	if affected == 0 {
		return false, false, RenameRecord{}, nil
	}

	record = RenameRecord{Op: "rename", Title: title}
	resultJSON, err := json.Marshal(record)
	if err != nil {
		return false, false, RenameRecord{}, err
	}
	if err := r.recordConversationCommand(ctx, r.newID(), buyerID, clientRequestID, conversationID, resultJSON); err != nil {
		return false, false, RenameRecord{}, err
	}
	return true, false, record, nil
}

// DeleteConversation 冪等軟刪。found=false 代表不存在／非本人（404）；
// alreadyDeleted=true 代表這次呼叫前就已經是刪除狀態（200 no-op，不是 404——
// 判準是「終態已達成」，見 spec §3 的 PATCH/DELETE 對照說明）。
func (r *Repo) DeleteConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID, clientRequestID string) (found bool, alreadyDeleted bool, err error) {
	existing, err := r.findConversationCommand(ctx, buyerID, clientRequestID)
	if err != nil {
		return false, false, err
	}
	if existing.Hit {
		if existing.ConversationID != conversationID {
			// 同 RenameConversation 的理由：這把 key 是別的 conversation 用過的，
			// 對這次呼叫而言不算重播——不能回成功卻一列都沒刪。
			return false, false, nil
		}
		var rec DeleteRecord
		if err := json.Unmarshal(existing.ResultJSON, &rec); err != nil {
			return false, false, err
		}
		if rec.Op != "delete" {
			return false, false, nil
		}
		return true, false, nil // 冪等重播：這次呼叫本身一定是成功過的，直接視為 found
	}

	deletedAt, err := r.q(ctx).GetProcurementConversationForDelete(ctx, gen.GetProcurementConversationForDeleteParams{
		ID: conversationID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	if deletedAt != nil {
		return true, true, nil
	}

	if err := r.q(ctx).SoftDeleteProcurementConversation(ctx, gen.SoftDeleteProcurementConversationParams{
		ID: conversationID, BuyerID: int64(buyerID),
	}); err != nil {
		return false, false, err
	}

	resultJSON, err := json.Marshal(DeleteRecord{Op: "delete"})
	if err != nil {
		return false, false, err
	}
	if err := r.recordConversationCommand(ctx, r.newID(), buyerID, clientRequestID, conversationID, resultJSON); err != nil {
		return false, false, err
	}
	return true, false, nil
}
