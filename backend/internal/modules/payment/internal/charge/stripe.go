package charge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

// StripeName 是 Stripe 付款 adapter 的 provider 代碼：付款事件走自己的 snapshot event destination 與簽章密鑰，
// 在 inbox 的 provider 名稱必須與收款帳戶的 adapter（"stripe"）不同。
const StripeName = "stripe_payments"

// StripeCurrency 是付款與轉帳的幣別（ADR D67）：訂單的整數金額當 USD 的最小單位（cent）照搬，不做匯率；port 與資料模型不帶幣別。
const StripeCurrency = "usd"

// stripeBalanceInsufficient 是平台餘額不足的 error code（樣本 transfer_platform_insufficient_funds）：沒有執行，補錢後重試會成功。
const stripeBalanceInsufficient = "balance_insufficient"

// Stripe 是 Provider 的 Stripe 實作：PaymentIntent、退款、轉帳與撤回都經 internal/platform/stripe 的遮罩 client
// （每個呼叫都有 deadline、429 在 client 內有限次退避），金額照搬 cent。
type Stripe struct {
	client *stripe.Client
}

// NewStripe 建立 Stripe adapter；client 的 WebhookSecret 必須是付款 event destination 的密鑰（不是收款帳戶的）。
func NewStripe(client *stripe.Client) (*Stripe, error) {
	if client == nil {
		return nil, errors.New("charge：Stripe client 必填")
	}
	return &Stripe{client: client}, nil
}

var _ Provider = (*Stripe)(nil)

// Name 實作 Provider。
func (s *Stripe) Name() string { return StripeName }

// classifyMoney 把搬錢的建立呼叫的錯誤分成三種（D10、ADR D68；依 HTTP 狀態與 error code，訊息只進 log，spikes.md）：
//   - 429、平台餘額不足（balance_insufficient）：Stripe 明確拒絕、沒有執行，包 ErrRetryableRefusal；
//   - 其他 4xx（400 參數與狀態不合、404、402，以及 409 idempotency_error：同一把鍵搭配了不同的參數）：包 ErrPermanent；
//   - 其餘（連線、逾時與 ctx 結束、5xx、408、401／403、其他 409、讀不懂的回應）：原樣回傳，結果不明——不得歸類成可重試拒絕或終局。
func classifyMoney(err error) error {
	var api *stripe.APIError
	if !errors.As(err, &api) {
		return err
	}
	switch {
	case api.Status == http.StatusTooManyRequests, api.Code == stripeBalanceInsufficient && api.Status >= 400 && api.Status < 500:
		return fmt.Errorf("%w：%w", ErrRetryableRefusal, err)
	case api.Permanent():
		return fmt.Errorf("%w：%w", ErrPermanent, err)
	}
	return err
}

// classifyRead 是讀取的錯誤分類：確定的 4xx（物件不存在等）包 ErrPermanent；限流、逾時、5xx 等原樣回傳，由呼叫方重試。
func classifyRead(err error) error {
	var api *stripe.APIError
	if errors.As(err, &api) && api.Status != http.StatusTooManyRequests && api.Permanent() {
		return fmt.Errorf("%w：%w", ErrPermanent, err)
	}
	return err
}

func intentState(status string) State {
	switch status {
	case "requires_payment_method", "requires_confirmation", "requires_action":
		return StateAwaitingPayment
	case "processing":
		return StateProcessing
	case "succeeded":
		return StateSucceeded
	case "canceled":
		return StateCanceled
	}
	return StateUnknown
}

func snapshotOf(pi stripe.PaymentIntent) Snapshot {
	snap := Snapshot{ExternalID: pi.ID, State: intentState(pi.Status), Amount: pi.Amount, LastError: pi.LastErrorCode, ClientSecret: pi.ClientSecret}
	if snap.State == StateSucceeded {
		snap.ChargeID, snap.PaidAt = pi.LatestChargeID, pi.PaidAt
	}
	return snap
}

// CreatePayment 實作 Provider。
func (s *Stripe) CreatePayment(ctx context.Context, in CreateInput) (Snapshot, error) {
	if in.IdempotencyKey == "" || in.TransferGroup == "" {
		return Snapshot{}, fmt.Errorf("%w：缺少冪等鍵或 transfer_group", ErrPermanent)
	}
	pi, err := s.client.CreatePaymentIntent(ctx, stripe.PaymentIntentInput{
		Amount: in.Amount, Currency: StripeCurrency, Description: in.Description, TransferGroup: in.TransferGroup, IdempotencyKey: in.IdempotencyKey,
		Metadata: map[string]string{"payment_id": in.PaymentID.String(), "store_id": strconv.FormatInt(in.StoreID, 10), "order_no": in.OrderNo},
	})
	if err != nil {
		return Snapshot{}, classifyMoney(err)
	}
	return snapshotOf(pi), nil
}

// FindPayment 實作 Provider：PaymentIntent 的列表沒有 metadata 篩選，由新到舊逐頁比對 attempt_id；翻不完回錯誤，不回「找不到」。
func (s *Stripe) FindPayment(ctx context.Context, attemptID string) (Snapshot, bool, error) {
	pi, found, err := s.client.FindPaymentIntentByAttempt(ctx, attemptID)
	if err != nil || !found {
		return Snapshot{}, false, err
	}
	return snapshotOf(pi), true, nil
}

// GetPayment 實作 Provider。
func (s *Stripe) GetPayment(ctx context.Context, externalID string) (Snapshot, error) {
	pi, err := s.client.GetPaymentIntent(ctx, externalID)
	if err != nil {
		return Snapshot{}, classifyRead(err)
	}
	return snapshotOf(pi), nil
}

// CancelPayment 實作 Provider：已取消或已入帳的 PaymentIntent 取消會被 Stripe 拒絕（文件只寫「回錯誤」，error code 是 payment_intent_unexpected_state；sandbox 未實測），
// 所以任何確定的 4xx 都重讀並回傳目前的快照：已取消視為成功，已入帳回入帳的快照（由呼叫方處理）；重讀到其他狀態、或重讀也失敗，回原本的錯誤分類。
func (s *Stripe) CancelPayment(ctx context.Context, externalID string) (Snapshot, error) {
	pi, err := s.client.CancelPaymentIntent(ctx, externalID)
	if err == nil {
		return snapshotOf(pi), nil
	}
	var api *stripe.APIError
	if !errors.As(err, &api) || !api.Permanent() {
		return Snapshot{}, classifyRead(err)
	}
	snap, gerr := s.GetPayment(ctx, externalID)
	if gerr != nil || (snap.State != StateCanceled && snap.State != StateSucceeded) {
		return Snapshot{}, classifyRead(err)
	}
	return snap, nil
}

func refundStateOf(status string) RefundState {
	switch status {
	case "pending":
		return RefundPending
	case "succeeded":
		return RefundSucceeded
	case "failed", "canceled":
		return RefundFailed
	}
	return RefundUnknown
}

func refundSnapshotOf(r stripe.Refund) RefundSnapshot {
	snap := RefundSnapshot{ExternalID: r.ID, State: refundStateOf(r.Status), Amount: r.Amount}
	if snap.State == RefundFailed {
		snap.FailureReason = r.FailureReason
		if snap.FailureReason == "" {
			snap.FailureReason = r.Status
		}
	}
	return snap
}

// CreateRefund 實作 Provider。
func (s *Stripe) CreateRefund(ctx context.Context, in RefundInput) (RefundSnapshot, error) {
	r, err := s.client.CreateRefund(ctx, in.ExternalPaymentID, in.Amount, in.IdempotencyKey)
	if err != nil {
		return RefundSnapshot{}, classifyMoney(err)
	}
	return refundSnapshotOf(r), nil
}

// FindRefund 實作 Provider：以 payment_intent 篩選列出退款，依 attempt_id 比對。
func (s *Stripe) FindRefund(ctx context.Context, externalPaymentID, attemptID string) (RefundSnapshot, bool, error) {
	r, found, err := s.client.FindRefundByAttempt(ctx, externalPaymentID, attemptID)
	if err != nil || !found {
		return RefundSnapshot{}, false, err
	}
	return refundSnapshotOf(r), true, nil
}

// GetRefund 實作 Provider。
func (s *Stripe) GetRefund(ctx context.Context, externalID string) (RefundSnapshot, error) {
	r, err := s.client.GetRefund(ctx, externalID)
	if err != nil {
		return RefundSnapshot{}, classifyRead(err)
	}
	return refundSnapshotOf(r), nil
}

// CreateTransfer 實作 Provider。
func (s *Stripe) CreateTransfer(ctx context.Context, in TransferInput) (TransferSnapshot, error) {
	t, err := s.client.CreateTransfer(ctx, stripe.TransferInput{
		Amount: in.Amount, Currency: StripeCurrency, SourceTransaction: in.SourceChargeID, Destination: in.DestinationAccountID,
		TransferGroup: in.TransferGroup, IdempotencyKey: in.IdempotencyKey,
	})
	if err != nil {
		return TransferSnapshot{}, classifyMoney(err)
	}
	return TransferSnapshot{ExternalID: t.ID, Amount: t.Amount, ReversedAmount: t.AmountReversed}, nil
}

// FindTransfer 實作 Provider：以 transfer_group 篩選列出轉帳，依 attempt_id 比對。
func (s *Stripe) FindTransfer(ctx context.Context, transferGroup, attemptID string) (TransferSnapshot, bool, error) {
	t, found, err := s.client.FindTransferByAttempt(ctx, transferGroup, attemptID)
	if err != nil || !found {
		return TransferSnapshot{}, false, err
	}
	return TransferSnapshot{ExternalID: t.ID, Amount: t.Amount, ReversedAmount: t.AmountReversed}, true, nil
}

// ReverseTransfer 實作 Provider：賣家餘額不足時 Stripe 仍回 200（餘額變負，spikes.md #9），所以沒有餘額分支。
func (s *Stripe) ReverseTransfer(ctx context.Context, in ReversalInput) (ReversalSnapshot, error) {
	r, err := s.client.CreateTransferReversal(ctx, in.ExternalTransferID, in.Amount, in.IdempotencyKey)
	if err != nil {
		return ReversalSnapshot{}, classifyMoney(err)
	}
	return ReversalSnapshot{ExternalID: r.ID, Amount: r.Amount}, nil
}

// FindReversal 實作 Provider：列出該轉帳的撤回，依 attempt_id 比對。
func (s *Stripe) FindReversal(ctx context.Context, externalTransferID, attemptID string) (ReversalSnapshot, bool, error) {
	r, found, err := s.client.FindReversalByAttempt(ctx, externalTransferID, attemptID)
	if err != nil || !found {
		return ReversalSnapshot{}, false, err
	}
	return ReversalSnapshot{ExternalID: r.ID, Amount: r.Amount}, true, nil
}

// ParseEvent 實作 Provider：只處理 PaymentIntent 與退款的事件；事件內容不攜帶狀態，處理者重讀物件。
func (s *Stripe) ParseEvent(header http.Header, body []byte) (Event, error) {
	ev, err := s.client.ParsePaymentEvent(header, body)
	switch {
	case errors.Is(err, stripe.ErrNotHandled):
		return Event{}, errors.Join(ErrIgnoredEvent, err)
	case errors.Is(err, stripe.ErrSignature), errors.Is(err, stripe.ErrMalformed):
		return Event{}, errors.Join(ErrInvalidEvent, err)
	case err != nil:
		return Event{}, err
	}
	kind := EventPayment
	if ev.ObjectType == stripe.PaymentObjectRefund {
		kind = EventRefund
	}
	return Event{ID: ev.ID, Kind: kind, ObjectID: ev.ObjectID}, nil
}
