package resend

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SignatureTolerance 是簽章時間戳與現在的最大差距。
const SignatureTolerance = 5 * time.Minute

// ErrSignature 標示簽章缺漏、格式錯、時間戳超出容許或不符。
var ErrSignature = errors.New("resend：簽章驗證失敗")

// ErrMalformed 標示簽章正確但事件本文不合格式。
var ErrMalformed = errors.New("resend：事件格式不合")

// 事件型別：只有這兩種會讓位址進 suppression 名單。
const (
	EventBounced    = "email.bounced"
	EventComplained = "email.complained"
)

// Event 是驗過簽章、解析過的 webhook 事件。
type Event struct {
	// ID 是 Svix 訊息 ID（svix-id）：重送同一則事件時不變，當 inbox 的冪等鍵。
	ID   string
	Type string
	To   []string
	// BounceType 是退信類型（Permanent、Transient、Undetermined），只有 email.bounced 有。
	BounceType string
}

// Suppresses 回報這個事件是否要把收件位址加進 suppression 名單：投訴一律是；退信只有永久性（Permanent）的是，暫時性與未定的不是。
func (e Event) Suppresses() bool {
	switch e.Type {
	case EventComplained:
		return true
	case EventBounced:
		return e.BounceType == "Permanent"
	}
	return false
}

// Verify 以 Svix 的簽章規則驗 webhook：簽章內容是 `<svix-id>.<svix-timestamp>.<本文>`，以 `whsec_` 之後的 base64 密鑰做 HMAC-SHA256，
// svix-signature 是空白分隔的 `v1,<base64>`，任一個相符即可；時間戳與 now 差距超過 SignatureTolerance 拒絕。通過後解析事件。
func Verify(secret string, header http.Header, body []byte, now time.Time) (Event, error) {
	id, ts, sigs := header.Get("svix-id"), header.Get("svix-timestamp"), header.Get("svix-signature")
	if id == "" || ts == "" || sigs == "" {
		return Event{}, fmt.Errorf("%w：缺少標頭", ErrSignature)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) == 0 {
		return Event{}, fmt.Errorf("%w：密鑰格式不合", ErrSignature)
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Event{}, fmt.Errorf("%w：時間戳", ErrSignature)
	}
	if d := now.Sub(time.Unix(sec, 0)); d > SignatureTolerance || d < -SignatureTolerance {
		return Event{}, fmt.Errorf("%w：時間戳超出容許", ErrSignature)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	want := mac.Sum(nil)
	ok := false
	for _, part := range strings.Fields(sigs) {
		v, sig, found := strings.Cut(part, ",")
		if !found || v != "v1" {
			continue
		}
		got, err := base64.StdEncoding.DecodeString(sig)
		if err == nil && hmac.Equal(got, want) {
			ok = true
		}
	}
	if !ok {
		return Event{}, ErrSignature
	}
	ev, err := ParseEvent(body)
	if err != nil {
		return Event{}, err
	}
	ev.ID = id
	return ev, nil
}

// ParseEvent 解析事件本文（不驗簽章；inbox 的 handler 重讀已驗過的 payload 用）。
func ParseEvent(body []byte) (Event, error) {
	var raw struct {
		Type string `json:"type"`
		Data struct {
			To     []string `json:"to"`
			Bounce struct {
				Type string `json:"type"`
			} `json:"bounce"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Event{}, fmt.Errorf("%w：%w", ErrMalformed, err)
	}
	if raw.Type == "" {
		return Event{}, fmt.Errorf("%w：缺少 type", ErrMalformed)
	}
	return Event{Type: raw.Type, To: raw.Data.To, BounceType: raw.Data.Bounce.Type}, nil
}
