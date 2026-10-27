//go:build integration

package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func TestGetCertHolderNames_RequiresForwardedAndPayuniActive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	sellerRepo := postgres.NewSellerRepository(pool)
	certRepo := postgres.NewCertificationRepository(pool, testCipher(t))
	now := time.Now()

	cases := []struct {
		name       string
		accountID  int64
		storeID    int64
		certID     int64
		certStatus domain.CertificationStatus
		payuni     *domain.PayuniStatus
		wantFound  bool
	}{
		{"forwarded + payuni active", 2617100001, 7617100001000000001, 3617100001, domain.CertStatusForwarded, ptrPayuni(domain.PayuniStatusActive), true},
		{"forwarded + payuni pending", 2617100002, 7617100002000000002, 3617100002, domain.CertStatusForwarded, ptrPayuni(domain.PayuniStatusPending), false},
		{"forwarded + payuni failed", 2617100003, 7617100003000000003, 3617100003, domain.CertStatusForwarded, ptrPayuni(domain.PayuniStatusFailed), false},
		{"forwarded without registration", 2617100004, 7617100004000000004, 3617100004, domain.CertStatusForwarded, nil, false},
		{"submitted", 2617100005, 7617100005000000005, 3617100005, domain.CertStatusSubmitted, ptrPayuni(domain.PayuniStatusActive), false},
	}
	for _, tc := range cases {
		require.NoError(t, sellerRepo.Create(ctx, &domain.Seller{
			AccountID: tc.accountID, StoreID: tc.storeID, StoreCode: tc.accountID,
			StoreName: "Store", Status: domain.SellerStatusActive, SellerType: domain.SellerTypeIndividual,
			CreatedAt: now, UpdatedAt: now,
		}))
		require.NoError(t, certRepo.CreateApplication(ctx, &domain.CertificationApplication{
			ID: tc.certID, AccountID: tc.accountID,
			ApplicationNo: "CERT-HOLDER-" + strconv.FormatInt(tc.certID, 10),
			Status:        tc.certStatus, SellerType: "individual",
			RealName:  "持有人" + strconv.FormatInt(tc.certID, 10),
			CreatedAt: now, UpdatedAt: now,
		}))
		require.NoError(t, sellerRepo.UpdateCurrentCertificationID(ctx, tc.accountID, tc.certID))
		if tc.payuni != nil {
			seedPayuniActive(ctx, t, pool, tc.accountID, tc.storeID, tc.certID, *tc.payuni)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, found, err := certRepo.GetCertHolderNames(ctx, tc.storeID)
			require.NoError(t, err)
			require.Equal(t, tc.wantFound, found)
			if tc.wantFound {
				require.Equal(t, "持有人"+strconv.FormatInt(tc.certID, 10), names.RealName)
			}
		})
	}
}

func TestCertificationStatusCheck_RejectsRetiredMirrorStates(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	for _, status := range []string{"approved", "pending_info"} {
		t.Run(status, func(t *testing.T) {
			_, err := pool.Exec(ctx,
				`INSERT INTO certification_applications (id, account_id, application_no, status, seller_type)
				 VALUES ($1, $2, $3, $4, 'individual')`,
				3617200000+int64(len(status)), int64(2617200000+len(status)), "CERT-CHECK-"+status, status)
			require.Error(t, err)
			require.Contains(t, err.Error(), "certification_applications_status_check")
		})
	}
}

func ptrPayuni(s domain.PayuniStatus) *domain.PayuniStatus { return &s }
