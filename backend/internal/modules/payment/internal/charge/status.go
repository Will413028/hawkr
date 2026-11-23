package charge

import (
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// ErrAnomaly 標示 provider 的快照與本地列對不起來（金額不符、外部 ID 換了、入帳沒有 charge）：不寫入，記 error log；同步 job 把它當永久失敗取消。
var ErrAnomaly = fmt.Errorf("%w：provider 的快照與付款列不一致", ErrPermanent)

// errUnknownState 標示快照的狀態不認得：不是有效值，不寫入；暫時性（provider 之後可能回已知狀態，或 adapter 補上對應），由 job 重試。
var errUnknownState = errors.New("charge: provider 回了不認得的 PaymentIntent 狀態")

// Next 由列目前的狀態與 provider 的快照推導下一個狀態。provider 是真相：除了入帳（終局，之後的任何快照都不倒退它，#35–#39 的 FAILED 不覆蓋 PAID）以外，
// 狀態直接跟著快照走；入帳一律勝出（包含列已經是 CANCELED：取消與付款競態時錢已經進來）。keep 為真表示快照與列衝突（列已入帳、快照不是入帳；
// 或列已是建立的終局或 ANOMALY）：呼叫方不寫入並記警示。快照的狀態不是有效值時回錯誤，不給出下一個狀態。
func Next(cur model.PaymentStatus, snap Snapshot) (next model.PaymentStatus, keep bool, err error) {
	if !snap.State.Valid() {
		return cur, false, errUnknownState
	}
	switch cur {
	case model.PaymentSucceeded:
		return cur, snap.State != StateSucceeded, nil
	case model.PaymentCreateFailed, model.PaymentAnomaly:
		return cur, true, nil
	case model.PaymentCreating, model.PaymentAwaitingPayment, model.PaymentProcessing, model.PaymentCanceled:
	}
	switch snap.State {
	case StateAwaitingPayment:
		return model.PaymentAwaitingPayment, false, nil
	case StateProcessing:
		return model.PaymentProcessing, false, nil
	case StateSucceeded:
		return model.PaymentSucceeded, false, nil
	case StateCanceled:
		return model.PaymentCanceled, false, nil
	case StateUnknown:
	}
	return cur, false, errUnknownState
}
