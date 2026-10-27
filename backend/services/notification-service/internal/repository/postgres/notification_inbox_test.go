//go:build integration

// 需要本機 Docker daemon（testcontainers 起 postgres 重放 db/migrations）。
// getTestPool 與 CI 接線見 dbtest_test.go 檔頭。
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// TestNotificationInboxRepository_RedactChatBySender_BatchMatchesFullIDSet proves
// RedactChatNotificationsBySender's `data->>'senderId' = ANY($1::text[])` batch
// match (db/queries/notifications.sql) actually redacts every row whose
// senderId is in the caller's id set, and leaves rows outside that set alone.
// This is the one behavior in the whole chat-redaction-seller-deleted-gap plan
// that was previously proven only by compiling — every other test in this repo
// exercises RedactChatBySender through a repository fake.
func TestNotificationInboxRepository_RedactChatBySender_BatchMatchesFullIDSet(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	mkItem := func(senderID string) *domain.InboxItem {
		return &domain.InboxItem{
			UserID:    900,
			UserType:  "buyer",
			EventType: "chat.message.sent",
			Title:     "sender-" + senderID,
			Body:      "hello from " + senderID,
			Data:      map[string]string{"senderId": senderID},
		}
	}

	item100 := mkItem("100")
	item200 := mkItem("200")
	item300 := mkItem("300")
	for _, item := range []*domain.InboxItem{item100, item200, item300} {
		if _, err := repo.Create(ctx, item); err != nil {
			t.Fatalf("create inbox item senderId=%s: %v", item.Data["senderId"], err)
		}
		if item.ID == 0 {
			t.Fatalf("expected ID assigned after insert for senderId=%s", item.Data["senderId"])
		}
	}

	n, err := repo.RedactChatBySender(ctx, []int64{100, 200})
	if err != nil {
		t.Fatalf("RedactChatBySender: %v", err)
	}
	if n != 2 {
		t.Errorf("RedactChatBySender rows affected = %d, want 2 (senderId 100 and 200)", n)
	}

	assertRedacted := func(id int64, wantRedacted bool) {
		t.Helper()
		var title, body string
		var deletedAt *time.Time
		row := pool.QueryRow(ctx, `SELECT title, body, deleted_at FROM notifications WHERE id = $1`, id)
		if err := row.Scan(&title, &body, &deletedAt); err != nil {
			t.Fatalf("query notification id=%d: %v", id, err)
		}
		if wantRedacted {
			if title != "" || body != "" {
				t.Errorf("id=%d: title=%q body=%q, want both blank after redaction", id, title, body)
			}
			if deletedAt == nil {
				t.Errorf("id=%d: deleted_at is nil, want set after redaction", id)
			}
		} else {
			if title == "" || body == "" {
				t.Errorf("id=%d: title=%q body=%q, want both non-blank (untouched)", id, title, body)
			}
			if deletedAt != nil {
				t.Errorf("id=%d: deleted_at = %v, want nil (untouched)", id, *deletedAt)
			}
		}
	}

	assertRedacted(item100.ID, true)
	assertRedacted(item200.ID, true)
	assertRedacted(item300.ID, false)
}

// TestNotificationInboxRepository_SoftDeleteByHistoricalBuyerIDs_OnlyBuyerRows
// proves the two halves of SoftDeleteByHistoricalBuyerIDs that only a real
// Postgres can show: the retired stints' rows get a deleted_at (so the existing
// 90-day cleanup job finally has something to collect), and the `user_type =
// 'buyer'` pin keeps the same numeric id in the seller space untouched.
//
// The seller row is the one that matters. First-generation accounts are
// grandfathered so that buyer_id == store_id == account_id, which means a
// dual-role account's still-active seller inbox sits on exactly the same
// user_id as the buyer stint being retired. Drop the user_type predicate and
// every test that only looks at buyer rows still passes, while the seller
// silently loses their whole inbox.
func TestNotificationInboxRepository_SoftDeleteByHistoricalBuyerIDs_OnlyBuyerRows(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	mkItem := func(userID int64, userType, ref string) *domain.InboxItem {
		return &domain.InboxItem{
			UserID:      userID,
			UserType:    userType,
			EventType:   "order.paid",
			ReferenceID: ref,
			Title:       "訂單成立",
			Body:        "您的訂單已成立",
		}
	}

	const retiredStint = int64(700) // the stint the buyer.deleted event closed
	const olderStint = int64(701)   // an earlier closed stint of the same account
	const otherBuyer = int64(702)   // an unrelated buyer — must not be touched
	retired := mkItem(retiredStint, "buyer", "R-1")
	older := mkItem(olderStint, "buyer", "R-2")
	other := mkItem(otherBuyer, "buyer", "R-3")
	// Same number as the retired buyer stint, but in the seller identity space:
	// a dual-role grandfathered account whose seller role is still live.
	sellerSameID := mkItem(retiredStint, "seller", "R-4")

	for _, item := range []*domain.InboxItem{retired, older, other, sellerSameID} {
		if _, err := repo.Create(ctx, item); err != nil {
			t.Fatalf("create inbox item ref=%s: %v", item.ReferenceID, err)
		}
	}

	n, err := repo.SoftDeleteByHistoricalBuyerIDs(ctx, []int64{retiredStint, olderStint})
	if err != nil {
		t.Fatalf("SoftDeleteByHistoricalBuyerIDs: %v", err)
	}
	if n != 2 {
		t.Errorf("rows affected = %d, want 2 (the two retired buyer stints only)", n)
	}

	assertSoftDeleted := func(item *domain.InboxItem, want bool) {
		t.Helper()
		var deletedAt *time.Time
		row := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, item.ID)
		if err := row.Scan(&deletedAt); err != nil {
			t.Fatalf("query notification ref=%s: %v", item.ReferenceID, err)
		}
		if want && deletedAt == nil {
			t.Errorf("ref=%s (user_id=%d user_type=%s): deleted_at is nil, want set",
				item.ReferenceID, item.UserID, item.UserType)
		}
		if !want && deletedAt != nil {
			t.Errorf("ref=%s (user_id=%d user_type=%s): deleted_at = %v, want nil (untouched)",
				item.ReferenceID, item.UserID, item.UserType, *deletedAt)
		}
	}

	assertSoftDeleted(retired, true)
	assertSoftDeleted(older, true)
	assertSoftDeleted(other, false)
	assertSoftDeleted(sellerSameID, false)

	// Idempotent: a redelivered buyer.deleted must not re-stamp deleted_at, or
	// the 90-day cleanup clock would restart on every retry.
	var before time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, retired.ID).Scan(&before); err != nil {
		t.Fatalf("read deleted_at: %v", err)
	}
	n, err = repo.SoftDeleteByHistoricalBuyerIDs(ctx, []int64{retiredStint, olderStint})
	if err != nil {
		t.Fatalf("SoftDeleteByHistoricalBuyerIDs (replay): %v", err)
	}
	if n != 0 {
		t.Errorf("replay rows affected = %d, want 0 (deleted_at IS NULL guard)", n)
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, retired.ID).Scan(&after); err != nil {
		t.Fatalf("re-read deleted_at: %v", err)
	}
	if !after.Equal(before) {
		t.Errorf("deleted_at moved from %v to %v on replay — the 90-day purge clock restarted", before, after)
	}
}

// TestNotificationInboxRepository_SoftDeleteByHistoricalStoreIDs_OnlySellerRows
// mirrors TestNotificationInboxRepository_SoftDeleteByHistoricalBuyerIDs_OnlyBuyerRows
// above for the seller side (issue #2707): proves the two halves of
// SoftDeleteByHistoricalStoreIDs that only a real Postgres can show — the
// retired incarnations' rows get a deleted_at (so the existing 90-day cleanup
// job finally has something to collect), and the `user_type = 'seller'` pin
// keeps the same numeric id in the buyer space untouched.
//
// The buyer row is the one that matters. First-generation accounts are
// grandfathered so that buyer_id == store_id == account_id, which means a
// dual-role account's still-active buyer inbox sits on exactly the same
// user_id as the seller incarnation being retired. Drop the user_type
// predicate and every test that only looks at seller rows still passes, while
// the buyer silently loses their whole inbox.
func TestNotificationInboxRepository_SoftDeleteByHistoricalStoreIDs_OnlySellerRows(t *testing.T) {
	pool := getTestPool(t)
	repo := NewNotificationInboxRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	mkItem := func(userID int64, userType, ref string) *domain.InboxItem {
		return &domain.InboxItem{
			UserID:      userID,
			UserType:    userType,
			EventType:   "order.paid",
			ReferenceID: ref,
			Title:       "訂單成立",
			Body:        "您的訂單已成立",
		}
	}

	const retiredIncarnation = int64(800) // the incarnation the seller.deleted event closed
	const olderIncarnation = int64(801)   // an earlier closed incarnation of the same account
	const otherSeller = int64(802)        // an unrelated seller — must not be touched
	retired := mkItem(retiredIncarnation, "seller", "S-1")
	older := mkItem(olderIncarnation, "seller", "S-2")
	other := mkItem(otherSeller, "seller", "S-3")
	// Same number as the retired seller incarnation, but in the buyer identity
	// space: a dual-role grandfathered account whose buyer role is still live.
	buyerSameID := mkItem(retiredIncarnation, "buyer", "S-4")

	for _, item := range []*domain.InboxItem{retired, older, other, buyerSameID} {
		if _, err := repo.Create(ctx, item); err != nil {
			t.Fatalf("create inbox item ref=%s: %v", item.ReferenceID, err)
		}
	}

	n, err := repo.SoftDeleteByHistoricalStoreIDs(ctx, []int64{retiredIncarnation, olderIncarnation})
	if err != nil {
		t.Fatalf("SoftDeleteByHistoricalStoreIDs: %v", err)
	}
	if n != 2 {
		t.Errorf("rows affected = %d, want 2 (the two retired seller incarnations only)", n)
	}

	assertSoftDeleted := func(item *domain.InboxItem, want bool) {
		t.Helper()
		var deletedAt *time.Time
		row := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, item.ID)
		if err := row.Scan(&deletedAt); err != nil {
			t.Fatalf("query notification ref=%s: %v", item.ReferenceID, err)
		}
		if want && deletedAt == nil {
			t.Errorf("ref=%s (user_id=%d user_type=%s): deleted_at is nil, want set",
				item.ReferenceID, item.UserID, item.UserType)
		}
		if !want && deletedAt != nil {
			t.Errorf("ref=%s (user_id=%d user_type=%s): deleted_at = %v, want nil (untouched)",
				item.ReferenceID, item.UserID, item.UserType, *deletedAt)
		}
	}

	assertSoftDeleted(retired, true)
	assertSoftDeleted(older, true)
	assertSoftDeleted(other, false)
	assertSoftDeleted(buyerSameID, false)

	// Idempotent: a redelivered seller.deleted must not re-stamp deleted_at, or
	// the 90-day cleanup clock would restart on every retry.
	var before time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, retired.ID).Scan(&before); err != nil {
		t.Fatalf("read deleted_at: %v", err)
	}
	n, err = repo.SoftDeleteByHistoricalStoreIDs(ctx, []int64{retiredIncarnation, olderIncarnation})
	if err != nil {
		t.Fatalf("SoftDeleteByHistoricalStoreIDs (replay): %v", err)
	}
	if n != 0 {
		t.Errorf("replay rows affected = %d, want 0 (deleted_at IS NULL guard)", n)
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT deleted_at FROM notifications WHERE id = $1`, retired.ID).Scan(&after); err != nil {
		t.Fatalf("re-read deleted_at: %v", err)
	}
	if !after.Equal(before) {
		t.Errorf("deleted_at moved from %v to %v on replay — the 90-day purge clock restarted", before, after)
	}
}
