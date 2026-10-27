package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// tierRepo is the narrow, handler-owned port onto
// repository.RankingFactsRepository (Task 11) — only the one method this
// handler calls, following the same small consumer-owned interface pattern
// as orderCompletedRepo (Task 12).
type tierRepo interface {
	UpsertTier(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error
}

// sellerMembershipChangedHandler maintains seller_ranking_facts.tier_rank and
// monthly_order_cap from seller.membership_changed events. Unlike
// order.completed (an increment), this is naturally idempotent — UpsertTier
// SETs the latest snapshotted values, so redelivery needs no dedup table. A
// parse failure (bad JSON, malformed store_id, malformed effective_at) still
// returns an error so ResilientConsumer retries/DLQs it — never silently
// drops it.
//
// event.EffectiveAt is parsed and passed through so UpsertTier's WHERE guard
// can reject a stale redelivery: RabbitMQ retry/requeue can let an older
// event finish processing after a newer one already landed (e.g. the newer
// event acked first, the older nacked+requeued arrives after) — without the
// guard, "後到覆蓋前到" would silently revert a seller's tier to a stale
// value. This is a Go→Go event (producer: seller-service, this repo,
// EffectiveAt.UTC().Format(time.RFC3339)) — unlike order.completed's Java
// producer, there is no wire-shape mismatch here, so a plain time.RFC3339
// parse is correct and sufficient.
func sellerMembershipChangedHandler(repo tierRepo) pkgmq.HandlerFunc {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event pkgmq.SellerMembershipChangedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return fmt.Errorf("unmarshal seller.membership_changed: %w", err)
		}
		storeIDInt, err := strconv.ParseInt(event.StoreID, 10, 64)
		if err != nil {
			return fmt.Errorf("seller.membership_changed event %s: invalid store_id %q: %w", event.EventID, event.StoreID, err)
		}
		effectiveAt, err := time.Parse(time.RFC3339, event.EffectiveAt)
		if err != nil {
			return fmt.Errorf("seller.membership_changed event %s: invalid effective_at %q: %w", event.EventID, event.EffectiveAt, err)
		}
		return repo.UpsertTier(ctx, identity.StoreID(storeIDInt), event.TierRank, event.MonthlyOrderCap, effectiveAt)
	}
}
