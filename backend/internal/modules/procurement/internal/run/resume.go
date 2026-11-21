package run

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

// Resume 回答買家自己的 PENDING 中斷，回傳子 run ID。同一個 transaction：鎖父 run、建子 run（CLARIFICATION_RESUME，不扣額度）、
// 中斷轉 RESOLVED、父 run 轉 RESUMED（寫 finished）、寫子 run 的輸入與 run_accepted、排入 job。
// 以 (buyer, client_request_id) 冪等，重送回同一個子 run，不受買家上限擋；不是這位買家的中斷回 NOT_FOUND；
// 中斷已被回答回 FAILED_PRECONDITION／APPROVAL_ALREADY_RESOLVED；父 run 不在等待輸入回 FAILED_PRECONDITION／RUN_NOT_ACTIVE。
// 子 run 失敗時 Ledger.Transition 把中斷重新開放、父 run 退回等待輸入，買家可以用新的 client_request_id 再答；防重送靠中斷 ID，不看 revision。
func (s Submitter) Resume(ctx context.Context, in ResumeInput) (uuid.UUID, error) {
	var childID uuid.UUID
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		if err := q.LockBuyerSubmits(ctx, strconv.FormatInt(in.BuyerID, 10)); err != nil {
			return fmt.Errorf("run：鎖買家的送出：%w", err)
		}
		existing, err := q.GetRunByIdempotencyKey(ctx, store.GetRunByIdempotencyKeyParams{BuyerID: in.BuyerID, ClientRequestID: in.ClientRequestID})
		if err == nil {
			childID, err = replayResume(ctx, q, existing, in)
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
		parent, err := q.LockRun(ctx, irq.RunID)
		if err != nil {
			return fmt.Errorf("run：鎖父 run：%w", err)
		}
		if irq.Status != "PENDING" {
			return apperr.FailedPrecondition("這個中斷已經回答過").WithReason(apperr.ReasonApprovalAlreadyResolved)
		}
		if parent.Status != StatusWaitingForInput {
			return apperr.FailedPrecondition("這個 run 不在等待輸入").WithReason(apperr.ReasonRunNotActive)
		}
		now := s.Ledger.Now()
		if err := s.checkLimits(ctx, q, in.BuyerID, nil, now); err != nil {
			return err
		}
		childID = uuid.NewV7()
		if _, err := s.insertRun(ctx, q, store.InsertRunParams{
			ID: childID, BuyerID: in.BuyerID, RequestID: parent.RequestID, ConversationID: parent.ConversationID, ParentRunID: &parent.ID,
			RunType: RunTypeResume, ClientRequestID: in.ClientRequestID, Now: now,
		}); err != nil {
			return err
		}
		if n, err := q.ResolveInterrupt(ctx, store.ResolveInterruptParams{ID: irq.ID, RunID: parent.ID, ChildRunID: &childID, Now: &now}); err != nil || n == 0 {
			return fmt.Errorf("run：中斷轉已回答（%d 列）：%w", n, err)
		}
		if _, err := s.Ledger.Transition(ctx, parent.ID, StatusResumed, ""); err != nil {
			return err
		}
		if _, err := q.SetResumedInto(ctx, store.SetResumedIntoParams{ID: parent.ID, ChildRunID: &childID}); err != nil {
			return fmt.Errorf("run：記下接手的子 run：%w", err)
		}
		return s.start(ctx, q, childID, in.Message, parent.ConversationID, now)
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return childID, nil
}

func replayResume(ctx context.Context, q *store.Queries, r store.ProcurementAgentRun, in ResumeInput) (uuid.UUID, error) {
	input, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: r.ID, BuyerID: in.BuyerID})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("run：讀取重送的輸入：%w", err)
	}
	irq, err := q.GetInterruptResolvedBy(ctx, &r.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, fmt.Errorf("run：讀取重送的中斷：%w", err)
	}
	// 子 run 失敗後中斷已重新開放（查不到被它回答的中斷）：同一把 client_request_id 的內容仍要相同，由訊息比對。
	sameInterrupt := errors.Is(err, pgx.ErrNoRows) || irq.ID == in.InterruptID
	if r.RunType != RunTypeResume || !sameInterrupt || input.BuyerMessage != in.Message {
		return uuid.UUID{}, apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
	}
	return r.ID, nil
}
