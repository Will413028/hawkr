package grpc

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testCertRepo struct {
	getByIDFn       func(ctx context.Context, id int64) (*domain.CertificationApplication, error)
	getCurrentFn    func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error)
	getDraftFn      func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error)
	certHolderFn    func(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error)
	createFn        func(ctx context.Context, app *domain.CertificationApplication) error
	updateFn        func(ctx context.Context, app *domain.CertificationApplication) error
	listForAdminFn  func(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error)
	countForAdminFn func(ctx context.Context, filter domain.CertificationListFilter) (int64, error)
	getReviewsFn    func(ctx context.Context, applicationID int64) ([]*domain.CertificationReview, error)
	createReviewFn  func(ctx context.Context, review *domain.CertificationReview) error
}

func (r *testCertRepo) GetApplicationByID(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (r *testCertRepo) GetApplicationByNo(ctx context.Context, no string) (*domain.CertificationApplication, error) {
	return nil, pkgerrors.NewNotFound("not found")
}
func (r *testCertRepo) GetApplicationsBySeller(ctx context.Context, accountID int64) ([]*domain.CertificationApplication, error) {
	return nil, nil
}
func (r *testCertRepo) GetCurrentApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	if r.getCurrentFn != nil {
		return r.getCurrentFn(ctx, accountID)
	}
	return nil, nil
}
func (r *testCertRepo) GetDraftApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	if r.getDraftFn != nil {
		return r.getDraftFn(ctx, accountID)
	}
	return nil, nil
}
func (r *testCertRepo) GetCertHolderNames(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error) {
	if r.certHolderFn != nil {
		return r.certHolderFn(ctx, storeID)
	}
	return domain.CertHolderNames{}, false, nil
}
func (r *testCertRepo) CreateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	if r.createFn != nil {
		return r.createFn(ctx, app)
	}
	return nil
}
func (r *testCertRepo) UpdateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, app)
	}
	return nil
}
func (r *testCertRepo) ListForAdmin(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error) {
	if r.listForAdminFn != nil {
		return r.listForAdminFn(ctx, filter)
	}
	return nil, nil
}
func (r *testCertRepo) CountForAdmin(ctx context.Context, filter domain.CertificationListFilter) (int64, error) {
	if r.countForAdminFn != nil {
		return r.countForAdminFn(ctx, filter)
	}
	return 0, nil
}
func (r *testCertRepo) GetStatsForAdmin(ctx context.Context) (*domain.CertificationStats, error) {
	return &domain.CertificationStats{}, nil
}
func (r *testCertRepo) GetReviewsByApplication(ctx context.Context, applicationID int64) ([]*domain.CertificationReview, error) {
	if r.getReviewsFn != nil {
		return r.getReviewsFn(ctx, applicationID)
	}
	return nil, nil
}
func (r *testCertRepo) GetLatestReview(ctx context.Context, applicationID int64) (*domain.CertificationReview, error) {
	return nil, nil
}
func (r *testCertRepo) CreateReview(ctx context.Context, review *domain.CertificationReview) error {
	if r.createReviewFn != nil {
		return r.createReviewFn(ctx, review)
	}
	return nil
}

type testPayuniClient struct {
	getSellerStatusFn func(ctx context.Context, merID string) (*usecase.SellerStatusResponse, error)
	bindSellerFn      func(ctx context.Context, req *usecase.BindSellerRequest) (*usecase.PayuniResponse, error)
}

func (c *testPayuniClient) GetSellerStatus(ctx context.Context, merID string) (*usecase.SellerStatusResponse, error) {
	if c.getSellerStatusFn != nil {
		return c.getSellerStatusFn(ctx, merID)
	}
	return &usecase.SellerStatusResponse{MerStatus: "0"}, nil
}
func (c *testPayuniClient) BindSeller(ctx context.Context, req *usecase.BindSellerRequest) (*usecase.PayuniResponse, error) {
	if c.bindSellerFn != nil {
		return c.bindSellerFn(ctx, req)
	}
	return &usecase.PayuniResponse{Success: true}, nil
}
func (c *testPayuniClient) UpdateMerchant(ctx context.Context, req *usecase.UpdateMerchantRequest) (*usecase.PayuniResponse, error) {
	return &usecase.PayuniResponse{Success: true}, nil
}

type testAccountClient struct{}

func (c *testAccountClient) GetEmail(ctx context.Context, accountID int64) (string, error) {
	return "test@example.com", nil
}
func (c *testAccountClient) GetAccountInfo(ctx context.Context, accountID int64) (*usecase.AccountInfo, error) {
	return &usecase.AccountInfo{
		Email: "test@example.com",
		Phone: "0912345678",
	}, nil
}
func (c *testAccountClient) BindSellerRole(ctx context.Context, accountID, storeID int64) error {
	return nil
}

type testMediaClient struct{}

func (c *testMediaClient) FinalizeImages(ctx context.Context, stagedTokens []string, refType string) ([]usecase.AssetInfo, error) {
	return nil, nil
}
func (c *testMediaClient) LinkImages(ctx context.Context, refType, refID string, assetIDs []string) error {
	return nil
}
func (c *testMediaClient) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	return nil
}

type testTxManager struct{}

func (m *testTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newCertServer(sellerRepo domain.SellerRepository, certRepo domain.CertificationRepository, payuniRepo domain.PayuniRepository) *SellerServer {
	sellerUC := usecase.NewSellerUseCase(sellerRepo, payuniRepo, certRepo, &testAccountClient{}, &testMediaClient{}, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 99999 }, &testTxManager{}, &testOutboxRepo{}, nil, testStoreCode, nil)
	payuniUC := usecase.NewPayuniUseCase(payuniRepo, sellerRepo, certRepo, &testAccountClient{}, &testPayuniClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, &testPayuniMerchantProfileUpdateRepo{}, func() int64 { return 99999 })
	certUC := usecase.NewCertificationUseCase(certRepo, sellerRepo, payuniRepo, &testPayuniClient{}, &testAccountClient{}, &testMediaClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, payuniUC, func() int64 { return 99999 }, nil)
	return &SellerServer{sellerUC: sellerUC, certificationUC: certUC}
}

func TestGRPC_GetSellerByMemberId_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop C",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetSellerByMemberId(context.Background(), &pb.GetSellerByMemberIdRequest{MemberId: 100})
	if err != nil {
		t.Fatalf("GetSellerByMemberId error: %v", err)
	}
}

func TestGRPC_CreateSeller_MinimalFields(t *testing.T) {
	var created *domain.Seller
	repo := &testSellerRepo{
		createFn: func(ctx context.Context, m *domain.Seller) error {
			created = m
			return nil
		},
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return nil, pkgerrors.NewNotFound("not found")
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.CreateSeller(context.Background(), &pb.CreateSellerRequest{
		UserId: 200, Name: "New Shop", ContactPhone: "0912345678",
	})
	if err != nil {
		t.Fatalf("CreateSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if created == nil {
		t.Fatal("createFn was not invoked; CreateSeller should take the create/INSERT path when no existing seller row is found")
	}
	if created.AccountID != 200 {
		t.Errorf("created.AccountID = %d, want 200", created.AccountID)
	}
}

func TestGRPC_UpdateSeller_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		updateFn: func(ctx context.Context, m *domain.Seller) error { return nil },
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Updated Shop",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		AccountId: 2600000001, Name: "Updated Shop",
	})
	if err != nil {
		t.Fatalf("UpdateSeller error: %v", err)
	}
	if resp.Data.Name != "Updated Shop" {
		t.Errorf("Name = %q, want %q", resp.Data.Name, "Updated Shop")
	}
}

func TestGRPC_GetSellerStats_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetSellerStats(context.Background(), &pb.GetSellerStatsRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetSellerStats error: %v", err)
	}
}

func TestGRPC_ListSellersForAdmin_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{}
	_ = now

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ListSellersForAdmin(context.Background(), &pb.ListSellersForAdminRequest{
		Page: 1, Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListSellersForAdmin error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil")
	}
}

func TestGRPC_ListSellersForAdmin_CarriesStoreID(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		listForAdminFn: func(ctx context.Context, f domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
			return []*domain.AdminSellerRow{{
				Seller: domain.Seller{
					AccountID: 943246063,
					StoreID:   7489247058450171597,
					StoreCode: 2685245877,
					StoreName: "Hawkr 賣家3954",
					Status:    domain.SellerStatusActive,
					CreatedAt: now, UpdatedAt: now,
				},
			}}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ListSellersForAdmin(context.Background(), &pb.ListSellersForAdminRequest{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListSellersForAdmin error: %v", err)
	}
	if len(resp.GetData().GetList()) != 1 {
		t.Fatalf("list length = %d, want 1", len(resp.GetData().GetList()))
	}
	item := resp.GetData().GetList()[0]
	if item.GetStoreId() != "7489247058450171597" {
		t.Errorf("StoreId = %q, want %q", item.GetStoreId(), "7489247058450171597")
	}
	if item.GetAccountId() != "943246063" {
		t.Errorf("AccountId = %q, want %q（登入 principal 不可被 store_id 覆寫）", item.GetAccountId(), "943246063")
	}
}

func TestGRPC_GetPayuniStatus_Registered(t *testing.T) {
	now := time.Now()
	certID := int64(5003)
	sellerRepo := &testSellerRepo{
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: id, CurrentCertificationID: &certID,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus:         domain.CertStatusForwarded.String(),
				PayuniApprovedAt:   &now,
				PayuniMerStatus:    "1",
				PayuniMemberStatus: "1",
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusActive, PaymentEnabled: true,
			}, nil
		},
	}

	srv := newCertServer(sellerRepo, &testCertRepo{}, payuniRepo)
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 100})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if resp.Data.OverallStatus != "approved" {
		t.Errorf("OverallStatus = %q, want %q", resp.Data.OverallStatus, "approved")
	}
}

func TestGRPC_GetPayuniStatus_NotRegistered(t *testing.T) {
	now := time.Now()
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return nil, pkgerrors.NewNotFound("no payuni registration")
		},
	}

	srv := newCertServer(sellerRepo, &testCertRepo{}, payuniRepo)
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 100})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if resp.Data.PayuniStatus != "unregistered" {
		t.Errorf("PayuniStatus = %q, want %q", resp.Data.PayuniStatus, "unregistered")
	}
	if resp.Data.Freshness != "not_applicable" {
		t.Errorf("Freshness = %q, want %q", resp.Data.Freshness, "not_applicable")
	}
}

func TestGRPC_RefreshPayuniStatus_Success(t *testing.T) {
	now := time.Now()
	certID := int64(500)
	const principalAccountID, storeID int64 = 100, 999

	var gotStoreIDs []int64
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreID: storeID, CurrentCertificationID: &certID,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: id, StoreID: storeID, CurrentCertificationID: &certID,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: "approved",
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByStoreIDFn: func(ctx context.Context, id int64) (*domain.PayuniRegistration, error) {
			gotStoreIDs = append(gotStoreIDs, id)
			return &domain.PayuniRegistration{
				AccountID: principalAccountID, StoreID: id, ApplicationID: certID, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusPending,
			}, nil
		},
	}
	pc := &testPayuniClient{
		getSellerStatusFn: func(ctx context.Context, merID string) (*usecase.SellerStatusResponse, error) {
			return &usecase.SellerStatusResponse{
				MerStatus: "1", MemberStatus: "1", CreditStatus: "0",
			}, nil
		},
	}

	certRepo := &testCertRepo{}
	sellerUC := usecase.NewSellerUseCase(sellerRepo, payuniRepo, certRepo, &testAccountClient{}, &testMediaClient{}, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 99999 }, &testTxManager{}, &testOutboxRepo{}, nil, nil, nil)
	payuniUC := usecase.NewPayuniUseCase(payuniRepo, sellerRepo, certRepo, &testAccountClient{}, pc, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, &testPayuniMerchantProfileUpdateRepo{}, func() int64 { return 99999 })
	srv := &SellerServer{sellerUC: sellerUC, payuniUC: payuniUC,
		certificationUC: usecase.NewCertificationUseCase(certRepo, sellerRepo, payuniRepo, pc, &testAccountClient{}, &testMediaClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, payuniUC, func() int64 { return 99999 }, nil)}
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.RefreshPayuniStatus(context.Background(), &pb.RefreshPayuniStatusRequest{UserId: principalAccountID})
	if err != nil {
		t.Fatalf("RefreshPayuniStatus error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if len(gotStoreIDs) == 0 {
		t.Fatalf("GetByStoreID was never called")
	}
	if got := gotStoreIDs[0]; got != storeID {
		t.Errorf("RefreshStatus payuni lookup key = %d, want %d (store_id); handler must forward profile.StoreID, not profile.AccountID", got, storeID)
	}
	if resp.Data.OverallStatus != "approved" {
		t.Errorf("OverallStatus = %q, want %q", resp.Data.OverallStatus, "approved")
	}
}

func TestGRPC_RefreshPayuniStatus_ReturnsFreshOverallStatus(t *testing.T) {
	now := time.Now()
	certID := int64(500)
	const principalAccountID, storeID int64 = 100, 999

	var getProfileCalls int
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreID: storeID, CurrentCertificationID: &certID,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			getProfileCalls++
			certStatus := "forwarded"
			if getProfileCalls > 1 {
				certStatus = "approved"
			}
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: id, StoreID: storeID, CurrentCertificationID: &certID,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: certStatus,
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByStoreIDFn: func(ctx context.Context, id int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: principalAccountID, StoreID: id, ApplicationID: certID, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusPending,
			}, nil
		},
	}
	pc := &testPayuniClient{
		getSellerStatusFn: func(ctx context.Context, merID string) (*usecase.SellerStatusResponse, error) {
			return &usecase.SellerStatusResponse{
				MerStatus: "1", MemberStatus: "1", CreditStatus: "0",
			}, nil
		},
	}

	certRepo := &testCertRepo{}
	sellerUC := usecase.NewSellerUseCase(sellerRepo, payuniRepo, certRepo, &testAccountClient{}, &testMediaClient{}, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 99999 }, &testTxManager{}, &testOutboxRepo{}, nil, nil, nil)
	payuniUC := usecase.NewPayuniUseCase(payuniRepo, sellerRepo, certRepo, &testAccountClient{}, pc, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, &testPayuniMerchantProfileUpdateRepo{}, func() int64 { return 99999 })
	srv := &SellerServer{sellerUC: sellerUC, payuniUC: payuniUC,
		certificationUC: usecase.NewCertificationUseCase(certRepo, sellerRepo, payuniRepo, pc, &testAccountClient{}, &testMediaClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, payuniUC, func() int64 { return 99999 }, nil)}
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.RefreshPayuniStatus(context.Background(), &pb.RefreshPayuniStatusRequest{UserId: principalAccountID})
	if err != nil {
		t.Fatalf("RefreshPayuniStatus error: %v", err)
	}
	if resp.Data.OverallStatus != "approved" {
		t.Errorf("OverallStatus = %q, want %q (must re-read profile after RefreshStatus, not the pre-refresh snapshot)", resp.Data.OverallStatus, "approved")
	}
	if getProfileCalls < 2 {
		t.Fatalf("GetProfileByAccountID called %d times, want >= 2 (once before RefreshStatus, once after)", getProfileCalls)
	}
}

func TestGRPC_CreateCertificationDraft_Success(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		createFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			app.ID = 1
			return nil
		},
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, AccountID: 200, Status: domain.CertStatusDraft,
				SellerType: "individual", TotalSteps: 8, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Description: "自我介紹", WebsiteURL: "https://my-shop.example.com"}, nil
		},
	}
	srv := newCertServer(sellerRepo, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.CreateCertificationDraft(context.Background(), &pb.CreateCertificationDraftRequest{
		AccountId: 200, SellerType: "individual",
	})
	if err != nil {
		t.Fatalf("CreateCertificationDraft error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
}

func TestGRPC_GetCertificationApplication_Success(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				CurrentStep: 5, TotalSteps: 8, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetCertificationApplication(context.Background(), &pb.GetCertificationApplicationRequest{
		ApplicationId: "42",
	})
	if err != nil {
		t.Fatalf("GetCertificationApplication error: %v", err)
	}
	if resp.Data.Id != "42" {
		t.Errorf("Id = %s, want 42", resp.Data.Id)
	}
	if resp.Data.CurrentStep != 5 {
		t.Errorf("CurrentStep = %d, want 5", resp.Data.CurrentStep)
	}
}

func TestGRPC_SaveCertificationProgress_Success(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		updateFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft,
				CurrentStep: 3, TotalSteps: 8, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.SaveCertificationProgress(context.Background(), &pb.SaveCertificationProgressRequest{
		ApplicationId: "10", CurrentStep: 3, StepData: `{"name":"test"}`,
	})
	if err != nil {
		t.Fatalf("SaveCertificationProgress error: %v", err)
	}
	if resp.Data.Id != "10" {
		t.Errorf("Id = %s, want 10", resp.Data.Id)
	}
}

func TestGRPC_CancelCertification_Success(t *testing.T) {
	var cancelled bool
	certRepo := &testCertRepo{
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			status := domain.CertStatusDraft
			if cancelled {
				status = domain.CertStatusCancelled
			}
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: status,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			app.InitFSM()
			return app, nil
		},
		updateFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			cancelled = true
			return nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.CancelCertification(context.Background(), &pb.CancelCertificationRequest{ApplicationId: "1"})
	if err != nil {
		t.Fatalf("CancelCertification error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data after cancel")
	}
	if resp.Data.Id != "1" {
		t.Errorf("Data.Id = %s, want 1", resp.Data.Id)
	}
}

func TestGRPC_ApproveCertification_Success(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				SellerType: "individual",
				CreatedAt:  now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		updateFn:       func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		createReviewFn: func(ctx context.Context, review *domain.CertificationReview) error { return nil },
	}
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, CreatedAt: now, UpdatedAt: now}, nil
		},
		updateFn: func(ctx context.Context, m *domain.Seller) error { return nil },
	}
	srv := newCertServer(sellerRepo, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.ApproveCertification(context.Background(), &pb.ApproveCertificationRequest{
		ApplicationId: "1", ReviewerId: 999, ReviewerName: "Admin", Comment: "Good",
	})
	if err != nil {
		t.Fatalf("ApproveCertification error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
}

func TestGRPC_RejectCertification_Success(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusSubmitted,
				SellerType: "individual",
				CreatedAt:  now, UpdatedAt: now,
			}
			app.InitFSM()
			return app, nil
		},
		updateFn:       func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		createReviewFn: func(ctx context.Context, review *domain.CertificationReview) error { return nil },
	}
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, CreatedAt: now, UpdatedAt: now}, nil
		},
		updateFn: func(ctx context.Context, m *domain.Seller) error { return nil },
	}
	srv := newCertServer(sellerRepo, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.RejectCertification(context.Background(), &pb.RejectCertificationRequest{
		ApplicationId: "1", ReviewerId: 999, ReviewerName: "Admin",
		Comment: "Missing docs", RejectionReason: "incomplete",
	})
	if err != nil {
		t.Fatalf("RejectCertification error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
}

func TestGRPC_GetCertificationReviews_Success(t *testing.T) {
	now := time.Now()
	reviewerID := int64(999)
	certRepo := &testCertRepo{
		getReviewsFn: func(ctx context.Context, applicationID int64) ([]*domain.CertificationReview, error) {
			return []*domain.CertificationReview{
				{
					ID: 1, ApplicationID: applicationID, ReviewerID: &reviewerID,
					ReviewerName: "Admin", Action: domain.ReviewActionSubmit, Result: domain.ReviewResultPending,
					Comment: "OK", CreatedAt: now,
				},
			}, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetCertificationReviews(context.Background(), &pb.GetCertificationReviewsRequest{
		ApplicationId: "1",
	})
	if err != nil {
		t.Fatalf("GetCertificationReviews error: %v", err)
	}
	if len(resp.Reviews) != 1 {
		t.Fatalf("len(Reviews) = %d, want 1", len(resp.Reviews))
	}
	if resp.Reviews[0].ReviewerName != "Admin" {
		t.Errorf("ReviewerName = %q, want %q", resp.Reviews[0].ReviewerName, "Admin")
	}
}

func TestGRPC_GetCertificationDraft_Found(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		getDraftFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: 5, AccountID: accountID, Status: domain.CertStatusDraft,
				CurrentStep: 2, TotalSteps: 8, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetCertificationDraft(context.Background(), &pb.GetDraftRequest{AccountId: 100})
	if err != nil {
		t.Fatalf("GetCertificationDraft error: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil, expected a draft")
	}
	if resp.Data.Id != "5" {
		t.Errorf("Id = %s, want 5", resp.Data.Id)
	}
}

func TestGRPC_GetCertificationDraft_NotFound(t *testing.T) {
	certRepo := &testCertRepo{
		getDraftFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetCertificationDraft(context.Background(), &pb.GetDraftRequest{AccountId: 100})
	if err != nil {
		t.Fatalf("GetCertificationDraft error: %v", err)
	}
	if resp.Data != nil {
		t.Error("Data should be nil when no draft exists")
	}
}

func TestGRPC_SubmitCertificationApplication_Error(t *testing.T) {
	certRepo := &testCertRepo{
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return nil, pkgerrors.NewNotFound("application not found")
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	_, err := client.SubmitCertificationApplication(context.Background(), &pb.SubmitCertificationApplicationRequest{
		ApplicationId: "999",
	})
	if err == nil {
		t.Fatal("expected gRPC error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestGRPC_SaveCertificationDraft_CreatesNew(t *testing.T) {
	now := time.Now()
	certRepo := &testCertRepo{
		getDraftFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			app.ID = 10
			return nil
		},
		updateFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
		getByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{
				ID: id, AccountID: 100, Status: domain.CertStatusDraft,
				CurrentStep: 1, TotalSteps: 8, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, Description: "自我介紹", WebsiteURL: "https://my-shop.example.com"}, nil
		},
	}
	srv := newCertServer(sellerRepo, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.SaveCertificationDraft(context.Background(), &pb.SaveDraftRequest{
		AccountId: 100, CurrentStep: 1, TotalSteps: 8, StepDataJson: `{}`,
	})
	if err != nil {
		t.Fatalf("SaveCertificationDraft error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
}

func TestGRPC_SubmitCertification_DraftNotFound(t *testing.T) {
	certRepo := &testCertRepo{
		getDraftFn: func(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
			return nil, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.SubmitCertification(context.Background(), &pb.SubmitCertificationRequest{AccountId: 100})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for missing draft, got 0")
	}
}

func TestGRPC_ListCertificationsForAdmin_Success(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 34, 56, 0, time.FixedZone("UTC+8", 8*60*60))
	certRepo := &testCertRepo{
		listForAdminFn: func(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error) {
			return []domain.AdminCertificationItem{
				{ID: 1, AccountID: 100, ApplicationNo: "APP-001", Status: "submitted",
					SellerType: "individual", RealName: "Wang", CreatedAt: now, UpdatedAt: now},
			}, nil
		},
		countForAdminFn: func(ctx context.Context, filter domain.CertificationListFilter) (int64, error) {
			return 1, nil
		},
	}
	srv := newCertServer(&testSellerRepo{}, certRepo, &testPayuniRepo{})
	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.ListCertificationsForAdmin(context.Background(), &pb.ListCertificationsForAdminRequest{
		Page: 1, Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListCertificationsForAdmin error: %v", err)
	}
	if resp.Data.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Data.Total)
	}
	if len(resp.Data.List) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(resp.Data.List))
	}
	if resp.Data.List[0].RealName != "Wang" {
		t.Errorf("RealName = %q, want %q", resp.Data.List[0].RealName, "Wang")
	}
	if got := resp.Data.List[0].CreatedAt; got != "2026-08-23T04:34:56Z" {
		t.Fatalf("CreatedAt = %q, want UTC ISO8601", got)
	}
	if got := resp.Data.List[0].UpdatedAt; got != "2026-08-23T04:34:56Z" {
		t.Fatalf("UpdatedAt = %q, want UTC ISO8601", got)
	}
}
