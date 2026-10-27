package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// newAccountDeletedHandler purges the deleted account's chat PII (Apple 5.1.1(v) +
// 個資法 right-to-erasure).
//
// Restored alongside newBuyerDeletedHandler (buyer_deleted.go, bound to
// buyer.deleted): account.deleted only fires when an account's LAST role is
// removed, so a seller-only account (never had a buyer stint) being deleted
// would otherwise never get its chat history purged — buyer.deleted alone
// does not cover that case. See social-service's account_deleted.go /
// buyer_deleted.go split for the same pattern (buyer-store-stint rekey W2/W3
// design §3.2 D6, docs/superpowers/specs/2026-07-16-buyer-store-stint-rekey-w2w3-design.md).
//
// A processing error is returned so the ResilientConsumer retries and
// eventually dead-letters, rather than silently dropping the erasure.
func newAccountDeletedHandler(msgUC *usecase.MessageUseCase, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return mq.BindTypedHandler(logger, func(ctx context.Context, _ string, payload *mq.AccountDeletedEvent) error {
		if payload.AccountID == 0 {
			logger.Warn("account.deleted event missing accountId")
			return fmt.Errorf("account.deleted event missing accountId")
		}
		logger.Info("processing account.deleted event", zap.Int64("account_id", payload.AccountID))
		if err := msgUC.RedactDeletedAccount(ctx, payload.AccountID); err != nil {
			logger.Error("failed to redact chat PII for deleted account",
				zap.Int64("account_id", payload.AccountID), zap.Error(err))
			return err
		}
		return nil
	})
}
