// Package provisioning 是接受報價之後的建單（A9-D45(1)、A9-D19 S6 修訂）：River worker 經 Creator 呼叫舊 ordering 的 CreateRfqOrder，
// 成功記下訂單號、暫時性失敗重試、終態拒絕或最後一次嘗試失敗就把接受單標成建單失敗並解除 live 鎖；Reconciler 是 job 死了的保底。
// 呼叫在 transaction 之外（外部 provider，ADR D5），冪等鍵是接受單的 order_ref。
package provisioning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 建單失敗的原因（寫進 order_failure_reason）。
const (
	ReasonRejected       = "ORDER_SERVICE_REJECTED"
	ReasonRetryExhausted = "RETRY_EXHAUSTED"
	ReasonBuyerDeleted   = "BUYER_DELETED"
	ReasonItemName       = "ITEM_NAME_UNAVAILABLE"
	ReasonBasisMissing   = "ORDER_BASIS_MISSING"
	ReasonJobLost        = "ORDER_JOB_LOST"
)

// internalErrorCode 是舊 order-service 的 INTERNAL_ERROR(500)：唯一當成暫時性的回應碼。
const internalErrorCode int32 = 500

// Command 是一筆建單命令；OrderRef 是冪等鍵，金額是接受時凍結的值。
type Command struct {
	OrderRef    int64
	BuyerID     int64
	StoreID     int64
	TotalAmount int64
	ShippingFee int64
	UnitPrice   int64
	ItemName    string
}

// Result 是舊 ordering 的回應；Code 不為 0 時其餘欄位沒有意義。
type Result struct {
	Code            int32
	Message         string
	OrderNo         string
	PaymentDeadline time.Time
}

// Creator 呼叫舊 ordering 建單；傳輸層錯誤回 error，業務拒絕以非 0 的 Code 回傳。
type Creator interface {
	CreateOrder(ctx context.Context, cmd Command) (Result, error)
}

// Outcome 是一個回應的分類。
type Outcome int

const (
	OutcomeCreated Outcome = iota
	OutcomeRetry
	OutcomeRejected
)

// Verdict 是分類結果；Reason 只在 OutcomeRejected 有值。
type Verdict struct {
	Outcome Outcome
	Reason  string
}

// Classify 把回應分三類：成功、可重試（只有 500）、終態拒絕（其餘非 0 碼，或成功卻缺訂單號、付款期限）。
func Classify(r Result) Verdict {
	switch {
	case r.Code == internalErrorCode:
		return Verdict{Outcome: OutcomeRetry}
	case r.Code != 0:
		return Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}
	case r.OrderNo == "" || r.PaymentDeadline.IsZero():
		return Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}
	}
	return Verdict{Outcome: OutcomeCreated}
}

// ErrItemNameUnavailable 表示組不出品名：品項、數量或單位是空的；輸入不可變，重試不會變好。
var ErrItemNameUnavailable = errors.New("provisioning：組不出品名")

// ItemName 組出訂單明細的品名「品項 數量 單位」：品項取解析後的標籤，沒有才用買家的原話。
func ItemName(item, label, quantity, unit string) (string, error) {
	name := strings.TrimSpace(label)
	if name == "" {
		name = strings.TrimSpace(item)
	}
	quantity, unit = strings.TrimSpace(quantity), strings.TrimSpace(unit)
	if name == "" || quantity == "" || unit == "" {
		return "", fmt.Errorf("%w：item=%q quantity=%q unit=%q", ErrItemNameUnavailable, name, quantity, unit)
	}
	return name + " " + quantity + " " + unit, nil
}

// Worker 是 procurement.order_provisioning 的 River worker。
type Worker struct {
	river.WorkerDefaults[quote.OrderProvisioningArgs]
	DB     *db.DB
	Orders Creator
	Now    func() time.Time
	Logger *slog.Logger
}

// Timeout 是一次嘗試的上限（舊 ordering 的呼叫自己另有逾時）。
func (w *Worker) Timeout(*river.Job[quote.OrderProvisioningArgs]) time.Duration { return time.Minute }

// Work 建單；暫時性失敗回 error 讓 River 重試，最後一次嘗試仍失敗先把接受單標成建單失敗。
func (w *Worker) Work(ctx context.Context, job *river.Job[quote.OrderProvisioningArgs]) error {
	return w.Provision(ctx, job.Args.AcceptanceID, job.Attempt >= job.MaxAttempts)
}

// Provision 處理一筆接受單；last 表示這是最後一次嘗試。已不是 CREATING_ORDER 的接受單不做事，回 nil。
func (w *Worker) Provision(ctx context.Context, id uuid.UUID, last bool) error {
	q := store.New(w.DB.Conn(ctx))
	b, err := q.GetOrderBasis(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.fail(ctx, id, ReasonBasisMissing)
	}
	if err != nil {
		return w.retry(ctx, id, last, fmt.Errorf("provisioning：讀取接受單 %s：%w", id, err))
	}
	if b.OrderProvisioningState != "CREATING_ORDER" {
		return nil
	}
	if b.BuyerID == nil {
		return w.fail(ctx, id, ReasonBuyerDeleted)
	}
	name, err := ItemName(deref(b.Item), deref(b.ItemResolutionLabel), b.OfferedQuantity, b.OfferedQuantityUnit)
	if err != nil {
		w.Logger.WarnContext(ctx, "procurement：組不出建單品名", "acceptance_id", id.String(), "error", err)
		return w.fail(ctx, id, ReasonItemName)
	}
	res, err := w.Orders.CreateOrder(ctx, Command{
		OrderRef: b.OrderRef, BuyerID: *b.BuyerID, StoreID: b.StoreID, TotalAmount: b.AcceptedDealPrice,
		ShippingFee: b.ShippingAmount, UnitPrice: b.GoodsAmount + b.TaxAmount, ItemName: name,
	})
	if err != nil {
		return w.retry(ctx, id, last, fmt.Errorf("provisioning：呼叫建單：%w", err))
	}
	switch v := Classify(res); v.Outcome {
	case OutcomeRetry:
		return w.retry(ctx, id, last, fmt.Errorf("provisioning：ordering 回暫時性錯誤 %d：%s", res.Code, res.Message))
	case OutcomeRejected:
		w.Logger.WarnContext(ctx, "procurement：ordering 拒絕建單", "acceptance_id", id.String(), "code", res.Code, "message", res.Message)
		return w.fail(ctx, id, v.Reason)
	case OutcomeCreated:
	}
	if _, err := q.MarkOrderCreated(ctx, store.MarkOrderCreatedParams{ID: id, OrderNo: &res.OrderNo, PaymentDeadline: &res.PaymentDeadline, Now: w.Now()}); err != nil {
		return w.retry(ctx, id, last, fmt.Errorf("provisioning：記下訂單 %s：%w", res.OrderNo, err))
	}
	return nil
}

// retry 回傳暫時性錯誤；最後一次嘗試（且不是因為關機取消）先標成建單失敗。標記失敗時回標記的錯誤，留給 reconciler。
func (w *Worker) retry(ctx context.Context, id uuid.UUID, last bool, cause error) error {
	if !last || ctx.Err() != nil {
		return cause
	}
	if err := w.fail(ctx, id, ReasonRetryExhausted); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (w *Worker) fail(ctx context.Context, id uuid.UUID, reason string) error {
	return markFailed(ctx, store.New(w.DB.Conn(ctx)), id, reason, w.Now())
}

func markFailed(ctx context.Context, q *store.Queries, id uuid.UUID, reason string, now time.Time) error {
	if _, err := q.MarkOrderFailed(ctx, store.MarkOrderFailedParams{ID: id, Reason: &reason, Now: now}); err != nil {
		return fmt.Errorf("provisioning：標記接受單 %s 建單失敗：%w", id, err)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ReconcileInterval 是 reconciler 的週期；reconcileBatch 是一批的列數。
const (
	ReconcileInterval = time.Minute
	reconcileBatch    = 200
)

// Reconciler 是建單 job 死了的保底：CREATING_ORDER 的接受單，其 job 已 discarded、cancelled、completed 卻沒收尾或不存在，標成建單失敗並解除 live 鎖。
// 逐列一個 transaction，一列失敗只記錄；job 還在排隊、重試或執行中一律略過。
type Reconciler struct {
	DB     *db.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// Reconcile 掃過所有 CREATING_ORDER 的接受單，回傳標成失敗的列數。
func (r Reconciler) Reconcile(ctx context.Context, states run.JobStates) (failed int, err error) {
	var after uuid.UUID
	for {
		ids, err := store.New(r.DB.Conn(ctx)).ListCreatingOrderIDsAfter(ctx, store.ListCreatingOrderIDsAfterParams{After: after, Batch: reconcileBatch})
		if err != nil {
			return failed, fmt.Errorf("provisioning：列出要對帳的接受單：%w", err)
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return failed, err
			}
			var marked bool
			if err := r.DB.InTx(ctx, func(ctx context.Context) error {
				var err error
				marked, err = r.reconcileOne(ctx, states, id)
				return err
			}); err != nil {
				if ctx.Err() != nil {
					return failed, ctx.Err()
				}
				r.Logger.ErrorContext(ctx, "procurement：建單對帳失敗，略過這一列", "acceptance_id", id.String(), "error", err)
				continue
			}
			if marked {
				failed++
			}
		}
		if len(ids) < reconcileBatch {
			break
		}
		after = ids[len(ids)-1]
	}
	if failed > 0 {
		r.Logger.WarnContext(ctx, "procurement：建單對帳把接受單標成建單失敗", "failed", failed)
	}
	return failed, nil
}

func (r Reconciler) reconcileOne(ctx context.Context, states run.JobStates, id uuid.UUID) (bool, error) {
	q := store.New(r.DB.Conn(ctx))
	row, err := q.LockCreatingAcceptance(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("provisioning：鎖定接受單：%w", err)
	}
	if row.OrderJobID != nil {
		state, found, err := states.JobState(ctx, *row.OrderJobID)
		if err != nil {
			return false, fmt.Errorf("provisioning：讀取 job %d：%w", *row.OrderJobID, err)
		}
		if found && inProgress(state) {
			return false, nil
		}
	}
	return true, markFailed(ctx, q, id, ReasonJobLost, r.Now())
}

func inProgress(s rivertype.JobState) bool {
	switch s {
	case rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable, rivertype.JobStateRunning, rivertype.JobStateScheduled:
		return true
	case rivertype.JobStateCancelled, rivertype.JobStateCompleted, rivertype.JobStateDiscarded:
		return false
	}
	return false
}

// ReconcileArgs 是週期對帳的 job。
type ReconcileArgs struct{}

// Kind 是 job 的種類。
func (ReconcileArgs) Kind() string { return "procurement.order_reconcile" }

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	reconciler Reconciler
}

func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.reconciler.Reconcile(ctx, run.ClientJobStates{})
	return err
}

func (w *reconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記建單 worker 與 reconciler。
func AddWorkers(workers *river.Workers, w *Worker, r Reconciler) {
	river.AddWorker(workers, w)
	river.AddWorker(workers, &reconcileWorker{reconciler: r})
}

// PeriodicJobs 回傳 reconciler 的排程（啟動即執行一次）。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(river.PeriodicInterval(ReconcileInterval),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{ID: "procurement.order_reconcile", RunOnStart: true})}
}
