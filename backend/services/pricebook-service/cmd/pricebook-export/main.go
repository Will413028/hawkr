// Command pricebook-export 匯出 [start, end] 區間內每個 (market, crop,
// trade_date) 目前最新 revision 的行情觀測，寫成確定性 CSV
// （usecase.Exporter.ObservationCSV 的手動觸發入口，Task 9：訓練資料對帳
// gate，見 task-9-brief.md 與 decisions.md）。
//
// 這份匯出唯一的用途是跟 資料團隊獨立產生的同格式 Silver 匯出逐筆 diff，藉此
// 判斷本服務重抓 MOA 資料的修訂選取與缺值處理，跟現有模型訓練資料的語意是否
// 一致——不一致會讓模型的 feature_mean/feature_scale 對不上、推論系統性失準
// 且不會報錯（train/serve skew）。CSV 格式本身（欄位、排序、小數位、缺值
// 一律空字串不寫 0）就是這個對帳能不能成立的前提，見
// internal/usecase/export.go 的 WriteObservationCSV。
//
// 比照本 repo 既有的同類 CLI（cmd/pricebook-backfill、
// services/account-service/cmd/auth-backfill、
// services/seller-service/cmd/backfillstorecode）：preflight §C Task 9 / B20
// 明確指出這支 binary 不進 Docker image（Dockerfile 的 builder stage 只建
// ./cmd/pricebook-service），從有網路可達目標 Postgres 的機器上直接 `go run`
// 執行；DB/MOA 設定走與主服務相同的 internal/config.Load()（讀環境變數，
// 未設時用開發預設值），不啟動 fx.App——手動組出 Exporter 需要的唯一依賴
// （*pgxpool.Pool）即可，比照 pricebook-backfill 不透過 fx 的做法。
//
// Usage（--start/--end/--out 是本 CLI 自己的契約：日期是 ISO 格式
// YYYY-MM-DD，不是 MOA 的民國格式）：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-export --start 2026-04-28 --end 2026-07-26 --out /tmp/ours.csv
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// dateLayout 是本 CLI 對 --start/--end 的契約：ISO YYYY-MM-DD。跟
// cmd/pricebook-backfill 的 dateLayout 同構但獨立宣告（各 CLI 是各自獨立的
// package main，無法共用彼此的檔案作用域內常數）。
const dateLayout = "2006-01-02"

// errFlagParseFailed 是 run 內部的 sentinel：flag.ContinueOnError 模式下
// fs.Parse 失敗時已經自己把錯誤訊息與 usage 印到 stderr 一次，main() 用這個
// sentinel 判斷「這個錯誤不用再印第二次」，只需要以非 0 結束（M6 的同一個
// 問題也出現在這支 CLI，final whole-branch review 一併修正，見
// cmd/pricebook-backfill/main.go 同名 sentinel 的說明）。
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
	fs := flag.NewFlagSet("pricebook-export", flag.ContinueOnError)
	startStr := fs.String("start", "", "匯出起始日期（含），ISO 格式 YYYY-MM-DD，必填")
	endStr := fs.String("end", "", "匯出結束日期（含），ISO 格式 YYYY-MM-DD，必填")
	outPath := fs.String("out", "", "輸出 CSV 檔案路徑，必填")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// -h/--help：fs.Parse 已經印出 usage，視為正常結束（exit 0）。
			return nil
		}
		return errFlagParseFailed
	}
	if *startStr == "" || *endStr == "" || *outPath == "" {
		fs.Usage()
		return fmt.Errorf("--start、--end、--out 皆為必填")
	}

	start, err := time.Parse(dateLayout, *startStr)
	if err != nil {
		return fmt.Errorf("--start 格式錯誤（需 YYYY-MM-DD）: %w", err)
	}
	end, err := time.Parse(dateLayout, *endStr)
	if err != nil {
		return fmt.Errorf("--end 格式錯誤（需 YYYY-MM-DD）: %w", err)
	}
	if end.Before(start) {
		return fmt.Errorf("匯出區間反轉：--start=%s 晚於 --end=%s", *startStr, *endStr)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("載入設定失敗: %w", err)
	}

	// database.NewPool（而非裸的 pgxpool.New）：注入 QueryExecModeExec（PgBouncer
	// transaction-pooling 相容，見 pkg/database/postgres.go）、connect_timeout=10、
	// 建構時 Ping，比照 internal/config/di.go 的 newPool 呼叫方式。
	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("連線資料庫失敗: %w", err)
	}
	defer pool.Close()

	// 先在目的檔的同一個目錄建暫存檔、成功才 rename 到 --out：避免匯出中途
	// 失敗（DB 斷線、磁碟滿）時在 --out 留下一份不完整、看起來存在但內容截斷
	// 的 CSV——對帳流程若誤把半成品當完整匯出去 diff，會產生一堆假的「我方
	// 缺列」差異。暫存檔刻意放在 filepath.Dir(*outPath) 而非系統 tmp 目錄：
	// os.Rename 跨檔案系統會回 EXDEV 失敗，同目錄可保證 rename 是同一個
	// filesystem 上的原子操作。
	outDir := filepath.Dir(*outPath)
	tmp, err := os.CreateTemp(outDir, ".pricebook-export-*.csv.tmp")
	if err != nil {
		return fmt.Errorf("在 %s 建立暫存檔失敗: %w", outDir, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // rename 成功後這裡會找不到檔案，Remove 靜默失敗，無妨。

	exporter := usecase.NewExporter(pool)
	rowCount, err := exporter.ObservationCSV(ctx, start, end, tmp)
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("匯出對帳觀測失敗: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("寫入暫存檔失敗: %w", err)
	}
	if err := os.Rename(tmpPath, *outPath); err != nil {
		return fmt.Errorf("寫入 --out 失敗: %w", err)
	}

	fmt.Printf("匯出完成：%d 列，%s ~ %s → %s\n", rowCount, *startStr, *endStr, *outPath)
	if rowCount == 0 {
		// 不是硬錯誤：週末等純休市區間本就可能真的 0 列（合法輸入）。但
		// WriteObservationCSV 一律先寫表頭，0 列匯出跟正常匯出一樣是「看起來
		// 有效」的 1 行 CSV，操作者拿去跟 資料團隊的 Silver 匯出 diff 前必須先看到
		// 這個警告，否則會誤讀成「我方資料完全缺失」而推向錯誤、昂貴的 Plan B
		// 決策（final whole-branch review Fix 2）。
		fmt.Fprintf(os.Stderr, "警告：匯出 0 列，請確認 --start/--end 區間內本應有交易資料\n")
	}
	return nil
}
