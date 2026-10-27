//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// TestApplyOutcome_OutboxInsertFailure_RollsBackCertUpdate proves the
// invariant spec §4 exists for: if the outbox insert fails, the paired
// certRepo.UpdateApplication in the SAME transaction must also roll back —
// otherwise a lost outbox row would silently strand the cert in its new
// status with no notification ever sent.
//
// The failure is forced by pre-seeding a merchant_verification_outbox row
// with the exact id the fixed tsid stub will mint next, so the real INSERT
// inside transitionCertFSM's WithTx hits a genuine primary-key violation.
func TestApplyOutcome_OutboxInsertFailure_RollsBackCertUpdate(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPayuniIntegrationDB(ctx, t)
	defer cleanup()

	const accountID, applicationID int64 = 100, 10
	const collidingOutboxID int64 = 555

	_, err := pool.Exec(ctx,
		`INSERT INTO certification_applications (id, account_id, application_no, status, seller_type, created_at, updated_at)
		 VALUES ($1, $2, 'CERT-TEST-1', 'forwarded', 'individual', now(), now())`,
		applicationID, accountID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO merchant_verification_outbox (id, account_id, event_type, payload, status, attempt_count, next_attempt_at, created_at)
		 VALUES ($1, $2, 'approved', '{}'::jsonb, 'pending', 0, now(), now())`,
		collidingOutboxID, accountID)
	require.NoError(t, err)

	// id_card_no/bank_account/step_data are all NULL on the row inserted
	// above, so any deterministic non-empty cipher key round-trips fine —
	// this test never touches those fields.
	enc, err := crypto.New("integration-test-master-key")
	require.NoError(t, err)

	uc := &PayuniUseCase{
		certRepo:   postgres.NewCertificationRepository(pool, enc),
		payuniRepo: postgres.NewPayuniRepository(pool),
		outboxRepo: postgres.NewMerchantVerificationOutboxRepository(pool),
		txManager:  database.NewTxManager(pool),
		tsid:       func() int64 { return collidingOutboxID },
	}
	info := &domain.PayuniRegistration{
		AccountID: accountID, ApplicationID: applicationID,
		PayuniMerID: "HAWK100",
	}

	ok := uc.ApplyObservation(ctx, info, obsOf("1", "1"))
	require.False(t, ok, "ApplyObservation must report failure when the outbox insert cannot commit")

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM certification_applications WHERE id = $1`, applicationID).Scan(&status))
	require.Equal(t, "forwarded", status, "cert status must have rolled back, not advanced to approved")

	var outboxCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM merchant_verification_outbox WHERE account_id = $1`, accountID).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount, "only the pre-seeded colliding row may exist — the real insert must not have landed")
}
