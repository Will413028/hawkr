// Package turn 是 TURN 類型 run 的 executor（A9-D24(2)）：一次模型呼叫抽出欄位，其餘是純函式（slots、resolver、planner、reply），
// 結果在單一 transaction 內寫成需求版本、輪次、需求狀態、對話標題與預覽，並與 turn_result 事件和 run 的終態轉換一起提交。
// 模型呼叫與分類樹讀取都在 transaction 外（單體規則 5）。
package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/extract"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// turns.source 的值（與 schema 的 CHECK 相同）。
const (
	sourceLLMExtract = "LLM_EXTRACT"
	sourceAnswer     = "ANSWER"
)

// SlotExtractor 把買家的訊息抽成 patch（*extract.Extractor 滿足它）。
type SlotExtractor interface {
	Extract(ctx context.Context, message string, known slots.Set) (slots.Patch, error)
}

// Executor 實作 run.Executor。
type Executor struct {
	DB        *db.DB
	Ledger    run.Ledger
	Extractor SlotExtractor
	Resolver  Resolver
	Now       func() time.Time
	Logger    *slog.Logger
}

var _ run.Executor = Executor{}

// ids 是這一輪寫入的實體 ID，turn_result 事件與中斷都要引用。
type ids struct {
	request, conversation, revision, turn, run uuid.UUID
}

type loaded struct {
	run     store.ProcurementAgentRun
	buyerID *int64
	message string
	// optionID 是回答中斷的 run 選的選項（message 與它恰一個非空）；TURN 沒有。
	optionID string
	// source 是這一輪寫進 turns.source 的來源；空字串是 LLM_EXTRACT。
	source         string
	locale         uitext.Locale
	conversationID uuid.UUID
	prior          priorState
	active         *store.ProcurementRequestRevision
	request        store.ProcurementRequest
}

// Execute 執行一個 TURN run。回傳 nil 之前 run 已 SUCCEEDED 或 WAITING_FOR_INPUT；任何錯誤由 Dispatcher 轉 FAILED 並退額度。
func (e Executor) Execute(ctx context.Context, r store.ProcurementAgentRun) error {
	ld, err := e.load(ctx, r)
	if err != nil {
		return err
	}
	patch, err := e.Extractor.Extract(ctx, ld.message, ld.prior.current)
	if err != nil {
		return classifyExtractErr(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := e.Now()
	d, err := decide(ctx, e.Resolver, ld.locale, now, ld.prior, patch)
	if err != nil {
		return err
	}
	return e.commit(ctx, ld, d, now)
}

// classifyExtractErr 把抽取失敗分類成 run 的失敗原因：服務不可用是 MODEL_UNAVAILABLE，其餘是 pipeline 失敗；ctx 結束的錯誤原樣回傳。
func classifyExtractErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var ee *extract.Error
	if errors.As(err, &ee) && ee.Unavailable {
		return &run.FailureError{Reason: run.FailureModelUnavailable, Err: err}
	}
	return &run.FailureError{Reason: run.FailureTurnPipelineErr, Err: err}
}

func (e Executor) load(ctx context.Context, r store.ProcurementAgentRun) (loaded, error) {
	if r.ConversationID == nil {
		return loaded{}, fmt.Errorf("turn：run %s 沒有 conversation_id", r.ID)
	}
	buyer := r.BuyerID
	q := store.New(e.DB.Conn(ctx))
	ld := loaded{run: r, buyerID: &buyer, conversationID: *r.ConversationID}

	in, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: r.ID, BuyerID: r.BuyerID})
	if err != nil {
		return loaded{}, fmt.Errorf("turn：讀取輸入：%w", err)
	}
	if ld.locale, err = run.LocaleOf(in.Locale); err != nil {
		return loaded{}, err
	}
	ld.message = in.BuyerMessage
	if in.OptionID != nil {
		ld.optionID = *in.OptionID
	}

	if ld.request, err = q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: r.RequestID, BuyerID: ld.buyerID}); err != nil {
		return loaded{}, fmt.Errorf("turn：讀取需求：%w", err)
	}
	if _, err := q.GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: ld.conversationID, BuyerID: r.BuyerID}); err != nil {
		return loaded{}, fmt.Errorf("turn：讀取對話：%w", err)
	}
	ld.prior.requestOf = ld.request.Status

	rev, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: r.RequestID, BuyerID: ld.buyerID})
	switch {
	case err == nil:
		ld.active = &rev
		ld.prior.current = setFromRevision(rev)
	case !errors.Is(err, pgx.ErrNoRows):
		return loaded{}, fmt.Errorf("turn：讀取現行版本：%w", err)
	}

	prev, err := q.GetLatestTurnByRequest(ctx, store.GetLatestTurnByRequestParams{RequestID: r.RequestID, BuyerID: r.BuyerID})
	switch {
	case err == nil:
		pt := &prevTurn{optionalStreak: int(prev.OptionalStreak)}
		for _, k := range prev.SkippedSlots {
			pt.skipped = append(pt.skipped, model.SlotKey(k))
		}
		ld.prior.prevTurn = pt
	case !errors.Is(err, pgx.ErrNoRows):
		return loaded{}, fmt.Errorf("turn：讀取上一輪：%w", err)
	}

	if ld.active != nil {
		_, err := q.GetLatestPricePreviewForRevision(ctx, store.GetLatestPricePreviewForRevisionParams{
			RequestID: r.RequestID, RequestRevisionID: ld.active.ID, BuyerID: ld.buyerID, Now: e.Now(),
		})
		switch {
		case err == nil:
			ld.prior.preview = true
		case !errors.Is(err, pgx.ErrNoRows):
			return loaded{}, fmt.Errorf("turn：讀取價格預覽：%w", err)
		}
		snap, err := q.GetLatestConfirmedSnapshotByRequest(ctx, store.GetLatestConfirmedSnapshotByRequestParams{RequestID: r.RequestID, BuyerID: ld.buyerID})
		switch {
		case err == nil:
			ld.prior.snapshot = &snapshotState{onCurrentRevision: snap.RequestRevisionID == ld.active.ID}
		case !errors.Is(err, pgx.ErrNoRows):
			return loaded{}, fmt.Errorf("turn：讀取確認快照：%w", err)
		}
	}
	return ld, nil
}

// commit 在單一 transaction 內寫完這一輪：先鎖自己的 run（之後的轉換不會被取消搶在中間），再寫版本、輪次、狀態、標題與預覽，
// 最後寫 turn_result 並轉 SUCCEEDED，或寫中斷。任何一步失敗整個回滾，run 由 Dispatcher 依回傳的錯誤收斂。
func (e Executor) commit(ctx context.Context, ld loaded, d decision, now time.Time) error {
	id := ids{request: ld.run.RequestID, conversation: ld.conversationID, revision: uuid.NewV7(), turn: uuid.NewV7(), run: ld.run.ID}
	var interruptID uuid.UUID
	if d.interrupt != nil {
		interruptID = uuid.NewV7()
	}
	event, err := turnResultEvent(d, id, now, interruptID)
	if err != nil {
		return err
	}
	var payload []byte
	if d.interrupt != nil {
		if payload, err = json.Marshal(d.interrupt); err != nil {
			return fmt.Errorf("turn：組中斷 payload：%w", err)
		}
	}
	streak := d.plan.OptionalStreak
	if streak < 0 || streak > math.MaxInt32 {
		return fmt.Errorf("turn：連續選填計數 %d 超出範圍", streak)
	}
	streak32 := int32(streak)
	recorded, err := json.Marshal(nonNil(d.recorded))
	if err != nil {
		return fmt.Errorf("turn：組 recorded_this_turn：%w", err)
	}

	source := ld.source
	if source == "" {
		source = sourceLLMExtract
	}
	return e.DB.InTx(ctx, func(ctx context.Context) error {
		// 回答 run 的鎖順序是提問的 run、中斷、回答的 run（ledger 的 lockAskingOf）；之後寫轉換時的 lockAskingOf 才不會反過來等。
		if err := e.Ledger.LockAskingOf(ctx, ld.run.ID); err != nil {
			return err
		}
		q := store.New(e.DB.Conn(ctx))
		cur, err := q.LockRun(ctx, ld.run.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("turn：run %s 已不存在", ld.run.ID)
		}
		if err != nil {
			return fmt.Errorf("turn：鎖 run：%w", err)
		}
		switch {
		case run.IsTerminal(cur.Status):
			return run.ErrRunTerminal
		case cur.Status != run.StatusRunning:
			return &run.InvalidTransitionError{From: cur.Status, To: run.StatusSucceeded}
		}

		var parent *uuid.UUID
		if ld.active != nil {
			parent = &ld.active.ID
		}
		if _, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: id.request, BuyerID: ld.buyerID}); err != nil {
			return fmt.Errorf("turn：取代現行版本：%w", err)
		}
		no, err := q.NextRevisionNo(ctx, store.NextRevisionNoParams{RequestID: id.request, BuyerID: ld.buyerID})
		if err != nil {
			return fmt.Errorf("turn：版本序號：%w", err)
		}
		n, err := q.InsertRevision(ctx, revisionParams(d.next, store.InsertRevisionParams{
			ID: id.revision, RevisionNo: no, ParentRevisionID: parent, Now: now, RequestID: id.request, BuyerID: ld.buyerID,
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
			ID: id.turn, RequestID: id.request, ConversationID: id.conversation, BuyerID: ld.run.BuyerID, ResultRevisionID: id.revision,
			Source: source, BuyerMessage: ld.message, AssistantReply: d.replyText, Steps: []byte("[]"), RecordedThisTurn: recorded,
			OptionalStreak: streak32, SkippedSlots: skipped, ClientRequestID: ld.run.ClientRequestID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("turn：寫入輪次：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("turn：client_request_id %s 已有輪次", ld.run.ClientRequestID)
		}
		if d.status != ld.request.Status {
			n, err := q.SetRequestStatus(ctx, store.SetRequestStatusParams{Status: d.status, Now: now, ID: id.request, BuyerID: ld.buyerID, FromStatus: ld.request.Status})
			if err != nil {
				return fmt.Errorf("turn：更新需求狀態：%w", err)
			}
			if n == 0 {
				return fmt.Errorf("turn：需求 %s 的狀態已不是 %s", id.request, ld.request.Status)
			}
		}
		if err := q.SetConversationPreview(ctx, store.SetConversationPreviewParams{Preview: bounded(d.replyText, maxPreviewRunes), Now: now, ID: id.conversation, BuyerID: ld.run.BuyerID}); err != nil {
			return fmt.Errorf("turn：更新對話預覽：%w", err)
		}
		if title := autoTitle(d.next); title != "" {
			if err := q.SetConversationAutoTitle(ctx, store.SetConversationAutoTitleParams{Title: title, Now: now, ID: id.conversation, BuyerID: ld.run.BuyerID}); err != nil {
				return fmt.Errorf("turn：更新自動標題：%w", err)
			}
		}

		if d.interrupt != nil {
			_, err := e.Ledger.Interrupt(ctx, run.InterruptInput{RunID: ld.run.ID, ID: interruptID, Payload: payload, TurnResultEvent: event})
			return err
		}
		if _, err := e.Ledger.Append(ctx, ld.run.ID, run.EventTurnResult, event); err != nil {
			return err
		}
		_, err = e.Ledger.Transition(ctx, ld.run.ID, run.StatusSucceeded, "")
		return err
	})
}

func nonNil(in []recordedJSON) []recordedJSON {
	if in == nil {
		return []recordedJSON{}
	}
	return in
}
