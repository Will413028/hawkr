//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestLoadSingleStintMapping_SingleVsMultiStint(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupSellerTestDB(ctx, t)
	defer cleanup()

	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8100, 1001, 100, true, 'Store A', 'active')`)
	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (200, 1002, 200, true, 'Store B', 'active')`)
	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8300, 1003, 300, false, 'Store C (closed)', 'closed')`)
	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8301, 1004, 300, true, 'Store C (reopened)', 'active')`)
	mustExecSeller(t, ctx, pool, `INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name, status) VALUES (8400, 1005, 400, false, 'Store D (closed, never reopened)', 'closed')`)

	mapping, multiStintCount, err := loadSingleStintMapping(ctx, pool)
	require.NoError(t, err)

	if multiStintCount != 1 {
		t.Errorf("multiStintCount = %d, want 1 (account 300)", multiStintCount)
	}
	if len(mapping) != 3 {
		t.Errorf("len(mapping) = %d, want 3 (accounts 100, 200, 400); mapping = %+v", len(mapping), mapping)
	}
	if got := mapping[100]; got != 8100 {
		t.Errorf("mapping[100] = %d, want 8100", got)
	}
	if got := mapping[200]; got != 200 {
		t.Errorf("mapping[200] = %d, want 200 (grandfathered — caller treats == as no-op, not this query)", got)
	}
	if got := mapping[400]; got != 8400 {
		t.Errorf("mapping[400] = %d, want 8400 (single-stint even though currently closed)", got)
	}
	if _, ok := mapping[300]; ok {
		t.Errorf("mapping contains account 300 (multi-stint), want excluded")
	}
}

func mustExecSeller(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(ctx, sql)
	require.NoError(t, err, "exec: %s", sql)
}
