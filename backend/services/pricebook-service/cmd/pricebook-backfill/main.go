// Command pricebook-backfill 是一次性、可重跑的 CLI：手動觸發某個歷史區間的
// 農業部批發市場行情 backfill（usecase.Backfill.Handle 的手動觸發入口，與
// Task 8 的每日排程共用同一個底層 usecase.PriceIngest.Handle，雙入口單一實作）。
//
// 🔴 責任邊界例外（2026-08-17，見
// docs/superpowers/specs/2026-08-17-pricebook-ingest-window-resilience-design.md
// §6）：docs/開發規範/後端服務責任邊界規範.md §四將 *BackfillCLI 列為不應
// 存在的元件，建議替代方案是「DDL 後既有 row 補值改用 SQL 範本」。這個替代
// 方案在這裡不成立——本 CLI 需要的正確資料不存在我方 DB 任何一張表裡，必須
// 呼叫外部 MOA API 才拿得到，沒有 SQL 語句能推導出來，屬於規範舉例（DDL 後
// 既有欄位補值、hashtag 計數，答案永遠可從既有 DB 資料重算）沒有涵蓋到的
// 情境類別。經 review 確認保留。
//
// decisions.md D1：backfill 不透過 admin gRPC 觸發，改用本 CLI。理由：一次
// 全歷史 backfill（2012 年至今約 170 個月份塊）要跑數小時，跑在 serving
// replica 裡的話任何一次 merge 到 dev 觸發的 helm upgrade 都會把它攔腰砍斷，
// 且與每日排程搶同一行程資源、受 pod memory limit 拘束——計畫沒有續跑邏輯，
// ingest_run 會永遠停在 status=running。
//
// 比照本 repo 既有的同類 CLI（services/account-service/cmd/auth-backfill、
// services/seller-service/cmd/backfillstorecode）：不進 Docker image
// （services/pricebook-service/Dockerfile 的 builder stage 只建
// ./cmd/pricebook-service 這一個 binary），從有網路可達目標 Postgres 的機器上
// 直接 `go run` 執行；DB/MOA 設定走與主服務相同的 internal/config.Load()（讀
// 環境變數，未設時用開發預設值），不啟動 fx.App——手動組出 Backfill 需要的
// 三個依賴（*moa.Client、*pgxpool.Pool、tsid 產生器）即可，比照兩個precedent
// CLI 皆不透過 fx 的做法。
//
// 本 CLI 有兩個互斥的模式：
//
//	backfill      補一段歷史區間（--start/--end）
//	accept-gap    接受單一格為「上游確認沒有資料」（--accept-gap）
//
// 第二個模式共用同一支 binary 而不是另開一個 cmd/，是為了不再新增一個落在
// 責任邊界規範 §四「不應存在的元件」清單裡的 CLI——它與 backfill 的例外理由
// 完全相同（正確答案不在我方 DB 任何一張表裡，必須呼叫 MOA 才拿得到），
// 掛在已經取得例外的這支底下，比開第二個新元件誠實。
//
// Usage（日期一律是本 CLI 自己的契約：ISO YYYY-MM-DD，不是 MOA 的民國格式）：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-backfill --start 2026-06-01 --end 2026-06-30
//
//	DB_HOST=... go run ./cmd/pricebook-backfill --accept-gap \
//	  --market 423 --date 2026-08-11 --by will \
//	  --reason "MOA 對該日回 Data: []，鄰日 08-12 有資料（positive control）"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// dateLayout 是本 CLI 對 --start/--end 的契約：ISO YYYY-MM-DD。
// 刻意不是 MOA 的民國格式（moa.ToROCDate 那套）——這是我們自己的 CLI 介面，
// 不是轉發給 MOA 的原始查詢參數，操作者不該被要求換算民國年。
const dateLayout = "2006-01-02"

// errFlagParseFailed 是 run 內部的 sentinel：flag.ContinueOnError 模式下
// fs.Parse 失敗時已經自己把錯誤訊息與 usage 印到 stderr 一次，main() 用這個
// sentinel 判斷「這個錯誤不用再印第二次」，只需要以非 0 結束（M6，final
// whole-branch review：修正前 main() 會把同一段錯誤訊息再印一次）。
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
	fs := flag.NewFlagSet("pricebook-backfill", flag.ContinueOnError)
	startStr := fs.String("start", "", "backfill 起始日期，ISO 格式 YYYY-MM-DD（backfill 模式必填）")
	endStr := fs.String("end", "", "backfill 結束日期（含），ISO 格式 YYYY-MM-DD（backfill 模式必填）")
	acceptGap := fs.Bool("accept-gap", false,
		"接受缺口模式：把單一 (市場,交易日) 標成「上游確認沒有資料」，不再告警")
	marketCode := fs.String("market", "", "市場代碼（--accept-gap 必填），例如 423")
	dateStr := fs.String("date", "", "交易日，ISO 格式 YYYY-MM-DD（--accept-gap 必填）")
	acceptedBy := fs.String("by", "", "接受人（--accept-gap 必填），寫進稽核欄位")
	reason := fs.String("reason", "", "接受理由（--accept-gap 必填），寫進稽核欄位")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// -h/--help：fs.Parse 已經印出 usage，視為正常結束（exit 0），
			// 比照 services/account-service/cmd/auth-backfill 的既有慣例
			// （M6：修正前 --help 會被當成一般錯誤處理、以 exit code 1 結束）。
			return nil
		}
		return errFlagParseFailed
	}

	// 兩個模式的旗標互斥，混用直接擋掉。--accept-gap 帶 --start/--end 幾乎
	// 一定是打錯（想補一段卻只會接受一格），靜默忽略比報錯危險得多。
	if *acceptGap {
		if *startStr != "" || *endStr != "" {
			fs.Usage()
			return fmt.Errorf("--accept-gap 不能與 --start/--end 併用")
		}
		return runAcceptGap(ctx, fs, *marketCode, *dateStr, *acceptedBy, *reason)
	}
	if *marketCode != "" || *dateStr != "" || *acceptedBy != "" || *reason != "" {
		fs.Usage()
		return fmt.Errorf("--market/--date/--by/--reason 只在 --accept-gap 模式下有意義")
	}
	if *startStr == "" || *endStr == "" {
		fs.Usage()
		return fmt.Errorf("--start 與 --end 皆為必填")
	}

	start, err := time.Parse(dateLayout, *startStr)
	if err != nil {
		return fmt.Errorf("--start 格式錯誤（需 YYYY-MM-DD）: %w", err)
	}
	end, err := time.Parse(dateLayout, *endStr)
	if err != nil {
		return fmt.Errorf("--end 格式錯誤（需 YYYY-MM-DD）: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("載入設定失敗: %w", err)
	}

	// 讓 usecase.Backfill.Handle 透過套件層級 zap.L() 印出的段落進度（見
	// internal/usecase/backfill.go Fix 3）在這個獨立行程裡真的印得出來——
	// 本 CLI 不啟動 fx.App，沒有 internal/config/di.go 的
	// fx.Invoke(zap.ReplaceGlobals) 會自動幫忙做這件事，不手動呼叫的話
	// zap.L() 預設回傳 no-op logger，所有 Info/Warn 都會被靜默丟掉，
	// 22 小時的 backfill 又會回到「完全零輸出」的原始問題（final
	// whole-branch review Fix 3）。用同一顆 logger.NewLogger(cfg.Env)，
	// 比照 internal/config/di.go 對正式服務的做法。
	zap.ReplaceGlobals(logger.NewLogger(cfg.Env))

	// database.NewPool（而非裸的 pgxpool.New）：注入 QueryExecModeExec（PgBouncer
	// transaction-pooling 相容，見 pkg/database/postgres.go）、connect_timeout=10、
	// 建構時 Ping——本 CLI 一跑就是數小時，連線在啟動當下就該壞得明明白白，而不是
	// 拖到第一次真正查詢才發現，比照 internal/config/di.go 的 newPool 呼叫方式。
	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	// 兩個 usecase 共用同一個 tsid 產生器即可——這是本 CLI 自己的獨立行程，
	// 不透過 fx.Provide(tsid.NewGenerator) 的那個唯一註冊點（那條規則管的是
	// 「同一個 fx 圖裡」不能重複 provide，本 binary 完全不起 fx.App）。
	idGen := tsid.NewGenerator()
	client := moa.NewClient(cfg.MOABaseURL, cfg.MOAOpenDataBaseURL)
	priceIngest := usecase.NewPriceIngest(client, pool, idGen)
	backfill := usecase.NewBackfill(priceIngest, pool, idGen)

	runID, err := backfill.Handle(ctx, start, end)
	if err != nil {
		if runID != 0 {
			return fmt.Errorf("backfill 失敗（ingest_run.id=%d，狀態已標記為失敗）: %w", runID, err)
		}
		return fmt.Errorf("backfill 失敗（尚未建立 ingest_run）: %w", err)
	}

	fmt.Printf("backfill 完成，ingest_run.id=%d\n", runID)
	return nil
}

// runAcceptGap 是 --accept-gap 模式：接受單一 (市場, 交易日) 為「上游確認沒有
// 資料」的終態，讓 PricebookMarketDataGap 對這一格停止告警。
//
// 🔴 這不是「把告警關掉」的按鈕。usecase.AcceptGap 會在寫入之前重新向 MOA 查
// 一次同一格：查得到交易資料就改走正常寫入並標 complete、回報拒絕接受；回休市
// 哨兵就標 rest_day。只有上游真的什麼都沒回，才會寫成 accepted_gap，並把
// --by/--reason 一併存進稽核欄位。設計取捨見 ADR
// 2026-09-03-pricebook-accepted-gap-terminal-state。
func runAcceptGap(ctx context.Context, fs *flag.FlagSet, marketCode, dateStr, acceptedBy, reason string) error {
	if marketCode == "" || dateStr == "" || acceptedBy == "" || reason == "" {
		fs.Usage()
		return fmt.Errorf("--accept-gap 模式下 --market/--date/--by/--reason 皆為必填")
	}
	day, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		return fmt.Errorf("--date 格式錯誤（需 YYYY-MM-DD）: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("載入設定失敗: %w", err)
	}
	zap.ReplaceGlobals(logger.NewLogger(cfg.Env))

	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	priceIngest := usecase.NewPriceIngest(moa.NewClient(cfg.MOABaseURL, cfg.MOAOpenDataBaseURL), pool, tsid.NewGenerator())

	outcome, err := priceIngest.AcceptGap(ctx, marketCode, day, acceptedBy, reason)
	if err != nil {
		return fmt.Errorf("接受缺口失敗（market=%s, date=%s）: %w", marketCode, dateStr, err)
	}

	switch outcome {
	case usecase.AcceptGapAccepted:
		fmt.Printf("已接受缺口：market=%s date=%s status=accepted_gap by=%s\n",
			marketCode, dateStr, acceptedBy)
	case usecase.AcceptGapUpstreamHasData:
		fmt.Printf("未接受：上游現在有 %s %s 的資料，已寫入並標記 complete\n", marketCode, dateStr)
	case usecase.AcceptGapRestDay:
		fmt.Printf("未接受：上游回報 %s %s 為休市，已標記 rest_day\n", marketCode, dateStr)
	}
	return nil
}
