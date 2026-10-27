package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// sellerDeletedSubscription scrubs a closed seller stint's PII from
// chat-notification inbox rows (個資法 right-to-erasure), soft-deletes the
// retired incarnation's own inbox rows, and deactivates the retired
// incarnation's own device_tokens rows (all issue #2707 / follow-up). Third leg
// alongside buyerDeletedSubscription and accountDeletedSubscription
// (account_deleted.go): covers a seller role closing alone on a multi-role
// account, which neither of the other two subscriptions fires for.
func sellerDeletedSubscription(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.Subscription {
	return mq.Subscription{
		Exchange:    mq.ExchangeSeller,
		Queue:       mq.QueueNotificationSellerDeleted,
		RoutingKeys: []string{mq.RoutingKeySellerDeleted},
		Handler:     newSellerDeletedHandler(inboxUC, logger),
		MaxRetries:  3,
	}
}

func newSellerDeletedHandler(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.HandlerFunc {
	return mq.BindTypedHandler(logger, func(ctx context.Context, _ string, payload *mq.SellerDeletedEvent) error {
		if payload.AccountID == 0 {
			logger.Warn("[worker] seller.deleted event missing accountId")
			return fmt.Errorf("seller.deleted event missing accountId")
		}
		n, err := inboxUC.RedactChatNotificationsByAccount(ctx, payload.AccountID)
		if err != nil {
			logger.Error("[worker] failed to scrub chat notifications for deleted seller stint",
				zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}
		logger.Info("[worker] scrubbed chat notification inbox PII for deleted seller stint",
			zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Int64("rows", n))

		// The redaction above only covers rows where the closed incarnation was the
		// *sender* of a chat notification (other people's inboxes). The rows
		// addressed TO that incarnation are a separate set and had no terminal state
		// at all: unreadable under the new store_id (gateway now resolves seller
		// notification identity to the current store_id, issue #2707) yet
		// deleted_at IS NULL, so the 90-day cleanup job never saw them. Soft-delete
		// hands them to that existing job — same handler, same write path, no new
		// consumer and no new schedule.
		//
		// Runs after the redaction and shares its fail-closed contract: an error
		// here retries the whole message, and the redaction is idempotent
		// (deleted_at IS NULL guards both statements). No StoreID == 0 guard here —
		// unlike the redaction call above, this step tolerates a zero StoreID the
		// same way SoftDeleteRetiredBuyerStintInbox tolerates a zero BuyerID: the
		// usecase's mergeUniqueStoreIDs drops zero, so a backfilled event (AccountID
		// set, StoreID always zero — see
		// seller-service/cmd/backfill-seller-deleted/main.go) still runs enumeration
		// and purges whatever historical incarnations it finds, instead of
		// dead-lettering forever on a field that backfill legitimately never sets.
		purged, err := inboxUC.SoftDeleteRetiredSellerIncarnationInbox(ctx, payload.AccountID, payload.StoreID)
		if err != nil {
			logger.Error("[worker] failed to soft-delete inbox rows of retired seller incarnation",
				zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}
		logger.Info("[worker] soft-deleted inbox rows of retired seller incarnation",
			zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Int64("rows", purged))

		// Third step: deactivate the retired incarnation's own device_tokens rows.
		// Without this, a seller who closes and reopens their store on the SAME
		// physical device without that device's FCM token rotating keeps a
		// still-active device_tokens row under the OLD store_id (UpsertDeviceToken's
		// ON CONFLICT (user_id, user_type, token) only ever inserts a new row under
		// the NEW store_id — nothing deactivates the old one) — a real, PERMANENT
		// duplicate-push cause, not the pre-deploy stale-token case above which
		// self-heals as tokens naturally rotate. Documented as an open residual risk
		// in 20260830100003_notification_announcement_identity_migrated_marker.sql
		// (b); this step closes it.
		//
		// Runs after the two steps above and shares their fail-closed contract: an
		// error here retries the whole message. No enumeration needed (unlike the
		// soft-delete step) — only the ONE incarnation this event just closed can
		// still hold an active row; see
		// DeactivateRetiredSellerIncarnationDeviceTokens's doc comment. Same
		// zero-StoreID tolerance as the soft-delete step above, for the same reason
		// (backfill never sets StoreID): a zero retiredStoreID is a no-op, not an
		// error.
		deactivated, err := inboxUC.DeactivateRetiredSellerIncarnationDeviceTokens(ctx, payload.StoreID)
		if err != nil {
			logger.Error("[worker] failed to deactivate device tokens of retired seller incarnation",
				zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}
		logger.Info("[worker] deactivated device tokens of retired seller incarnation",
			zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID), zap.Int64("rows", deactivated))
		return nil
	})
}
