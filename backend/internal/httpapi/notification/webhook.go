package notification

import (
	"context"
	"log/slog"
	"net/http"

	httppayment "github.com/Will413028/hawkr/backend/internal/httpapi/payment"
)

// ResendWebhookRoute 是 Resend 退信與投訴 webhook 的入口；簽章就是驗證，所以掛在 Connect、身分與 CSRF 中介層之外。
const ResendWebhookRoute = "POST /webhooks/resend"

// maxResendWebhookBytes 是本文的上限：退信與投訴事件只有事件資訊與收件位址（幾 KB）。
const maxResendWebhookBytes = 64 << 10

// ResendEvents 是 webhook 入口需要的模組方法：驗簽、同一個 transaction 寫 platform inbox 並排入派送 job（commit 之後才回 2xx）。
type ResendEvents interface {
	ReceiveResendEvent(ctx context.Context, header http.Header, body []byte) error
}

type resendEvents struct{ e ResendEvents }

func (r resendEvents) ReceiveEvent(ctx context.Context, header http.Header, body []byte) error {
	return r.e.ReceiveResendEvent(ctx, header, body)
}

// NewResendWebhook 回傳 Resend webhook 入口。接受、重送與不處理的事件回 204；簽章或格式不合回 400；本文超過上限回 413；沒有設定密鑰回 503
// （provider 稍後重送）；其他失敗回 500（回應規則與收款 webhook 相同）。
func NewResendWebhook(events ResendEvents, log *slog.Logger) http.Handler {
	return httppayment.Webhook{Label: "Resend webhook", MaxBytes: maxResendWebhookBytes}.Handler(resendEvents{events}, log)
}
