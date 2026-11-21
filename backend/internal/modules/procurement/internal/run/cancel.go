package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 取消命令的結果（對應 proto CancelDisposition）。
const (
	DispositionRequested        = "REQUESTED"
	DispositionAlreadyCancelled = "ALREADY_CANCELLED"
	DispositionAlreadyTerminal  = "ALREADY_TERMINAL"
)

// ErrNotCancellable 表示 run 停在等待輸入：它沒有進行中的 job，取消的語意沒有決定（A9-D22 沒寫）。
var ErrNotCancellable = errors.New("run：等待輸入中的 run 不能取消")

// Canceller 是取消路徑：轉 CANCEL_REQUESTED 與取消 River job 在同一個 transaction。
// 買家取消不退額度；executor 收到 ctx 取消後轉 CANCELLED，沒有 executor 在跑（job 尚未被領走）的由 reconciler 收斂。
type Canceller struct {
	DB     *db.DB
	Ledger Ledger
	Jobs   JobQueue
}

// CancelOutcome 是取消命令的結果：Status 是命令之後 run 的狀態。
type CancelOutcome struct {
	Status      string
	Disposition string
}

// Cancel 取消買家自己的 run；不是這位買家的 run 回 NOT_FOUND。冪等。
func (c Canceller) Cancel(ctx context.Context, buyerID int64, runID uuid.UUID) (CancelOutcome, error) {
	var out CancelOutcome
	err := c.DB.InTx(ctx, func(ctx context.Context) error {
		r, err := store.New(c.DB.Conn(ctx)).LockRun(ctx, runID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && r.BuyerID != buyerID {
			return apperr.NotFound("找不到這個 run")
		}
		if err != nil {
			return fmt.Errorf("run：讀取 run：%w", err)
		}
		switch r.Status {
		case StatusCancelled:
			out = CancelOutcome{Status: r.Status, Disposition: DispositionAlreadyCancelled}
		case StatusCancelRequested:
			out = CancelOutcome{Status: r.Status, Disposition: DispositionRequested}
		case StatusWaitingForInput:
			return ErrNotCancellable
		case StatusQueued, StatusRunning:
			return c.request(ctx, r, &out)
		default:
			out = CancelOutcome{Status: r.Status, Disposition: DispositionAlreadyTerminal}
		}
		return nil
	})
	return out, err
}

func (c Canceller) request(ctx context.Context, r store.ProcurementAgentRun, out *CancelOutcome) error {
	if r.JobID == nil {
		return fmt.Errorf("run：%s 沒有 job，無法取消", r.ID)
	}
	if _, err := c.Ledger.Transition(ctx, r.ID, StatusCancelRequested, ""); err != nil {
		return err
	}
	if err := c.Jobs.CancelJob(ctx, *r.JobID); err != nil {
		return err
	}
	*out = CancelOutcome{Status: StatusCancelRequested, Disposition: DispositionRequested}
	return nil
}
