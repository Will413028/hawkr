package charge

import (
	"context"
	"errors"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
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

// inResendWindow 回傳這把鍵是否還可以重送：沒呼叫過，或第一次呼叫起還在 ResendWindow 內。
func inResendWindow(calledAt *time.Time, now time.Time) bool {
	return calledAt == nil || now.Sub(*calledAt) < ResendWindow
}

// publishAnomaly 在呼叫方的 transaction 內發布 MoneyAnomaly。
func (s *Service) publishAnomaly(ctx context.Context, kind model.AnomalyKind, objectID, paymentID uuid.UUID, reason string) error {
	return events.Publish(ctx, s.d.Events, model.MoneyAnomaly{OccurredAt: s.d.Now().UTC(), Kind: kind, ObjectID: objectID, PaymentID: paymentID, Reason: reason})
}
