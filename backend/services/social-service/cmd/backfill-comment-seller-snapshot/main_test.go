//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestLoadSellerSnapshotMapping_ResolvesByEitherAccountIDOrStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, logo, status) VALUES (8100, 1001, 100, true, '文忠商店', 'sellers/8100/logo.png', 'active')`)

	mapping, err := loadSellerSnapshotMapping(ctx, pool, []int64{100, 8100})
	require.NoError(t, err)

	if len(mapping) != 2 {
		t.Fatalf("len(mapping) = %d, want 2 (both the account_id and store_id keys resolve); mapping = %+v", len(mapping), mapping)
	}
	for _, id := range []int64{100, 8100} {
		snap, ok := mapping[id]
		if !ok {
			t.Fatalf("mapping[%d] missing", id)
		}
		if snap.UserName != "文忠商店" || snap.UserAvatar != "sellers/8100/logo.png" {
			t.Errorf("mapping[%d] = %+v, want {文忠商店 sellers/8100/logo.png}", id, snap)
		}
	}
}

// TestLoadSellerSnapshotMapping_OldStoreIDResolvesToCurrentStint proves a
// user_id holding a stale, CLOSED store_id (a comment written during an
// earlier stint, before the seller reopened) still resolves to that
// account's CURRENT store's name/logo — matching what CreateComment would
// look up today if asked, not a frozen historical snapshot of the closed
// store.
func TestLoadSellerSnapshotMapping_OldStoreIDResolvesToCurrentStint(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, logo, status) VALUES (8200, 1002, 200, false, '舊店（已關）', 'old-logo.png', 'closed')`)
	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, logo, status) VALUES (8201, 1003, 200, true, '新店（重開）', 'new-logo.png', 'active')`)

	mapping, err := loadSellerSnapshotMapping(ctx, pool, []int64{8200})
	require.NoError(t, err)

	snap, ok := mapping[8200]
	if !ok {
		t.Fatal("mapping[8200] missing")
	}
	if snap.UserName != "新店（重開）" || snap.UserAvatar != "new-logo.png" {
		t.Errorf("mapping[8200] = %+v, want the CURRENT stint {新店（重開） new-logo.png}, not the closed one", snap)
	}
}

func TestLoadSellerSnapshotMapping_GrandfatheredAccountIDEqualsStoreID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, logo, status) VALUES (300, 1004, 300, true, '元老商店', 'grandfather.png', 'active')`)

	mapping, err := loadSellerSnapshotMapping(ctx, pool, []int64{300})
	require.NoError(t, err)

	snap, ok := mapping[300]
	if !ok {
		t.Fatal("mapping[300] missing")
	}
	if snap.UserName != "元老商店" {
		t.Errorf("mapping[300].UserName = %q, want 元老商店", snap.UserName)
	}
}

// TestLoadSellerSnapshotMapping_NoLogoRoundTripsAsEmptyString: a seller who
// never uploaded a logo (logo column NULL) must resolve with UserAvatar ==
// "" — BackfillCommentSnapshots (not this function) is the layer
// responsible for turning that back into SQL NULL on write.
func TestLoadSellerSnapshotMapping_NoLogoRoundTripsAsEmptyString(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8400, 1005, 400, true, '無頭貼商店', 'active')`)

	mapping, err := loadSellerSnapshotMapping(ctx, pool, []int64{400})
	require.NoError(t, err)

	snap, ok := mapping[400]
	if !ok {
		t.Fatal("mapping[400] missing")
	}
	if snap.UserAvatar != "" {
		t.Errorf("mapping[400].UserAvatar = %q, want \"\" (no logo uploaded)", snap.UserAvatar)
	}
}

// TestLoadSellerSnapshotMapping_UnknownIDIsAbsent: an id matching no
// sellers row at all (deleted account, bogus value) must be absent from the
// result — not an error, not a zero-value entry — so the caller's
// "len(mapping) vs len(userIDs)" unresolved count stays accurate.
func TestLoadSellerSnapshotMapping_UnknownIDIsAbsent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8100, 1001, 100, true, '文忠商店', 'active')`)

	mapping, err := loadSellerSnapshotMapping(ctx, pool, []int64{100, 999})
	require.NoError(t, err)

	if len(mapping) != 1 {
		t.Fatalf("len(mapping) = %d, want 1; mapping = %+v", len(mapping), mapping)
	}
	if _, ok := mapping[999]; ok {
		t.Error("mapping contains unknown id 999, want absent")
	}
}

// TestLoadSellerSnapshotMapping_EmptyInput: no ids to resolve must not
// error and must not query anything meaningful.
func TestLoadSellerSnapshotMapping_EmptyInput(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mapping, err := loadSellerSnapshotMapping(ctx, pool, nil)
	require.NoError(t, err)
	if len(mapping) != 0 {
		t.Errorf("len(mapping) = %d, want 0", len(mapping))
	}
}

func mustExecSeller(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(ctx, sql)
	require.NoError(t, err, "exec: %s", sql)
}
