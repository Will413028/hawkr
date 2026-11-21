package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ResumeInput 是回答一個中斷的輸入。
type ResumeInput struct {
	BuyerID         int64
	InterruptID     uuid.UUID
	ClientRequestID uuid.UUID
	Message         string
}

// Resume 回答買家自己的 PENDING 中斷，回傳回答 run 的 ID。同一個 transaction：鎖提問的 run、建回答 run（CLARIFICATION_RESUME，interrupt_id 指向被回答的中斷，不扣額度）、
// 寫它的輸入與 run_accepted、排入 job。送出時不解決中斷也不動提問的 run：中斷與提問 run 在回答 run 成功（或寫入新中斷）的提交點才轉（Ledger.Transition）。
// 以 (buyer, client_request_id) 冪等，重送回同一個回答 run（不論它現在的狀態），不受買家上限擋；不是這位買家的中斷回 NOT_FOUND；
// 中斷已解決回 FAILED_PRECONDITION／APPROVAL_ALREADY_RESOLVED；中斷已被取消或提問的 run 不在等待輸入回 FAILED_PRECONDITION／RUN_NOT_ACTIVE；
// 同一個需求已有進行中的 run（包含另一個正在回答的 run）回 FAILED_PRECONDITION／PROCUREMENT_RUN_IN_PROGRESS。回答失敗或取消後中斷仍是 PENDING，買家用新的 client_request_id 再答。
func (s Submitter) Resume(ctx context.Context, in ResumeInput) (uuid.UUID, error) {
	var answerID uuid.UUID
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		if err := q.LockBuyerSubmits(ctx, in.BuyerID); err != nil {
			return fmt.Errorf("run：鎖買家的送出：%w", err)
		}
		existing, err := q.GetRunByIdempotencyKey(ctx, store.GetRunByIdempotencyKeyParams{BuyerID: in.BuyerID, ClientRequestID: in.ClientRequestID})
		if err == nil {
			answerID, err = replayResume(ctx, q, existing, in)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("run：讀取重送的 run：%w", err)
		}
		irq, err := q.GetInterruptForBuyer(ctx, store.GetInterruptForBuyerParams{ID: in.InterruptID, BuyerID: in.BuyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這個中斷")
		}
		if err != nil {
			return fmt.Errorf("run：讀取中斷：%w", err)
		}
		asking, err := q.LockRun(ctx, irq.RunID)
		if err != nil {
			return fmt.Errorf("run：鎖提問的 run：%w", err)
		}
		irq, err = q.LockInterrupt(ctx, irq.ID)
		if err != nil {
			return fmt.Errorf("run：鎖中斷：%w", err)
		}
		if irq.Status == "RESOLVED" {
			return apperr.FailedPrecondition("這個中斷已經回答過").WithReason(apperr.ReasonApprovalAlreadyResolved)
		}
		if irq.Status != "PENDING" || asking.Status != StatusWaitingForInput {
			return apperr.FailedPrecondition("這個 run 不在等待輸入").WithReason(apperr.ReasonRunNotActive)
		}
		now := s.Ledger.Now()
		if err := s.checkLimits(ctx, q, in.BuyerID, &asking.RequestID, now); err != nil {
			return err
		}
		answerID = uuid.NewV7()
		if _, err := s.insertRun(ctx, q, store.InsertRunParams{
			ID: answerID, BuyerID: in.BuyerID, RequestID: asking.RequestID, ConversationID: asking.ConversationID, InterruptID: &irq.ID,
			RunType: RunTypeResume, ClientRequestID: in.ClientRequestID, Now: now,
		}); err != nil {
			return err
		}
		return s.start(ctx, q, answerID, in.Message, asking.ConversationID, now)
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return answerID, nil
}

func replayResume(ctx context.Context, q *store.Queries, r store.ProcurementAgentRun, in ResumeInput) (uuid.UUID, error) {
	input, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: r.ID, BuyerID: in.BuyerID})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("run：讀取重送的輸入：%w", err)
	}
	if r.RunType != RunTypeResume || r.InterruptID == nil || *r.InterruptID != in.InterruptID || input.BuyerMessage != in.Message {
		return uuid.UUID{}, apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
	}
	return r.ID, nil
}
