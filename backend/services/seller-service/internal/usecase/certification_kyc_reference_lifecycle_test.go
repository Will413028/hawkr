package usecase

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	mediapkg "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// issue #2777: KYC secure-file lifecycle. Four gaps, none gated on a
// retention-policy decision:
//  1. finalizeStepDataTokens must fail closed when FinalizeImages itself
//     fails, instead of silently persisting an unconverted (soon-expired)
//     staged token.
//  2. CancelDraft must release the application's IDENTITY_VERIFICATION
//     asset references once the cancel commits — nothing will ever read
//     them again.
//  3. Switching seller type mid-draft wipes StepData without releasing the
//     asset references the old type's StepData held.
//  4. Submit links a second CERTIFICATION reference over the same assets
//     but never retires the original IDENTITY_VERIFICATION one — an
//     unbounded duplicate, not an explicit transition.

func TestCertUC_FinalizeStepDataTokens_FinalizeImagesFailure_FailsClosed(t *testing.T) {
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, tokens []string, refType string) ([]AssetInfo, error) {
			return nil, stderrors.New("media-service unavailable")
		},
	}
	uc := &CertificationUseCase{mediaClient: media}

	_, err := uc.finalizeStepDataTokens(context.Background(), `{"bank_book_staged_token":"tok-bank"}`, "42")
	if err == nil {
		t.Fatal("expected error when FinalizeImages fails, got nil — staged token would silently persist and expire unreadable")
	}
}

func TestCertUC_CancelDraft_ReleasesIdentityVerificationReferences(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusDraft, CreatedAt: now, UpdatedAt: now,
				StepData: []byte(`{"bank_book_asset_id":"asset-bank"}`),
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	var releasedRefType string
	var releasedRefIDs, releasedKeep []string
	media := &mockMediaClient{
		ReleaseReferencesFn: func(_ context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
			releasedRefType, releasedRefIDs, releasedKeep = refType, refIDs, keepAssetIDs
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: media}
	if err := uc.CancelDraft(context.Background(), 7, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if releasedRefType != mediapkg.RefTypeIdentityVerification {
		t.Errorf("release refType = %q, want IDENTITY_VERIFICATION", releasedRefType)
	}
	if len(releasedRefIDs) != 1 || releasedRefIDs[0] != "7" {
		t.Errorf("release refIDs = %v, want [\"7\"]", releasedRefIDs)
	}
	if releasedKeep != nil {
		t.Errorf("release keepAssetIDs = %v, want nil (release everything — draft is cancelled)", releasedKeep)
	}
}

func TestCertUC_CreateDraft_SwitchType_ReleasesOldIdentityVerificationReferences(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 22, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
		CurrentStep: 3, TotalSteps: 9,
		StepData:  []byte(`{"id_card_front_asset_id":"asset-front","bank_book_asset_id":"asset-bank"}`),
		CreatedAt: now, UpdatedAt: now,
	}
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	var released bool
	var releasedRefType string
	var releasedRefIDs []string
	media := &mockMediaClient{
		ReleaseReferencesFn: func(_ context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
			released = true
			releasedRefType, releasedRefIDs = refType, refIDs
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: media}
	if _, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !released {
		t.Fatal("expected ReleaseReferences to be called after StepData is wiped by the type switch — old KYC images are now unreferenced but never released")
	}
	if releasedRefType != mediapkg.RefTypeIdentityVerification {
		t.Errorf("release refType = %q, want IDENTITY_VERIFICATION", releasedRefType)
	}
	if len(releasedRefIDs) != 1 || releasedRefIDs[0] != "22" {
		t.Errorf("release refIDs = %v, want [\"22\"]", releasedRefIDs)
	}
}

func TestCertUC_CreateDraft_SwitchType_NoStepData_DoesNotCallRelease(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 23, AccountID: 101, Status: domain.CertStatusDraft, SellerType: "business",
		CreatedAt: now, UpdatedAt: now,
	}
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	released := false
	media := &mockMediaClient{
		ReleaseReferencesFn: func(_ context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
			released = true
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: media}
	if _, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 101, SellerType: "individual"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if released {
		t.Error("ReleaseReferences should not be called when there was no StepData to release — nothing was ever linked")
	}
}

func TestSubmit_ReleasesIdentityVerificationReferencesAfterCertificationLink(t *testing.T) {
	now := time.Now()
	certRepo := newIndividualDraftApp(now)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: accountID, StoreName: "測試店", Description: "測試",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號",
				PostalCode: "110", ContactPhone: "0912345678", WebsiteURL: "",
			}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return true, nil }}

	var linkedRefType string
	var releasedRefType string
	var releasedRefIDs []string
	media := &mockMediaClient{
		LinkImagesFn: func(_ context.Context, refType, refID string, assetIDs []string) error {
			linkedRefType = refType
			return nil
		},
		ReleaseReferencesFn: func(_ context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
			releasedRefType, releasedRefIDs = refType, refIDs
			return nil
		},
	}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: media, txManager: &mockTxManager{}, socialClient: social,
	}
	if err := uc.Submit(context.Background(), validIndividualSubmitReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if linkedRefType != mediapkg.RefTypeCertification {
		t.Fatalf("precondition failed: LinkImages refType = %q, want CERTIFICATION", linkedRefType)
	}
	if releasedRefType != mediapkg.RefTypeIdentityVerification {
		t.Errorf("release refType = %q, want IDENTITY_VERIFICATION — the old reference must be retired once CERTIFICATION takes over, not kept forever", releasedRefType)
	}
	if len(releasedRefIDs) != 1 || releasedRefIDs[0] != "1" {
		t.Errorf("release refIDs = %v, want [\"1\"] (application id)", releasedRefIDs)
	}
}
