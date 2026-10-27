// Command pricebook-forecast 對單一 (market, crop) 或一個大類跑七日推論，
// 印出對外用的預測與觀測窗（usecase.Forecast.SeriesForecast /
// usecase.Forecast.GroupForecast 的手動觸發入口）。
//
// P4 沒有任何對外 surface（proto/gateway 歸 Plan C），本 CLI 是唯一能拿到
// live 證據的地方，比照本 repo 既有的同類 CLI（cmd/pricebook-backfill、
// cmd/pricebook-export）：**這支 binary 不進 Docker image**
// （services/pricebook-service/Dockerfile 的 builder stage 只建
// ./cmd/pricebook-service 這一個 binary），從有網路可達目標 Postgres 的機器上
// 直接 `go run` 執行；DB 設定走與主服務相同的 internal/config.Load()（讀環境
// 變數，未設時用開發預設值），不啟動 fx.App——手動組出 Forecast 需要的依賴
// （domain.ForecastRepository、*config.Config、*zap.Logger）即可，比照三支
// precedent CLI 皆不透過 fx 的做法。
//
// 刻意不在啟動時載入 model.json：artifact 路徑一律由 --model 傳入，避開
// 「production model.json 154MB 怎麼進 image / 要不要放物件儲存」這個前置條件
// （從上游訓練機取得那份檔案）還沒完成的決策（見
// internal/usecase/forecast.go 的 Forecast 型別註解）。
//
// Usage（單一 series 模式，--market 與 --crop 皆必填）：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-forecast \
//	    --model testdata/parity/model-demo.json \
//	    --market 109 --crop A1
//
// Usage（大類模式，--group 取代 --market/--crop——GroupForecast 本身就是跨
// 市場聚合，不需要、也不接受 --market 縮限到單一市場）：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-forecast --model testdata/parity/model-demo.json --group <group_code>
//
// --origin 選填，RFC3339 格式，預設現在時間（UTC）；指一個較早的時間點可以
// 驗證 as-of cutoff 真的生效（data_through_date 會隨之變小，見 task-11-brief.md
// Step 4 的驗收判準）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// errFlagParseFailed 是 run 內部的 sentinel：flag.ContinueOnError 模式下
// fs.Parse 失敗時已經自己把錯誤訊息與 usage 印到 stderr 一次，main() 用這個
// sentinel 判斷「這個錯誤不用再印第二次」，只需要以非 0 結束（比照
// cmd/pricebook-backfill、cmd/pricebook-export 同名 sentinel 的既有慣例）。
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
	fs := flag.NewFlagSet("pricebook-forecast", flag.ContinueOnError)
	modelPath := fs.String("model", "", "model.json 路徑，必填")
	marketCode := fs.String("market", "", "市場代碼；--crop 模式必填，--group 模式不使用（GroupForecast 本身跨市場聚合）")
	cropCode := fs.String("crop", "", "作物代碼，與 --group 二選一")
	groupCode := fs.String("group", "", "大類代碼，與 --crop 二選一")
	originStr := fs.String("origin", "", "推論基準時間，RFC3339 格式，選填，預設現在時間（UTC）")
	jsonOut := fs.Bool("json", false, "輸出 JSON 而非人類可讀表格")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// -h/--help：fs.Parse 已經印出 usage，視為正常結束（exit 0）。
			return nil
		}
		return errFlagParseFailed
	}

	if *modelPath == "" {
		fs.Usage()
		return fmt.Errorf("--model 為必填")
	}
	if (*cropCode == "") == (*groupCode == "") {
		fs.Usage()
		return fmt.Errorf("--crop 與 --group 恰須擇一（不能兩者皆空，也不能兩者皆填）")
	}
	if *cropCode != "" && *marketCode == "" {
		fs.Usage()
		return fmt.Errorf("--crop 模式下 --market 為必填")
	}
	if *groupCode != "" && *marketCode != "" {
		fs.Usage()
		return fmt.Errorf("--group 模式不接受 --market（GroupForecast 本身跨市場聚合，語意上不該被單一市場縮限；" +
			"package doc／flag usage 文字原本已宣稱「不接受」，但實際上從未驗證過，task-11-report.md §6 記錄了這個落差）")
	}

	origin := time.Now().UTC()
	if *originStr != "" {
		parsed, err := time.Parse(time.RFC3339, *originStr)
		if err != nil {
			return fmt.Errorf("--origin 格式錯誤（需 RFC3339）: %w", err)
		}
		origin = parsed
	}

	artifact, err := forecast.LoadArtifact(*modelPath)
	if err != nil {
		return fmt.Errorf("載入 model.json 失敗: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("載入設定失敗: %w", err)
	}

	// 讓 usecase.Forecast 透過套件層級 zap.L()／注入的 *zap.Logger 印出的
	// Warn（陷阱防禦：休市日曆缺覆蓋、artifact 血緣異常等）在這個獨立行程裡
	// 真的印得出來——本 CLI 不啟動 fx.App，沒有 internal/config/di.go 的
	// fx.Invoke(zap.ReplaceGlobals) 會自動幫忙做這件事，比照
	// cmd/pricebook-backfill 的既有做法。
	log := logger.NewLogger(cfg.Env)
	zap.ReplaceGlobals(log)

	// database.NewPool（而非裸的 pgxpool.New）：注入 QueryExecModeExec（PgBouncer
	// transaction-pooling 相容）、connect_timeout=10、建構時 Ping，比照
	// internal/config/di.go 的 newPool 呼叫方式。
	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	repo := postgres.NewForecastRepository(pool)
	fc := usecase.NewForecast(repo, cfg, log)

	if *cropCode != "" {
		out, err := fc.SeriesForecast(ctx, usecase.SeriesForecastInput{
			Artifact: artifact, MarketCode: *marketCode, CropCode: *cropCode, Origin: origin,
		})
		if err != nil {
			return fmt.Errorf("推論失敗: %w", err)
		}
		return printSeriesForecast(out, *jsonOut)
	}

	out, err := fc.GroupForecast(ctx, usecase.GroupForecastInput{
		Artifact: artifact, GroupCode: *groupCode, Origin: origin,
	})
	if err != nil {
		return fmt.Errorf("推論失敗: %w", err)
	}
	return printGroupForecast(out, *jsonOut)
}

func printSeriesForecast(out *usecase.SeriesForecastOutput, jsonOut bool) error {
	if jsonOut {
		return printJSON(out)
	}
	fmt.Printf("series_id:         %s\n", out.SeriesID)
	fmt.Printf("model_version:     %s\n", out.ModelVersion)
	fmt.Printf("dataset_digest:    %s\n", out.DatasetDigest)
	fmt.Printf("data_through_date: %s\n", formatDatePtr(out.DataThroughDate))
	fmt.Printf("freshness:         %s\n", out.Freshness)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "horizon\ttarget_date\tstatus\tmissing_reason\tmodel_kind\tp10\tp50\tp90"); err != nil {
		return err
	}
	for _, p := range out.Forecast {
		if _, err := fmt.Fprintf(w, "D+%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			p.HorizonDays, p.TargetDate, p.Status, orDash(string(p.MissingReason)), orDash(p.ModelKind),
			formatFloatPtr(p.P10), formatFloatPtr(p.P50), formatFloatPtr(p.P90)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printGroupForecast(out *usecase.GroupForecastOutput, jsonOut bool) error {
	if jsonOut {
		return printJSON(out)
	}
	fmt.Printf("group_code:        %s\n", out.GroupCode)
	fmt.Printf("model_version:     %s\n", out.ModelVersion)
	fmt.Printf("dataset_digest:    %s\n", out.DatasetDigest)
	fmt.Printf("data_through_date: %s\n", formatDatePtr(out.DataThroughDate))
	fmt.Printf("freshness:         %s\n", out.Freshness)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "horizon\ttarget_date\tstatus\tmissing_reason\tp10\tp50\tp90\taggregation_method\teffective_variant_count\ttotal_variant_count\tmarket_support_count"); err != nil {
		return err
	}
	for _, p := range out.Forecast {
		if _, err := fmt.Fprintf(w, "D+%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			p.HorizonDays, p.TargetDate, p.Status, orDash(string(p.MissingReason)),
			formatFloatPtr(p.P10), formatFloatPtr(p.P50), formatFloatPtr(p.P90),
			orDash(p.AggregationMethod), p.EffectiveVariantCount, p.TotalVariantCount, p.MarketSupportCount); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func formatFloatPtr(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.4f", *v)
}

func formatDatePtr(d *forecast.Date) string {
	if d == nil {
		return "-"
	}
	return d.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
