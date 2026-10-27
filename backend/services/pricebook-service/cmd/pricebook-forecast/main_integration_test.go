//go:build integration

// main_integration_test.go 是 task-11-brief.md Step 4「對 dev DB live 驗證」
// 在沒有 dev DB 密碼時的替代做法：用一個 postgres:16-alpine testcontainer
// （本 package 共用，見 dbtest_test.go）取代 dev DB，套用 db/migrations/、
// 每個測試在自己的資料庫裡種入形狀正確的資料，
// 直接呼叫本 package 真正的 CLI 入口（run()，跟 `go run ./cmd/pricebook-forecast`
// 執行的是同一段程式碼），驗證 flag 解析 → config.Load() → database.NewPool →
// postgres.NewForecastRepository → usecase.Forecast → CLI 輸出格式這條完整路徑
// 真的能跑通。
//
// 🔴 這不是 live 驗證，只是替代驗證。它證明的是「程式路徑對」，不是「對真實
// dev/prod 資料的推論結果合理」——後者需要 brief 原本要求的 dev DB 存取，
// 目前不可得。報告裡必須清楚標示這個差異，不能把這份輸出講成 live 驗證。
//
// 用 `go test -tags integration ./cmd/pricebook-forecast/...`（或
// `make test-integration`）跑，需要本機 Docker daemon。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// TSID 基底：避開 internal/repository/postgres 底下既有整合測試用掉的區段
// （700…/900…/981…），這個 package 是獨立的測試二進位，理論上不會撞，仍照
// repo 慣例挑一個沒用過的前綴。
const (
	cliMarketID    int64 = 992000000000000001
	cliCropGroupID int64 = 992000000000000002
	cliCropID      int64 = 992000000000000003
	cliObsIDBase   int64 = 992000000000001000
	cliRestDayID   int64 = 992000000000002000
)

// setupTestDB 見 dbtest_test.go：本 package 共用一個 Postgres 容器，每個測試從
// 跑完 migrations 的 template 複製出一份自己的資料庫，並把 DB_* 環境變數指向它。

// seedCLIFixture 種一個市場（109）、一個大類、一個作物（A1，category N05，
// 對齊 testdata/parity/model-demo.json 的 moa:109:A1 series），加上：
//   - originDateUTC 往回 40 天、逐日一筆的行情觀測，avg_price/volume_kg
//     有變化（避免特徵向量退化成全常數），available_at = trade_date + 20h
//     （模擬 MOA 隔天才發布——這讓「以較早的 origin 查詢，data_through_date
//     應該更小」這個 as-of 不變量在種子資料裡是真的成立，不是湊出來的）。
//   - 一筆 2026 年、market_type='F'（對應 calendar.go 的 N05）的休市日曆列，
//     日期刻意落在推論窗（originDateUTC 起 7 天）之外，只用來提供
//     RestDayCoverage，不會把任何一個 D+1..D+7 標成休市。
func seedCLIFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool, originDateUTC time.Time) {
	t.Helper()
	q := gen.New(pool)

	marketID, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{ID: cliMarketID, MarketCode: "109", Name: "台北二"})
	require.NoError(t, err)

	categoryCode := "N05"
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID: cliCropGroupID, GroupCode: "CLI-GRP-A1", Name: "測試大類",
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	cropID, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID: cliCropID, CropCode: "A1", GroupID: groupID, Name: "測試作物",
		CategoryCode: &categoryCode,
	})
	require.NoError(t, err)

	for offset := 0; offset <= 40; offset++ {
		tradeDate := originDateUTC.AddDate(0, 0, -offset)
		availableAt := tradeDate.Add(20 * time.Hour)
		price, err := postgres.NumericFromString(fmt.Sprintf("%.4f", 55+float64(offset%10)))
		require.NoError(t, err)
		volume, err := postgres.NumericFromString(fmt.Sprintf("%.3f", 900+float64(offset%5)*20))
		require.NoError(t, err)

		require.NoError(t, q.InsertPriceObservation(ctx, gen.InsertPriceObservationParams{
			ID: cliObsIDBase + int64(offset), MarketID: marketID, CropID: cropID,
			TradeDate: pgtype.Date{Time: tradeDate, Valid: true}, RevisionNo: 1,
			AvailableAt: pgtype.Timestamptz{Time: availableAt, Valid: true},
			AvgPrice:    price, VolumeKg: volume,
			Source: "moa", SourceDigest: "cli-integration-test",
		}))
	}

	marketType := "F"
	require.NoError(t, q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
		ID: cliRestDayID, MarketID: marketID, MarketType: &marketType,
		RestDate:  pgtype.Date{Time: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		FetchedAt: pgtype.Timestamptz{Time: originDateUTC, Valid: true},
	}))
}

// captureStdout 重導 os.Stdout 執行 fn，回傳期間寫出的內容。run() 直接寫
// os.Stdout（跟 cmd/pricebook-export、cmd/pricebook-backfill 同構，未注入
// io.Writer），這是不改動 main.go 產品程式碼、驗證真實輸出的最小做法。
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String(), runErr
}

// TestRun_SeriesForecast_EndToEndAgainstTestcontainer 是本檔最重要的一條：
// 完整跑一次 run()，驗證輸出的結構化欄位。
//
// 不硬性斷言至少一列 status=ready——demo artifact 的 ridge 係數是對真實 MOA
// 資料訓練出來的，餵合成資料進去，實際預測值落在哪個品質閘門不是這個測試
// 能保證的事（也不重要，user 已表態不在乎預測準不準）。但下面幾件事是純
// wiring 正確性問題，必須斷言：
//   - 七列都印出來，horizon/target_date 連續。
//   - model_version / dataset_digest 非空，且等於 demo artifact 裡的值。
//   - 沒有任何一列是 calendar_unverified 或 model_lineage_missing——這兩個
//     原因分別對應陷阱 2（VerifiedCalendar 沒建對）與品質閘門的血緣檢查，
//     種子資料已經提供了休市日曆覆蓋與合法 artifact，若這兩個原因出現，
//     代表 CLI → usecase → repository 的接線本身有問題，不是資料不足。
//   - 沒有任何一列是 market_closed——休市日曆列刻意種在推論窗之外。
func TestRun_SeriesForecast_EndToEndAgainstTestcontainer(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDateUTC := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	seedCLIFixture(ctx, t, pool, originDateUTC)

	modelPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "parity", "model-demo.json"))
	require.NoError(t, err)

	output, runErr := captureStdout(t, func() error {
		return run(ctx, []string{
			"--model", modelPath, "--market", "109", "--crop", "A1",
			"--origin", origin.Format(time.RFC3339), "--json",
		})
	})
	require.NoError(t, runErr, "CLI 輸出:\n%s", output)

	tableOutput, tableErr := captureStdout(t, func() error {
		return run(ctx, []string{
			"--model", modelPath, "--market", "109", "--crop", "A1",
			"--origin", origin.Format(time.RFC3339),
		})
	})
	require.NoError(t, tableErr)
	t.Logf("CLI 人類可讀輸出:\n%s", tableOutput)
	t.Logf("CLI 實際輸出（testcontainer 替代驗證，非 live 驗證）:\n%s", output)

	var result usecase.SeriesForecastOutput
	require.NoError(t, json.Unmarshal([]byte(output), &result), "輸出不是合法 JSON:\n%s", output)

	require.Equal(t, "moa:109:A1", result.SeriesID)
	require.Equal(t, "direct-ridge-v2-aa66e30432a2729a", result.ModelVersion)
	require.NotEmpty(t, result.DatasetDigest)
	require.Len(t, result.Forecast, 7, "七列都要印出來")

	wantDate := forecast.NewDate(2026, 7, 25)
	for i, p := range result.Forecast {
		require.Equal(t, i+1, p.HorizonDays)
		wantDate = wantDate.AddDays(1)
		require.Equal(t, wantDate, p.TargetDate, "第 %d 列 target_date 應連續", i)
		require.NotEqual(t, forecast.MissingReasonCalendarUnverified, p.MissingReason,
			"種子資料已提供 2026 年 N05 的休市日曆覆蓋，不該出現 calendar_unverified（陷阱 2 的接線若沒做對會在這裡現形）")
		require.NotEqual(t, forecast.MissingReasonModelLineageMissing, p.MissingReason,
			"demo artifact 血緣完整，不該出現 model_lineage_missing")
		require.NotEqual(t, forecast.PredictionStatusMarketClosed, p.Status,
			"休市日曆列種在推論窗之外，D+1..D+7 不該有任何一天被標成休市")
		if p.Status == forecast.PredictionStatusReady {
			require.NotNil(t, p.P10)
			require.NotNil(t, p.P50)
			require.NotNil(t, p.P90)
			require.LessOrEqual(t, *p.P10, *p.P50)
			require.LessOrEqual(t, *p.P50, *p.P90)
		}
	}
}

// TestRun_SeriesForecast_OriginCutoffShrinksDataThroughDate 對應
// task-11-brief.md Step 4 的驗收判準：用較早的 --origin 查詢，
// data_through_date 必須跟著變小——這證明 as-of cutoff 真的生效，不是無視
// cutoff 撈最新資料。
func TestRun_SeriesForecast_OriginCutoffShrinksDataThroughDate(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	originDateUTC := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	seedCLIFixture(ctx, t, pool, originDateUTC)

	modelPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "parity", "model-demo.json"))
	require.NoError(t, err)

	runOnce := func(origin time.Time) *forecast.Date {
		output, runErr := captureStdout(t, func() error {
			return run(ctx, []string{
				"--model", modelPath, "--market", "109", "--crop", "A1",
				"--origin", origin.Format(time.RFC3339), "--json",
			})
		})
		require.NoError(t, runErr, "CLI 輸出:\n%s", output)
		var result usecase.SeriesForecastOutput
		require.NoError(t, json.Unmarshal([]byte(output), &result))
		return result.DataThroughDate
	}

	// origin 早於 artifact.TrainedAt（2026-07-18T15:44:30Z）會被 SevenDays 擋掉，
	// 所以「較早」的 origin 只能取 trained_at 之後、種子資料之前的一個時間點。
	laterThrough := runOnce(time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC))
	earlierThrough := runOnce(time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC))

	require.NotNil(t, laterThrough)
	require.NotNil(t, earlierThrough)
	require.True(t, earlierThrough.Before(*laterThrough),
		"origin 較早時 data_through_date（%s）應早於 origin 較晚時（%s）——否則 as-of cutoff 沒有生效",
		earlierThrough, laterThrough)
}
