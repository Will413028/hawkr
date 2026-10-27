package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

type mockSellerMembershipRepo struct {
	GetPlanByCodeFn   func(ctx context.Context, planCode string) (domain.SellerMembershipPlan, error)
	FindByRequestIDFn func(ctx context.Context, requestID string) (domain.SellerMembershipSubscription, bool, error)
	AssignPlanTxFn    func(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error)
	ListPlansFn       func(ctx context.Context) ([]domain.SellerMembershipPlan, error)
	UpdatePlanFn      func(ctx context.Context, in domain.UpdateSellerMembershipPlanInput) (domain.SellerMembershipPlan, error)
}

func (m *mockSellerMembershipRepo) GetPlanByCode(ctx context.Context, planCode string) (domain.SellerMembershipPlan, error) {
	return m.GetPlanByCodeFn(ctx, planCode)
}
func (m *mockSellerMembershipRepo) ListPlans(ctx context.Context) ([]domain.SellerMembershipPlan, error) {
	return m.ListPlansFn(ctx)
}
func (m *mockSellerMembershipRepo) UpdatePlan(ctx context.Context, in domain.UpdateSellerMembershipPlanInput) (domain.SellerMembershipPlan, error) {
	return m.UpdatePlanFn(ctx, in)
}
func (m *mockSellerMembershipRepo) GetActiveSubscription(ctx context.Context, storeID int64) (domain.SellerMembershipSubscription, error) {
	panic("not used by AssignMembership tests")
}
func (m *mockSellerMembershipRepo) FindByRequestID(ctx context.Context, requestID string) (domain.SellerMembershipSubscription, bool, error) {
	return m.FindByRequestIDFn(ctx, requestID)
}
func (m *mockSellerMembershipRepo) AssignPlan(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error) {
	panic("AssignMembership uses AssignPlanTx, not AssignPlan")
}
func (m *mockSellerMembershipRepo) AssignPlanTx(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error) {
	return m.AssignPlanTxFn(ctx, in)
}

// noopNewID is passed to NewSellerMembershipUseCase in these mock-based tests.
// Neither scenario below reaches the txMgr.WithTx path (the first returns on
// the plan-code lookup error, the second on the idempotent found-by-requestID
// branch), so newID is never actually invoked; it exists only to satisfy the
// constructor signature.
func noopNewID() int64 {
	panic("newID should not be called by these AssignMembership scenarios")
}

func TestAssignMembership_RejectsUnknownPlanCode(t *testing.T) {
	repo := &mockSellerMembershipRepo{
		FindByRequestIDFn: func(ctx context.Context, requestID string) (domain.SellerMembershipSubscription, bool, error) {
			return domain.SellerMembershipSubscription{}, false, nil
		},
		GetPlanByCodeFn: func(ctx context.Context, planCode string) (domain.SellerMembershipPlan, error) {
			return domain.SellerMembershipPlan{}, pgx.ErrNoRows
		},
	}
	uc := NewSellerMembershipUseCase(repo, nil, nil, noopNewID) // outboxRepo/txMgr unused before the plan-code check
	_, err := uc.AssignMembership(context.Background(), AssignMembershipRequest{
		StoreID: 42, PlanCode: "does-not-exist", RequestID: "req-1",
	})
	require.Equal(t, bizcode.SellerInvalidPlanCode, int32(pkgerrors.GetCode(err)))
}

func TestAssignMembership_SameRequestID_ReturnsExistingWithoutReassigning(t *testing.T) {
	existing := domain.SellerMembershipSubscription{
		PlanCode: "pro", EffectiveAt: time.Now(),
	}
	assignCalled := false
	repo := &mockSellerMembershipRepo{
		FindByRequestIDFn: func(ctx context.Context, requestID string) (domain.SellerMembershipSubscription, bool, error) {
			require.Equal(t, "req-dup", requestID)
			return existing, true, nil
		},
		AssignPlanTxFn: func(ctx context.Context, in domain.AssignSellerMembershipInput) (domain.SellerMembershipSubscription, error) {
			assignCalled = true
			return domain.SellerMembershipSubscription{}, nil
		},
		GetPlanByCodeFn: func(ctx context.Context, planCode string) (domain.SellerMembershipPlan, error) {
			return domain.SellerMembershipPlan{PlanCode: "pro", DisplayName: "旗艦方案", TierRank: 2, MonthlyOrderCap: 1000}, nil
		},
	}
	uc := NewSellerMembershipUseCase(repo, nil, nil, noopNewID)
	got, err := uc.AssignMembership(context.Background(), AssignMembershipRequest{
		StoreID: 42, PlanCode: "pro", RequestID: "req-dup",
	})
	require.NoError(t, err)
	require.False(t, assignCalled, "duplicate requestId must not call AssignPlanTx again")
	require.Equal(t, "pro", got.PlanCode)
}

func TestListMembershipPlanCatalog_ReturnsPlansFromRepo(t *testing.T) {
	want := []domain.SellerMembershipPlan{
		{PlanCode: "free", DisplayName: "免費方案", TierRank: 0, MonthlyOrderCap: 50, IsPurchasable: true},
		{PlanCode: "plus", DisplayName: "進階方案", TierRank: 1, MonthlyOrderCap: 200, IsPurchasable: true},
	}
	repo := &mockSellerMembershipRepo{
		ListPlansFn: func(ctx context.Context) ([]domain.SellerMembershipPlan, error) {
			return want, nil
		},
	}
	uc := NewSellerMembershipUseCase(repo, nil, nil, noopNewID)

	got, err := uc.ListMembershipPlanCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "free", got[0].PlanCode)
	require.Equal(t, 50, got[0].MonthlyOrderCap)
	require.Equal(t, "plus", got[1].PlanCode)
	require.Equal(t, 1, got[1].TierRank, "tier_rank surfaces in the read model even though it's not editable")
}

func TestUpdateMembershipPlanCatalog_PassesOnlyProvidedFieldsToRepo(t *testing.T) {
	cap := 500
	var captured domain.UpdateSellerMembershipPlanInput
	repo := &mockSellerMembershipRepo{
		UpdatePlanFn: func(ctx context.Context, in domain.UpdateSellerMembershipPlanInput) (domain.SellerMembershipPlan, error) {
			captured = in
			return domain.SellerMembershipPlan{PlanCode: "plus", MonthlyOrderCap: cap}, nil
		},
	}
	uc := NewSellerMembershipUseCase(repo, nil, nil, noopNewID)

	got, err := uc.UpdateMembershipPlanCatalog(context.Background(), UpdateMembershipPlanCatalogRequest{
		PlanCode: "plus", MonthlyOrderCap: &cap,
	})
	require.NoError(t, err)
	require.Equal(t, "plus", captured.PlanCode)
	require.NotNil(t, captured.MonthlyOrderCap)
	require.Equal(t, 500, *captured.MonthlyOrderCap)
	require.Nil(t, captured.MonthlyPriceTWD, "unset field must stay nil, not zero-valued, so the repo COALESCEs it")
	require.Nil(t, captured.IsPurchasable)
	require.Equal(t, "plus", got.PlanCode)
}

func TestUpdateMembershipPlanCatalog_RejectsNonPositiveCap(t *testing.T) {
	zero := 0
	uc := NewSellerMembershipUseCase(&mockSellerMembershipRepo{}, nil, nil, noopNewID)

	_, err := uc.UpdateMembershipPlanCatalog(context.Background(), UpdateMembershipPlanCatalogRequest{
		PlanCode: "plus", MonthlyOrderCap: &zero,
	})
	require.Error(t, err)
	require.Equal(t, int32(pkgerrors.CodeInvalidInput), int32(pkgerrors.GetCode(err)))
}

func TestUpdateMembershipPlanCatalog_RejectsNegativePrice(t *testing.T) {
	neg := -1
	uc := NewSellerMembershipUseCase(&mockSellerMembershipRepo{}, nil, nil, noopNewID)

	_, err := uc.UpdateMembershipPlanCatalog(context.Background(), UpdateMembershipPlanCatalogRequest{
		PlanCode: "plus", MonthlyPriceTWD: &neg,
	})
	require.Error(t, err)
	require.Equal(t, int32(pkgerrors.CodeInvalidInput), int32(pkgerrors.GetCode(err)))
}

func TestUpdateMembershipPlanCatalog_NoFieldsProvided_ReturnsInvalidInput(t *testing.T) {
	uc := NewSellerMembershipUseCase(&mockSellerMembershipRepo{}, nil, nil, noopNewID)

	_, err := uc.UpdateMembershipPlanCatalog(context.Background(), UpdateMembershipPlanCatalogRequest{PlanCode: "plus"})
	require.Error(t, err)
	require.Equal(t, int32(pkgerrors.CodeInvalidInput), int32(pkgerrors.GetCode(err)))
}

func TestUpdateMembershipPlanCatalog_UnknownPlanCode_ReturnsSellerInvalidPlanCode(t *testing.T) {
	cap := 500
	repo := &mockSellerMembershipRepo{
		UpdatePlanFn: func(ctx context.Context, in domain.UpdateSellerMembershipPlanInput) (domain.SellerMembershipPlan, error) {
			return domain.SellerMembershipPlan{}, pgx.ErrNoRows
		},
	}
	uc := NewSellerMembershipUseCase(repo, nil, nil, noopNewID)

	_, err := uc.UpdateMembershipPlanCatalog(context.Background(), UpdateMembershipPlanCatalogRequest{
		PlanCode: "does-not-exist", MonthlyOrderCap: &cap,
	})
	require.Equal(t, bizcode.SellerInvalidPlanCode, int32(pkgerrors.GetCode(err)))
}
