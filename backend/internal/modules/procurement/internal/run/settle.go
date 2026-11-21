package run

import (
	"context"
	"fmt"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// FailAndRefund 在同一個 transaction 內把 run 轉成 FAILED 並退回它扣的那一次額度（外部失敗退款，A9-D9）。
// run 已在終態回 ErrRunTerminal，不退款：退款只跟著這次成功的轉換。
func (l Ledger) FailAndRefund(ctx context.Context, runID uuid.UUID, reason string) (store.ProcurementAgentRun, error) {
	var out store.ProcurementAgentRun
	err := l.DB.InTx(ctx, func(ctx context.Context) error {
		r, err := l.Transition(ctx, runID, StatusFailed, reason)
		if err != nil {
			return err
		}
		out = r
		if _, err := store.New(l.DB.Conn(ctx)).RefundUsage(ctx, store.RefundUsageParams{BuyerID: r.BuyerID, PeriodStart: r.QuotaPeriodStart, Now: l.Now()}); err != nil {
			return fmt.Errorf("run：退回額度：%w", err)
		}
		return nil
	})
	return out, err
}
