package worker

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"go.uber.org/zap"
)

func newSellerDeletedHandler(repo domain.MarketWatchCleanupRepository, logger *zap.Logger) mq.HandlerFunc {
	return mq.BindTypedHandler(logger, func(ctx context.Context, _ string, event *mq.SellerDeletedEvent) error {
		if event.AccountID <= 0 {
			return fmt.Errorf("seller.deleted: accountId must be positive")
		}
		if err := repo.DeleteByAccountID(ctx, event.AccountID); err != nil {
			logger.Error("failed to delete seller market watches", zap.Int64("account_id", event.AccountID), zap.Error(err))
			return fmt.Errorf("seller.deleted cleanup: %w", err)
		}
		return nil
	})
}
