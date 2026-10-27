//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// testSellerMembershipID returns a monotonically-increasing id generator,
// standing in for the production TSID generator. AssignPlan is called more
// than once per test against the same store_id, so a fixed id would collide
// on seller_membership_subscriptions' primary key.
func testSellerMembershipID() func() int64 {
	var n int64 = 800_000_000
	return func() int64 {
		n++
		return n
	}
}

func TestSellerMembershipRepository_AssignPlan_SupersedesPreviousActive(t *testing.T) {
	pool, cleanup := setupTestDB(context.Background(), t)
	defer cleanup()
	txManager := database.NewTxManager(pool)
	repo := postgres.NewSellerMembershipRepository(pool, txManager, testSellerMembershipID())
	ctx := context.Background()

	storeID := int64(900001)
	now := time.Now().UTC()

	first, err := repo.AssignPlan(ctx, domain.AssignSellerMembershipInput{
		StoreID: storeID, PlanCode: "plus", EffectiveAt: now, Source: "admin", RequestID: "req-1",
	})
	require.NoError(t, err)
	require.Equal(t, "active", first.Status)

	second, err := repo.AssignPlan(ctx, domain.AssignSellerMembershipInput{
		StoreID: storeID, PlanCode: "pro", EffectiveAt: now, Source: "admin", RequestID: "req-2",
	})
	require.NoError(t, err)
	require.Equal(t, "active", second.Status)

	active, err := repo.GetActiveSubscription(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, "pro", active.PlanCode, "second assign must supersede the first")
}

func TestSellerMembershipRepository_ListPlans_ReturnsSeededCatalogOrderedByTierRank(t *testing.T) {
	pool, cleanup := setupTestDB(context.Background(), t)
	defer cleanup()
	repo := postgres.NewSellerMembershipRepository(pool, database.NewTxManager(pool), testSellerMembershipID())

	plans, err := repo.ListPlans(context.Background())
	require.NoError(t, err)
	require.Len(t, plans, 3, "migration seeds exactly free/plus/pro")
	require.Equal(t, "free", plans[0].PlanCode)
	require.Equal(t, 0, plans[0].TierRank)
	require.Equal(t, "plus", plans[1].PlanCode)
	require.Equal(t, 1, plans[1].TierRank)
	require.Equal(t, "pro", plans[2].PlanCode)
	require.Equal(t, 2, plans[2].TierRank)
	require.Equal(t, 1000, plans[2].MonthlyOrderCap)
	require.False(t, plans[2].CreatedAt.IsZero())
	require.False(t, plans[2].UpdatedAt.IsZero())
}

func TestSellerMembershipRepository_UpdatePlan_PartialUpdateOnlyChangesGivenFields(t *testing.T) {
	pool, cleanup := setupTestDB(context.Background(), t)
	defer cleanup()
	repo := postgres.NewSellerMembershipRepository(pool, database.NewTxManager(pool), testSellerMembershipID())
	ctx := context.Background()

	before, err := repo.GetPlanByCode(ctx, "plus")
	require.NoError(t, err)

	newCap := 250
	updated, err := repo.UpdatePlan(ctx, domain.UpdateSellerMembershipPlanInput{
		PlanCode: "plus", MonthlyOrderCap: &newCap,
	})
	require.NoError(t, err)
	require.Equal(t, 250, updated.MonthlyOrderCap, "given field must change")
	require.Equal(t, before.MonthlyPriceTWD, updated.MonthlyPriceTWD, "unset field must keep its stored value, not be zeroed")
	require.Equal(t, before.IsPurchasable, updated.IsPurchasable)
	require.Equal(t, before.TierRank, updated.TierRank, "tier_rank has no write path here at all — must be provably unchanged")
	require.Equal(t, before.DisplayName, updated.DisplayName)
	require.True(t, updated.UpdatedAt.After(before.UpdatedAt) || updated.UpdatedAt.Equal(before.UpdatedAt))

	refetched, err := repo.GetPlanByCode(ctx, "plus")
	require.NoError(t, err)
	require.Equal(t, 250, refetched.MonthlyOrderCap, "change must be durably persisted, not just returned by RETURNING")
}

func TestSellerMembershipRepository_UpdatePlan_UnknownPlanCode_ReturnsErrNoRows(t *testing.T) {
	pool, cleanup := setupTestDB(context.Background(), t)
	defer cleanup()
	repo := postgres.NewSellerMembershipRepository(pool, database.NewTxManager(pool), testSellerMembershipID())

	newCap := 500
	_, err := repo.UpdatePlan(context.Background(), domain.UpdateSellerMembershipPlanInput{
		PlanCode: "does-not-exist", MonthlyOrderCap: &newCap,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestSellerMembershipRepository_GetActiveSubscription_NoRow_ReturnsErrNoActiveSellerMembership(t *testing.T) {
	pool, cleanup := setupTestDB(context.Background(), t)
	defer cleanup()
	txManager := database.NewTxManager(pool)
	repo := postgres.NewSellerMembershipRepository(pool, txManager, testSellerMembershipID())

	_, err := repo.GetActiveSubscription(context.Background(), 999999999)
	require.ErrorIs(t, err, domain.ErrNoActiveSellerMembership)
}

// TestAssignMembership_NewAssignment_EnqueuesLifecycleOutbox proves
// SellerMembershipUseCase.AssignMembership's one-transaction composition
// (Task 3) is genuinely atomic against a real Postgres instance, not a mock:
// the subscription write (via AssignPlanTx) and the seller_deleted_outbox
// insert both go through the same *database.TxManager-backed transaction, so
// a successful AssignMembership call must leave both rows committed
// together. This deliberately does not exercise AssignPlan (Task 2's
// all-in-one method, which opens its own nested transaction) — the usecase
// under test never calls it.
func TestAssignMembership_NewAssignment_EnqueuesLifecycleOutbox(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	txManager := database.NewTxManager(pool)
	membershipRepo := postgres.NewSellerMembershipRepository(pool, txManager, testSellerMembershipID())
	outboxRepo := postgres.NewSellerDeletedOutboxRepository(pool)

	// Independent counter from testSellerMembershipID(): the usecase's newID
	// mints ids for seller_deleted_outbox rows, a different table from
	// seller_membership_subscriptions, so there is no shared-PK collision risk
	// between the two counters even though both start fresh per test.
	var outboxSeq int64 = 850_000_000
	newOutboxID := func() int64 {
		outboxSeq++
		return outboxSeq
	}

	uc := usecase.NewSellerMembershipUseCase(membershipRepo, outboxRepo, txManager, newOutboxID)

	const storeID int64 = 900101
	const grantedByAccountID int64 = 700202

	view, err := uc.AssignMembership(ctx, usecase.AssignMembershipRequest{
		StoreID:            storeID,
		PlanCode:           "pro",
		Reason:             "integration test upgrade",
		GrantedByAccountID: grantedByAccountID,
		RequestID:          "req-integration-1",
	})
	require.NoError(t, err)
	require.Equal(t, "pro", view.PlanCode)
	require.Equal(t, storeID, view.StoreID)

	// Assert the subscription row was actually committed.
	active, err := membershipRepo.GetActiveSubscription(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, "pro", active.PlanCode)
	require.Equal(t, "active", active.Status)

	// Assert the lifecycle outbox row was committed in the SAME transaction —
	// this is the atomicity proof: AssignMembership returned successfully, so
	// both writes must be visible now.
	var outboxStatus, outboxEventType string
	var outboxPayload []byte
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT status, event_type, payload FROM seller_deleted_outbox WHERE account_id=$1", grantedByAccountID).
		Scan(&outboxStatus, &outboxEventType, &outboxPayload))
	require.Equal(t, "pending", outboxStatus, "outbox row status must be 'pending'")
	require.Equal(t, domain.SellerLifecycleEventMembershipChanged, outboxEventType)

	var payload domain.SellerMembershipOutboxPayload
	require.NoError(t, json.Unmarshal(outboxPayload, &payload))
	require.Equal(t, storeID, payload.StoreID)
	require.Equal(t, "pro", payload.PlanCode)
	require.Equal(t, 2, payload.TierRank, "pro plan's seeded tier_rank")
	require.Equal(t, 1000, payload.MonthlyOrderCap, "pro plan's seeded monthly_order_cap")
}

// TestAssignMembership_SameRequestID_DoesNotDuplicateOutboxRow is the
// idempotency counterpart at the integration layer: a second AssignMembership
// call with the same RequestID must not insert a second subscription row nor
// a second outbox row, proving idempotency holds against real Postgres state
// (not just the mock-based unit test's assertion that AssignPlanTx wasn't
// called again).
func TestAssignMembership_SameRequestID_DoesNotDuplicateOutboxRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	txManager := database.NewTxManager(pool)
	membershipRepo := postgres.NewSellerMembershipRepository(pool, txManager, testSellerMembershipID())
	outboxRepo := postgres.NewSellerDeletedOutboxRepository(pool)

	var outboxSeq int64 = 860_000_000
	newOutboxID := func() int64 {
		outboxSeq++
		return outboxSeq
	}

	uc := usecase.NewSellerMembershipUseCase(membershipRepo, outboxRepo, txManager, newOutboxID)

	const storeID int64 = 900102
	const grantedByAccountID int64 = 700203
	req := usecase.AssignMembershipRequest{
		StoreID:            storeID,
		PlanCode:           "plus",
		Reason:             "integration test idempotency",
		GrantedByAccountID: grantedByAccountID,
		RequestID:          "req-integration-dup",
	}

	first, err := uc.AssignMembership(ctx, req)
	require.NoError(t, err)

	second, err := uc.AssignMembership(ctx, req)
	require.NoError(t, err)
	require.Equal(t, first, second, "duplicate call must return the same view without reassigning")

	var subCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM seller_membership_subscriptions WHERE store_id=$1", storeID).Scan(&subCount))
	require.Equal(t, 1, subCount, "duplicate RequestID must not create a second subscription row")

	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM seller_deleted_outbox WHERE account_id=$1 AND event_type=$2",
		grantedByAccountID, domain.SellerLifecycleEventMembershipChanged).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount, "duplicate RequestID must not enqueue a second outbox row")
}
