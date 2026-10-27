// Command pricebook-feature-health 對一條 series（--market/--crop）跑
// forecast.FeatureHealth（Task 12），檢查我方 ingest 資料標準化後的 z 分佈
// 是否偏離 model.json 的訓練分佈。
//
// 存在理由（Level 0 ADR，wiki ADR 2026-07-29-pricebook-python-boundary-level-0）：
// 本服務不自己訓練模型，只消費上游訓練機產出的 model.json；那份 artifact 的
// feature_mean/feature_scale 是從上游的訓練資料算出來的。我方 ingest 的資料
// 若在修訂選取（as-of cutoff）、缺值處理（NULL vs 0）或單位上有任何語意差，
// 標準化就會對不上，推論會穩定失準且完全不報錯——原本的防線是「逐筆對帳」，
// 但上游已停止訓練（不再重訓、無 GPU、VM 部署映像無 torch），那條路連同
// Level 1（自行重訓）退路一併消失。改用「特徵分佈健檢」：拿 model.json 的
// feature_mean/feature_scale 對我方資料算標準化後的 z 分佈，系統性偏移即為
// skew 訊號——這是本決策唯一的防線。
//
// 比照本 repo 既有同類 CLI（cmd/pricebook-backfill、cmd/pricebook-export、
// cmd/pricebook-forecast、cmd/pricebook-seed-check）：**這支 binary 不進
// Docker image**（services/pricebook-service/Dockerfile 的 builder stage 只建
// ./cmd/pricebook-service 這一個 binary），從有網路可達目標 Postgres 的機器上
// 直接 `go run` 執行；DB 設定走與主服務相同的 internal/config.Load()（讀環境
// 變數，未設時用開發預設值），不啟動 fx.App。
//
// 流程：對最近 --origins 個 origin 日，各查一次
// domain.ForecastRepository.SeriesObservations（as-of cutoff = 那個 origin），
// 用 forecast.BuildFeatureVectorFromDailyPoints 建出 7 個 horizon 各一條特徵
// 向量，累積成每個 horizon 自己的一批向量，再對 7 個 horizon 各呼叫一次
// forecast.FeatureHealth（每個 (series, horizon) 各有自己的一組
// feature_mean/feature_scale，混算會讓不同方向的偏移互相抵銷——理由見
// skew.go 的 FeatureHealthReport 註解）。
//
// 這支 CLI 只驗證「這條程式路徑跑得動、輸出格式正確」，**不是** skew 檢查
// 本身完成的證據——後者要看實際輸出的 suspect 維數，且必須對真正的
// production 資料跑過。輸出結尾印判讀指引供人工判讀。
//
// Usage:
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-feature-health \
//	    --model testdata/parity/model-demo.json --market 109 --crop A1
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
)

// errFlagParseFailed 是 run 內部的 sentinel：flag.ContinueOnError 模式下
// fs.Parse 失敗時已經自己把錯誤訊息與 usage 印到 stderr 一次，main() 用這個
// sentinel 判斷「這個錯誤不用再印第二次」，只需要以非 0 結束（比照
// cmd/pricebook-forecast、cmd/pricebook-seed-check 同名 sentinel 的既有慣例；
// 每支 CLI 各自獨立宣告，package main 之間不共用檔案作用域外的常數）。
var errFlagParseFailed = errors.New("flag parse failed")

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if !errors.Is(err, errFlagParseFailed) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pricebook-feature-health", flag.ContinueOnError)
	modelPath := fs.String("model", "", "model.json 路徑，必填")
	marketCode := fs.String("market", "", "市場代碼，必填")
	cropCode := fs.String("crop", "", "作物代碼，必填")
	origins := fs.Int("origins", 30, "用最近 N 個 origin 日各建一次特徵向量")
	threshold := fs.Float64("threshold", forecast.DefaultSkewZThreshold, "判定「這一維系統性偏移」的 |mean_z| 門檻")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errFlagParseFailed
	}

	if *modelPath == "" {
		fs.Usage()
		return fmt.Errorf("--model 為必填")
	}
	if *marketCode == "" {
		fs.Usage()
		return fmt.Errorf("--market 為必填")
	}
	if *cropCode == "" {
		fs.Usage()
		return fmt.Errorf("--crop 為必填")
	}
	if *origins <= 0 {
		fs.Usage()
		return fmt.Errorf("--origins 必須為正整數，得到 %d", *origins)
	}

	artifact, err := forecast.LoadArtifact(*modelPath)
	if err != nil {
		return fmt.Errorf("載入 model.json 失敗: %w", err)
	}

	seriesID := "moa:" + *marketCode + ":" + *cropCode
	if _, ok := artifact.Series[seriesID]; !ok {
		return fmt.Errorf("model.json 沒有 series %q（--market/--crop 對不到 artifact 覆蓋的組合）", seriesID)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("載入設定失敗: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	repo := postgres.NewForecastRepository(pool)

	vectorsByHorizon, err := collectFeatureVectors(ctx, repo, artifact, *marketCode, *cropCode, seriesID, *origins, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("蒐集特徵向量失敗: %w", err)
	}

	return printFeatureHealth(artifact, seriesID, vectorsByHorizon, *threshold)
}

// calendarTimezoneOf 鏡射 internal/usecase/forecast.go 同名函式
// （calendarTimezoneOf，該檔第 96-101 行）——本 CLI 不透過 usecase 層（要在
// predict 之前先拿到特徵向量本身，usecase.Forecast 只吐最終預測），兩處各自
// 獨立小段複製，改動需同步（理由同 sevendays.go 對 calendarTimezoneOf 的
// 既有雙處複製模式）。
func calendarTimezoneOf(artifact forecast.ModelArtifact) string {
	if v, ok := artifact.Metadata["calendar_timezone"].(string); ok && v != "" {
		return v
	}
	return "UTC"
}

// collectFeatureVectors 對最近 origins 個 origin 日（now, now-1d, ...,
// now-(origins-1)d）各查一次 as-of 觀測、建出 7 個 horizon 各一條特徵向量，
// 回傳 map[horizonDays][]向量。每個 origin 各自呼叫 SeriesObservations（而不是
// 查一次寬窗共用）：cutoff 隨 origin 變動才是 point-in-time 語意（同
// usecase.Forecast.SeriesForecast 的 Step 1-2），避免用「現在才確定」的修訂
// 去污染較早 origin 本該只看到的資料版本。
func collectFeatureVectors(
	ctx context.Context,
	repo domain.ForecastRepository,
	artifact forecast.ModelArtifact,
	marketCode, cropCode, seriesID string,
	origins int,
	now time.Time,
) (map[int][][]float64, error) {
	calendarTimezone := calendarTimezoneOf(artifact)
	featureConfig := artifact.Config.FeatureConfig
	lookback := featureConfig.MaxLookbackDays()

	vectorsByHorizon := make(map[int][][]float64, 7)
	for h := 1; h <= 7; h++ {
		vectorsByHorizon[h] = nil
	}

	for i := 0; i < origins; i++ {
		origin := now.AddDate(0, 0, -i)
		originDate, cutoff, err := forecast.OriginDateAndCutoff(origin, calendarTimezone)
		if err != nil {
			return nil, fmt.Errorf("origin %s: %w", origin.Format(time.RFC3339), err)
		}

		from := originDate.AddDays(-lookback).ToTime()
		to := originDate.ToTime()
		observations, err := repo.SeriesObservations(ctx, marketCode, cropCode, from, to, cutoff)
		if err != nil {
			return nil, fmt.Errorf("SeriesObservations(%s, %s) at origin %s: %w", marketCode, cropCode, originDate, err)
		}

		pointsByDate := make(map[forecast.Date]forecast.PricePoint, len(observations))
		for _, obs := range observations {
			d := forecast.DateFromTime(obs.TradeDate)
			isActive := obs.MarketActive && obs.CropActive
			pt, err := forecast.NewPricePoint(forecast.PricePointInput{
				SeriesID: seriesID, MarketID: "moa:" + marketCode, CropID: "moa:" + cropCode,
				TradeDate: d, AveragePrice: obs.AvgPrice, Volume: obs.VolumeKg,
				AvailableAt: obs.AvailableAt, CategoryCode: obs.CategoryCode, IsActive: &isActive,
			})
			if err != nil {
				return nil, fmt.Errorf("building price point for %s on %s: %w", seriesID, d, err)
			}
			pointsByDate[d] = pt
		}

		for h := 1; h <= 7; h++ {
			fv, err := forecast.BuildFeatureVectorFromDailyPoints(pointsByDate, origin, h, featureConfig, calendarTimezone)
			if err != nil {
				return nil, fmt.Errorf("BuildFeatureVectorFromDailyPoints(%s, horizon=%d) at origin %s: %w", seriesID, h, originDate, err)
			}
			vectorsByHorizon[h] = append(vectorsByHorizon[h], fv.Values)
		}
	}

	return vectorsByHorizon, nil
}

// printFeatureHealth 對 7 個 horizon 各呼叫一次 forecast.FeatureHealth 並印出
// 逐維表格＋各自的摘要（suspect 維數/總維數）；沒有 Ridge 的 horizon
// （kind != direct_ridge）印出跳過原因，不中斷其餘 horizon 的報告。7 個
// horizon 的 suspect 計數刻意不加總成單一分數——每個 (series, horizon) 各有
// 自己的 feature_mean/feature_scale，混算會讓不同方向的偏移互相抵銷（理由見
// skew.go 的 FeatureHealthReport 型別註解），CLI 的最終摘要因此逐 horizon
// 列出，不是一個跨 horizon 的總分。
func printFeatureHealth(artifact forecast.ModelArtifact, seriesID string, vectorsByHorizon map[int][][]float64, threshold float64) error {
	type horizonSummary struct {
		horizon  int
		suspectN int
		dimN     int
		skipped  string // 非空代表這個 horizon 被跳過的原因
	}
	summaries := make([]horizonSummary, 0, 7)

	for h := 1; h <= 7; h++ {
		fmt.Printf("=== series=%s horizon=D+%d ===\n", seriesID, h)

		report, err := forecast.FeatureHealth(artifact, vectorsByHorizon[h], seriesID, h, threshold)
		if err != nil {
			fmt.Printf("跳過（%v）\n\n", err)
			summaries = append(summaries, horizonSummary{horizon: h, skipped: err.Error()})
			continue
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "index\tname\tmean_z\tmax_abs_z\tsample_n\tsuspect"); err != nil {
			return err
		}
		for _, d := range report.Dimensions {
			if _, err := fmt.Fprintf(w, "%d\t%s\t%.4f\t%.4f\t%d\t%v\n", d.Index, d.Name, d.MeanZ, d.MaxAbsZ, d.SampleN, d.Suspect); err != nil {
				return err
			}
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Printf("vector_n=%d threshold=%.1f suspect=%d/%d\n\n", report.VectorN, report.Threshold, report.SuspectN, len(report.Dimensions))

		summaries = append(summaries, horizonSummary{horizon: h, suspectN: report.SuspectN, dimN: len(report.Dimensions)})
	}

	fmt.Printf("=== 摘要（逐 horizon，各自獨立不加總——理由見上）===\n")
	for _, s := range summaries {
		if s.skipped != "" {
			fmt.Printf("D+%d: 跳過（%s）\n", s.horizon, s.skipped)
			continue
		}
		fmt.Printf("D+%d: suspect 維數 %d/%d\n", s.horizon, s.suspectN, s.dimN)
	}
	fmt.Println()

	fmt.Println("判讀：suspect 維數 0 → 無系統性偏移，Level 0（沿用上游 model.json）的前提成立。")
	fmt.Println("      suspect 維數 > 0 → 逐維檢查該維的語意（lag/rolling/missing flag/last_price），")
	fmt.Println("      常見成因是修訂選取（as-of cutoff）、缺值處理（NULL vs 0）、或單位差異。")
	fmt.Println("      Level 1（自行重訓）退路在上游停訓後已不存在，偏移必須另尋解法")
	fmt.Println("      （見 wiki ADR 2026-07-29-pricebook-python-boundary-level-0 的 Revocation Triggers）。")

	return nil
}
