package deliver

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

// sendEmail 在 transaction 外寄信。投遞紀錄 ID 當冪等鍵：送出後寫回結果前當掉重試時，Resend 合併同鍵的請求，不重寄。
// 永久性錯誤不重試；其餘（暫時性與無法分類的）交給 River 重試。
func (s *Service) sendEmail(ctx context.Context, p plan, final bool) error {
	err := s.d.Mail.Send(ctx, resend.Email{To: p.address, Subject: p.rendered.Title, Text: p.rendered.Body, IdempotencyKey: p.delivery.ID.String()})
	switch {
	case err == nil:
		return s.settle(ctx, p, true, "")
	case errors.Is(err, resend.ErrPermanent):
		s.log().WarnContext(ctx, "email 被 provider 永久拒絕", "delivery_id", p.delivery.ID, "error", err)
		return s.settle(ctx, p, false, ReasonEmailRejected)
	default:
		return s.retryOrFail(ctx, p, final, fmt.Errorf("寄信暫時性失敗：%w", err))
	}
}
