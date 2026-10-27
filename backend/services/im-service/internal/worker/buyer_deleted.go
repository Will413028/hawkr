package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

func newBuyerDeletedHandler(msgUC *usecase.MessageUseCase, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return func(ctx context.Context, _ string, body []byte) error {
		var payload mq.BuyerDeletedEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			logger.Error("failed to unmarshal buyer.deleted event", zap.Error(err))
			return fmt.Errorf("unmarshal buyer.deleted event: %w", err)
		}
		if payload.AccountID == 0 {
			logger.Warn("buyer.deleted event missing accountId")
			return fmt.Errorf("buyer.deleted event missing accountId")
		}
		logger.Info("processing buyer.deleted event",
			zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID))
		if err := msgUC.RedactDeletedAccount(ctx, payload.AccountID); err != nil {
			logger.Error("failed to redact chat PII for deleted buyer",
				zap.Int64("account_id", payload.AccountID), zap.Error(err))
			return err
		}
		return nil
	}
}
