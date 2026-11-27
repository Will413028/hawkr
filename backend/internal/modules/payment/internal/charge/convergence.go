package charge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type callOutcome int

const (
	callSucceeded callOutcome = iota
	callRefused
	callRetryable
	callUnknown
)

func (s *Service) classify(ctx context.Context, op string, err error) callOutcome {
	var rejected *AuthRejectedError
	if errors.As(err, &rejected) {
		s.d.Logger.ErrorContext(ctx, "payment：provider 拒絕認證或權限（金鑰或權限壞了，需要人修；結果視為不明，同鍵重送或查回）",
			"op", op, "provider", s.d.Provider.Name(), "http_status", rejected.Status, "provider_error_code", rejected.Code)
	}
	return classifyCall(err)
}

func classifyCall(err error) callOutcome {
	switch {
	case err == nil:
		return callSucceeded
	case errors.Is(err, ErrPermanent):
		return callRefused
	case errors.Is(err, ErrRetryableRefusal):
		return callRetryable
	}
	return callUnknown
}

var errNotFoundYet = errors.New("charge: 查回查無，但還沒過最短等待時間")

var errRefundsInFlight = errors.New("charge: 退款涵蓋整筆付款但還有進行中的，轉帳等它們終局")

type refundsInFlightError struct {
	transferID uuid.UUID
	refundIDs  []uuid.UUID
}

func (e *refundsInFlightError) Error() string {
	return fmt.Sprintf("%v：轉帳 %s", errRefundsInFlight, e.transferID)
}

func (e *refundsInFlightError) Is(target error) bool { return target == errRefundsInFlight }

func inResendWindow(calledAt *time.Time, now time.Time) bool {
	return calledAt == nil || now.Sub(*calledAt) < ResendWindow
}

func firstCall(before, after *time.Time, now time.Time) bool {
	return before == nil && after != nil && after.Sub(now).Abs() < 2*time.Microsecond
}

func withDetail(reason, detail string) string {
	if detail == "" {
		return reason
	}
	return reason + ":" + detail
}

func (s *Service) publishAnomaly(ctx context.Context, kind model.AnomalyKind, objectID, paymentID uuid.UUID, reason string) error {
	return events.Publish(ctx, s.d.Events, model.MoneyAnomaly{OccurredAt: s.d.Now().UTC(), Kind: kind, ObjectID: objectID, PaymentID: paymentID, Reason: reason})
}

func (s *Service) publishOrderAnomaly(ctx context.Context, kind model.AnomalyKind, objectID, paymentOrderID uuid.UUID, reason string) error {
	order, err := store.New(s.d.DB.Conn(ctx)).PaymentOrderByID(ctx, paymentOrderID)
	if err != nil {
		return err
	}
	return events.Publish(ctx, s.d.Events, model.MoneyAnomaly{
		OccurredAt: s.d.Now().UTC(), Kind: kind, ObjectID: objectID, PaymentID: order.PaymentID, PaymentOrderID: order.ID, Reason: reason,
	})
}

// TransferGroupOf 是付款的 transfer_group（一個 charge、N 筆轉帳都帶它）：付款群組的 ID。
func TransferGroupOf(paymentID uuid.UUID) string { return paymentID.String() }

// ListClockSkew 是查回的 created 下限對時鐘誤差的餘裕：下限取「這把鍵第一次呼叫的記錄時刻」往前推它，本機時鐘比 provider 慢也不會把剛建好的物件排除在列表之外。
const ListClockSkew = 5 * time.Minute

func createdLowerBound(calledAt time.Time) time.Time { return calledAt.Add(-ListClockSkew).UTC() }

func lookupLowerBound(calledAt *time.Time, createdAt time.Time) time.Time {
	if calledAt != nil {
		return createdLowerBound(*calledAt)
	}
	return createdLowerBound(createdAt)
}

func paymentOfOrder(ctx context.Context, q *store.Queries, paymentOrderID uuid.UUID) (store.PaymentPayment, store.PaymentPaymentOrder, error) {
	order, err := q.PaymentOrderByID(ctx, paymentOrderID)
	if err != nil {
		return store.PaymentPayment{}, store.PaymentPaymentOrder{}, err
	}
	pay, err := q.PaymentByID(ctx, order.PaymentID)
	if err != nil {
		return store.PaymentPayment{}, store.PaymentPaymentOrder{}, err
	}
	return pay, order, nil
}

func lockOrder(ctx context.Context, q *store.Queries, paymentOrderID uuid.UUID) (store.PaymentPayment, store.PaymentPaymentOrder, error) {
	order, err := q.PaymentOrderByID(ctx, paymentOrderID)
	if err != nil {
		return store.PaymentPayment{}, store.PaymentPaymentOrder{}, err
	}
	pay, err := q.LockPayment(ctx, order.PaymentID)
	if err != nil {
		return store.PaymentPayment{}, store.PaymentPaymentOrder{}, err
	}
	locked, err := q.LockPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return store.PaymentPayment{}, store.PaymentPaymentOrder{}, err
	}
	return pay, locked, nil
}
