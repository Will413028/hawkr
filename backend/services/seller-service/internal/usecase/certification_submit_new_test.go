package usecase

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// newIndividualDraftApp returns a mockCertificationRepo that serves an
// individual-seller draft application keyed by ID 1, account 100.
func newIndividualDraftApp(now time.Time) *mockCertificationRepo {
	return &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100,
				Status: domain.CertStatusDraft, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		CreateReviewFn:      func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
}

// newIndividualSubmittedApp returns a mockCertificationRepo that serves an
// individual-seller application already in "submitted" state.
func newIndividualSubmittedApp(now time.Time) *mockCertificationRepo {
	return &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			submitted := now.Add(-time.Hour)
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100,
				Status: domain.CertStatusSubmitted, SellerType: "individual",
				SubmittedAt: &submitted,
				CreatedAt:   now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		CreateReviewFn:      func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
}

// newIndividualSellerRepo returns a mockSellerRepo that serves a valid
// individual seller with onboarding completed (store name + address).
func newIndividualSellerRepo() *mockSellerRepo {
	return &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID:     accountID,
				StoreID:       accountID,
				StoreName:     "測試店",
				Description:   "測試",
				City:          "台北市",
				District:      "信義區",
				AddressDetail: "信義路五段 1 號",
				PostalCode:    "110",
				ContactPhone:  "0912345678",
				WebsiteURL:    "https://example.com",
			}, nil
		},
	}
}

// validIndividualSubmitReq returns a SubmitApplication payload that would
// pass validatePayuniFields for an individual seller.
func validIndividualSubmitReq() *dto.SubmitApplication {
	return &dto.SubmitApplication{
		ID:                 1,
		RealName:           "王小明",
		IDCardNo:           "A123456789",
		EnglishName:        "Xiao Ming Wang",
		IDCardFrontAssetID: "asset-id-card-front",
		IDCardBackAssetID:  "asset-id-card-back",
		BankBookAssetID:    "asset-bank-book",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation failure (Requirement 3)
// ─────────────────────────────────────────────────────────────────────────────

func TestSubmit_Validation_MultipleFieldsError(t *testing.T) {
	// Bad email + bad english name + missing ID card → should get ValidationError
	// with multiple messages joined by "；", cert state unchanged.
	now := time.Now()
	var updateAppCalled bool
	certRepo := newIndividualDraftApp(now)
	certRepo.UpdateApplicationFn = func(ctx context.Context, app *domain.CertificationApplication) error {
		updateAppCalled = true
		return nil
	}

	acct := defaultAccountClient()
	acct.GetAccountInfoFn = func(ctx context.Context, accountID int64) (*AccountInfo, error) {
		return &AccountInfo{Email: "bad-email", Phone: "0912345678"}, nil
	}

	uc := &CertificationUseCase{
		tsid:          func() int64 { return 99999 },
		certRepo:      certRepo,
		sellerRepo:    newIndividualSellerRepo(),
		payuniRepo:    defaultPayuniRepo(),
		accountClient: acct,
		mediaClient:   &mockMediaClient{},
		txManager:     &mockTxManager{},
	}

	err := uc.Submit(context.Background(), &dto.SubmitApplication{
		ID:          1,
		RealName:    "王小明",
		IDCardNo:    "",     // missing → error
		EnglishName: "示範商店", // Chinese → charset error
	})
	if err == nil {
		t.Fatal("expected ValidationError")
	}
	var verr *ValidationError
	if !stderrors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	// Expect at least 3 errors: email, english name, id card no
	if len(verr.Messages) < 3 {
		t.Errorf("expected ≥3 validation errors, got %d: %v", len(verr.Messages), verr.Messages)
	}
	if !strings.Contains(verr.Error(), "；") {
		t.Error("expected errors joined by full-width semicolon")
	}
	if updateAppCalled {
		t.Error("UpdateApplication should NOT be called when validation fails (state unchanged)")
	}
}

// TestSubmit_AlreadySubmitted_Rejects: 賣家在 submitted（等待平台審核）狀態
// 下再次呼叫 Submit，應該直接拒絕（已鎖定，不可修改），mirrors SaveProgress。
func TestSubmit_AlreadySubmitted_Rejects(t *testing.T) {
	now := time.Now()
	var createReviewCalled bool
	certRepo := newIndividualSubmittedApp(now)
	certRepo.CreateReviewFn = func(ctx context.Context, r *domain.CertificationReview) error {
		createReviewCalled = true
		return nil
	}

	uc := &CertificationUseCase{
		tsid:          func() int64 { return 99999 },
		certRepo:      certRepo,
		sellerRepo:    newIndividualSellerRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{},
		txManager:     &mockTxManager{},
	}

	err := uc.Submit(context.Background(), validIndividualSubmitReq())
	if err == nil {
		t.Fatal("expected FailedPrecondition for submit while submitted")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
	if createReviewCalled {
		t.Error("CreateReview should NOT be called when submit is rejected")
	}
}

// TestSubmit_NeverCallsPayuni: Submit() 不再是 PayUni 呼叫的觸發點，這條測試
// 用 payuniClient=nil 確保任何不慎殘留的呼叫會直接 panic 而不是被靜默吃掉。
func TestSubmit_NeverCallsPayuni(t *testing.T) {
	now := time.Now()
	uc := &CertificationUseCase{
		tsid:          func() int64 { return 99999 },
		certRepo:      newIndividualDraftApp(now),
		sellerRepo:    newIndividualSellerRepo(),
		accountClient: defaultAccountClient(),
		mediaClient:   &mockMediaClient{},
		txManager:     &mockTxManager{},
	}

	if err := uc.Submit(context.Background(), validIndividualSubmitReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Terminal state rejection
// ─────────────────────────────────────────────────────────────────────────────

func TestSubmit_FromCancelled_Rejects(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100,
				Status:    domain.CertStatusCancelled,
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}
	// Note: no payuniRepo / accountClient — the state check should return
	// before hitting them.
	uc := &CertificationUseCase{
		tsid:       func() int64 { return 99999 },
		certRepo:   certRepo,
		sellerRepo: &mockSellerRepo{},
		txManager:  &mockTxManager{},
	}

	err := uc.Submit(context.Background(), &dto.SubmitApplication{ID: 1})
	if err == nil {
		t.Fatal("expected FailedPrecondition")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Post-or-website gate (Task 9)
// ─────────────────────────────────────────────────────────────────────────────

func TestSubmit_NoPostNoWebsite_Rejects(t *testing.T) {
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
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return false, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	err := uc.Submit(context.Background(), validIndividualSubmitReq())
	if err == nil {
		t.Fatal("expected error when seller has no post and no website_url")
	}
}

func TestSubmit_HasDescriptionAndWebsite_NoPost_Allows(t *testing.T) {
	now := time.Now()
	certRepo := newIndividualDraftApp(now)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: accountID, StoreName: "測試店", Description: "測試",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號",
				PostalCode: "110", ContactPhone: "0912345678", WebsiteURL: "https://my-shop.example.com",
			}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) {
		t.Fatal("socialClient must not be called when description+website are both set (short-circuit)")
		return false, nil
	}}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	if err := uc.Submit(context.Background(), validIndividualSubmitReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSubmit_WebsiteOnly_NoDescription_NoPost_Rejects reproduces #1893: the old
// gate only checked seller.WebsiteURL != "", so a seller with a website but a
// blank self-introduction (and no post) could submit — bypassing the website
// gate's actual intent of requiring both fields together.
func TestSubmit_WebsiteOnly_NoDescription_NoPost_Rejects(t *testing.T) {
	now := time.Now()
	certRepo := newIndividualDraftApp(now)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: accountID, StoreName: "測試店", Description: "",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號",
				PostalCode: "110", ContactPhone: "0912345678", WebsiteURL: "https://my-shop.example.com",
			}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return false, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	err := uc.Submit(context.Background(), validIndividualSubmitReq())
	if err == nil {
		t.Fatal("expected error when seller has website but no description and no post")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
}

// TestSubmit_WhitespaceDescription_HasWebsite_NoPost_Rejects covers the
// trim-before-check requirement from #1893: whitespace-only description must
// not count as "provided".
func TestSubmit_WhitespaceDescription_HasWebsite_NoPost_Rejects(t *testing.T) {
	now := time.Now()
	certRepo := newIndividualDraftApp(now)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: accountID, StoreName: "測試店", Description: "   ",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號",
				PostalCode: "110", ContactPhone: "0912345678", WebsiteURL: "https://my-shop.example.com",
			}, nil
		},
	}
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) { return false, nil }}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	err := uc.Submit(context.Background(), validIndividualSubmitReq())
	if err == nil {
		t.Fatal("expected error when description is whitespace-only")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
}

func TestSubmit_HasPost_NoWebsite_Allows(t *testing.T) {
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

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	if err := uc.Submit(context.Background(), validIndividualSubmitReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmit_SocialServiceUnavailable_FailsClosed(t *testing.T) {
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
	social := &mockSocialClient{HasPublishedPostFn: func(ctx context.Context, storeID int64) (bool, error) {
		return false, pkgerrors.NewUnavailable("social-service unavailable")
	}}

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo, sellerRepo: sellerRepo,
		payuniRepo: defaultPayuniRepo(), accountClient: defaultAccountClient(),
		mediaClient: &mockMediaClient{}, txManager: &mockTxManager{}, socialClient: social,
	}
	err := uc.Submit(context.Background(), validIndividualSubmitReq())
	if pkgerrors.GetCode(err) != pkgerrors.CodeUnavailable {
		t.Fatalf("GetCode(err) = %v, want CodeUnavailable; err=%v", pkgerrors.GetCode(err), err)
	}
}
