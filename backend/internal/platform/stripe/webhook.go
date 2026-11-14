package stripe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stripe/stripe-go/v87/webhook"
)

// SignatureTolerance 是簽章時間戳與現在的最大差距（SDK 只拒絕過舊的時間戳）。
const SignatureTolerance = webhook.DefaultTolerance

// ErrSignature 標示簽章缺漏、格式錯、時間戳過舊或不符。
var ErrSignature = errors.New("stripe：簽章驗證失敗")

// ErrMalformed 標示簽章正確但事件本文不合格式（缺 id 或相關物件）。
var ErrMalformed = errors.New("stripe：事件格式不合")

// ErrNotHandled 標示簽章正確、格式合法，但不是平台自己的 v2.core.account 事件。
var ErrNotHandled = errors.New("stripe：不處理的事件型別")

// AccountEvent 是驗過簽章的帳戶事件：事件 ID 與帳戶 ID。
type AccountEvent struct {
	ID        string
	AccountID string
}

const accountObjectType = "v2.core.account"

// ParseEvent 以 SDK 驗 Stripe-Signature（預設時間戳容許，任一 v1 相符即可）；簽章之後只讀 id、type 與 related_object.{id,type}，其餘欄位不管。
// 只收 type 為 v2.core.account.* 或 v2.core.account[…].* 而且 related_object.type 為 v2.core.account 的 thin event。
func (c *Client) ParseEvent(header http.Header, body []byte) (AccountEvent, error) {
	if err := webhook.ValidatePayload(body, header.Get("Stripe-Signature"), c.whsecret); err != nil {
		return AccountEvent{}, fmt.Errorf("%w：%w", ErrSignature, err)
	}
	var ev struct {
		ID            string `json:"id"`
		Type          string `json:"type"`
		RelatedObject struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"related_object"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return AccountEvent{}, fmt.Errorf("%w：%w", ErrMalformed, err)
	}
	if ev.ID == "" || ev.Type == "" {
		return AccountEvent{}, fmt.Errorf("%w：缺少 id 或 type", ErrMalformed)
	}
	if !accountEventType(ev.Type) || ev.RelatedObject.Type != accountObjectType {
		return AccountEvent{}, fmt.Errorf("%w：%s", ErrNotHandled, ev.Type)
	}
	if ev.RelatedObject.ID == "" {
		return AccountEvent{}, fmt.Errorf("%w：缺少 related_object.id", ErrMalformed)
	}
	return AccountEvent{ID: ev.ID, AccountID: ev.RelatedObject.ID}, nil
}

func accountEventType(t string) bool {
	rest, ok := strings.CutPrefix(t, accountObjectType)
	return ok && (strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "["))
}
