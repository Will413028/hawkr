// Package httpclient 是本 repo 出站 HTTP 呼叫的唯一入口。
//
// 為什麼要有這個套件：inbound 方向有 pkg/middleware/grpc/interceptor 的 OTel
// interceptor，outbound 方向在此之前完全沒有——整條修圖路徑上最慢、最常失敗
// 的那一跳（呼叫影像模型）在 trace 上不存在。裸 &http.Client{} 不會自己長出
// span，所以入口要收成一個。
//
// 目前沒有機械式強制：曾經嘗試用 .golangci.yml 的 forbidigo 規則擋裸
// &http.Client{}，但 forbidigo 的比對機制只看 Ident／SelectorExpr 節點文字，
// 結構上就不可能匹配到一個 composite literal，該規則已完整移除（不留任何
// 殘餘設定）。現階段繞過只能靠 code review 攔。要做到真正強制，待辦是：先把
// 既有的裸 &http.Client{} 呼叫點（pkg/payuni、pkg/mail/resend、pkg/sms、
// pkg/revalidate 等）轉成呼叫這個工廠，再用型別感知的 gocritic ruleguard
// （這個 repo 本來就用它擋 gateway 直呼 gin JSON，見 .golangci.yml 的
// gocritic.ruleguard 設定）補一條規則。
package httpclient

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// defaultTimeout 只是一個保守預設；有明確時間預算的呼叫端一律用 WithTimeout
// 顯式覆寫，不要依賴這個值。
const defaultTimeout = 30 * time.Second

// Option 調整工廠產出的 *http.Client。Transport 刻意不開放覆寫——可覆寫的話
// 這個套件就失去存在意義了。要換底層 RoundTripper 的話改這個檔案。
type Option func(*http.Client)

// WithTimeout 設定整個請求（含 body 讀取）的上限。
func WithTimeout(d time.Duration) Option {
	return func(c *http.Client) { c.Timeout = d }
}

// WithCheckRedirect 覆寫 redirect 策略。
func WithCheckRedirect(f func(req *http.Request, via []*http.Request) error) Option {
	return func(c *http.Client) { c.CheckRedirect = f }
}

// New 回傳一個 Transport 已包 otelhttp 的 *http.Client。
func New(opts ...Option) *http.Client {
	c := &http.Client{
		Timeout:   defaultTimeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
