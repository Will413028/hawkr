package charge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const batchMax = 500

var errStillCreating = errors.New("charge: 付款的建立還沒收斂，稍後再關閉")

// PaymentByOrderNo 以訂單編號讀付款（不驗呼叫者，給訂單、發票與售後一方的服務間讀取）：回付款群組與被問到的那張訂單應收；不回 client_secret。沒有回 NotFound。
func (s *Service) PaymentByOrderNo(ctx context.Context, orderNo string) (model.OrderPayment, error) {
	q := store.New(s.d.DB.Conn(ctx))
	orderNo = strings.TrimSpace(orderNo)
	row, err := q.PaymentByOrderNo(ctx, orderNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.OrderPayment{}, apperr.NotFound("找不到這筆付款")
	}
	if err != nil {
		return model.OrderPayment{}, err
	}
	orders, err := q.PaymentOrdersOfPayment(ctx, row.ID)
	if err != nil {
		return model.OrderPayment{}, err
	}
	out := model.OrderPayment{Payment: toModel(row, orders, "")}
	for _, o := range out.Orders {
		if o.OrderNo == orderNo {
			out.Order = o
		}
	}
	return out, nil
}

// PaymentsByOrderNos 批次以訂單編號讀付款：每張找得到的訂單回一列（付款群組與那張訂單應收）；沒有付款的訂單不出現在結果裡，順序不保證。
func (s *Service) PaymentsByOrderNos(ctx context.Context, orderNos []string) ([]model.OrderPayment, error) {
	keys, err := batchKeys(orderNos, "訂單編號")
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.PaymentsByOrderNos(ctx, keys)
	if err != nil {
		return nil, err
	}
	siblings := make(map[uuid.UUID][]store.PaymentPaymentOrder)
	out := make([]model.OrderPayment, 0, len(rows))
	for _, r := range rows {
		orders, ok := siblings[r.PaymentPayment.ID]
		if !ok {
			if orders, err = q.PaymentOrdersOfPayment(ctx, r.PaymentPayment.ID); err != nil {
				return nil, err
			}
			siblings[r.PaymentPayment.ID] = orders
		}
		out = append(out, model.OrderPayment{
			Payment: toModel(r.PaymentPayment, orders, ""),
			Order:   model.PaymentOrder{ID: r.PaymentPaymentOrder.ID, OrderNo: r.PaymentPaymentOrder.OrderNo, StoreID: r.PaymentPaymentOrder.StoreID, Amount: r.PaymentPaymentOrder.Amount},
		})
	}
	return out, nil
}

// RefundByRequestKey 以呼叫端建立退款時給的冪等鍵讀退款。沒有回 NotFound；同一把鍵出現在不同訂單應收（鍵只在同一張訂單應收內唯一）回 FailedPrecondition，不猜是哪一筆。
func (s *Service) RefundByRequestKey(ctx context.Context, requestKey string) (model.Refund, error) {
	rows, err := s.RefundsByRequestKeys(ctx, []string{requestKey})
	if err != nil {
		return model.Refund{}, err
	}
	switch len(rows) {
	case 0:
		return model.Refund{}, apperr.NotFound("找不到這筆退款")
	case 1:
		return rows[0], nil
	}
	return model.Refund{}, apperr.FailedPrecondition("這把退款鍵對到多張訂單的退款")
}

// RefundsByRequestKeys 批次以冪等鍵讀退款；同一把鍵在多張訂單應收各有一筆時都回傳，沒有的鍵不出現。
func (s *Service) RefundsByRequestKeys(ctx context.Context, requestKeys []string) ([]model.Refund, error) {
	keys, err := batchKeys(requestKeys, "退款冪等鍵")
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).RefundsByRequestKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]model.Refund, 0, len(rows))
	for _, r := range rows {
		v := toRefund(r.PaymentRefund)
		v.OrderNo = r.OrderNo
		out = append(out, v)
	}
	return out, nil
}

// LatestRefundsByOrderNos 批次讀每張訂單最新的一筆退款（建立時間，同時間依 ID）；沒有退款的訂單不出現。
func (s *Service) LatestRefundsByOrderNos(ctx context.Context, orderNos []string) ([]model.Refund, error) {
	keys, err := batchKeys(orderNos, "訂單編號")
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).LatestRefundsByOrderNos(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]model.Refund, 0, len(rows))
	for _, r := range rows {
		v := toRefund(r.PaymentRefund)
		v.OrderNo = r.OrderNo
		out = append(out, v)
	}
	return out, nil
}

func batchKeys(in []string, what string) ([]string, error) {
	if len(in) > batchMax {
		return nil, apperr.InvalidInput(what + "一次最多 500 筆")
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			return nil, apperr.InvalidInput(what + "不得為空")
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out, nil
}

// CloseArgs 是依訂單關閉付款的 job（取消有效嘗試的 PaymentIntent），以付款 ID 為唯一鍵：訂單取消或逾期時由訂單一方經 CloseForOrder 排入，provider 在 transaction 外呼叫。
type CloseArgs struct {
	PaymentID uuid.UUID `json:"payment_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (CloseArgs) Kind() string { return "payment.charge_close" }

// InsertOpts 是關閉 job 的排入選項。
func (CloseArgs) InsertOpts() river.InsertOpts { return jobs.UniqueByArgs(maxAttempts) }

type closeWorker struct {
	river.WorkerDefaults[CloseArgs]
	service *Service
}

// Work 執行一次關閉；付款的建立還沒收斂是預期的等待，snooze（不計嘗試、不放棄）。
func (w *closeWorker) Work(ctx context.Context, job *river.Job[CloseArgs]) error {
	err := w.service.Close(ctx, job.Args.PaymentID)
	if errors.Is(err, errStillCreating) {
		return river.JobSnooze(closeWaitDelay)
	}
	return jobError(err)
}

// Timeout 是一次關閉的時間上限。
func (w *closeWorker) Timeout(*river.Job[CloseArgs]) time.Duration { return jobTimeout }

// CloseForOrder 是訂單取消或逾期時關閉這張訂單的付款（舊 ClosePaymentByOrderNo 的去向）：只有還沒入帳、provider 還能取消的付款（建立中或等待付款）才排入關閉 job，
// 回傳是否排入；沒有付款、已入帳、處理中（provider 不能取消）或已終局都回 false，不是錯誤。取消與買家付款競態時 provider 回入帳，照一般路徑記成入帳並發 PaymentChanged，
// 由訂單一方依訂單已取消發起自動退款（D5 修訂）。呼叫方不能在 transaction 內呼叫。
func (s *Service) CloseForOrder(ctx context.Context, orderNo string) (bool, error) {
	if err := s.requireProvider(); err != nil {
		return false, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return false, errInTransaction
	}
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return false, apperr.InvalidInput("訂單編號不得為空")
	}
	queued := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		pay, err := store.New(s.d.DB.Conn(ctx)).LockPaymentByOrderNo(ctx, orderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		switch model.PaymentStatus(pay.Status) {
		case model.PaymentCreating, model.PaymentAwaitingPayment:
		case model.PaymentProcessing, model.PaymentSucceeded, model.PaymentCanceled, model.PaymentCreateFailed, model.PaymentAnomaly:
			return nil
		}
		args := CloseArgs{PaymentID: pay.ID}
		opts := args.InsertOpts()
		queued = true
		return s.d.Jobs.Enqueue(ctx, args, &opts)
	})
	return queued, err
}

// Close 是關閉 job（payment.charge_close）的內容：付款等待中時在 transaction 外向 provider 取消有效嘗試的 PaymentIntent，結果快照照一般路徑寫入（取消與入帳競態時入帳勝出）。
// 付款還在建立中（沒有 PaymentIntent 可取消）時先讓建立收斂：以建立本身的規則（同一把冪等鍵重送，過了 ResendWindow 只查回）往前推一步，收斂成等待付款就接著取消；
// 還沒收斂（建立暫時失敗或查回還沒有答案）回 errStillCreating，job 等它而不放棄，所以不會有建好卻沒人取消的 PaymentIntent。其餘狀態什麼都不做。
func (s *Service) Close(ctx context.Context, paymentID uuid.UUID) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	p := s.d.Provider
	q := store.New(s.d.DB.Conn(ctx))
	pay, err := q.PaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if pay.Status == string(model.PaymentCreating) {
		res, cerr := s.create(ctx, paymentID, callTimeout)
		if cerr != nil {
			return fmt.Errorf("%w：%w", errStillCreating, cerr)
		}
		pay = res.row
		if pay.Status == string(model.PaymentCreating) {
			return errStillCreating
		}
	}
	switch model.PaymentStatus(pay.Status) {
	case model.PaymentAwaitingPayment:
	case model.PaymentCreating, model.PaymentProcessing, model.PaymentSucceeded, model.PaymentCanceled, model.PaymentCreateFailed, model.PaymentAnomaly:
		return nil
	}
	att, err := q.EffectiveAttempt(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if att.Provider != p.Name() || att.ExternalPaymentID == nil || attemptClosed(att.State) {
		return nil
	}
	startedAt := s.d.Now().UTC()
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := p.CancelPayment(callCtx, *att.ExternalPaymentID)
	if err != nil {
		return err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := s.applySnapshot(ctx, paymentID, att.ID, p.Name(), snap, startedAt)
		return err
	})
}
