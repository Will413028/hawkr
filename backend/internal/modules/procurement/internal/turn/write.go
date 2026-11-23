package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// turnWrite 是一輪要寫進資料庫的內容：run 驅動的提交（Executor.commit）與不經 run 的提交（SubmitAnswer、Rewind）共用。
type turnWrite struct {
	ids ids
	// buyerID 是需求的擁有者；寫入都經擁有的需求，不屬於它時回錯。
	buyerID int64
	// parent 是新版本的內容祖先（A9-D33(3)）；沒有現行版本的新需求為 nil。
	parent          *uuid.UUID
	d               decision
	source, message string
	clientRequestID uuid.UUID
	// fromStatus 是這一輪開始前讀到的需求狀態，轉移用條件式 UPDATE。
	fromStatus string
	// result 是 TurnResult 的 wire JSON，原樣存進 turns.turn_result。
	result []byte
	now    time.Time
}

// writeTurn 在呼叫端的 transaction 內寫：取代現行版本、新版本、輪次（含 turn_result）、需求狀態、對話預覽與自動標題。呼叫端已鎖好需求相關的列。
func writeTurn(ctx context.Context, q *store.Queries, w turnWrite) error {
	id, d, buyer := w.ids, w.d, w.buyerID
	streak := d.plan.OptionalStreak
	if streak < 0 || streak > math.MaxInt32 {
		return fmt.Errorf("turn：連續選填計數 %d 超出範圍", streak)
	}
	recorded, err := json.Marshal(nonNil(d.recorded))
	if err != nil {
		return fmt.Errorf("turn：組 recorded_this_turn：%w", err)
	}
	if _, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: id.request, BuyerID: &buyer}); err != nil {
		return fmt.Errorf("turn：取代現行版本：%w", err)
	}
	no, err := q.NextRevisionNo(ctx, store.NextRevisionNoParams{RequestID: id.request, BuyerID: &buyer})
	if err != nil {
		return fmt.Errorf("turn：版本序號：%w", err)
	}
	n, err := q.InsertRevision(ctx, revisionParams(d.next, store.InsertRevisionParams{
		ID: id.revision, RevisionNo: no, ParentRevisionID: w.parent, Now: w.now, RequestID: id.request, BuyerID: &buyer,
	}))
	if err != nil {
		return fmt.Errorf("turn：寫入版本：%w", err)
	}
	if n == 0 {
		return fmt.Errorf("turn：需求 %s 不屬於這位買家", id.request)
	}
	skipped := make([]string, 0, len(d.plan.Skipped))
	for _, k := range d.plan.Skipped {
		skipped = append(skipped, string(k))
	}
	n, err = q.InsertTurn(ctx, store.InsertTurnParams{
		ID: id.turn, RequestID: id.request, ConversationID: id.conversation, BuyerID: buyer, ResultRevisionID: id.revision,
		Source: w.source, BuyerMessage: w.message, AssistantReply: d.replyText, Steps: []byte("[]"), RecordedThisTurn: recorded,
		OptionalStreak: int32(streak), SkippedSlots: skipped, TurnResult: w.result, ClientRequestID: w.clientRequestID, Now: w.now,
	})
	if err != nil {
		return fmt.Errorf("turn：寫入輪次：%w", err)
	}
	if n == 0 {
		return fmt.Errorf("turn：client_request_id %s 已有輪次", w.clientRequestID)
	}
	if d.status != w.fromStatus {
		n, err := q.SetRequestStatus(ctx, store.SetRequestStatusParams{Status: d.status, Now: w.now, ID: id.request, BuyerID: &buyer, FromStatus: w.fromStatus})
		if err != nil {
			return fmt.Errorf("turn：更新需求狀態：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("turn：需求 %s 的狀態已不是 %s", id.request, w.fromStatus)
		}
	}
	if err := q.SetConversationPreview(ctx, store.SetConversationPreviewParams{Preview: bounded(d.replyText, maxPreviewRunes), Now: w.now, ID: id.conversation, BuyerID: buyer}); err != nil {
		return fmt.Errorf("turn：更新對話預覽：%w", err)
	}
	if title := autoTitle(d.next); title != "" {
		if err := q.SetConversationAutoTitle(ctx, store.SetConversationAutoTitleParams{Title: title, Now: w.now, ID: id.conversation, BuyerID: buyer}); err != nil {
			return fmt.Errorf("turn：更新自動標題：%w", err)
		}
	}
	return nil
}
