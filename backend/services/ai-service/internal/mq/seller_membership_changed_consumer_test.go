package mq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// stubTierRepo satisfies only the narrow tierRepo interface the handler
// actually depends on (see seller_membership_changed_consumer.go).
type stubTierRepo struct {
	UpsertTierFn func(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error
}

func (s *stubTierRepo) UpsertTier(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error {
	return s.UpsertTierFn(ctx, storeID, tierRank, monthlyOrderCap, effectiveAt)
}

func TestSellerMembershipChangedHandler_UpsertsTierAndCap(t *testing.T) {
	var gotStoreID identity.StoreID
	var gotTier, gotCap int
	var gotEffectiveAt time.Time
	repo := &stubTierRepo{
		UpsertTierFn: func(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error {
			gotStoreID, gotTier, gotCap, gotEffectiveAt = storeID, tierRank, monthlyOrderCap, effectiveAt
			return nil
		},
	}
	handler := sellerMembershipChangedHandler(repo)

	wantEffectiveAt := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	event := pkgmq.SellerMembershipChangedEvent{
		BaseEvent: pkgmq.BaseEvent{EventID: "evt-1"},
		StoreID:   "700001", PlanCode: "pro", TierRank: 2, MonthlyOrderCap: 1000,
		EffectiveAt: wantEffectiveAt.Format(time.RFC3339),
	}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeySellerMembershipChanged, body)
	require.NoError(t, err)
	require.Equal(t, identity.StoreID(700001), gotStoreID)
	require.Equal(t, 2, gotTier)
	require.Equal(t, 1000, gotCap)
	require.True(t, wantEffectiveAt.Equal(gotEffectiveAt))
}

func TestSellerMembershipChangedHandler_InvalidStoreID_ReturnsErrorForRetryNotSilentDrop(t *testing.T) {
	repo := &stubTierRepo{}
	handler := sellerMembershipChangedHandler(repo)

	event := pkgmq.SellerMembershipChangedEvent{
		BaseEvent: pkgmq.BaseEvent{EventID: "evt-2"},
		StoreID:   "not-a-number", PlanCode: "pro", TierRank: 2, MonthlyOrderCap: 1000,
		EffectiveAt: time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeySellerMembershipChanged, body)
	require.Error(t, err, "seller.membership_changed with a malformed store_id must fail loud, not silently no-op")
}

func TestSellerMembershipChangedHandler_MissingEffectiveAt_ReturnsErrorForRetryNotSilentDrop(t *testing.T) {
	repo := &stubTierRepo{}
	handler := sellerMembershipChangedHandler(repo)

	event := pkgmq.SellerMembershipChangedEvent{
		BaseEvent: pkgmq.BaseEvent{EventID: "evt-3"},
		StoreID:   "700001", PlanCode: "pro", TierRank: 2, MonthlyOrderCap: 1000,
		// EffectiveAt left empty — must fail loud (DLQ), never fall back to a
		// zero time.Time that would make this event look infinitely stale
		// (or, worse, always "newest") to the UpsertTier guard.
	}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeySellerMembershipChanged, body)
	require.Error(t, err, "seller.membership_changed with a missing effective_at must fail loud, not silently no-op")
}

func TestSellerMembershipChangedHandler_MalformedEffectiveAt_ReturnsErrorForRetryNotSilentDrop(t *testing.T) {
	repo := &stubTierRepo{}
	handler := sellerMembershipChangedHandler(repo)

	event := pkgmq.SellerMembershipChangedEvent{
		BaseEvent: pkgmq.BaseEvent{EventID: "evt-4"},
		StoreID:   "700001", PlanCode: "pro", TierRank: 2, MonthlyOrderCap: 1000,
		EffectiveAt: "2026/08/29",
	}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeySellerMembershipChanged, body)
	require.Error(t, err, "seller.membership_changed with a malformed effective_at must fail loud, not silently no-op")
}
