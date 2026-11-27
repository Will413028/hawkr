package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

type turnWrite struct {
	ids             ids
	buyerID         int64
	parent          *uuid.UUID
	d               decision
	source, message string
	clientRequestID uuid.UUID
	fromStatus      string
	result          []byte
	commandSlot     model.SlotKey
	now             time.Time
}

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
	var commandSlot *string
	if w.commandSlot != "" {
		cs := string(w.commandSlot)
		commandSlot = &cs
	}
	n, err = q.InsertTurn(ctx, store.InsertTurnParams{
		ID: id.turn, RequestID: id.request, ConversationID: id.conversation, BuyerID: buyer, ResultRevisionID: id.revision,
		Source: w.source, BuyerMessage: w.message, AssistantReply: d.replyText, Steps: []byte("[]"), RecordedThisTurn: recorded,
		OptionalStreak: int32(streak), SkippedSlots: skipped, TurnResult: w.result, CommandSlot: commandSlot, ClientRequestID: w.clientRequestID, Now: w.now,
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
