//go:build integration

package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

func TestGetPendingPayuniRegistrations_IncludesNonTerminalOutcomesOnTerminalRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	longAgo := time.Now().UTC().Add(-24 * time.Hour)
	rows := []struct {
		id      int64
		status  string
		mer     string
		member  string
		wantHit bool
	}{
		{1, "active", "1", "5", true},
		{2, "active", "1", "9", true},
		{3, "active", "9", "1", true},
		{4, "failed", "9", "5", true},
		{5, "failed", "91", "1", true},
		{6, "active", "1", "1", false},
		{7, "failed", "2", "1", false},
		{8, "active", "1", "92", false},
		{9, "failed", "99", "5", false},
		{10, "pending", "", "", true},
	}
	for _, r := range rows {
		_, err := pool.Exec(ctx,
			`INSERT INTO payuni_registrations
			   (id, account_id, store_id, application_id, payuni_mer_id,
			    registration_status, mer_status, member_status, payment_enabled, last_claimed_at, last_synced_at, updated_at)
			 VALUES ($1, $1, $1, 0, $6, $2, NULLIF($3, ''), NULLIF($4, ''), false, $5, $5, now())`,
			r.id, r.status, r.mer, r.member, longAgo, "HAWK"+strconv.FormatInt(r.id, 10))
		require.NoError(t, err)
	}

	staleBefore := time.Now().UTC().Add(-2 * time.Hour)
	q := gen.New(pool)
	got, err := q.GetPendingPayuniRegistrations(ctx, gen.GetPendingPayuniRegistrationsParams{StaleBefore: &staleBefore, RowLimit: 100})
	require.NoError(t, err)
	hits := map[int64]bool{}
	for _, g := range got {
		hits[g.ID] = true
	}
	for _, r := range rows {
		require.Equal(t, r.wantHit, hits[r.id], "id=%d status=%s mer=%q member=%q", r.id, r.status, r.mer, r.member)
	}

	for _, r := range rows {
		n, err := q.ClaimPayuniRegistrationForSync(ctx, gen.ClaimPayuniRegistrationForSyncParams{ID: r.id, ClaimedAt: timePtrIT(time.Now().UTC()), StaleBefore: &staleBefore})
		require.NoError(t, err)
		require.Equal(t, r.wantHit, n == 1, "claim id=%d", r.id)
	}
}

func timePtrIT(t time.Time) *time.Time { return &t }
