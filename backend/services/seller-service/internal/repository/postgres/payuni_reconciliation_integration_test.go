//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

func TestGetPendingPayuniRegistrations_NoStarvationBeyondBatchSize(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	const (
		total     = 25
		batchSize = 10
		window    = 2 * time.Hour
	)

	longAgo := time.Now().UTC().Add(-24 * time.Hour)
	for i := 1; i <= total; i++ {
		_, err := pool.Exec(ctx,
			`INSERT INTO payuni_registrations
			   (id, account_id, store_id, application_id, payuni_mer_id,
			    registration_status, payment_enabled, last_claimed_at, last_synced_at, updated_at)
			 VALUES ($1, $1, $1, 0, $2, 'pending', false, $3, $3, now())`,
			i, fmt.Sprintf("HAWK%03d", i), longAgo)
		require.NoError(t, err)
	}

	q := gen.New(pool)
	seen := map[int64]int{}

	for tick := 0; tick < total/batchSize+2; tick++ {
		now := time.Now().UTC()
		staleBefore := now.Add(-window)

		rows, err := q.GetPendingPayuniRegistrations(ctx, gen.GetPendingPayuniRegistrationsParams{
			StaleBefore: &staleBefore,
			RowLimit:    batchSize,
		})
		require.NoError(t, err)

		for _, r := range rows {
			claimedAt := time.Now().UTC()
			affected, err := q.ClaimPayuniRegistrationForSync(ctx, gen.ClaimPayuniRegistrationForSyncParams{
				ID:          r.ID,
				ClaimedAt:   &claimedAt,
				StaleBefore: &staleBefore,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1, affected,
				"一筆剛從取件查詢撈出來的列必須能被 claim；取件與 CAS 的述詞不一致的話這裡會是 0（空轉）")
			seen[r.ID]++
		}
	}

	require.Len(t, seen, total,
		"每一筆 pending 都必須在有限輪內被取件到。少於 total ⇒ 取件被截斷在固定的前幾筆，"+
			"後面的列永遠同步不到（starvation，且完全無聲）")
	for id, times := range seen {
		require.Equal(t, 1, times,
			"註冊 %d 在同一個節流窗內被取件 %d 次；租約沒有把它排除在候選集外", id, times)
	}
}

func TestGetPendingPayuniRegistrations_NeverClaimedRowsComeFirst(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	longAgo := time.Now().UTC().Add(-24 * time.Hour)
	older := time.Now().UTC().Add(-10 * time.Hour)

	_, err := pool.Exec(ctx,
		`INSERT INTO payuni_registrations
		   (id, account_id, store_id, application_id, payuni_mer_id,
		    registration_status, payment_enabled, last_claimed_at, updated_at)
		 VALUES (1, 1, 1, 0, 'HAWK001', 'pending', false, $1, now()),
		        (2, 2, 2, 0, 'HAWK002', 'pending', false, NULL, now()),
		        (3, 3, 3, 0, 'HAWK003', 'pending', false, $2, now())`,
		longAgo, older)
	require.NoError(t, err)

	staleBefore := time.Now().UTC().Add(-2 * time.Hour)
	q := gen.New(pool)
	rows, err := q.GetPendingPayuniRegistrations(ctx, gen.GetPendingPayuniRegistrationsParams{
		StaleBefore: &staleBefore,
		RowLimit:    10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	require.EqualValues(t, 2, rows[0].ID, "從未 claim 過的列必須排最前面")
	require.EqualValues(t, 1, rows[1].ID, "其餘依 last_claimed_at 由舊到新")
	require.EqualValues(t, 3, rows[2].ID)
}

func TestGetPendingPayuniRegistrations_ExcludesFreshlySyncedByWebhook(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	longAgo := time.Now().UTC().Add(-24 * time.Hour)
	justNow := time.Now().UTC()

	_, err := pool.Exec(ctx,
		`INSERT INTO payuni_registrations
		   (id, account_id, store_id, application_id, payuni_mer_id,
		    registration_status, payment_enabled, last_claimed_at, last_synced_at, updated_at)
		 VALUES (1, 1, 1, 0, 'HAWK001', 'pending', false, $1, $2, now())`,
		longAgo, justNow)
	require.NoError(t, err)

	staleBefore := time.Now().UTC().Add(-2 * time.Hour)
	q := gen.New(pool)
	rows, err := q.GetPendingPayuniRegistrations(ctx, gen.GetPendingPayuniRegistrationsParams{
		StaleBefore: &staleBefore,
		RowLimit:    10,
	})
	require.NoError(t, err)
	require.Empty(t, rows,
		"webhook 剛更新過的列這輪不該被輪詢取件；只看 last_claimed_at 會讓 poller 在 webhook 之後多打一次 PayUni")
}

func TestClaimPayuniRegistrationForSync_DoesNotTouchLastSyncedAt(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	originalSync := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	_, err := pool.Exec(ctx,
		`INSERT INTO payuni_registrations
		   (id, account_id, store_id, application_id, payuni_mer_id,
		    registration_status, payment_enabled, last_claimed_at, last_synced_at, updated_at)
		 VALUES (1, 1, 1, 0, 'HAWK001', 'pending', false, $1, $1, now())`,
		originalSync)
	require.NoError(t, err)

	claimedAt := time.Now().UTC()
	staleBefore := claimedAt.Add(-2 * time.Hour)
	q := gen.New(pool)

	affected, err := q.ClaimPayuniRegistrationForSync(ctx, gen.ClaimPayuniRegistrationForSyncParams{
		ID:          1,
		ClaimedAt:   &claimedAt,
		StaleBefore: &staleBefore,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)

	row, err := q.GetPayuniRegistrationByMerID(ctx, strPtr("HAWK001"))
	require.NoError(t, err)

	require.NotNil(t, row.LastClaimedAt)
	require.WithinDuration(t, claimedAt, *row.LastClaimedAt, time.Second,
		"租約欄位必須被 CAS 推進")
	require.NotNil(t, row.LastSyncedAt)
	require.WithinDuration(t, originalSync, *row.LastSyncedAt, time.Millisecond,
		"last_synced_at 不得被 claim 動到。它的語意是『上次真的觀測成功是什麼時候』，"+
			"讓 claim 推進它就是 PayUni 故障時 freshness 仍回報 fresh 的根因")

	affected, err = q.ClaimPayuniRegistrationForSync(ctx, gen.ClaimPayuniRegistrationForSyncParams{
		ID:          1,
		ClaimedAt:   &claimedAt,
		StaleBefore: &staleBefore,
	})
	require.NoError(t, err)
	require.EqualValues(t, 0, affected, "租約仍在有效期內，第二次 claim 必須落空")
}

func strPtr(s string) *string { return &s }
