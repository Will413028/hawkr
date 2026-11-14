// Package payment 是 payment 模組的對外入口：後台的 PaymentAdminService 與收款 provider 的 webhook（Stripe 計畫 D29、D30）。
package payment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// WebhookRoute 是 Stripe webhook 的入口；簽章就是驗證，所以掛在 Connect、身分與 CSRF 中介層之外。
const WebhookRoute = "POST /webhooks/stripe"

// maxWebhookBytes 是 webhook 本文的上限；thin event 只有幾百 bytes。
const maxWebhookBytes = 64 << 10

// Events 是 webhook 入口需要的模組方法：驗簽、同一個 transaction 寫 platform inbox 並排入派送 job（commit 之後才回 2xx）。
type Events interface {
	ReceiveEvent(ctx context.Context, header http.Header, body []byte) error
}

// NewWebhook 回傳 webhook 入口。接受、重送與不處理的事件回 204；簽章或格式不合回 400；沒有設定 provider 回 503（provider 稍後重送）；
// 其他失敗回 500。
func NewWebhook(events Events, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		err = events.ReceiveEvent(r.Context(), r.Header, body)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case apperr.KindOf(err) == apperr.KindInvalidInput:
			http.Error(w, "invalid event", http.StatusBadRequest)
		case apperr.KindOf(err) == apperr.KindFailedPrecondition:
			http.Error(w, "payment provider not configured", http.StatusServiceUnavailable)
		default:
			log.ErrorContext(r.Context(), "收款 webhook 處理失敗", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
}
