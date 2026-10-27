package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

type mockInboxRepo struct {
	err                  error
	createdItem          *domain.InboxItem
	createAffected       int64
	listResult           []*domain.InboxItem
	countResult          int64
	unreadResult         int64
	markResult           int64
	markAllResult        int64
	deleteResult         int64
	gotCategory          string
	gotListLimit         int32
	gotRedactSenderIDs   []int64
	redactChatBySenderFn func(ctx context.Context, senderIDs []int64) (int64, error)

	gotSoftDeleteBuyerIDs   []int64
	softDeleteBuyerIDsCalls int
	softDeleteBuyerIDsFn    func(ctx context.Context, buyerIDs []int64) (int64, error)

	gotSoftDeleteStoreIDs   []int64
	softDeleteStoreIDsCalls int
	softDeleteStoreIDsFn    func(ctx context.Context, storeIDs []int64) (int64, error)
}

func (m *mockInboxRepo) Create(_ context.Context, item *domain.InboxItem) (int64, error) {
	m.createdItem = item
	return m.createAffected, m.err
}
func (m *mockInboxRepo) ListByUser(_ context.Context, _ int64, _ string, _ int64, limit int32, category string) ([]*domain.InboxItem, error) {
	m.gotListLimit = limit
	m.gotCategory = category
	return m.listResult, m.err
}
func (m *mockInboxRepo) CountByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return m.countResult, m.err
}
func (m *mockInboxRepo) CountUnread(_ context.Context, _ int64, _ string) (int64, error) {
	return m.unreadResult, m.err
}
func (m *mockInboxRepo) MarkAsRead(_ context.Context, _, _ int64, _ string) (int64, error) {
	return m.markResult, m.err
}
func (m *mockInboxRepo) MarkAllAsRead(_ context.Context, _ int64, _ string) (int64, error) {
	return m.markAllResult, m.err
}
func (m *mockInboxRepo) SoftDeleteByIDs(_ context.Context, _ []int64, _ int64, _ string) (int64, error) {
	return m.deleteResult, m.err
}

func (m *mockInboxRepo) RedactChatBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	m.gotRedactSenderIDs = senderIDs
	if m.redactChatBySenderFn != nil {
		return m.redactChatBySenderFn(ctx, senderIDs)
	}
	return m.deleteResult, m.err
}

func (m *mockInboxRepo) SoftDeleteByHistoricalBuyerIDs(ctx context.Context, buyerIDs []int64) (int64, error) {
	m.gotSoftDeleteBuyerIDs = buyerIDs
	m.softDeleteBuyerIDsCalls++
	if m.softDeleteBuyerIDsFn != nil {
		return m.softDeleteBuyerIDsFn(ctx, buyerIDs)
	}
	return m.deleteResult, m.err
}

func (m *mockInboxRepo) SoftDeleteByHistoricalStoreIDs(ctx context.Context, storeIDs []int64) (int64, error) {
	m.gotSoftDeleteStoreIDs = storeIDs
	m.softDeleteStoreIDsCalls++
	if m.softDeleteStoreIDsFn != nil {
		return m.softDeleteStoreIDsFn(ctx, storeIDs)
	}
	return m.deleteResult, m.err
}

func TestListNotifications_Success(t *testing.T) {
	repo := &mockInboxRepo{
		listResult:  []*domain.InboxItem{{ID: 1, Title: "test"}},
		countResult: 1,
	}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	result, err := uc.ListNotifications(context.Background(), 42, "buyer", 0, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if len(result.Records) != 1 {
		t.Errorf("len(Records) = %d, want 1", len(result.Records))
	}
	if result.HasMore {
		t.Error("HasMore = true, want false")
	}
}

func TestListNotifications_DefaultPageSize(t *testing.T) {
	repo := &mockInboxRepo{countResult: 0}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	result, err := uc.ListNotifications(context.Background(), 42, "buyer", 0, 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if repo.gotListLimit != DefaultInboxPageSize+1 {
		t.Errorf("ListByUser limit = %d, want %d", repo.gotListLimit, DefaultInboxPageSize+1)
	}
}

func TestListNotifications_ClampsPageSizeAtMax(t *testing.T) {
	repo := &mockInboxRepo{countResult: 0}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	result, err := uc.ListNotifications(context.Background(), 42, "buyer", 0, MaxInboxPageSize+1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if repo.gotListLimit != MaxInboxPageSize+1 {
		t.Errorf("ListByUser limit = %d, want %d", repo.gotListLimit, MaxInboxPageSize+1)
	}
}

func TestListNotifications_HasMore(t *testing.T) {
	// 模擬 repo 回傳 pageSize+1 筆（21 筆）
	items := make([]*domain.InboxItem, 21)
	for i := range items {
		items[i] = &domain.InboxItem{ID: int64(21 - i)}
	}
	repo := &mockInboxRepo{
		listResult:  items,
		countResult: 50,
	}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	result, err := uc.ListNotifications(context.Background(), 42, "buyer", 0, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasMore {
		t.Error("HasMore = false, want true")
	}
	if len(result.Records) != 20 {
		t.Errorf("len(Records) = %d, want 20", len(result.Records))
	}
}

func TestGetUnreadCount(t *testing.T) {
	repo := &mockInboxRepo{unreadResult: 5}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	count, err := uc.GetUnreadCount(context.Background(), 42, "buyer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestMarkAsRead_Idempotent(t *testing.T) {
	// affected=0 means already read → should still succeed
	repo := &mockInboxRepo{markResult: 0}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	err := uc.MarkAsRead(context.Background(), 1, 42, "buyer")
	if err != nil {
		t.Errorf("expected no error for idempotent mark, got %v", err)
	}
}

func TestMarkAllAsRead(t *testing.T) {
	repo := &mockInboxRepo{markAllResult: 12}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	updated, err := uc.MarkAllAsRead(context.Background(), 42, "buyer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 12 {
		t.Errorf("updated = %d, want 12", updated)
	}
}

func TestDeleteNotifications_EmptyIDs(t *testing.T) {
	repo := &mockInboxRepo{}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	_, err := uc.DeleteNotifications(context.Background(), nil, 42, "buyer")
	if err == nil {
		t.Error("expected error for empty ids")
	}
}

func TestDeleteNotifications_Success(t *testing.T) {
	repo := &mockInboxRepo{deleteResult: 2}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	deleted, err := uc.DeleteNotifications(context.Background(), []int64{1, 2, 99}, 42, "buyer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}
}

func TestCreateInboxItem_Success(t *testing.T) {
	repo := &mockInboxRepo{}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	item := &domain.InboxItem{
		UserID:      42,
		UserType:    "buyer",
		EventType:   "order.paid",
		ReferenceID: "ORD-001",
		Title:       "訂單成立",
		Body:        "您的訂單已成立",
	}
	if err := uc.CreateInboxItem(context.Background(), item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createdItem == nil {
		t.Fatal("expected createdItem to be set")
	}
	if repo.createdItem.UserID != 42 {
		t.Errorf("UserID = %d, want 42", repo.createdItem.UserID)
	}
	if repo.createdItem.EventType != "order.paid" {
		t.Errorf("EventType = %q, want order.paid", repo.createdItem.EventType)
	}
}

func TestCreateInboxItem_RepoError(t *testing.T) {
	repo := &mockInboxRepo{err: errors.New("db error")}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	item := &domain.InboxItem{UserID: 1, UserType: "buyer", EventType: "order.paid", Title: "T", Body: "B"}
	if err := uc.CreateInboxItem(context.Background(), item); err == nil {
		t.Error("expected error, got nil")
	}
}

type mockInboxPublisher struct {
	inboxNewCalls      int
	unreadChangedCalls int
}

func (m *mockInboxPublisher) PublishNotificationInboxNew(_ context.Context, _ int64, _ string, _ map[string]interface{}, _ int64) error {
	m.inboxNewCalls++
	return nil
}
func (m *mockInboxPublisher) PublishNotificationUnreadChanged(_ context.Context, _ int64, _ string, _ int64) error {
	m.unreadChangedCalls++
	return nil
}

// 新寫入一筆（INSERT 影響 1 列）→ 推一次即時 notification.inbox.new。
func TestCreateInboxItem_NewRowPublishesInboxNew(t *testing.T) {
	repo := &mockInboxRepo{createAffected: 1}
	pub := &mockInboxPublisher{}
	uc := NewNotificationInboxUseCase(repo, pub, nil, nil, nil)

	item := &domain.InboxItem{UserID: 42, UserType: "seller", EventType: "shipment.anomaly.detected.seller", ReferenceID: "O123|DELIVERY_OVERDUE", Title: "配送異常通知", Body: "..."}
	if err := uc.CreateInboxItem(context.Background(), item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.inboxNewCalls != 1 {
		t.Errorf("inboxNewCalls = %d, want 1", pub.inboxNewCalls)
	}
}

// 重複事件被 ON CONFLICT DO NOTHING 去重（INSERT 影響 0 列）→ 不得再推即時事件，
// 否則 scheduler 每輪重發會讓 web 端每隔幾分鐘跳一次重複通知 toast。
func TestCreateInboxItem_DedupedDoesNotPublishInboxNew(t *testing.T) {
	repo := &mockInboxRepo{createAffected: 0}
	pub := &mockInboxPublisher{}
	uc := NewNotificationInboxUseCase(repo, pub, nil, nil, nil)

	item := &domain.InboxItem{UserID: 42, UserType: "seller", EventType: "shipment.anomaly.detected.seller", ReferenceID: "O123|DELIVERY_OVERDUE", Title: "配送異常通知", Body: "..."}
	if err := uc.CreateInboxItem(context.Background(), item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.inboxNewCalls != 0 {
		t.Errorf("inboxNewCalls = %d, want 0 (deduped insert must not re-publish)", pub.inboxNewCalls)
	}
}

func TestListNotifications_CategoryFilter(t *testing.T) {
	repo := &mockInboxRepo{
		listResult:  []*domain.InboxItem{{ID: 1, Title: "test"}},
		countResult: 1,
	}
	uc := NewNotificationInboxUseCase(repo, nil, nil, nil, nil)

	result, err := uc.ListNotifications(context.Background(), 42, "buyer", 0, 20, "order")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if repo.gotCategory != "order" {
		t.Errorf("gotCategory = %q, want %q", repo.gotCategory, "order")
	}
}

// ==================== RedactChatNotificationsByAccount ====================

// mockBuyerClient/mockSellerClient are minimal usecase.BuyerClient/SellerClient
// fakes local to this test file (only ListIDsByAccount, matching the interfaces
// in client_interfaces.go).
type mockBuyerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (m *mockBuyerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if m.ListIDsByAccountFn != nil {
		return m.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

type mockSellerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (m *mockSellerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if m.ListIDsByAccountFn != nil {
		return m.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockSellerClient) EnsureInvite(context.Context, string, string, string) (string, string, string, error) {
	return "", "", "", nil
}

func (m *mockSellerClient) MarkInviteSecretDelivered(context.Context, string, string) error {
	return nil
}

// TestRedactChatNotificationsByAccount_MergesAccountAndBuyerID mirrors
// im-service's TestRedactDeletedAccount_EnumeratesAllHistoricalBuyerAndStoreIDs
// (services/im-service/internal/usecase/message_test.go) assertion style: the
// purge set is compared as a set (map), not by slice order, since
// mergeUniqueSenderIDs makes no ordering guarantee callers should depend on.
func TestRedactChatNotificationsByAccount_MergesAccountAndBuyerID(t *testing.T) {
	repo := &mockInboxRepo{deleteResult: 2}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 100 {
			t.Fatalf("buyerClient.ListIDsByAccount accountID = %d, want 100", accountID)
		}
		return []int64{42}, nil
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 100 {
			t.Fatalf("sellerClient.ListIDsByAccount accountID = %d, want 100", accountID)
		}
		return []int64{}, nil
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, sellerClient, nil)

	n, err := uc.RedactChatNotificationsByAccount(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("n = %d, want 2", n)
	}
	want := map[int64]bool{100: true, 42: true}
	if len(repo.gotRedactSenderIDs) != len(want) {
		t.Fatalf("RedactChatBySender senderIDs = %v, want exactly %v", repo.gotRedactSenderIDs, want)
	}
	for _, id := range repo.gotRedactSenderIDs {
		if !want[id] {
			t.Errorf("unexpected senderID %d in purge set", id)
		}
	}
}

// TestRedactChatNotificationsByAccount_DedupesDuplicateIDs mirrors im-service's
// mergeUniqueIDs test coverage: overlapping ids across accountID/buyer
// list/seller list must appear exactly once in the purge set.
func TestRedactChatNotificationsByAccount_DedupesDuplicateIDs(t *testing.T) {
	repo := &mockInboxRepo{}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		return []int64{accountID, 42}, nil // accountID itself re-appears as a buyer_id (current stint == accountID)
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{42, 999}, nil // 42 duplicated across buyer and seller lists
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, sellerClient, nil)

	if _, err := uc.RedactChatNotificationsByAccount(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int64]bool{100: true, 42: true, 999: true}
	if len(repo.gotRedactSenderIDs) != len(want) {
		t.Fatalf("RedactChatBySender senderIDs = %v, want exactly %v (deduplicated)", repo.gotRedactSenderIDs, want)
	}
	for _, id := range repo.gotRedactSenderIDs {
		if !want[id] {
			t.Errorf("unexpected senderID %d in purge set", id)
		}
	}
}

// TestRedactChatNotificationsByAccount_PropagatesBuyerClientError: fail-closed —
// an enumeration error must abort before the repo is ever called, matching
// im-service.RedactDeletedAccount's error-propagation behavior for the same step.
func TestRedactChatNotificationsByAccount_PropagatesBuyerClientError(t *testing.T) {
	repo := &mockInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) {
			t.Fatal("RedactChatBySender must not be called when buyer-client enumeration fails")
			return 0, nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, errors.New("buyer-service unavailable")
	}}
	sellerClient := &mockSellerClient{}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, sellerClient, nil)

	if _, err := uc.RedactChatNotificationsByAccount(context.Background(), 100); err == nil {
		t.Fatal("expected error when buyer-service enumeration fails, got nil")
	}
}

// TestRedactChatNotificationsByAccount_PropagatesSellerClientError mirrors the
// buyer-client case above for the seller-client enumeration step.
func TestRedactChatNotificationsByAccount_PropagatesSellerClientError(t *testing.T) {
	repo := &mockInboxRepo{
		redactChatBySenderFn: func(context.Context, []int64) (int64, error) {
			t.Fatal("RedactChatBySender must not be called when seller-client enumeration fails")
			return 0, nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{42}, nil
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, errors.New("seller-service unavailable")
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, sellerClient, nil)

	if _, err := uc.RedactChatNotificationsByAccount(context.Background(), 100); err == nil {
		t.Fatal("expected error when seller-service enumeration fails, got nil")
	}
}

// TestSoftDeleteRetiredBuyerStintInbox_UnionsRetiredIDWithEnumeration: the id
// the event carries must be in the purge set even when enumeration doesn't
// return it (the buyers row could already be gone), and enumeration must
// contribute the older stints the event knows nothing about.
func TestSoftDeleteRetiredBuyerStintInbox_UnionsRetiredIDWithEnumeration(t *testing.T) {
	repo := &mockInboxRepo{deleteResult: 7}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 100 {
			t.Fatalf("buyerClient.ListIDsByAccount accountID = %d, want 100", accountID)
		}
		return []int64{555}, nil // an older closed stint; 42 deliberately absent
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, &mockSellerClient{}, nil)

	n, err := uc.SoftDeleteRetiredBuyerStintInbox(context.Background(), 100, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("n = %d, want 7", n)
	}
	want := map[int64]bool{42: true, 555: true}
	if len(repo.gotSoftDeleteBuyerIDs) != len(want) {
		t.Fatalf("SoftDeleteByHistoricalBuyerIDs buyerIDs = %v, want exactly %v", repo.gotSoftDeleteBuyerIDs, want)
	}
	for _, id := range repo.gotSoftDeleteBuyerIDs {
		if !want[id] {
			t.Errorf("unexpected buyerID %d in purge set", id)
		}
	}
}

// TestSoftDeleteRetiredBuyerStintInbox_NeverSeedsAccountID is the reverse
// assertion that keeps this path inside the buyer identity space. account_id and
// buyer_id are separate spaces (44 §10 D6), so seeding the purge set with
// accountID — the way mergeUniqueSenderIDs does for chat senderId — would
// soft-delete the inbox of whichever buyer happens to hold that number.
func TestSoftDeleteRetiredBuyerStintInbox_NeverSeedsAccountID(t *testing.T) {
	repo := &mockInboxRepo{}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{555}, nil // enumeration does NOT contain accountID 100
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, &mockSellerClient{}, nil)

	if _, err := uc.SoftDeleteRetiredBuyerStintInbox(context.Background(), 100, 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range repo.gotSoftDeleteBuyerIDs {
		if id == 100 {
			t.Fatalf("purge set %v contains the account_id 100 — buyer_id and account_id are "+
				"different identity spaces; a buyer who happens to own that buyer_id would lose their inbox",
				repo.gotSoftDeleteBuyerIDs)
		}
	}
}

// TestSoftDeleteRetiredBuyerStintInbox_DedupesAndDropsZero: grandfathered
// accounts have buyer_id == account_id, so the event's BuyerID and the
// enumeration overlap constantly; a zero BuyerID (payload without the field)
// must not become a purge candidate matching rows whose user_id is 0.
func TestSoftDeleteRetiredBuyerStintInbox_DedupesAndDropsZero(t *testing.T) {
	repo := &mockInboxRepo{}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{42, 42, 0}, nil
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, &mockSellerClient{}, nil)

	if _, err := uc.SoftDeleteRetiredBuyerStintInbox(context.Background(), 100, 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.gotSoftDeleteBuyerIDs) != 1 || repo.gotSoftDeleteBuyerIDs[0] != 42 {
		t.Fatalf("buyerIDs = %v, want exactly [42] (deduplicated, zero dropped)", repo.gotSoftDeleteBuyerIDs)
	}
}

// TestSoftDeleteRetiredBuyerStintInbox_PropagatesEnumerationError: fail-closed,
// same contract as RedactChatNotificationsByAccount — under-purging silently is
// worse than a retry/DLQ.
func TestSoftDeleteRetiredBuyerStintInbox_PropagatesEnumerationError(t *testing.T) {
	repo := &mockInboxRepo{
		softDeleteBuyerIDsFn: func(context.Context, []int64) (int64, error) {
			t.Fatal("SoftDeleteByHistoricalBuyerIDs must not be called when enumeration fails")
			return 0, nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, errors.New("buyer-service unavailable")
	}}
	uc := NewNotificationInboxUseCase(repo, nil, buyerClient, &mockSellerClient{}, nil)

	if _, err := uc.SoftDeleteRetiredBuyerStintInbox(context.Background(), 100, 42); err == nil {
		t.Fatal("expected error when buyer-service enumeration fails, got nil")
	}
}

// TestSoftDeleteRetiredBuyerStintInbox_EmptySetSkipsRepo: an empty purge set
// would become `user_id = ANY('{}')`, matching nothing — but issuing it anyway
// costs a round trip and makes the row count meaningless.
func TestSoftDeleteRetiredBuyerStintInbox_EmptySetSkipsRepo(t *testing.T) {
	repo := &mockInboxRepo{}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, &mockSellerClient{}, nil)

	n, err := uc.SoftDeleteRetiredBuyerStintInbox(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if repo.softDeleteBuyerIDsCalls != 0 {
		t.Errorf("SoftDeleteByHistoricalBuyerIDs called %d times, want 0 for an empty purge set", repo.softDeleteBuyerIDsCalls)
	}
}

// ==================== SoftDeleteRetiredSellerIncarnationInbox ====================
//
// Mirrors the SoftDeleteRetiredBuyerStintInbox test block above, seller side
// (issue #2707): swap Buyer→Seller, buyer→seller, BuyerID→StoreID. accountID
// stays accountID throughout.

// TestSoftDeleteRetiredSellerIncarnationInbox_UnionsRetiredIDWithEnumeration:
// the id the event carries must be in the purge set even when enumeration
// doesn't return it (the seller row could already be gone), and enumeration
// must contribute the older incarnations the event knows nothing about.
func TestSoftDeleteRetiredSellerIncarnationInbox_UnionsRetiredIDWithEnumeration(t *testing.T) {
	repo := &mockInboxRepo{deleteResult: 7}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 100 {
			t.Fatalf("sellerClient.ListIDsByAccount accountID = %d, want 100", accountID)
		}
		return []int64{555}, nil // an older closed incarnation; 42 deliberately absent
	}}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, sellerClient, nil)

	n, err := uc.SoftDeleteRetiredSellerIncarnationInbox(context.Background(), 100, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("n = %d, want 7", n)
	}
	want := map[int64]bool{42: true, 555: true}
	if len(repo.gotSoftDeleteStoreIDs) != len(want) {
		t.Fatalf("SoftDeleteByHistoricalStoreIDs storeIDs = %v, want exactly %v", repo.gotSoftDeleteStoreIDs, want)
	}
	for _, id := range repo.gotSoftDeleteStoreIDs {
		if !want[id] {
			t.Errorf("unexpected storeID %d in purge set", id)
		}
	}
}

// TestSoftDeleteRetiredSellerIncarnationInbox_NeverSeedsAccountID is the
// reverse assertion that keeps this path inside the seller identity space.
// account_id and store_id are separate spaces, so seeding the purge set with
// accountID — the way mergeUniqueSenderIDs does for chat senderId — would
// soft-delete the inbox of whichever seller happens to hold that number.
func TestSoftDeleteRetiredSellerIncarnationInbox_NeverSeedsAccountID(t *testing.T) {
	repo := &mockInboxRepo{}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{555}, nil // enumeration does NOT contain accountID 100
	}}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, sellerClient, nil)

	if _, err := uc.SoftDeleteRetiredSellerIncarnationInbox(context.Background(), 100, 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range repo.gotSoftDeleteStoreIDs {
		if id == 100 {
			t.Fatalf("purge set %v contains the account_id 100 — store_id and account_id are "+
				"different identity spaces; a seller who happens to own that store_id would lose their inbox",
				repo.gotSoftDeleteStoreIDs)
		}
	}
}

// TestSoftDeleteRetiredSellerIncarnationInbox_DedupesAndDropsZero:
// grandfathered accounts have store_id == account_id, so the event's StoreID
// and the enumeration overlap constantly; a zero StoreID (payload without the
// field, e.g. a backfilled event) must not become a purge candidate matching
// rows whose user_id is 0.
func TestSoftDeleteRetiredSellerIncarnationInbox_DedupesAndDropsZero(t *testing.T) {
	repo := &mockInboxRepo{}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{42, 42, 0}, nil
	}}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, sellerClient, nil)

	if _, err := uc.SoftDeleteRetiredSellerIncarnationInbox(context.Background(), 100, 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.gotSoftDeleteStoreIDs) != 1 || repo.gotSoftDeleteStoreIDs[0] != 42 {
		t.Fatalf("storeIDs = %v, want exactly [42] (deduplicated, zero dropped)", repo.gotSoftDeleteStoreIDs)
	}
}

// TestSoftDeleteRetiredSellerIncarnationInbox_PropagatesEnumerationError:
// fail-closed, same contract as RedactChatNotificationsByAccount —
// under-purging silently is worse than a retry/DLQ.
func TestSoftDeleteRetiredSellerIncarnationInbox_PropagatesEnumerationError(t *testing.T) {
	repo := &mockInboxRepo{
		softDeleteStoreIDsFn: func(context.Context, []int64) (int64, error) {
			t.Fatal("SoftDeleteByHistoricalStoreIDs must not be called when enumeration fails")
			return 0, nil
		},
	}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, errors.New("seller-service unavailable")
	}}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, sellerClient, nil)

	if _, err := uc.SoftDeleteRetiredSellerIncarnationInbox(context.Background(), 100, 42); err == nil {
		t.Fatal("expected error when seller-service enumeration fails, got nil")
	}
}

// TestSoftDeleteRetiredSellerIncarnationInbox_EmptySetSkipsRepo: an empty
// purge set would become `user_id = ANY('{}')`, matching nothing — but
// issuing it anyway costs a round trip and makes the row count meaningless.
func TestSoftDeleteRetiredSellerIncarnationInbox_EmptySetSkipsRepo(t *testing.T) {
	repo := &mockInboxRepo{}
	uc := NewNotificationInboxUseCase(repo, nil, &mockBuyerClient{}, &mockSellerClient{}, nil)

	n, err := uc.SoftDeleteRetiredSellerIncarnationInbox(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if repo.softDeleteStoreIDsCalls != 0 {
		t.Errorf("SoftDeleteByHistoricalStoreIDs called %d times, want 0 for an empty purge set", repo.softDeleteStoreIDsCalls)
	}
}

// ==================== DeactivateRetiredSellerIncarnationDeviceTokens ====================
//
// Issue #2707 follow-up: closes residual risk (b) documented in
// 20260830100003_notification_announcement_identity_migrated_marker.sql —
// retired-incarnation device tokens on the same physical device were never
// deactivated, causing permanent duplicate push delivery.

// TestDeactivateRetiredSellerIncarnationDeviceTokens_CallsRepoWithSellerType:
// the repo call must use domain.UserTypeSeller (never a bare "buyer" or an
// unscoped call) and the exact retiredStoreID passed in — ownership/scope
// safety lives entirely in this one call's arguments, since there is no
// enumeration/union step to also verify.
func TestDeactivateRetiredSellerIncarnationDeviceTokens_CallsRepoWithSellerType(t *testing.T) {
	var gotUserID int64
	var gotUserType string
	repo := &mockTokenRepo{
		deactivateByUserFn: func(userID int64, userType string) (int64, error) {
			gotUserID = userID
			gotUserType = userType
			return 3, nil
		},
	}
	uc := NewNotificationInboxUseCase(&mockInboxRepo{}, nil, &mockBuyerClient{}, &mockSellerClient{}, repo)

	n, err := uc.DeactivateRetiredSellerIncarnationDeviceTokens(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("n = %d, want 3", n)
	}
	if gotUserID != 42 {
		t.Errorf("DeactivateByUser userID = %d, want 42", gotUserID)
	}
	if gotUserType != domain.UserTypeSeller {
		t.Errorf("DeactivateByUser userType = %q, want %q", gotUserType, domain.UserTypeSeller)
	}
}

// TestDeactivateRetiredSellerIncarnationDeviceTokens_ZeroStoreIDSkipsRepo:
// mirrors SoftDeleteRetiredSellerIncarnationInbox's empty-set skip above —
// a backfilled seller.deleted event always carries StoreID=0
// (seller-service/cmd/backfill-seller-deleted/main.go), and WHERE user_id = 0
// AND user_type = 'seller' has no ownership meaning, so it must not reach the
// repo at all.
func TestDeactivateRetiredSellerIncarnationDeviceTokens_ZeroStoreIDSkipsRepo(t *testing.T) {
	repo := &mockTokenRepo{
		deactivateByUserFn: func(int64, string) (int64, error) {
			t.Fatal("DeactivateByUser must not be called for a zero retiredStoreID")
			return 0, nil
		},
	}
	uc := NewNotificationInboxUseCase(&mockInboxRepo{}, nil, &mockBuyerClient{}, &mockSellerClient{}, repo)

	n, err := uc.DeactivateRetiredSellerIncarnationDeviceTokens(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

// TestDeactivateRetiredSellerIncarnationDeviceTokens_PropagatesRepoError:
// fail-closed, same contract as SoftDeleteRetiredSellerIncarnationInbox and
// RedactChatNotificationsByAccount — silently leaving the retired
// incarnation's tokens active is worse than a retry/DLQ.
func TestDeactivateRetiredSellerIncarnationDeviceTokens_PropagatesRepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &mockTokenRepo{
		deactivateByUserFn: func(int64, string) (int64, error) { return 0, dbErr },
	}
	uc := NewNotificationInboxUseCase(&mockInboxRepo{}, nil, &mockBuyerClient{}, &mockSellerClient{}, repo)

	if _, err := uc.DeactivateRetiredSellerIncarnationDeviceTokens(context.Background(), 42); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}
