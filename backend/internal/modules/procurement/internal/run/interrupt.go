package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// InterruptInput 是 run 停下來等買家回答的寫入。Payload 是中斷內容（JSON 物件），也是接續的唯一狀態；
// TurnResultEvent 非空時以 turn_result 事件寫進串流（與狀態轉換同 transaction，client 看得到等待就讀得到它）。
type InterruptInput struct {
	RunID           uuid.UUID
	Payload         []byte
	FactsAsOf       *time.Time
	TurnResultEvent []byte
}

// Interrupt 在同一個 transaction 內寫 turn_result 事件、把 run 轉 WAITING_FOR_INPUT 並新增 PENDING 的 CLARIFICATION 中斷，回傳中斷 ID。
// run 是回答別的中斷的 run 時，轉換同時是回答的提交點（被回答的中斷轉 RESOLVED、提問的 run 轉 SUCCEEDED），之後才寫新中斷，所以新中斷是該需求唯一的 PENDING。
// run 不在 RUNNING 回 *InvalidTransitionError，終態回 ErrRunTerminal（executor 停止），被回答的中斷已不在等待回答回 ErrInterruptNotPending；同一個需求已有別的 PENDING 中斷由唯一索引擋下。
func (l Ledger) Interrupt(ctx context.Context, in InterruptInput) (uuid.UUID, error) {
	id := uuid.NewV7()
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		if len(in.TurnResultEvent) > 0 {
			if _, err := l.append(ctx, in.RunID, EventTurnResult, in.TurnResultEvent, []string{StatusRunning}); err != nil {
				return err
			}
		}
		if _, err := l.Transition(ctx, in.RunID, StatusWaitingForInput, ""); err != nil {
			return err
		}
		if err := store.New(l.DB.Conn(ctx)).InsertInterrupt(ctx, store.InsertInterruptParams{
			ID: id, RunID: in.RunID, Payload: in.Payload, FactsAsOf: in.FactsAsOf, Now: l.Now(),
		}); err != nil {
			return fmt.Errorf("run：寫入中斷：%w", err)
		}
		return nil
	})
	return id, err
}

// OpenInterrupt 是等買家回答的中斷。Processing 為真代表有回答它的 run 還在進行中（讀取模型呈現「處理中」而不是「等回答」）。
type OpenInterrupt struct {
	store.ProcurementAgentRunInterrupt
	Processing bool
}

// OpenInterruptForRun 回買家自己的、狀態為 WAITING_FOR_INPUT 的 run 的未回答中斷；沒有回 found=false。被取消或被取代的 run 沒有。
func (l Ledger) OpenInterruptForRun(ctx context.Context, buyerID int64, runID uuid.UUID) (OpenInterrupt, bool, error) {
	i, err := store.New(l.DB.Conn(ctx)).GetOpenInterruptForRun(ctx, store.GetOpenInterruptForRunParams{RunID: runID, BuyerID: buyerID})
	return foundInterrupt(openInterruptFrom(i.ID, i.RunID, i.RequestID, i.InterruptKind, i.Status, i.Payload, i.FactsAsOf, i.CreatedAt, i.ClosedAt, i.Processing), err)
}

// OpenInterruptForRequest 回買家的需求目前等回答的中斷；沒有回 found=false。
func (l Ledger) OpenInterruptForRequest(ctx context.Context, buyerID int64, requestID uuid.UUID) (OpenInterrupt, bool, error) {
	i, err := store.New(l.DB.Conn(ctx)).GetOpenInterruptForRequest(ctx, store.GetOpenInterruptForRequestParams{RequestID: requestID, BuyerID: buyerID})
	return foundInterrupt(openInterruptFrom(i.ID, i.RunID, i.RequestID, i.InterruptKind, i.Status, i.Payload, i.FactsAsOf, i.CreatedAt, i.ClosedAt, i.Processing), err)
}

func foundInterrupt(i OpenInterrupt, err error) (OpenInterrupt, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return OpenInterrupt{}, false, nil
	}
	if err != nil {
		return OpenInterrupt{}, false, fmt.Errorf("run：讀取中斷：%w", err)
	}
	return i, true, nil
}

func openInterruptFrom(id, runID, requestID uuid.UUID, kind, status string, payload []byte, factsAsOf *time.Time, createdAt time.Time, closedAt *time.Time, processing bool) OpenInterrupt {
	return OpenInterrupt{
		ProcurementAgentRunInterrupt: store.ProcurementAgentRunInterrupt{
			ID: id, RunID: runID, RequestID: requestID, InterruptKind: kind, Status: status, Payload: payload,
			FactsAsOf: factsAsOf, CreatedAt: createdAt, ClosedAt: closedAt,
		},
		Processing: processing,
	}
}
