// Command pricebook-import-raw 是一次性 CLI：把 資料團隊提供的 legacy/bronze 原始
// JSON 匯出檔匯入 price_observation，重用 usecase.Backfill.Handle 的全部正確性
// 邏輯（revision 判定、digest 去重、crop/market/crop_group upsert、ingest_run
// 稽核），只是把資料來源從 MOA API 換成本機檔案（NewPriceIngestWithClient 注入
// internal/importraw.FileClient）。不進 Docker image，從有網路可達目標 Postgres
// 的機器上直接 go run 執行；DB 設定走與主服務相同的 internal/config.Load()。
//
// 🔴 fileImportRetryDelay（2026-08-17，final whole-branch review Finding I3）：
// 本 CLI 呼叫 usecase.NewPriceIngestWithClient 時顯式傳入近乎為 0 的
// retryDelay，不是生產用的 5 秒（usecase 套件內部 emptyResponseRetryDelay 的
// 值）。fetchAllWithEmptyRetry 的空回應重試機制是為了接住 MOA API 暫時壅塞
// 設計的——同一個查詢再打一次，MOA 有機會恢復、拿到不同（非空）的結果；但
// FileClient 讀的是本機檔案，同一個 (market, day) 讀 N 次保證回傳一模一樣的
// 結果，重試對它完全沒有「等一等可能拿到不同結果」這個前提。修法前這裡沿用
// NewPriceIngestWithClient 的舊預設（生產用 emptyResponseRetryDelay），archive
// 裡任何一個真缺口都會白等 5 秒 × 2 次重試 ≈ 10 秒，大範圍歷史匯入（下方
// Usage 範例本身就是跨年、21 個市場的範圍）累加起來可能是數小時的純浪費。
// 這裡不是重用 usecase.SetRetryDelayForTest——那個方法明確限定只給整合測試
// 使用；NewPriceIngestWithClient 改成要求呼叫端顯式傳入 retryDelay，逼每個
// 非 MOA 來源的呼叫端自己決定這個語意，不會悄悄退回生產預設值。
//
// Usage：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-import-raw \
//	  --legacy-dir /path/to/price --bronze-dir /path/to/bronze/source=afa_legacy/dataset=farm_trans_data \
//	  --start 2022-08-12 --end 2026-07-20
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
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/importraw"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

const dateLayout = "2006-01-02"

// fileImportRetryDelay 是傳給 usecase.NewPriceIngestWithClient 的 retryDelay：
// FileClient 讀本機檔案是確定性的，重試沒有機會拿到不同結果（見上方 package
// comment 的說明），選近乎為 0（但非字面 0，明確表達「刻意設極短」而非誤用
// 零值）的極短延遲，行為（哪些缺口仍會被標記進 IngestStats.EmptyAfterRetry）
// 完全不變，只是不再讓每個真缺口白等生產用的 5 秒 × emptyResponseMaxRetries 次。
const fileImportRetryDelay = time.Millisecond

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
	fs := flag.NewFlagSet("pricebook-import-raw", flag.ContinueOnError)
	legacyDir := fs.String("legacy-dir", "", "legacy 封存目錄（其下為 date=YYYY-MM-DD/data.json），必填")
	bronzeDir := fs.String("bronze-dir", "", "bronze 封存目錄（其下為 partition=YYYY-MM-DD/snapshot=*/payload.json），必填")
	startStr := fs.String("start", "", "匯入起始日期，ISO 格式 YYYY-MM-DD，必填")
	endStr := fs.String("end", "", "匯入結束日期（含），ISO 格式 YYYY-MM-DD，必填")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errFlagParseFailed
	}
	if *legacyDir == "" || *bronzeDir == "" || *startStr == "" || *endStr == "" {
		fs.Usage()
		return fmt.Errorf("--legacy-dir、--bronze-dir、--start、--end 皆為必填")
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

	zap.ReplaceGlobals(logger.NewLogger(cfg.Env))

	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	idGen := tsid.NewGenerator()
	client := importraw.NewFileClient(*legacyDir, *bronzeDir)
	priceIngest := usecase.NewPriceIngestWithClient(client, pool, idGen, fileImportRetryDelay)
	backfill := usecase.NewBackfill(priceIngest, pool, idGen)

	runID, err := backfill.Handle(ctx, start, end)
	if err != nil {
		if runID != 0 {
			return fmt.Errorf("匯入失敗（ingest_run.id=%d，狀態已標記為失敗）: %w", runID, err)
		}
		return fmt.Errorf("匯入失敗（尚未建立 ingest_run）: %w", err)
	}

	fmt.Printf("匯入完成，ingest_run.id=%d\n", runID)
	return nil
}
