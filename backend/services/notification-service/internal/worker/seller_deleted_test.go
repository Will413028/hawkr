package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

func validSellerDeletedEvent(accountID, storeID int64) []byte {
	b, _ := json.Marshal(mq.SellerDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeSellerDeleted, "seller-service"),
		AccountID: accountID,
		StoreID:   storeID,
	})
	return b
}

// mockDeviceTokenRepo is a domain.DeviceTokenRepository fake for the
// device-token deactivation step this fix round adds to seller_deleted.go's
// handler. Embeds the nil interface so any unexpected method call panics —
// only DeactivateByUser is exercised by this handler. deactivateByUserFn has
// deliberately no default, mirroring mockNotificationInboxRepo's
// "unimplemented methods blow up" contract above (account_deleted_test.go):
// a test that reaches this step without setting the fn fails loudly instead
// of silently returning a zero value.
type mockDeviceTokenRepo struct {
	domain.DeviceTokenRepository
	deactivateByUserFn func(ctx context.Context, userID int64, userType string) (int64, error)
}

func (m *mockDeviceTokenRepo) DeactivateByUser(ctx context.Context, userID int64, userType string) (int64, error) {
	return m.deactivateByUserFn(ctx, userID, userType)
}

func TestSellerDeletedSubscription_RoutingKey(t *testing.T) {
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{}, nil, nil, nil, nil)
	sub := sellerDeletedSubscription(uc, zap.NewNop())

	if sub.Exchange != mq.ExchangeSeller {
		t.Errorf("Exchange = %q, want %q", sub.Exchange, mq.ExchangeSeller)
	}
	if sub.Queue != mq.QueueNotificationSellerDeleted {
		t.Errorf("Queue = %q, want %q", sub.Queue, mq.QueueNotificationSellerDeleted)
	}
	if len(sub.RoutingKeys) != 1 || sub.RoutingKeys[0] != mq.RoutingKeySellerDeleted {
		t.Errorf("RoutingKeys = %v, want [%q]", sub.RoutingKeys, mq.RoutingKeySellerDeleted)
	}
}

func TestSellerDeletedHandler_Success(t *testing.T) {
	var gotAccountID int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			if len(senderIDs) > 0 {
				gotAccountID = senderIDs[0]
			}
			return 3, nil
		},
		softDeleteStoreIDsFn: func(context.Context, []int64) (int64, error) { return 0, nil },
	}
	tokenRepo := &mockDeviceTokenRepo{
		deactivateByUserFn: func(context.Context, int64, string) (int64, error) { return 0, nil },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, tokenRepo)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	// accountID=100 is passed; storeID=42 is the closed seller stint's surrogate.
	// Since this task, RedactChatNotificationsByAccount enumerates ALL historical
	// buyer_id/store_id surrogates for accountID itself — the handler now passes
	// accountID, not storeID, and enumeration (not the payload) is what recovers 42
	// and any other diverged surrogate. The fake buyer/seller clients return no
	// extra ids, so the merged purge set collapses to just [accountID].
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAccountID != 100 {
		t.Errorf("RedactChatNotificationsByAccount got accountID=%d, want 100 (payload.AccountID, not storeId 42)", gotAccountID)
	}
}

func TestSellerDeletedHandler_ZeroAccountID(t *testing.T) {
	noop := func(context.Context, []int64) (int64, error) { t.Fatal("should not be called"); return 0, nil }
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{redactChatBySenderFn: noop}, nil, nil, nil, nil)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	// accountId missing/zero must be rejected (indicates malformed event from seller-service)
	if err := handler(context.Background(), "", validSellerDeletedEvent(0, 42)); err == nil {
		t.Fatal("zero accountId should return err (framework retries → DLQ)")
	}
}

// TestSellerDeletedHandler_ZeroStoreID_Succeeds mirrors what
// backfill-seller-deleted actually publishes (services/seller-service/cmd/backfill-seller-deleted/main.go):
// AccountID set, StoreID always zero. The guard must key on AccountID, not
// StoreID, or every backfilled event dead-letters. Also proves the device-token
// deactivation step (this fix round) is equally zero-StoreID-tolerant: a
// backfilled event must not reach DeactivateByUser with a zero user_id either.
func TestSellerDeletedHandler_ZeroStoreID_Succeeds(t *testing.T) {
	var gotAccountID int64
	var gotStoreIDs []int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			if len(senderIDs) > 0 {
				gotAccountID = senderIDs[0]
			}
			return 1, nil
		},
		softDeleteStoreIDsFn: func(_ context.Context, storeIDs []int64) (int64, error) {
			gotStoreIDs = storeIDs
			return 0, nil
		},
	}
	tokenRepo := &mockDeviceTokenRepo{
		deactivateByUserFn: func(context.Context, int64, string) (int64, error) {
			t.Fatal("DeactivateByUser should not be called for a zero StoreID")
			return 0, nil
		},
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, tokenRepo)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 0)); err != nil {
		t.Fatalf("unexpected error for zero storeId with non-zero accountId: %v", err)
	}
	if gotAccountID != 100 {
		t.Errorf("RedactChatNotificationsByAccount got accountID=%d, want 100", gotAccountID)
	}
	// A zero StoreID (backfill's shape) must not become a soft-delete call at
	// all: the fake seller client returns no historical ids either, so the
	// merged purge set is empty and SoftDeleteRetiredSellerIncarnationInbox
	// returns early without calling the repo (mirrors
	// TestSoftDeleteRetiredBuyerStintInbox_EmptySetSkipsRepo at the usecase
	// layer) — softDeleteStoreIDsFn being nil-safe here would hide that.
	if gotStoreIDs != nil {
		t.Errorf("SoftDeleteByHistoricalStoreIDs got storeIDs=%v, want not called (empty purge set)", gotStoreIDs)
	}
	// tokenRepo.deactivateByUserFn's t.Fatal above is the assertion for the
	// device-token step: reaching it at all is the failure.
}

func TestSellerDeletedHandler_RepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestSellerDeletedHandler_SoftDeletesRetiredIncarnationInbox mirrors
// TestBuyerDeletedHandler_SoftDeletesRetiredStintInbox (account_deleted_test.go):
// the redaction above only covers rows where the closed incarnation was the
// *sender* of a chat notification (other people's inboxes). The rows addressed
// TO that incarnation had no terminal state at all — unreadable under the new
// store_id yet deleted_at IS NULL — until this soft-delete step.
func TestSellerDeletedHandler_SoftDeletesRetiredIncarnationInbox(t *testing.T) {
	var gotStoreIDs []int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteStoreIDsFn: func(_ context.Context, storeIDs []int64) (int64, error) {
			gotStoreIDs = storeIDs
			return 5, nil
		},
	}
	tokenRepo := &mockDeviceTokenRepo{
		deactivateByUserFn: func(context.Context, int64, string) (int64, error) { return 0, nil },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, tokenRepo)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The fake seller client returns no historical ids, so the purge set is just
	// the incarnation the event closed — and notably NOT accountID 100, which
	// lives in a different identity space (see the usecase-level
	// TestSoftDeleteRetiredSellerIncarnationInbox_NeverSeedsAccountID).
	if len(gotStoreIDs) != 1 || gotStoreIDs[0] != 42 {
		t.Errorf("SoftDeleteByHistoricalStoreIDs got storeIDs=%v, want [42] (payload.StoreID)", gotStoreIDs)
	}
}

// TestSellerDeletedHandler_SoftDeleteErrorRetries: fail-closed. Swallowing this
// error would ack the message and leave the retired incarnation's rows live
// forever with no second chance and no signal.
func TestSellerDeletedHandler_SoftDeleteErrorRetries(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteStoreIDsFn: func(context.Context, []int64) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestSellerDeletedHandler_DeactivatesRetiredIncarnationDeviceTokens is the
// third step this fix round adds: proves the handler calls DeactivateByUser
// with the closed incarnation's own store_id and user_type="seller" — the
// real, permanent duplicate-push cause documented as an open residual risk in
// 20260830100003_notification_announcement_identity_migrated_marker.sql (b).
func TestSellerDeletedHandler_DeactivatesRetiredIncarnationDeviceTokens(t *testing.T) {
	var gotUserID int64
	var gotUserType string
	var callCount int
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteStoreIDsFn: func(context.Context, []int64) (int64, error) { return 0, nil },
	}
	tokenRepo := &mockDeviceTokenRepo{
		deactivateByUserFn: func(_ context.Context, userID int64, userType string) (int64, error) {
			callCount++
			gotUserID = userID
			gotUserType = userType
			return 2, nil
		},
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, tokenRepo)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("DeactivateByUser call count = %d, want 1", callCount)
	}
	if gotUserID != 42 {
		t.Errorf("DeactivateByUser userID = %d, want 42 (payload.StoreID, not accountID 100)", gotUserID)
	}
	if gotUserType != domain.UserTypeSeller {
		t.Errorf("DeactivateByUser userType = %q, want %q", gotUserType, domain.UserTypeSeller)
	}
}

// TestSellerDeletedHandler_DeviceTokenDeactivationErrorRetries: fail-closed,
// same contract as the other two steps in this handler. Swallowing this error
// would leave a retired incarnation's device tokens is_active=true forever,
// silently keeping the duplicate-push bug alive with no second chance and no
// signal.
func TestSellerDeletedHandler_DeviceTokenDeactivationErrorRetries(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteStoreIDsFn: func(context.Context, []int64) (int64, error) { return 0, nil },
	}
	tokenRepo := &mockDeviceTokenRepo{
		deactivateByUserFn: func(context.Context, int64, string) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, tokenRepo)
	handler := newSellerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}
