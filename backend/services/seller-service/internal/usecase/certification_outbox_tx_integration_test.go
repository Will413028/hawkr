//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// TestReject_OutboxInsertFailure_RollsBackReviewAndStatus proves Reject()'s
// outbox insert shares its transaction with certRepo.UpdateApplication and
// CreateReview: a forced PK collision on the outbox insert must roll back
// both, leaving the application status untouched.
func TestReject_OutboxInsertFailure_RollsBackReviewAndStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	const accountID, applicationID int64 = 100, 20
	const collidingOutboxID int64 = 777
	const storeID int64 = 100

	_, err := pool.Exec(ctx,
		`INSERT INTO certification_applications (id, account_id, application_no, status, seller_type, created_at, updated_at)
		 VALUES ($1, $2, 'CERT-TEST-2', 'submitted', 'individual', now(), now())`,
		applicationID, accountID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO merchant_verification_outbox (id, account_id, event_type, payload, status, attempt_count, next_attempt_at, created_at)
		 VALUES ($1, $2, 'rejected', '{}'::jsonb, 'pending', 0, now(), now())`,
		collidingOutboxID, accountID)
	require.NoError(t, err)

	// Reject resolves the seller's store_id for the outbox payload BEFORE
	// opening the transaction (#2707 followup), and returns "load seller" if
	// that read fails. Without this row the test would still go green — status
	// untouched, no review — but for the wrong reason: it would never reach the
	// outbox insert it exists to exercise. The assertion below pins that.
	_, err = pool.Exec(ctx,
		`INSERT INTO sellers (account_id, store_id, store_code, is_current, store_name, seller_type, created_at, updated_at)
		 VALUES ($1, $2, $2, true, 'Reject Outbox Store', 'individual', now(), now())`,
		accountID, storeID)
	require.NoError(t, err)

	// id_card_no/bank_account/step_data are all NULL on the row inserted
	// above, so any deterministic non-empty cipher key round-trips fine —
	// this test never touches those fields.
	enc, err := crypto.New("integration-test-master-key")
	require.NoError(t, err)

	uc := &CertificationUseCase{
		certRepo:   postgres.NewCertificationRepository(pool, enc),
		sellerRepo: postgres.NewSellerRepository(pool),
		outboxRepo: postgres.NewMerchantVerificationOutboxRepository(pool),
		txManager:  database.NewTxManager(pool),
		tsid:       func() int64 { return collidingOutboxID },
	}

	err = uc.Reject(ctx, &dto.ReviewApplication{ApplicationID: applicationID, ReviewerID: 1, ReviewerName: "Admin", RejectionReason: "test"})
	require.Error(t, err, "Reject must surface the outbox insert failure")
	// The failure has to be the outbox collision, not an earlier bail-out. This
	// test panicked on a nil sellerRepo once Reject started reading the seller
	// (undetected because the integration workflow rarely runs on dev); the next
	// dependency Reject acquires must break this line loudly rather than let the
	// test pass without reaching the transaction.
	require.NotContains(t, err.Error(), "load seller",
		"Reject bailed out before the outbox insert — the test is no longer exercising the shared transaction")

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM certification_applications WHERE id = $1`, applicationID).Scan(&status))
	require.Equal(t, "submitted", status, "status must have rolled back")

	var reviewCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM certification_reviews WHERE application_id = $1`, applicationID).Scan(&reviewCount))
	require.Equal(t, 0, reviewCount, "the review insert in the same tx must also have rolled back")
}
