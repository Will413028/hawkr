//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// TestListActiveCropGroupsOnlyReturnsActive 驗證 active=false 的 group 不會
// 出現在結果裡（既有 active 過濾欄位的正確性，這個 query 是本次新增，之前
// 沒有測過這個過濾條件是否真的接上）。
func TestListActiveCropGroupsOnlyReturnsActive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(970100000000000000)

	_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "active-grp", Name: "啟用中",
	})
	require.NoError(t, err)

	// UpsertCropGroup 沒有 active 參數（既有 query 固定寫 true），這裡直接
	// 用 pool 補一筆 active=false 的資料驗證過濾條件——不透過 sqlc query
	// 是因為現有 upsert query 不支援寫入 inactive 狀態，這個測試只需要
	// DB 裡真的存在一筆 inactive 資料。
	_, err = pool.Exec(ctx,
		`INSERT INTO crop_group (id, group_code, name, active) VALUES ($1, $2, $3, false)`,
		tsid(), "inactive-grp", "已停用")
	require.NoError(t, err)

	groups, err := q.ListActiveCropGroups(ctx)
	require.NoError(t, err)
	require.Contains(t, groups, "active-grp")
	require.NotContains(t, groups, "inactive-grp")
}

// TestInsertCropGroupForecastResultWritesAllRows 驗證批次寫入：一次呼叫寫入
// 一個 group 的 3 筆不同 horizon_day 列（用 3 不是 7 純粹是測試資料量沒必要
// 湊滿七天窗口），逐筆核對欄位值正確落地，且陣列位置對齊正確
// （用不同 horizon_days/target_date 交叉核對，避免位置錯位的測試設計缺陷
// ——同類問題本 repo 稍早的批次寫入 review 抓到過一次，這裡直接照那次的
// 修法設計，不要重蹈覆轍）。
func TestInsertCropGroupForecastResultWritesAllRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(970200000000000000)

	_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "grp-a", Name: "測試大類",
	})
	require.NoError(t, err)

	now := time.Now().UTC()
	generatedAt := pgxutil.TimestamptzParam(now)
	d1 := now.AddDate(0, 0, 1)
	d2 := now.AddDate(0, 0, 2)
	d3 := now.AddDate(0, 0, 3)

	// MissingReasons 是 []string 不是 []pgtype.Text：sqlc 對「unnest(...)::text[]
	// 搭配 ROWS FROM 批次寫入」這個形狀的陣列參數，個別元素的 nullability 一律
	// 忽略（sqlc-dev/sqlc issue 1851，本 repo 用的 1.31.1 仍未修，sqlc.arg／
	// sqlc.narg 生成結果逐位元組相同，均已實測），無法直接生成 nullable 型別。
	// 呼叫端仍傳 []string，空字串代表「沒有 missing reason」，但 query 內部用
	// NULLIF(missing_reason, '') 在寫入前轉換，DB 實際落地的是真正的 SQL NULL，
	// 不是空字串 sentinel——下面的斷言會直接讀回驗證這件事（Valid == false）。
	err = q.InsertCropGroupForecastResult(ctx, gen.InsertCropGroupForecastResultParams{
		Ids:                    []int64{tsid(), tsid(), tsid()},
		GroupCodes:             []string{"grp-a", "grp-a", "grp-a"},
		GeneratedAts:           []pgtype.Timestamptz{generatedAt, generatedAt, generatedAt},
		TargetDates:            []pgtype.Date{pgxutil.DateParam(&d1), pgxutil.DateParam(&d2), pgxutil.DateParam(&d3)},
		HorizonDays:            []int32{1, 2, 3},
		Statuses:               []string{"ready", "ready", "insufficient_data"},
		MissingReasons:         []string{"", "", "insufficient_recent_data"},
		AggregationMethods:     []string{"volume_weighted", "volume_weighted", "volume_weighted"},
		EffectiveVariantCounts: []int32{3, 3, 2},
		TotalVariantCounts:     []int32{3, 3, 3},
		MarketSupportCounts:    []int32{2, 2, 1},
		P10s:                   []pgtype.Numeric{mustNumeric(t, "10.0000"), mustNumeric(t, "11.0000"), {}},
		P50s:                   []pgtype.Numeric{mustNumeric(t, "12.0000"), mustNumeric(t, "13.0000"), {}},
		P90s:                   []pgtype.Numeric{mustNumeric(t, "14.0000"), mustNumeric(t, "15.0000"), {}},
		Freshnesses:            []string{"fresh", "fresh", "fresh"},
		ModelVersions:          []string{"v1", "v1", "v1"},
		DatasetDigests:         []string{"digest-a", "digest-a", "digest-a"},
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM crop_group_forecast_result`).Scan(&count))
	require.Equal(t, 3, count)

	var status string
	var p50 string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, p50::text FROM crop_group_forecast_result WHERE horizon_days = 2`).
		Scan(&status, &p50))
	require.Equal(t, "ready", status)
	require.Equal(t, "13.0000", p50, "horizon_days=2 那一列的 p50 必須是索引 1 的值，不能跟其他 horizon 錯位")

	var missingReason pgtype.Text
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT missing_reason FROM crop_group_forecast_result WHERE horizon_days = 3`).
		Scan(&missingReason))
	require.True(t, missingReason.Valid)
	require.Equal(t, "insufficient_recent_data", missingReason.String)

	// horizon_days 1、2 兩列呼叫端傳的是空字串（"沒有 reason"），這裡直接驗證
	// DB 真的落地成 SQL NULL（Valid == false），不是把空字串當成 sentinel 存進去
	// ——這是 NULLIF 轉換要保證的事，只驗到「寫入沒報錯」不足以證明它有效。
	var missingReason1, missingReason2 pgtype.Text
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT missing_reason FROM crop_group_forecast_result WHERE horizon_days = 1`).
		Scan(&missingReason1))
	require.False(t, missingReason1.Valid, "horizon_days=1 沒有 missing reason，應落地為真正的 SQL NULL")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT missing_reason FROM crop_group_forecast_result WHERE horizon_days = 2`).
		Scan(&missingReason2))
	require.False(t, missingReason2.Valid, "horizon_days=2 沒有 missing reason，應落地為真正的 SQL NULL")
}

// TestInsertCropGroupForecastResultDeduplicatesSameDayRerun 驗證
// ON CONFLICT DO NOTHING：同一天對同一個 (group, target_date) 寫入兩次，
// 第二次應該是無害的 no-op，不產生重複列。
func TestInsertCropGroupForecastResultDeduplicatesSameDayRerun(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(970300000000000000)

	_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "grp-b", Name: "測試大類 B",
	})
	require.NoError(t, err)

	now := time.Now().UTC()
	d1 := now.AddDate(0, 0, 1)
	insertOne := func(generatedAt time.Time, id int64) error {
		ga := pgxutil.TimestamptzParam(generatedAt)
		return q.InsertCropGroupForecastResult(ctx, gen.InsertCropGroupForecastResultParams{
			Ids: []int64{id}, GroupCodes: []string{"grp-b"}, GeneratedAts: []pgtype.Timestamptz{ga},
			TargetDates: []pgtype.Date{pgxutil.DateParam(&d1)}, HorizonDays: []int32{1},
			Statuses: []string{"ready"}, MissingReasons: []string{""},
			AggregationMethods:     []string{"volume_weighted"},
			EffectiveVariantCounts: []int32{1}, TotalVariantCounts: []int32{1}, MarketSupportCounts: []int32{1},
			P10s: []pgtype.Numeric{mustNumeric(t, "1.0000")}, P50s: []pgtype.Numeric{mustNumeric(t, "2.0000")}, P90s: []pgtype.Numeric{mustNumeric(t, "3.0000")},
			Freshnesses: []string{"fresh"}, ModelVersions: []string{"v1"}, DatasetDigests: []string{"digest-b"},
		})
	}

	require.NoError(t, insertOne(now, tsid()))
	// 同一天（same generated_at 的 UTC 曆日）第二次呼叫，即使 generated_at 的
	// 時分秒不同（模擬 rolling deploy 重疊時兩個 pod 各自在稍微不同的時刻跑）
	// 也要被擋下來——unique index 用的是 (generated_at AT TIME ZONE 'UTC')::date，
	// 不是完整時間戳。
	require.NoError(t, insertOne(now.Add(5*time.Minute), tsid()))

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM crop_group_forecast_result WHERE group_code = 'grp-b'`).Scan(&count))
	require.Equal(t, 1, count, "同一天重複寫入應該被 ON CONFLICT DO NOTHING 擋下來，不產生第二列")

	// 順便驗證這條路徑也一樣：呼叫端傳的空字串 missing_reason 落地是真正的
	// SQL NULL，不是空字串 sentinel。
	var missingReason pgtype.Text
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT missing_reason FROM crop_group_forecast_result WHERE group_code = 'grp-b'`).
		Scan(&missingReason))
	require.False(t, missingReason.Valid, "空字串 missing_reason 應落地為真正的 SQL NULL")
}
