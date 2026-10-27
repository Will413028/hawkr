package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func defaultPayuniRepo() *mockPayuniRepo {
	return &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return nil, pkgerrors.NewNotFound("payuni registration not found")
		},
		UpsertFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		CreateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
}

func defaultAccountClient() *mockAccountClient {
	return &mockAccountClient{
		GetEmailFn: func(ctx context.Context, accountID int64) (string, error) {
			return "test@example.com", nil
		},
		GetAccountInfoFn: func(ctx context.Context, accountID int64) (*AccountInfo, error) {
			return &AccountInfo{Email: "test@example.com", Phone: "0912345678"}, nil
		},
	}
}

type mockBuyerClient struct {
	GetSubscriberCountFn func(ctx context.Context, storeID int64) (int64, error)
}

func (m *mockBuyerClient) GetSubscriberCount(ctx context.Context, storeID int64) (int64, error) {
	if m.GetSubscriberCountFn != nil {
		return m.GetSubscriberCountFn(ctx, storeID)
	}
	return 0, nil
}

func (m *mockBuyerClient) BatchGetSubscriberCounts(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func (m *mockBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

type mockCertificationRepo struct {
	GetApplicationByIDFn      func(ctx context.Context, id int64) (*domain.CertificationApplication, error)
	GetApplicationByNoFn      func(ctx context.Context, no string) (*domain.CertificationApplication, error)
	GetApplicationsBySellerFn func(ctx context.Context, accountID int64) ([]*domain.CertificationApplication, error)
	GetCurrentApplicationFn   func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error)
	GetDraftApplicationFn     func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error)
	GetCertHolderNamesFn      func(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error)
	CreateApplicationFn       func(ctx context.Context, app *domain.CertificationApplication) error
	UpdateApplicationFn       func(ctx context.Context, app *domain.CertificationApplication) error
	ListForAdminFn            func(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error)
	CountForAdminFn           func(ctx context.Context, filter domain.CertificationListFilter) (int64, error)
	GetStatsForAdminFn        func(ctx context.Context) (*domain.CertificationStats, error)
	GetReviewsByApplicationFn func(ctx context.Context, applicationID int64) ([]*domain.CertificationReview, error)
	GetLatestReviewFn         func(ctx context.Context, applicationID int64) (*domain.CertificationReview, error)
	CreateReviewFn            func(ctx context.Context, review *domain.CertificationReview) error
}

func (m *mockCertificationRepo) GetApplicationByID(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
	return m.GetApplicationByIDFn(ctx, id)
}
func (m *mockCertificationRepo) GetApplicationByNo(ctx context.Context, no string) (*domain.CertificationApplication, error) {
	return m.GetApplicationByNoFn(ctx, no)
}
func (m *mockCertificationRepo) GetApplicationsBySeller(ctx context.Context, accountID int64) ([]*domain.CertificationApplication, error) {
	return m.GetApplicationsBySellerFn(ctx, accountID)
}
func (m *mockCertificationRepo) GetCurrentApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	return m.GetCurrentApplicationFn(ctx, accountID)
}
func (m *mockCertificationRepo) GetDraftApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	return m.GetDraftApplicationFn(ctx, accountID)
}
func (m *mockCertificationRepo) GetCertHolderNames(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error) {
	if m.GetCertHolderNamesFn != nil {
		return m.GetCertHolderNamesFn(ctx, storeID)
	}
	return domain.CertHolderNames{}, false, nil
}
func (m *mockCertificationRepo) CreateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	return m.CreateApplicationFn(ctx, app)
}
func (m *mockCertificationRepo) UpdateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	return m.UpdateApplicationFn(ctx, app)
}
func (m *mockCertificationRepo) ListForAdmin(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error) {
	return m.ListForAdminFn(ctx, filter)
}
func (m *mockCertificationRepo) CountForAdmin(ctx context.Context, filter domain.CertificationListFilter) (int64, error) {
	return m.CountForAdminFn(ctx, filter)
}
func (m *mockCertificationRepo) GetStatsForAdmin(ctx context.Context) (*domain.CertificationStats, error) {
	return m.GetStatsForAdminFn(ctx)
}
func (m *mockCertificationRepo) GetReviewsByApplication(ctx context.Context, applicationID int64) ([]*domain.CertificationReview, error) {
	return m.GetReviewsByApplicationFn(ctx, applicationID)
}
func (m *mockCertificationRepo) GetLatestReview(ctx context.Context, applicationID int64) (*domain.CertificationReview, error) {
	return m.GetLatestReviewFn(ctx, applicationID)
}
func (m *mockCertificationRepo) CreateReview(ctx context.Context, review *domain.CertificationReview) error {
	return m.CreateReviewFn(ctx, review)
}

type mockSellerRepo struct {
	GetSellerMetaByStoreIDsFn         func(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error)
	GetPrincipalsByStoreIDsFn         func(ctx context.Context, ids []int64) (map[int64]int64, []int64, error)
	GetByAccountIDFn                  func(ctx context.Context, accountID int64) (*domain.Seller, error)
	GetByAccountIDForUpdateFn         func(ctx context.Context, accountID int64) (*domain.Seller, error)
	GetCurrentByStoreIDFn             func(ctx context.Context, storeID int64) (*domain.Seller, error)
	UpdateLogoFn                      func(ctx context.Context, storeID int64, logoKey string) error
	UpdateCoverFn                     func(ctx context.Context, storeID int64, coverKey string) error
	LastExpectedVersion               *int64
	LastWriteStoreID                  int64
	GetProfileByAccountIDFn           func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error)
	GetPublicProfileByStoreIDOrCodeFn func(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error)
	GetAccountIDByStoreIDFn           func(ctx context.Context, storeID int64) (int64, error)
	GetProfileByStoreIDFn             func(ctx context.Context, storeID int64) (*domain.SellerProfileRow, error)
	GetByStoreIDsFn                   func(ctx context.Context, storeIDs []int64) ([]*domain.Seller, error)
	GetApprovedExcludingFn            func(ctx context.Context, excludeStoreIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error)
	CreateFn                          func(ctx context.Context, merchant *domain.Seller) error
	UpdateFn                          func(ctx context.Context, merchant *domain.Seller) error
	RetireCurrentStintFn              func(ctx context.Context, accountID int64, now time.Time) error
	UpdateStatusByStoreIDFn           func(ctx context.Context, storeID int64, status domain.SellerStatus, unlinkedAt *time.Time, deletedAt *time.Time, now time.Time) error
	UpdateStatusCurrentByAccountIDFn  func(ctx context.Context, accountID int64, status domain.SellerStatus, now time.Time) error
	UpdateCurrentCertIDFn             func(ctx context.Context, accountID int64, certID int64) error
	CompleteOnboardingFn              func(ctx context.Context, accountID int64) error
	ListForAdminFn                    func(ctx context.Context, filter domain.SellerListFilter) ([]*domain.AdminSellerRow, error)
	CountForAdminFn                   func(ctx context.Context, filter domain.SellerListFilter) (int64, error)
	GetStatsForAdminFn                func(ctx context.Context) (*domain.SellerStats, error)
	UpdateSEOFn                       func(ctx context.Context, accountID int64, seoTitle, seoDescription string) error
	UpdateFoodBusinessRegistrationFn  func(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error
	GetStoreIDsByAccountIDFn          func(ctx context.Context, accountID int64) ([]int64, error)
}

func (m *mockSellerRepo) GetPublicStoreIDsByCity(context.Context, []string) ([]int64, error) {
	return nil, nil
}
func (m *mockSellerRepo) GetPublicStoreIDsGroupedByCity(context.Context) (map[string][]int64, error) {
	return map[string][]int64{}, nil
}

func (m *mockSellerRepo) GetSellerMetaByStoreIDs(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error) {
	return m.GetSellerMetaByStoreIDsFn(ctx, storeIDs)
}
func (m *mockSellerRepo) GetPrincipalsByStoreIDs(ctx context.Context, ids []int64) (map[int64]int64, []int64, error) {
	if m.GetPrincipalsByStoreIDsFn != nil {
		return m.GetPrincipalsByStoreIDsFn(ctx, ids)
	}
	return nil, nil, nil
}
func (m *mockSellerRepo) GetByAccountID(ctx context.Context, accountID int64) (*domain.Seller, error) {
	return m.GetByAccountIDFn(ctx, accountID)
}

func (m *mockSellerRepo) GetCurrentByStoreID(ctx context.Context, storeID int64) (*domain.Seller, error) {
	if m.GetCurrentByStoreIDFn == nil {
		return nil, fmt.Errorf("mockSellerRepo: GetCurrentByStoreIDFn not set (store_id=%d)", storeID)
	}
	return m.GetCurrentByStoreIDFn(ctx, storeID)
}
func (m *mockSellerRepo) GetProfileByAccountID(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
	if m.GetProfileByAccountIDFn != nil {
		return m.GetProfileByAccountIDFn(ctx, accountID)
	}
	s, err := m.GetByAccountIDFn(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &domain.SellerProfileRow{Seller: s, CertStatus: domain.OverallCertNotStarted}, nil
}
func (m *mockSellerRepo) GetPublicProfileByStoreIDOrCode(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
	if m.GetPublicProfileByStoreIDOrCodeFn != nil {
		return m.GetPublicProfileByStoreIDOrCodeFn(ctx, storeIDOrCode)
	}
	return nil, pkgerrors.NewNotFound("seller not found")
}
func (m *mockSellerRepo) GetAccountIDByStoreID(ctx context.Context, storeID int64) (int64, error) {
	if m.GetAccountIDByStoreIDFn != nil {
		return m.GetAccountIDByStoreIDFn(ctx, storeID)
	}
	return storeID, nil
}
func (m *mockSellerRepo) GetProfileByStoreID(ctx context.Context, storeID int64) (*domain.SellerProfileRow, error) {
	if m.GetProfileByStoreIDFn != nil {
		return m.GetProfileByStoreIDFn(ctx, storeID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerRepo) GetByStoreIDs(ctx context.Context, storeIDs []int64) ([]*domain.Seller, error) {
	if m.GetByStoreIDsFn != nil {
		return m.GetByStoreIDsFn(ctx, storeIDs)
	}
	return nil, nil
}
func (m *mockSellerRepo) GetApprovedExcluding(ctx context.Context, excludeStoreIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
	if m.GetApprovedExcludingFn != nil {
		return m.GetApprovedExcludingFn(ctx, excludeStoreIDs, withPublicProducts, limit)
	}
	return nil, nil
}
func (m *mockSellerRepo) Create(ctx context.Context, merchant *domain.Seller) error {
	return m.CreateFn(ctx, merchant)
}
func (m *mockSellerRepo) Update(ctx context.Context, merchant *domain.Seller, expectedVersion *int64) error {
	m.LastExpectedVersion = expectedVersion
	m.LastWriteStoreID = merchant.StoreID
	return m.UpdateFn(ctx, merchant)
}
func (m *mockSellerRepo) RetireCurrentStint(ctx context.Context, accountID int64, now time.Time) error {
	if m.RetireCurrentStintFn != nil {
		return m.RetireCurrentStintFn(ctx, accountID, now)
	}
	return nil
}
func (m *mockSellerRepo) UpdateStatusByStoreID(ctx context.Context, storeID int64, status domain.SellerStatus, unlinkedAt *time.Time, deletedAt *time.Time, now time.Time) error {
	if m.UpdateStatusByStoreIDFn != nil {
		return m.UpdateStatusByStoreIDFn(ctx, storeID, status, unlinkedAt, deletedAt, now)
	}
	return nil
}
func (m *mockSellerRepo) UpdateStatusCurrentByAccountID(ctx context.Context, accountID int64, status domain.SellerStatus, now time.Time) error {
	if m.UpdateStatusCurrentByAccountIDFn != nil {
		return m.UpdateStatusCurrentByAccountIDFn(ctx, accountID, status, now)
	}
	return nil
}
func (m *mockSellerRepo) UpdateCurrentCertificationID(ctx context.Context, accountID int64, certID int64) error {
	if m.UpdateCurrentCertIDFn != nil {
		return m.UpdateCurrentCertIDFn(ctx, accountID, certID)
	}
	return nil
}
func (m *mockSellerRepo) CompleteOnboarding(ctx context.Context, accountID int64) error {
	if m.CompleteOnboardingFn != nil {
		return m.CompleteOnboardingFn(ctx, accountID)
	}
	return nil
}
func (m *mockSellerRepo) ListForAdmin(ctx context.Context, filter domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
	return m.ListForAdminFn(ctx, filter)
}
func (m *mockSellerRepo) CountForAdmin(ctx context.Context, filter domain.SellerListFilter) (int64, error) {
	return m.CountForAdminFn(ctx, filter)
}
func (m *mockSellerRepo) GetStatsForAdmin(ctx context.Context) (*domain.SellerStats, error) {
	return m.GetStatsForAdminFn(ctx)
}
func (m *mockSellerRepo) GetByAccountIDForUpdate(ctx context.Context, accountID int64) (*domain.Seller, error) {
	if m.GetByAccountIDForUpdateFn != nil {
		return m.GetByAccountIDForUpdateFn(ctx, accountID)
	}
	if m.GetByAccountIDFn != nil {
		return m.GetByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}
func (m *mockSellerRepo) ClearPIIFields(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (m *mockSellerRepo) ListPendingPIICleanup(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	return nil, nil
}
func (m *mockSellerRepo) UpdateLogo(ctx context.Context, storeID int64, logoKey string, expectedVersion *int64) error {
	m.LastExpectedVersion = expectedVersion
	m.LastWriteStoreID = storeID
	if m.UpdateLogoFn != nil {
		return m.UpdateLogoFn(ctx, storeID, logoKey)
	}
	return nil
}
func (m *mockSellerRepo) UpdateCover(ctx context.Context, storeID int64, coverKey string, expectedVersion *int64) error {
	m.LastExpectedVersion = expectedVersion
	m.LastWriteStoreID = storeID
	if m.UpdateCoverFn != nil {
		return m.UpdateCoverFn(ctx, storeID, coverKey)
	}
	return nil
}
func (m *mockSellerRepo) UpdateSEO(ctx context.Context, accountID int64, seoTitle, seoDescription string) error {
	if m.UpdateSEOFn != nil {
		return m.UpdateSEOFn(ctx, accountID, seoTitle, seoDescription)
	}
	return nil
}
func (m *mockSellerRepo) UpdateRatingByStoreID(ctx context.Context, storeID int64, rating float64) error {
	return nil
}
func (m *mockSellerRepo) UpdateFoodBusinessRegistration(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error {
	if m.UpdateFoodBusinessRegistrationFn != nil {
		return m.UpdateFoodBusinessRegistrationFn(ctx, accountID, isProcessedFood, regNo, updatedAt)
	}
	return nil
}
func (m *mockSellerRepo) GetStoreIDsByAccountID(ctx context.Context, accountID int64) ([]int64, error) {
	if m.GetStoreIDsByAccountIDFn != nil {
		return m.GetStoreIDsByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

type mockPayuniRepo struct {
	GetByAccountIDFn                   func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error)
	GetByStoreIDFn                     func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error)
	GetByMerIDFn                       func(ctx context.Context, merID string) (*domain.PayuniRegistration, error)
	GetPendingFn                       func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error)
	ClaimForSyncFn                     func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error)
	CreateFn                           func(ctx context.Context, reg *domain.PayuniRegistration) error
	UpdateFn                           func(ctx context.Context, reg *domain.PayuniRegistration) error
	UpsertFn                           func(ctx context.Context, reg *domain.PayuniRegistration) error
	UpdateMerchantProfileFn            func(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error)
	AcquireMerchantProfileUpdateLockFn func(ctx context.Context, storeID int64) error
	InsertStatusHistoryFn              func(ctx context.Context, h *domain.PayuniStatusHistory) error
	DeleteStatusHistoryOlderThanFn     func(ctx context.Context, before time.Time) (int64, error)
}

func (m *mockPayuniRepo) GetByStoreID(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
	if m.GetByStoreIDFn != nil {
		return m.GetByStoreIDFn(ctx, storeID)
	}
	if m.GetByAccountIDFn != nil {
		return m.GetByAccountIDFn(ctx, storeID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}

func (m *mockPayuniRepo) ClaimForSync(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
	if m.ClaimForSyncFn != nil {
		return m.ClaimForSyncFn(ctx, id, claimedAt, staleBefore)
	}
	return true, nil
}

func (m *mockPayuniRepo) GetByMerID(ctx context.Context, merID string) (*domain.PayuniRegistration, error) {
	if m.GetByMerIDFn != nil {
		return m.GetByMerIDFn(ctx, merID)
	}
	if m.GetByAccountIDFn != nil {
		id, _ := strconv.ParseInt(strings.TrimPrefix(merID, "HAWK"), 10, 64)
		return m.GetByAccountIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockPayuniRepo) GetPending(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
	return m.GetPendingFn(ctx, limit, staleBefore)
}
func (m *mockPayuniRepo) Create(ctx context.Context, reg *domain.PayuniRegistration) error {
	return m.CreateFn(ctx, reg)
}
func (m *mockPayuniRepo) Update(ctx context.Context, reg *domain.PayuniRegistration) error {
	return m.UpdateFn(ctx, reg)
}
func (m *mockPayuniRepo) Upsert(ctx context.Context, reg *domain.PayuniRegistration) error {
	return m.UpsertFn(ctx, reg)
}
func (m *mockPayuniRepo) UpdateMerchantProfile(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
	return m.UpdateMerchantProfileFn(ctx, storeID, expectedVersion, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel)
}

func (m *mockPayuniRepo) AcquireMerchantProfileUpdateLock(ctx context.Context, storeID int64) error {
	if m.AcquireMerchantProfileUpdateLockFn != nil {
		return m.AcquireMerchantProfileUpdateLockFn(ctx, storeID)
	}
	return nil
}
func (m *mockPayuniRepo) InsertStatusHistory(ctx context.Context, h *domain.PayuniStatusHistory) error {
	if m.InsertStatusHistoryFn != nil {
		return m.InsertStatusHistoryFn(ctx, h)
	}
	return nil
}
func (m *mockPayuniRepo) DeleteStatusHistoryOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if m.DeleteStatusHistoryOlderThanFn != nil {
		return m.DeleteStatusHistoryOlderThanFn(ctx, before)
	}
	return 0, nil
}

type mockPayuniClient struct {
	BindSellerFn      func(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error)
	GetSellerStatusFn func(ctx context.Context, merID string) (*SellerStatusResponse, error)
	UpdateMerchantFn  func(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error)
}

func (m *mockPayuniClient) BindSeller(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error) {
	return m.BindSellerFn(ctx, req)
}
func (m *mockPayuniClient) GetSellerStatus(ctx context.Context, merID string) (*SellerStatusResponse, error) {
	return m.GetSellerStatusFn(ctx, merID)
}
func (m *mockPayuniClient) UpdateMerchant(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error) {
	return m.UpdateMerchantFn(ctx, req)
}

type mockPayuniMerchantProfileUpdateRepo struct {
	InsertFn            func(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error)
	FindByStoreAndKeyFn func(ctx context.Context, storeID int64, idempotencyKey string) (*domain.PayuniMerchantProfileUpdate, error)
	MarkSucceededFn     func(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error
	MarkFailedFn        func(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error
	DeleteFn            func(ctx context.Context, id int64) error
}

func (m *mockPayuniMerchantProfileUpdateRepo) Insert(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) {
	return m.InsertFn(ctx, u)
}
func (m *mockPayuniMerchantProfileUpdateRepo) FindByStoreAndKey(ctx context.Context, storeID int64, idempotencyKey string) (*domain.PayuniMerchantProfileUpdate, error) {
	return m.FindByStoreAndKeyFn(ctx, storeID, idempotencyKey)
}
func (m *mockPayuniMerchantProfileUpdateRepo) MarkSucceeded(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	if m.MarkSucceededFn != nil {
		return m.MarkSucceededFn(ctx, id, providerStatus, providerMessage, completedAt)
	}
	return nil
}
func (m *mockPayuniMerchantProfileUpdateRepo) MarkFailed(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	if m.MarkFailedFn != nil {
		return m.MarkFailedFn(ctx, id, providerStatus, providerMessage, completedAt)
	}
	return nil
}
func (m *mockPayuniMerchantProfileUpdateRepo) Delete(ctx context.Context, id int64) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

type mockAccountClient struct {
	GetEmailFn        func(ctx context.Context, accountID int64) (string, error)
	GetAccountInfoFn  func(ctx context.Context, accountID int64) (*AccountInfo, error)
	BindSellerRoleFn  func(ctx context.Context, accountID, storeID int64) error
	BindSellerRoleFor []int64
}

func (m *mockAccountClient) GetEmail(ctx context.Context, accountID int64) (string, error) {
	return m.GetEmailFn(ctx, accountID)
}
func (m *mockAccountClient) GetAccountInfo(ctx context.Context, accountID int64) (*AccountInfo, error) {
	return m.GetAccountInfoFn(ctx, accountID)
}
func (m *mockAccountClient) BindSellerRole(ctx context.Context, accountID, storeID int64) error {
	m.BindSellerRoleFor = append(m.BindSellerRoleFor, storeID)
	if m.BindSellerRoleFn != nil {
		return m.BindSellerRoleFn(ctx, accountID, storeID)
	}
	return nil
}

type mockMediaClient struct {
	FinalizeImagesFn    func(ctx context.Context, stagedTokens []string, refType string) ([]AssetInfo, error)
	LinkImagesFn        func(ctx context.Context, refType string, refID string, assetIDs []string) error
	ReleaseReferencesFn func(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error
}

func (m *mockMediaClient) FinalizeImages(ctx context.Context, stagedTokens []string, refType string) ([]AssetInfo, error) {
	return m.FinalizeImagesFn(ctx, stagedTokens, refType)
}
func (m *mockMediaClient) LinkImages(ctx context.Context, refType string, refID string, assetIDs []string) error {
	if m.LinkImagesFn == nil {
		return nil
	}
	return m.LinkImagesFn(ctx, refType, refID, assetIDs)
}
func (m *mockMediaClient) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	if m.ReleaseReferencesFn == nil {
		return nil
	}
	return m.ReleaseReferencesFn(ctx, refType, refIDs, keepAssetIDs)
}

type mockStoreVisitorRepo struct {
	UpsertVisitFn      func(ctx context.Context, storeID, visitorBuyerID int64) error
	ListFn             func(ctx context.Context, storeID int64, limit, offset int32) ([]*domain.StoreVisitorRow, error)
	CountFn            func(ctx context.Context, storeID int64) (int64, error)
	CountByDateRangeFn func(ctx context.Context, storeID int64, from, to time.Time) (int64, error)
}

func (m *mockStoreVisitorRepo) UpsertVisit(ctx context.Context, storeID, visitorBuyerID int64) error {
	return m.UpsertVisitFn(ctx, storeID, visitorBuyerID)
}
func (m *mockStoreVisitorRepo) List(ctx context.Context, storeID int64, limit, offset int32) ([]*domain.StoreVisitorRow, error) {
	return m.ListFn(ctx, storeID, limit, offset)
}
func (m *mockStoreVisitorRepo) Count(ctx context.Context, storeID int64) (int64, error) {
	return m.CountFn(ctx, storeID)
}
func (m *mockStoreVisitorRepo) CountByDateRange(ctx context.Context, storeID int64, from, to time.Time) (int64, error) {
	return m.CountByDateRangeFn(ctx, storeID, from, to)
}

type mockMerchantVerificationOutboxRepo struct {
	Inserted []*domain.MerchantVerificationOutbox
	InsertFn func(ctx context.Context, row *domain.MerchantVerificationOutbox) error
}

func (m *mockMerchantVerificationOutboxRepo) Insert(ctx context.Context, row *domain.MerchantVerificationOutbox) error {
	m.Inserted = append(m.Inserted, row)
	if m.InsertFn != nil {
		return m.InsertFn(ctx, row)
	}
	return nil
}
func (m *mockMerchantVerificationOutboxRepo) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*domain.MerchantVerificationOutbox, error) {
	return nil, nil
}
func (m *mockMerchantVerificationOutboxRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	return 0, nil
}
func (m *mockMerchantVerificationOutboxRepo) UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error {
	return nil
}
func (m *mockMerchantVerificationOutboxRepo) MarkDone(ctx context.Context, id int64, now time.Time) error {
	return nil
}
func (m *mockMerchantVerificationOutboxRepo) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	return nil
}
func (m *mockMerchantVerificationOutboxRepo) CountPending(ctx context.Context) (int, error) {
	return len(m.Inserted), nil
}
func (m *mockMerchantVerificationOutboxRepo) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (m *mockMerchantVerificationOutboxRepo) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return 0, nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockProductClient struct {
	storeIDs []int64
	err      error
	calls    atomic.Int32
}

func productStores(storeIDs ...int64) *mockProductClient {
	return &mockProductClient{storeIDs: storeIDs}
}

func productServiceDown() *mockProductClient {
	return &mockProductClient{err: fmt.Errorf("product-service unreachable")}
}

func (m *mockProductClient) GetProductsByIDs(_ context.Context, _ []int64) ([]*ProductInfo, error) {
	return nil, nil
}

func (m *mockProductClient) ListStoreIDsWithPublicProducts(_ context.Context) ([]int64, error) {
	m.calls.Add(1)
	if m.err != nil {
		return nil, m.err
	}
	return m.storeIDs, nil
}
