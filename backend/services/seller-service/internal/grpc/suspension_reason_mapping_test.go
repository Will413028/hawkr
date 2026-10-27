package grpc

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"

	"testing"
)

// 46 §7.2：GetSeller(/app/sellers/me 底層 RPC) 附上 active suspension 的原因碼，
// 供賣家自身撈原因碼顯示停權公告頁。本檔測 getSellerResponseByProfile 新增的映射邏輯，
// 不重測 SellerSuspensionUseCase.GetActive 本身（usecase 套件已覆蓋）。

// ---- minimal fakes：只需滿足介面，非 GetActive 路徑不會被呼叫 ----

type stubSellerSuspensionRepo struct {
	activeByAccountID map[int64]*domain.SellerSuspension
}

func (s *stubSellerSuspensionRepo) Insert(context.Context, *domain.SellerSuspension) error {
	return nil
}
func (s *stubSellerSuspensionRepo) GetActiveByAccountID(_ context.Context, accountID int64) (*domain.SellerSuspension, error) {
	return s.activeByAccountID[accountID], nil
}
func (s *stubSellerSuspensionRepo) HasActiveByAccountID(_ context.Context, accountID int64) (bool, error) {
	return s.activeByAccountID[accountID] != nil, nil
}
func (s *stubSellerSuspensionRepo) GetByID(context.Context, int64) (*domain.SellerSuspension, error) {
	return nil, nil
}
func (s *stubSellerSuspensionRepo) Lift(context.Context, int64, int64, time.Time) (*domain.SellerSuspension, error) {
	return nil, nil
}
func (s *stubSellerSuspensionRepo) ListByAccountID(context.Context, int64, int32, int32) ([]*domain.SellerSuspension, error) {
	return nil, nil
}

// stubSellerViolationRepo is a no-op domain.SellerViolationRepository — only needed to
// satisfy NewSellerSuspensionUseCase's constructor; GetActive never touches it.
type stubSellerViolationRepo struct{}

func (stubSellerViolationRepo) AcquireCreateLock(context.Context, int64) error { return nil }
func (stubSellerViolationRepo) CountMediumLast90d(context.Context, int64) (int64, error) {
	return 0, nil
}
func (stubSellerViolationRepo) CreateViolation(context.Context, *domain.SellerViolation) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) GetByID(context.Context, int64) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) AdminUpdateMutable(context.Context, int64, string, *string, *time.Time) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) AdminRevoke(context.Context, int64, domain.RevokedReason) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) ListBySellerSelf(context.Context, int64, int32, int32) ([]*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) ListPublicLast90dHigh(context.Context, int64) ([]*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) AdminList(context.Context, domain.SellerViolationListFilter) ([]*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) ListExpiredFullSuspensions(context.Context) ([]*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) HasActiveFullSuspensionBySeller(context.Context, int64) (bool, error) {
	return false, nil
}
func (stubSellerViolationRepo) SubmitAppeal(context.Context, int64, string, []string) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) DecideAppealAccept(context.Context, int64, int64, string) (*domain.SellerViolation, error) {
	return nil, nil
}
func (stubSellerViolationRepo) DecideAppealReject(context.Context, int64, int64, string) (*domain.SellerViolation, error) {
	return nil, nil
}

// stubSellerSuspensionRedis is a no-op — GetActive never touches Redis.
type stubSellerSuspensionRedis struct{}

func (stubSellerSuspensionRedis) SetSuspended(context.Context, int64, string) {}
func (stubSellerSuspensionRedis) ClearSuspended(context.Context, int64)       {}

func newSuspensionUC(active map[int64]*domain.SellerSuspension, sellerRepo domain.SellerRepository) *usecase.SellerSuspensionUseCase {
	return usecase.NewSellerSuspensionUseCase(
		&stubSellerSuspensionRepo{activeByAccountID: active},
		sellerRepo,
		stubSellerViolationRepo{},
		&testOutboxRepo{},
		stubSellerSuspensionRedis{},
		&testTxManager{},
		func() int64 { return 99999 }, nil)
}

func TestGRPC_GetSeller_SuspendedSeller_SurfacesReasonCode(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Suspended Shop",
				Status:    domain.SellerStatusSuspended,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	active := map[int64]*domain.SellerSuspension{
		// issue #1534：domain/DB 直接持有 CodeMeta Value 數字碼。
		2600000001: {ID: 1, AccountID: 2600000001, ReasonCode: domain.SellerSuspensionReasonPolicyViolation, Status: domain.SellerSuspensionActive},
	}

	srv := newSellerServer(repo, nil)
	srv.suspensionUC = newSuspensionUC(active, repo)

	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2600000001})
	if err != nil {
		t.Fatalf("GetSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("Code = %d, want 0", resp.Code)
	}
	if resp.Data.LifecycleStatus != "suspended" {
		t.Errorf("LifecycleStatus = %q, want %q", resp.Data.LifecycleStatus, "suspended")
	}
	// issue #1534：全鏈路統一 CodeMeta Value 數字碼——wire 斷言維持 "9601" 不變
	// （#1514 時代由輸出邊界轉換得到，現在是 domain 原樣輸出）。
	if resp.Data.SuspensionReasonCode != "9601" {
		t.Errorf("SuspensionReasonCode = %q, want %q", resp.Data.SuspensionReasonCode, "9601")
	}
}

func TestGRPC_GetSeller_ActiveSeller_ReasonCodeEmpty(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Active Shop",
				Status:    domain.SellerStatusActive,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}
	// Deliberately non-empty active map to prove the "not suspended" branch never queries it.
	active := map[int64]*domain.SellerSuspension{
		2600000002: {ID: 1, AccountID: 2600000002, ReasonCode: domain.SellerSuspensionReasonPolicyViolation, Status: domain.SellerSuspensionActive},
	}

	srv := newSellerServer(repo, nil)
	srv.suspensionUC = newSuspensionUC(active, repo)

	client, cleanup := newTestGRPCClient(t, srv)
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2600000002})
	if err != nil {
		t.Fatalf("GetSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("Code = %d, want 0", resp.Code)
	}
	if resp.Data.LifecycleStatus != "active" {
		t.Errorf("LifecycleStatus = %q, want %q", resp.Data.LifecycleStatus, "active")
	}
	if resp.Data.SuspensionReasonCode != "" {
		t.Errorf("SuspensionReasonCode = %q, want empty", resp.Data.SuspensionReasonCode)
	}
}

// TestGRPC_GetSeller_SuspendedSeller_NilSuspensionUC_NoPanic guards the nil-fallback
// used by other handler tests that construct SellerServer without wiring suspensionUC
// (newSellerServer leaves it nil) — must not panic even if a seller happens to carry
// status=suspended.
func TestGRPC_GetSeller_SuspendedSeller_NilSuspensionUC_NoPanic(t *testing.T) {
	now := time.Now()
	repo := &testSellerRepo{
		getByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: id, StoreName: "Suspended Shop",
				Status:    domain.SellerStatusSuspended,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{AccountId: 2600000003})
	if err != nil {
		t.Fatalf("GetSeller error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("Code = %d, want 0", resp.Code)
	}
	if resp.Data.SuspensionReasonCode != "" {
		t.Errorf("SuspensionReasonCode = %q, want empty (suspensionUC nil)", resp.Data.SuspensionReasonCode)
	}
}
