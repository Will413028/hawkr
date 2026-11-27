package deliver

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

func (s *Service) sendEmail(ctx context.Context, p plan, final bool) error {
	err := s.d.Mail.Send(ctx, resend.Email{To: p.address, Subject: p.rendered.Title, Text: p.rendered.Body, IdempotencyKey: p.delivery.ID.String()})
	switch {
	case err == nil:
		return s.settle(ctx, p, true, "")
	case errors.Is(err, resend.ErrPermanent):
		s.log().WarnContext(ctx, "email 被 provider 永久拒絕", "delivery_id", p.delivery.ID, "error", err)
		return s.settle(ctx, p, false, ReasonEmailRejected)
	default:
		if errors.Is(err, resend.ErrAuth) {
			s.log().ErrorContext(ctx, "Resend 金鑰被拒（401、403），email 送不出去，需要修設定", "delivery_id", p.delivery.ID)
		}
		return s.retryOrFail(ctx, p, final, fmt.Errorf("寄信暫時性失敗：%w", err))
	}
}
