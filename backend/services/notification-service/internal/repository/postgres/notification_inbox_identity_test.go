//go:build integration

// Canonical identity ownership tests for notifications.user_id.
//
// These tests deliberately use re-keyed IDs and a buyer/seller numeric collision.
// Grandfathered accounts where account_id == buyer_id == store_id would let a
// wrong identity implementation pass unnoticed.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

func insertTestTemplate(t *testing.T, pool *pgxpool.Pool, eventType, userType, targetField string, identityMigrated bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO notification_templates (
			exchange, routing_key, event_type, target_user_type, target_user_field,
			category, title_template, body_template, data_type, identity_migrated
		) VALUES (
			'test.exchange', 'test.routing_key', $1, $2, $3,
			'order_update', 'title', 'body', 'TEST', $4
		)
	`, eventType, userType, targetField, identityMigrated)
	require.NoError(t, err, "insert test template for event_type=%s", eventType)
}

func mkIdentityItem(userID int64, userType, eventType, ref string) *domain.InboxItem {
	return &domain.InboxItem{
		UserID:      userID,
		UserType:    userType,
		EventType:   eventType,
		ReferenceID: ref,
		Title:       "測試通知",
		Body:        "測試內容",
	}
}

func TestNotificationInboxRepository_CanonicalIdentitySupportsFullOwnershipFlow(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	const sellerEvent = "test.canonical.seller"
	const buyerEvent = "test.canonical.buyer"
	insertTestTemplate(t, pool, sellerEvent, "seller", "store_id", true)
	insertTestTemplate(t, pool, buyerEvent, "buyer", "buyer_id", false)

	const storeID = int64(91001)
	const accountID = int64(91002) // deliberately different from the store_id
	const buyerID = int64(92001)

	sellerFirst := mkIdentityItem(storeID, "seller", sellerEvent, "CANONICAL-1")
	sellerSecond := mkIdentityItem(storeID, "seller", sellerEvent, "CANONICAL-2")
	buyerItem := mkIdentityItem(buyerID, "buyer", buyerEvent, "CANONICAL-BUYER-1")
	for _, item := range []*domain.InboxItem{sellerFirst, sellerSecond, buyerItem} {
		_, err := repo.Create(ctx, item)
		require.NoError(t, err)
		require.NotZero(t, item.ID)
	}

	found, err := repo.ListByUser(ctx, storeID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, found, 2)

	legacyLookup, err := repo.ListByUser(ctx, accountID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Empty(t, legacyLookup, "seller inbox must not expose account_id compatibility rows")

	count, err := repo.CountByUser(ctx, storeID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	unread, err := repo.CountUnread(ctx, storeID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(2), unread)

	changed, err := repo.MarkAsRead(ctx, sellerFirst.ID, accountID, "seller")
	require.NoError(t, err)
	require.Zero(t, changed, "wrong account_id must not pass ownership")
	changed, err = repo.MarkAsRead(ctx, sellerFirst.ID, storeID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(1), changed)

	changed, err = repo.MarkAllAsRead(ctx, storeID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(1), changed, "MarkAllAsRead should update only the remaining unread canonical row")
	unread, err = repo.CountUnread(ctx, storeID, "seller")
	require.NoError(t, err)
	require.Zero(t, unread)

	deleted, err := repo.SoftDeleteByIDs(ctx, []int64{sellerFirst.ID}, accountID, "seller")
	require.NoError(t, err)
	require.Zero(t, deleted, "wrong account_id must not pass delete ownership")
	deleted, err = repo.SoftDeleteByIDs(ctx, []int64{sellerFirst.ID}, storeID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	buyerFound, err := repo.ListByUser(ctx, buyerID, "buyer", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, buyerFound, 1)
	buyerLegacyLookup, err := repo.ListByUser(ctx, accountID, "buyer", 0, 10, "")
	require.NoError(t, err)
	require.Empty(t, buyerLegacyLookup, "buyer inbox must use buyer_id, not account_id")
}

func TestNotificationInboxRepository_BuyerAndSellerNumericCollisionRemainIsolated(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	const sellerEvent = "test.collision.seller"
	const buyerEvent = "test.collision.buyer"
	insertTestTemplate(t, pool, sellerEvent, "seller", "store_id", true)
	insertTestTemplate(t, pool, buyerEvent, "buyer", "buyer_id", false)

	const sharedNumericID = int64(93001)
	sellerItem := mkIdentityItem(sharedNumericID, "seller", sellerEvent, "COLLISION-SELLER")
	buyerItem := mkIdentityItem(sharedNumericID, "buyer", buyerEvent, "COLLISION-BUYER")
	for _, item := range []*domain.InboxItem{sellerItem, buyerItem} {
		_, err := repo.Create(ctx, item)
		require.NoError(t, err)
	}

	sellerFound, err := repo.ListByUser(ctx, sharedNumericID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, sellerFound, 1)
	require.Equal(t, sellerItem.ID, sellerFound[0].ID)

	buyerFound, err := repo.ListByUser(ctx, sharedNumericID, "buyer", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, buyerFound, 1)
	require.Equal(t, buyerItem.ID, buyerFound[0].ID)

	deleted, err := repo.SoftDeleteByIDs(ctx, []int64{sellerItem.ID}, sharedNumericID, "buyer")
	require.NoError(t, err)
	require.Zero(t, deleted, "buyer ownership must not delete a seller row with the same numeric ID")
	deleted, err = repo.SoftDeleteByIDs(ctx, []int64{sellerItem.ID}, sharedNumericID, "seller")
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	sellerFound, err = repo.ListByUser(ctx, sharedNumericID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Empty(t, sellerFound)
	buyerFound, err = repo.ListByUser(ctx, sharedNumericID, "buyer", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, buyerFound, 1, "deleting seller notifications must not affect buyer notifications")
}

func TestNotificationTemplate_SellerAccountIDFallbackIsRejected(t *testing.T) {
	pool := getTestPool(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO notification_templates (
			exchange, routing_key, event_type, target_user_type, target_user_field,
			category, title_template, body_template, data_type, identity_migrated
		) VALUES (
			'test.exchange', 'test.routing_key', 'test.seller.account_fallback', 'seller',
			'seller_account_id', 'order_update', 'title', 'body', 'TEST', false
		)
	`)
	require.Error(t, err, "seller account_id template must be rejected after canonical cutover")
}

func TestNotificationInboxRepository_AnnouncementMarkerUsesCanonicalStoreID(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	var identityMigrated bool
	err := pool.QueryRow(ctx, `
		SELECT identity_migrated FROM notification_templates
		WHERE event_type = 'notification.announcement' AND target_user_type = 'seller'
	`).Scan(&identityMigrated)
	require.NoError(t, err, "announcement marker row must exist")
	require.True(t, identityMigrated)

	const storeID = int64(94001)
	const accountID = int64(94002)
	item := mkIdentityItem(storeID, "seller", "notification.announcement", "announcement-1")
	_, err = repo.Create(ctx, item)
	require.NoError(t, err)

	found, err := repo.ListByUser(ctx, storeID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, item.ID, found[0].ID)

	notFound, err := repo.ListByUser(ctx, accountID, "seller", 0, 10, "")
	require.NoError(t, err)
	require.Empty(t, notFound, "direct-write announcement rows must not fall back to account_id")
}
