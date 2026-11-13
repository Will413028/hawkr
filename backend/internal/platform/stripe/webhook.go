package stripe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/stripe/stripe-go/v87"
)

// SignatureTolerance 是簽章時間戳與現在的最大差距（SDK 只拒絕過舊的時間戳）。
const SignatureTolerance = 300 * time.Second

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

// ParseEvent 以 SDK 的 ParseEventNotification 驗 Stripe-Signature（時間戳容許 SignatureTolerance，任一 v1 相符即可）並解析 thin event 後，
// 只收 type 為 v2.core.account.* 或 v2.core.account[…].* 而且 related_object.type 為 v2.core.account 的事件。
func (c *Client) ParseEvent(header http.Header, body []byte) (AccountEvent, error) {
	n, err := c.sdk.ParseEventNotification(body, header.Get("Stripe-Signature"), c.whsecret, sdk.WithTolerance(SignatureTolerance))
	if err != nil {
		switch {
		case errors.Is(err, sdk.ErrWebhookEmptySecret), errors.Is(err, sdk.ErrWebhookInvalidHeader), errors.Is(err, sdk.ErrWebhookNoValidSignature),
			errors.Is(err, sdk.ErrWebhookNotSigned), errors.Is(err, sdk.ErrWebhookTooOld):
			return AccountEvent{}, fmt.Errorf("%w：%w", ErrSignature, err)
		}
		var obj struct {
			Object string `json:"object"`
		}
		if json.Unmarshal(body, &obj) == nil && obj.Object == "event" {
			return AccountEvent{}, fmt.Errorf("%w：snapshot 事件", ErrNotHandled)
		}
		return AccountEvent{}, fmt.Errorf("%w：%w", ErrMalformed, err)
	}
	ev := n.GetEventNotification()
	if ev.ID == "" || ev.Type == "" {
		return AccountEvent{}, fmt.Errorf("%w：缺少 id 或 type", ErrMalformed)
	}
	var related struct {
		RelatedObject sdk.V2CoreEventRelatedObject `json:"related_object"`
	}
	if err := json.Unmarshal(body, &related); err != nil {
		return AccountEvent{}, fmt.Errorf("%w：%w", ErrMalformed, err)
	}
	if !accountEventType(ev.Type) || related.RelatedObject.Type != accountObjectType {
		return AccountEvent{}, fmt.Errorf("%w：%s", ErrNotHandled, ev.Type)
	}
	if related.RelatedObject.ID == "" {
		return AccountEvent{}, fmt.Errorf("%w：缺少 related_object.id", ErrMalformed)
	}
	return AccountEvent{ID: ev.ID, AccountID: related.RelatedObject.ID}, nil
}

func accountEventType(t string) bool {
	rest, ok := strings.CutPrefix(t, accountObjectType)
	return ok && (strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "["))
}
