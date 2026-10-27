package worker

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/stretchr/testify/require"
)

// Role sync's two halves have opposite time semantics (issue #3189). These
// tests are written so that the pre-fix implementation fails them:
//
//   - the old removal passed no stint at all, so there was nothing for
//     account-service to fence on;
//   - the old worker had no grant sub-step, so "store but no role" had no way
//     back.
//
// A test that only asserted "deletion removes the role" would pass before and
// after the fix. Each case below is built around a supersession instead.

func lifecycleRow(t *testing.T, id, accountID int64, eventType string, storeID int64) *domain.SellerDeletedOutbox {
	t.Helper()
	row, err := domain.NewLifecycleOutboxRow(id, accountID, eventType,
		domain.SellerLifecyclePayload{AccountID: accountID, StoreID: storeID}, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	return row
}

// The removal must name the stint whose closure authorised it — the snapshot in
// the row, not whatever is current. That is the whole input to the fence: a
// removal that named "the seller role, whatever justifies it now" would strip
// the role the account regained by re-onboarding.
func TestRoleSync_DeletionNamesTheStintItClosed(t *testing.T) {
	const accountID, closedStint int64 = 5001, 7001
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{
			lifecycleRow(t, 1, accountID, domain.SellerLifecycleEventDeleted, closedStint),
		},
	}
	remover := &fakeSellerRoleRemover{}
	// The account has ALREADY re-onboarded onto a newer stint by the time this
	// row drains — the live-reproduced timeline from the issue.
	current := &fakeCurrentStint{seller: &domain.Seller{AccountID: accountID, StoreID: 7002}}

	w := NewSellerDeletedOutboxWorker(repo, &fakeSellerDeletedPublisher{}, remover,
		&fakeSellerRoleGranter{}, current, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	require.NoError(t, w.RunOnce(context.Background()))

	require.Equal(t, []int64{closedStint}, remover.storeIDCalledWith,
		"the removal must carry the stint it closed, so account-service can reject it as stale")
}

// Legacy rows carry no payload and therefore no attribution. They must resolve
// to 0 (unattributed), NOT to the account id — asserting that an account id is
// a stint id would let the fence match, or fail to match, by coincidence.
func TestRoleSync_LegacyDeletionRowIsUnattributed(t *testing.T) {
	const accountID int64 = 5002
	row := lifecycleRow(t, 2, accountID, domain.SellerLifecycleEventDeleted, 0)
	row.EventType = "" // pre-generalization row
	row.Payload = nil
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{row}}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, &fakeSellerDeletedPublisher{}, remover,
		&fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	require.NoError(t, w.RunOnce(context.Background()))

	require.Equal(t, []int64{0}, remover.storeIDCalledWith)
}

// The grant is level-triggered: it reads the account's current stint at apply
// time and IGNORES the surrogate snapshotted into the row. A retry that lands
// after a delete + re-onboard must attribute the role to the stint that exists
// now — attributing it to the row's retired stint would re-arm exactly the
// revocation the fence is there to reject.
func TestRoleSync_OnboardGrantUsesTheCurrentStintNotThePayload(t *testing.T) {
	const accountID, staleStint, liveStint int64 = 5003, 8001, 8002
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{
			lifecycleRow(t, 3, accountID, domain.SellerLifecycleEventOnboarded, staleStint),
		},
	}
	granter := &fakeSellerRoleGranter{}
	current := &fakeCurrentStint{seller: &domain.Seller{AccountID: accountID, StoreID: liveStint}}

	w := NewSellerDeletedOutboxWorker(repo, &fakeSellerDeletedPublisher{}, &fakeSellerRoleRemover{},
		granter, current, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	require.NoError(t, w.RunOnce(context.Background()))

	require.Equal(t, []int64{accountID}, granter.calledWith, "the onboarding row must drive a grant")
	require.Equal(t, []int64{liveStint}, granter.storeIDs,
		"the grant must name the stint that is current at apply time, not the row's snapshot")
	require.Equal(t, 1, current.calls, "the lookup is the level-triggered read; skipping it is the bug")
}

// No live stint means the account has no storefront right now — the deletion
// path owns that state. Granting anyway would manufacture a role nothing
// justifies, and would fight the very revocation that removed it.
func TestRoleSync_OnboardGrantSkippedWhenNoLiveStint(t *testing.T) {
	const accountID int64 = 5004
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{
			lifecycleRow(t, 4, accountID, domain.SellerLifecycleEventOnboarded, 8101),
		},
	}
	granter := &fakeSellerRoleGranter{}

	w := NewSellerDeletedOutboxWorker(repo, &fakeSellerDeletedPublisher{}, &fakeSellerRoleRemover{},
		granter, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	require.NoError(t, w.RunOnce(context.Background()))

	require.Empty(t, granter.calledWith, "no live stint ⇒ nothing justifies the role ⇒ no grant")
	require.Equal(t, []int64{4}, repo.MarkDoneCalls, "and the row still converges rather than retrying forever")
}

// Suspension and membership rows must not touch roles in either direction.
// Suspension is not role loss (the /me write routes stay behind requireSeller
// and surface suspension separately), so a role write here would be a silent
// authorization change riding on an unrelated event.
func TestRoleSync_SuspensionAndMembershipRowsTouchNoRole(t *testing.T) {
	const accountID int64 = 5005
	suspended, err := domain.NewLifecycleOutboxRow(5, accountID, domain.SellerLifecycleEventSuspended,
		domain.SellerSuspensionOutboxPayload{SuspensionID: 1, AccountID: accountID, StoreID: 9001, ReasonCode: "OTHER"},
		time.Now().Add(-time.Minute))
	require.NoError(t, err)
	repo := &fakeSellerDeletedOutboxRepo{pending: []*domain.SellerDeletedOutbox{suspended}}
	granter := &fakeSellerRoleGranter{}
	remover := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, &fakeSellerDeletedPublisher{}, remover,
		granter, &fakeCurrentStint{seller: &domain.Seller{AccountID: accountID, StoreID: 9001}},
		&fakeWorkerBuyerClient{}, testOutboxMetrics)
	require.NoError(t, w.RunOnce(context.Background()))

	require.Empty(t, granter.calledWith)
	require.Empty(t, remover.calledWith)
}
