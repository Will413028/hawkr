package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/domain"
)

type sellerHubKeyResolver interface {
	CurrentStoreID(ctx context.Context, accountID int64) int64
}

// newSellerSuspendedHandler force-disconnects a seller's existing WS connections
// the instant they are suspended (14 號 §6.5.4，46 號 §4.4「已定裁決」，理由：停權
// 即時生效，不容既有長連線在停權後續傳非訂單關聯訊息；斷線後重連時 SendMessage
// gating（§6.5.3）即對每則送訊生效). Idempotent: Hub.DisconnectUser is a no-op
// (returns false) when the user has no live connections, so replays (retry /
// re-drive / duplicate delivery) are safe — matches 36 §3.6f's stated idempotency
// posture for this event family (逐筆判定，重送 0 副作用).
func newSellerSuspendedHandler(hub domain.Hub, sellerResolver sellerHubKeyResolver, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, _ string, body []byte) error {
		var event mq.SellerSuspendedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Error("failed to unmarshal seller.suspended event", zap.Error(err))
			return fmt.Errorf("unmarshal seller.suspended event: %w", err)
		}
		if event.AccountID == 0 {
			logger.Warn("seller.suspended event missing accountId")
			return fmt.Errorf("seller.suspended event missing accountId")
		}

		hubKey := event.AccountID
		if sellerResolver != nil {
			hubKey = sellerResolver.CurrentStoreID(ctx, event.AccountID)
		}
		disconnected := hub.DisconnectUser(hubKey)
		logger.Info("processed seller.suspended event: force-disconnected WS",
			zap.Int64("account_id", event.AccountID),
			zap.Int64("hub_key", hubKey),
			zap.Int64("suspension_id", event.SuspensionID),
			zap.String("reason_code", event.ReasonCode),
			zap.Bool("had_connection", disconnected))
		return nil
	}
}
