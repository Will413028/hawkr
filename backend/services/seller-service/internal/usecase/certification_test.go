package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	mediapkg "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

func TestCertUC_CreateDraft_New(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
		CreateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			app.ID = 1
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{GetByAccountIDFn: satisfiesCertificationPrecondition}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "business"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Status != "draft" {
		t.Errorf("Status = %q, want %q", resp.Status, "draft")
	}
	if resp.SellerType != "business" {
		t.Errorf("SellerType = %q, want %q", resp.SellerType, "business")
	}
}

func TestCertUC_CreateDraft_BusinessTotalSteps(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
		CreateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.TotalSteps != 9 {
				t.Errorf("business TotalSteps = %d, want 9", app.TotalSteps)
			}
			app.ID = 1
			return nil
		},
	}
	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{GetByAccountIDFn: satisfiesCertificationPrecondition}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "business"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalSteps != 9 {
		t.Errorf("response TotalSteps = %d, want 9", resp.TotalSteps)
	}
}

func TestCertUC_CreateDraft_IndividualTotalSteps(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
		CreateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.TotalSteps != 8 {
				t.Errorf("individual TotalSteps = %d, want 8", app.TotalSteps)
			}
			app.ID = 2
			return nil
		},
	}
	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{GetByAccountIDFn: satisfiesCertificationPrecondition}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 101, SellerType: "individual"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalSteps != 8 {
		t.Errorf("response TotalSteps = %d, want 8", resp.TotalSteps)
	}
}

func TestCertUC_CreateDraft_ExistingReturned(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 5, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 5 {
		t.Errorf("ID = %d, want 5 (existing draft)", resp.ID)
	}
}

func TestCertUC_CreateDraft_BlockedStatuses(t *testing.T) {
	blocked := []struct {
		name      string
		status    domain.CertificationStatus
		merStatus string
		wantMsg   string
	}{
		{"forwarded, payuni pending", domain.CertStatusForwarded, "", "認證審核中，無法重新申請"},
		{"forwarded, payuni rejected", domain.CertStatusForwarded, "2", "認證審核中，無法重新申請"},
		{"forwarded, payuni approved", domain.CertStatusForwarded, "1", "認證已通過，無需重新申請"},
		{"forward_failed", domain.CertStatusForwardFailed, "", "認證審核中，無法重新申請"},
	}
	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			certRepo := &mockCertificationRepo{
				GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
					return &domain.CertificationApplication{ID: 5, AccountID: 100, Status: tc.status}, nil
				},
				CreateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
					t.Error("CreateApplication must not be called for a blocked status")
					return nil
				},
			}
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
					return &domain.Seller{AccountID: accountID, StoreID: 7}, nil
				},
				UpdateCurrentCertIDFn: func(ctx context.Context, accountID, certID int64) error {
					t.Error("UpdateCurrentCertificationID must not be called for a blocked status")
					return nil
				},
			}
			payuniRepo := &mockPayuniRepo{
				GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
					if tc.merStatus == "" {
						return nil, nil
					}
					return &domain.PayuniRegistration{StoreID: storeID, MerStatus: tc.merStatus, MemberStatus: "1"}, nil
				},
			}

			uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, payuniRepo: payuniRepo}
			_, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})
			if err == nil {
				t.Fatal("expected PreconditionFailed, got nil")
			}
			if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
				t.Errorf("code = %d, want CodePreconditionFailed", pkgerrors.GetCode(err))
			}
			if err.Error() != tc.wantMsg {
				t.Errorf("message = %q, want %q", err.Error(), tc.wantMsg)
			}
		})
	}
}

func TestCertUC_CreateDraft_FromRejected_CreatesNewDraft(t *testing.T) {
	var created bool
	var repointedTo int64
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{ID: 5, AccountID: 100, Status: domain.CertStatusRejected}, nil
		},
		CreateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			created = true
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: satisfiesCertificationPrecondition,
		UpdateCurrentCertIDFn: func(ctx context.Context, accountID, certID int64) error {
			repointedTo = certID
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected a new draft to be created from rejected")
	}
	if resp.Status != "draft" {
		t.Errorf("Status = %q, want draft", resp.Status)
	}
	if repointedTo != resp.ID {
		t.Errorf("current_certification_id repointed to %d, want %d", repointedTo, resp.ID)
	}
}

func TestCertUC_GetDraft_Found(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetDraftApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: 1, AccountID: accountID, Status: domain.CertStatusDraft, RealName: "Test",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.GetDraft(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.RealName != "Test" {
		t.Errorf("RealName = %q, want %q", resp.RealName, "Test")
	}
}

func TestCertUC_GetDraft_NotFound(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetDraftApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.GetDraft(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
}

func TestCertUC_GetApplication_Success(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.GetApplication(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Status != "submitted" {
		t.Errorf("Status = %q, want %q", resp.Status, "submitted")
	}
}

func TestCertUC_CancelDraft_Success(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusDraft, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.Status != domain.CertStatusCancelled {
				t.Errorf("saved Status = %v, want Cancelled", app.Status)
			}
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	if err := uc.CancelDraft(context.Background(), 1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCertUC_Cancel_FromSubmitted_Success(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusSubmitted, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.Status != domain.CertStatusCancelled {
				t.Errorf("saved Status = %v, want Cancelled", app.Status)
			}
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	if err := uc.CancelDraft(context.Background(), 1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCertUC_Cancel_FromForwarded_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusForwarded, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	err := uc.CancelDraft(context.Background(), 1, "")
	if err == nil {
		t.Fatal("expected error for cancel from approved")
	}
}

func TestCertUC_SaveProgress_Draft(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusDraft, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.CurrentStep != 3 {
				t.Errorf("CurrentStep = %d, want 3", app.CurrentStep)
			}
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	err := uc.SaveProgress(context.Background(), &dto.SaveProgress{ApplicationID: 1, CurrentStep: 3, StepData: `{"step":"data"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCertUC_SaveProgress_LinksIdentityVerification(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusDraft, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	var linkedRefType, linkedRefID string
	var linkedAssetIDs []string
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, tokens []string, refType string) ([]AssetInfo, error) {
			if refType != mediapkg.RefTypeIdentityVerification {
				t.Errorf("finalize refType = %q, want IDENTITY_VERIFICATION", refType)
			}
			out := make([]AssetInfo, len(tokens))
			for i, tk := range tokens {
				out[i] = AssetInfo{StagedToken: tk, AssetId: "asset-" + tk}
			}
			return out, nil
		},
		LinkImagesFn: func(_ context.Context, refType, refID string, assetIDs []string) error {
			linkedRefType, linkedRefID, linkedAssetIDs = refType, refID, assetIDs
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: media}
	err := uc.SaveProgress(context.Background(), &dto.SaveProgress{
		ApplicationID: 42, CurrentStep: 2,
		StepData: `{"id_card_front_staged_token":"tok-front","bank_book_staged_token":"tok-bank"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if linkedRefType != mediapkg.RefTypeIdentityVerification {
		t.Errorf("link refType = %q, want IDENTITY_VERIFICATION (%q)", linkedRefType, mediapkg.RefTypeIdentityVerification)
	}
	if linkedRefID != "42" {
		t.Errorf("link refID = %q, want application id 42", linkedRefID)
	}
	if len(linkedAssetIDs) != 2 {
		t.Errorf("linked %d assets, want 2 (front + bank book): %v", len(linkedAssetIDs), linkedAssetIDs)
	}
}

func TestCertUC_SaveProgress_Rejected_ResetsToDraft(t *testing.T) {
	now := time.Now()
	var savedApp *domain.CertificationApplication
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusRejected, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			savedApp = app
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	err := uc.SaveProgress(context.Background(), &dto.SaveProgress{ApplicationID: 1, CurrentStep: 2, StepData: `{}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if savedApp.Status != domain.CertStatusDraft {
		t.Errorf("Status = %v, want Draft (should reset from rejected)", savedApp.Status)
	}
}

func TestCertUC_SaveProgress_Submitted_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusSubmitted, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	err := uc.SaveProgress(context.Background(), &dto.SaveProgress{ApplicationID: 1, CurrentStep: 2})
	if err == nil {
		t.Fatal("expected error for submitted application")
	}
}

func TestCertUC_Approve_Success(t *testing.T) {
	now := time.Now()
	var reviewSaved bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.Status != domain.CertStatusForwarded {
				t.Errorf("app Status = %v, want Forwarded", app.Status)
			}
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			reviewSaved = true
			if review.Action != domain.ReviewActionApprove {
				t.Errorf("review Action = %v, want Approve", review.Action)
			}
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{},
	}
	err := uc.Approve(context.Background(), &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reviewSaved {
		t.Error("review was not saved")
	}
}

func TestCertUC_Approve_ProjectsFoodBusinessRegistration(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "business",
				IsProcessedFood: true, FoodBusinessRegNo: "A123456789",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		CreateReviewFn:      func(ctx context.Context, review *domain.CertificationReview) error { return nil },
	}
	var gotAccountID int64
	var gotIsProcessedFood bool
	var gotRegNo string
	var projected bool
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error {
			projected = true
			gotAccountID = accountID
			gotIsProcessedFood = isProcessedFood
			gotRegNo = regNo
			return nil
		},
	}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{},
	}
	err := uc.Approve(context.Background(), &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !projected {
		t.Fatal("expected UpdateFoodBusinessRegistration to be called")
	}
	if gotAccountID != 100 {
		t.Errorf("accountID = %d, want 100", gotAccountID)
	}
	if !gotIsProcessedFood {
		t.Error("isProcessedFood = false, want true")
	}
	if gotRegNo != "A123456789" {
		t.Errorf("regNo = %q, want %q", gotRegNo, "A123456789")
	}
}

func TestCertUC_Approve_SkipsFoodBusinessRegistrationProjectionWhenNotDeclared(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		CreateReviewFn:      func(ctx context.Context, review *domain.CertificationReview) error { return nil },
	}
	var projected bool
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error {
			projected = true
			return nil
		},
	}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{},
	}
	err := uc.Approve(context.Background(), &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projected {
		t.Error("expected UpdateFoodBusinessRegistration NOT to be called when IsProcessedFood is false")
	}
}

func TestCertUC_Approve_FromDraft_Fails(t *testing.T) {
	now := time.Now()
	var appUpdated, reviewCreated bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 10, Status: domain.CertStatusDraft, SellerType: "individual", CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			appUpdated = true
			return nil
		},
		CreateReviewFn: func(ctx context.Context, r *domain.CertificationReview) error {
			reviewCreated = true
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo, txManager: &mockTxManager{}}
	err := uc.Approve(context.Background(), &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99})
	if err == nil {
		t.Fatal("expected error approving a draft application, got nil")
	}
	if appUpdated || reviewCreated {
		t.Error("expected no application update or review when approving a draft")
	}
}

func TestCertUC_Reject_Success(t *testing.T) {
	now := time.Now()
	var reviewSaved bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			if app.Status != domain.CertStatusRejected {
				t.Errorf("app Status = %v, want Rejected", app.Status)
			}
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			reviewSaved = true
			if review.RejectionReason != "incomplete docs" {
				t.Errorf("RejectionReason = %q, want %q", review.RejectionReason, "incomplete docs")
			}
			return nil
		},
	}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: newIndividualSellerRepo(),
		txManager: &mockTxManager{}, outboxRepo: &mockMerchantVerificationOutboxRepo{},
	}
	err := uc.Reject(context.Background(), &dto.ReviewApplication{
		ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin",
		Comment: "docs missing", RejectionReason: "incomplete docs",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reviewSaved {
		t.Error("review was not saved")
	}
}

func TestCertUC_Reject_InvalidTransition(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusForwarded, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, txManager: &mockTxManager{}}
	err := uc.Reject(context.Background(), &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99})
	if err == nil {
		t.Fatal("expected error for forwarded → reject")
	}
}

func TestCertUC_Submit_InvalidAddress_NoDistrict(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "Test", City: "桃園市", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1, RealName: "Test", IDCardNo: "A123456789", EnglishName: "Test"})

	if err == nil {
		t.Fatal("expected error for address with city only, got nil")
	}
	if !strings.Contains(err.Error(), "完整的縣市和區域") {
		t.Errorf("error = %q, want containing '完整的縣市和區域'", err.Error())
	}
}

func TestCertUC_Submit_InvalidAddress_UnknownCity(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "Test", City: "火星市", District: "某某區", AddressDetail: "1號", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1, RealName: "Test", IDCardNo: "A123456789", EnglishName: "Test"})

	if err == nil {
		t.Fatal("expected error for unrecognized city, got nil")
	}
	if !strings.Contains(err.Error(), "無法辨識地址的縣市") {
		t.Errorf("error = %q, want containing '無法辨識地址的縣市'", err.Error())
	}
}

func TestCertUC_Submit_EmptyStoreName_Individual(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "", City: "台北市", District: "中正區", AddressDetail: "1號", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1, RealName: "Test", IDCardNo: "A123456789", EnglishName: "Test"})

	if err == nil {
		t.Fatal("expected error for empty store name, got nil")
	}
	if !strings.Contains(err.Error(), "店名不可為空") {
		t.Errorf("error = %q, want containing '店名不可為空'", err.Error())
	}
}

func TestCertUC_Submit_EmptyAddress_Individual(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
		CreateReviewFn: func(ctx context.Context, review *domain.CertificationReview) error {
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "Test", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1, RealName: "Test", IDCardNo: "A123456789", EnglishName: "Test"})

	if err == nil {
		t.Fatal("expected error for empty city, got nil")
	}
	if !strings.Contains(err.Error(), "縣市不可為空") {
		t.Errorf("error = %q, want containing '地址不可為空'", err.Error())
	}
}

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+886912345678", "0912345678"},
		{"886912345678", "0912345678"},
		{"0912345678", "0912345678"},
		{"", ""},
		{"12345", ""},
		{"+886-912-345-678", "0912345678"},
	}
	for _, tc := range tests {
		got := normalizePhoneNumber(tc.input)
		if got != tc.want {
			t.Errorf("normalizePhoneNumber(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCertUC_Submit_Business_FirstSubmission_TransitionsToSubmitted(t *testing.T) {
	now := time.Now()
	var savedStatus domain.CertificationStatus
	var reviewSaved bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			savedStatus = app.Status
			return nil
		},
		CreateReviewFn: func(ctx context.Context, r *domain.CertificationReview) error {
			reviewSaved = true
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "Test Shop", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo:      certRepo,
		sellerRepo:    sellerRepo,
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{FinalizeImagesFn: func(ctx context.Context, tokens []string, refType string) ([]AssetInfo, error) { return nil, nil }},
		txManager:     &mockTxManager{},
	}
	if err := uc.Submit(context.Background(), &dto.SubmitApplication{
		ID: 1, RealName: "Test", CompanyName: "測試公司", TaxID: "11111101", EnglishName: "Test Shop",
		IDCardFrontAssetID: "asset-id-card-front",
		IDCardBackAssetID:  "asset-id-card-back",
		BankBookAssetID:    "asset-bank-book",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if savedStatus != domain.CertStatusSubmitted {
		t.Errorf("Status = %v, want Submitted", savedStatus)
	}
	if !reviewSaved {
		t.Error("review was not saved on first submission")
	}
}

func TestCertUC_Submit_FromCancelled_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusCancelled,
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo:    certRepo,
		sellerRepo:  &mockSellerRepo{},
		mediaClient: &mockMediaClient{FinalizeImagesFn: func(ctx context.Context, tokens []string, refType string) ([]AssetInfo, error) { return nil, nil }},
		txManager:   &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1})
	if err == nil {
		t.Fatal("expected error for submit from cancelled, got nil")
	}
}

func TestCertUC_Submit_FromForwarded_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusForwarded,
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo:    certRepo,
		sellerRepo:  &mockSellerRepo{},
		mediaClient: &mockMediaClient{FinalizeImagesFn: func(ctx context.Context, tokens []string, refType string) ([]AssetInfo, error) { return nil, nil }},
		txManager:   &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1})
	if err == nil {
		t.Fatal("expected error for submit from forwarded, got nil")
	}
}

func TestCertUC_Submit_FromSubmitted_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo:    certRepo,
		sellerRepo:  &mockSellerRepo{},
		mediaClient: &mockMediaClient{FinalizeImagesFn: func(ctx context.Context, tokens []string, refType string) ([]AssetInfo, error) { return nil, nil }},
		txManager:   &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1})
	if err == nil {
		t.Fatal("expected error for submit from submitted, got nil")
	}
}

func TestCertUC_SaveProgress_FromForwarded_Fails(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, Status: domain.CertStatusForwarded, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) { return nil, nil }}, payuniRepo: &mockPayuniRepo{}}
	err := uc.SaveProgress(context.Background(), &dto.SaveProgress{ApplicationID: 1, CurrentStep: 2})
	if err == nil {
		t.Fatal("expected error for save progress on forwarded application")
	}
}

func TestCertUC_CreateDraft_SwitchBusinessToIndividual(t *testing.T) {
	now := time.Now()
	regDate := now.AddDate(-1, 0, 0)
	existing := &domain.CertificationApplication{
		ID: 22, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
		CompanyName: "範例商貿有限公司", TaxID: "12345678", CompanyType: "company",
		CompanyCity: "台北市", CompanyDistrict: "中山區", CompanyAddressDetail: "中山路1號",
		CompanyEmail: "test@example.com", CompanyPhone: "02-12345678",
		CompanyRegisterDate: &regDate, ProductType: "生鮮蔬果", IsProcessedFood: true,
		CurrentStep: 5, TotalSteps: 9,
		StepData:  []byte(`{"companyName":"範例商貿有限公司"}`),
		CreatedAt: now, UpdatedAt: now,
	}

	var updated *domain.CertificationApplication
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			updated = app
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: &mockMediaClient{}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated == nil {
		t.Fatal("expected UpdateApplication to be called")
	}
	if resp.SellerType != "individual" {
		t.Errorf("SellerType = %q, want %q", resp.SellerType, "individual")
	}
	if resp.TotalSteps != 8 {
		t.Errorf("TotalSteps = %d, want 8 (individual)", resp.TotalSteps)
	}
	if updated.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0 (reset)", updated.CurrentStep)
	}
	if updated.StepData != nil {
		t.Errorf("StepData = %s, want nil (cleared)", string(updated.StepData))
	}
	if updated.CompanyName != "" {
		t.Errorf("CompanyName = %q, want empty", updated.CompanyName)
	}
	if updated.TaxID != "" {
		t.Errorf("TaxID = %q, want empty", updated.TaxID)
	}
	if updated.CompanyCity != "" {
		t.Errorf("CompanyCity = %q, want empty", updated.CompanyCity)
	}
	if updated.CompanyRegisterDate != nil {
		t.Errorf("CompanyRegisterDate = %v, want nil", updated.CompanyRegisterDate)
	}
	if updated.IsProcessedFood {
		t.Error("IsProcessedFood = true, want false")
	}
}

func TestCertUC_CreateDraft_SwitchIndividualToBusiness(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 30, AccountID: 200, Status: domain.CertStatusDraft, SellerType: "individual",
		RealName: "王小明", IDCardNo: "A123456789",
		CurrentStep: 3, TotalSteps: 8,
		StepData:  []byte(`{"real_name":"王小明"}`),
		CreatedAt: now, UpdatedAt: now,
	}

	var updated *domain.CertificationApplication
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			updated = app
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}, mediaClient: &mockMediaClient{}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 200, SellerType: "business"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated == nil {
		t.Fatal("expected UpdateApplication to be called")
	}
	if resp.SellerType != "business" {
		t.Errorf("SellerType = %q, want %q", resp.SellerType, "business")
	}
	if resp.TotalSteps != 9 {
		t.Errorf("TotalSteps = %d, want 9 (business)", resp.TotalSteps)
	}
	if updated.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0 (reset)", updated.CurrentStep)
	}
	if updated.StepData != nil {
		t.Errorf("StepData = %s, want nil (cleared)", string(updated.StepData))
	}
}

func TestCertUC_CreateDraft_SameType_NoSwitch(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 22, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
		CompanyName: "範例商貿有限公司", CurrentStep: 5, TotalSteps: 9,
		CreatedAt: now, UpdatedAt: now,
	}
	updateCalled := false
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			updateCalled = true
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "business"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updateCalled {
		t.Error("UpdateApplication should NOT be called for same type")
	}
	if resp.SellerType != "business" {
		t.Errorf("SellerType = %q, want %q", resp.SellerType, "business")
	}
	if resp.TotalSteps != 9 {
		t.Errorf("TotalSteps = %d, want 9 (unchanged)", resp.TotalSteps)
	}
}

func TestCertUC_CreateDraft_SubmittedNoSwitch(t *testing.T) {
	now := time.Now()
	existing := &domain.CertificationApplication{
		ID: 22, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "business",
		CreatedAt: now, UpdatedAt: now,
	}
	updateCalled := false
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return existing, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			updateCalled = true
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	resp, err := uc.CreateDraft(context.Background(), &dto.CreateCertificationDraft{AccountID: 100, SellerType: "individual"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updateCalled {
		t.Error("UpdateApplication should NOT be called for submitted application")
	}
	if resp.SellerType != "business" {
		t.Errorf("SellerType = %q, want %q (unchanged)", resp.SellerType, "business")
	}
}

func TestCertUC_Submit_BusinessEmptyCompanyName(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "好會賣", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{
		ID: 22, RealName: "好會賣", IDCardNo: "A123456789", EnglishName: "Test",
	})

	if err == nil {
		t.Fatal("expected error for business with empty company_name, got nil")
	}
	if !strings.Contains(err.Error(), "公司名稱不可為空") {
		t.Errorf("error = %q, want containing '公司名稱不可為空'", err.Error())
	}
}

func TestCertUC_Submit_BusinessEmptyTaxID(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft, SellerType: "business",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "好會賣", Description: "測試", WebsiteURL: "https://example.com"}, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 },
		certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo:    defaultPayuniRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{}, txManager: &mockTxManager{},
	}
	err := uc.Submit(context.Background(), &dto.SubmitApplication{
		ID: 22, RealName: "好會賣", IDCardNo: "A123456789", CompanyName: "範例商貿有限公司", EnglishName: "Test",
	})

	if err == nil {
		t.Fatal("expected error for business with empty tax_id, got nil")
	}
	if !strings.Contains(err.Error(), "統一編號不可為空") {
		t.Errorf("error = %q, want containing '統一編號不可為空'", err.Error())
	}
}

func TestCertUC_CancelDraft_FromForwardFailed_Success(t *testing.T) {
	now := time.Now()
	var savedStatus domain.CertificationStatus
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusForwardFailed, CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			savedStatus = app.Status
			return nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	if err := uc.CancelDraft(context.Background(), 1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if savedStatus != domain.CertStatusCancelled {
		t.Errorf("Status = %v, want Cancelled", savedStatus)
	}
}

func TestCertUC_CancelDraft_FromForwarded_Fails(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusForwarded, CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
	}

	uc := &CertificationUseCase{tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: &mockSellerRepo{}}
	err := uc.CancelDraft(context.Background(), 1, "")
	if err == nil {
		t.Fatal("expected error for cancel from forwarded")
	}
	if app.Status != domain.CertStatusForwarded {
		t.Errorf("Status = %v, want unchanged Forwarded (no UpdateApplication should have been called)", app.Status)
	}
}
