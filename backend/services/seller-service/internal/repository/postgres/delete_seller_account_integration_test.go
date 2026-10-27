//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/stretchr/testify/require"
)

// fakeOrderCount is a stub that always reports 2 active orders (triggers override path).
type fakeOrderCount struct{}

func (f *fakeOrderCount) GetActiveMerchantOrderCount(_ context.Context, _ int64) (int, error) {
	return 2, nil
}

// fakeAccountRoleRemover is a stub that no-ops RemoveSellerRole.
type fakeAccountRoleRemover struct{}

func (f *fakeAccountRoleRemover) RemoveSellerRole(_ context.Context, _ int64, _ rolestate.OtherRoleState, _ int64) error {
	return nil
}

// fakeBuyerClient is a stub that reports no active buyer (OtherRoleAbsent),
// preserving these tests' pre-existing "always cascades if last role" intent.
type fakeBuyerClient struct{}

func (f *fakeBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

func (f *fakeBuyerClient) GetSubscriberCount(_ context.Context, _ int64) (int64, error) {
	panic("unused")
}

func (f *fakeBuyerClient) BatchGetSubscriberCounts(_ context.Context, _ []int64) (map[int64]int64, error) {
	panic("unused")
}

func TestDeleteSellerAccount_AdminOverride_WritesAuditAndPersistsDeleted(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	auditRepo := postgres.NewSellerDeletionAuditRepository(pool)
	outboxRepo := postgres.NewSellerDeletedOutboxRepository(pool)
	txm := database.NewTxManager(pool)

	// Seed an active seller row.
	now := time.Now()
	const accountID int64 = 2613000001

	require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreName:  "Integration Test Store",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	// Construct usecase with real DB repos and stubs for external clients.
	fixedTSID := func() int64 { return 111222333 }
	uc := usecase.NewDeleteSellerAccountUseCase(
		sellerRepo,
		&fakeOrderCount{},
		&fakeAccountRoleRemover{},
		&fakeBuyerClient{},
		auditRepo,
		outboxRepo,
		fixedTSID,
		txm,
		nil,
	)

	err := uc.Execute(ctx, usecase.DeleteSellerAccountInput{
		AccountID:            accountID,
		IsAdmin:              true,
		OperatorID:           42,
		OverrideActiveOrders: true,
		Reason:               "fraud",
	})
	require.NoError(t, err)

	// Assert audit row was written.
	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM seller_deletion_audit WHERE account_id=$1", accountID).Scan(&auditCount))
	require.Equal(t, 1, auditCount, "expected one seller_deletion_audit row")

	var operatorID int64
	var activeOrderCount int
	var reason string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT operator_id, active_order_count, reason FROM seller_deletion_audit WHERE account_id=$1", accountID).
		Scan(&operatorID, &activeOrderCount, &reason))
	require.Equal(t, int64(42), operatorID, "operator_id must match")
	require.Equal(t, 2, activeOrderCount, "active_order_count must match stub return value")
	require.Equal(t, "fraud", reason, "reason must match input")

	// Assert seller status and deleted_at.
	var status string
	var deletedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT status, deleted_at FROM sellers WHERE account_id=$1", accountID).
		Scan(&status, &deletedAt))
	require.Equal(t, "closed", status, "seller status must be 'closed'")
	require.NotNil(t, deletedAt, "deleted_at must not be NULL")

	// Assert outbox row committed atomically with sellers.status=closed.
	var outboxStatus string
	var outboxAccountID int64
	var outboxEventType string
	var outboxPayload []byte
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT status, account_id, event_type, payload FROM seller_deleted_outbox WHERE account_id=$1", accountID).
		Scan(&outboxStatus, &outboxAccountID, &outboxEventType, &outboxPayload))
	require.Equal(t, "pending", outboxStatus, "outbox row status must be 'pending'")
	require.Equal(t, accountID, outboxAccountID, "outbox account_id must match")
	require.Equal(t, domain.SellerLifecycleEventDeleted, outboxEventType, "outbox event_type must be 'seller.deleted'")

	var payload domain.SellerLifecyclePayload
	require.NoError(t, json.Unmarshal(outboxPayload, &payload))
	require.Equal(t, accountID, payload.StoreID,
		"payload.storeId must equal the seeded surrogate (enqueue-time snapshot, grandfathered account_id==store_id)")
}

// TestDeleteSellerAccount_TxFailure_NoOutboxRow asserts that when the deletion tx fails
// (seller row does not exist → GetByAccountIDForUpdate errors), no seller_deleted_outbox
// row is written. This demonstrates the atomic guarantee: outbox and sellers.status=closed
// commit together or not at all.
func TestDeleteSellerAccount_TxFailure_NoOutboxRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	auditRepo := postgres.NewSellerDeletionAuditRepository(pool)
	outboxRepo := postgres.NewSellerDeletedOutboxRepository(pool)
	txm := database.NewTxManager(pool)
	const nonExistentAccountID int64 = 9999999999

	fixedTSID := func() int64 { return 999888777 }
	uc := usecase.NewDeleteSellerAccountUseCase(
		sellerRepo,
		&fakeOrderCount{},
		&fakeAccountRoleRemover{},
		&fakeBuyerClient{},
		auditRepo,
		outboxRepo,
		fixedTSID,
		txm,
		nil,
	)

	err := uc.Execute(ctx, usecase.DeleteSellerAccountInput{AccountID: nonExistentAccountID})
	require.Error(t, err, "expected error for non-existent seller")

	// Assert NO outbox row was written (tx failed, rolled back).
	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM seller_deleted_outbox WHERE account_id=$1", nonExistentAccountID).Scan(&outboxCount))
	require.Equal(t, 0, outboxCount, "expected NO seller_deleted_outbox row when tx fails")
}
