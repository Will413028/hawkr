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

// Interrupt 在同一個 transaction 內寫 turn_result 事件、新增 PENDING 的 CLARIFICATION 中斷並把 run 轉 WAITING_FOR_INPUT，回傳中斷 ID。
// run 不在 RUNNING 回 *InvalidTransitionError，終態回 ErrRunTerminal（executor 停止）；同一個 run 已有 PENDING 中斷由唯一索引擋下。
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

// OpenInterruptForRun 回買家自己的、狀態為 WAITING_FOR_INPUT 的 run 的未回答中斷；沒有回 found=false。被取消或被取代的 run 沒有。
func (l Ledger) OpenInterruptForRun(ctx context.Context, buyerID int64, runID uuid.UUID) (store.ProcurementAgentRunInterrupt, bool, error) {
	i, err := store.New(l.DB.Conn(ctx)).GetOpenInterruptForRun(ctx, store.GetOpenInterruptForRunParams{RunID: runID, BuyerID: buyerID})
	return foundInterrupt(i, err)
}

// OpenInterruptForRequest 回買家的需求目前等回答的中斷；沒有回 found=false。
func (l Ledger) OpenInterruptForRequest(ctx context.Context, buyerID int64, requestID uuid.UUID) (store.ProcurementAgentRunInterrupt, bool, error) {
	i, err := store.New(l.DB.Conn(ctx)).GetOpenInterruptForRequest(ctx, store.GetOpenInterruptForRequestParams{RequestID: requestID, BuyerID: buyerID})
	return foundInterrupt(i, err)
}

func foundInterrupt(i store.ProcurementAgentRunInterrupt, err error) (store.ProcurementAgentRunInterrupt, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementAgentRunInterrupt{}, false, nil
	}
	if err != nil {
		return store.ProcurementAgentRunInterrupt{}, false, fmt.Errorf("run：讀取中斷：%w", err)
	}
	return i, true, nil
}
