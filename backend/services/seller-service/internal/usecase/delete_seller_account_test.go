package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDelSellerRepo struct {
	seller          *domain.Seller
	statusUpdated   domain.SellerStatus
	updatedStoreID  int64
	statusCalled    bool
	updateStatusErr error
}

func (r *fakeDelSellerRepo) GetByAccountIDForUpdate(_ context.Context, _ int64) (*domain.Seller, error) {
	return r.seller, nil
}
func (r *fakeDelSellerRepo) UpdateStatusByStoreID(_ context.Context, storeID int64, s domain.SellerStatus, _ *time.Time, _ *time.Time, _ time.Time) error {
	r.updatedStoreID = storeID
	r.statusUpdated = s
	r.statusCalled = true
	return r.updateStatusErr
}
func (r *fakeDelSellerRepo) ClearPIIFields(_ context.Context, _ int64, _ time.Time) error { return nil }
func (r *fakeDelSellerRepo) ListPendingPIICleanup(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	return nil, nil
}

// fakeSellerDeletedOutboxRepo is a spy for domain.SellerDeletedOutboxRepository.
type fakeSellerDeletedOutboxRepo struct {
	inserted     *domain.SellerDeletedOutbox
	insertCalled int
}

func (f *fakeSellerDeletedOutboxRepo) Insert(_ context.Context, row *domain.SellerDeletedOutbox) error {
	f.insertCalled++
	cp := *row
	f.inserted = &cp
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) FetchPendingDue(_ context.Context, _ time.Time, _ int) ([]*domain.SellerDeletedOutbox, error) {
	return nil, nil
}
func (f *fakeSellerDeletedOutboxRepo) UpdateAttempt(_ context.Context, _ int64, _ int, _ time.Time, _ string) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkDone(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) Claim(_ context.Context, _ int64, _, _ time.Time) (int64, error) {
	return 1, nil
}

func (f *fakeSellerDeletedOutboxRepo) MarkFailed(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkEventPublished(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkRoleRemoved(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) CountPending(_ context.Context) (int, error) {
	return 0, nil
}
func (f *fakeSellerDeletedOutboxRepo) OldestPendingAge(_ context.Context, _ time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (f *fakeSellerDeletedOutboxRepo) DeleteDoneBefore(_ context.Context, _ time.Time, _ int) (int, error) {
	return 0, nil
}

type fakeMerchantOrderCount struct{ count int }

func (f *fakeMerchantOrderCount) GetActiveMerchantOrderCount(_ context.Context, _ int64) (int, error) {
	return f.count, nil
}

type fakeSellerAccountSoftDelete struct{ called bool }

func (f *fakeSellerAccountSoftDelete) RemoveSellerRole(_ context.Context, _ int64, _ rolestate.OtherRoleState, _ int64) error {
	f.called = true
	return nil
}

type fakeSellerAccountSoftDeleteErr struct{ err error }

func (f *fakeSellerAccountSoftDeleteErr) RemoveSellerRole(_ context.Context, _ int64, _ rolestate.OtherRoleState, _ int64) error {
	return f.err
}

// fakeBuyerClient stubs BuyerClient.HasActiveBuyer for delete-usecase tests
// that don't care about subscriber counts.
type fakeBuyerClient struct {
	hasActive bool
	err       error
}

func (f *fakeBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return f.hasActive, f.err
}

func (f *fakeBuyerClient) GetSubscriberCount(_ context.Context, _ int64) (int64, error) {
	panic("unused")
}

func (f *fakeBuyerClient) BatchGetSubscriberCounts(_ context.Context, _ []int64) (map[int64]int64, error) {
	panic("unused")
}

// fakeRoleRemover is a spy for AccountRoleRemover that records the rolestate.OtherRoleState
// it was called with, so tests can assert what seller-service actually told
// account-service about the buyer side.
type fakeRoleRemover struct {
	lastOtherRole rolestate.OtherRoleState
	err           error
	// storeIDCalledWith records the stint each removal named (issue #3189).
	storeIDCalledWith []int64
}

func (f *fakeRoleRemover) RemoveSellerRole(_ context.Context, _ int64, other rolestate.OtherRoleState, storeID int64) error {
	f.storeIDCalledWith = append(f.storeIDCalledWith, storeID)
	f.lastOtherRole = other
	return f.err
}

// newTestDeleteSellerUseCase builds a DeleteSellerAccountUseCase wired with the
// given buyers/account fakes and an otherwise-happy-path seller (active, no
// active orders) so Execute reaches the RemoveSellerRole call.
func newTestDeleteSellerUseCase(t *testing.T, buyers BuyerClient, account AccountRoleRemover) *DeleteSellerAccountUseCase {
	t.Helper()
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 1, StoreID: 1, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 0}
	audit := &fakeSellerDeletionAudit{}
	outbox := &fakeSellerDeletedOutboxRepo{}
	return NewDeleteSellerAccountUseCase(repo, orders, account, buyers, audit, outbox, fakeTSID, &mockTxManager{}, nil)
}

// TestDeleteSellerAccount_PassesBuyerActive asserts that when the buyer lookup
// reports an active buyer stint, seller-service tells account-service
// rolestate.OtherRoleActive so account-service does NOT cascade-delete the account.
func TestDeleteSellerAccount_PassesBuyerActive(t *testing.T) {
	buyers := &fakeBuyerClient{hasActive: true}
	acct := &fakeRoleRemover{}
	uc := newTestDeleteSellerUseCase(t, buyers, acct)

	require.NoError(t, uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 1}))
	assert.Equal(t, rolestate.OtherRoleActive, acct.lastOtherRole)
}

// TestDeleteSellerAccount_PassesBuyerAbsent asserts that when the buyer lookup
// confirms no buyer, seller-service tells account-service rolestate.OtherRoleAbsent so
// account-service can safely cascade-delete the account.
func TestDeleteSellerAccount_PassesBuyerAbsent(t *testing.T) {
	buyers := &fakeBuyerClient{hasActive: false}
	acct := &fakeRoleRemover{}
	uc := newTestDeleteSellerUseCase(t, buyers, acct)

	require.NoError(t, uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 1}))
	assert.Equal(t, rolestate.OtherRoleAbsent, acct.lastOtherRole)
}

// TestDeleteSellerAccount_BuyerLookupFailsPassesUnknown asserts that when the
// buyer lookup itself errors, seller-service passes rolestate.OtherRoleUnknown so
// account-service fail-closes rather than guessing.
func TestDeleteSellerAccount_BuyerLookupFailsPassesUnknown(t *testing.T) {
	buyers := &fakeBuyerClient{err: errors.New("timeout")}
	acct := &fakeRoleRemover{}
	uc := newTestDeleteSellerUseCase(t, buyers, acct)

	require.NoError(t, uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 1}))
	assert.Equal(t, rolestate.OtherRoleUnknown, acct.lastOtherRole, "查詢失敗要讓 account-service fail-closed")
}

type fakeSellerDeletionAudit struct {
	inserted *domain.SellerDeletionAudit
}

func (f *fakeSellerDeletionAudit) Insert(_ context.Context, a *domain.SellerDeletionAudit) error {
	f.inserted = a
	return nil
}

func fakeTSID() int64 { return 999 }

func TestDeleteSellerAccount_SelfWithOrders_Returns9205(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 1, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 2}
	account := &fakeSellerAccountSoftDelete{}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, account, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 1})
	if err == nil {
		t.Fatal("expected 9205 for seller with active orders, got nil")
	}
	if !pkgerrors.IsCode(err, 9205) {
		t.Fatalf("expected 9205, got: %v", err)
	}
	if repo.statusCalled {
		t.Fatal("status should NOT update when blocked by 9205")
	}
	if account.called {
		t.Fatal("RemoveSellerRole should NOT be called when blocked")
	}
	if audit.inserted != nil {
		t.Fatal("audit should NOT be written when not overriding")
	}
}

func TestDeleteSellerAccount_AdminWithOrdersNoOverride_Returns9205(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 5, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 1}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, &fakeSellerAccountSoftDelete{}, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 5, IsAdmin: true, OverrideActiveOrders: false})
	if !pkgerrors.IsCode(err, 9205) {
		t.Fatalf("admin without override must still hit 9205, got: %v", err)
	}
	if audit.inserted != nil {
		t.Fatal("audit should NOT be written when override=false")
	}
}

func TestDeleteSellerAccount_AdminOverride_WritesAuditAndDeletes(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 7, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 3}
	account := &fakeSellerAccountSoftDelete{}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, account, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{
		AccountID: 7, IsAdmin: true, OperatorID: 42, OverrideActiveOrders: true, Reason: "fraud",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.statusUpdated != domain.SellerStatusClosed {
		t.Fatalf("expected CLOSED, got %s", repo.statusUpdated)
	}
	if audit.inserted == nil {
		t.Fatal("audit row must be written on override")
	}
	if audit.inserted.OperatorID != 42 || audit.inserted.ActiveOrderCount != 3 || audit.inserted.Reason != "fraud" {
		t.Fatalf("audit fields wrong: %+v", audit.inserted)
	}
	if !account.called {
		t.Fatal("expected RemoveSellerRole called")
	}
}

func TestDeleteSellerAccount_NoOrders_Deleted(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 2, StoreID: 777, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 0}
	account := &fakeSellerAccountSoftDelete{}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, account, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	if err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.statusUpdated != domain.SellerStatusClosed {
		t.Fatalf("expected CLOSED, got %s", repo.statusUpdated)
	}
	if repo.updatedStoreID != 777 {
		t.Fatalf("close must target the stint PK (store_id=777), got %d", repo.updatedStoreID)
	}
	if audit.inserted != nil {
		t.Fatal("no audit when orders==0")
	}
	if !account.called {
		t.Fatal("expected RemoveSellerRole called")
	}
}

func TestDeleteSellerAccount_AdminNoOrders_StillWritesAudit(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 6, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 0}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, &fakeSellerAccountSoftDelete{}, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{
		AccountID: 6, IsAdmin: true, OperatorID: 42, Reason: "customer request",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audit.inserted == nil {
		t.Fatal("admin-initiated delete must always write an audit row, even with 0 active orders")
	}
	if audit.inserted.OperatorID != 42 || audit.inserted.ActiveOrderCount != 0 || audit.inserted.Reason != "customer request" {
		t.Fatalf("audit fields wrong: %+v", audit.inserted)
	}
}

func TestDeleteSellerAccount_AlreadyDeleted_RetriesRemoveSellerRole(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 3, Status: domain.SellerStatusClosed}}
	account := &fakeSellerAccountSoftDelete{}
	uc := NewDeleteSellerAccountUseCase(repo, &fakeMerchantOrderCount{count: 0}, account, &fakeBuyerClient{}, &fakeSellerDeletionAudit{}, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 3})
	if err != nil {
		t.Fatalf("expected success (idempotent retry), got: %v", err)
	}
	if !account.called {
		t.Fatal("expected RemoveSellerRole to be retried when seller already deleted")
	}
	if repo.statusCalled {
		t.Fatal("must NOT re-run UpdateStatus when already deleted (would duplicate outbox row)")
	}
}

func TestDeleteSellerAccount_AlreadyDeleted_RemoveSellerRoleFails_ReturnsUnderlyingError(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 3, Status: domain.SellerStatusClosed}}
	account := &fakeSellerAccountSoftDeleteErr{err: errors.New("account-service unavailable")}
	uc := NewDeleteSellerAccountUseCase(repo, &fakeMerchantOrderCount{count: 0}, account, &fakeBuyerClient{}, &fakeSellerDeletionAudit{}, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 3})
	if err == nil {
		t.Fatal("expected the underlying RemoveSellerRole error to surface")
	}
	if pkgerrors.IsCode(err, 9204) {
		t.Fatal("must NOT be 9204 — should be the real RemoveSellerRole failure")
	}
}

func TestDeleteSellerAccount_SuspendedGovernance_StillReturns9204(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 4, Status: domain.SellerStatusSuspendedGovernance}}
	account := &fakeSellerAccountSoftDelete{}
	uc := NewDeleteSellerAccountUseCase(repo, &fakeMerchantOrderCount{count: 0}, account, &fakeBuyerClient{}, &fakeSellerDeletionAudit{}, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 4})
	if !pkgerrors.IsCode(err, 9204) {
		t.Fatalf("suspended_governance must still hit 9204 (unchanged behavior), got: %v", err)
	}
	if account.called {
		t.Fatal("RemoveSellerRole must NOT be called for suspended_governance")
	}
}

func TestDeleteSellerAccount_SelfCannotOverride_Returns9205(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: 9, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 4}
	audit := &fakeSellerDeletionAudit{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, &fakeSellerAccountSoftDelete{}, &fakeBuyerClient{}, audit, &fakeSellerDeletedOutboxRepo{}, fakeTSID, &mockTxManager{}, nil)

	// SELF caller setting OverrideActiveOrders must NOT bypass the 9205 guard.
	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 9, IsAdmin: false, OverrideActiveOrders: true})
	if !pkgerrors.IsCode(err, 9205) {
		t.Fatalf("SELF must never override active-order guard, got: %v", err)
	}
	if audit.inserted != nil {
		t.Fatal("SELF override attempt must not write audit")
	}
}

// TestDeleteSellerAccount_WritesOutboxInTx asserts that Execute writes exactly one
// pending outbox row in the same transaction as UpdateStatus, and does NOT invoke
// any fire-and-forget publisher.
func TestDeleteSellerAccount_WritesOutboxInTx(t *testing.T) {
	const accountID int64 = 20
	repo := &fakeDelSellerRepo{seller: &domain.Seller{AccountID: accountID, StoreID: 777, Status: domain.SellerStatusActive}}
	orders := &fakeMerchantOrderCount{count: 0}
	account := &fakeSellerAccountSoftDelete{}
	audit := &fakeSellerDeletionAudit{}
	outbox := &fakeSellerDeletedOutboxRepo{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, account, &fakeBuyerClient{}, audit, outbox, fakeTSID, &mockTxManager{}, nil)

	if err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: accountID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outbox.insertCalled != 1 {
		t.Fatalf("expected outbox.Insert called once, got %d", outbox.insertCalled)
	}
	if outbox.inserted == nil {
		t.Fatal("outbox row must not be nil")
	}
	if outbox.inserted.Status != domain.SellerDeletedOutboxPending {
		t.Fatalf("expected status=%s, got %s", domain.SellerDeletedOutboxPending, outbox.inserted.Status)
	}
	if outbox.inserted.AccountID != accountID {
		t.Fatalf("expected AccountID=%d, got %d", accountID, outbox.inserted.AccountID)
	}
	if outbox.inserted.AttemptCount != 0 {
		t.Fatalf("expected AttemptCount=0, got %d", outbox.inserted.AttemptCount)
	}
	if outbox.inserted.EventType != domain.SellerLifecycleEventDeleted {
		t.Fatalf("EventType = %q, want seller.deleted", outbox.inserted.EventType)
	}
	var p domain.SellerLifecyclePayload
	if err := json.Unmarshal(outbox.inserted.Payload, &p); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if p.AccountID != accountID || p.StoreID != 777 {
		t.Fatalf("payload = %+v, want {AccountID:%d StoreID:777} (enqueue-time snapshot)", p, accountID)
	}
}

// TestDeleteSellerAccount_UpdateStatusError_NoOutboxRow asserts that when UpdateStatus
// errors (tx rolls back), the outbox Insert is never called.
func TestDeleteSellerAccount_UpdateStatusError_NoOutboxRow(t *testing.T) {
	const accountID int64 = 21
	repo := &fakeDelSellerRepo{
		seller:          &domain.Seller{AccountID: accountID, Status: domain.SellerStatusActive},
		updateStatusErr: errors.New("simulated db error"),
	}
	orders := &fakeMerchantOrderCount{count: 0}
	outbox := &fakeSellerDeletedOutboxRepo{}
	uc := NewDeleteSellerAccountUseCase(repo, orders, &fakeSellerAccountSoftDelete{}, &fakeBuyerClient{}, &fakeSellerDeletionAudit{}, outbox, fakeTSID, &mockTxManager{}, nil)

	err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: accountID})
	if err == nil {
		t.Fatal("expected error from UpdateStatus, got nil")
	}
	if outbox.insertCalled != 0 {
		t.Fatalf("expected NO outbox.Insert on tx failure, got %d calls", outbox.insertCalled)
	}
}
