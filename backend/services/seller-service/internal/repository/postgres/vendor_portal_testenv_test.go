//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

type vendorPortalEnv struct {
	Pool       *pgxpool.Pool
	Repo       domain.VendorPortalRepository
	ProspectID int64
	nextID     int64
	close      func()
}

func (e *vendorPortalEnv) Close() { e.close() }

func newVendorPortalTestEnv(t *testing.T) *vendorPortalEnv {
	t.Helper()
	pool, cleanup := setupTestDB(context.Background(), t)

	env := &vendorPortalEnv{Pool: pool, nextID: 700000, close: cleanup}
	env.Repo = postgres.NewVendorPortalRepository(pool, database.NewTxManager(pool), env.newID)
	env.ProspectID = env.InsertProspect(t, "測試業者")
	return env
}

func (e *vendorPortalEnv) newID() int64 { e.nextID++; return e.nextID }

func (e *vendorPortalEnv) InsertProspect(t *testing.T, name string) int64 {
	t.Helper()
	return e.InsertProspectWithEmail(t, name, "vendor@example.com")
}

func (e *vendorPortalEnv) InsertProspectWithEmail(t *testing.T, name, email string) int64 {
	t.Helper()
	id := e.newID()
	_, err := e.Pool.Exec(context.Background(), `
		INSERT INTO seller_prospects (id, display_name, vendor_type, status, outreach_status, public_email)
		VALUES ($1, $2, 'business', 'ACTIVE', 'NOT_CONTACTED', $3)`, id, name, email)
	require.NoError(t, err)
	return id
}

func (e *vendorPortalEnv) SeedInvite(t *testing.T, rfqRecipientID int64, expiresAt time.Time) (domain.VendorPortalInvite, string) {
	t.Helper()
	inv, secret, err := e.Repo.EnsureInvite(context.Background(), domain.EnsureInviteInput{
		ProspectID: e.ProspectID, RFQRecipientID: rfqRecipientID,
		ExpiresAt: expiresAt.UTC(),
	})
	require.NoError(t, err)
	return inv, secret
}

func (e *vendorPortalEnv) SeedInviteForSameIdentity(t *testing.T, rfqRecipientID int64, expiresAt time.Time) (domain.VendorPortalInvite, string) {
	t.Helper()
	return e.SeedInvite(t, rfqRecipientID, expiresAt)
}

func (e *vendorPortalEnv) RevokeInvite(t *testing.T, rfqRecipientID int64) {
	t.Helper()
	_, err := e.Pool.Exec(context.Background(),
		`UPDATE vendor_portal_invites SET revoked_at = now() WHERE rfq_recipient_id = $1`, rfqRecipientID)
	require.NoError(t, err)
}

func (e *vendorPortalEnv) RevokeIdentity(t *testing.T, identityID int64) {
	t.Helper()
	_, err := e.Pool.Exec(context.Background(),
		`UPDATE vendor_portal_identities SET status = 'REVOKED' WHERE id = $1`, identityID)
	require.NoError(t, err)
}
