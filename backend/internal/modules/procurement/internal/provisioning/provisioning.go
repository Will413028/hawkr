// Package provisioning 是接受報價之後的建單（A9-D45(1)、A9-D19 S6 修訂）：River worker 經 Creator 呼叫舊 ordering 的 CreateRfqOrder，
// 成功記下訂單號、暫時性失敗重試、終態拒絕或最後一次嘗試失敗就把接受單標成建單失敗並解除 live 鎖；Reconciler 是 job 死了的保底。
// 標成失敗之前（最後一次嘗試、reconciler）先用同一個 order_ref 再呼叫一次：舊 ordering 對 order_ref 冪等，回應遺失而單已建好時這一次取回既有訂單（A9-D45(1) 補充）。
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
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
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
	// Replayed 為真表示 order_ref 命中既有訂單、這次沒有建立新單。
	Replayed bool
}

// Parties 是訂單的買家、賣方店家與狀態。
type Parties struct {
	BuyerID int64
	StoreID int64
	// Status 是舊 order-service 的訂單狀態（OrderStatus 的字串值）；剛建好的單是 "0"（待付款）。
	Status string
}

// Creator 呼叫舊 ordering 建單；傳輸層錯誤回 error，業務拒絕以非 0 的 Code 回傳。
// OrderParties 讀既有訂單的買家、店家與狀態：CreateOrder 回 Replayed 時用它確認命中的是這張接受單自己、還在待付款的訂單。
type Creator interface {
	CreateOrder(ctx context.Context, cmd Command) (Result, error)
	OrderParties(ctx context.Context, orderNo string) (Parties, error)
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
	o, cause := w.create(ctx, id)
	if cause == nil {
		return nil
	}
	return w.retry(ctx, id, last, o, cause)
}

// outcome 是 create 的結果。
type outcome int

const (
	// outcomeNone 是沒有轉換：接受單已不是 CREATING_ORDER、標記沒有更新任何列，或是暫時性錯誤。
	outcomeNone outcome = iota
	outcomeCreated
	outcomeFailed
	// outcomeUnrecorded 是 ordering 已回了訂單號，但還沒記進接受單（確認既有訂單或寫入暫時失敗）：訂單可能已存在，絕不標成失敗。
	outcomeUnrecorded
)

// markError 是終態答案已知（reason）、但標記建單失敗沒寫成：之後只重試標記，不再呼叫 ordering，也不用別的原因覆蓋。
type markError struct {
	reason string
	err    error
}

func (e *markError) Error() string { return e.err.Error() }
func (e *markError) Unwrap() error { return e.err }

// finalTimeout 是最後一次再呼叫與標記失敗的上限：job 的 context 可能已經因逾時結束，所以另起一個。
const finalTimeout = 30 * time.Second

// pendingPaymentStatus 是舊 order-service 的待付款狀態（OrderStatus 0）：剛建好、還沒付款的單。
const pendingPaymentStatus = "0"

// create 嘗試建單一次。終態（已建好、已標失敗、已不是 CREATING_ORDER）回 nil error；暫時性失敗回 cause，由呼叫端決定重試或收尾。
func (w *Worker) create(ctx context.Context, id uuid.UUID) (outcome, error) {
	q := store.New(w.DB.Conn(ctx))
	b, err := q.GetOrderBasis(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return w.failed(ctx, id, ReasonBasisMissing)
	}
	if err != nil {
		return outcomeNone, fmt.Errorf("provisioning：讀取接受單 %s：%w", id, err)
	}
	if b.OrderProvisioningState != "CREATING_ORDER" {
		return outcomeNone, nil
	}
	if b.BuyerID == nil {
		return w.failed(ctx, id, ReasonBuyerDeleted)
	}
	name, err := ItemName(deref(b.Item), deref(b.ItemResolutionLabel), b.OfferedQuantity, b.OfferedQuantityUnit)
	if err != nil {
		w.Logger.WarnContext(ctx, "procurement：組不出建單品名", "acceptance_id", id.String(), "error", err)
		return w.failed(ctx, id, ReasonItemName)
	}
	res, err := w.Orders.CreateOrder(ctx, Command{
		OrderRef: b.OrderRef, BuyerID: *b.BuyerID, StoreID: b.StoreID, TotalAmount: b.AcceptedDealPrice,
		ShippingFee: b.ShippingAmount, UnitPrice: b.GoodsAmount + b.TaxAmount, ItemName: name,
	})
	if err != nil {
		return outcomeNone, fmt.Errorf("provisioning：呼叫建單：%w", err)
	}
	switch v := Classify(res); v.Outcome {
	case OutcomeRetry:
		return outcomeNone, fmt.Errorf("provisioning：ordering 回暫時性錯誤 %d：%s", res.Code, res.Message)
	case OutcomeRejected:
		w.Logger.WarnContext(ctx, "procurement：ordering 拒絕建單", "acceptance_id", id.String(), "code", res.Code, "message", res.Message)
		return w.failed(ctx, id, v.Reason)
	case OutcomeCreated:
	}
	if res.Replayed {
		// order_ref 從 1 起算，舊 order-service 的 orders.acceptance_id 可能已有同值的別人的單或早已取消的單：命中既有訂單時要確認
		// 買家與店家都對、訂單還在待付款、付款期限沒過。
		p, err := w.Orders.OrderParties(ctx, res.OrderNo)
		if err != nil {
			return outcomeUnrecorded, fmt.Errorf("provisioning：確認既有訂單 %s 的買家與店家：%w", res.OrderNo, err)
		}
		if p.BuyerID != *b.BuyerID || p.StoreID != b.StoreID || p.Status != pendingPaymentStatus || !res.PaymentDeadline.After(w.Now()) {
			w.Logger.ErrorContext(ctx, "procurement：order_ref 命中的既有訂單不是這張接受單的待付款訂單，不採用",
				"acceptance_id", id.String(), "order_ref", b.OrderRef, "order_no", res.OrderNo, "status", p.Status, "payment_deadline", res.PaymentDeadline)
			return w.failed(ctx, id, ReasonRejected)
		}
	}
	n, err := q.MarkOrderCreated(ctx, store.MarkOrderCreatedParams{ID: id, OrderNo: &res.OrderNo, PaymentDeadline: &res.PaymentDeadline, Now: w.Now()})
	if err != nil {
		return outcomeUnrecorded, fmt.Errorf("provisioning：記下訂單 %s：%w", res.OrderNo, err)
	}
	if n == 0 {
		w.Logger.ErrorContext(ctx, "procurement：ordering 已建單，但接受單已不是 CREATING_ORDER，訂單號沒有記下", "acceptance_id", id.String(), "order_no", res.OrderNo)
		return outcomeNone, nil
	}
	return outcomeCreated, nil
}

// failed 標記建單失敗並換算成 create 的結果：沒有更新任何列不算轉換；寫入失敗回 *markError，帶著已知的終態原因。
func (w *Worker) failed(ctx context.Context, id uuid.UUID, reason string) (outcome, error) {
	changed, err := w.fail(ctx, id, reason)
	switch {
	case err != nil:
		return outcomeNone, err
	case changed:
		return outcomeFailed, nil
	}
	return outcomeNone, nil
}

// detached 回傳收尾用的 context：job 逾時（DeadlineExceeded）時脫離父 context 的期限；關機取消（Canceled）要連它一起取消。
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	fresh, cancel := context.WithTimeout(context.WithoutCancel(ctx), finalTimeout)
	if ctx.Err() != nil {
		return fresh, cancel
	}
	stop := context.AfterFunc(ctx, func() {
		if errors.Is(ctx.Err(), context.Canceled) {
			cancel()
		}
	})
	return fresh, func() { stop(); cancel() }
}

// retry 回傳暫時性錯誤。已記下訂單號的暫時失敗（outcomeUnrecorded）絕不標成失敗，留給重試或 reconciler。
// 終態答案已知但標記沒寫成（*markError）只重試標記、用那個原因。最後一次嘗試先用同一個 order_ref 再呼叫一次（回應遺失時取回既有訂單），
// 仍失敗才標成建單失敗；關機取消（context.Canceled）不收尾，逾時則用新的 context 收尾。標記失敗時連同原因一併回傳，留給 reconciler。
func (w *Worker) retry(ctx context.Context, id uuid.UUID, last bool, o outcome, cause error) error {
	if o == outcomeUnrecorded || errors.Is(ctx.Err(), context.Canceled) {
		return cause
	}
	var terminal *markError
	if errors.As(cause, &terminal) {
		fresh, cancel := detached(ctx)
		defer cancel()
		if _, err := w.fail(fresh, id, terminal.reason); err != nil {
			return errors.Join(cause, err)
		}
		return nil
	}
	if !last {
		return cause
	}
	fresh, cancel := detached(ctx)
	defer cancel()
	o, again := w.create(fresh, id)
	if again == nil {
		return nil
	}
	if o == outcomeUnrecorded || errors.Is(ctx.Err(), context.Canceled) {
		return errors.Join(cause, again)
	}
	reason := ReasonRetryExhausted
	if errors.As(again, &terminal) {
		reason = terminal.reason
	}
	if _, err := w.fail(fresh, id, reason); err != nil {
		return errors.Join(cause, again, err)
	}
	return cause
}

// fail 標記建單失敗；回傳是否真的轉換了一列。
func (w *Worker) fail(ctx context.Context, id uuid.UUID, reason string) (bool, error) {
	changed, err := markFailed(ctx, w.Logger, store.New(w.DB.Conn(ctx)), id, reason, w.Now())
	if err != nil {
		return false, &markError{reason: reason, err: err}
	}
	return changed, nil
}

func markFailed(ctx context.Context, log *slog.Logger, q *store.Queries, id uuid.UUID, reason string, now time.Time) (bool, error) {
	n, err := q.MarkOrderFailed(ctx, store.MarkOrderFailedParams{ID: id, Reason: &reason, Now: now})
	if err != nil {
		return false, fmt.Errorf("provisioning：標記接受單 %s 建單失敗：%w", id, err)
	}
	if n == 0 {
		log.ErrorContext(ctx, "procurement：標記建單失敗沒有更新任何列，接受單已不是 CREATING_ORDER", "acceptance_id", id.String(), "reason", reason)
	}
	return n > 0, nil
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

// Reconciler 是建單 job 死了的保底：CREATING_ORDER 的接受單，其 job 已 discarded、cancelled、completed 卻沒收尾或不存在，先用同一個 order_ref 再呼叫一次建單
// （成功就記下訂單），仍失敗才標成建單失敗並解除 live 鎖。判斷 job 是否已死在一個 transaction 內，呼叫 ordering 在 transaction 之外；
// 逐列處理，一列失敗只記錄；job 還在排隊、重試或執行中一律略過。
type Reconciler struct {
	DB     *db.DB
	Orders Creator
	Now    func() time.Time
	Logger *slog.Logger
}

// Reconcile 掃過所有 CREATING_ORDER 的接受單，回傳標成失敗的列數。
func (r Reconciler) Reconcile(ctx context.Context, states run.JobStates) (failed int, err error) {
	var after uuid.UUID
	recovered := 0
	for {
		ids, err := store.New(r.DB.Conn(ctx)).ListCreatingOrderIDsAfter(ctx, store.ListCreatingOrderIDsAfterParams{After: after, Batch: reconcileBatch})
		if err != nil {
			return failed, fmt.Errorf("provisioning：列出要對帳的接受單：%w", err)
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return failed, err
			}
			res, err := r.reconcileOne(ctx, states, id)
			if err != nil {
				if ctx.Err() != nil {
					return failed, ctx.Err()
				}
				r.Logger.ErrorContext(ctx, "procurement：建單對帳失敗，略過這一列", "acceptance_id", id.String(), "error", err)
				continue
			}
			switch res {
			case outcomeFailed:
				failed++
			case outcomeCreated:
				recovered++
			case outcomeNone:
			}
		}
		if len(ids) < reconcileBatch {
			break
		}
		after = ids[len(ids)-1]
	}
	if failed > 0 || recovered > 0 {
		r.Logger.WarnContext(ctx, "procurement：建單對帳收尾了 job 已死的接受單", "failed", failed, "recovered", recovered)
	}
	return failed, nil
}

func (r Reconciler) reconcileOne(ctx context.Context, states run.JobStates, id uuid.UUID) (res outcome, err error) {
	var dead bool
	if err := r.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		dead, err = r.jobIsDead(ctx, states, id)
		return err
	}); err != nil {
		return outcomeNone, err
	}
	if !dead {
		return outcomeNone, nil
	}
	// 呼叫 ordering 與記下結果在同一個持有接受單 advisory lock 的 transaction 內（A9-D45 補充）：重疊的對帳拿不到鎖就略過，
	// 不會一個記下訂單、另一個把同一筆標成失敗。這個 transaction 在外部呼叫期間佔一條連線，由唯一的週期 job 限制成一條。
	err = r.DB.InTx(ctx, func(ctx context.Context) error {
		locked, err := store.New(r.DB.Conn(ctx)).TryLockOrderProvisioning(ctx, id)
		if err != nil {
			return fmt.Errorf("provisioning：鎖定接受單 %s 的建單：%w", id, err)
		}
		if !locked {
			return nil
		}
		w := Worker{DB: r.DB, Orders: r.Orders, Now: r.Now, Logger: r.Logger}
		o, cause := w.create(ctx, id)
		if cause == nil {
			res = o
			return nil
		}
		if o == outcomeUnrecorded {
			return cause
		}
		reason := ReasonJobLost
		var terminal *markError
		if errors.As(cause, &terminal) {
			reason = terminal.reason
		}
		res, err = w.failed(ctx, id, reason)
		if err != nil {
			return errors.Join(cause, err)
		}
		return nil
	})
	if err != nil {
		return outcomeNone, err
	}
	return res, nil
}

func (r Reconciler) jobIsDead(ctx context.Context, states run.JobStates, id uuid.UUID) (bool, error) {
	row, err := store.New(r.DB.Conn(ctx)).LockCreatingAcceptance(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("provisioning：鎖定接受單：%w", err)
	}
	if row.OrderJobID == nil {
		return true, nil
	}
	state, found, err := states.JobState(ctx, *row.OrderJobID)
	if err != nil {
		return false, fmt.Errorf("provisioning：讀取 job %d：%w", *row.OrderJobID, err)
	}
	return !found || !inProgress(state), nil
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

// InsertOpts 讓同時只有一個未結束的對帳 job：重疊的對帳不會同時處理同一批接受單。
func (ReconcileArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(0) }

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
