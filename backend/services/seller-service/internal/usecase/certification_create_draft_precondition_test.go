package usecase

import (
	"context"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// ─────────────────────────────────────────────────────────────────────────────
// CreateDraft precondition gate (#1893)
//
// CreateDraft previously had NO precondition check at all — a caller could
// bypass the website gate entirely by hitting POST /app/certifications
// directly. These tests apply the same domain.MeetsCertificationPrecondition
// gate used by Submit.
// ─────────────────────────────────────────────────────────────────────────────

func noExistingCertificationApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	return nil, pkgerrors.NewNotFound("no current application")
}

// satisfiesCertificationPrecondition is a mockSellerRepo.GetByAccountIDFn for
// CreateDraft tests that don't care about the #1893 gate itself — it always
// returns a seller with description+website set, so the gate passes without
// requiring a socialClient mock too.
func satisfiesCertificationPrecondition(ctx context.Context, accountID int64) (*domain.Seller, error) {
	return &domain.Seller{
		AccountID: accountID, StoreID: accountID,
		Description: "自我介紹", WebsiteURL: "https://my-shop.example.com",
	}, nil
}

func TestCreateDraft_NoPostNoDescriptionNoWebsite_Rejects(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: noExistingCertificationApplication,
		CreateApplicationFn:     func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Description: "", WebsiteURL: ""}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return false, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, socialClient: social,
	}
	_, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})
	if err == nil {
		t.Fatal("expected error when seller has no post, no description, no website")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
}

// TestCreateDraft_WebsiteOnly_NoDescription_NoPost_Rejects reproduces #1893's
// actual bypass: today CreateDraft has no gate at all, so a caller can
// create a draft with just a website URL — even a blank self-introduction.
func TestCreateDraft_WebsiteOnly_NoDescription_NoPost_Rejects(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: noExistingCertificationApplication,
		CreateApplicationFn:     func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Description: "", WebsiteURL: "https://my-shop.example.com"}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return false, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, socialClient: social,
	}
	_, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})
	if err == nil {
		t.Fatal("expected error when seller has website but no description and no post")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
}

func TestCreateDraft_HasPost_Allows(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: noExistingCertificationApplication,
		CreateApplicationFn:     func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Description: "", WebsiteURL: ""}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return true, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, socialClient: social,
	}
	if _, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDraft_HasDescriptionAndWebsite_NoPost_Allows(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: noExistingCertificationApplication,
		CreateApplicationFn:     func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Description: "自我介紹", WebsiteURL: "https://my-shop.example.com"}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) {
		t.Fatal("socialClient must not be called when description+website are both set (short-circuit)")
		return false, nil
	}}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, socialClient: social,
	}
	if _, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreateDraft_ExistingDraft_SkipsPreconditionRecheck covers the scope
// boundary from the design doc: returning an already-existing draft/submitted
// application does not create anything new, so it must not re-run the
// precondition gate (a seller mid-flow shouldn't get locked out of editing
// their own in-progress draft if their post/website state later changes).
func TestCreateDraft_ExistingDraft_SkipsPreconditionRecheck(t *testing.T) {
	existing := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
	}
	existing.InitFSM()
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			t.Fatal("sellerRepo must not be consulted when an existing draft/submitted application is simply returned")
			return nil, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) {
		t.Fatal("socialClient must not be consulted when an existing draft/submitted application is simply returned")
		return false, nil
	}}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, socialClient: social,
	}
	if _, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
