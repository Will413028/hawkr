//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

// TestUpdateApplication_ReflectsPayuniWebsiteURL is a regression guard for the
// same bug class certification_applications.store_description silently hit:
// UpdateApplication is a full explicit-column UPDATE, and the read side goes
// through a single hand-written mapper (toDomainCertificationApplication) —
// either one forgetting the new column lets the write succeed while the read
// comes back empty with no error anywhere.
func TestUpdateApplication_ReflectsPayuniWebsiteURL(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()
	const appID int64 = 5900100001
	const accountID int64 = 5900100001

	app := &domain.CertificationApplication{
		ID: appID, AccountID: accountID, ApplicationNo: "CERT-INTEG-PAYUNI-URL",
		Status: domain.CertStatusDraft, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, certRepo.CreateApplication(ctx, app))

	loaded, err := certRepo.GetApplicationByID(ctx, appID)
	require.NoError(t, err)
	require.Equal(t, "", loaded.PayuniWebsiteURL, "should be NULL/empty right after create")

	loaded.PayuniWebsiteURL = "https://payuni-override.example.com"
	loaded.UpdatedAt = now
	require.NoError(t, certRepo.UpdateApplication(ctx, loaded))

	reread, err := certRepo.GetApplicationByID(ctx, appID)
	require.NoError(t, err)
	require.Equal(t, "https://payuni-override.example.com", reread.PayuniWebsiteURL,
		"read path must reflect the write — payuni_website_url came back empty")
}
