package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

var testAccountClient = &mockAccountClient{
	GetEmailFn: func(ctx context.Context, accountID int64) (string, error) {
		return "test@example.com", nil
	},
	GetAccountInfoFn: func(ctx context.Context, accountID int64) (*AccountInfo, error) {
		return &AccountInfo{Email: "test@example.com", Phone: "0912345678"}, nil
	},
}

func testTSID() int64 { return 700700700 }

func testStoreCode() (int64, error) { return 2_600_000_000, nil }

func TestMerchantUC_GetByAccountID_NotFound(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetProfileByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
			return nil, errors.New("not found")
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	_, err := uc.GetByAccountID(context.Background(), 2699999999)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMerchantUC_GetByAccountID_Success(t *testing.T) {
	now := time.Now()
	sellerRepo := &mockSellerRepo{
		GetProfileByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: accountID, StoreName: "My Shop",
					ContactPhone: "0912345678",
					CreatedAt:    now, UpdatedAt: now,
				},
				CertStatus: domain.OverallCertNotStarted,
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, defaultPayuniRepo(), nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetByAccountID(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccountID != 100 {
		t.Errorf("AccountID = %d, want 100", resp.AccountID)
	}
	if resp.OverallCertificationStatus != domain.OverallCertNotStarted {
		t.Errorf("OverallCertificationStatus = %q, want %q", resp.OverallCertificationStatus, domain.OverallCertNotStarted)
	}
	if resp.ApprovalStatus != "unregistered" {
		t.Errorf("ApprovalStatus = %q, want %q", resp.ApprovalStatus, "unregistered")
	}
	if resp.PayuniStatus != 0 {
		t.Errorf("PayuniStatus = %d, want 0", resp.PayuniStatus)
	}
}

func TestMerchantUC_GetByAccountID_DerivesFromCertApplication(t *testing.T) {
	now := time.Now()
	approvedAt := now.Add(-time.Hour)
	sellerRepo := &mockSellerRepo{
		GetProfileByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: accountID, StoreName: "My Shop",
					ContactPhone: "0912345678",
					SellerType:   domain.SellerTypeIndividual,
					Status:       domain.SellerStatusActive,
					CreatedAt:    now, UpdatedAt: now,
				},
				CertStatus:               domain.CertStatusForwarded.String(),
				PayuniApprovedAt:         &approvedAt,
				PayuniRegistrationStatus: domain.PayuniStatusActive.String(),
				CertSellerType:           domain.SellerTypeBusiness.String(),
				PayuniMerStatus:          "1",
				PayuniMemberStatus:       "1",
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, defaultPayuniRepo(), nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetByAccountID(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OverallCertificationStatus != "approved" {
		t.Errorf("OverallCertificationStatus = %q, want %q", resp.OverallCertificationStatus, "approved")
	}
	if resp.ApprovalStatus != "approved" {
		t.Errorf("ApprovalStatus = %q, want %q", resp.ApprovalStatus, "approved")
	}
	if resp.PayuniStatus != 1 {
		t.Errorf("PayuniStatus = %d, want 1", resp.PayuniStatus)
	}
	if resp.CertifiedAt == nil || !resp.CertifiedAt.Equal(approvedAt) {
		t.Errorf("CertifiedAt = %v, want payuni_registrations.approved_at %v", resp.CertifiedAt, approvedAt)
	}
	if resp.SellerType != domain.SellerTypeBusiness.String() {
		t.Errorf("SellerType = %q, want %q (cert app is the SSOT)", resp.SellerType, "business")
	}
}

func TestMerchantUC_GetByAccountID_BackfillContactPhone(t *testing.T) {
	now := time.Now()
	var updatedPhone string
	sellerRepo := &mockSellerRepo{
		GetProfileByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: accountID, StoreName: "My Shop",
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: domain.OverallCertNotStarted,
			}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			updatedPhone = merchant.ContactPhone
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, defaultPayuniRepo(), nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetByAccountID(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContactPhone != "0912345678" {
		t.Errorf("ContactPhone = %q, want %q", resp.ContactPhone, "0912345678")
	}
	if updatedPhone != "0912345678" {
		t.Errorf("updatedPhone = %q, want %q (backfill should have called Update)", updatedPhone, "0912345678")
	}
}

func TestMerchantUC_GetByStoreID_ClosedStintNoBackfill(t *testing.T) {
	now := time.Now()
	deletedAt := now.Add(-24 * time.Hour)
	var gotID int64
	updateCalled := false
	sellerRepo := &mockSellerRepo{
		GetProfileByStoreIDFn: func(_ context.Context, storeID int64) (*domain.SellerProfileRow, error) {
			gotID = storeID
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID:    2600000001,
					StoreID:      storeID,
					StoreName:    "已歇業商店",
					ContactPhone: "", // 空 phone：若誤走回填路徑會呼叫 Update
					DeletedAt:    &deletedAt,
					CreatedAt:    now, UpdatedAt: now,
				},
				CertStatus: domain.OverallCertNotStarted,
			}, nil
		},
		UpdateFn: func(_ context.Context, _ *domain.Seller) error {
			updateCalled = true
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	p, err := uc.GetByStoreID(context.Background(), 7300000000000000123)
	if err != nil {
		t.Fatalf("GetByStoreID error: %v", err)
	}
	if gotID != 7300000000000000123 {
		t.Errorf("repo queried with %d, want surrogate 7300000000000000123", gotID)
	}
	if p.StoreID != 7300000000000000123 {
		t.Errorf("StoreID = %d, want 7300000000000000123", p.StoreID)
	}
	if p.StoreName != "已歇業商店" {
		t.Errorf("StoreName = %q, want 已歇業商店（closed stint 必須回 profile）", p.StoreName)
	}
	if updateCalled {
		t.Error("closed-stint read 不得觸發 contact_phone 回填寫入")
	}
}

func TestMerchantUC_Create_Success(t *testing.T) {
	var createdSeller *domain.Seller
	sellerRepo := &mockSellerRepo{
		CreateFn: func(ctx context.Context, merchant *domain.Seller) error {
			createdSeller = merchant
			return nil
		},
	}
	outbox := &fakeSellerDeletedOutboxRepo{}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, outbox, nil, testStoreCode, nil)
	resp, err := uc.Create(context.Background(), 100, "New Store", "0912345678")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StoreName != "New Store" {
		t.Errorf("StoreName = %q, want %q", resp.StoreName, "New Store")
	}
	if createdSeller == nil {
		t.Fatal("expected seller to be created")
	}
	if createdSeller.AccountID != 100 {
		t.Errorf("AccountID = %d, want 100", createdSeller.AccountID)
	}
	if createdSeller.StoreID != 700700700 {
		t.Errorf("StoreID = %d, want 700700700 (minted fresh, must never grandfather-equal AccountID even on first-time create)", createdSeller.StoreID)
	}
	if createdSeller.StoreID == createdSeller.AccountID {
		t.Errorf("StoreID must never equal AccountID, even on first-time create: both are %d", createdSeller.StoreID)
	}
	if createdSeller.StoreCode != 2_600_000_000 {
		t.Errorf("StoreCode = %d, want 2_600_000_000 (from testStoreCode fake) - the minted code must land on the persisted seller", createdSeller.StoreCode)
	}
	if outbox.insertCalled != 1 || outbox.inserted.EventType != domain.SellerLifecycleEventOnboarded {
		t.Fatalf("first-time onboard must enqueue seller.onboarded, got calls=%d row=%+v", outbox.insertCalled, outbox.inserted)
	}
}

func TestMerchantUC_Create_RetriesStoreCodeOnCollision(t *testing.T) {
	createCalls := 0
	sellerRepo := &mockSellerRepo{
		CreateFn: func(ctx context.Context, merchant *domain.Seller) error {
			createCalls++
			if createCalls == 1 {
				return &domain.ErrDuplicate{ConstraintName: "uq_sellers_store_code"}
			}
			return nil
		},
	}
	outbox := &fakeSellerDeletedOutboxRepo{}

	storeCodeCalls := 0
	fakeStoreCode := func() (int64, error) {
		storeCodeCalls++
		return 2_600_000_000 + int64(storeCodeCalls), nil
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, outbox, nil, fakeStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "New Store", "0912345678")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalls != 2 {
		t.Fatalf("Create calls = %d, want 2 (1 collision + 1 success)", createCalls)
	}
	if storeCodeCalls != 2 {
		t.Fatalf("storeCode generator calls = %d, want 2", storeCodeCalls)
	}
}

func TestMerchantUC_Create_GivesUpAfterMaxStoreCodeAttempts(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		CreateFn: func(ctx context.Context, merchant *domain.Seller) error {
			return &domain.ErrDuplicate{ConstraintName: "uq_sellers_store_code"}
		},
	}
	storeCodeCalls := 0
	fakeStoreCode := func() (int64, error) {
		storeCodeCalls++
		return 2_600_000_000, nil
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, fakeStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "New Store", "0912345678")

	if err == nil {
		t.Fatal("expected error after exhausting all store_code retry attempts")
	}
	if storeCodeCalls != 5 {
		t.Fatalf("storeCode generator calls = %d, want 5 (maxStoreCodeAttempts)", storeCodeCalls)
	}
}

func TestMerchantUC_Create_DoesNotRetryOnUnrelatedConstraintViolation(t *testing.T) {
	createCalls := 0
	sellerRepo := &mockSellerRepo{
		CreateFn: func(ctx context.Context, merchant *domain.Seller) error {
			createCalls++
			return &domain.ErrDuplicate{ConstraintName: "uq_sellers_account_current"}
		},
	}
	storeCodeCalls := 0
	fakeStoreCode := func() (int64, error) {
		storeCodeCalls++
		return 2_600_000_000, nil
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, fakeStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "New Store", "0912345678")

	if err == nil {
		t.Fatal("expected the original error to propagate")
	}
	var dupErr *domain.ErrDuplicate
	if !errors.As(err, &dupErr) || dupErr.ConstraintName != "uq_sellers_account_current" {
		t.Fatalf("expected original ErrDuplicate{uq_sellers_account_current} to propagate unchanged, got %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("Create calls = %d, want 1 (no retry for a constraint violation unrelated to store_code)", createCalls)
	}
	if storeCodeCalls != 1 {
		t.Fatalf("storeCode generator calls = %d, want 1 (no retry ⇒ no re-mint)", storeCodeCalls)
	}
}

func TestMerchantUC_Create_ReonboardsClosedSeller(t *testing.T) {
	var created *domain.Seller
	retireCalled := false
	outbox := &fakeSellerDeletedOutboxRepo{}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusClosed}, nil
		},
		RetireCurrentStintFn: func(ctx context.Context, accountID int64, _ time.Time) error {
			retireCalled = true
			return nil
		},
		CreateFn: func(ctx context.Context, m *domain.Seller) error { created = m; return nil },
	}

	uc := NewSellerUseCase(sellerRepo, &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 999 }, &mockTxManager{}, outbox, nil, testStoreCode, nil)
	resp, err := uc.Create(context.Background(), 100, "Reopened Store", "0912345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retireCalled {
		t.Fatal("expected the closed stint to be retired (is_current=false)")
	}
	if created == nil {
		t.Fatal("expected a brand-new stint row to be inserted (no in-place mutate)")
	}
	if created.AccountID != 100 {
		t.Errorf("new stint AccountID = %d, want 100", created.AccountID)
	}
	if created.StoreID != 999 {
		t.Errorf("new stint StoreID = %d, want 999 (fresh incarnation mint)", created.StoreID)
	}
	if outbox.insertCalled != 1 || outbox.inserted.EventType != domain.SellerLifecycleEventOnboarded {
		t.Fatalf("expected exactly one seller.onboarded outbox row, got calls=%d row=%+v", outbox.insertCalled, outbox.inserted)
	}
	var p domain.SellerLifecyclePayload
	if err := json.Unmarshal(outbox.inserted.Payload, &p); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if p.AccountID != 100 || p.StoreID != 999 {
		t.Errorf("payload = %+v, want {AccountID:100 StoreID:999}", p)
	}
	if resp.StoreName != "Reopened Store" {
		t.Errorf("store = %q, want Reopened Store", resp.StoreName)
	}
}

func TestMerchantUC_Create_RejectsSuspendedSeller(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusSuspendedGovernance}, nil
		},
		CreateFn:             func(ctx context.Context, m *domain.Seller) error { t.Fatal("Create must not run"); return nil },
		RetireCurrentStintFn: func(ctx context.Context, _ int64, _ time.Time) error { t.Fatal("Retire must not run"); return nil },
	}
	uc := NewSellerUseCase(sellerRepo, &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "Store", "0912345678")
	if err == nil {
		t.Fatal("expected conflict for suspended stint")
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeConflict {
		t.Errorf("error code = %v, want Conflict(409)", code)
	}
}

func TestMerchantUC_Create_RetireFailurePropagates(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusClosed}, nil
		},
		RetireCurrentStintFn: func(ctx context.Context, _ int64, _ time.Time) error {
			return errors.New("retire failed")
		},
		CreateFn: func(ctx context.Context, m *domain.Seller) error {
			t.Fatal("Create must not run when retire fails (tx rolls back)")
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 999 }, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	if _, err := uc.Create(context.Background(), 100, "Reopened Store", "0912345678"); err == nil {
		t.Fatal("expected retire failure to fail Create")
	}
}

func TestMerchantUC_Create_RejectsAlreadyActiveSeller(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, Status: domain.SellerStatusActive}, nil
		},
		CreateFn:             func(ctx context.Context, m *domain.Seller) error { t.Fatal("Create must not run"); return nil },
		RetireCurrentStintFn: func(ctx context.Context, _ int64, _ time.Time) error { t.Fatal("Retire must not run"); return nil },
	}

	uc := NewSellerUseCase(sellerRepo, &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "Store", "0912345678")
	if err == nil {
		t.Fatal("expected conflict error when an active seller already exists")
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeConflict {
		t.Errorf("error code = %v, want Conflict(409)", code)
	}
}

func TestMerchantUC_Create_NormalizesE164Phone(t *testing.T) {
	var createdSeller *domain.Seller
	sellerRepo := &mockSellerRepo{
		CreateFn: func(ctx context.Context, merchant *domain.Seller) error {
			createdSeller = merchant
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	_, err := uc.Create(context.Background(), 100, "Store", "+886912345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdSeller.ContactPhone != "0912345678" {
		t.Errorf("ContactPhone = %q, want %q (+886 prefix must be normalized to local 09 format)", createdSeller.ContactPhone, "0912345678")
	}
}

func TestMerchantUC_UpdateProfile_NormalizesE164Phone(t *testing.T) {
	now := time.Now()
	var saved *domain.Seller
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CreatedAt: now, UpdatedAt: now}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = merchant
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Store",
		ContactPhone: "+886912345678",
		City:         "台北市", District: "中山區", AddressDetail: "中山路1號",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ContactPhone != "0912345678" {
		t.Errorf("ContactPhone = %q, want %q (+886 prefix must be normalized to local 09 format)", saved.ContactPhone, "0912345678")
	}
}

func TestMerchantUC_UpdateProfile_Success(t *testing.T) {
	now := time.Now()
	var saved bool
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CreatedAt: now, UpdatedAt: now}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = true
			if merchant.StoreName != "Updated" {
				t.Errorf("StoreName = %q, want %q", merchant.StoreName, "Updated")
			}
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Updated", Description: "desc",
		ContactPhone: "0912345678", City: "台北市", District: "中山區", AddressDetail: "中山路1號", Prize: "Prize",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !saved {
		t.Error("merchant was not saved")
	}
}

func TestMerchantUC_UpdateProfile_PreservesFieldsWhenEmpty(t *testing.T) {
	now := time.Now()
	existing := &domain.Seller{
		AccountID: 2600000001,
		StoreName: "Old",
		Logo:      "existing-logo.png",
		CoverKey:  "existing-cover.png",
		Birthday:  "1990-01-01",
		Gender:    "M",
		CreatedAt: now,
		UpdatedAt: now,
	}
	var saved *domain.Seller
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return existing, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = merchant
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Updated", Description: "desc",
		ContactPhone: "0912345678", City: "台北市", District: "中山區", AddressDetail: "中山路1號", Prize: "Prize",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved == nil {
		t.Fatal("merchant was not saved")
	}
	if saved.Logo != "existing-logo.png" {
		t.Errorf("Logo = %q, want %q (既有值必須保留)", saved.Logo, "existing-logo.png")
	}
	if saved.CoverKey != "existing-cover.png" {
		t.Errorf("CoverKey = %q, want %q (既有值必須保留)", saved.CoverKey, "existing-cover.png")
	}
	if saved.Birthday != "1990-01-01" {
		t.Errorf("Birthday = %q, want %q (既有值必須保留)", saved.Birthday, "1990-01-01")
	}
	if saved.Gender != "M" {
		t.Errorf("Gender = %q, want %q (既有值必須保留)", saved.Gender, "M")
	}
}

func TestMerchantUC_UpdateProfile_PreservesStoreNameWhenEmpty(t *testing.T) {
	now := time.Now()
	var saved *domain.Seller
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreName: "Hawkr 賣家1234", CreatedAt: now, UpdatedAt: now}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = merchant
			return nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "", Description: "desc",
		ContactPhone: "0912345678", City: "台北市", District: "中山區",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.StoreName != "Hawkr 賣家1234" {
		t.Errorf("StoreName = %q, want %q (空字串不應覆寫既有店名)", saved.StoreName, "Hawkr 賣家1234")
	}
}

func TestMerchantUC_UpdateProfile_SetsWebsiteURL(t *testing.T) {
	now := time.Now()
	var saved *domain.Seller
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CreatedAt: now, UpdatedAt: now}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = merchant
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Store", ContactPhone: "0912345678",
		City: "台北市", District: "中山區", WebsiteURL: "https://my-shop.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.WebsiteURL != "https://my-shop.example.com" {
		t.Errorf("WebsiteURL = %q, want %q", saved.WebsiteURL, "https://my-shop.example.com")
	}
}

func TestMerchantUC_UpdateProfile_ClearsWebsiteURLWhenEmpty(t *testing.T) {
	now := time.Now()
	existing := &domain.Seller{AccountID: 2600000001, WebsiteURL: "https://old.example.com", CreatedAt: now, UpdatedAt: now}
	var saved *domain.Seller
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) { return existing, nil },
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			saved = merchant
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Store", ContactPhone: "0912345678",
		City: "台北市", District: "中山區", WebsiteURL: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.WebsiteURL != "" {
		t.Errorf("WebsiteURL = %q, want empty (explicit clear)", saved.WebsiteURL)
	}
}

func TestMerchantUC_UpdateProfile_RejectsInvalidWebsiteURL(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
		UpdateFn: func(ctx context.Context, merchant *domain.Seller) error {
			t.Fatal("Update must not be called when website_url is invalid")
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target: dto.TargetByAccountID(2600000001), StoreName: "Store", ContactPhone: "0912345678",
		City: "台北市", District: "中山區", WebsiteURL: "not-a-url",
	})
	if pkgerrors.GetCode(err) != pkgerrors.ErrorCode(bizcode.SellerWebsiteUrlInvalid) {
		t.Fatalf("GetCode(err) = %v, want SellerWebsiteUrlInvalid (%d); err=%v", pkgerrors.GetCode(err), bizcode.SellerWebsiteUrlInvalid, err)
	}
}

func TestMerchantUC_GetPayuniStatus_Success(t *testing.T) {
	certID := int64(500)
	registeredAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	lastSyncedAt := time.Date(2026, 8, 29, 3, 0, 0, 0, time.UTC)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CurrentCertificationID: &certID}, nil
		},
	}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, ApplicationID: certID, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusActive, PaymentEnabled: true, MerStatus: "1", MemberStatus: "1",
				Reason:         "身分證影本模糊",
				ApplePayStatus: "1", GooglePayStatus: "0", ATMStatus: "1",
				ErrorMessage: "", RegisteredAt: &registeredAt, ApprovedAt: &approvedAt,
				LastSyncedAt: &lastSyncedAt,
				ContactName:  "王小明", ContactMobile: "0912345678", ContactTel: "",
				MerEmail: "seller@example.com", MerAddr: "台北市大安區某路 1 號",
				MerAddrPost: "106", MerTel: "", MerchantProfileVersion: 3,
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, payuniRepo, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetPayuniStatus(context.Background(), 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PayuniMerID != "HAWK2612345678" {
		t.Errorf("PayuniMerID = %q, want %q", resp.PayuniMerID, "HAWK2612345678")
	}
	if !resp.PaymentEnabled {
		t.Error("PaymentEnabled = false, want true")
	}
	if resp.Reason != "身分證影本模糊" {
		t.Errorf("Reason = %q, want %q", resp.Reason, "身分證影本模糊")
	}
	if resp.ApplePayStatus != "1" {
		t.Errorf("ApplePayStatus = %q, want %q", resp.ApplePayStatus, "1")
	}
	if resp.GooglePayStatus != "0" {
		t.Errorf("GooglePayStatus = %q, want %q", resp.GooglePayStatus, "0")
	}
	if resp.ATMStatus != "1" {
		t.Errorf("ATMStatus = %q, want %q", resp.ATMStatus, "1")
	}
	if resp.RegisteredAt == nil || !resp.RegisteredAt.Equal(registeredAt) {
		t.Errorf("RegisteredAt = %v, want %v", resp.RegisteredAt, registeredAt)
	}
	if resp.ApprovedAt == nil || !resp.ApprovedAt.Equal(approvedAt) {
		t.Errorf("ApprovedAt = %v, want %v", resp.ApprovedAt, approvedAt)
	}
	if resp.LastSyncedAt == nil || !resp.LastSyncedAt.Equal(lastSyncedAt) {
		t.Errorf("LastSyncedAt = %v, want %v", resp.LastSyncedAt, lastSyncedAt)
	}
	if resp.MerchantProfileVersion != 3 {
		t.Errorf("MerchantProfileVersion = %d, want 3", resp.MerchantProfileVersion)
	}
	if resp.ContactName != "王小明" {
		t.Errorf("ContactName = %q, want %q", resp.ContactName, "王小明")
	}
	if resp.ContactMobile != "0912345678" {
		t.Errorf("ContactMobile = %q, want %q", resp.ContactMobile, "0912345678")
	}
	if resp.MerEmail != "seller@example.com" {
		t.Errorf("MerEmail = %q, want %q", resp.MerEmail, "seller@example.com")
	}
	if resp.MerAddr != "台北市大安區某路 1 號" {
		t.Errorf("MerAddr = %q, want %q", resp.MerAddr, "台北市大安區某路 1 號")
	}
	if resp.MerAddrPost != "106" {
		t.Errorf("MerAddrPost = %q, want %q", resp.MerAddrPost, "106")
	}
	if len(resp.AllowedActions) != 1 || resp.AllowedActions[0] != "edit_merchant_profile" {
		t.Errorf("AllowedActions = %v, want [edit_merchant_profile]", resp.AllowedActions)
	}
}

func TestMerchantUC_GetPayuniStatus_NotFound(t *testing.T) {
	certID := int64(500)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CurrentCertificationID: &certID}, nil
		},
	}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return nil, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, payuniRepo, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	_, err := uc.GetPayuniStatus(context.Background(), 100)

	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestMerchantUC_GetPayuniStatus_StaleRegistration_NoCurrentApplication(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CurrentCertificationID: nil}, nil
		},
	}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, ApplicationID: 999, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusActive, PaymentEnabled: true, MerStatus: "1",
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, payuniRepo, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	_, err := uc.GetPayuniStatus(context.Background(), 100)

	if err == nil {
		t.Fatal("expected not-found error for stale registration with no current application")
	}
	if !pkgerrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestMerchantUC_GetPayuniStatus_StaleRegistration_MismatchedApplication(t *testing.T) {
	currentCertID := int64(500)
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, CurrentCertificationID: &currentCertID}, nil
		},
	}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, ApplicationID: 111,
				PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusActive,
				PaymentEnabled: true, MerStatus: "1",
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, payuniRepo, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	_, err := uc.GetPayuniStatus(context.Background(), 100)

	if err == nil {
		t.Fatal("expected not-found error for stale registration belonging to a different application")
	}
	if !pkgerrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func fullSellerForShipping() *domain.Seller {
	return &domain.Seller{
		AccountID:     100,
		StoreID:       100,
		StoreName:     "My Shop",
		Email:         "shop@example.com",
		ContactPhone:  "0912345678",
		City:          "台北市",
		District:      "中山區",
		AddressDetail: "中山路1號",
		PostalCode:    "10401",
	}
}

func shippingRepoWith(m *domain.Seller) *mockSellerRepo {
	return &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return m, nil
		},
	}
}

func TestGetShippingProfile_AllFieldsPresent(t *testing.T) {
	uc := NewSellerUseCase(shippingRepoWith(fullSellerForShipping()), &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetShippingProfile(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContactEmail != "shop@example.com" {
		t.Errorf("ContactEmail = %q, want %q", resp.ContactEmail, "shop@example.com")
	}
	if resp.ZipCode != "10401" {
		t.Errorf("ZipCode = %q, want %q", resp.ZipCode, "10401")
	}
}

func TestGetShippingProfile_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*domain.Seller)
		wantField string
	}{
		{"missing sender_phone", func(m *domain.Seller) { m.ContactPhone = "" }, "sender_phone"},
		{"missing city", func(m *domain.Seller) { m.City = "" }, "city"},
		{"missing district", func(m *domain.Seller) { m.District = "" }, "district"},
		{"missing address_detail", func(m *domain.Seller) { m.AddressDetail = "" }, "address_detail"},
		{"missing zip_code", func(m *domain.Seller) { m.PostalCode = "" }, "zip_code"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fullSellerForShipping()
			tc.mutate(m)
			uc := NewSellerUseCase(shippingRepoWith(m), &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
			_, err := uc.GetShippingProfile(context.Background(), 100, 0)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
				t.Errorf("error code = %v, want CodePreconditionFailed", pkgerrors.GetCode(err))
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("error message %q does not contain field name %q", err.Error(), tc.wantField)
			}
		})
	}
}

func TestGetShippingProfile_MultipleFieldsMissing(t *testing.T) {
	m := fullSellerForShipping()
	m.ContactPhone = ""
	m.City = ""
	m.PostalCode = ""

	uc := NewSellerUseCase(shippingRepoWith(m), &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	_, err := uc.GetShippingProfile(context.Background(), 100, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("error code = %v, want CodePreconditionFailed", pkgerrors.GetCode(err))
	}
	for _, field := range []string{"sender_phone", "city", "zip_code"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error message %q does not contain field name %q", err.Error(), field)
		}
	}
}

func TestGetShippingProfile_MerIDDerivesFromStoreCode(t *testing.T) {
	const longStoreID int64 = 7300000000000000123
	const shortStoreCode int64 = 2612345678

	m := &domain.Seller{
		AccountID: 100, StoreID: longStoreID, StoreCode: shortStoreCode,
		StoreName: "My Shop", Email: "shop@example.com", ContactPhone: "0912345678",
		City: "台北市", District: "中山區", AddressDetail: "中山路1號", PostalCode: "10401",
	}
	uc := NewSellerUseCase(shippingRepoWith(m), &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetShippingProfile(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MerID != "HAWK2612345678" {
		t.Errorf("MerID = %q, want %q (must derive from store_code, not store_id/account_id)", resp.MerID, "HAWK2612345678")
	}
	if resp.MerID == "HAWK7300000000000000123" {
		t.Errorf("MerID = %q, regressed to deriving from store_id", resp.MerID)
	}
	if resp.StoreID != "7300000000000000123" {
		t.Errorf("StoreID = %q, want %q", resp.StoreID, "7300000000000000123")
	}
}

func TestGetShippingProfile_ByStoreID_UsesPublicProfile(t *testing.T) {
	const accountID int64 = 100
	const storeID int64 = 8001

	fixture := &domain.Seller{
		AccountID: accountID, StoreID: storeID, StoreCode: 2612345678,
		StoreName: "My Shop", Email: "shop@example.com", ContactPhone: "0912345678",
		City: "台北市", District: "中山區", AddressDetail: "中山路1號", PostalCode: "10401",
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, gotAccountID int64) (*domain.Seller, error) {
			t.Fatalf("GetByAccountID should not be called when storeID > 0, got accountID=%d", gotAccountID)
			return nil, nil
		},
		GetPublicProfileByStoreIDOrCodeFn: func(ctx context.Context, gotStoreID int64) (*domain.SellerProfileRow, error) {
			if gotStoreID != storeID {
				t.Errorf("GetPublicProfileByStoreIDOrCode called with %d, want %d", gotStoreID, storeID)
			}
			return &domain.SellerProfileRow{Seller: fixture, CertStatus: domain.OverallCertNotStarted}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, &mockPayuniRepo{}, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	resp, err := uc.GetShippingProfile(context.Background(), accountID, storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContactEmail != "shop@example.com" {
		t.Errorf("ContactEmail = %q, want %q", resp.ContactEmail, "shop@example.com")
	}
	if resp.StoreID != "8001" {
		t.Errorf("StoreID = %q, want %q", resp.StoreID, "8001")
	}
}

func TestMerchantUC_BatchSellerMeta_Empty(t *testing.T) {
	uc := NewSellerUseCase(nil, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	result, err := uc.BatchGetSellerMeta(context.Background(), []int64{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestMerchantUC_BatchSellerMeta_Success(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetSellerMetaByStoreIDsFn: func(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error) {
			return []*domain.SellerMetaRow{
				{AccountID: 100, StoreID: 100, StoreName: "Shop A", Logo: "a.png", CertStatus: domain.CertStatusForwarded.String(), Status: string(domain.SellerStatusActive)},
				{AccountID: 200, StoreID: 999, StoreName: "Shop B (reonboarded)", Logo: "b.png", CertStatus: domain.CertStatusForwarded.String(), Status: string(domain.SellerStatusActive)},
			}, nil
		},
	}

	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	result, err := uc.BatchGetSellerMeta(context.Background(), []int64{100, 999})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[100] == nil || result[100].SellerName != "Shop A" {
		t.Errorf("result[100] = %+v, want Shop A (grandfathered key)", result[100])
	}
	if result[999] == nil || result[999].SellerName != "Shop B (reonboarded)" {
		t.Fatalf("result[999] = %+v, want Shop B keyed by store_id", result[999])
	}
	if result[999].AccountID != 200 || result[999].StoreID != 999 {
		t.Errorf("result[999] ids = {%d %d}, want {200 999}", result[999].AccountID, result[999].StoreID)
	}
}

func TestBatchGetSellerMeta_MerIDDerivesFromStoreCode(t *testing.T) {
	const longStoreID int64 = 7300000000000000123
	const shortStoreCode int64 = 2612345678

	sellerRepo := &mockSellerRepo{
		GetSellerMetaByStoreIDsFn: func(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error) {
			return []*domain.SellerMetaRow{
				{AccountID: 100, StoreID: longStoreID, StoreCode: shortStoreCode, StoreName: "My Shop", CertStatus: "approved", Status: "active"},
			}, nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)
	metas, err := uc.BatchGetSellerMeta(context.Background(), []int64{100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	meta, ok := metas[longStoreID]
	if !ok {
		t.Fatalf("meta must be keyed by store_id, got %+v", metas)
	}
	if meta.PayuniMerID != "HAWK2612345678" {
		t.Errorf("PayuniMerID = %q, want %q (must derive from store_code, not store_id/account_id)", meta.PayuniMerID, "HAWK2612345678")
	}
	if meta.PayuniMerID == "HAWK7300000000000000123" {
		t.Errorf("PayuniMerID = %q, regressed to deriving from store_id", meta.PayuniMerID)
	}
}

func TestSellerUC_UpdateSEOFields(t *testing.T) {
	var gotID int64
	var gotTitle, gotDesc string
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, StoreName: "Shop"}, nil
		},
		UpdateSEOFn: func(ctx context.Context, id int64, title, desc string) error {
			gotID, gotTitle, gotDesc = id, title, desc
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	err := uc.UpdateSEOFields(context.Background(), 100, "新標題", "新描述")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != 100 || gotTitle != "新標題" || gotDesc != "新描述" {
		t.Errorf("UpdateSEO got (%d,%q,%q), want (100,新標題,新描述)", gotID, gotTitle, gotDesc)
	}
}

func TestSellerUC_UpdateFoodBusinessRegistration_Success(t *testing.T) {
	var gotFood bool
	var gotReg string
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Status: "active"}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, id int64, food bool, reg string, _ time.Time) error {
			gotFood, gotReg = food, reg
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	food, reg, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), 100, true, "D-200197022-00001-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !food || reg != "D-200197022-00001-4" || !gotFood || gotReg != "D-200197022-00001-4" {
		t.Errorf("got (%v,%q) persisted (%v,%q)", food, reg, gotFood, gotReg)
	}
}

func TestSellerUC_UpdateFoodBusinessRegistration_ClearsWhenFalse(t *testing.T) {
	gotReg := "SENTINEL"
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Status: "active"}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, id int64, food bool, reg string, _ time.Time) error {
			gotReg = reg
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	_, reg, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), 100, false, "STALE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg != "" || gotReg != "" {
		t.Errorf("reg not cleared: returned %q persisted %q", reg, gotReg)
	}
}

func TestSellerUC_UpdateFoodBusinessRegistration_RejectsInactive(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Status: "suspended"}, nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	if _, _, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), 100, true, "D-1"); err == nil {
		t.Fatal("expected error for inactive seller")
	}
}

func TestSellerUC_UpdateFoodBusinessRegistration_RejectsEmptyRegWhenTrue(t *testing.T) {
	writeCalled := false
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Status: "active"}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, id int64, food bool, reg string, _ time.Time) error {
			writeCalled = true
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	for _, regNo := range []string{"", "   "} {
		_, _, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), 100, true, regNo)
		if err == nil {
			t.Fatalf("expected error for isProcessedFood=true with reg %q", regNo)
		}
		if pkgerrors.GetCode(err) != pkgerrors.ErrorCode(bizcode.FoodBusinessRegNoInvalid) {
			t.Fatalf("GetCode(err) = %v, want FoodBusinessRegNoInvalid (%d); err=%v", pkgerrors.GetCode(err), bizcode.FoodBusinessRegNoInvalid, err)
		}
	}
	if writeCalled {
		t.Error("repo write fn should not be called when reg is empty")
	}
}

func TestSellerUC_UpdateFoodBusinessRegistration_RejectsOverLongRegWhenTrue(t *testing.T) {
	writeCalled := false
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Status: "active"}, nil
		},
		UpdateFoodBusinessRegistrationFn: func(ctx context.Context, id int64, food bool, reg string, _ time.Time) error {
			writeCalled = true
			return nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	overLong := strings.Repeat("A", 51)
	_, _, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), 100, true, overLong)
	if err == nil {
		t.Fatal("expected error for over-long reg (51 runes)")
	}
	if pkgerrors.GetCode(err) != pkgerrors.ErrorCode(bizcode.FoodBusinessRegNoInvalid) {
		t.Fatalf("GetCode(err) = %v, want FoodBusinessRegNoInvalid (%d); err=%v", pkgerrors.GetCode(err), bizcode.FoodBusinessRegNoInvalid, err)
	}
	if writeCalled {
		t.Error("repo write fn should not be called when reg is over-long")
	}
}

func TestMerchantUC_ListForAdmin_StoreCode(t *testing.T) {
	sellerRepo := &mockSellerRepo{
		ListForAdminFn: func(ctx context.Context, filter domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
			return []*domain.AdminSellerRow{
				{Seller: domain.Seller{AccountID: 42, StoreName: "老王鮮魚", StoreCode: 2612345678}},
			}, nil
		},
		CountForAdminFn:    func(ctx context.Context, filter domain.SellerListFilter) (int64, error) { return 1, nil },
		GetStatsForAdminFn: func(ctx context.Context) (*domain.SellerStats, error) { return &domain.SellerStats{}, nil },
	}
	uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, nil, nil)

	resp, err := uc.ListForAdmin(context.Background(), &dto.AdminListSellersRequest{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 1 {
		t.Fatalf("len(resp.List) = %d, want 1", len(resp.List))
	}
	if resp.List[0].StoreCode != 2612345678 {
		t.Errorf("StoreCode = %d, want 2612345678", resp.List[0].StoreCode)
	}
}

func TestComputePayuniFreshness(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		status       domain.PayuniStatus
		outcome      payuniOutcome
		lastSyncedAt *time.Time
		want         string
	}{
		{"active + approved is not_applicable regardless of age", domain.PayuniStatusActive, outcomeApproved, timePtr(now.Add(-48 * time.Hour)), "not_applicable"},
		{"failed + rejected is not_applicable regardless of age", domain.PayuniStatusFailed, outcomeRejected, timePtr(now.Add(-48 * time.Hour)), "not_applicable"},
		{"active + pending_info is measured like pending", domain.PayuniStatusActive, outcomePendingInfo, timePtr(now.Add(-payuniPollMinInterval - time.Minute)), "stale"},
		{"failed + under_review is measured like pending", domain.PayuniStatusFailed, outcomeUnderReview, nil, "very_stale"},
		{"pending, synced 1 minute ago, is fresh", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-1 * time.Minute)), "fresh"},
		{"pending, just inside the poll window, is fresh", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniPollMinInterval + time.Minute)), "fresh"},
		{"pending, just past the poll window, is stale", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniPollMinInterval - time.Minute)), "stale"},
		{"pending, just inside the very_stale threshold, is stale", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniFreshnessStaleThreshold + time.Hour)), "stale"},
		{"pending, past the very_stale threshold, is very_stale", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniFreshnessStaleThreshold - time.Hour)), "very_stale"},
		{"pending, never synced, is very_stale", domain.PayuniStatusPending, "", nil, "very_stale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePayuniFreshness(tt.status, tt.outcome, tt.lastSyncedAt, now)
			if got != tt.want {
				t.Errorf("computePayuniFreshness() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputePayuniAllowedActions(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		status       domain.PayuniStatus
		outcome      payuniOutcome
		lastSyncedAt *time.Time
		want         []string
	}{
		{"active + approved allows edit_merchant_profile only", domain.PayuniStatusActive, outcomeApproved, nil, []string{"edit_merchant_profile"}},
		{"active + pending_info past the manual window allows resync too", domain.PayuniStatusActive, outcomePendingInfo, timePtr(now.Add(-payuniManualRefreshMinInterval - time.Minute)), []string{"edit_merchant_profile", "resync"}},
		{"active + pending_info inside the manual window has no resync yet", domain.PayuniStatusActive, outcomePendingInfo, timePtr(now.Add(-time.Minute)), []string{"edit_merchant_profile"}},
		{"failed + under_review allows resync", domain.PayuniStatusFailed, outcomeUnderReview, nil, []string{"resync"}},
		{"pending, synced 1 minute ago, no resync yet", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-1 * time.Minute)), nil},
		{"pending, just inside the manual window, no resync yet", domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniManualRefreshMinInterval + time.Minute)), nil},
		{"pending, past the manual window but well inside the poll window, resync allowed",
			domain.PayuniStatusPending, outcomeUnderReview, timePtr(now.Add(-payuniManualRefreshMinInterval - time.Minute)), []string{"resync"}},
		{"pending, never synced, resync allowed", domain.PayuniStatusPending, "", nil, []string{"resync"}},
		{"failed + rejected has no actions", domain.PayuniStatusFailed, outcomeRejected, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePayuniAllowedActions(tt.status, tt.outcome, tt.lastSyncedAt, now)
			if len(got) != len(tt.want) {
				t.Fatalf("computePayuniAllowedActions() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("computePayuniAllowedActions()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func sqlReconcilePredicateMirror(status domain.PayuniStatus, mer, member string) bool {
	in := func(v string, set ...string) bool {
		for _, s := range set {
			if v == s {
				return true
			}
		}
		return false
	}
	return status == domain.PayuniStatusPending ||
		mer == "91" ||
		(mer == "9" && !in(member, "8", "92", "99")) ||
		(mer == "1" && !in(member, "1", "8", "92", "99")) ||
		(!in(mer, "1", "2", "8", "9", "91", "92", "99") && member == "5")
}

func TestReconcilePredicate_MirrorsDerivePayuniOutcome(t *testing.T) {
	codes := []string{"", "1", "2", "5", "8", "9", "77", "91", "92", "99"}
	for _, status := range []domain.PayuniStatus{domain.PayuniStatusActive, domain.PayuniStatusFailed, domain.PayuniStatusPending} {
		for _, mer := range codes {
			for _, member := range codes {
				want := payuniNeedsReconciliation(status, derivePayuniOutcome(mer, member))
				got := sqlReconcilePredicateMirror(status, mer, member)
				if got != want {
					t.Errorf("status=%s mer=%q member=%q: SQL predicate=%v, derive says needsReconciliation=%v", status, mer, member, got, want)
				}
			}
		}
	}
}

func timePtr(t time.Time) *time.Time { return &t }
