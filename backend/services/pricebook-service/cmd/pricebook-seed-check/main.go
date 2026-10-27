// Command pricebook-seed-check 對 DB 驗證
// reference/product_crop_seed.csv（Task 13 的 mapping 種子對照表）的解析率：
// 逐列檢查 crop_group_name 是否真的存在於 crop_group.name、crop_code 是否
// 存在於 crop.crop_code、market_code（若有填）是否存在於 market.market_code，
// 輸出每一列的結果與整體解析率摘要。
//
// 這是一支診斷 / 稽核工具，不是部署閘門：即使有列 NOT_FOUND 或 AMBIGUOUS，
// CLI 仍以 exit 0 結束——它的價值在於把「這份手工策劃的種子表，此刻在這個
// 環境的 DB 裡到底對不對得上」攤開給人看，讓人決定要不要修 CSV，而不是自動
// 幫忙決定或擋任何流程。
//
// 🔴 FindCropGroupByName 回多筆時不會被算進「解析成功」，會被標記
// AMBIGUOUS 並把命中的全部列印出來：Task 2 之後 crop_group.group_code 的 hash
// 含 category_code，同一個大類名在不同 category 下是不同的列（例如「甘藍」
// 理論上可能同時存在於 N04 蔬菜與 N05 水果）。單靠名稱查不到唯一列時，這支
// CLI 存在的目的就是把這個歧義攤開，不是自動選一筆——選錯會讓賣家的定價
// 建議指向錯誤的市場行情。
//
// 比照本 repo 既有同類 CLI（cmd/pricebook-backfill、cmd/pricebook-export）：
// 不進 Docker image、不啟動 fx.App，DB 設定走與主服務相同的
// internal/config.Load()。
//
// Usage（--csv/--manifest 預設相對於 services/pricebook-service 根目錄，
// 對齊 reference/product_crop_seed.csv 實際存放位置；在服務根目錄執行時可
// 省略）：
//
//	DB_HOST=<dev DB host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
//	  go run ./cmd/pricebook-seed-check
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// errFlagParseFailed 是 run 內部的 sentinel：flag.ContinueOnError 模式下
// fs.Parse 失敗時已經自己把錯誤訊息與 usage 印到 stderr 一次，main() 用這個
// sentinel 判斷「這個錯誤不用再印第二次」，只需要以非 0 結束（比照
// cmd/pricebook-export/main.go 同名 sentinel 的說明；每支 CLI 各自獨立宣告，
// package main 之間不共用檔案作用域外的常數）。
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
	fs := flag.NewFlagSet("pricebook-seed-check", flag.ContinueOnError)
	csvPath := fs.String("csv", "reference/product_crop_seed.csv", "種子表 CSV 路徑")
	manifestPath := fs.String("manifest", "reference/product_crop_seed.manifest.json", "種子表 manifest 路徑")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errFlagParseFailed
	}

	// LoadSeed 的 digest 檢查在這裡就會硬性擋下——種子表被改過而 manifest
	// 沒同步更新時，這支 CLI 拒絕往下跑，不會拿一份可能被繞過審查的資料去
	// 產生「看起來正常」的解析率報告。
	seed, err := mapping.LoadSeed(*csvPath, *manifestPath)
	if err != nil {
		return fmt.Errorf("載入種子表失敗: %w", err)
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

	db := &pgSeedCheckDB{q: gen.New(pool), pool: pool}

	resolved, err := printReport(ctx, os.Stdout, db, seed)
	if err != nil {
		return err
	}

	fmt.Printf("\n解析成功 %d/%d 列\n", resolved, seed.Len())
	return nil
}

// rowStatus 是單一種子列對 DB 的解析結果分類。三態而非布林：AMBIGUOUS 與
// NOT_FOUND 都不算「解析成功」，但兩者需要的後續動作不同（前者要人工選一筆，
// 後者要人工確認種子表是否寫錯或 DB 是否漏了這個作物），列印時分開標記。
type rowStatus string

const (
	statusResolved  rowStatus = "OK"
	statusNotFound  rowStatus = "NOT_FOUND"
	statusAmbiguous rowStatus = "AMBIGUOUS"
)

// seedCheckDB 是 printReport 對 DB 的全部依賴，故意收斂成三個方法：讓
// checkRow／printReport 可以用假實作單元測試（尤其是 AMBIGUOUS 分支——不需要
// 真的種兩筆同名不同 category 的 crop_group 到一個 testcontainer 裡才能驗證
// 「印出全部＋標記需人工裁決」這件事）。*gen.Queries 結構性地滿足前兩個
// 方法；MarketExists 由 pgSeedCheckDB 用 ad hoc SQL 補上（見該方法註解）。
type seedCheckDB interface {
	FindCropGroupByName(ctx context.Context, name string) ([]gen.FindCropGroupByNameRow, error)
	FindCropByCode(ctx context.Context, cropCode string) (gen.FindCropByCodeRow, error)
	MarketExists(ctx context.Context, marketCode string) (bool, error)
}

type pgSeedCheckDB struct {
	q    *gen.Queries
	pool *pgxpool.Pool
}

func (d *pgSeedCheckDB) FindCropGroupByName(ctx context.Context, name string) ([]gen.FindCropGroupByNameRow, error) {
	return d.q.FindCropGroupByName(ctx, name)
}

func (d *pgSeedCheckDB) FindCropByCode(ctx context.Context, cropCode string) (gen.FindCropByCodeRow, error) {
	return d.q.FindCropByCode(ctx, cropCode)
}

// MarketExists 刻意不透過 sqlc：Task 13 的範圍只授權新增兩支 sqlc 查詢
// （FindCropGroupByName／FindCropByCode，見 db/queries/forecast.sql），
// market_code 存在性檢查用最小的 ad hoc SQL 直接查一個 EXISTS，比照本 repo
// 既有整合測試（internal/repository/postgres/dbtest_test.go 一類）直接對
// pool 下 SQL 字串的既有做法，不為了一個布林值再多開一支 generated query。
func (d *pgSeedCheckDB) MarketExists(ctx context.Context, marketCode string) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM market WHERE market_code = $1)`, marketCode).Scan(&exists)
	return exists, err
}

// rowResult 是 checkRow 對單一種子列的檢查結果，供 printReport 逐列輸出。
type rowResult struct {
	Entry  mapping.SeedEntry
	Target string // "crop_group:甘藍" 或 "crop:LA1"
	Status rowStatus
	Detail string // 命中列的 group_code/category_code，或 AMBIGUOUS 時列出全部候選
}

// checkRow 對單一種子列查 DB，回傳分類結果。CropGroupName／CropCode 恰有
// 一個非空是 LoadSeed 已保證的不變量，default 分支只是防禦性檢查。
func checkRow(ctx context.Context, db seedCheckDB, entry mapping.SeedEntry) (rowResult, error) {
	res := rowResult{Entry: entry}

	switch {
	case entry.CropGroupName != "":
		res.Target = "crop_group:" + entry.CropGroupName
		rows, err := db.FindCropGroupByName(ctx, entry.CropGroupName)
		if err != nil {
			return rowResult{}, fmt.Errorf("查詢 crop_group %q 失敗: %w", entry.CropGroupName, err)
		}
		res.Status, res.Detail = classifyGroupRows(rows)

	case entry.CropCode != "":
		res.Target = "crop:" + entry.CropCode
		row, err := db.FindCropByCode(ctx, entry.CropCode)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			res.Status = statusNotFound
		case err != nil:
			return rowResult{}, fmt.Errorf("查詢 crop %q 失敗: %w", entry.CropCode, err)
		default:
			res.Status = statusResolved
			res.Detail = fmt.Sprintf("name=%s category=%s", row.Name, textOrDash(row.CategoryCode))
		}

	default:
		return rowResult{}, fmt.Errorf("alias_name=%s 既無 crop_group_name 也無 crop_code（LoadSeed 應已擋下這種列，代表不變量被破壞）", entry.AliasName)
	}

	// market_code 是可選欄位；只有在主目標（crop_group 或 crop）已唯一解出、
	// 且這一列確實指定了 market_code 時才需要再查一次。指定了但查無這個
	// 市場，代表本列整體仍不算「解析成功」，即使作物本身對得上。
	if res.Status == statusResolved && entry.MarketCode != "" {
		ok, err := db.MarketExists(ctx, entry.MarketCode)
		if err != nil {
			return rowResult{}, fmt.Errorf("查詢 market %q 失敗: %w", entry.MarketCode, err)
		}
		if !ok {
			res.Status = statusNotFound
			res.Detail += fmt.Sprintf("；market_code=%s 不存在於 market 表", entry.MarketCode)
		}
	}

	return res, nil
}

// classifyGroupRows 依 FindCropGroupByName 回傳的列數分三態。
func classifyGroupRows(rows []gen.FindCropGroupByNameRow) (rowStatus, string) {
	switch len(rows) {
	case 0:
		return statusNotFound, ""
	case 1:
		r := rows[0]
		return statusResolved, fmt.Sprintf("group_code=%s category=%s", r.GroupCode, textOrDash(r.CategoryCode))
	default:
		parts := make([]string, 0, len(rows))
		for _, r := range rows {
			parts = append(parts, fmt.Sprintf("group_code=%s category=%s", r.GroupCode, textOrDash(r.CategoryCode)))
		}
		return statusAmbiguous, fmt.Sprintf("命中 %d 筆，需人工裁決: [%s]", len(rows), strings.Join(parts, "; "))
	}
}

func textOrDash(t *string) string {
	if t == nil {
		return "-"
	}
	return *t
}

// printReport 逐列印出 alias_name | match_kind | target | resolved? | detail，
// 回傳解析成功（statusResolved）的列數供摘要行使用。out 收 io.Writer 而不是
// 直接綁 *os.File：production 路徑傳 os.Stdout，測試路徑傳 *bytes.Buffer
// 斷言輸出內容，兩者不需要各寫一份邏輯。
func printReport(ctx context.Context, out io.Writer, db seedCheckDB, seed mapping.Seed) (resolved int, err error) {
	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	// tabwriter.Writer.Write 的錯誤只會在底層 out 寫入失敗時發生（例如管線另一端
	// 已關閉），buffer 本身不會累積成錯誤狀態；真正的寫入結果由後面的 w.Flush()
	// 統一檢查並回傳，這裡比照 ai-service seller_assistant.go 的既有做法忽略單次
	// 呼叫的回傳值，不是漏檢查。
	_, _ = fmt.Fprintln(w, "alias_name\tmatch_kind\ttarget\tresolved?\tdetail")
	for _, entry := range seed.Entries() {
		res, err := checkRow(ctx, db, entry)
		if err != nil {
			return resolved, fmt.Errorf("檢查 alias_name=%s 失敗: %w", entry.AliasName, err)
		}
		if res.Status == statusResolved {
			resolved++
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.AliasName, entry.MatchKind, res.Target, res.Status, res.Detail)
	}
	if err := w.Flush(); err != nil {
		return resolved, fmt.Errorf("輸出報告失敗: %w", err)
	}
	return resolved, nil
}
