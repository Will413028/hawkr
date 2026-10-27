package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// newSellerDeletedHandler purges the deleted seller stint's chat PII (Apple
// 5.1.1(v) + 個資法 right-to-erasure). Third leg of the buyer/account/seller
// split alongside newBuyerDeletedHandler (buyer_deleted.go) and
// newAccountDeletedHandler (account_deleted.go): seller.deleted covers a
// seller role closing alone on a multi-role account (account.deleted only
// fires when an account's LAST role is removed, so a seller who closes their
// seller role while keeping a buyer role never triggers account.deleted, and
// buyer.deleted never fires for a seller-role-only closure either). See
// account_deleted.go's doc comment for the complementary account.deleted
// case, and social-service's equivalent three-way split
// (buyer_deleted.go/account_deleted.go/seller_deleted.go) for the same
// pattern applied to a different domain.
func newSellerDeletedHandler(msgUC *usecase.MessageUseCase, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return mq.BindTypedHandler(logger, func(ctx context.Context, _ string, payload *mq.SellerDeletedEvent) error {
		if payload.AccountID == 0 {
			logger.Warn("seller.deleted event missing accountId")
			return fmt.Errorf("seller.deleted event missing accountId")
		}
		logger.Info("processing seller.deleted event",
			zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID))
		if err := msgUC.RedactDeletedAccount(ctx, payload.AccountID); err != nil {
			logger.Error("failed to redact chat PII for deleted seller stint",
				zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}
		return nil
	})
}
