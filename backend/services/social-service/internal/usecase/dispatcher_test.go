package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

type fakeAccountClientForBan struct {
	suspendCalled bool
	suspendUserID int64
	suspendErr    error
}

func (f *fakeAccountClientForBan) GetAccountInfo(ctx context.Context, accountID int64) (*AccountInfo, error) {
	return nil, nil
}
func (f *fakeAccountClientForBan) GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*AccountInfo, error) {
	return nil, nil
}
func (f *fakeAccountClientForBan) SuspendAccount(ctx context.Context, accountID int64, reason string) error {
	f.suspendCalled = true
	f.suspendUserID = accountID
	return f.suspendErr
}

// fakeBuyerClientForBan defaults to identity passthrough (no divergence) unless
// resolveFn overrides it — mirrors the real Client.ResolvePrincipal fail-open
// default of "echo back unchanged" for a non-buyer_id / RPC failure.
type fakeBuyerClientForBan struct {
	resolveInput int64
	resolveFn    func(int64) int64
	strictFn     func(int64) (PrincipalResolution, error)
}

func (f *fakeBuyerClientForBan) GetBuyerProfile(ctx context.Context, accountID int64) (*AccountInfo, error) {
	return nil, nil
}
func (f *fakeBuyerClientForBan) GetBuyerMetaBatch(ctx context.Context, accountIDs []int64) (map[int64]*AccountInfo, error) {
	return nil, nil
}
func (f *fakeBuyerClientForBan) GetSubscriberIDs(ctx context.Context, sellerID int64) ([]int64, error) {
	return nil, nil
}
func (f *fakeBuyerClientForBan) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	f.resolveInput = buyerID
	if f.resolveFn != nil {
		return f.resolveFn(buyerID)
	}
	return buyerID
}

func (f *fakeBuyerClientForBan) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (PrincipalResolution, error) {
	f.resolveInput = buyerID
	if f.strictFn != nil {
		return f.strictFn(buyerID)
	}
	if f.resolveFn != nil {
		if got := f.resolveFn(buyerID); got != buyerID {
			return PrincipalResolution{AccountID: got, Found: true}, nil
		}
	}
	return PrincipalResolution{}, nil // 查無 → 呼叫端維持原值往下送
}

// fakeSellerClientForBan is the seller-axis mirror of fakeBuyerClientForBan:
// identity passthrough by default (matching the real Client.ResolvePrincipal's
// fail-open "echo back unchanged" for a non-store_id / RPC failure), overridable
// per test via resolveFn.
type fakeSellerClientForBan struct {
	resolveInput int64
	resolveFn    func(int64) int64
	strictFn     func(int64) (PrincipalResolution, error)
}

func (f *fakeSellerClientForBan) GetSellerInfo(ctx context.Context, storeID int64) (*AccountInfo, error) {
	return nil, nil
}
func (f *fakeSellerClientForBan) GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error) {
	return nil, nil
}
func (f *fakeSellerClientForBan) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	f.resolveInput = storeID
	if f.resolveFn != nil {
		return f.resolveFn(storeID)
	}
	return storeID
}

func (f *fakeSellerClientForBan) ResolvePrincipalStrict(ctx context.Context, storeID int64) (PrincipalResolution, error) {
	f.resolveInput = storeID
	if f.strictFn != nil {
		return f.strictFn(storeID)
	}
	if f.resolveFn != nil {
		if got := f.resolveFn(storeID); got != storeID {
			return PrincipalResolution{AccountID: got, Found: true}, nil
		}
	}
	return PrincipalResolution{}, nil // 查無 → 呼叫端維持原值往下送
}

func TestAccountBanDispatcher_BanUser_CallsAccountSuspend(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	d := NewAccountBanDispatcher(accountClient, &fakeSellerClientForBan{}, &fakeBuyerClientForBan{})
	if err := d.BanUser(context.Background(), 999, "repeat offender"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !accountClient.suspendCalled || accountClient.suspendUserID != 999 {
		t.Errorf("expected SuspendAccount(999) to be called")
	}
}

func TestAccountBanDispatcher_BanUser_SuspendFailurePropagates(t *testing.T) {
	accountClient := &fakeAccountClientForBan{suspendErr: errors.New("account-service unavailable")}
	d := NewAccountBanDispatcher(accountClient, &fakeSellerClientForBan{}, &fakeBuyerClientForBan{})
	if err := d.BanUser(context.Background(), 999, "test"); err == nil {
		t.Errorf("expected error to propagate when SuspendAccount fails")
	}
}

func TestAccountBanDispatcher_BanUser_ResolvesDivergedBuyerIDBeforeSuspend(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	buyerClient := &fakeBuyerClientForBan{resolveFn: func(int64) int64 { return 42 }}
	d := NewAccountBanDispatcher(accountClient, &fakeSellerClientForBan{}, buyerClient)
	if err := d.BanUser(context.Background(), 999, "repeat offender"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buyerClient.resolveInput != 999 {
		t.Errorf("expected ResolvePrincipal(999) to be called, got %d", buyerClient.resolveInput)
	}
	if !accountClient.suspendCalled || accountClient.suspendUserID != 42 {
		t.Errorf("expected SuspendAccount(42) (resolved principal), got %d", accountClient.suspendUserID)
	}
}

// TestAccountBanDispatcher_BanUser_ResolvesDivergedStoreIDBeforeSuspend is the
// seller-axis mirror of the buyer-axis test above, and the regression guard for
// the gap it documents: before 2026-08-05 AccountBanDispatcher had no
// sellerClient field at all, so a store_id went into SuspendAccount unresolved
// and account-service answered NotFound — banning a seller was structurally
// impossible.
func TestAccountBanDispatcher_BanUser_ResolvesDivergedStoreIDBeforeSuspend(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	sellerClient := &fakeSellerClientForBan{resolveFn: func(storeID int64) int64 {
		if storeID == 9001 {
			return 42
		}
		return storeID
	}}
	d := NewAccountBanDispatcher(accountClient, sellerClient, &fakeBuyerClientForBan{})
	if err := d.BanUser(context.Background(), 9001, "seller repeat offender"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sellerClient.resolveInput != 9001 {
		t.Errorf("expected seller ResolvePrincipal(9001) to be called, got %d", sellerClient.resolveInput)
	}
	if !accountClient.suspendCalled || accountClient.suspendUserID != 42 {
		t.Errorf("expected SuspendAccount(42) (resolved store principal), got %d", accountClient.suspendUserID)
	}
}

// TestAccountBanDispatcher_BanUser_SellerAxisRunsBeforeBuyerAxis pins the chain
// ORDER, not just that both clients exist: the buyer resolver must see the value
// the seller resolver produced (mirroring BlockUseCase.BlockUser). A future
// refactor that resolves both axes from the ORIGINAL id and picks one would pass
// the two single-axis tests above and fail this one.
func TestAccountBanDispatcher_BanUser_SellerAxisRunsBeforeBuyerAxis(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	sellerClient := &fakeSellerClientForBan{resolveFn: func(storeID int64) int64 {
		if storeID == 9001 {
			return 42
		}
		return storeID
	}}
	buyerClient := &fakeBuyerClientForBan{}
	d := NewAccountBanDispatcher(accountClient, sellerClient, buyerClient)
	if err := d.BanUser(context.Background(), 9001, "order check"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buyerClient.resolveInput != 42 {
		t.Errorf("expected buyer resolver to receive the seller-resolved principal 42, got %d", buyerClient.resolveInput)
	}
}

// TestAccountBanDispatcher_BanUser_ZeroOwnerIsRejected: "no owner" must never
// reach account-service as SuspendAccount(0).
func TestAccountBanDispatcher_BanUser_ZeroOwnerIsRejected(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	d := NewAccountBanDispatcher(accountClient, &fakeSellerClientForBan{}, &fakeBuyerClientForBan{})
	if err := d.BanUser(context.Background(), 0, "no owner"); !errors.Is(err, ErrUnknownTargetOwner) {
		t.Fatalf("expected ErrUnknownTargetOwner, got %v", err)
	}
	if accountClient.suspendCalled {
		t.Error("SuspendAccount must not be called for a zero owner id")
	}
}

func TestAccountBanDispatcher_BanUser_UnresolvableIDPassesThroughUnchanged(t *testing.T) {
	accountClient := &fakeAccountClientForBan{}
	d := NewAccountBanDispatcher(accountClient, &fakeSellerClientForBan{}, &fakeBuyerClientForBan{})
	if err := d.BanUser(context.Background(), 700001, "unresolvable"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !accountClient.suspendCalled || accountClient.suspendUserID != 700001 {
		t.Errorf("expected the unresolved id to be passed through to SuspendAccount, got %d", accountClient.suspendUserID)
	}
}

// TestAccountBanDispatcher_BanUser_ResolverErrorAbortsBan pins the 2026-08-05
// decision: on the BAN path a resolver ERROR is not the same as "that id is not
// mine". A ban is an irreversible write keyed on an identity, so an unanswerable
// resolver must abort the whole resolution — leaving the report PENDING for the
// admin to retry — instead of handing an unresolved store_id/buyer_id to
// SuspendAccount and relying on account-service's GetByID to catch it (which it
// does NOT, in the id-collision case this whole change exists to close).
func TestAccountBanDispatcher_BanUser_ResolverErrorAbortsBan(t *testing.T) {
	rpcDown := func(int64) (PrincipalResolution, error) {
		return PrincipalResolution{}, errors.New("resolve principals: connection refused")
	}
	cases := []struct {
		name   string
		seller *fakeSellerClientForBan
		buyer  *fakeBuyerClientForBan
	}{
		{"seller_axis_error", &fakeSellerClientForBan{strictFn: rpcDown}, &fakeBuyerClientForBan{}},
		{"buyer_axis_error", &fakeSellerClientForBan{}, &fakeBuyerClientForBan{strictFn: rpcDown}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accountClient := &fakeAccountClientForBan{}
			d := NewAccountBanDispatcher(accountClient, tc.seller, tc.buyer)
			if err := d.BanUser(context.Background(), 9001, "rpc down"); err == nil {
				t.Fatal("expected the resolver error to abort the ban")
			}
			if accountClient.suspendCalled {
				t.Error("SuspendAccount must not be called when principal resolution failed")
			}
		})
	}
}

type fakePostRepoForOwner struct {
	domain.PostRepository // nil embed: 任何未 override 的方法被呼叫會 panic
	sellerID              int64
}

func (f *fakePostRepoForOwner) GetByID(ctx context.Context, id int64) (*gen.CmsPost, error) {
	return &gen.CmsPost{ID: id, SellerID: f.sellerID}, nil
}

func TestResolveOwner_POST(t *testing.T) {
	d := &FanoutHideDispatcher{postRepo: &fakePostRepoForOwner{sellerID: 999}}
	ownerID, err := d.ResolveOwner(context.Background(), "POST", 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ownerID != 999 {
		t.Errorf("expected owner 999, got %d", ownerID)
	}
}

func TestResolveOwner_USER(t *testing.T) {
	d := &FanoutHideDispatcher{}
	ownerID, err := d.ResolveOwner(context.Background(), "USER", 555)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ownerID != 555 {
		t.Errorf("expected owner 555 (self), got %d", ownerID)
	}
}

func TestResolveOwner_CrossServiceUnknown(t *testing.T) {
	d := &FanoutHideDispatcher{}
	ownerID, err := d.ResolveOwner(context.Background(), "REVIEW", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ownerID != 0 {
		t.Errorf("expected unknown owner (0), got %d", ownerID)
	}
}

func TestHideTarget_POST_RoutesThroughPostHider(t *testing.T) {
	var hideCalled bool
	repo := &mockPostRepo{
		getByIDFn: func(_ context.Context, id int64) (*gen.CmsPost, error) {
			return &gen.CmsPost{ID: id, SellerID: 777}, nil
		},
		hidePostFn: func(_ context.Context, _ int64, _ string) error {
			hideCalled = true
			return nil
		},
	}
	pub := &mockContentEventPublisher{}
	hider := NewPostHider(repo, NewHashtagUseCase(&mockHashtagRepo{}, func() int64 { return 1 }), pub, &mockSellerClient{}, nil)
	d := &FanoutHideDispatcher{postHider: hider}

	if err := d.HideTarget(context.Background(), "POST", 123, "spam"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hideCalled {
		t.Error("expected postRepo.HidePost to be called via PostHider")
	}
	if len(pub.postHiddenCalls) != 1 || pub.postHiddenCalls[0].sellerID != 777 {
		t.Errorf("expected PostHiddenByPlatform for seller 777, got %+v", pub.postHiddenCalls)
	}
}
