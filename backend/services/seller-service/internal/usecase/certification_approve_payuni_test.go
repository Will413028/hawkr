package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

type fakeOutcomeApplier struct {
	applied []string
	result  bool
	fn      func(ctx context.Context, info *domain.PayuniRegistration, now time.Time, source string) bool
}

func (f *fakeOutcomeApplier) ReapplyOutcome(ctx context.Context, info *domain.PayuniRegistration, now time.Time, source string) bool {
	f.applied = append(f.applied, source)
	if f.fn != nil {
		return f.fn(ctx, info, now, source)
	}
	return f.result
}

func newSubmittedAppForApproval(now time.Time) *mockCertificationRepo {
	return &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		CreateReviewFn:      func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
}

func sharedAppRepo(app *domain.CertificationApplication, saved **domain.CertificationApplication) *mockCertificationRepo {
	return &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error {
			*saved = a
			return nil
		},
		CreateReviewFn: func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
}

func realOutcomeApplier(certRepo domain.CertificationRepository, outboxRepo domain.MerchantVerificationOutboxRepository) *PayuniUseCase {
	return &PayuniUseCase{certRepo: certRepo, outboxRepo: outboxRepo, txManager: &mockTxManager{}, tsid: func() int64 { return 88888 }}
}

func validReviewReq() *dto.ReviewApplication {
	return &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin"}
}

func TestApprove_SubmittedWithPendingReg_Relinks(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	var upsertedReg *domain.PayuniRegistration
	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{ID: 42, AccountID: accountID, ApplicationID: 999, Status: domain.PayuniStatusPending}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upsertedReg = reg
		return nil
	}

	var bindCalled bool
	fake := &fakeOutcomeApplier{}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: fake,
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bindCalled {
		t.Error("bind_merchant should NOT be called when a registration already exists")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded, got %+v", savedApp)
	}
	if upsertedReg == nil || upsertedReg.ApplicationID != 1 {
		t.Errorf("expected registration relinked to application_id=1, got %+v", upsertedReg)
	}
	if len(fake.applied) != 0 {
		t.Errorf("funnel should NOT be consulted for empty mer_status, got %v", fake.applied)
	}
}

func TestApprove_Relink_SkipsAccountServiceCall(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByStoreIDFn = func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{ID: 42, AccountID: 100, StoreID: storeID, ApplicationID: 999, Status: domain.PayuniStatusPending}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(),
		payuniRepo: payuniRepo,
		accountClient: &mockAccountClient{
			GetAccountInfoFn: func(ctx context.Context, accountID int64) (*AccountInfo, error) {
				t.Error("GetAccountInfo must not be called on relink")
				return nil, pkgerrors.NewUnavailable("account-service down")
			},
		},
		txManager:     &mockTxManager{},
		outboxRepo:    &mockMerchantVerificationOutboxRepo{},
		payuniOutcome: &fakeOutcomeApplier{},
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("relink approve must succeed without account info, got %v", err)
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded, got %+v", savedApp)
	}
}

func TestApprove_Relink_SoftDeletedSeller_StillRelinks(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusClosed, DeletedAt: &now}, nil
		},
	}
	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByStoreIDFn = func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{ID: 42, AccountID: 100, StoreID: storeID, ApplicationID: 999, Status: domain.PayuniStatusPending}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	var bindCalled bool
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: sellerRepo, payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo:    &mockMerchantVerificationOutboxRepo{},
		payuniOutcome: &fakeOutcomeApplier{},
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("relink of a soft-deleted seller must succeed, got %v", err)
	}
	if bindCalled {
		t.Error("bind_merchant must NOT be called on relink")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded, got %+v", savedApp)
	}
}

func TestApprove_Reopen_IgnoresStaleOldIncarnationRegistration(t *testing.T) {
	const currentStoreID int64 = 7300000000000000456
	const currentStoreCode int64 = 2612345679

	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: currentStoreID, StoreCode: currentStoreCode, StoreName: "測試店",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號",
				PostalCode: "110", ContactPhone: "0912345678",
			}, nil
		},
	}

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByStoreIDFn = func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
		return nil, pkgerrors.NewNotFound("payuni registration not found")
	}
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{ID: 42, AccountID: accountID, StoreID: 100, ApplicationID: 999, Status: domain.PayuniStatusPending}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	var boundSellerID string
	uc := &CertificationUseCase{
		tsid: func() int64 { return 88888 }, certRepo: certRepo,
		sellerRepo: sellerRepo, payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				boundSellerID = req.SellerID
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo:    &mockMerchantVerificationOutboxRepo{},
		payuniOutcome: &fakeOutcomeApplier{},
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if boundSellerID != "HAWK2612345679" {
		t.Errorf("bind SellerID = %q, want HAWK2612345679 (fresh bind under current id's store_code, not relink to stale)", boundSellerID)
	}
	if boundSellerID == "HAWK7300000000000000456" {
		t.Errorf("bind SellerID = %q, regressed to deriving from store_id", boundSellerID)
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded, got %+v", savedApp)
	}
}

func TestApprove_FromForwarded_Blocks(t *testing.T) {
	now := time.Now()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusForwarded, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
	}
	var bindCalled bool
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo,
	}

	err := uc.Approve(context.Background(), validReviewReq())
	if err == nil {
		t.Fatal("expected PreconditionFailed for an already-forwarded application, got nil")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %d", pkgerrors.GetCode(err))
	}
	if bindCalled {
		t.Error("bind_merchant must NOT be called when the entry-state check rejects")
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Errorf("no outbox row expected when the entry-state check rejects, got %v", outboxRepo.Inserted)
	}
}

func TestApprove_HappyPath_BindsPayuniAndCommitsForwarded(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	var reviewCreated bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { savedApp = a; return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { reviewCreated = true; return nil },
	}

	var upserted *domain.PayuniRegistration
	payuniRepo := defaultPayuniRepo()
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upserted = reg
		return nil
	}

	var bindCalled bool
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	fake := &fakeOutcomeApplier{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: fake,
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bindCalled {
		t.Error("expected bind_merchant to be called")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded, got %+v", savedApp)
	}
	if savedApp != nil && savedApp.ForwardedAt == nil {
		t.Error("expected forwarded_at to be stamped")
	}
	if upserted == nil || upserted.Status != domain.PayuniStatusPending {
		t.Errorf("expected payuni registration upserted with status=pending, got %+v", upserted)
	}
	if !reviewCreated {
		t.Error("expected a review to be created")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "forwarded_to_payuni" {
		t.Errorf("outbox inserted = %v, want exactly one forwarded_to_payuni row", outboxRepo.Inserted)
	}
	if len(fake.applied) != 0 {
		t.Errorf("funnel should NOT be consulted for a fresh bind, got %v", fake.applied)
	}
}

func TestApprove_PayuniWebsiteURLOverride_TakesPriority(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}

	var boundWebsiteURL string
	payuniRepo := defaultPayuniRepo()
	fake := &fakeOutcomeApplier{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				boundWebsiteURL = req.WebsiteURL
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{}, payuniOutcome: fake,
	}

	req := &dto.ReviewApplication{
		ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin",
		PayuniWebsiteURL: "https://payuni-override.example.com",
	}
	if err := uc.Approve(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if boundWebsiteURL != "https://payuni-override.example.com" {
		t.Errorf("BindSellerRequest.WebsiteURL = %q, want override value", boundWebsiteURL)
	}
}

func TestApprove_PayuniWebsiteURLOverride_EmptyMeansNoOverride(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}

	var boundWebsiteURL string
	fake := &fakeOutcomeApplier{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				boundWebsiteURL = req.WebsiteURL
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{}, payuniOutcome: fake,
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if boundWebsiteURL != "" {
		t.Errorf("BindSellerRequest.WebsiteURL = %q, want empty (seller.WebsiteURL must not leak in)", boundWebsiteURL)
	}
}

func TestApprove_PayuniWebsiteURLOverride_InvalidFormat_Rejected(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			now := time.Now()
			app := &domain.CertificationApplication{
				ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
				CreatedAt: now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error {
			t.Fatal("UpdateApplication must not be called when payuni_website_url is invalid")
			return nil
		},
	}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: defaultPayuniRepo(),
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				t.Fatal("BindSeller must not be called when payuni_website_url is invalid")
				return nil, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
	}

	req := &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin", PayuniWebsiteURL: "not-a-url"}
	err := uc.Approve(context.Background(), req)
	if pkgerrors.GetCode(err) != pkgerrors.ErrorCode(bizcode.SellerWebsiteUrlInvalid) {
		t.Fatalf("GetCode(err) = %v, want SellerWebsiteUrlInvalid (%d); err=%v", pkgerrors.GetCode(err), bizcode.SellerWebsiteUrlInvalid, err)
	}
}

func TestApprove_Relink_PayuniWebsiteURLOverride_PersistedButNotSentToPayuni(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{ID: 42, AccountID: accountID, ApplicationID: 999, Status: domain.PayuniStatusPending}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	var bindCalled bool
	fake := &fakeOutcomeApplier{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{}, payuniOutcome: fake,
	}

	req := &dto.ReviewApplication{
		ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin",
		PayuniWebsiteURL: "https://payuni-override.example.com",
	}
	if err := uc.Approve(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bindCalled {
		t.Error("bind_merchant should NOT be called on relink — override cannot reach PayUni this time")
	}
	if savedApp == nil || savedApp.PayuniWebsiteURL != "https://payuni-override.example.com" {
		t.Errorf("expected override persisted on application record even though relink skipped PayUni, got %+v", savedApp)
	}
}

func TestApprove_HappyPath_MerIDDerivesFromStoreCode(t *testing.T) {
	const longStoreID int64 = 7300000000000000123
	const shortStoreCode int64 = 2612345678

	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: longStoreID, StoreCode: shortStoreCode,
				StoreName: "測試店", ContactPhone: "0912345678",
				City: "台北市", District: "信義區", AddressDetail: "信義路五段 1 號", PostalCode: "110",
			}, nil
		},
	}

	var upserted *domain.PayuniRegistration
	var boundSellerID, boundStoreID, boundStoreCode string
	payuniRepo := defaultPayuniRepo()
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upserted = reg
		return nil
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: sellerRepo, payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				boundSellerID = req.SellerID
				boundStoreID = req.StoreID
				boundStoreCode = req.StoreCode
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: &fakeOutcomeApplier{},
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if boundSellerID != "HAWK2612345678" {
		t.Errorf("bind_merchant SellerID = %q, want %q (must derive from store_code, not store_id/account_id)", boundSellerID, "HAWK2612345678")
	}
	if boundSellerID == "HAWK7300000000000000123" {
		t.Errorf("bind_merchant SellerID = %q, regressed to deriving from store_id", boundSellerID)
	}
	if boundStoreID != "7300000000000000123" {
		t.Errorf("bind_merchant StoreID = %q, want %q (must stay the plain store_id, not MerID)", boundStoreID, "7300000000000000123")
	}
	if boundStoreCode != "2612345678" {
		t.Errorf("bind_merchant StoreCode = %q, want %q (must derive MerURL from store_code, not store_id)", boundStoreCode, "2612345678")
	}
	if upserted == nil || upserted.StoreID != longStoreID {
		t.Errorf("upserted registration StoreID = %+v, want %d", upserted, longStoreID)
	}
	if upserted == nil || upserted.PayuniMerID != "HAWK2612345678" {
		t.Errorf("upserted registration PayuniMerID = %+v, want HAWK2612345678", upserted)
	}
	if upserted != nil && upserted.PayuniMerID == "HAWK7300000000000000123" {
		t.Errorf("upserted registration PayuniMerID = %+v, regressed to deriving from store_id", upserted)
	}

	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("outbox rows inserted = %d, want 1", len(outboxRepo.Inserted))
	}
	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(outboxRepo.Inserted[0].Payload, &event); err != nil {
		t.Fatalf("outbox payload did not unmarshal: %v", err)
	}
	if event.StoreID != longStoreID {
		t.Errorf("outbox payload StoreID = %d, want %d", event.StoreID, longStoreID)
	}
	if event.MerchantID != 100 {
		t.Errorf("outbox payload MerchantID = %d, want 100 (account_id)", event.MerchantID)
	}
}

func TestApprove_PayuniBusinessFailure_CertGoesForwardFailed(t *testing.T) {
	now := time.Now()
	var savedStatus domain.CertificationStatus
	certRepo := newSubmittedAppForApproval(now)
	certRepo.UpdateApplicationFn = func(ctx context.Context, app *domain.CertificationApplication) error {
		savedStatus = app.Status
		return nil
	}
	var review *domain.CertificationReview
	certRepo.CreateReviewFn = func(ctx context.Context, r *domain.CertificationReview) error {
		review = r
		return nil
	}
	var upserted *domain.PayuniRegistration
	payuniRepo := defaultPayuniRepo()
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upserted = reg
		return nil
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: false, ErrorCode: "AGENT07003", Message: "系統錯誤"}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo,
	}

	err := uc.Approve(context.Background(), validReviewReq())
	if err == nil {
		t.Fatal("expected *PayuniBindError, got nil")
	}
	if savedStatus != domain.CertStatusForwardFailed {
		t.Errorf("cert status = %v, want ForwardFailed", savedStatus)
	}
	if upserted == nil || upserted.Status != domain.PayuniStatusFailed {
		t.Errorf("expected payuni registration upserted with status=failed, got %+v", upserted)
	}
	if review == nil {
		t.Fatal("expected an audit review to be created on bind failure")
	}
	if review.Action != domain.ReviewActionApprove || review.Result != domain.ReviewResultForwardFailed {
		t.Errorf("review action/result = %v/%v, want approve/forward_failed", review.Action, review.Result)
	}
	if review.RejectionReason == "" {
		t.Error("expected the bind error captured in review.RejectionReason")
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Errorf("no forwarded_to_payuni outbox row expected on bind failure, got %v", outboxRepo.Inserted)
	}
}

func TestRegisterPayuni_BusinessFailure_PersistsErrorCode(t *testing.T) {
	now := time.Now()
	certRepo := newSubmittedAppForApproval(now)
	var upserted *domain.PayuniRegistration
	payuniRepo := defaultPayuniRepo()
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upserted = reg
		return nil
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: false, ErrorCode: "AGENT03019", Message: "會員帳號或手機或Email資料重複"}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo,
	}

	err := uc.Approve(context.Background(), validReviewReq())
	if err == nil {
		t.Fatal("expected *PayuniBindError, got nil")
	}
	if upserted == nil {
		t.Fatal("expected payuni registration to be upserted on bind failure")
	}
	if upserted.ErrorCode != "AGENT03019" {
		t.Errorf("ErrorCode = %q, want AGENT03019", upserted.ErrorCode)
	}
	if upserted.ErrorMessage != "會員帳號或手機或Email資料重複" {
		t.Errorf("ErrorMessage = %q, want 會員帳號或手機或Email資料重複", upserted.ErrorMessage)
	}
}

func TestApprove_ForwardFailedRetry_Succeeds(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusForwardFailed, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)
	var reviewCreated bool
	certRepo.CreateReviewFn = func(ctx context.Context, r *domain.CertificationReview) error {
		reviewCreated = true
		return nil
	}

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{AccountID: accountID, Status: domain.PayuniStatusFailed}, nil
	}

	var bindCalled bool
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo,
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if !bindCalled {
		t.Error("expected bind_merchant to be called on retry")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("expected cert status forwarded after successful retry, got %+v", savedApp)
	}
	if !reviewCreated {
		t.Error("expected a review to be created on retry (no in-place-retry branch should skip CreateReview)")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "forwarded_to_payuni" {
		t.Errorf("outbox inserted = %v, want exactly one forwarded_to_payuni row", outboxRepo.Inserted)
	}
}

func TestApprove_ForwardFailedRetry_BindFailsAgain(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusForwardFailed, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{AccountID: accountID, Status: domain.PayuniStatusFailed}, nil
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: false, ErrorCode: "AGENT07003", Message: "系統錯誤"}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo,
	}

	err := uc.Approve(context.Background(), validReviewReq())
	if err == nil {
		t.Fatal("expected the bind error to propagate, got nil")
	}
	var bindErr *PayuniBindError
	if !errors.As(err, &bindErr) {
		t.Fatalf("error type = %T (%v), want *PayuniBindError (not masked by ErrInvalidTransition)", err, err)
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwardFailed {
		t.Errorf("expected forward_failed persisted after failed retry, got %+v", savedApp)
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Errorf("no outbox row expected on a failed retry, got %v", outboxRepo.Inserted)
	}
}

func TestApprove_RelinkActive_FunnelsToApproved(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{
			ID: 42, AccountID: accountID, ApplicationID: 999, PayuniMerID: "HAWK100",
			MerStatus: "1", MemberStatus: "1", Status: domain.PayuniStatusActive,
		}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var bindCalled bool
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: realOutcomeApplier(certRepo, outboxRepo),
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bindCalled {
		t.Error("bind_merchant should NOT be called when PayUni is already active")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("cert 應停在 forwarded（PayUni 的審核結果不進工作流），got %+v", savedApp)
	}
	foundApproved := false
	for _, row := range outboxRepo.Inserted {
		if row.EventType == "approved" {
			foundApproved = true
		}
	}
	if !foundApproved {
		t.Errorf("expected approved outbox row inserted via funnel, got %+v", outboxRepo.Inserted)
	}
}

func TestApprove_RelinkActive_AdvancesToApproved(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	payuniRepo.GetByAccountIDFn = func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
		return &domain.PayuniRegistration{
			ID: 42, AccountID: accountID, ApplicationID: 999, PayuniMerID: "HAWK100",
			MerStatus: "1", MemberStatus: "1", Status: domain.PayuniStatusActive,
		}, nil
	}
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil }

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var bindCalled bool
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				bindCalled = true
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: realOutcomeApplier(certRepo, outboxRepo),
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bindCalled {
		t.Error("bind_merchant should NOT be called when PayUni is already active")
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("cert 應停在 forwarded（PayUni 的審核結果不進工作流），got %+v", savedApp)
	}
}

func TestApprove_AGENT04017_TerminalMerchant_ForwardFailed(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	var upserted *domain.PayuniRegistration
	payuniRepo := defaultPayuniRepo()
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upserted = reg
		return nil
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: false, ErrorCode: "AGENT04017", Message: "商店代號已存在"}, nil
			},
			GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
				return &SellerStatusResponse{MerID: merID, MerStatus: "8", RawResponse: json.RawMessage(`{}`)}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: realOutcomeApplier(certRepo, outboxRepo),
	}

	err := uc.Approve(context.Background(), validReviewReq())
	if err == nil {
		t.Fatal("expected business error for terminal AGENT04017 merchant, got nil")
	}
	var bindErr *PayuniBindError
	if !errors.As(err, &bindErr) {
		t.Fatalf("error type = %T (%v), want *PayuniBindError", err, err)
	}
	wantMsg := "PayUni 端商店代號已存在且為退件/關閉狀態，需人工向 PayUni 處理後再重試"
	if bindErr.Message != wantMsg {
		t.Errorf("message = %q, want %q", bindErr.Message, wantMsg)
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwardFailed {
		t.Errorf("expected cert status forward_failed (NOT rejected — fresh app never reached PayUni), got %+v", savedApp)
	}
	if upserted == nil || upserted.Status != domain.PayuniStatusFailed {
		t.Errorf("expected failed registration upserted, got %+v", upserted)
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Errorf("no outbox row expected (no forwarded, no payuni_reject), got %v", outboxRepo.Inserted)
	}
}

func TestApprove_AGENT04017_RecoversWithActiveStatus_SetsApproved(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	var upsertedStatuses []domain.PayuniStatus
	payuniRepo.UpsertFn = func(ctx context.Context, reg *domain.PayuniRegistration) error {
		upsertedStatuses = append(upsertedStatuses, reg.Status)
		return nil
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: false, ErrorCode: "AGENT04017", Message: "商店代號已存在"}, nil
			},
			GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
				return &SellerStatusResponse{MerID: merID, MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: realOutcomeApplier(certRepo, outboxRepo),
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("expected self-heal to succeed, got %v", err)
	}
	if savedApp == nil || savedApp.Status != domain.CertStatusForwarded {
		t.Errorf("AGENT04017 自癒後 cert 應停在 forwarded，got %+v", savedApp)
	}
	if len(upsertedStatuses) < 2 ||
		upsertedStatuses[0] != domain.PayuniStatusPending ||
		upsertedStatuses[len(upsertedStatuses)-1] != domain.PayuniStatusActive {
		t.Errorf("expected upsert sequence pending→active (recover defers terminal to funnel), got %v", upsertedStatuses)
	}
	foundApproved := false
	for _, row := range outboxRepo.Inserted {
		if row.EventType == "approved" {
			foundApproved = true
		}
	}
	if !foundApproved {
		t.Errorf("expected approved outbox row inserted via funnel, got %+v", outboxRepo.Inserted)
	}
}

func TestApprove_ForwardedToPayuni_InsertsOutboxRowInSameTx(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{
		ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, SellerType: "individual",
		CreatedAt: now, UpdatedAt: now,
	}
	app.InitFSM()
	var savedApp *domain.CertificationApplication
	certRepo := sharedAppRepo(app, &savedApp)

	payuniRepo := defaultPayuniRepo()
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	fake := &fakeOutcomeApplier{}
	uc := &CertificationUseCase{
		tsid: func() int64 { return 99999 }, certRepo: certRepo,
		sellerRepo: newIndividualSellerRepo(), payuniRepo: payuniRepo,
		payuniClient: &mockPayuniClient{
			BindSellerFn: func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
				return &PayuniResponse{Success: true}, nil
			},
		},
		accountClient: defaultAccountClient(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, payuniOutcome: fake,
	}

	if err := uc.Approve(context.Background(), validReviewReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("expected exactly 1 outbox row inserted, got %d: %+v", len(outboxRepo.Inserted), outboxRepo.Inserted)
	}
	if outboxRepo.Inserted[0].EventType != "forwarded_to_payuni" {
		t.Errorf("EventType = %q, want %q", outboxRepo.Inserted[0].EventType, "forwarded_to_payuni")
	}
	if outboxRepo.Inserted[0].AccountID != 100 {
		t.Errorf("AccountID = %d, want 100", outboxRepo.Inserted[0].AccountID)
	}
}

func TestReject_InsertsOutboxRowWithRejectionReason(t *testing.T) {
	now := time.Now()
	app := &domain.CertificationApplication{ID: 1, AccountID: 100, Status: domain.CertStatusSubmitted, CreatedAt: now, UpdatedAt: now}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) { return app, nil },
		UpdateApplicationFn:  func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
		CreateReviewFn:       func(ctx context.Context, r *domain.CertificationReview) error { return nil },
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &CertificationUseCase{
		certRepo: certRepo, sellerRepo: newIndividualSellerRepo(), txManager: &mockTxManager{},
		outboxRepo: outboxRepo, tsid: func() int64 { return 1 },
	}

	req := &dto.ReviewApplication{ApplicationID: 1, ReviewerID: 99, ReviewerName: "Admin", RejectionReason: "文件不齊全"}
	if err := uc.Reject(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("expected exactly 1 outbox row inserted, got %d: %+v", len(outboxRepo.Inserted), outboxRepo.Inserted)
	}
	if outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("EventType = %q, want %q", outboxRepo.Inserted[0].EventType, "rejected")
	}
	if outboxRepo.Inserted[0].Reason != "文件不齊全" {
		t.Errorf("Reason = %q, want %q", outboxRepo.Inserted[0].Reason, "文件不齊全")
	}
}
