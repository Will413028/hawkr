package deliver

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
)

// sendPush 在 transaction 外推給收件人在對應 App 的全部 token。至少一個 token 收下就算已送出（其餘暫時失敗的不重試，免得已收到的裝置重複收到）；
// 沒有任何 token 收下時：有暫時性失敗或憑證錯誤就重試，有永久失敗記永久失敗，全部是失效 token 記 token_unregistered。
// 只有 UNREGISTERED 與 SENDER_ID_MISMATCH 會停用（刪除）token。
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
		// 先定案再刪 token：定案後 job 不會再被重試，刪除失敗只記 log（下次推到這個 token 時 FCM 會再回失效）；順序反過來，刪除失敗的重試會對已收到的裝置重推。
		// 取消不影響定案：provider 已經收下，帳本一定要記到。
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

// dropTokens 刪除失效的 token；失敗只記 log，不影響投遞結果。
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
