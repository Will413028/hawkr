// Package grpcclient 是單體呼叫舊服務 gRPC 的共用部分：HTTP/2 cleartext（h2c）的 client 與服務間的 HMAC 簽章
// （設計書 52）。stub 由 buf 從 api/proto 生成到 internal/legacy/gen，以 connect-go 的 gRPC 協定呼叫，單體不依賴
// grpc-go 與舊的 go-pkg。最後一個舊服務改寫後（遷移計畫階段 C）連同 internal/legacy 刪除。
package grpcclient

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
)

// CallerName 是簽章裡的呼叫方名稱（x-internal-service）。舊服務只用它記 log，不做白名單。
const CallerName = "hawkr-monolith"

// HTTPClient 回傳走 h2c 的 client：舊服務（grpc-go、grpc-java）的 gRPC 都是 HTTP/2 cleartext。
func HTTPClient() *http.Client {
	tr := &http.Transport{}
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetUnencryptedHTTP2(true)
	return &http.Client{Transport: tr}
}

// Signer 為每個呼叫加上服務間的 HMAC 簽章（設計書 52；舊服務的 InternalAuthVerifier）：
//
//	x-internal-sig = base64(HMAC-SHA256(secret, service + "\n" + ts + "\n" + nonce + "\n" + accountID))
//
// accountID 是同一個請求的 x-account-id，沒有就是空字串。secret 是空字串時不簽，只有對方的
// INTERNAL_AUTH_MODE 不是 enforcing 時能這樣用。now 為 nil 時用 time.Now。
func Signer(secret string, now func() time.Time) connect.UnaryInterceptorFunc {
	if now == nil {
		now = time.Now
	}
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			h := req.Header()
			h.Set("x-internal-service", CallerName)
			if secret != "" {
				ts := strconv.FormatInt(now().Unix(), 10)
				nonce := newNonce()
				h.Set("x-internal-ts", ts)
				h.Set("x-internal-nonce", nonce)
				h.Set("x-internal-sig", Sign(secret, CallerName, ts, nonce, h.Get("x-account-id")))
			}
			return next(ctx, req)
		}
	}
}

// Sign 算出簽章；給 Signer 與測試用。
func Sign(secret, service, ts, nonce, accountID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(service + "\n" + ts + "\n" + nonce + "\n" + accountID))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand 不會失敗
	return hex.EncodeToString(b)
}
