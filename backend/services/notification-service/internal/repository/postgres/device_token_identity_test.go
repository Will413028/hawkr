//go:build integration

// 需要本機 Docker daemon（testcontainers 起 postgres 重放 db/migrations）。
// getTestPool 與 CI 接線見 dbtest_test.go 檔頭。
//
// 這支測試證明 DeactivateByUser（issue #2707 follow-up：
// internal/worker/seller_deleted.go 的第三步）的 SQL scoping 本身正確，鏡射
// notification_inbox_identity_test.go 的 ownership-safety 風格——走 real
// Postgres，不用 repo fake：`UPDATE ... WHERE user_id = $1 AND user_type = $2`
// 這種述詞正確性最容易在改寫時不小心壞掉、又最容易被 mock 掩蓋過去。
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func insertTestDeviceToken(t *testing.T, pool *pgxpool.Pool, id, userID int64, userType, token string, isActive bool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO device_tokens (id, user_id, user_type, token, platform, is_active)
		VALUES ($1, $2, $3, $4, 'ios', $5)
	`, id, userID, userType, token, isActive)
	require.NoError(t, err, "insert test device_tokens row id=%d", id)
}

func getDeviceTokenIsActive(t *testing.T, pool *pgxpool.Pool, id int64) bool {
	t.Helper()
	var isActive bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT is_active FROM device_tokens WHERE id = $1`, id).Scan(&isActive))
	return isActive
}

// TestDeviceTokenRepository_DeactivateByUser_OwnershipSafety proves the four
// quadrants that matter for issue #2707's retired-incarnation fix:
//
//  1. A token under the retired store_id (user_type='seller') IS deactivated.
//  2. A token under a DIFFERENT, unrelated seller's store_id is untouched.
//  3. A BUYER row that happens to share the exact same numeric id as the
//     retired store_id is untouched — user_id alone is not ownership,
//     (user_id, user_type) is (44 號設計書 §10, notifications.sql's own
//     user_type='seller' guard on SoftDeleteByHistoricalStoreIDs).
//  4. A second, already-inactive token under the retired store_id also ends
//     up is_active=false (idempotent — this is a blanket per-user
//     deactivation, not a single-token operation), and the returned row count
//     reflects exactly the rows actually flipped.
func TestDeviceTokenRepository_DeactivateByUser_OwnershipSafety(t *testing.T) {
	pool := getTestPool(t)
	repo := NewDeviceTokenRepository(pool, func() int64 { return time.Now().UnixNano() })
	ctx := context.Background()

	const retiredStoreID = int64(92001)     // the incarnation that just closed
	const otherStoreID = int64(92002)       // a different, unrelated seller
	const sameNumericBuyer = retiredStoreID // a buyer whose buyer_id happens to equal retiredStoreID

	targetActive := time.Now().UnixNano()
	insertTestDeviceToken(t, pool, targetActive, retiredStoreID, "seller", "tok-target-active", true)
	targetAlreadyInactive := targetActive + 1
	insertTestDeviceToken(t, pool, targetAlreadyInactive, retiredStoreID, "seller", "tok-target-already-inactive", false)
	otherSellerTok := targetActive + 2
	insertTestDeviceToken(t, pool, otherSellerTok, otherStoreID, "seller", "tok-other-seller", true)
	buyerTok := targetActive + 3
	insertTestDeviceToken(t, pool, buyerTok, sameNumericBuyer, "buyer", "tok-same-numeric-buyer", true)

	n, err := repo.DeactivateByUser(ctx, retiredStoreID, "seller")
	require.NoError(t, err)
	// Only the one still-active row under the retired store_id actually flips —
	// the already-inactive row is a no-op UPDATE, so RowsAffected still counts
	// it (Postgres counts matched rows, not changed rows), giving 2.
	require.Equal(t, int64(2), n, "DeactivateByUser must report exactly the two rows it matched under (retiredStoreID, seller)")

	require.False(t, getDeviceTokenIsActive(t, pool, targetActive),
		"device token under the retired store_id must be deactivated")
	require.False(t, getDeviceTokenIsActive(t, pool, targetAlreadyInactive),
		"already-inactive device token under the retired store_id must remain deactivated")

	require.True(t, getDeviceTokenIsActive(t, pool, otherSellerTok),
		"a DIFFERENT seller's device token must be untouched")
	require.True(t, getDeviceTokenIsActive(t, pool, buyerTok),
		"a buyer row sharing the same numeric id as the retired store_id must be untouched — user_type separates the two identity spaces")
}
