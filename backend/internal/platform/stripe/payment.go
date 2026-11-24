package stripe

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdk "github.com/stripe/stripe-go/v87"
)

// MetaAttemptID 是搬錢物件的 metadata 鍵：值是建立呼叫的冪等鍵（嘗試 ID）。建立時由這個 client 寫入，Find* 依它比對，
// 兩邊只在這裡對得上（Stripe 的 Search 是最終一致的，不用；列表是強一致的）。
const MetaAttemptID = "attempt_id"

const (
	listPageLimit = 100
	// maxListPages 是一次查回最多翻的頁數；翻完仍沒有答案時回錯誤，不回「找不到」：沒有看完全部候選就不能證明它不存在。
	maxListPages = 50
)

// PaymentIntentInput 是建立 PaymentIntent 的輸入；Currency 由呼叫方決定（D67：USD，金額是 cent）。
type PaymentIntentInput struct {
	Amount         int64
	Currency       string
	Description    string
	TransferGroup  string
	IdempotencyKey string
	// Metadata 是額外的對帳欄位；MetaAttemptID 由 client 寫入，這裡帶同名的鍵會被覆寫。
	Metadata map[string]string
}

// PaymentIntent 是 PaymentIntent 的快照。PaidAt 是最近一筆 charge 的建立時間（讀取時展開 latest_charge；沒有展開或沒有 charge 時為零值）。
type PaymentIntent struct {
	ID             string
	Status         string
	Amount         int64
	Currency       string
	ClientSecret   string
	LatestChargeID string
	PaidAt         time.Time
	// LastErrorCode 是最近一次付款嘗試失敗的原因碼：error code，沒有時用 decline code。
	LastErrorCode string
	Metadata      map[string]string
}

const expandLatestCharge = "latest_charge"

func paymentIntentOf(pi *sdk.PaymentIntent) (PaymentIntent, error) {
	if pi == nil || pi.ID == "" {
		return PaymentIntent{}, errors.New("stripe：PaymentIntent 的回應沒有 id")
	}
	out := PaymentIntent{
		ID: pi.ID, Status: string(pi.Status), Amount: pi.Amount, Currency: string(pi.Currency),
		ClientSecret: pi.ClientSecret, Metadata: pi.Metadata,
	}
	if ch := pi.LatestCharge; ch != nil {
		out.LatestChargeID = ch.ID
		if ch.Created != 0 {
			out.PaidAt = time.Unix(ch.Created, 0).UTC()
		}
	}
	if e := pi.LastPaymentError; e != nil {
		out.LastErrorCode = string(e.Code)
		if out.LastErrorCode == "" {
			out.LastErrorCode = string(e.DeclineCode)
		}
	}
	return out, nil
}

// CreatePaymentIntent 以冪等鍵建立 PaymentIntent（Payment Element 用的自動付款方式、帶 transfer_group、不帶 on_behalf_of；spikes.md）。
func (c *Client) CreatePaymentIntent(ctx context.Context, in PaymentIntentInput) (PaymentIntent, error) {
	params := &sdk.PaymentIntentCreateParams{
		Amount:                  sdk.Int64(in.Amount),
		Currency:                sdk.String(in.Currency),
		TransferGroup:           sdk.String(in.TransferGroup),
		AutomaticPaymentMethods: &sdk.PaymentIntentCreateAutomaticPaymentMethodsParams{Enabled: sdk.Bool(true)},
		Metadata:                withAttempt(in.Metadata, in.IdempotencyKey),
	}
	if in.Description != "" {
		params.Description = sdk.String(in.Description)
	}
	params.SetIdempotencyKey(in.IdempotencyKey)
	var out *sdk.PaymentIntent
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1PaymentIntents.Create(ctx, params)
		return err
	}); err != nil {
		return PaymentIntent{}, err
	}
	return paymentIntentOf(out)
}

// GetPaymentIntent 讀 PaymentIntent（展開 latest_charge 以取入帳時間）。
func (c *Client) GetPaymentIntent(ctx context.Context, id string) (PaymentIntent, error) {
	params := &sdk.PaymentIntentRetrieveParams{}
	params.AddExpand(expandLatestCharge)
	var out *sdk.PaymentIntent
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1PaymentIntents.Retrieve(ctx, id, params)
		return err
	}); err != nil {
		return PaymentIntent{}, err
	}
	return paymentIntentOf(out)
}

// CancelPaymentIntent 取消 PaymentIntent；已取消或已入帳時 Stripe 回 400，由呼叫方決定怎麼處理。
func (c *Client) CancelPaymentIntent(ctx context.Context, id string) (PaymentIntent, error) {
	params := &sdk.PaymentIntentCancelParams{}
	params.AddExpand(expandLatestCharge)
	var out *sdk.PaymentIntent
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1PaymentIntents.Cancel(ctx, id, params)
		return err
	}); err != nil {
		return PaymentIntent{}, err
	}
	return paymentIntentOf(out)
}

// FindPaymentIntentByAttempt 由新到舊列出 PaymentIntent，回傳 metadata.attempt_id 相符的那筆。PaymentIntent 的列表沒有 metadata 或 transfer_group 篩選，
// 所以只能比對；翻到 maxListPages 仍沒找到而且還有更多時回錯誤（不是 found=false）。
func (c *Client) FindPaymentIntentByAttempt(ctx context.Context, attemptID string) (PaymentIntent, bool, error) {
	var found *sdk.PaymentIntent
	err := listPages(ctx, c, func(ctx context.Context, after string) *sdk.V1List[*sdk.PaymentIntent] {
		params := &sdk.PaymentIntentListParams{}
		params.Limit = sdk.Int64(listPageLimit)
		params.Single = true
		params.AddExpand("data." + expandLatestCharge)
		if after != "" {
			params.StartingAfter = sdk.String(after)
		}
		return c.sdk.V1PaymentIntents.List(ctx, params)
	}, func(pi *sdk.PaymentIntent) string { return pi.ID }, func(pi *sdk.PaymentIntent) bool {
		if pi.Metadata[MetaAttemptID] == attemptID {
			found = pi
			return true
		}
		return false
	})
	if err != nil || found == nil {
		return PaymentIntent{}, false, err
	}
	pi, err := paymentIntentOf(found)
	return pi, err == nil, err
}

// Refund 是退款的快照。
type Refund struct {
	ID            string
	Status        string
	Amount        int64
	FailureReason string
	Metadata      map[string]string
}

func refundOf(r *sdk.Refund) (Refund, error) {
	if r == nil || r.ID == "" {
		return Refund{}, errors.New("stripe：退款的回應沒有 id")
	}
	return Refund{ID: r.ID, Status: string(r.Status), Amount: r.Amount, FailureReason: string(r.FailureReason), Metadata: r.Metadata}, nil
}

// CreateRefund 以冪等鍵對 PaymentIntent 建立退款（金額是 cent）；不帶 reverse_transfer（那是 destination charge 的參數，SCT 的撤回另行呼叫，spikes.md #7）。
func (c *Client) CreateRefund(ctx context.Context, paymentIntentID string, amount int64, idempotencyKey string) (Refund, error) {
	params := &sdk.RefundCreateParams{
		PaymentIntent: sdk.String(paymentIntentID),
		Amount:        sdk.Int64(amount),
		Metadata:      withAttempt(nil, idempotencyKey),
	}
	params.SetIdempotencyKey(idempotencyKey)
	var out *sdk.Refund
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1Refunds.Create(ctx, params)
		return err
	}); err != nil {
		return Refund{}, err
	}
	return refundOf(out)
}

// GetRefund 讀退款。
func (c *Client) GetRefund(ctx context.Context, id string) (Refund, error) {
	var out *sdk.Refund
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1Refunds.Retrieve(ctx, id, &sdk.RefundRetrieveParams{})
		return err
	}); err != nil {
		return Refund{}, err
	}
	return refundOf(out)
}

// FindRefundByAttempt 列出 PaymentIntent 底下的退款（payment_intent 篩選，強一致），回傳 metadata.attempt_id 相符的那筆。
func (c *Client) FindRefundByAttempt(ctx context.Context, paymentIntentID, attemptID string) (Refund, bool, error) {
	var found *sdk.Refund
	err := listPages(ctx, c, func(ctx context.Context, after string) *sdk.V1List[*sdk.Refund] {
		params := &sdk.RefundListParams{PaymentIntent: sdk.String(paymentIntentID)}
		params.Limit = sdk.Int64(listPageLimit)
		params.Single = true
		if after != "" {
			params.StartingAfter = sdk.String(after)
		}
		return c.sdk.V1Refunds.List(ctx, params)
	}, func(r *sdk.Refund) string { return r.ID }, func(r *sdk.Refund) bool {
		if r.Metadata[MetaAttemptID] == attemptID {
			found = r
			return true
		}
		return false
	})
	if err != nil || found == nil {
		return Refund{}, false, err
	}
	r, err := refundOf(found)
	return r, err == nil, err
}

// TransferInput 是建立轉帳的輸入：帶 source_transaction（付款的 charge）、金額是 cent、幣別必須等於入帳幣別（spikes.md #10）。
type TransferInput struct {
	Amount            int64
	Currency          string
	SourceTransaction string
	Destination       string
	TransferGroup     string
	IdempotencyKey    string
}

// Transfer 是轉帳的快照；AmountReversed 是累計已撤回的金額。
type Transfer struct {
	ID             string
	Amount         int64
	AmountReversed int64
	Metadata       map[string]string
}

func transferOf(t *sdk.Transfer) (Transfer, error) {
	if t == nil || t.ID == "" {
		return Transfer{}, errors.New("stripe：轉帳的回應沒有 id")
	}
	return Transfer{ID: t.ID, Amount: t.Amount, AmountReversed: t.AmountReversed, Metadata: t.Metadata}, nil
}

// CreateTransfer 以冪等鍵建立帶 source_transaction 的轉帳。
func (c *Client) CreateTransfer(ctx context.Context, in TransferInput) (Transfer, error) {
	params := &sdk.TransferCreateParams{
		Amount:            sdk.Int64(in.Amount),
		Currency:          sdk.String(in.Currency),
		Destination:       sdk.String(in.Destination),
		SourceTransaction: sdk.String(in.SourceTransaction),
		TransferGroup:     sdk.String(in.TransferGroup),
		Metadata:          withAttempt(nil, in.IdempotencyKey),
	}
	params.SetIdempotencyKey(in.IdempotencyKey)
	var out *sdk.Transfer
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1Transfers.Create(ctx, params)
		return err
	}); err != nil {
		return Transfer{}, err
	}
	return transferOf(out)
}

// FindTransferByAttempt 以 transfer_group 篩選列出轉帳（強一致；Stripe 文件，List Transfers），回傳 metadata.attempt_id 相符的那筆。transferGroup 必填：
// 沒有篩選就會列出整個平台的轉帳。
func (c *Client) FindTransferByAttempt(ctx context.Context, transferGroup, attemptID string) (Transfer, bool, error) {
	if transferGroup == "" {
		return Transfer{}, false, errors.New("stripe：查回轉帳需要 transfer_group")
	}
	var found *sdk.Transfer
	err := listPages(ctx, c, func(ctx context.Context, after string) *sdk.V1List[*sdk.Transfer] {
		params := &sdk.TransferListParams{TransferGroup: sdk.String(transferGroup)}
		params.Limit = sdk.Int64(listPageLimit)
		params.Single = true
		if after != "" {
			params.StartingAfter = sdk.String(after)
		}
		return c.sdk.V1Transfers.List(ctx, params)
	}, func(t *sdk.Transfer) string { return t.ID }, func(t *sdk.Transfer) bool {
		if t.Metadata[MetaAttemptID] == attemptID {
			found = t
			return true
		}
		return false
	})
	if err != nil || found == nil {
		return Transfer{}, false, err
	}
	t, err := transferOf(found)
	return t, err == nil, err
}

// Reversal 是一次轉帳撤回的快照。
type Reversal struct {
	ID       string
	Amount   int64
	Metadata map[string]string
}

func reversalOf(r *sdk.TransferReversal) (Reversal, error) {
	if r == nil || r.ID == "" {
		return Reversal{}, errors.New("stripe：轉帳撤回的回應沒有 id")
	}
	return Reversal{ID: r.ID, Amount: r.Amount, Metadata: r.Metadata}, nil
}

// CreateTransferReversal 以冪等鍵撤回轉帳的一部分或全部（金額是 cent）。
func (c *Client) CreateTransferReversal(ctx context.Context, transferID string, amount int64, idempotencyKey string) (Reversal, error) {
	params := &sdk.TransferReversalCreateParams{
		ID:       sdk.String(transferID),
		Amount:   sdk.Int64(amount),
		Metadata: withAttempt(nil, idempotencyKey),
	}
	params.SetIdempotencyKey(idempotencyKey)
	var out *sdk.TransferReversal
	if err := c.call(ctx, func(ctx context.Context) (err error) {
		out, err = c.sdk.V1TransferReversals.Create(ctx, params)
		return err
	}); err != nil {
		return Reversal{}, err
	}
	return reversalOf(out)
}

// FindReversalByAttempt 列出轉帳的撤回（強一致），回傳 metadata.attempt_id 相符的那筆。
func (c *Client) FindReversalByAttempt(ctx context.Context, transferID, attemptID string) (Reversal, bool, error) {
	var found *sdk.TransferReversal
	err := listPages(ctx, c, func(ctx context.Context, after string) *sdk.V1List[*sdk.TransferReversal] {
		params := &sdk.TransferReversalListParams{ID: sdk.String(transferID)}
		params.Limit = sdk.Int64(listPageLimit)
		params.Single = true
		if after != "" {
			params.StartingAfter = sdk.String(after)
		}
		return c.sdk.V1TransferReversals.List(ctx, params)
	}, func(r *sdk.TransferReversal) string { return r.ID }, func(r *sdk.TransferReversal) bool {
		if r.Metadata[MetaAttemptID] == attemptID {
			found = r
			return true
		}
		return false
	})
	if err != nil || found == nil {
		return Reversal{}, false, err
	}
	r, err := reversalOf(found)
	return r, err == nil, err
}

func withAttempt(extra map[string]string, attemptID string) map[string]string {
	m := make(map[string]string, len(extra)+1)
	for k, v := range extra {
		m[k] = v
	}
	m[MetaAttemptID] = attemptID
	return m
}

// listPages 逐頁（每頁各是一次有 deadline 的呼叫）列出項目，match 為真就停；翻完回 nil（沒有符合的），
// 到 maxListPages 仍有更多、或有更多卻沒有可接續的項目時回錯誤。
func listPages[T any](ctx context.Context, c *Client, fetch func(ctx context.Context, after string) *sdk.V1List[T], id func(T) string, match func(T) bool) error {
	after := ""
	for range maxListPages {
		var data []T
		var hasMore bool
		if err := c.call(ctx, func(ctx context.Context) error {
			list := fetch(ctx, after)
			if err := list.Err(); err != nil {
				return err
			}
			data, hasMore = list.Data(), list.Meta().HasMore
			return nil
		}); err != nil {
			return err
		}
		for _, item := range data {
			if match(item) {
				return nil
			}
		}
		if !hasMore {
			return nil
		}
		if len(data) == 0 {
			return errors.New("stripe：列表 has_more 為真但沒有可接續的項目")
		}
		after = id(data[len(data)-1])
		if after == "" {
			return errors.New("stripe：列表的最後一項沒有 id")
		}
	}
	return fmt.Errorf("stripe：列表超過 %d 頁仍未看完，不能證明不存在", maxListPages)
}
