package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// wsTicketSecret 是 HMAC ws-ticket 的金鑰，取自 JWT_SECRET。gateway 不再驗 JWT（認證在單體），JWT_SECRET
// 只剩這個用途。
var wsTicketSecret = []byte(os.Getenv("JWT_SECRET"))

// chatWSTicketDomain binds an HMAC ws-ticket to this exact purpose. It is mixed
// into the signed bytes so the key can never produce a string that also
// verifies as something else signed with JWT_SECRET. Bump the suffix on any
// payload-shape change to invalidate old tokens at the verifier.
const chatWSTicketDomain = "hkwst1:"

// chatWSTicketClaims is the identity baked into a stateless HMAC ws-ticket — the
// degradation path used only when Redis (the single-use ticket store) is down,
// so a Redis outage no longer fail-closes the whole web chat (IM design D5).
// Mirrors pkgredis.WSTicket plus an expiry. Short field names keep the URL short.
type chatWSTicketClaims struct {
	AccountID   int64 `json:"a"`
	AccountType int32 `json:"t"`
	ActiveRole  int32 `json:"r"`
	Exp         int64 `json:"e"` // unix seconds
}

// SignChatWSTicket mints a stateless, HMAC-SHA256-signed ws-ticket carrying the
// resolved identity and a short expiry. Format: base64url(payload).base64url(mac).
//
// This is NOT a JWT and is honoured only by ChatWSAuth's ticket path, so even
// within its TTL it grants nothing beyond opening the chat WebSocket. The
// trade-off vs the Redis ticket is that it is replayable within its (short)
// TTL: D5 accepts that for the Redis-outage window ("can connect" beats
// "strictly single-use").
//
// 金鑰是 wsTicketSecret（JWT_SECRET）。空值不特別處理：config.Validate 在啟動時就要求 JWT_SECRET。
func SignChatWSTicket(accountID int64, accountType, activeRole int32, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(chatWSTicketClaims{
		AccountID:   accountID,
		AccountType: accountType,
		ActiveRole:  activeRole,
		Exp:         time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(chatWSTicketMAC(body)), nil
}

// chatWSTicketMAC computes HMAC-SHA256(wsTicketSecret, domain || body).
func chatWSTicketMAC(body string) []byte {
	mac := hmac.New(sha256.New, wsTicketSecret)
	mac.Write([]byte(chatWSTicketDomain))
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

// verifyChatWSTicket validates an HMAC ws-ticket's signature and expiry and
// returns the baked identity. ok=false for any malformed, wrong-signature, or
// expired token; the caller treats that as "not a valid degraded ticket" and
// 401s. The signature is checked (constant-time) before the payload is trusted.
func verifyChatWSTicket(token string) (*identity, bool) {
	body, sig, found := strings.Cut(token, ".")
	if !found || body == "" || sig == "" {
		return nil, false
	}
	gotMAC, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, false
	}
	if !hmac.Equal(gotMAC, chatWSTicketMAC(body)) {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, false
	}
	var cl chatWSTicketClaims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return nil, false
	}
	if time.Now().Unix() >= cl.Exp {
		return nil, false
	}
	// The HMAC proves the gateway minted this ticket, not that the baked active
	// role is still one the account holds — the ticket outlives a role removal.
	// Hold it to the role set like any other stored role (see sanitizeActiveRole).
	return newIdentity(cl.AccountID, cl.AccountType, cl.ActiveRole), true
}
