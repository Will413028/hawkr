//go:build integration

// forecast_batch_integration_test.go 驗證 ForecastBatch.Handle 自己的邏輯
// （列舉 group、逐一呼叫 GroupForecast、寫入或跳過），不是驗證 GroupForecast
// 推論本身的正確性——那由 forecast_test.go（fake repo）與既有的
// forecast_curve_integration_test.go 覆蓋。
//
// 關鍵設計：不需要真的餵出「有效預測值」的完整 price_observation 歷史才能
// 測試寫入路徑——GroupForecast 對「有 crops 但沒有觀測資料」的 group 仍會
// 回傳合法的 *GroupForecastOutput（每個 horizon day 的 Status 落在
// insufficient_data 之類的非 ready 狀態、P10/P50/P90 皆為 nil），不是
// error。這足以驗證 writeGroupResult 的批次寫入路徑（含 nil P10/P50/P90
// 轉 NULL 的正確性），不需要建構複雜的 ridge model 數值 fixture。
//
// 同一個 //go:build integration + package usecase_test 檔案群組，重用
// dbtest_test.go 的 setupTestDB/newSeqTSID。
package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// TestForecastBatchHandle_WritesReachableGroupSkipsUnreachableGroup 驗證
// Handle 的核心正確性條件：一個有 crops 的 group 應該成功寫入（即使預測值
// 全是 insufficient_data 沒有真實 P10/P50/P90，只要 GroupForecast 沒有回
// error 就該落地），一個完全沒有 crop 的 group（mapping seed 沒有任何子
// 品項）應該被跳過而不是讓整個 Handle 中止。
func TestForecastBatchHandle_WritesReachableGroupSkipsUnreachableGroup(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(980100000000000000)

	// group-with-crop：真的有一個 active crop，GroupForecast 內部的
	// CropsInGroup/活躍過濾/artifact 覆蓋檢查都能通過，回傳非 error
	// 結果（即使沒有觀測資料，points 會是 insufficient_data 狀態）。
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "group-with-crop", Name: "有子品項的大類",
	})
	require.NoError(t, err)
	_, err = q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: tsid(), CropCode: "CROP1", GroupID: groupID, Name: "測試品項",
	})
	require.NoError(t, err)

	// group-empty：沒有任何 crop，GroupForecast 會在 CropsInGroup 之後
	// 因為 len(crops)==0 回傳 error（forecast.go:434 一帶），Handle 必須
	// 跳過它而不是中止。
	_, err = q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "group-empty", Name: "沒有子品項的大類",
	})
	require.NoError(t, err)

	repo := postgres.NewForecastRepository(pool)
	fc := usecase.NewForecast(repo, testConfig(), zap.NewNop())
	// Horizons 必須涵蓋 1..7：SevenDays（sevendays.go:127-132）對 series 缺任一
	// horizon 直接回 error（"artifact requires all of 1..7"），會讓 GroupForecast
	// 整個回傳 error，導致 group-with-crop 被誤判成不可達的 group 而跳過——不是
	// 本測試要驗證的路徑。零值 HorizonModel（Kind=""、CalibrationMethod=""）
	// 會在 CalibrationGateReason（gate.go:65-70）因方法不符判定
	// invalid_calibration_lineage，point 落在 not_predicted、P10/P50/P90 維持
	// nil——這正是本測試需要的「沒有觀測資料但仍是合法輸出」情境，不需要真的
	// 準備 ridge model 數值。
	horizons := map[string]forecast.HorizonModel{}
	for h := 1; h <= 7; h++ {
		horizons[fmt.Sprintf("%d", h)] = forecast.HorizonModel{}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: "test-v1", DatasetDigest: "0000000000000000000000000000000000000000000000000000000000000000",
		Series: map[string]forecast.SeriesModel{
			"moa:104:CROP1": {SeriesID: "moa:104:CROP1", MarketID: "moa:104", CropID: "moa:CROP1", Horizons: horizons},
		},
	}

	batch := usecase.NewForecastBatch(fc, pool, tsid, artifact, zap.NewNop())
	stats, err := batch.Handle(ctx, time.Now().UTC())
	require.NoError(t, err, "唯一寫入成功的 group（group-with-crop）讓 GroupsWritten>0，"+
		"不該觸發 Finding I1 新增的『全數失敗/跳過』backstop")

	require.Equal(t, 2, stats.GroupsTotal)
	require.Equal(t, 1, stats.GroupsWritten)
	require.Equal(t, 1, stats.GroupsSkipped)
	require.Equal(t, []string{"group-empty"}, stats.SkippedGroups)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM crop_group_forecast_result WHERE group_code = 'group-with-crop'`).Scan(&count))
	require.Greater(t, count, 0, "group-with-crop 應該至少寫入一列")

	var noRows int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM crop_group_forecast_result WHERE group_code = 'group-empty'`).Scan(&noRows))
	require.Equal(t, 0, noRows, "被跳過的 group 不該有任何列落地")

	// 交叉核對 nullableForecastNumeric 對 nil P10/P50/P90 的處理：沒有觀測
	// 資料時應該全部是 SQL NULL，不是誤存成 0 或某個 sentinel 值。
	var p10 pgtype.Numeric
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT p10 FROM crop_group_forecast_result WHERE group_code = 'group-with-crop' LIMIT 1`).Scan(&p10))
	require.False(t, p10.Valid, "沒有觀測資料時 p10 應該是 NULL，不是 0 或其他 sentinel 值")

	// final whole-branch review Finding M3：missing_reason 只驗證過 Task 1 的
	// raw-query 層級（手動組 params），從沒透過真正的 Handle → writeGroupResult
	// → InsertCropGroupForecastResult 這條路徑驗證過內容——這條路徑串起了
	// Task 1 的 NULLIF 空字串轉 NULL 處理與 Task 2 的 string(p.MissingReason)
	// passthrough，兩個 task 的接縫值得單獨斷言，不能只靠上面的 p10.Valid==false。
	//
	// 本測試 fixture 的 horizons 全是零值 HorizonModel（Kind=""、
	// CalibrationMethod=""）：CalibrationGateReason（gate.go:61-97）先查
	// CalibrationCount(0) < MinCalibrationSamples，但這裡的 artifact 沒有設
	// Config，MinCalibrationSamples 零值也是 0，0<0 為 false 不觸發；接著查
	// CalibrationMethod 是否等於 expectedMethod（Kind!=direct_ridge 時預期是
	// rolling_point_in_time_pseudo_residuals），""!=該值，判定
	// invalid_calibration_lineage（gate.go:65-70）。這個 SeriesPoint 是本
	// group 唯一的子品項/唯一的 series，group.go:aggregateOneHorizon 在
	// EffectiveVariantCount==0 時用 firstMissingReason（group.go:390-396）
	// 直接沿用它、不是預設值——已讀過 sevendays.go/group.go 原始碼確認整條鏈路，
	// 不只是相信本段註解的說法。
	var missingReason pgtype.Text
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT missing_reason FROM crop_group_forecast_result WHERE group_code = 'group-with-crop' LIMIT 1`).Scan(&missingReason))
	require.True(t, missingReason.Valid, "missing_reason 應該有值（gate 判定 invalid_calibration_lineage），不是 NULL")
	require.Equal(t, string(forecast.MissingReasonInvalidCalibrationLineage), missingReason.String)
}

// TestForecastBatchHandle_AllGroupsSkipped_ReturnsError 是 final whole-branch
// review Finding I1 的回歸測試：驗證迴圈後面新加的 backstop
// （stats.GroupsTotal>0 && stats.GroupsWritten==0 → 回傳 error）真的會觸發。
//
// 不需要真的模擬 DB 斷線（testcontainer 環境下很難做到）——兩個 active group
// 都沒有任何 crop，GroupForecast 對兩者都會在 CropsInGroup 之後因
// len(crops)==0 回傳業務性錯誤（forecast.go:438-440），被 Handle 的迴圈當成
// 業務性失敗跳過。這正是 backstop 要抓的統計形狀：GroupsTotal=2、
// GroupsWritten=0——跟「DB 斷線導致每個 group 的第一個 DB 呼叫都失敗」是
// 一模一樣的 (total>0, written==0) 形狀，backstop 本身刻意不區分兩者
// （見 forecast_batch.go Handle 迴圈後的註解），所以這裡驗證的雖然是「沒有
// 配置有用資料」這個業務情境，也同時證明了 backstop 對真正的 infra 斷線
// 一樣會生效——兩者對 Handle 而言是同一種可觀測形狀。
func TestForecastBatchHandle_AllGroupsSkipped_ReturnsError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	tsid := newSeqTSID(980200000000000000)

	_, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "group-empty-a", Name: "沒有子品項的大類 A",
	})
	require.NoError(t, err)
	_, err = q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "group-empty-b", Name: "沒有子品項的大類 B",
	})
	require.NoError(t, err)

	repo := postgres.NewForecastRepository(pool)
	fc := usecase.NewForecast(repo, testConfig(), zap.NewNop())
	artifact := forecast.ModelArtifact{
		ModelVersion: "test-v1", DatasetDigest: strings.Repeat("0", 64),
	}

	batch := usecase.NewForecastBatch(fc, pool, tsid, artifact, zap.NewNop())
	stats, err := batch.Handle(ctx, time.Now().UTC())

	require.Error(t, err, "兩個 active group 全數跳過，GroupsWritten==0，Handle 必須回傳錯誤而不是 nil")
	require.Equal(t, 2, stats.GroupsTotal)
	require.Equal(t, 0, stats.GroupsWritten)
	require.Equal(t, 2, stats.GroupsSkipped)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM crop_group_forecast_result`).Scan(&count))
	require.Equal(t, 0, count, "全數跳過時不該有任何列落地")
}

// TestForecastBatchHandle_WriteFailureAbortsWholeHandle 驗證 Handle 對
// InsertCropGroupForecastResult 錯誤的處理跟業務性錯誤（GroupForecast 回傳
// error）截然不同：業務性錯誤只跳過該 group、繼續下一個；寫入失敗必須立刻
// 中止整個 Handle，不繼續處理排在後面的 group。這是設計書 Handle 文件註解
// 一直宣稱、但 Task 1/2/3 都沒有測試覆蓋過的另一半錯誤處理路徑。
//
// 'a-group' 排在 'z-group' 前面（ListActiveCropGroups 依 group_code 排序），
// 兩者都設了完整的 crop + series fixture（跟
// TestForecastBatchHandle_WritesReachableGroupSkipsUnreachableGroup 的
// group-with-crop 同一個「零值 HorizonModel」手法），若沒有中止，
// 'z-group' 原本也會成功寫入——用來證明 'z-group' 完全沒被處理到，不是
// 「剛好也失敗了」。
//
// 觸發真實寫入失敗的手法：手動預先插入一列，id 精確等於 Handle 處理 'a-group'
// 時 writeGroupResult 會用到的第一個 tsid()（tsid 是本測試自己掌控的
// newSeqTSID 遞增序列，4 次手動呼叫消耗 start+1..start+4 建立兩個 group +
// 兩個 crop，下一次呼叫——也就是 writeGroupResult 對 7 個 horizon day 的
// 第一次呼叫——保證是 start+5，見下方算式）。這一列的 group_code/target_date
// 刻意選不會撞 dedup unique index 的組合（target_date=1970-01-01，遠早於
// Handle 會產生的任何 horizon 日期），這樣撞到的是 id 這個 PRIMARY KEY，
// 不是 ON CONFLICT 攔得住的 dedup key——INSERT 整句會因為 unique_violation
// 失敗，不需要模擬真的斷線就能得到一個 deterministic 的 DB 層寫入錯誤。
//
// 沒有嘗試透過 mock/fake query 層（那需要把 ForecastBatch 從具體
// *pgxpool.Pool 改成介面，是本次修正範圍外的更大重構）——這個 PK 碰撞手法
// 是在不改動 production 型別的前提下，唯一能在 testcontainer 環境下
// deterministic 觸發真實寫入失敗的辦法。
func TestForecastBatchHandle_WriteFailureAbortsWholeHandle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	const start int64 = 980500000000000000
	tsid := newSeqTSID(start)

	groupIDA, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "a-group", Name: "A 大類",
	})
	require.NoError(t, err)
	_, err = q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: tsid(), CropCode: "CROPA", GroupID: groupIDA, Name: "A 子品項",
	})
	require.NoError(t, err)

	groupIDZ, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: tsid(), GroupCode: "z-group", Name: "Z 大類",
	})
	require.NoError(t, err)
	_, err = q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: tsid(), CropCode: "CROPZ", GroupID: groupIDZ, Name: "Z 子品項",
	})
	require.NoError(t, err)

	// 4 個手動 tsid() 呼叫（上面兩組 UpsertCropGroup/UpsertCrop）依序消耗
	// start+1..start+4。writeGroupResult 對 'a-group'（排序最先處理）會為
	// 7 個 horizon day 各呼叫一次 tsid()，第一次必定拿到 start+5。
	collisionID := start + 5
	_, err = pool.Exec(ctx, `
		INSERT INTO crop_group_forecast_result (
			id, group_code, generated_at, target_date, horizon_days,
			status, aggregation_method, effective_variant_count, total_variant_count,
			market_support_count, freshness, model_version, dataset_digest
		) VALUES ($1, 'a-group', now(), '1970-01-01', 1,
			'seed', 'seed', 0, 0, 0, 'seed', 'seed', $2)`,
		collisionID, strings.Repeat("0", 64))
	require.NoError(t, err, "手動預先插入的碰撞列必須先成功，才能保證後面的 Handle 撞到的是這一列")

	repo := postgres.NewForecastRepository(pool)
	fc := usecase.NewForecast(repo, testConfig(), zap.NewNop())
	horizons := map[string]forecast.HorizonModel{}
	for h := 1; h <= 7; h++ {
		horizons[fmt.Sprintf("%d", h)] = forecast.HorizonModel{}
	}
	artifact := forecast.ModelArtifact{
		ModelVersion: "test-v1", DatasetDigest: strings.Repeat("0", 64),
		Series: map[string]forecast.SeriesModel{
			"moa:104:CROPA": {SeriesID: "moa:104:CROPA", MarketID: "moa:104", CropID: "moa:CROPA", Horizons: horizons},
			"moa:104:CROPZ": {SeriesID: "moa:104:CROPZ", MarketID: "moa:104", CropID: "moa:CROPZ", Horizons: horizons},
		},
	}

	batch := usecase.NewForecastBatch(fc, pool, tsid, artifact, zap.NewNop())
	stats, err := batch.Handle(ctx, time.Now().UTC())

	require.Error(t, err, "寫入失敗必須讓 Handle 整體回傳錯誤，不能被業務性錯誤的跳過邏輯吞掉")
	require.Contains(t, err.Error(), "a-group", "錯誤訊息應該指出是哪個 group 的寫入失敗")
	require.Equal(t, 0, stats.GroupsWritten, "寫入失敗那個 group 不算成功寫入")

	var zGroupCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM crop_group_forecast_result WHERE group_code = 'z-group'`).Scan(&zGroupCount))
	require.Equal(t, 0, zGroupCount, "寫入失敗必須中止整個 Handle，排在後面的 z-group 不該被處理到")
}
