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

// callOutcome 是一次搬錢呼叫的結果三分（ADR D68）。
type callOutcome int

const (
	// callSucceeded provider 回了物件。
	callSucceeded callOutcome = iota
	// callRefused 終局拒絕：同一把鍵只會拿回同一個錯誤。
	callRefused
	// callRetryable provider 明確拒絕、沒有執行：換新冪等鍵重試，不計任何期限。
	callRetryable
	// callUnknown 其餘的錯誤：provider 那邊可能已執行，只能同鍵重送或查回。
	callUnknown
)

// classify 是 classifyCall 加上 D-F 的告警：401／403 仍是「不明」，但金鑰或權限壞了需要人修，所以每次發生都記一筆錯誤等級的 log（操作名稱、HTTP 狀態、provider 的錯誤代碼；不含金鑰與回應內容）。
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

// errNotFoundYet 標示查回查無、但從第一次呼叫起還不到 NotFoundAfter：不能當成確定不存在，稍後再查。
var errNotFoundYet = errors.New("charge: 查回查無，但還沒過最短等待時間")

// errRefundsInFlight 標示退款已涵蓋整筆付款、但其中還有沒終局的退款（PENDING、REFUNDING、ANOMALY）：轉帳不能先當成全額退款終結，它們失敗時賣家要收到這筆錢。
// 轉帳保持 PENDING、金額不承諾：job 以 transferWaitDelay snooze（不計嘗試），退款進終局時由同一個 transaction 排入的喚醒 job 立刻重新 claim。
// 擋住的退款卡在 ANOMALY 時，轉帳一直等到人工結案那筆退款；每次 snooze 回來都記一筆警示 log，帶轉帳與擋住它的退款 ID。
var errRefundsInFlight = errors.New("charge: 退款涵蓋整筆付款但還有進行中的，轉帳等它們終局")

// refundsInFlightError 是 errRefundsInFlight 帶上轉帳與擋住它的退款。
type refundsInFlightError struct {
	transferID uuid.UUID
	refundIDs  []uuid.UUID
}

func (e *refundsInFlightError) Error() string {
	return fmt.Sprintf("%v：轉帳 %s", errRefundsInFlight, e.transferID)
}

func (e *refundsInFlightError) Is(target error) bool { return target == errRefundsInFlight }

// inResendWindow 回傳這把鍵是否還可以重送：沒呼叫過，或第一次呼叫起還在 ResendWindow 內。
func inResendWindow(calledAt *time.Time, now time.Time) bool {
	return calledAt == nil || now.Sub(*calledAt) < ResendWindow
}

// firstCall 回傳剛記下的呼叫時刻是不是這把鍵的第一次呼叫：呼叫前讀到的 called_at 是空的，而且記下的時刻就是這次的 now（沒有被並行的呼叫搶先記過）。
// 只有第一次呼叫被 provider 明確拒絕，才證明這把鍵底下沒有任何東西執行過、可以換新鍵；重送被拒絕時前一次的結果仍不明（ADR D68）。
func firstCall(before, after *time.Time, now time.Time) bool {
	return before == nil && after != nil && after.Sub(now).Abs() < 2*time.Microsecond
}

// withDetail 把原因碼與外部 ID 併成事件的原因；沒有外部 ID 就是原因碼本身。
func withDetail(reason, detail string) string {
	if detail == "" {
		return reason
	}
	return reason + ":" + detail
}

// publishAnomaly 在呼叫方的 transaction 內發布付款本身的 MoneyAnomaly（沒有訂單應收）。
func (s *Service) publishAnomaly(ctx context.Context, kind model.AnomalyKind, objectID, paymentID uuid.UUID, reason string) error {
	return events.Publish(ctx, s.d.Events, model.MoneyAnomaly{OccurredAt: s.d.Now().UTC(), Kind: kind, ObjectID: objectID, PaymentID: paymentID, Reason: reason})
}

// publishOrderAnomaly 在呼叫方的 transaction 內發布屬於一張訂單應收的物件（轉帳、退款、撤回、補差額）的 MoneyAnomaly，帶上訂單應收與所屬付款。
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

// createdLowerBound 是查回列表的 created 下限：呼叫 provider 之前記下的時刻（第一次呼叫）減時鐘誤差。物件一定是在記下時刻之後才建立的，所以列出 created 不早於下限的全部物件，
// 看完整份列表沒有找到，才證明這把鍵底下沒有物件（D-E）。
func createdLowerBound(calledAt time.Time) time.Time { return calledAt.Add(-ListClockSkew).UTC() }

// lookupLowerBound 是查回的 created 下限：這把鍵第一次呼叫前記下的時刻（沒有記錄時用列的建立時刻，一定更早）減時鐘誤差。
func lookupLowerBound(calledAt *time.Time, createdAt time.Time) time.Time {
	if calledAt != nil {
		return createdLowerBound(*calledAt)
	}
	return createdLowerBound(createdAt)
}

// paymentOfOrder 讀（不鎖）訂單應收與它所屬的付款；轉帳、退款、撤回與補差額的 job 在 transaction 外取 provider 呼叫需要的外部 ID 與 transfer_group 用。
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

// lockOrder 依鎖順序鎖付款列、再鎖訂單應收列（付款列先於訂單應收列、轉帳列、退款列與撤回列）並回傳兩者；寫退款、轉帳、撤回與補差額的 transaction 都先呼叫它。
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
