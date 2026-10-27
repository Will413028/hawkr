package grpc

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testSellerRepo struct {
	getByAccountIDFn                  func(ctx context.Context, accountID int64) (*domain.Seller, error)
	getCurrentByStoreIDFn             func(ctx context.Context, storeID int64) (*domain.Seller, error)
	getProfileByAccountIDFn           func(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error)
	getPublicProfileByStoreIDOrCodeFn func(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error)
	getAccountIDByStoreIDFn           func(ctx context.Context, storeID int64) (int64, error)
	getProfileByStoreIDFn             func(ctx context.Context, storeID int64) (*domain.SellerProfileRow, error)
	getSellerMetaByStoreIDsFn         func(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error)
	getPublicStoreIDsByCityFn         func(ctx context.Context, cityAliases []string) ([]int64, error)
	getPublicStoreIDsGroupedByCityFn  func(ctx context.Context) (map[string][]int64, error)
	getPrincipalsFn                   func(ctx context.Context, ids []int64) (map[int64]int64, []int64, error)
	createFn                          func(ctx context.Context, m *domain.Seller) error
	updateFn                          func(ctx context.Context, m *domain.Seller) error
	casArgs                           []*int64
	writeStoreIDs                     []int64
	updateSEOFn                       func(ctx context.Context, accountID int64, seoTitle, seoDescription string) error
	updateFoodBusinessRegFn           func(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error
	getStoreIDsByAccountIDFn          func(ctx context.Context, accountID int64) ([]int64, error)
	listForAdminFn                    func(ctx context.Context, f domain.SellerListFilter) ([]*domain.AdminSellerRow, error)
}

func (r *testSellerRepo) GetByAccountID(ctx context.Context, id int64) (*domain.Seller, error) {
	if r.getByAccountIDFn != nil {
		return r.getByAccountIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}

func (r *testSellerRepo) GetCurrentByStoreID(ctx context.Context, storeID int64) (*domain.Seller, error) {
	if r.getCurrentByStoreIDFn != nil {
		return r.getCurrentByStoreIDFn(ctx, storeID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}

func (r *testSellerRepo) GetProfileByAccountID(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
	if r.getProfileByAccountIDFn != nil {
		return r.getProfileByAccountIDFn(ctx, id)
	}
	s, err := r.GetByAccountID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &domain.SellerProfileRow{Seller: s, CertStatus: domain.OverallCertNotStarted}, nil
}
func (r *testSellerRepo) GetPublicProfileByStoreIDOrCode(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
	if r.getPublicProfileByStoreIDOrCodeFn != nil {
		return r.getPublicProfileByStoreIDOrCodeFn(ctx, storeIDOrCode)
	}
	return nil, pkgerrors.NewNotFound("seller not found")
}
func (r *testSellerRepo) GetAccountIDByStoreID(ctx context.Context, storeID int64) (int64, error) {
	if r.getAccountIDByStoreIDFn != nil {
		return r.getAccountIDByStoreIDFn(ctx, storeID)
	}
	return storeID, nil
}
func (r *testSellerRepo) GetProfileByStoreID(ctx context.Context, storeID int64) (*domain.SellerProfileRow, error) {
	if r.getProfileByStoreIDFn != nil {
		return r.getProfileByStoreIDFn(ctx, storeID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (r *testSellerRepo) GetSellerMetaByStoreIDs(ctx context.Context, ids []int64) ([]*domain.SellerMetaRow, error) {
	if r.getSellerMetaByStoreIDsFn != nil {
		return r.getSellerMetaByStoreIDsFn(ctx, ids)
	}
	return nil, nil
}
func (r *testSellerRepo) GetPublicStoreIDsByCity(ctx context.Context, aliases []string) ([]int64, error) {
	if r.getPublicStoreIDsByCityFn != nil {
		return r.getPublicStoreIDsByCityFn(ctx, aliases)
	}
	return nil, nil
}

func (r *testSellerRepo) GetPublicStoreIDsGroupedByCity(ctx context.Context) (map[string][]int64, error) {
	if r.getPublicStoreIDsGroupedByCityFn != nil {
		return r.getPublicStoreIDsGroupedByCityFn(ctx)
	}
	return map[string][]int64{}, nil
}
func (r *testSellerRepo) GetPrincipalsByStoreIDs(ctx context.Context, ids []int64) (map[int64]int64, []int64, error) {
	if r.getPrincipalsFn != nil {
		return r.getPrincipalsFn(ctx, ids)
	}
	return nil, nil, nil
}
func (r *testSellerRepo) GetStoreIDsByAccountID(ctx context.Context, accountID int64) ([]int64, error) {
	if r.getStoreIDsByAccountIDFn != nil {
		return r.getStoreIDsByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}
func (r *testSellerRepo) Create(ctx context.Context, m *domain.Seller) error {
	if r.createFn != nil {
		return r.createFn(ctx, m)
	}
	return nil
}
func (r *testSellerRepo) Update(ctx context.Context, m *domain.Seller, expectedVersion *int64) error {
	r.casArgs = append(r.casArgs, expectedVersion)
	r.writeStoreIDs = append(r.writeStoreIDs, m.StoreID)
	if r.updateFn != nil {
		return r.updateFn(ctx, m)
	}
	return nil
}
func (r *testSellerRepo) UpdateCurrentCertificationID(ctx context.Context, accountID int64, certID int64) error {
	return nil
}
func (r *testSellerRepo) CompleteOnboarding(ctx context.Context, id int64) error { return nil }
func (r *testSellerRepo) ListForAdmin(ctx context.Context, f domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
	if r.listForAdminFn != nil {
		return r.listForAdminFn(ctx, f)
	}
	return nil, nil
}
func (r *testSellerRepo) CountForAdmin(ctx context.Context, f domain.SellerListFilter) (int64, error) {
	return 0, nil
}
func (r *testSellerRepo) GetStatsForAdmin(ctx context.Context) (*domain.SellerStats, error) {
	return &domain.SellerStats{}, nil
}
func (r *testSellerRepo) GetByStoreIDs(ctx context.Context, ids []int64) ([]*domain.Seller, error) {
	return nil, nil
}
func (r *testSellerRepo) GetApprovedExcluding(_ context.Context, _ []int64, _ domain.PublicProductStores, _ int32) ([]*domain.Seller, error) {
	return nil, nil
}
func (r *testSellerRepo) UpdateLogo(ctx context.Context, storeID int64, logo string, expectedVersion *int64) error {
	r.casArgs = append(r.casArgs, expectedVersion)
	r.writeStoreIDs = append(r.writeStoreIDs, storeID)
	return nil
}
func (r *testSellerRepo) UpdateCover(ctx context.Context, storeID int64, coverURL string, expectedVersion *int64) error {
	r.casArgs = append(r.casArgs, expectedVersion)
	r.writeStoreIDs = append(r.writeStoreIDs, storeID)
	return nil
}
func (r *testSellerRepo) UpdateSEO(ctx context.Context, id int64, seoTitle, seoDescription string) error {
	if r.updateSEOFn != nil {
		return r.updateSEOFn(ctx, id, seoTitle, seoDescription)
	}
	return nil
}
func (r *testSellerRepo) UpdateRatingByStoreID(ctx context.Context, storeID int64, rating float64) error {
	return nil
}
func (r *testSellerRepo) UpdateFoodBusinessRegistration(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error {
	if r.updateFoodBusinessRegFn != nil {
		return r.updateFoodBusinessRegFn(ctx, accountID, isProcessedFood, regNo, updatedAt)
	}
	return nil
}
func (r *testSellerRepo) GetByAccountIDForUpdate(ctx context.Context, accountID int64) (*domain.Seller, error) {
	return r.GetByAccountID(ctx, accountID)
}
func (r *testSellerRepo) RetireCurrentStint(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (r *testSellerRepo) UpdateStatusByStoreID(_ context.Context, _ int64, _ domain.SellerStatus, _ *time.Time, _ *time.Time, _ time.Time) error {
	return nil
}
func (r *testSellerRepo) UpdateStatusCurrentByAccountID(_ context.Context, _ int64, _ domain.SellerStatus, _ time.Time) error {
	return nil
}
func (r *testSellerRepo) ClearPIIFields(_ context.Context, _ int64, _ time.Time) error { return nil }
func (r *testSellerRepo) ListPendingPIICleanup(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	return nil, nil
}

type testPayuniRepo struct {
	getByAccountIDFn func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error)
	getByStoreIDFn   func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error)
}

func (r *testPayuniRepo) GetByStoreID(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
	if r.getByStoreIDFn != nil {
		return r.getByStoreIDFn(ctx, storeID)
	}
	if r.getByAccountIDFn != nil {
		return r.getByAccountIDFn(ctx, storeID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}

func (r *testPayuniRepo) GetByMerID(ctx context.Context, merID string) (*domain.PayuniRegistration, error) {
	if r.getByAccountIDFn != nil {
		id, _ := strconv.ParseInt(strings.TrimPrefix(merID, "HAWK"), 10, 64)
		return r.getByAccountIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (r *testPayuniRepo) GetPending(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
	return nil, nil
}

func (r *testPayuniRepo) ClaimForSync(_ context.Context, _ int64, _, _ time.Time) (bool, error) {
	return true, nil
}

func (r *testPayuniRepo) Create(ctx context.Context, reg *domain.PayuniRegistration) error {
	return nil
}
func (r *testPayuniRepo) Update(ctx context.Context, reg *domain.PayuniRegistration) error {
	return nil
}
func (r *testPayuniRepo) Upsert(ctx context.Context, reg *domain.PayuniRegistration) error {
	return nil
}

func (r *testPayuniRepo) UpdateMerchantProfile(ctx context.Context, storeID, expectedVersion int64, contactName, contactMobile, contactTel, merEmail, merAddr, merAddrPost, merTel string) (*domain.PayuniRegistration, error) {
	return nil, nil
}

func (r *testPayuniRepo) AcquireMerchantProfileUpdateLock(_ context.Context, _ int64) error {
	return nil
}
func (r *testPayuniRepo) InsertStatusHistory(_ context.Context, _ *domain.PayuniStatusHistory) error {
	return nil
}
func (r *testPayuniRepo) DeleteStatusHistoryOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

type testPayuniMerchantProfileUpdateRepo struct{}

func (r *testPayuniMerchantProfileUpdateRepo) Insert(ctx context.Context, u *domain.PayuniMerchantProfileUpdate) (bool, error) {
	return true, nil
}
func (r *testPayuniMerchantProfileUpdateRepo) FindByStoreAndKey(ctx context.Context, storeID int64, idempotencyKey string) (*domain.PayuniMerchantProfileUpdate, error) {
	return nil, pkgerrors.NewNotFound("not found")
}
func (r *testPayuniMerchantProfileUpdateRepo) MarkSucceeded(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	return nil
}
func (r *testPayuniMerchantProfileUpdateRepo) MarkFailed(ctx context.Context, id int64, providerStatus, providerMessage string, completedAt time.Time) error {
	return nil
}
func (r *testPayuniMerchantProfileUpdateRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

type testOutboxRepo struct{}

func (testOutboxRepo) Insert(context.Context, *domain.SellerDeletedOutbox) error { return nil }
func (testOutboxRepo) FetchPendingDue(context.Context, time.Time, int) ([]*domain.SellerDeletedOutbox, error) {
	return nil, nil
}
func (testOutboxRepo) UpdateAttempt(context.Context, int64, int, time.Time, string) error {
	return nil
}
func (testOutboxRepo) MarkDone(context.Context, int64, time.Time) error { return nil }
func (testOutboxRepo) Claim(context.Context, int64, time.Time, time.Time) (int64, error) {
	return 1, nil
}
func (testOutboxRepo) MarkFailed(context.Context, int64, string, time.Time) error { return nil }
func (testOutboxRepo) MarkEventPublished(context.Context, int64, time.Time) error { return nil }
func (testOutboxRepo) MarkRoleRemoved(context.Context, int64, time.Time) error    { return nil }
func (testOutboxRepo) CountPending(context.Context) (int, error)                  { return 0, nil }
func (testOutboxRepo) OldestPendingAge(context.Context, time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (testOutboxRepo) DeleteDoneBefore(context.Context, time.Time, int) (int, error) { return 0, nil }

type testMerchantVerificationOutboxRepo struct{}

func (testMerchantVerificationOutboxRepo) Insert(context.Context, *domain.MerchantVerificationOutbox) error {
	return nil
}
func (testMerchantVerificationOutboxRepo) FetchPendingDue(context.Context, time.Time, int) ([]*domain.MerchantVerificationOutbox, error) {
	return nil, nil
}
func (testMerchantVerificationOutboxRepo) Claim(context.Context, int64, time.Time, time.Time) (int64, error) {
	return 1, nil
}
func (testMerchantVerificationOutboxRepo) UpdateAttempt(context.Context, int64, int, time.Time, string) error {
	return nil
}
func (testMerchantVerificationOutboxRepo) MarkDone(context.Context, int64, time.Time) error {
	return nil
}
func (testMerchantVerificationOutboxRepo) MarkFailed(context.Context, int64, string, time.Time) error {
	return nil
}
func (testMerchantVerificationOutboxRepo) CountPending(context.Context) (int, error) { return 0, nil }
func (testMerchantVerificationOutboxRepo) OldestPendingAge(context.Context, time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (testMerchantVerificationOutboxRepo) DeleteDoneBefore(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func newTestGRPCClient(t *testing.T, srv *SellerServer) (pb.SellerServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer(
		gogrpc.UnaryInterceptor(interceptor.ErrorInterceptor()),
	)
	pb.RegisterSellerServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	return pb.NewSellerServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
	}
}

type testBuyerClient struct{}

func (c *testBuyerClient) GetSubscriberCount(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (c *testBuyerClient) BatchGetSubscriberCounts(_ context.Context, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func (c *testBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

type testReviewClient struct{}

func (c *testReviewClient) GetSellerReviewStats(_ context.Context, _ int64) (*usecase.ReviewStats, error) {
	return &usecase.ReviewStats{}, nil
}

func testStoreCode() (int64, error) { return 2_600_000_000, nil }

func newSellerServer(sellerRepo domain.SellerRepository, payuniRepo domain.PayuniRepository) *SellerServer {
	return newSellerServerWithCertRepo(sellerRepo, payuniRepo, &testCertRepo{})
}

func newSellerServerWithCertRepo(sellerRepo domain.SellerRepository, payuniRepo domain.PayuniRepository, certRepo domain.CertificationRepository) *SellerServer {
	if payuniRepo == nil {
		payuniRepo = &testPayuniRepo{}
	}
	sellerUC := usecase.NewSellerUseCase(sellerRepo, payuniRepo, certRepo, &testAccountClient{}, &testMediaClient{}, &config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil, func() int64 { return 99999 }, &testTxManager{}, &testOutboxRepo{}, nil, testStoreCode, nil)
	payuniUC := usecase.NewPayuniUseCase(payuniRepo, sellerRepo, certRepo, &testAccountClient{}, &testPayuniClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, &testPayuniMerchantProfileUpdateRepo{}, func() int64 { return 99999 })
	certUC := usecase.NewCertificationUseCase(certRepo, sellerRepo, payuniRepo, &testPayuniClient{}, &testAccountClient{}, &testMediaClient{}, &testTxManager{}, &testMerchantVerificationOutboxRepo{}, payuniUC, func() int64 { return 99999 }, nil)
	return &SellerServer{sellerUC: sellerUC, certificationUC: certUC, payuniUC: payuniUC, buyerClient: &testBuyerClient{}, reviewClient: &testReviewClient{}}
}

func TestGRPC_GetSeller_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop A",
				CreatedAt: now, UpdatedAt: now,
				IsProcessedFood: true, FoodBusinessRegNo: "北市衛食字第12345號",
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data.Name != "Shop A" {
		t.Errorf("Name = %q, want %q", resp.Data.Name, "Shop A")
	}
	if !resp.Data.IsProcessedFood {
		t.Errorf("IsProcessedFood = %v, want true", resp.Data.IsProcessedFood)
	}
	if resp.Data.FoodBusinessRegNo != "北市衛食字第12345號" {
		t.Errorf("FoodBusinessRegNo = %q, want %q", resp.Data.FoodBusinessRegNo, "北市衛食字第12345號")
	}
}

func TestGRPC_GetSeller_RepoError(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return nil, pkgerrors.NewNotFound("seller not found")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2699999999})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for missing seller, got 0")
	}
	if resp.Code != bizcode.SellerNotFound {
		t.Errorf("Code = %d, want %d (bizcode.SellerNotFound)", resp.Code, bizcode.SellerNotFound)
	}
	if resp.Message != "找不到您的商家資料" {
		t.Errorf("Message = %q, want 找不到您的商家資料 (must not leak raw English repo error)", resp.Message)
	}
}

func TestGRPC_GetSeller_ByStoreId_NotFound_StaysCodeNotFound(t *testing.T) {
	repo := &testSellerRepo{
		getPublicProfileByStoreIDOrCodeFn: func(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
			return nil, pkgerrors.NewNotFound("seller not found")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{StoreId: 999999999})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code != int32(pkgerrors.CodeNotFound) {
		t.Errorf("Code = %d, want %d (pkgerrors.CodeNotFound) — must NOT collapse to bizcode.SellerNotFound(%d)", resp.Code, pkgerrors.CodeNotFound, bizcode.SellerNotFound)
	}
}

func TestGRPC_GetSeller_DeletedSeller_Returns4040(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Deleted Shop",
				Status:    domain.SellerStatusClosed,
				DeletedAt: &now,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2600000099})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code != 4040 {
		t.Errorf("Code = %d, want 4040", resp.Code)
	}
	if resp.Message != "商家不存在或已關閉" {
		t.Errorf("Message = %q, want %q", resp.Message, "商家不存在或已關閉")
	}
}

func TestGRPC_GetSeller_ByStoreID_UsesPublicPath(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getPublicProfileByStoreIDOrCodeFn: func(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: 42,
					StoreID:   storeIDOrCode,
					StoreName: "老王鮮魚",
					Status:    domain.SellerStatusActive,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: domain.OverallCertNotStarted,
			}, nil
		},
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			t.Errorf("account current-stint path called (GetByAccountID id=%d); expected PUBLIC PK path", id)
			return nil, pkgerrors.NewNotFound("sentinel: account path must not be hit")
		},
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			t.Errorf("account current-stint path called (GetProfileByAccountID id=%d); expected PUBLIC PK path", id)
			return nil, pkgerrors.NewNotFound("sentinel: account path must not be hit")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{StoreId: 2600000099, AccountId: 0})
	if err != nil {
		t.Fatalf("GetSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil")
	}
	if resp.Data.StoreId != "2600000099" {
		t.Errorf("StoreId = %q, want %q (surrogate should echo through the public path)", resp.Data.StoreId, "2600000099")
	}
	if resp.Data.Name != "老王鮮魚" {
		t.Errorf("Name = %q, want %q", resp.Data.Name, "老王鮮魚")
	}
}

func TestGRPC_GetSellerStats_ByStoreID_UsesPublicPath(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getPublicProfileByStoreIDOrCodeFn: func(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: 42,
					StoreID:   storeIDOrCode,
					StoreName: "老王鮮魚",
					Status:    domain.SellerStatusActive,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: domain.OverallCertNotStarted,
			}, nil
		},
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			t.Errorf("account current-stint path called (GetByAccountID id=%d); expected PUBLIC PK path", id)
			return nil, pkgerrors.NewNotFound("sentinel: account path must not be hit")
		},
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			t.Errorf("account current-stint path called (GetProfileByAccountID id=%d); expected PUBLIC PK path", id)
			return nil, pkgerrors.NewNotFound("sentinel: account path must not be hit")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSellerStats(context.Background(), &pb.GetSellerStatsRequest{StoreId: 2600000099, AccountId: 0})
	if err != nil {
		t.Fatalf("GetSellerStats error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil")
	}
	if resp.Data.StoreId != "2600000099" {
		t.Errorf("stats StoreId = %q, want %q (surrogate should echo through the public path)", resp.Data.StoreId, "2600000099")
	}
}

func TestGRPC_GetSellerByUserId_Success(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop B",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetSellerByUserId(context.Background(), &pb.GetSellerByUserIdRequest{UserId: 100})
	if err != nil {
		t.Fatalf("GetSellerByUserId error: %v", err)
	}
}

func TestGRPC_ValidateSeller_Approved(t *testing.T) {
	now := time.Now()
	certID := int64(5001)
	repo := &testSellerRepo{
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: id, CurrentCertificationID: &certID,
					Status:    domain.SellerStatusActive,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus:               domain.CertStatusForwarded.String(),
				PayuniApprovedAt:         &now,
				PayuniRegistrationStatus: domain.PayuniStatusActive.String(),
				PayuniMerStatus:          "1",
				PayuniMemberStatus:       "1",
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ValidateSeller(context.Background(), &pb.ValidateSellerRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("ValidateSeller error: %v", err)
	}
	if !resp.Valid {
		t.Error("Valid = false, want true for approved seller")
	}
}

func TestGRPC_ValidateSeller_Pending(t *testing.T) {
	now := time.Now()
	certID := int64(5002)
	repo := &testSellerRepo{
		getProfileByAccountIDFn: func(ctx context.Context, id int64) (*domain.SellerProfileRow, error) {
			return &domain.SellerProfileRow{
				Seller: &domain.Seller{
					AccountID: id, CurrentCertificationID: &certID,
					Status:    domain.SellerStatusActive,
					CreatedAt: now, UpdatedAt: now,
				},
				CertStatus: domain.CertStatusSubmitted.String(),
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ValidateSeller(context.Background(), &pb.ValidateSellerRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("ValidateSeller error: %v", err)
	}
	if resp.Valid {
		t.Error("Valid = true, want false for pending seller")
	}
}

func TestGRPC_ValidateSeller_NotFound(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return nil, pkgerrors.NewNotFound("seller not found")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ValidateSeller(context.Background(), &pb.ValidateSellerRequest{AccountId: 2699999999})
	if err != nil {
		t.Fatalf("ValidateSeller error: %v", err)
	}
	if resp.Valid {
		t.Error("Valid = true, want false for not-found seller")
	}
}

func TestGRPC_GetSellerPayuniConfig_Enabled(t *testing.T) {
	now := time.Now()
	certID := int64(500)
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop A", CurrentCertificationID: &certID,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, ApplicationID: certID, PayuniMerID: "HAWK2612345678",
				Status: domain.PayuniStatusActive, PaymentEnabled: true, CreditStatus: "1",
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetSellerPayuniConfig(context.Background(), &pb.GetSellerPayuniConfigRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetSellerPayuniConfig error: %v", err)
	}
	if !resp.Data.IsEnabled {
		t.Error("IsEnabled = false, want true for active PAYUNi")
	}
	if !resp.Data.CreditEnabled {
		t.Error("CreditEnabled = false, want true for CreditStatus==\"1\"")
	}
}

func TestGRPC_GetSellerPayuniConfig_NotRegistered(t *testing.T) {
	now := time.Now()
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Shop A",
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return nil, pkgerrors.NewNotFound("no payuni registration")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetSellerPayuniConfig(context.Background(), &pb.GetSellerPayuniConfigRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetSellerPayuniConfig error: %v", err)
	}
	if resp.Data.IsEnabled {
		t.Error("IsEnabled = true, want false when PAYUNi not registered")
	}
}

func TestGRPC_RegisterPayuni_Retired_FailedPrecondition(t *testing.T) {
	client, cleanup := newTestGRPCClient(t, newSellerServer(&testSellerRepo{}, nil))
	defer cleanup()

	_, err := client.RegisterPayuni(context.Background(), &pb.RegisterPayuniRequest{UserId: 100})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("RegisterPayuni code = %v, want FailedPrecondition", status.Code(err))
	}

	_, err = client.RetryPayuniRegistration(context.Background(), &pb.RetryPayuniRequest{UserId: 100})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("RetryPayuniRegistration code = %v, want FailedPrecondition", status.Code(err))
	}
}

func newPayuniStatusFixture(reg *domain.PayuniRegistration) (*testSellerRepo, *testPayuniRepo) {
	now := time.Now()
	certID := int64(500)
	reg.ApplicationID = certID
	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreID: id, StoreName: "Shop A", CurrentCertificationID: &certID,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			reg.AccountID = accountID
			return reg, nil
		},
	}
	return sellerRepo, payuniRepo
}

func TestGetPayuniStatus_PopulatesSellerFacingError(t *testing.T) {
	sellerRepo, payuniRepo := newPayuniStatusFixture(&domain.PayuniRegistration{
		Status: domain.PayuniStatusFailed, MerStatus: "92", MemberStatus: "1", Reason: "", ErrorCode: "MER_92",
	})
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 2600000001})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if got := resp.Data.GetErrorCode(); got != "MER_92" {
		t.Errorf("ErrorCode = %q, want %q", got, "MER_92")
	}
	if got := resp.Data.GetErrorDisplayMessage(); got != "金流申請審核未通過，請確認資料後重新提交" {
		t.Errorf("ErrorDisplayMessage = %q, want %q", got, "金流申請審核未通過，請確認資料後重新提交")
	}
	if got := resp.Data.GetErrorRemedy(); got != "SELLER_FIX" {
		t.Errorf("ErrorRemedy = %q, want %q", got, "SELLER_FIX")
	}
	if got := resp.Data.GetRetryStep(); got != "start" {
		t.Errorf("RetryStep = %q, want %q", got, "start")
	}
}

func TestGetPayuniStatus_ReasonOverridesCatalogText(t *testing.T) {
	sellerRepo, payuniRepo := newPayuniStatusFixture(&domain.PayuniRegistration{
		Status: domain.PayuniStatusFailed, MerStatus: "91", MemberStatus: "1", Reason: "請補上營業登記證影本",
	})
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 2600000001})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if got := resp.Data.GetErrorDisplayMessage(); got != "請補上營業登記證影本" {
		t.Errorf("ErrorDisplayMessage = %q, want %q", got, "請補上營業登記證影本")
	}
	if got := resp.Data.GetErrorRemedy(); got != "SELLER_FIX" {
		t.Errorf("ErrorRemedy = %q, want %q", got, "SELLER_FIX")
	}
}

func TestGetPayuniStatus_ActiveMerchantHasNoError(t *testing.T) {
	sellerRepo, payuniRepo := newPayuniStatusFixture(&domain.PayuniRegistration{
		Status: domain.PayuniStatusActive, MerStatus: "1", MemberStatus: "1", ErrorCode: "",
	})
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 2600000001})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if got := resp.Data.GetErrorCode(); got != "" {
		t.Errorf("ErrorCode = %q, want empty", got)
	}
	if got := resp.Data.GetErrorDisplayMessage(); got != "" {
		t.Errorf("ErrorDisplayMessage = %q, want empty", got)
	}
	if got := resp.Data.GetErrorRemedy(); got != "" {
		t.Errorf("ErrorRemedy = %q, want empty", got)
	}
	if got := resp.Data.GetRetryStep(); got != "" {
		t.Errorf("RetryStep = %q, want empty", got)
	}
}

func TestGetPayuniStatus_StaleErrorCodeIsNotShown(t *testing.T) {
	sellerRepo, payuniRepo := newPayuniStatusFixture(&domain.PayuniRegistration{
		Status: domain.PayuniStatusFailed, MerStatus: "91", MemberStatus: "1", Reason: "", ErrorCode: "MER_92",
	})
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 2600000001})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if got := resp.Data.GetErrorCode(); got != "MER_91" {
		t.Errorf("ErrorCode = %q, want %q (not the stale MER_92)", got, "MER_91")
	}
	if got := resp.Data.GetErrorDisplayMessage(); got != "金流審核需要補件，請依指示補齊資料後重新提交" {
		t.Errorf("ErrorDisplayMessage = %q, want %q", got, "金流審核需要補件，請依指示補齊資料後重新提交")
	}
}

func TestGetPayuniStatus_ApprovedAfterFailureShowsNoError(t *testing.T) {
	sellerRepo, payuniRepo := newPayuniStatusFixture(&domain.PayuniRegistration{
		Status: domain.PayuniStatusActive, MerStatus: "1", MemberStatus: "1", ErrorCode: "MER_92",
	})
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.GetPayuniStatus(context.Background(), &pb.GetPayuniStatusRequest{UserId: 2600000001})
	if err != nil {
		t.Fatalf("GetPayuniStatus error: %v", err)
	}
	if got := resp.Data.GetErrorCode(); got != "" {
		t.Errorf("ErrorCode = %q, want empty", got)
	}
	if got := resp.Data.GetErrorDisplayMessage(); got != "" {
		t.Errorf("ErrorDisplayMessage = %q, want empty", got)
	}
	if got := resp.Data.GetErrorRemedy(); got != "" {
		t.Errorf("ErrorRemedy = %q, want empty", got)
	}
	if got := resp.Data.GetRetryStep(); got != "" {
		t.Errorf("RetryStep = %q, want empty", got)
	}
}

func TestGRPC_BatchGetSellerMeta_Success(t *testing.T) {
	repo := &testSellerRepo{
		getSellerMetaByStoreIDsFn: func(ctx context.Context, ids []int64) ([]*domain.SellerMetaRow, error) {
			return []*domain.SellerMetaRow{
				{AccountID: 100, StoreID: 100, StoreName: "Shop A", Logo: "a.png", CertStatus: domain.CertStatusForwarded.String(), PayuniStatus: domain.PayuniStatusActive.String(), Status: string(domain.SellerStatusActive)},
				{AccountID: 200, StoreID: 200, StoreName: "Shop B", Logo: "b.png", CertStatus: domain.CertStatusSubmitted.String(), PayuniStatus: domain.PayuniStatusPending.String(), Status: string(domain.SellerStatusActive)},
				{AccountID: 300, StoreID: 300, StoreName: "Shop C", Logo: "c.png", PayuniStatus: domain.PayuniStatusActive.String(), Status: string(domain.SellerStatusSuspended)},
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.BatchGetSellerMeta(context.Background(), &pb.BatchGetSellerMetaRequest{
		StoreIds: []int64{100, 200, 300},
	})
	if err != nil {
		t.Fatalf("BatchGetSellerMeta error: %v", err)
	}
	if len(resp.Sellers) != 3 {
		t.Fatalf("len(Sellers) = %d, want 3", len(resp.Sellers))
	}
	// can_manage_products 只看店家營運狀態：PayUni 未核准的店可以上架，停權的店不行。
	for id, want := range map[int64]bool{100: true, 200: true, 300: false} {
		if got := resp.Sellers[id].CanManageProducts; got != want {
			t.Errorf("Sellers[%d].CanManageProducts = %v, want %v", id, got, want)
		}
	}
	if resp.Sellers[200].SellerName != "Shop B" {
		t.Errorf("Sellers[200].SellerName = %q, want %q", resp.Sellers[200].SellerName, "Shop B")
	}
	if !resp.Sellers[100].CanAcceptOrders {
		t.Error("Sellers[100].CanAcceptOrders = false, want true for approved cert")
	}
	if resp.Sellers[200].CanAcceptOrders {
		t.Error("Sellers[200].CanAcceptOrders = true, want false for submitted (non-approved) cert")
	}
}

func TestGRPC_BatchGetSellerMeta_Empty(t *testing.T) {
	client, cleanup := newTestGRPCClient(t, newSellerServer(&testSellerRepo{}, nil))
	defer cleanup()

	resp, err := client.BatchGetSellerMeta(context.Background(), &pb.BatchGetSellerMetaRequest{
		StoreIds: []int64{},
	})
	if err != nil {
		t.Fatalf("BatchGetSellerMeta error: %v", err)
	}
	if len(resp.Sellers) != 0 {
		t.Errorf("len(Sellers) = %d, want 0", len(resp.Sellers))
	}
}

func TestGRPC_ListPublicStoreIDsByCity_NormalizesAlias(t *testing.T) {
	repo := &testSellerRepo{getPublicStoreIDsByCityFn: func(_ context.Context, aliases []string) ([]int64, error) {
		if len(aliases) != 2 || aliases[0] != "台北市" || aliases[1] != "臺北市" {
			t.Fatalf("aliases = %v", aliases)
		}
		return []int64{101, 102}, nil
	}}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ListPublicStoreIDsByCity(context.Background(), &pb.ListPublicStoreIDsByCityRequest{City: "臺北市"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.City != "台北市" || strings.Join(resp.StoreIds, ",") != "101,102" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestGRPC_ListPublicStoresByCity_ReturnsAll22CitiesAndBoundsPreview(t *testing.T) {
	repo := &testSellerRepo{getPublicStoreIDsGroupedByCityFn: func(_ context.Context) (map[string][]int64, error) {
		return map[string][]int64{
			"台北市": {101, 102, 103, 104, 105, 106, 107},
		}, nil
	}}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ListPublicStoresByCity(context.Background(), &pb.ListPublicStoresByCityRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Cities) != 22 {
		t.Fatalf("cities = %d, want 22", len(resp.Cities))
	}
	var taipei *pb.PublicCityStores
	for _, city := range resp.Cities {
		if city.CityCode == "TW-TPE" {
			taipei = city
			break
		}
	}
	if taipei == nil {
		t.Fatal("TW-TPE missing")
	}
	if taipei.CityName != "台北市" || len(taipei.StoreIds) != 7 || len(taipei.PreviewStoreIds) != 6 {
		t.Fatalf("taipei = %+v", taipei)
	}
}

func TestGRPC_GetSellerBatch_PKLookupKeyedByRequestedSurrogate(t *testing.T) {
	now := time.Now()
	deletedAt := now.Add(-time.Hour)
	repo := &testSellerRepo{
		getProfileByStoreIDFn: func(_ context.Context, storeID int64) (*domain.SellerProfileRow, error) {
			switch storeID {
			case 7300000000000000001:
				return &domain.SellerProfileRow{Seller: &domain.Seller{
					AccountID: 2611111111, StoreID: storeID,
					StoreName: "已歇業商店", DeletedAt: &deletedAt,
					CreatedAt: now, UpdatedAt: now,
				}, CertStatus: domain.OverallCertNotStarted}, nil
			case 7300000000000000002:
				return &domain.SellerProfileRow{Seller: &domain.Seller{
					AccountID: 2622222222, StoreID: storeID,
					StoreName: "Shop B", CreatedAt: now, UpdatedAt: now,
				}, CertStatus: domain.OverallCertNotStarted}, nil
			default:
				return nil, pkgerrors.NewNotFound("not found")
			}
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSellerBatch(context.Background(), &pb.GetSellerBatchRequest{
		StoreIds: []int64{7300000000000000001, 7300000000000000002, 7399999999999999999},
	})
	if err != nil {
		t.Fatalf("GetSellerBatch error: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].AccountId != 2611111111 {
		t.Errorf("Data[0].AccountId = %d, want principal account_id 2611111111", resp.Data[0].AccountId)
	}
	if resp.Data[0].StoreId != "7300000000000000001" {
		t.Errorf("Data[0].StoreId = %q, want requested store_id 7300000000000000001", resp.Data[0].StoreId)
	}
	if resp.Data[0].Name != "已歇業商店" {
		t.Errorf("Data[0].Name = %q, want 已歇業商店（closed stint profile）", resp.Data[0].Name)
	}
	if resp.Data[1].AccountId != 2622222222 {
		t.Errorf("Data[1].AccountId = %d, want principal account_id 2622222222", resp.Data[1].AccountId)
	}
	if resp.Data[1].StoreId != "7300000000000000002" {
		t.Errorf("Data[1].StoreId = %q, want requested store_id 7300000000000000002", resp.Data[1].StoreId)
	}
}

func TestGRPC_UpdateSellerSEO_Success(t *testing.T) {
	var gotID int64
	var gotTitle, gotDesc string
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: id, StoreName: "Shop A"}, nil
		},
		updateSEOFn: func(ctx context.Context, id int64, title, desc string) error {
			gotID, gotTitle, gotDesc = id, title, desc
			return nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.UpdateSellerSEO(context.Background(), &pb.UpdateSellerSEORequest{
		AccountId: 100, SeoTitle: "t", SeoDescription: "d",
	})
	if err != nil {
		t.Fatalf("UpdateSellerSEO error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if gotID != 100 {
		t.Errorf("accountID = %d, want 100", gotID)
	}
	if gotTitle != "t" || gotDesc != "d" {
		t.Errorf("seo = (%q, %q), want (%q, %q)", gotTitle, gotDesc, "t", "d")
	}
}

func TestGRPC_ResolveSellerAccountID_Success(t *testing.T) {
	var gotStoreID int64
	repo := &testSellerRepo{
		getAccountIDByStoreIDFn: func(ctx context.Context, storeID int64) (int64, error) {
			gotStoreID = storeID
			return 2670447993, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.ResolveSellerAccountID(context.Background(), &pb.ResolveSellerAccountIDRequest{
		StoreId: 7487768729113814280,
	})
	if err != nil {
		t.Fatalf("ResolveSellerAccountID error: %v", err)
	}
	if gotStoreID != 7487768729113814280 {
		t.Errorf("repo called with store_id = %d, want 7487768729113814280", gotStoreID)
	}
	if resp.AccountId != 2670447993 {
		t.Errorf("AccountId = %d, want 2670447993 (diverged from store_id)", resp.AccountId)
	}
}

func TestGRPC_ResolveSellerAccountID_NotFound(t *testing.T) {
	repo := &testSellerRepo{
		getAccountIDByStoreIDFn: func(ctx context.Context, storeID int64) (int64, error) {
			return 0, pkgerrors.NewNotFound("seller not found")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.ResolveSellerAccountID(context.Background(), &pb.ResolveSellerAccountIDRequest{StoreId: 99999})
	if err == nil {
		t.Fatal("expected gRPC error for missing store_id, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status code = %v, want codes.NotFound (this RPC has no Code/Message envelope, error must be a real gRPC status)", status.Code(err))
	}
}

func TestGRPC_GetCurrentStoreID_Success(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreID: 2600000123, StoreName: "Shop A",
				Status: domain.SellerStatusActive,
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetCurrentStoreID(context.Background(), &pb.GetCurrentStoreIDRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetCurrentStoreID error: %v", err)
	}
	if resp.StoreId != 2600000123 {
		t.Errorf("StoreId = %d, want 2600000123", resp.StoreId)
	}
}

func TestGRPC_GetCurrentStoreID_NoRow_NotFound(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return nil, pkgerrors.NewNotFound("seller not found")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetCurrentStoreID(context.Background(), &pb.GetCurrentStoreIDRequest{AccountId: 2699999999})
	if err == nil {
		t.Fatal("expected gRPC error for account with no sellers row, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status code = %v, want codes.NotFound (this RPC has no Code/Message envelope, error must be a real gRPC status)", status.Code(err))
	}
}

func TestGRPC_GetCurrentStoreID_Closed_NotFound(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreID: id, StoreName: "Closed Shop",
				Status:    domain.SellerStatusClosed,
				DeletedAt: &now,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetCurrentStoreID(context.Background(), &pb.GetCurrentStoreIDRequest{AccountId: 2600000099})
	if err == nil {
		t.Fatal("expected gRPC error for closed store, got nil")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status code = %v, want codes.NotFound (this RPC has no Code/Message envelope, error must be a real gRPC status)", status.Code(err))
	}
}

func TestGRPC_GetCurrentStoreID_InternalError_PropagatesAsInternal(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return nil, errors.New("db unavailable")
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.GetCurrentStoreID(context.Background(), &pb.GetCurrentStoreIDRequest{AccountId: 2600000001})
	if err == nil {
		t.Fatal("expected gRPC error for repo failure, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("status code = %v, want codes.Internal (a real infra failure must not be misread as \"no current store\")", status.Code(err))
	}
}
