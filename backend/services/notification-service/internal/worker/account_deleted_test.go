package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// mockNotificationInboxRepo embeds the full repo interface — unimplemented
// methods panic (nil embedded interface call), which is what we want: if the
// handler calls something unexpected, the test fails immediately. Mirrors
// social-service's account_deleted_test.go mock pattern.
type mockNotificationInboxRepo struct {
	domain.NotificationInboxRepository
	redactChatBySenderFn func(ctx context.Context, senderIDs []int64) (int64, error)
	// Deliberately no default: a nil fn panics, keeping the "unimplemented
	// methods blow up" contract above. The account.deleted tests below never set
	// it, which is itself the assertion that only the buyer.deleted path
	// soft-deletes a retired buyer stint's inbox.
	softDeleteBuyerIDsFn func(ctx context.Context, buyerIDs []int64) (int64, error)
	// Same "deliberately no default" contract as softDeleteBuyerIDsFn above, for
	// the seller.deleted path's retired-incarnation soft-delete.
	softDeleteStoreIDsFn func(ctx context.Context, storeIDs []int64) (int64, error)
}

func (m *mockNotificationInboxRepo) RedactChatBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	return m.redactChatBySenderFn(ctx, senderIDs)
}

func (m *mockNotificationInboxRepo) SoftDeleteByHistoricalBuyerIDs(ctx context.Context, buyerIDs []int64) (int64, error) {
	return m.softDeleteBuyerIDsFn(ctx, buyerIDs)
}

func (m *mockNotificationInboxRepo) SoftDeleteByHistoricalStoreIDs(ctx context.Context, storeIDs []int64) (int64, error) {
	return m.softDeleteStoreIDsFn(ctx, storeIDs)
}

// fakeBuyerClient/fakeSellerClient are minimal usecase.BuyerClient/SellerClient
// fakes for these worker-level tests. ListIDsByAccountFn defaults to returning
// nil (no extra historical stints), so RedactChatNotificationsByAccount's merge
// collapses to just accountID — these worker-level tests only need enumeration
// to not error and to confirm which id the handler passed in, not to exercise
// multi-stint enumeration/merge itself (that's covered at the usecase layer by
// internal/usecase/notification_inbox_test.go's
// TestRedactChatNotificationsByAccount_* tests). Mirrors im-service's
// fakeBuyerClient/fakeSellerClient (internal/worker/buyer_deleted_test.go).
type fakeBuyerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (f *fakeBuyerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if f.ListIDsByAccountFn != nil {
		return f.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

type fakeSellerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (f *fakeSellerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if f.ListIDsByAccountFn != nil {
		return f.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

func (f *fakeSellerClient) EnsureInvite(context.Context, string, string, string) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakeSellerClient) MarkInviteSecretDelivered(context.Context, string, string) error {
	return nil
}

func validBuyerDeletedEvent(accountID, buyerID int64) []byte {
	b, _ := json.Marshal(mq.BuyerDeletedEvent{
		BaseEvent: mq.BaseEvent{EventType: mq.EventTypeBuyerDeleted},
		AccountID: accountID,
		BuyerID:   buyerID,
	})
	return b
}

func TestBuyerDeletedSubscription_RoutingKey(t *testing.T) {
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{}, nil, nil, nil, nil)
	sub := buyerDeletedSubscription(uc, zap.NewNop())

	if sub.Exchange != mq.ExchangeBuyer {
		t.Errorf("Exchange = %q, want %q", sub.Exchange, mq.ExchangeBuyer)
	}
	if sub.Queue != mq.QueueNotificationBuyerDeleted {
		t.Errorf("Queue = %q, want %q", sub.Queue, mq.QueueNotificationBuyerDeleted)
	}
	if len(sub.RoutingKeys) != 1 || sub.RoutingKeys[0] != mq.RoutingKeyBuyerDeleted {
		t.Errorf("RoutingKeys = %v, want [%q]", sub.RoutingKeys, mq.RoutingKeyBuyerDeleted)
	}
}

func TestBuyerDeletedHandler_Success(t *testing.T) {
	var gotSenderIDs []int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			gotSenderIDs = senderIDs
			return 3, nil
		},
		softDeleteBuyerIDsFn: func(context.Context, []int64) (int64, error) { return 0, nil },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	// accountID=100 survives (buyer role closed, account may keep a seller role);
	// buyerID=42 is the closed stint's surrogate. Since D8/this task,
	// RedactChatNotificationsByAccount enumerates ALL historical buyer_id/store_id
	// surrogates for accountID itself — the handler now passes accountID, not
	// buyerID, and enumeration (not the payload) is what recovers 42 and any
	// other diverged surrogate. The fake buyer/seller clients here return no
	// extra ids, so the merged purge set collapses to just [accountID].
	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSenderIDs) != 1 || gotSenderIDs[0] != 100 {
		t.Errorf("RedactChatBySender got senderIDs=%v, want [100] (payload.AccountID, not buyerId 42)", gotSenderIDs)
	}
}

func TestBuyerDeletedHandler_MalformedJSON(t *testing.T) {
	noop := func(context.Context, []int64) (int64, error) { t.Fatal("should not be called"); return 0, nil }
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{redactChatBySenderFn: noop}, nil, nil, nil, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", []byte("not json")); err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ)")
	}
}

func TestBuyerDeletedHandler_ZeroAccountID(t *testing.T) {
	noop := func(context.Context, []int64) (int64, error) { t.Fatal("should not be called"); return 0, nil }
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{redactChatBySenderFn: noop}, nil, nil, nil, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	// buyerId present (42) but accountId missing/zero must still be rejected —
	// redaction now keys on accountId (enumeration's seed), not buyerId.
	if err := handler(context.Background(), "", validBuyerDeletedEvent(0, 42)); err == nil {
		t.Fatal("zero accountId should return err (framework retries → DLQ)")
	}
}

func TestBuyerDeletedHandler_RepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// validAccountDeletedEvent builds an account.deleted wire body matching the
// real publisher's format (mq.AccountDeletedEvent). Built as a map, not the
// struct, so this test doesn't take on a hidden compile-time assumption about
// the payload shape beyond what the handler itself decodes.
func validAccountDeletedEvent(accountID int64) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"event_type": string(mq.EventTypeAccountDeleted),
		"accountId":  accountID,
	})
	return b
}

func TestAccountDeletedSubscription_RoutingKey(t *testing.T) {
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{}, nil, nil, nil, nil)
	sub := accountDeletedSubscription(uc, zap.NewNop())

	if sub.Exchange != mq.ExchangeAccount {
		t.Errorf("Exchange = %q, want %q", sub.Exchange, mq.ExchangeAccount)
	}
	if sub.Queue != mq.QueueNotificationAccountDeleted {
		t.Errorf("Queue = %q, want %q", sub.Queue, mq.QueueNotificationAccountDeleted)
	}
	if len(sub.RoutingKeys) != 1 || sub.RoutingKeys[0] != mq.RoutingKeyAccountDeleted {
		t.Errorf("RoutingKeys = %v, want [%q]", sub.RoutingKeys, mq.RoutingKeyAccountDeleted)
	}
}

func TestAccountDeletedHandler_Success(t *testing.T) {
	var gotSenderIDs []int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			gotSenderIDs = senderIDs
			return 3, nil
		},
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newAccountDeletedHandler(uc, zap.NewNop())

	// accountID=100 is a seller-only account (no buyer stint) being deleted —
	// redaction must key on THIS, not a buyer id, since account.deleted's
	// payload never carries one. The fake buyer/seller clients return no extra
	// ids, so the merged purge set collapses to just [accountID].
	if err := handler(context.Background(), "", validAccountDeletedEvent(100)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSenderIDs) != 1 || gotSenderIDs[0] != 100 {
		t.Errorf("RedactChatBySender got senderIDs=%v, want [100] (payload.AccountID)", gotSenderIDs)
	}
}

func TestAccountDeletedHandler_MalformedJSON(t *testing.T) {
	noop := func(context.Context, []int64) (int64, error) { t.Fatal("should not be called"); return 0, nil }
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{redactChatBySenderFn: noop}, nil, nil, nil, nil)
	handler := newAccountDeletedHandler(uc, zap.NewNop())

	err := handler(context.Background(), "account.deleted", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from mq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
}

func TestAccountDeletedHandler_ZeroAccountID(t *testing.T) {
	noop := func(context.Context, []int64) (int64, error) { t.Fatal("should not be called"); return 0, nil }
	uc := usecase.NewNotificationInboxUseCase(&mockNotificationInboxRepo{redactChatBySenderFn: noop}, nil, nil, nil, nil)
	handler := newAccountDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validAccountDeletedEvent(0)); err == nil {
		t.Fatal("zero accountId should return err (framework retries → DLQ)")
	}
}

func TestAccountDeletedHandler_RepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newAccountDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validAccountDeletedEvent(100)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestAccountDeletedHandler_EventTypeTagMatchesRealWire guards against the
// wire-tag bug this task fixes: accountDeletedPayload tagged EventType as
// json:"eventType", but the real publisher (account-service's
// delete_account.go / remove_role.go, via mq.NewBaseEvent) emits event_type
// on the wire (pkg/mq/events.go's BaseEvent, line 84). Decoding real traffic
// through the old tag silently zeroes EventType — latent today only because
// nothing downstream reads it.
func TestAccountDeletedHandler_EventTypeTagMatchesRealWire(t *testing.T) {
	wire, err := json.Marshal(mq.AccountDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeAccountDeleted, "account-service"),
		AccountID: 777,
	})
	if err != nil {
		t.Fatalf("marshal real wire event: %v", err)
	}

	var payload mq.AccountDeletedEvent
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.EventType != mq.EventTypeAccountDeleted {
		t.Errorf("EventType = %q, want %q (payload's tag must match the real wire key event_type, not eventType)", payload.EventType, mq.EventTypeAccountDeleted)
	}
	if payload.AccountID != 777 {
		t.Errorf("AccountID = %d, want 777", payload.AccountID)
	}
}

// TestBuyerDeletedHandler_SoftDeletesRetiredStintInbox: the redaction above the
// new call only covers rows where the closed stint was the *sender* of a chat
// notification (i.e. rows in other people's inboxes). The rows addressed TO that
// stint had no terminal state at all — unreadable under the new identity
// (44 號設計書 §10 D6 / §7.6 第 6 步) yet deleted_at IS NULL, so cleanup.go's
// 90-day sweep never saw them and they stayed forever, order numbers and all.
func TestBuyerDeletedHandler_SoftDeletesRetiredStintInbox(t *testing.T) {
	var gotBuyerIDs []int64
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteBuyerIDsFn: func(_ context.Context, buyerIDs []int64) (int64, error) {
			gotBuyerIDs = buyerIDs
			return 5, nil
		},
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The fake buyer client returns no historical ids, so the purge set is just
	// the stint the event closed — and notably NOT accountID 100, which lives in
	// a different identity space (see the usecase-level
	// TestSoftDeleteRetiredBuyerStintInbox_NeverSeedsAccountID).
	if len(gotBuyerIDs) != 1 || gotBuyerIDs[0] != 42 {
		t.Errorf("SoftDeleteByHistoricalBuyerIDs got buyerIDs=%v, want [42] (payload.BuyerID)", gotBuyerIDs)
	}
}

// TestBuyerDeletedHandler_SoftDeleteErrorRetries: fail-closed. Swallowing this
// error would ack the message and leave the retired stint's rows live forever
// with no second chance and no signal.
func TestBuyerDeletedHandler_SoftDeleteErrorRetries(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteBuyerIDsFn: func(context.Context, []int64) (int64, error) { return 0, dbErr },
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newBuyerDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validBuyerDeletedEvent(100, 42)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestAccountDeletedHandler_DoesNotSoftDeleteInbox is the reverse assertion.
// account.deleted carries no buyer surrogate at all, so the only id it could
// pass is the account_id — a value in the *account* space that would match
// whichever buyer_id happens to equal it. Every buyer stint closure already
// emits its own buyer.deleted (buyer-service's delete_buyer_account.go and
// complete_pending_deletion.go both enqueue the outbox row), so this path has
// nothing left to purge and no safe id to purge it with.
func TestAccountDeletedHandler_DoesNotSoftDeleteInbox(t *testing.T) {
	repo := &mockNotificationInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) { return 0, nil },
		softDeleteBuyerIDsFn: func(_ context.Context, buyerIDs []int64) (int64, error) {
			t.Fatalf("account.deleted must not soft-delete inbox rows (got buyerIDs=%v) —"+
				" it has no buyer surrogate, only an account_id from a different identity space", buyerIDs)
			return 0, nil
		},
	}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, &fakeBuyerClient{}, &fakeSellerClient{}, nil)
	handler := newAccountDeletedHandler(uc, zap.NewNop())

	if err := handler(context.Background(), "", validAccountDeletedEvent(100)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
