package stripe

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/stripe/stripe-go/v87/webhook"
)

// PaymentObjectType 是付款事件指到的物件種類（data.object.object）。
type PaymentObjectType string

// 付款事件處理的物件種類；其他種類（轉帳、撤回、charge 等）是 ErrNotHandled。
const (
	PaymentObjectPaymentIntent PaymentObjectType = "payment_intent"
	PaymentObjectRefund        PaymentObjectType = "refund"
)

// PaymentEvent 是驗過簽章的付款事件：事件 ID、型別與它指的物件。
type PaymentEvent struct {
	ID         string
	Type       string
	ObjectType PaymentObjectType
	ObjectID   string
}

// ParsePaymentEvent 以付款 event destination 自己的簽章密鑰驗 Stripe-Signature（snapshot 事件，與帳戶的 thin destination 是兩把密鑰，spikes.md），
// 簽章之後只讀 id、type 與 data.object.{object,id}，其餘欄位不管（處理者一律重讀物件）。簽章錯回 ErrSignature，本文不合格式回 ErrMalformed，
// 物件不是 PaymentIntent 或退款回 ErrNotHandled。client 的 WebhookSecret 必須是付款 destination 的密鑰。
func (c *Client) ParsePaymentEvent(header http.Header, body []byte) (PaymentEvent, error) {
	if err := webhook.ValidatePayload(body, header.Get("Stripe-Signature"), c.whsecret); err != nil {
		return PaymentEvent{}, fmt.Errorf("%w：%w", ErrSignature, err)
	}
	var ev struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID     string `json:"id"`
				Object string `json:"object"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return PaymentEvent{}, fmt.Errorf("%w：%w", ErrMalformed, err)
	}
	if ev.ID == "" || ev.Type == "" {
		return PaymentEvent{}, fmt.Errorf("%w：缺少 id 或 type", ErrMalformed)
	}
	kind := PaymentObjectType(ev.Data.Object.Object)
	if kind != PaymentObjectPaymentIntent && kind != PaymentObjectRefund {
		return PaymentEvent{}, fmt.Errorf("%w：%s", ErrNotHandled, ev.Type)
	}
	if ev.Data.Object.ID == "" {
		return PaymentEvent{}, fmt.Errorf("%w：缺少 data.object.id", ErrMalformed)
	}
	return PaymentEvent{ID: ev.ID, Type: ev.Type, ObjectType: kind, ObjectID: ev.Data.Object.ID}, nil
}
