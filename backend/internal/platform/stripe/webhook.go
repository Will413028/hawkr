package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SignatureTolerance 是簽章時間戳與現在的最大差距。
const SignatureTolerance = 300 * time.Second

// ErrSignature 標示簽章缺漏、格式錯、時間超出容許或不符。
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

// ParseEvent 驗 Stripe-Signature（`t=…,v1=…`，HMAC-SHA256 對 `{t}.{本文}`，任一 v1 相符即可，時間差在 SignatureTolerance 內）後，
// 只收 type 為 v2.core.account.* 或 v2.core.account[…].* 而且 related_object.type 為 v2.core.account 的 thin event。
func (c *Client) ParseEvent(header http.Header, body []byte) (AccountEvent, error) {
	if err := c.verify(header.Get("Stripe-Signature"), body); err != nil {
		return AccountEvent{}, err
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

func (c *Client) verify(header string, body []byte) error {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		switch {
		case !ok:
		case k == "t":
			ts = v
		case k == "v1":
			sigs = append(sigs, v)
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return fmt.Errorf("%w：標頭缺少 t 或 v1", ErrSignature)
	}
	if d := c.now().Sub(time.Unix(sec, 0)); d > SignatureTolerance || d < -SignatureTolerance {
		return fmt.Errorf("%w：時間戳超出容許範圍", ErrSignature)
	}
	mac := hmac.New(sha256.New, []byte(c.whsecret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	want := mac.Sum(nil)
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(got, want) {
			return nil
		}
	}
	return fmt.Errorf("%w：簽章不符", ErrSignature)
}
