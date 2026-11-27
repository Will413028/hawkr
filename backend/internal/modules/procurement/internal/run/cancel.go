package run

import (
	"context"
	"fmt"

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
		var err error
		out, err = c.cancelLocked(ctx, buyerID, runID)
		return err
	})
	return out, err
}

// CancelAllOfRequest 取消需求所有還沒結束的 run（含等待輸入的），在呼叫端已開的 transaction 內，沿用 Cancel 的每一條路徑；
// 依 id 升序逐個鎖並重讀狀態（提問的 run 的 id 先於回答它的 run，符合鎖順序；已被前一個取消順帶處理的回答 run 重讀後走冪等分支）。
func (c Canceller) CancelAllOfRequest(ctx context.Context, buyerID int64, requestID uuid.UUID) error {
	ids, err := store.New(c.DB.Conn(ctx)).ListUnfinishedRunIDsByRequest(ctx, store.ListUnfinishedRunIDsByRequestParams{RequestID: requestID, BuyerID: buyerID})
	if err != nil {
		return fmt.Errorf("run：列出需求還沒結束的 run：%w", err)
	}
	for _, id := range ids {
		if _, err := c.cancelLocked(ctx, buyerID, id); err != nil {
			return fmt.Errorf("run：取消 run %s：%w", id, err)
		}
	}
	return nil
}

func (c Canceller) cancelLocked(ctx context.Context, buyerID int64, runID uuid.UUID) (CancelOutcome, error) {
	r, err := c.Ledger.lockRun(ctx, runID)
	if err != nil {
		return CancelOutcome{}, err
	}
	if r.BuyerID != buyerID {
		return CancelOutcome{}, apperr.NotFound("找不到這個 run")
	}
	var out CancelOutcome
	switch r.Status {
	case StatusCancelled:
		out = CancelOutcome{Status: r.Status, Disposition: DispositionAlreadyCancelled}
	case StatusWaitingForInput:
		if err := cancelWaiting(ctx, c.DB, c.Ledger, c.Jobs, r.ID); err != nil {
			return CancelOutcome{}, err
		}
		out = CancelOutcome{Status: StatusCancelled, Disposition: DispositionRequested}
	case StatusQueued, StatusRunning, StatusCancelRequested:
		if err := c.request(ctx, r, &out); err != nil {
			return CancelOutcome{}, err
		}
	default:
		out = CancelOutcome{Status: r.Status, Disposition: DispositionAlreadyTerminal}
	}
	return out, nil
}

func (c Canceller) request(ctx context.Context, r store.ProcurementAgentRun, out *CancelOutcome) error {
	status, err := requestCancel(ctx, c.Ledger, c.Jobs, r)
	if err != nil {
		return err
	}
	*out = CancelOutcome{Status: status, Disposition: DispositionRequested}
	return nil
}

func requestCancel(ctx context.Context, l Ledger, jobs JobQueue, r store.ProcurementAgentRun) (string, error) {
	if r.JobID == nil {
		return "", fmt.Errorf("run：%s 沒有 job，無法取消", r.ID)
	}
	state, found, err := jobs.CancelJob(ctx, *r.JobID)
	if err != nil {
		return "", err
	}
	if !found {
		o := decide(r, "", false)
		if err := l.converge(ctx, r.ID, o); err != nil {
			return "", err
		}
		if o.action == actionCancel {
			return StatusCancelled, nil
		}
		return StatusFailed, nil
	}
	if r.Status != StatusCancelRequested {
		if _, err := l.Transition(ctx, r.ID, StatusCancelRequested, ""); err != nil {
			return "", err
		}
	}
	if jobCancelledBeforeStart(state) {
		if _, err := l.Transition(ctx, r.ID, StatusCancelled, ""); err != nil {
			return "", err
		}
		return StatusCancelled, nil
	}
	return StatusCancelRequested, nil
}

func cancelWaiting(ctx context.Context, d *db.DB, l Ledger, jobs JobQueue, runID uuid.UUID) error {
	if _, err := l.Transition(ctx, runID, StatusCancelled, ""); err != nil {
		return err
	}
	answers, err := store.New(d.Conn(ctx)).LockInProgressAnswerRuns(ctx, runID)
	if err != nil {
		return fmt.Errorf("run：列出進行中的回答 run：%w", err)
	}
	for _, a := range answers {
		if _, err := requestCancel(ctx, l, jobs, a); err != nil {
			return fmt.Errorf("run：取消回答 run %s：%w", a.ID, err)
		}
	}
	return nil
}

func jobCancelledBeforeStart(state rivertype.JobState) bool {
	return state == rivertype.JobStateCancelled
}
