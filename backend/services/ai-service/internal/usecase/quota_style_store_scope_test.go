//go:build integration

package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
)

func TestQuota_GetQuota_ReadsStoreUsage(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	const storeID identity.StoreID = 8001
	const seeded = 3

	weekStart, err := time.Parse("2006-01-02", currentWeekStart())
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO ai_usage (store_id, week_start, usage_count) VALUES ($1, $2, $3)`,
		int64(storeID), weekStart, seeded)
	require.NoError(t, err)

	uc := &QuotaUseCase{q: gen.New(pool)}
	resp, err := uc.GetQuota(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, "3", resp.WeeklyUsage, "must read the store's usage")
}

func TestStyle_GetUsageAndLimit_ReadsStoreUsage(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	const storeID identity.StoreID = 8001
	const seeded = 5

	weekStart, err := time.Parse("2006-01-02", currentWeekStart())
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO ai_usage (store_id, week_start, usage_count) VALUES ($1, $2, $3)`,
		int64(storeID), weekStart, seeded)
	require.NoError(t, err)

	uc := &StyleUseCase{q: gen.New(pool)}
	usage, _, err := uc.getUsageAndLimit(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, seeded, usage, "must read the store's usage")
}

func TestQuota_GetQuota_ReadsStoreLimit(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	const storeID identity.StoreID = 8001

	_, err := pool.Exec(ctx,
		`INSERT INTO ai_quotas (store_id, weekly_limit, memo) VALUES ($1, $2, $3)`,
		int64(storeID), 50, "VIP")
	require.NoError(t, err)

	uc := &QuotaUseCase{q: gen.New(pool)}
	resp, err := uc.GetQuota(ctx, storeID)
	require.NoError(t, err)
	require.Equal(t, "50", resp.WeeklyLimit, "must read the store's override, not the default")
	require.True(t, resp.IsCustom)
}

func TestQuota_UpdateQuota_WritesUnderStore(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	const storeID identity.StoreID = 8001

	uc := &QuotaUseCase{q: gen.New(pool)}
	require.NoError(t, uc.UpdateQuota(ctx, storeID, dto.UpdateQuotaRequest{WeeklyLimit: "77", Memo: "VIP"}))

	quota, err := uc.q.GetAiQuotaByStoreID(ctx, int64(storeID))
	require.NoError(t, err, "row must be written under store_id")
	require.Equal(t, int32(77), quota.WeeklyLimit)

	require.NoError(t, uc.UpdateQuota(ctx, storeID, dto.UpdateQuotaRequest{WeeklyLimit: "88", Memo: "VIP2"}))
	quota, err = uc.q.GetAiQuotaByStoreID(ctx, int64(storeID))
	require.NoError(t, err)
	require.Equal(t, int32(88), quota.WeeklyLimit, "second update must take the ON CONFLICT branch, not fail on the PK")

	require.NoError(t, uc.DeleteQuota(ctx, storeID))
	_, err = uc.q.GetAiQuotaByStoreID(ctx, int64(storeID))
	require.Error(t, err, "delete must target store_id too")
}
