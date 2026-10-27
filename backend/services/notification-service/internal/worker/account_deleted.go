package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// buyerDeletedSubscription scrubs a closed buyer stint's PII from
// chat-notification inbox rows (個資法 right-to-erasure). It's a static
// subscription appended to the template-driven ones.
//
// This subscription co-exists with accountDeletedSubscription below (bound to
// account.deleted): buyer.deleted covers buyer-role-only closures on a
// dual-role account (account.deleted only fires when an account's LAST role
// is removed, so a buyer who closes their buyer role while keeping a seller
// role never triggers account.deleted), while accountDeletedSubscription
// covers full-account teardown — including a seller-only account that never
// had a buyer stint, which buyer.deleted alone would never see. See
// social-service's equivalent split (buyer store-stint rekey W2/W3 design
// §3.2 D6, docs/superpowers/specs/2026-07-16-buyer-store-stint-rekey-w2w3-design.md).
//
// Keyed on AccountID, same as accountDeletedSubscription below — do NOT switch
// this to BuyerID. RedactChatNotificationsByAccount enumerates every historical
// buyer_id/store_id surrogate for the account itself (including the just-closed
// stint), so passing the single BuyerID from this event's payload would both
// duplicate what enumeration already covers and miss any other diverged
// surrogates for the same account.
func buyerDeletedSubscription(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.Subscription {
	return mq.Subscription{
		Exchange:    mq.ExchangeBuyer,
		Queue:       mq.QueueNotificationBuyerDeleted,
		RoutingKeys: []string{mq.RoutingKeyBuyerDeleted},
		Handler:     newBuyerDeletedHandler(inboxUC, logger),
		MaxRetries:  3,
	}
}

func newBuyerDeletedHandler(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.HandlerFunc {
	return func(ctx context.Context, _ string, body []byte) error {
		var payload mq.BuyerDeletedEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			logger.Error("[worker] failed to unmarshal buyer.deleted event", zap.Error(err))
			return fmt.Errorf("unmarshal buyer.deleted event: %w", err)
		}
		if payload.AccountID == 0 {
			logger.Warn("[worker] buyer.deleted event missing accountId", zap.Int64("buyer_id", payload.BuyerID))
			return fmt.Errorf("buyer.deleted event missing accountId")
		}
		n, err := inboxUC.RedactChatNotificationsByAccount(ctx, payload.AccountID)
		if err != nil {
			logger.Error("[worker] failed to scrub chat notifications for deleted buyer stint",
				zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}
		logger.Info("[worker] scrubbed chat notification inbox PII for deleted buyer stint",
			zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID), zap.Int64("rows", n))

		// The redaction above only covers rows where the closed stint was the
		// *sender* of a chat notification (other people's inboxes). The rows
		// addressed TO that stint are a separate set and had no terminal state at
		// all: unreadable under the new identity (44 §10 D6 / §7.6 step 6) yet
		// deleted_at IS NULL, so the 90-day cleanup job never saw them. Soft-delete
		// hands them to that existing job — same handler, same write path, no new
		// consumer and no new schedule.
		//
		// Runs after the redaction and shares its fail-closed contract: an error
		// here retries the whole message, and the redaction is idempotent
		// (deleted_at IS NULL guards both statements).
		purged, err := inboxUC.SoftDeleteRetiredBuyerStintInbox(ctx, payload.AccountID, payload.BuyerID)
		if err != nil {
			logger.Error("[worker] failed to soft-delete inbox rows of retired buyer stint",
				zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID), zap.Error(err))
			return err
		}
		logger.Info("[worker] soft-deleted inbox rows of retired buyer stint",
			zap.Int64("account_id", payload.AccountID), zap.Int64("buyer_id", payload.BuyerID), zap.Int64("rows", purged))
		return nil
	}
}

// accountDeletedSubscription scrubs a deleted account's PII from
// chat-notification inbox rows (個資法 right-to-erasure). It's a static
// subscription appended to the template-driven ones, restored alongside
// buyerDeletedSubscription above.
//
// account.deleted only fires when an account's LAST role is removed, so this
// is the only cleanup path for a seller-only account (one that never had a
// buyer stint) being deleted — buyer.deleted never fires for it. See
// buyerDeletedSubscription's doc comment above for the complementary case
// this doesn't cover (buyer-role-only closure on a dual-role account).
func accountDeletedSubscription(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.Subscription {
	return mq.Subscription{
		Exchange:    mq.ExchangeAccount,
		Queue:       mq.QueueNotificationAccountDeleted,
		RoutingKeys: []string{mq.RoutingKeyAccountDeleted},
		Handler:     newAccountDeletedHandler(inboxUC, logger),
		MaxRetries:  3,
	}
}

func newAccountDeletedHandler(inboxUC *usecase.NotificationInboxUseCase, logger *zap.Logger) mq.HandlerFunc {
	return mq.BindTypedHandler(logger, func(ctx context.Context, _ string, payload *mq.AccountDeletedEvent) error {
		if payload.AccountID == 0 {
			logger.Warn("[worker] account.deleted event missing accountId")
			return fmt.Errorf("account.deleted event missing accountId")
		}
		// Keyed on AccountID — do NOT switch this to BuyerID. Unlike
		// buyer.deleted's payload, account.deleted carries no buyer surrogate at
		// all, and RedactChatNotificationsByAccount enumerates every historical
		// buyer_id/store_id surrogate for the account itself, so accountID is
		// both the only identity this event has and all enumeration needs.
		n, err := inboxUC.RedactChatNotificationsByAccount(ctx, payload.AccountID)
		if err != nil {
			logger.Error("[worker] failed to scrub chat notifications for deleted account",
				zap.Int64("account_id", payload.AccountID), zap.Error(err))
			return err
		}
		logger.Info("[worker] scrubbed chat notification inbox PII for deleted account",
			zap.Int64("account_id", payload.AccountID), zap.Int64("rows", n))
		return nil
	})
}
