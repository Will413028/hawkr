package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

type mockBlockRepo struct {
	CreateFn func(ctx context.Context, id, blockerID, blockedID int64) (*gen.UserBlock, error)
}

func (m *mockBlockRepo) Create(ctx context.Context, id, blockerID, blockedID int64) (*gen.UserBlock, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, id, blockerID, blockedID)
	}
	return &gen.UserBlock{}, nil
}
func (m *mockBlockRepo) Delete(ctx context.Context, blockerID, blockedID int64) error { return nil }
func (m *mockBlockRepo) Get(ctx context.Context, blockerID, blockedID int64) (*gen.UserBlock, error) {
	return nil, errors.New("not found")
}
func (m *mockBlockRepo) ListByBlocker(ctx context.Context, blockerID int64, limit, offset int64) ([]*gen.UserBlock, error) {
	return nil, nil
}

var _ domain.BlockRepository = (*mockBlockRepo)(nil)

func TestBlockUser_Grandfathered_DifferentPrincipals_Allowed(t *testing.T) {
	uc := NewBlockUseCase(&mockBlockRepo{}, &mockSellerClient{}, &mockBuyerClient{}, nil, func() int64 { return 1 })
	if err := uc.BlockUser(context.Background(), 42, 200); err != nil {
		t.Fatalf("unexpected error blocking a different principal: %v", err)
	}
}

func TestBlockUser_Grandfathered_SameIDBothSides_Blocked(t *testing.T) {
	uc := NewBlockUseCase(&mockBlockRepo{}, &mockSellerClient{}, &mockBuyerClient{}, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 42)
	if !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf, got %v", err)
	}
}

// TestBlockUser_Diverged_BlockedIDResolvesToBlockerPrincipal_StillBlocked is
// the guard-regression case (ADR §13): blockedID (999) is a diverged
// store_id whose principal is 42 — same human as the blocker.
func TestBlockUser_Diverged_BlockedIDResolvesToBlockerPrincipal_StillBlocked(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalFn: func(ctx context.Context, storeID int64) int64 {
			if storeID == 999 {
				return 42
			}
			return storeID
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, &mockBuyerClient{}, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 999)
	if !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf for same-principal block via diverged id, got %v", err)
	}
}

// TestBlockUser_Diverged_BlockerIDResolvesToBlockedPrincipal_StillBlocked is the
// symmetric guard-regression case: unlike
// TestBlockUser_Diverged_BlockedIDResolvesToBlockerPrincipal_StillBlocked (which
// diverges blockedID), this diverges blockerID — proving BOTH sides of the
// resolve-to-principal call are independently exercised, not just one.
func TestBlockUser_Diverged_BlockerIDResolvesToBlockedPrincipal_StillBlocked(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalFn: func(ctx context.Context, storeID int64) int64 {
			if storeID == 999 {
				return 42
			}
			return storeID
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, &mockBuyerClient{}, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 999, 42)
	if !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf for same-principal block via diverged blocker id, got %v", err)
	}
}

// TestBlockUser_Diverged_BuyerAxis_BlockedIDResolvesToBlockerPrincipal_StillBlocked
// is the buyer-axis mirror of TestBlockUser_Diverged_BlockedIDResolvesToBlockerPrincipal_StillBlocked:
// blockedID (888) is a diverged buyer_id whose principal is 42 — same human
// as the blocker. Before block.go also chained buyerClient.ResolvePrincipal,
// the seller-only resolution would echo 888 back unchanged (not a known
// store_id) and fail to catch this as a self-block.
func TestBlockUser_Diverged_BuyerAxis_BlockedIDResolvesToBlockerPrincipal_StillBlocked(t *testing.T) {
	buyerClient := &mockBuyerClient{
		resolvePrincipalFn: func(ctx context.Context, buyerID int64) int64 {
			if buyerID == 888 {
				return 42
			}
			return buyerID
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, &mockSellerClient{}, buyerClient, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 888)
	if !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf for same-principal block via diverged buyer id, got %v", err)
	}
}

// TestBlockUser_Diverged_BuyerAxis_BlockerIDResolvesToBlockedPrincipal_StillBlocked
// is the symmetric buyer-axis case: diverges blockerID instead of blockedID,
// proving both sides go through the buyer resolver too, not just one.
func TestBlockUser_Diverged_BuyerAxis_BlockerIDResolvesToBlockedPrincipal_StillBlocked(t *testing.T) {
	buyerClient := &mockBuyerClient{
		resolvePrincipalFn: func(ctx context.Context, buyerID int64) int64 {
			if buyerID == 888 {
				return 42
			}
			return buyerID
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, &mockSellerClient{}, buyerClient, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 888, 42)
	if !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf for same-principal block via diverged buyer blocker id, got %v", err)
	}
}

// TestBlockUser_CrossAxis_DifferentPrincipals_Allowed exercises both
// resolvers together (blockerID resolves via the buyer axis, blockedID via
// the seller axis) to different principals, proving the seller→buyer
// resolution chain doesn't produce false positives when the two axes are
// simply unrelated to each other.
func TestBlockUser_CrossAxis_DifferentPrincipals_Allowed(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalFn: func(ctx context.Context, storeID int64) int64 {
			if storeID == 999 {
				return 99
			}
			return storeID
		},
	}
	buyerClient := &mockBuyerClient{
		resolvePrincipalFn: func(ctx context.Context, buyerID int64) int64 {
			if buyerID == 888 {
				return 42
			}
			return buyerID
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, buyerClient, nil, func() int64 { return 1 })
	if err := uc.BlockUser(context.Background(), 888, 999); err != nil {
		t.Fatalf("unexpected error blocking a different principal across axes: %v", err)
	}
}

// ==================== #2789：身分未解析時的守門行為 ====================

// TestBlockUser_UnresolvedBlocked_FailsClosed is the discriminating test for
// stage B2a's social half. blockedID is not a live store_id and not a live
// buyer_id (soft-deleted stint, or an id belonging to neither space); blockerID
// resolves normally.
//
// It fails against BOTH rejected designs:
//   - today's echo-back (`return id`): 999 != 42, guard passes, block created
//   - the "return 0" proposal:         0   != 42, guard passes, block created
//
// Only reading unresolved_ids and refusing produces an error here.
func TestBlockUser_UnresolvedBlocked_FailsClosed(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalStrictFn: func(_ context.Context, storeID int64) (PrincipalResolution, error) {
			if storeID == 999 {
				return PrincipalResolution{}, nil // neither axis knows it
			}
			return PrincipalResolution{AccountID: storeID, Found: true}, nil
		},
	}
	buyerClient := &mockBuyerClient{
		resolvePrincipalStrictFn: func(_ context.Context, buyerID int64) (PrincipalResolution, error) {
			if buyerID == 999 {
				return PrincipalResolution{}, nil
			}
			return PrincipalResolution{AccountID: buyerID, Found: true}, nil
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, buyerClient, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 999)
	if !errors.Is(err, ErrUnresolvedBlockPrincipal) {
		t.Fatalf("expected ErrUnresolvedBlockPrincipal for an unresolvable blocked id, got %v", err)
	}
}

// TestBlockUser_BothSidesUnresolved_NotReportedAsSelfBlock is the pin against
// the "unresolved -> return 0" proposal specifically.
//
// Under that proposal both sides collapse to 0, `0 == 0` holds, and two
// genuinely different people produce ErrCannotBlockSelf — a false positive
// dressed up as a guard hit, and (unlike the fail-open it replaced) one the
// user sees as "you cannot block yourself" about somebody else. Fail-closed
// refuses too, so the assertion is on WHICH refusal fires.
func TestBlockUser_BothSidesUnresolved_NotReportedAsSelfBlock(t *testing.T) {
	unresolvedSeller := func(_ context.Context, _ int64) (PrincipalResolution, error) {
		return PrincipalResolution{}, nil
	}
	unresolvedBuyer := func(_ context.Context, _ int64) (PrincipalResolution, error) {
		return PrincipalResolution{}, nil
	}
	uc := NewBlockUseCase(&mockBlockRepo{},
		&mockSellerClient{resolvePrincipalStrictFn: unresolvedSeller},
		&mockBuyerClient{resolvePrincipalStrictFn: unresolvedBuyer},
		nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 999)
	if errors.Is(err, ErrCannotBlockSelf) {
		t.Fatal("two unresolved ids must never be collapsed into a self-block verdict")
	}
	if !errors.Is(err, ErrUnresolvedBlockPrincipal) {
		t.Fatalf("expected ErrUnresolvedBlockPrincipal, got %v", err)
	}
}

// TestBlockUser_ResolveError_IsNotAnUnresolvedVerdict keeps "the role service
// is down" separate from "this id is not a live surrogate". Both refuse, but
// the former is not a statement about the id, and the gRPC handler maps only
// the latter to InvalidArgument.
func TestBlockUser_ResolveError_IsNotAnUnresolvedVerdict(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalStrictFn: func(_ context.Context, _ int64) (PrincipalResolution, error) {
			return PrincipalResolution{}, errors.New("seller-service unavailable")
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, &mockBuyerClient{}, nil, func() int64 { return 1 })
	err := uc.BlockUser(context.Background(), 42, 200)
	if err == nil {
		t.Fatal("expected an error when the seller axis cannot answer, got nil")
	}
	if errors.Is(err, ErrUnresolvedBlockPrincipal) || errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("a transport failure must not be reported as a verdict about the ids, got %v", err)
	}
}

// TestBlockUser_SellerAxisResolves_BuyerAxisSaysNotMine_StillResolved pins the
// stickiness of the seller-then-buyer chain. A store_id resolves on the seller
// axis; the resulting account_id is then offered to the buyer axis, which
// legitimately answers "not mine". That second answer must not undo the first
// — requiring BOTH axes to claim an id would refuse every seller-only account.
func TestBlockUser_SellerAxisResolves_BuyerAxisSaysNotMine_StillResolved(t *testing.T) {
	sellerClient := &mockSellerClient{
		resolvePrincipalStrictFn: func(_ context.Context, storeID int64) (PrincipalResolution, error) {
			if storeID == 999 {
				return PrincipalResolution{AccountID: 77, Found: true}, nil
			}
			return PrincipalResolution{AccountID: storeID, Found: true}, nil
		},
	}
	buyerClient := &mockBuyerClient{
		resolvePrincipalStrictFn: func(_ context.Context, _ int64) (PrincipalResolution, error) {
			return PrincipalResolution{}, nil // no buyer stint for either account
		},
	}
	uc := NewBlockUseCase(&mockBlockRepo{}, sellerClient, buyerClient, nil, func() int64 { return 1 })
	if err := uc.BlockUser(context.Background(), 42, 999); err != nil {
		t.Fatalf("seller-axis resolution must survive the buyer axis answering 'not mine': %v", err)
	}
}
