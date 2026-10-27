package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// ForecastBatch 是每日預測 cron 的批次邏輯：對所有 active crop_group 各跑一次
// Forecast.GroupForecast，把七天預測窗口寫進 crop_group_forecast_result。
//
// 不改動、不重寫 GroupForecast 本身的推論邏輯——這裡只是外層迴圈 + 落地，
// 跟 PriceIngest.Handle 對 fetchAllWithEmptyRetry 的關係同理：業務邏輯留在
// 原本的地方，這裡只加"批次跑 + 寫結果"這一層。
type ForecastBatch struct {
	forecast *Forecast
	pool     *pgxpool.Pool
	tsid     func() int64
	artifact forecast.ModelArtifact
	logger   *zap.Logger
}

// NewForecastBatch 建構 ForecastBatch。artifact 直接複用 di.go 既有的
// provideModelArtifact 產出的 forecast.ModelArtifact，不另外載入第二份
// model.json（同一份 artifact 已經在 fx 圖裡，見 internal/usecase/di.go
// provideModelArtifact 的說明：這是整個服務唯一一處註冊，其餘消費端一律
// 直接依賴這個型別參數，不重複 provide）。
//
// logger（final whole-branch review Finding M1）：改用注入的 *zap.Logger，
// 不再用全域 zap.L()——跟 Forecast/TrendSummary/ObservedPrice 等 sibling
// usecase 的既有慣例一致，也讓 skip-warning log 在測試裡可以用
// zap/zaptest/observer 斷言，不像全域 logger 在測試binary 裡預設是 no-op。
func NewForecastBatch(f *Forecast, pool *pgxpool.Pool, tsid func() int64, artifact forecast.ModelArtifact, logger *zap.Logger) *ForecastBatch {
	return &ForecastBatch{forecast: f, pool: pool, tsid: tsid, artifact: artifact, logger: logger}
}

// ForecastBatchStats 是一次 Handle 呼叫的統計，供呼叫端（cron job）記 log。
// 不寫進任何 audit table（本次改動刻意不比照 ingest_run 稽核，見 design spec
// 的「範圍外」段）。
type ForecastBatchStats struct {
	GroupsTotal   int
	GroupsWritten int
	GroupsSkipped int
	// SkippedGroups 是被跳過的 group_code，供 log 診斷用（不寫進任何表）。
	SkippedGroups []string
}

// Handle 對所有 active crop_group 各跑一次 GroupForecast，寫入結果。
//
// 單一 group 呼叫 GroupForecast 失敗（例如該 group 沒有任何 series 被 model
// 覆蓋、或該 group 在 mapping seed 裡查無 crop）只跳過該 group、記 Warn log，
// 不中止整個 Handle——GroupForecast 對這幾種情境回傳的都是「業務性」錯誤
// （forecast.go:434/477/488 一帶的 fmt.Errorf），不是 DB/infra 層級的錯誤，
// 跟 PriceIngest.Handle 對 errParseTradeDate 這種單筆壞資料的處理哲學一致：
// 一個 group 有問題不該讓其他 group 也連帶抓不到當天的預測。
//
// 寫入失敗（InsertCropGroupForecastResult 回錯誤，例如 DB 連線斷線）視為
// infra 層級錯誤，直接中止整個 Handle 並回傳錯誤——不像業務性錯誤那樣可以
// 安全跳過，因為這代表接下來所有 group 的寫入大概率都會用同樣的方式失敗，
// 讓呼叫端（cron job）的錯誤處理清楚看到「這輪整批都沒寫成功」而不是
// 誤判成「大部分 group 剛好都被跳過」。
//
// final whole-branch review Finding I1：迴圈本身無法區分 GroupForecast 內部
// 的 DB 呼叫（CropsInGroup/GroupObservations/RestDays/RestDayCoverage/
// RecentVolume，見 forecast.go GroupForecast/RecentVolume 一帶）是因為業務性
// 原因失敗（group 沒有子品項、artifact 沒有覆蓋）還是 infra 層級失敗（DB
// 斷線、逾時）——兩者回傳的 error 型別/外觀完全相同。若 DB 在整輪 21:00 UTC
// 執行期間掛掉，每個 group 都會在各自第一個 DB 呼叫就失敗、被當成業務性
// 錯誤跳過，迴圈跑完後 GroupsWritten 恆為 0，若不額外處理，Handle 會回傳
// nil error——呼叫端（forecastJob.run）會用 Info level 記一筆「批次預測
// 完成，groups_written=0」，把整場 infra 斷線悄悄記成「今天剛好沒什麼好寫」
// 的正常日子，違反本檔案開頭説明的錯誤處理哲學。
//
// 修法是迴圈後面的 backstop（見下方），不是在這裡改動重新分類每個錯誤——
// 那需要改動 GroupForecast/SeriesForecast 本身的錯誤型別，是本次修正範圍外
// 的更大改動。
func (u *ForecastBatch) Handle(ctx context.Context, origin time.Time) (ForecastBatchStats, error) {
	var stats ForecastBatchStats

	q := gen.New(database.GetDBTX(ctx, u.pool))

	groupCodes, err := q.ListActiveCropGroups(ctx)
	if err != nil {
		return stats, fmt.Errorf("列出 active crop_group 失敗: %w", err)
	}
	stats.GroupsTotal = len(groupCodes)

	for _, groupCode := range groupCodes {
		out, err := u.forecast.GroupForecast(ctx, GroupForecastInput{
			Artifact:  u.artifact,
			GroupCode: groupCode,
			Origin:    origin,
		})
		if err != nil {
			u.logger.Warn("forecast_batch: 略過無法產生預測的大類",
				zap.String("group_code", groupCode), zap.Error(err))
			stats.GroupsSkipped++
			stats.SkippedGroups = append(stats.SkippedGroups, groupCode)
			continue
		}

		if err := u.writeGroupResult(ctx, q, groupCode, origin, out); err != nil {
			return stats, fmt.Errorf("寫入預測結果失敗（group=%s）: %w", groupCode, err)
		}
		stats.GroupsWritten++
	}

	// Finding I1 的 backstop：GroupsTotal>0（有 active group 可跑）但
	// GroupsWritten==0（一個都沒真的寫成功）代表「全部 group 都被跳過」或
	// 「全部 group 都失敗」，跟真正的 infra 層級全面故障是同一種統計形狀，
	// 值得回傳錯誤讓呼叫端走 Error level log／觸發告警，而不是被上面
	// 「單一 group 業務性失敗可以跳過」的邏輯悄悄吸收成一個看似正常的
	// nil error。GroupsTotal==0（沒有任何 active group）刻意不觸發這個
	// 條件——那是另一種合法的「今天真的沒事做」情境，不是故障。
	//
	// 這是粗略但有效的判斷：不區分「所有 group 剛好都是業務性資料缺失」與
	// 「DB 真的斷線」（兩者在這裡看起來一樣），但至少不會讓後者偽裝成
	// 一個平靜無事的日子。
	if stats.GroupsTotal > 0 && stats.GroupsWritten == 0 {
		return stats, fmt.Errorf(
			"批次預測全數失敗或跳過（total=%d, skipped=%d）：疑似 infra 層級故障或全面性業務資料缺失，非單一 group 的正常業務性跳過",
			stats.GroupsTotal, stats.GroupsSkipped)
	}

	return stats, nil
}

// writeGroupResult 把一次 GroupForecast 呼叫的結果（out.Forecast，通常 7 筆，
// 每個 horizon day 各一筆）一次批次寫入 crop_group_forecast_result。
func (u *ForecastBatch) writeGroupResult(
	ctx context.Context,
	q *gen.Queries,
	groupCode string,
	generatedAt time.Time,
	out *GroupForecastOutput,
) error {
	n := len(out.Forecast)
	if n == 0 {
		return nil
	}

	ids := make([]int64, n)
	groupCodes := make([]string, n)
	generatedAts := make([]pgtype.Timestamptz, n)
	targetDates := make([]pgtype.Date, n)
	horizonDays := make([]int32, n)
	statuses := make([]string, n)
	missingReasons := make([]string, n)
	aggregationMethods := make([]string, n)
	effectiveVariantCounts := make([]int32, n)
	totalVariantCounts := make([]int32, n)
	marketSupportCounts := make([]int32, n)
	p10s := make([]pgtype.Numeric, n)
	p50s := make([]pgtype.Numeric, n)
	p90s := make([]pgtype.Numeric, n)
	freshnesses := make([]string, n)
	modelVersions := make([]string, n)
	datasetDigests := make([]string, n)

	genAt := pgxutil.TimestamptzParam(generatedAt)

	for i, p := range out.Forecast {
		targetTime := p.TargetDate.ToTime()

		ids[i] = u.tsid()
		groupCodes[i] = groupCode
		generatedAts[i] = genAt
		targetDates[i] = pgxutil.DateParam(&targetTime)
		horizonDays[i] = int32(p.HorizonDays)
		statuses[i] = string(p.Status)
		// MissingReasons 是 []string，不是 []pgtype.Text：nullability 由
		// Task 1 的 SQL 查詢（NULLIF(missing_reason, '')）在寫入前轉換，
		// 空字串在這裡直接傳，DB 落地時才變成真正的 SQL NULL——不在 Go 層
		// 重複這個轉換（見 gen/crop_group_forecast.sql.go 該欄位的說明）。
		missingReasons[i] = string(p.MissingReason)
		aggregationMethods[i] = p.AggregationMethod
		effectiveVariantCounts[i] = int32(p.EffectiveVariantCount)
		totalVariantCounts[i] = int32(p.TotalVariantCount)
		marketSupportCounts[i] = int32(p.MarketSupportCount)

		p10, err := nullableForecastNumeric(p.P10)
		if err != nil {
			return fmt.Errorf("p10 轉換失敗（group=%s, horizon=%d）: %w", groupCode, p.HorizonDays, err)
		}
		p10s[i] = p10
		p50, err := nullableForecastNumeric(p.P50)
		if err != nil {
			return fmt.Errorf("p50 轉換失敗（group=%s, horizon=%d）: %w", groupCode, p.HorizonDays, err)
		}
		p50s[i] = p50
		p90, err := nullableForecastNumeric(p.P90)
		if err != nil {
			return fmt.Errorf("p90 轉換失敗（group=%s, horizon=%d）: %w", groupCode, p.HorizonDays, err)
		}
		p90s[i] = p90

		freshnesses[i] = string(out.Freshness)
		modelVersions[i] = out.ModelVersion
		datasetDigests[i] = out.DatasetDigest
	}

	return q.InsertCropGroupForecastResult(ctx, gen.InsertCropGroupForecastResultParams{
		Ids: ids, GroupCodes: groupCodes, GeneratedAts: generatedAts, TargetDates: targetDates,
		HorizonDays: horizonDays, Statuses: statuses, MissingReasons: missingReasons,
		AggregationMethods: aggregationMethods, EffectiveVariantCounts: effectiveVariantCounts,
		TotalVariantCounts: totalVariantCounts, MarketSupportCounts: marketSupportCounts,
		P10s: p10s, P50s: p50s, P90s: p90s,
		Freshnesses: freshnesses, ModelVersions: modelVersions, DatasetDigests: datasetDigests,
	})
}

// nullableForecastNumeric 把 *float64 轉成 pgtype.Numeric，nil 轉成 SQL NULL
// （GroupPoint.P10/P50/P90 在 Status 非 ready 時是 nil）。固定 4 位小數，
// 比照 PriceIngest.resolveRecord 對價格欄位的既有慣例（避免 %v/%g 把極端值
// 格式化成科學記號）。
func nullableForecastNumeric(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	return postgres.NumericFromString(fmt.Sprintf("%.4f", *f))
}
