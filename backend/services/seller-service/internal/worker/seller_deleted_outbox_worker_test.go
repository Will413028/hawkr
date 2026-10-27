package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

// testOutboxMetrics is shared by every test in this package that calls
// NewSellerDeletedOutboxWorker — see buyer-service's identical rationale.
var testOutboxMetrics = outbox.NewMetrics("seller_deleted_test")

// --- fakes ---

type fakeSellerDeletedOutboxRepo struct {
	pending               []*domain.SellerDeletedOutbox
	UpdateAttemptCalls    []fakeUpdateAttemptCall
	MarkDoneCalls         []int64
	MarkFailedCalls       []int64
	MarkEventPublishedIDs []int64
	MarkRoleRemovedIDs    []int64

	// issue #2294 搶佔閘門：ClaimCalls 記錄每次 lease CAS；
	// ClaimLost/ClaimErr 讓測試模擬「另一個副本先搶到」與「claim 本身失敗」。
	ClaimCalls []fakeClaimCall
	ClaimLost  bool
	ClaimErr   error
}

type fakeClaimCall struct {
	ID              int64
	LeaseUntil      time.Time
	ClaimableBefore time.Time
}

type fakeUpdateAttemptCall struct {
	ID            int64
	AttemptCount  int
	NextAttemptAt time.Time
	LastError     string
}

func (f *fakeSellerDeletedOutboxRepo) Insert(_ context.Context, _ *domain.SellerDeletedOutbox) error {
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) FetchPendingDue(_ context.Context, _ time.Time, _ int) ([]*domain.SellerDeletedOutbox, error) {
	return f.pending, nil
}
func (f *fakeSellerDeletedOutboxRepo) UpdateAttempt(_ context.Context, id int64, attempt int, next time.Time, lastErr string) error {
	f.UpdateAttemptCalls = append(f.UpdateAttemptCalls, fakeUpdateAttemptCall{
		ID: id, AttemptCount: attempt, NextAttemptAt: next, LastError: lastErr,
	})
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) Claim(_ context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	f.ClaimCalls = append(f.ClaimCalls, fakeClaimCall{ID: id, LeaseUntil: leaseUntil, ClaimableBefore: claimableBefore})
	if f.ClaimErr != nil {
		return 0, f.ClaimErr
	}
	if f.ClaimLost {
		return 0, nil
	}
	return 1, nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkDone(_ context.Context, id int64, _ time.Time) error {
	f.MarkDoneCalls = append(f.MarkDoneCalls, id)
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkFailed(_ context.Context, id int64, _ string, _ time.Time) error {
	f.MarkFailedCalls = append(f.MarkFailedCalls, id)
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkEventPublished(_ context.Context, id int64, _ time.Time) error {
	f.MarkEventPublishedIDs = append(f.MarkEventPublishedIDs, id)
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) MarkRoleRemoved(_ context.Context, id int64, _ time.Time) error {
	f.MarkRoleRemovedIDs = append(f.MarkRoleRemovedIDs, id)
	return nil
}
func (f *fakeSellerDeletedOutboxRepo) CountPending(_ context.Context) (int, error) {
	return len(f.pending), nil
}
func (f *fakeSellerDeletedOutboxRepo) OldestPendingAge(_ context.Context, _ time.Time) (time.Duration, bool, error) {
	return 0, false, nil
}
func (f *fakeSellerDeletedOutboxRepo) DeleteDoneBefore(_ context.Context, _ time.Time, _ int) (int, error) {
	return 0, nil
}

type fakeSellerDeletedPublisher struct {
	err                     error
	PublishedIDs            []int64
	DeletedEvents           []*pkgmq.SellerDeletedEvent
	OnboardedEvents         []*pkgmq.SellerOnboardedEvent
	SuspendedEvents         []*pkgmq.SellerSuspendedEvent
	UnsuspendedEvents       []*pkgmq.SellerUnsuspendedEvent
	MembershipChangedEvents []*pkgmq.SellerMembershipChangedEvent
	// delayFirst 讓第一次 publish 耗掉一段可觀測的時間，供
	// TestSellerDeletedOutboxWorker_LeaseBaselineIsPerItemNotPerTick 鑑別租約基準。
	delayFirst time.Duration
}

func (f *fakeSellerDeletedPublisher) PublishSellerDeleted(_ context.Context, event *pkgmq.SellerDeletedEvent) error {
	if f.delayFirst > 0 && len(f.DeletedEvents) == 0 {
		time.Sleep(f.delayFirst)
	}
	if f.err != nil {
		return f.err
	}
	f.PublishedIDs = append(f.PublishedIDs, event.AccountID)
	f.DeletedEvents = append(f.DeletedEvents, event)
	return nil
}

func (f *fakeSellerDeletedPublisher) PublishSellerOnboarded(_ context.Context, event *pkgmq.SellerOnboardedEvent) error {
	if f.err != nil {
		return f.err
	}
	f.OnboardedEvents = append(f.OnboardedEvents, event)
	return nil
}

func (f *fakeSellerDeletedPublisher) PublishSellerSuspended(_ context.Context, event *pkgmq.SellerSuspendedEvent) error {
	if f.err != nil {
		return f.err
	}
	f.SuspendedEvents = append(f.SuspendedEvents, event)
	return nil
}

func (f *fakeSellerDeletedPublisher) PublishSellerUnsuspended(_ context.Context, event *pkgmq.SellerUnsuspendedEvent) error {
	if f.err != nil {
		return f.err
	}
	f.UnsuspendedEvents = append(f.UnsuspendedEvents, event)
	return nil
}

func (f *fakeSellerDeletedPublisher) PublishSellerMembershipChanged(_ context.Context, event *pkgmq.SellerMembershipChangedEvent) error {
	if f.err != nil {
		return f.err
	}
	f.MembershipChangedEvents = append(f.MembershipChangedEvents, event)
	return nil
}

type fakeSellerRoleRemover struct {
	err                 error
	calledWith          []int64
	otherRoleCalledWith []rolestate.OtherRoleState
	// storeIDCalledWith records the stint each removal named — the fence input
	// (issue #3189). 0 means the row carried no attribution.
	storeIDCalledWith []int64
}

func (f *fakeSellerRoleRemover) RemoveSellerRole(_ context.Context, accountID int64, other rolestate.OtherRoleState, storeID int64) error {
	f.calledWith = append(f.calledWith, accountID)
	f.otherRoleCalledWith = append(f.otherRoleCalledWith, other)
	f.storeIDCalledWith = append(f.storeIDCalledWith, storeID)
	if f.err != nil {
		return f.err
	}
	return nil
}

// fakeSellerRoleGranter records the onboarding grants the worker performed.
type fakeSellerRoleGranter struct {
	err        error
	calledWith []int64 // accountIDs
	storeIDs   []int64 // the stint each grant was attributed to
}

func (f *fakeSellerRoleGranter) BindSellerRole(_ context.Context, accountID, storeID int64) error {
	f.calledWith = append(f.calledWith, accountID)
	f.storeIDs = append(f.storeIDs, storeID)
	return f.err
}

// fakeCurrentStint is the level-triggered lookup the grant sub-step reads at
// apply time. A nil seller means "no live stint", i.e. NotFound.
type fakeCurrentStint struct {
	seller *domain.Seller
	err    error
	calls  int
}

func (f *fakeCurrentStint) GetByAccountID(_ context.Context, accountID int64) (*domain.Seller, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.seller == nil {
		return nil, pkgerrors.NewNotFound("seller")
	}
	return f.seller, nil
}

// fakeWorkerBuyerClient stubs BuyerClient.HasActiveBuyer for worker
// tests that don't care about subscriber counts.
type fakeWorkerBuyerClient struct {
	hasActive bool
	err       error
}

func (f *fakeWorkerBuyerClient) HasActiveBuyer(_ context.Context, _ int64) (bool, error) {
	return f.hasActive, f.err
}

func (f *fakeWorkerBuyerClient) GetSubscriberCount(_ context.Context, _ int64) (int64, error) {
	panic("unused")
}

func (f *fakeWorkerBuyerClient) BatchGetSubscriberCounts(_ context.Context, _ []int64) (map[int64]int64, error) {
	panic("unused")
}

func newPendingSellerRow(id int64, accountID int64, attempt int) *domain.SellerDeletedOutbox {
	return &domain.SellerDeletedOutbox{
		ID:            id,
		AccountID:     accountID,
		Status:        domain.SellerDeletedOutboxPending,
		AttemptCount:  attempt,
		NextAttemptAt: time.Now().Add(-time.Minute),
	}
}

func newEventPublishedRow(id int64, accountID int64, attempt int) *domain.SellerDeletedOutbox {
	row := newPendingSellerRow(id, accountID, attempt)
	published := time.Now().Add(-time.Hour)
	row.EventPublishedAt = &published
	return row
}

// --- tests ---

// TestSellerDeletedOutboxWorker_Success_MarkDone verifies that a successful
// publish results in MarkDone being called with the row id, and the correct
// AccountID is published.
func TestSellerDeletedOutboxWorker_Success_MarkDone(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}

	w := NewSellerDeletedOutboxWorker(repo, pub, &fakeSellerRoleRemover{}, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 1 {
		t.Fatalf("MarkDoneCalls = %+v, want [1]", repo.MarkDoneCalls)
	}
	if len(pub.PublishedIDs) != 1 || pub.PublishedIDs[0] != 1001 {
		t.Errorf("PublishedIDs = %v, want [1001]", pub.PublishedIDs)
	}
	if len(repo.UpdateAttemptCalls) != 0 {
		t.Errorf("expected no UpdateAttempt calls, got %+v", repo.UpdateAttemptCalls)
	}
}

// TestSellerDeletedOutboxWorker_PublishError_BumpAttempt verifies that a
// publish failure calls UpdateAttempt with attempt+1 and a future next_attempt_at
// (backoff applied), and does NOT call MarkDone or MarkFailed.
func TestSellerDeletedOutboxWorker_PublishError_BumpAttempt(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(2, 2002, 0)},
	}
	pub := &fakeSellerDeletedPublisher{err: errors.New("rabbitmq connection refused")}

	w := NewSellerDeletedOutboxWorker(repo, pub, &fakeSellerRoleRemover{}, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	attemptFailedBefore := testutil.ToFloat64(testOutboxMetrics.AttemptFailedTotal("process"))
	before := time.Now()
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	after := time.Now()

	// OnAttemptFailed(cause="process") must fire and be counted — this is
	// the only signal a struggling (not-yet-terminal) row produces before it
	// either recovers or exhausts MaxAttempts.
	require.InDelta(t, attemptFailedBefore+1, testutil.ToFloat64(testOutboxMetrics.AttemptFailedTotal("process")), 0.0001)

	if len(repo.UpdateAttemptCalls) != 1 {
		t.Fatalf("UpdateAttemptCalls = %+v, want 1 call", repo.UpdateAttemptCalls)
	}
	call := repo.UpdateAttemptCalls[0]
	if call.AttemptCount != 1 {
		t.Errorf("attempt = %d, want 1", call.AttemptCount)
	}
	// Expected delay for newAttempt=1: backoffFor(1) = 2*1min = 2min.
	expectedMin := before.Add(2 * time.Minute)
	expectedMax := after.Add(2 * time.Minute).Add(time.Second) // small slack
	if call.NextAttemptAt.Before(expectedMin) || call.NextAttemptAt.After(expectedMax) {
		t.Errorf("nextAttemptAt = %v, want approx now+2m (between %v and %v)",
			call.NextAttemptAt, expectedMin, expectedMax)
	}
	if len(repo.MarkDoneCalls) != 0 || len(repo.MarkFailedCalls) != 0 {
		t.Errorf("unexpected MarkDone/MarkFailed; done=%v failed=%v",
			repo.MarkDoneCalls, repo.MarkFailedCalls)
	}
}

// TestSellerDeletedOutboxWorker_AtCap_MarkFailed verifies that when a row has
// already reached attempt 4 (one below maxAttempts=5) and publish fails again,
// MarkFailed is called and UpdateAttempt is NOT called.
func TestSellerDeletedOutboxWorker_AtCap_MarkFailed(t *testing.T) {
	// attempt=4 already; one more failure hits maxAttempts=5 → MarkFailed
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(3, 3003, 4)},
	}
	pub := &fakeSellerDeletedPublisher{err: errors.New("timeout")}

	w := NewSellerDeletedOutboxWorker(repo, pub, &fakeSellerRoleRemover{}, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	before := testutil.ToFloat64(testOutboxMetrics.FailedTotal())
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(repo.MarkFailedCalls) != 1 || repo.MarkFailedCalls[0] != 3 {
		t.Fatalf("MarkFailedCalls = %+v, want [3]", repo.MarkFailedCalls)
	}
	if len(repo.UpdateAttemptCalls) != 0 {
		t.Errorf("unexpected UpdateAttempt at terminal; got %+v", repo.UpdateAttemptCalls)
	}
	require.InDelta(t, before+1, testutil.ToFloat64(testOutboxMetrics.FailedTotal()), 0.0001)
}

// TestSellerDeletedOutboxWorker_PublisherNil_SkipsRow verifies that when the
// worker is constructed with a nil publisher (MQ unavailable at startup),
// RunOnce is a no-op and never mutates the repo — mirrors buyer-service's
// identical publisherMissing rationale (pkg/outbox always claims before
// invoking ProcessFunc; gating here avoids claiming a row we already know we
// can't publish for).
func TestSellerDeletedOutboxWorker_PublisherNil_SkipsRow(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(4, 4004, 0)},
	}
	w := NewSellerDeletedOutboxWorker(repo, nil, &fakeSellerRoleRemover{}, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.MarkDoneCalls) != 0 || len(repo.UpdateAttemptCalls) != 0 || len(repo.MarkFailedCalls) != 0 ||
		len(repo.MarkEventPublishedIDs) != 0 || len(repo.MarkRoleRemovedIDs) != 0 {
		t.Fatalf("expected no repo mutation when publisher nil, got done=%v update=%v failed=%v eventPublished=%v roleRemoved=%v",
			repo.MarkDoneCalls, repo.UpdateAttemptCalls, repo.MarkFailedCalls, repo.MarkEventPublishedIDs, repo.MarkRoleRemovedIDs)
	}
}

// TestSellerDeletedOutboxWorker_Success_AlsoRemovesRole verifies that a fresh
// row goes through publish AND role removal before MarkDone fires.
func TestSellerDeletedOutboxWorker_Success_AlsoRemovesRole(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(remover.calledWith) != 1 || remover.calledWith[0] != 1001 {
		t.Fatalf("expected RemoveSellerRole(1001) called once, got %v", remover.calledWith)
	}
	if len(repo.MarkEventPublishedIDs) != 1 || repo.MarkEventPublishedIDs[0] != 1 {
		t.Fatalf("MarkEventPublishedIDs = %v, want [1]", repo.MarkEventPublishedIDs)
	}
	if len(repo.MarkRoleRemovedIDs) != 1 || repo.MarkRoleRemovedIDs[0] != 1 {
		t.Fatalf("MarkRoleRemovedIDs = %v, want [1]", repo.MarkRoleRemovedIDs)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 1 {
		t.Fatalf("MarkDoneCalls = %v, want [1]", repo.MarkDoneCalls)
	}
}

// TestSellerDeletedOutboxWorker_EventAlreadyPublished_OnlyRetriesRoleRemoval verifies
// that a row whose event was already published on a prior tick does NOT republish,
// and completes once role removal succeeds.
func TestSellerDeletedOutboxWorker_EventAlreadyPublished_OnlyRetriesRoleRemoval(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newEventPublishedRow(2, 2002, 1)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(pub.PublishedIDs) != 0 {
		t.Fatalf("expected NO republish for already-published row, got %v", pub.PublishedIDs)
	}
	if len(repo.MarkEventPublishedIDs) != 0 {
		t.Fatalf("expected NO MarkEventPublished call, got %v", repo.MarkEventPublishedIDs)
	}
	if len(remover.calledWith) != 1 || remover.calledWith[0] != 2002 {
		t.Fatalf("expected RemoveSellerRole(2002) retried, got %v", remover.calledWith)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 2 {
		t.Fatalf("MarkDoneCalls = %v, want [2]", repo.MarkDoneCalls)
	}
}

// TestSellerDeletedOutboxWorker_RoleRemovalFails_BumpAttempt_NoMarkDone verifies that
// a role-removal failure (after successful publish) bumps the attempt and does NOT
// mark the row done.
func TestSellerDeletedOutboxWorker_RoleRemovalFails_BumpAttempt_NoMarkDone(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newEventPublishedRow(3, 3003, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{err: errors.New("account-service unavailable")}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(repo.UpdateAttemptCalls) != 1 {
		t.Fatalf("UpdateAttemptCalls = %+v, want 1 call", repo.UpdateAttemptCalls)
	}
	if len(repo.MarkDoneCalls) != 0 {
		t.Fatalf("expected NO MarkDone on role-removal failure, got %v", repo.MarkDoneCalls)
	}
	if len(repo.MarkRoleRemovedIDs) != 0 {
		t.Fatalf("expected NO MarkRoleRemoved on failure, got %v", repo.MarkRoleRemovedIDs)
	}
}

// TestSellerDeletedOutboxWorker_RoleRemoval_PassesBuyerActive verifies that the
// worker's own retry path queries HasActiveBuyer (not just the primary
// synchronous DeleteSellerAccountUseCase call) and passes OtherRoleActive
// through to RemoveSellerRole when the account still has an active buyer.
// This closes the crash-recovery gap where this retry could be the first (and
// only) RemoveSellerRole call for the account.
func TestSellerDeletedOutboxWorker_RoleRemoval_PassesBuyerActive(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}
	buyers := &fakeWorkerBuyerClient{hasActive: true}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, buyers, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(remover.otherRoleCalledWith) != 1 || remover.otherRoleCalledWith[0] != rolestate.OtherRoleActive {
		t.Fatalf("otherRoleCalledWith = %v, want [OtherRoleActive]", remover.otherRoleCalledWith)
	}
}

// TestSellerDeletedOutboxWorker_RoleRemoval_PassesBuyerAbsent verifies the
// worker passes OtherRoleAbsent when HasActiveBuyer confirms no buyer.
func TestSellerDeletedOutboxWorker_RoleRemoval_PassesBuyerAbsent(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}
	buyers := &fakeWorkerBuyerClient{hasActive: false}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, buyers, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(remover.otherRoleCalledWith) != 1 || remover.otherRoleCalledWith[0] != rolestate.OtherRoleAbsent {
		t.Fatalf("otherRoleCalledWith = %v, want [OtherRoleAbsent]", remover.otherRoleCalledWith)
	}
}

// TestSellerDeletedOutboxWorker_RoleRemoval_BuyerLookupFailsPassesUnknown verifies
// that a HasActiveBuyer error still fail-closes to OtherRoleUnknown (the
// invariant is unchanged by this fix — only the "no info available" default
// case moves from unconditional to query-failure-only).
func TestSellerDeletedOutboxWorker_RoleRemoval_BuyerLookupFailsPassesUnknown(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}
	buyers := &fakeWorkerBuyerClient{err: errors.New("timeout")}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, buyers, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(remover.otherRoleCalledWith) != 1 || remover.otherRoleCalledWith[0] != rolestate.OtherRoleUnknown {
		t.Fatalf("otherRoleCalledWith = %v, want [OtherRoleUnknown] (fail-closed)", remover.otherRoleCalledWith)
	}
}

// A seller.deleted row carries the enqueue-time surrogate snapshot into the event.
func TestSellerLifecycleOutbox_DeletedRow_CarriesSnapshottedSurrogate(t *testing.T) {
	row := newPendingSellerRow(3, 3003, 0)
	row.EventType = domain.SellerLifecycleEventDeleted
	row.Payload = []byte(`{"accountId":3003,"storeId":777}`)
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.DeletedEvents) != 1 || pub.DeletedEvents[0].StoreID != 777 {
		t.Fatalf("DeletedEvents = %+v, want one event with StoreID=777", pub.DeletedEvents)
	}
	if len(remover.calledWith) != 1 || remover.calledWith[0] != 3003 {
		t.Fatalf("expected RemoveSellerRole(3003) called once for explicit deleted row, got %v", remover.calledWith)
	}
}

func TestSellerLifecycleOutbox_LegacyRow_FallsBackToAccountID(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(4, 4004, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}

	w := NewSellerDeletedOutboxWorker(repo, pub, &fakeSellerRoleRemover{}, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.DeletedEvents) != 1 || pub.DeletedEvents[0].StoreID != 4004 {
		t.Fatalf("DeletedEvents = %+v, want StoreID fallback 4004", pub.DeletedEvents)
	}
}

// seller.suspended rows dispatch to PublishSellerSuspended (which binds
// ExchangeSeller + RoutingKeySellerSuspended) with the payload's suspensionId /
// accountId / reasonCode, and must NOT trigger RemoveSellerRole. The int64 TSID
// fields serialize as JSON strings (,string tag — TSID precision contract).
func TestSellerLifecycleOutbox_SuspendedRow_DispatchesSuspendedEvent(t *testing.T) {
	row := newPendingSellerRow(11, 900001, 0)
	row.EventType = domain.SellerLifecycleEventSuspended
	// issue #1534：outbox payload 的 reasonCode 已為 CodeMeta Value 數字碼。
	row.Payload = []byte(`{"accountId":900001,"suspensionId":7482084104842517848,"reasonCode":"9602","storeId":800002}`)
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.SuspendedEvents) != 1 {
		t.Fatalf("SuspendedEvents = %+v, want exactly one", pub.SuspendedEvents)
	}
	ev := pub.SuspendedEvents[0]
	if ev.SuspensionID != 7482084104842517848 || ev.AccountID != 900001 || ev.ReasonCode != "9602" {
		t.Fatalf("event = %+v, want {suspensionId=7482084104842517848, accountId=900001, reasonCode=9602}", ev)
	}
	// issue #1514：worker 附上 human-readable label 供 notification 模板引擎渲染
	// （模板引擎只做字面代入，無 lookup 能力）。
	if ev.ReasonLabel != "屢勸不改" {
		t.Fatalf("ReasonLabel = %q, want %q", ev.ReasonLabel, "屢勸不改")
	}
	if ev.StoreID != 800002 {
		t.Fatalf("StoreID = %d, want 800002 (snapshotted at suspend time)", ev.StoreID)
	}
	if ev.EventType != pkgmq.EventTypeSellerSuspended {
		t.Fatalf("EventType = %s, want %s", ev.EventType, pkgmq.EventTypeSellerSuspended)
	}
	// TSID precision contract: the wire JSON must carry both int64 ids as strings
	// (a generic-map consumer would truncate a bare int64 through float64).
	wire, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, want := range []string{`"suspensionId":"7482084104842517848"`, `"accountId":"900001"`, `"store_id":"800002"`} {
		if !strings.Contains(string(wire), want) {
			t.Fatalf("wire JSON = %s, missing %s", wire, want)
		}
	}
	if len(pub.DeletedEvents)+len(pub.OnboardedEvents)+len(pub.UnsuspendedEvents) != 0 {
		t.Fatalf("suspended row must not dispatch other event types: %+v %+v %+v",
			pub.DeletedEvents, pub.OnboardedEvents, pub.UnsuspendedEvents)
	}
	if len(remover.calledWith) != 0 {
		t.Fatalf("RemoveSellerRole must NOT run for suspended events, got %v", remover.calledWith)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 11 {
		t.Fatalf("MarkDoneCalls = %+v, want [11]", repo.MarkDoneCalls)
	}
}

// seller.unsuspended rows dispatch to PublishSellerUnsuspended (ExchangeSeller +
// RoutingKeySellerUnsuspended) with suspensionId/accountId; no reasonCode on the
// unsuspend wire, no role removal, and the same ,string TSID serialization.
func TestSellerLifecycleOutbox_UnsuspendedRow_DispatchesUnsuspendedEvent(t *testing.T) {
	row := newPendingSellerRow(12, 900001, 0)
	row.EventType = domain.SellerLifecycleEventUnsuspended
	row.Payload = []byte(`{"accountId":900001,"suspensionId":7482084104842517848,"storeId":800002}`)
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.UnsuspendedEvents) != 1 {
		t.Fatalf("UnsuspendedEvents = %+v, want exactly one", pub.UnsuspendedEvents)
	}
	ev := pub.UnsuspendedEvents[0]
	if ev.SuspensionID != 7482084104842517848 || ev.AccountID != 900001 {
		t.Fatalf("event = %+v, want {suspensionId=7482084104842517848, accountId=900001}", ev)
	}
	if ev.StoreID != 800002 {
		t.Fatalf("StoreID = %d, want 800002 (snapshotted at lift time)", ev.StoreID)
	}
	if ev.EventType != pkgmq.EventTypeSellerUnsuspended {
		t.Fatalf("EventType = %s, want %s", ev.EventType, pkgmq.EventTypeSellerUnsuspended)
	}
	wire, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, want := range []string{`"suspensionId":"7482084104842517848"`, `"accountId":"900001"`, `"store_id":"800002"`} {
		if !strings.Contains(string(wire), want) {
			t.Fatalf("wire JSON = %s, missing %s", wire, want)
		}
	}
	if len(pub.SuspendedEvents)+len(pub.DeletedEvents)+len(pub.OnboardedEvents) != 0 {
		t.Fatalf("unsuspended row must not dispatch other event types: %+v %+v %+v",
			pub.SuspendedEvents, pub.DeletedEvents, pub.OnboardedEvents)
	}
	if len(remover.calledWith) != 0 {
		t.Fatalf("RemoveSellerRole must NOT run for unsuspended events, got %v", remover.calledWith)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 12 {
		t.Fatalf("MarkDoneCalls = %+v, want [12]", repo.MarkDoneCalls)
	}
}

// seller.membership_changed rows dispatch to PublishSellerMembershipChanged
// (子系統 1) with the payload's storeId/planCode/tierRank/monthlyOrderCap, a
// UTC RFC3339 effectiveAt, and must NOT trigger RemoveSellerRole (that
// sub-task is deletion-only — 子系統 1 never matches the role-removal
// allowlist, which only fires for "" and seller.deleted).
func TestPublishSellerLifecycleRow_MembershipChanged_DispatchesEvent(t *testing.T) {
	row := newPendingSellerRow(13, 900001, 0)
	row.EventType = domain.SellerLifecycleEventMembershipChanged
	payload, err := json.Marshal(domain.SellerMembershipOutboxPayload{
		StoreID:         555,
		PlanCode:        "pro",
		TierRank:        2,
		MonthlyOrderCap: 1000,
		EffectiveAt:     time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	row.Payload = payload
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.MembershipChangedEvents) != 1 {
		t.Fatalf("MembershipChangedEvents = %+v, want exactly one", pub.MembershipChangedEvents)
	}
	ev := pub.MembershipChangedEvents[0]
	require.Equal(t, "555", ev.StoreID)
	require.Equal(t, "pro", ev.PlanCode)
	require.Equal(t, 2, ev.TierRank)
	require.Equal(t, 1000, ev.MonthlyOrderCap)
	require.Equal(t, "2026-08-29T00:00:00Z", ev.EffectiveAt)
	if ev.EventType != pkgmq.EventTypeSellerMembershipChanged {
		t.Fatalf("EventType = %s, want %s", ev.EventType, pkgmq.EventTypeSellerMembershipChanged)
	}
	if len(pub.SuspendedEvents)+len(pub.UnsuspendedEvents)+len(pub.DeletedEvents)+len(pub.OnboardedEvents) != 0 {
		t.Fatalf("membership_changed row must not dispatch other event types: %+v %+v %+v %+v",
			pub.SuspendedEvents, pub.UnsuspendedEvents, pub.DeletedEvents, pub.OnboardedEvents)
	}
	if len(remover.calledWith) != 0 {
		t.Fatalf("RemoveSellerRole must NOT run for membership_changed events, got %v", remover.calledWith)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 13 {
		t.Fatalf("MarkDoneCalls = %+v, want [13]", repo.MarkDoneCalls)
	}
}

// seller.onboarded rows publish the onboarded event and must NOT trigger
// RemoveSellerRole (that sub-task is deletion-only).
func TestSellerLifecycleOutbox_OnboardedRow_NoRoleRemoval(t *testing.T) {
	row := newPendingSellerRow(9, 9009, 0)
	row.EventType = domain.SellerLifecycleEventOnboarded
	row.Payload = []byte(`{"accountId":9009,"storeId":999}`)
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	pub := &fakeSellerDeletedPublisher{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, remover, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.OnboardedEvents) != 1 || pub.OnboardedEvents[0].StoreID != 999 || pub.OnboardedEvents[0].AccountID != 9009 {
		t.Fatalf("OnboardedEvents = %+v, want one event {9009, 999}", pub.OnboardedEvents)
	}
	if len(remover.calledWith) != 0 {
		t.Fatalf("RemoveSellerRole must NOT run for onboarded events, got %v", remover.calledWith)
	}
	if len(repo.MarkDoneCalls) != 1 || repo.MarkDoneCalls[0] != 9 {
		t.Fatalf("MarkDoneCalls = %+v, want [9]", repo.MarkDoneCalls)
	}
}
