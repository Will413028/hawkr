package deliver

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
)

func (s *Service) sendPush(ctx context.Context, p plan, final bool) error {
	results, err := s.d.Push.Send(ctx, p.tokens, pushMessage(p, p.inboxItemID, p.imageURL))
	if err != nil {
		return s.retryOrFail(ctx, p, final, fmt.Errorf("推播呼叫失敗：%w", err))
	}
	if len(results) != len(p.tokens) {
		return s.retryOrFail(ctx, p, final, fmt.Errorf("推播結果 %d 筆，預期 %d", len(results), len(p.tokens)))
	}
	var sent, transient, permanent, auth int
	var unregistered []string
	for _, r := range results {
		switch r.Outcome {
		case fcm.OutcomeSent:
			sent++
		case fcm.OutcomeUnregistered:
			unregistered = append(unregistered, r.Token)
		case fcm.OutcomeTransient:
			transient++
		case fcm.OutcomePermanent:
			permanent++
		case fcm.OutcomeAuth:
			auth++
		}
	}
	if auth > 0 {
		s.log().ErrorContext(ctx, "FCM 憑證或 APNs 認證被拒（THIRD_PARTY_AUTH_ERROR 等），推播送不出去，需要修設定", "delivery_id", p.delivery.ID, "tokens", auth)
	}
	if sent > 0 {
		err := s.settle(context.WithoutCancel(ctx), p, true, "")
		s.dropTokens(ctx, unregistered)
		return err
	}
	s.dropTokens(ctx, unregistered)
	switch {
	case transient > 0 || auth > 0:
		return s.retryOrFail(ctx, p, final, fmt.Errorf("推播暫時性失敗：%d 個 token 暫時失敗、%d 個憑證錯誤", transient, auth))
	case permanent > 0:
		return s.settle(ctx, p, false, ReasonPushRejected)
	default:
		return s.settle(ctx, p, false, ReasonTokenGone)
	}
}

func (s *Service) dropTokens(ctx context.Context, tokens []string) {
	if len(tokens) == 0 {
		return
	}
	if err := s.deleteTokens(context.WithoutCancel(ctx), tokens); err != nil {
		s.log().ErrorContext(ctx, "刪除失效的裝置 token 失敗，下次推送時 FCM 會再回失效", "error", err)
	}
}

func (s *Service) deleteTokens(ctx context.Context, tokens []string) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if _, err := store.New(s.d.DB.Conn(ctx)).DeleteDeviceTokensByValue(ctx, tokens); err != nil {
			return fmt.Errorf("刪除失效的裝置 token：%w", err)
		}
		return nil
	})
}
