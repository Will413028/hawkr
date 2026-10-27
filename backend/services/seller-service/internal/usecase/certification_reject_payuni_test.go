package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// TestReject_OutboxPayloadCarriesSellerStoreID verifies the admin-initiated
// Reject path (issue #2707 followup, Phase A) resolves the seller's current
// store_id via sellerRepo.GetByAccountIDForUpdate and threads it into the
// rejected outbox payload, distinct from accountID so a swapped argument
// would be caught.
func TestReject_OutboxPayloadCarriesSellerStoreID(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, CreatedAt: now, UpdatedAt: now}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}

	const wantStoreID int64 = 7300000000000000456
	var gotAccountID int64
	sellerRepo := &mockSellerRepo{
		GetByAccountIDForUpdateFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			gotAccountID = accountID
			return &domain.Seller{AccountID: accountID, StoreID: wantStoreID}, nil
		},
		// GetByAccountIDFn is deliberately left unset (nil) so this test fails
		// loudly (nil dereference) if Reject regresses to calling the plain
		// GetByAccountID instead of the ForUpdate variant.
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		certRepo: certRepo, sellerRepo: sellerRepo, txManager: &mockTxManager{},
		outboxRepo: outboxRepo, tsid: func() int64 { return 1 },
	}

	req := &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin", RejectionReason: "文件不齊全"}
	if err := uc.Reject(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAccountID != 100 {
		t.Errorf("GetByAccountIDForUpdate called with accountID=%d, want 100", gotAccountID)
	}
	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("outbox rows inserted = %d, want 1", len(outboxRepo.Inserted))
	}
	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(outboxRepo.Inserted[0].Payload, &event); err != nil {
		t.Fatalf("outbox payload did not unmarshal: %v", err)
	}
	if event.StoreID != wantStoreID {
		t.Errorf("outbox payload StoreID = %d, want %d", event.StoreID, wantStoreID)
	}
	if event.MerchantID != 100 {
		t.Errorf("outbox payload MerchantID = %d, want 100 (account_id)", event.MerchantID)
	}
}

// TestReject_SoftDeletedSeller_StillResolvesStoreID locks in the gotcha this
// change depends on: Reject must resolve the seller via the UNFILTERED
// GetByAccountIDForUpdate, not the deleted_at-filtered GetByAccountID.
// Reject previously never touched sellerRepo at all, so it succeeded
// unconditionally regardless of the seller's deletion state (e.g. an admin
// rejecting a pending application after the seller self-deleted mid-review).
// A regression to the filtered GetByAccountID would introduce a NEW failure
// mode here — Reject starting to 404 on a soft-deleted seller it previously
// always accepted.
//
// The two sellerRepo functions are set to DIFFERENT fakes (unlike most tests
// in this package, where GetByAccountIDForUpdate falls back to
// GetByAccountIDFn) precisely so this test cannot pass by accident: only a
// call to the true GetByAccountIDForUpdate reaches the seller with
// DeletedAt set; GetByAccountIDFn simulates the filtered query's
// deleted_at IS NULL behavior by returning NotFound.
func TestReject_SoftDeletedSeller_StillResolvesStoreID(t *testing.T) {
	now := time.Now()
	deletedAt := now.Add(-time.Hour)
	app := &domain.CertificationApplication{ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, CreatedAt: now, UpdatedAt: now}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}

	const wantStoreID int64 = 7300000000000000789
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			// Simulates the real GetByAccountID's `deleted_at IS NULL` filter:
			// a soft-deleted seller is NotFound through this method.
			return nil, pkgerrors.NewNotFound("seller not found")
		},
		GetByAccountIDForUpdateFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			// Simulates the real GetByAccountIDForUpdate's unfiltered read: the
			// soft-deleted seller still resolves.
			return &domain.Seller{AccountID: accountID, StoreID: wantStoreID, DeletedAt: &deletedAt}, nil
		},
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		certRepo: certRepo, sellerRepo: sellerRepo, txManager: &mockTxManager{},
		outboxRepo: outboxRepo, tsid: func() int64 { return 1 },
	}

	req := &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin", RejectionReason: "文件不齊全"}
	if err := uc.Reject(context.Background(), req); err != nil {
		t.Fatalf("Reject of an application whose seller has been soft-deleted must still succeed, got: %v", err)
	}

	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("outbox rows inserted = %d, want 1", len(outboxRepo.Inserted))
	}
	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(outboxRepo.Inserted[0].Payload, &event); err != nil {
		t.Fatalf("outbox payload did not unmarshal: %v", err)
	}
	if event.StoreID != wantStoreID {
		t.Errorf("outbox payload StoreID = %d, want %d (resolved via the unfiltered GetByAccountIDForUpdate)", event.StoreID, wantStoreID)
	}
}
