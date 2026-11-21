package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river/rivertype"
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

// Canceller 是取消路徑：轉 CANCEL_REQUESTED 與取消 River job 在同一個 transaction。
// 買家取消不退額度；executor 收到 ctx 取消後轉 CANCELLED；job 已被 River 直接取消（還沒被領走）的在同一個 transaction 轉 CANCELLED；
// 等待輸入的 run 沒有 job，直接轉 CANCELLED（A9-D23）。其餘由 reconciler 收斂。
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
			if _, err := c.Ledger.Transition(ctx, r.ID, StatusCancelled, ""); err != nil {
				return err
			}
			out = CancelOutcome{Status: StatusCancelled, Disposition: DispositionRequested}
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
	state, err := c.Jobs.CancelJob(ctx, *r.JobID)
	if err != nil {
		return err
	}
	*out = CancelOutcome{Status: StatusCancelRequested, Disposition: DispositionRequested}
	if jobCancelledBeforeStart(state) {
		if _, err := c.Ledger.Transition(ctx, r.ID, StatusCancelled, ""); err != nil {
			return err
		}
		out.Status = StatusCancelled
	}
	return nil
}

// jobCancelledBeforeStart：River 回報 job 已是 cancelled 代表它不會有 executor 領走，run 不必等 reconciler。
func jobCancelledBeforeStart(state rivertype.JobState) bool {
	return state == rivertype.JobStateCancelled
}
