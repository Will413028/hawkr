//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

func TestBackfillSingleStintSellerIdentity_UpdatesSingleStintAcrossAllTables(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	// account 100 is single-stint (only account in this test's mapping) →
	// rekeyed to store 8100. account 200 is NOT in the mapping (simulates a
	// multi-stint/reopened seller the caller deliberately excluded) → every
	// row seeded for it must remain untouched at 200.
	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5001, 1, 100, 2)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5002, 1, 200, 2)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_seller_forum (id, seller_id, user_id, content, role) VALUES (6001, 900, 100, 'f', 2)`)
	mustExec(t, ctx, pool, `INSERT INTO cms_seller_forum (id, seller_id, user_id, content, role) VALUES (6002, 900, 200, 'f', 2)`)
	mustExec(t, ctx, pool, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason_code) VALUES (7001, 100, 'POST', 1, 'SPAM')`)
	mustExec(t, ctx, pool, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason_code) VALUES (7002, 200, 'POST', 1, 'SPAM')`)
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8001, 100, 555)`)
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8002, 200, 555)`)

	mapping := postgres.SingleStintSellerMapping{100: 8100}
	result, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, mapping)
	require.NoError(t, err)

	if result.CmsCommentUserID != 1 {
		t.Errorf("CmsCommentUserID = %d, want 1", result.CmsCommentUserID)
	}
	if result.CmsSellerForumUserID != 1 {
		t.Errorf("CmsSellerForumUserID = %d, want 1", result.CmsSellerForumUserID)
	}
	if result.ReportsReporterID != 1 {
		t.Errorf("ReportsReporterID = %d, want 1", result.ReportsReporterID)
	}
	if result.UserBlocksBlockerID != 1 {
		t.Errorf("UserBlocksBlockerID = %d, want 1", result.UserBlocksBlockerID)
	}
	if result.Total() != 4 {
		t.Errorf("Total() = %d, want 4", result.Total())
	}

	// Single-stint account 100's rows: rekeyed to store 8100.
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_comment WHERE id = 5001`, 8100)
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_seller_forum WHERE id = 6001`, 8100)
	assertColumnValue(t, ctx, pool, `SELECT reporter_id FROM reports WHERE id = 7001`, 8100)
	assertColumnValue(t, ctx, pool, `SELECT blocker_id FROM user_blocks WHERE id = 8001`, 8100)

	// Multi-stint (excluded from mapping) account 200's rows: untouched.
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_comment WHERE id = 5002`, 200)
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_seller_forum WHERE id = 6002`, 200)
	assertColumnValue(t, ctx, pool, `SELECT reporter_id FROM reports WHERE id = 7002`, 200)
	assertColumnValue(t, ctx, pool, `SELECT blocker_id FROM user_blocks WHERE id = 8002`, 200)
}

func TestBackfillSingleStintSellerIdentity_GrandfatherNoOp(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5003, 1, 300, 2)`)

	mapping := postgres.SingleStintSellerMapping{300: 300}
	result, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, mapping)
	require.NoError(t, err)

	if result.Total() != 0 {
		t.Errorf("Total() = %d, want 0 (grandfather account_id==store_id is a no-op)", result.Total())
	}
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_comment WHERE id = 5003`, 300)
}

// TestBackfillSingleStintSellerIdentity_EmptyMapping: no accounts to backfill
// (e.g. every seller in the environment has reopened at least once) must not
// error and must not touch any row.
func TestBackfillSingleStintSellerIdentity_EmptyMapping(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5004, 1, 400, 2)`)

	result, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, postgres.SingleStintSellerMapping{})
	require.NoError(t, err)
	if result.Total() != 0 {
		t.Errorf("Total() = %d, want 0", result.Total())
	}
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_comment WHERE id = 5004`, 400)
}

func TestBackfillSingleStintSellerIdentity_SkipsUniqueConstraintCollisions(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)

	// user_blocks: blocker=100 blocking blocked=555 collides with an
	// existing blocker=8100 blocking the same blocked=555. blocker=100
	// blocking blocked=777 has no conflict. blocker=100 blocking
	// blocked=8100 would (post-rekey) equal blocker==blocked (self-block) —
	// must be skipped by the inequality guard even though no OTHER row
	// exists at (8100, 8100).
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8001, 100, 555)`)
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8002, 8100, 555)`)
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8003, 100, 777)`)
	mustExec(t, ctx, pool, `INSERT INTO user_blocks (id, blocker_id, blocked_id) VALUES (8004, 100, 8100)`)

	mapping := postgres.SingleStintSellerMapping{100: 8100}
	result, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, mapping)
	require.NoError(t, err)

	// Only the non-colliding rows counted as updated.
	if result.UserBlocksBlockerID != 1 {
		t.Errorf("UserBlocksBlockerID = %d, want 1 (only blocker_id=100→blocked_id=777)", result.UserBlocksBlockerID)
	}
	assertColumnValue(t, ctx, pool, `SELECT blocker_id FROM user_blocks WHERE id = 8001`, 100)
	assertColumnValue(t, ctx, pool, `SELECT blocker_id FROM user_blocks WHERE id = 8004`, 100)
	assertColumnValue(t, ctx, pool, `SELECT blocker_id FROM user_blocks WHERE id = 8003`, 8100)
}

func TestBackfillSingleStintSellerIdentity_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	mustExec(t, ctx, pool, `INSERT INTO cms_post (id, seller_id, content) VALUES (1, 900, 'p')`)
	mustExec(t, ctx, pool, `INSERT INTO cms_comment (id, post_id, user_id, role) VALUES (5005, 1, 100, 2)`)

	mapping := postgres.SingleStintSellerMapping{100: 8100}
	first, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, mapping)
	require.NoError(t, err)
	if first.Total() != 1 {
		t.Fatalf("first run Total() = %d, want 1", first.Total())
	}

	second, err := postgres.BackfillSingleStintSellerIdentity(ctx, pool, mapping)
	require.NoError(t, err)
	if second.Total() != 0 {
		t.Errorf("second run Total() = %d, want 0 (idempotent re-run)", second.Total())
	}
	assertColumnValue(t, ctx, pool, `SELECT user_id FROM cms_comment WHERE id = 5005`, 8100)
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(ctx, sql)
	require.NoError(t, err, "exec: %s", sql)
}

func assertColumnValue(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, want int64) {
	t.Helper()
	var got int64
	err := pool.QueryRow(ctx, query).Scan(&got)
	require.NoError(t, err, "query: %s", query)
	if got != want {
		t.Errorf("query %q = %d, want %d", query, got, want)
	}
}
