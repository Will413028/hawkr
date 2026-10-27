//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

const roleSeller int16 = 2 // domain.RoleSeller — repository package doesn't import usecase/domain, mirror the raw value

// TestListBrokenSellerCommentUserIDs_FindsOnlyNullSnapshotSellerRows proves
// the detection query's WHERE clause: only seller-role, non-deleted rows
// with a NULL user_name or user_avatar count as "broken" — a buyer comment
// with a NULL snapshot (a different, unrelated code path) must not surface,
// nor a seller row that already has both fields populated, nor a soft-
// deleted broken row.
func TestListBrokenSellerCommentUserIDs_FindsOnlyNullSnapshotSellerRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)

	// Broken: seller comment, both fields NULL.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)
	// Broken: seller reply, only avatar NULL.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role, user_name) VALUES (5002, 1, 101, 2, 'has name')`)
	// NOT broken: seller comment already fully populated.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role, user_name, user_avatar) VALUES (5003, 1, 102, 2, 'ok', 'ok.png')`)
	// NOT broken (different bug class): buyer comment with NULL snapshot —
	// buyers resolve via a completely different client (GetBuyerProfile),
	// out of scope for this backfill.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5004, 1, 200, 1)`)
	// NOT broken: soft-deleted, would otherwise match.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role, deleted_at) VALUES (5005, 1, 103, 2, NOW())`)
	// Same broken seller (100) commenting twice — must be deduped to one id.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5006, 1, 100, 2)`)

	ids, err := postgres.ListBrokenSellerCommentUserIDs(ctx, pool, roleSeller)
	require.NoError(t, err)

	got := map[int64]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if len(got) != 2 {
		t.Fatalf("ListBrokenSellerCommentUserIDs = %v, want exactly {100, 101}", ids)
	}
	if !got[100] || !got[101] {
		t.Errorf("ListBrokenSellerCommentUserIDs = %v, want {100, 101}", ids)
	}
}

// TestBackfillCommentSnapshots_WritesResolvedNameAndAvatar proves the write
// side: every still-broken row for a resolved user_id gets both columns
// filled, update_time bumped, and rows for OTHER user_ids (resolved or not)
// are left alone.
func TestBackfillCommentSnapshots_WritesResolvedNameAndAvatar(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	// Two broken rows for the same seller (100) — both must get fixed by a
	// single mapping entry.
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5002, 1, 100, 2)`)
	// A different broken seller (200) NOT included in mapping — must stay
	// untouched (simulates "failed to resolve, skipped").
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5003, 1, 200, 2)`)

	mapping := map[int64]postgres.BrokenCommentSnapshot{
		100: {UserName: "文忠商店", UserAvatar: "sellers/100/logo.png"},
	}
	affected, err := postgres.BackfillCommentSnapshots(ctx, pool, roleSeller, mapping)
	require.NoError(t, err)
	if affected != 2 {
		t.Errorf("affected = %d, want 2", affected)
	}

	assertTextColumn(t, ctx, pool, `SELECT user_name FROM cms_comment WHERE id = 5001`, "文忠商店")
	assertTextColumn(t, ctx, pool, `SELECT user_avatar FROM cms_comment WHERE id = 5001`, "sellers/100/logo.png")
	assertTextColumn(t, ctx, pool, `SELECT user_name FROM cms_comment WHERE id = 5002`, "文忠商店")

	// Seller 200 was omitted from mapping (unresolved) — stays NULL, not
	// touched, not errored on.
	assertColumnIsNull(t, ctx, pool, `SELECT user_name FROM cms_comment WHERE id = 5003`)
}

// TestBackfillCommentSnapshots_EmptyAvatarWritesNullNotEmptyString proves a
// seller who resolved successfully but genuinely has no logo uploaded yet
// (AvatarKey == "") round-trips as SQL NULL, matching CreateComment's own
// pgtype.Text{Valid: x != ""} convention — writing ” instead would make the
// row indistinguishable from "still broken" today, yet permanently immune to
// a future retry (a ” avatar no longer matches this backfill's own
// `user_avatar IS NULL` detection query).
func TestBackfillCommentSnapshots_EmptyAvatarWritesNullNotEmptyString(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)

	mapping := map[int64]postgres.BrokenCommentSnapshot{
		100: {UserName: "無頭貼商店", UserAvatar: ""},
	}
	affected, err := postgres.BackfillCommentSnapshots(ctx, pool, roleSeller, mapping)
	require.NoError(t, err)
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}

	assertTextColumn(t, ctx, pool, `SELECT user_name FROM cms_comment WHERE id = 5001`, "無頭貼商店")
	assertColumnIsNull(t, ctx, pool, `SELECT user_avatar FROM cms_comment WHERE id = 5001`)
}

// TestBackfillCommentSnapshots_Idempotent: re-running with the same mapping
// after a successful pass matches zero rows (both columns already non-NULL).
func TestBackfillCommentSnapshots_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)

	mapping := map[int64]postgres.BrokenCommentSnapshot{100: {UserName: "商店", UserAvatar: "a.png"}}

	first, err := postgres.BackfillCommentSnapshots(ctx, pool, roleSeller, mapping)
	require.NoError(t, err)
	if first != 1 {
		t.Fatalf("first run affected = %d, want 1", first)
	}

	second, err := postgres.BackfillCommentSnapshots(ctx, pool, roleSeller, mapping)
	require.NoError(t, err)
	if second != 0 {
		t.Errorf("second run affected = %d, want 0 (idempotent re-run)", second)
	}
}

// TestBackfillCommentSnapshots_EmptyMapping: nothing to backfill must not
// error and must not touch any row.
func TestBackfillCommentSnapshots_EmptyMapping(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)

	affected, err := postgres.BackfillCommentSnapshots(ctx, pool, roleSeller, map[int64]postgres.BrokenCommentSnapshot{})
	require.NoError(t, err)
	if affected != 0 {
		t.Errorf("affected = %d, want 0", affected)
	}
	assertColumnIsNull(t, ctx, pool, `SELECT user_name FROM cms_comment WHERE id = 5001`)
}

func assertTextColumn(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, want string) {
	t.Helper()
	var got string
	err := pool.QueryRow(ctx, query).Scan(&got)
	require.NoError(t, err, "query: %s", query)
	if got != want {
		t.Errorf("query %q = %q, want %q", query, got, want)
	}
}

func assertColumnIsNull(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) {
	t.Helper()
	var got *string
	err := pool.QueryRow(ctx, query).Scan(&got)
	require.NoError(t, err, "query: %s", query)
	if got != nil {
		t.Errorf("query %q = %q, want NULL", query, *got)
	}
}
