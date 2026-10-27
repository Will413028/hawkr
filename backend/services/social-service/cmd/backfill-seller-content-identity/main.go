package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "actually write; without this flag, only reports what WOULD be updated")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：SELLER_DATABASE_URL=<seller DSN> DATABASE_URL=<social DSN> go run ./cmd/backfill-seller-content-identity [-apply]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  SELLER_DATABASE_URL：唯讀連線 hawkr_seller（讀 sellers 表判斷 single-stint）")
		fmt.Fprintln(os.Stderr, "  DATABASE_URL：連線 hawkr_social（本服務自己的 DB，實際寫入的 6 個欄位都在這裡）")
		fmt.Fprintln(os.Stderr, "  未加 -apply：dry-run，只報告會更新幾筆，不寫入。")
	}
	flag.Parse()

	ctx := context.Background()

	sellerDSN := os.Getenv("SELLER_DATABASE_URL")
	if sellerDSN == "" {
		return fmt.Errorf("錯誤：未設定 SELLER_DATABASE_URL（唯讀連線 hawkr_seller）")
	}
	socialDSN := os.Getenv("DATABASE_URL")
	if socialDSN == "" {
		return fmt.Errorf("錯誤：未設定 DATABASE_URL（hawkr_social，本服務自己的 DB）")
	}

	sellerPool, err := pgxpool.New(ctx, sellerDSN)
	if err != nil {
		return fmt.Errorf("連線 hawkr_seller 失敗: %w", err)
	}
	defer sellerPool.Close()

	socialPool, err := pgxpool.New(ctx, socialDSN)
	if err != nil {
		return fmt.Errorf("連線 hawkr_social 失敗: %w", err)
	}
	defer socialPool.Close()

	mapping, multiStintCount, err := loadSingleStintMapping(ctx, sellerPool)
	if err != nil {
		return fmt.Errorf("讀取 single-stint 賣家對照表失敗: %w", err)
	}
	fmt.Printf("single-stint 賣家（可回填）：%d\n", len(mapping))
	fmt.Printf("multi-stint 賣家（重開過店，無法回推寫入當下是哪個 stint，略過）：%d\n", multiStintCount)

	mode := "DRY-RUN（未加 -apply，不寫入）"
	if *apply {
		mode = "APPLY（實際寫入）"
	}
	fmt.Printf("模式：%s\n", mode)

	if !*apply {
		fmt.Println("\n加 -apply 才會實際執行 UPDATE。")
		return nil
	}

	result, err := postgres.BackfillSingleStintSellerIdentity(ctx, socialPool, mapping)
	if err != nil {
		return fmt.Errorf("backfill 失敗: %w", err)
	}
	fmt.Println("\n已寫入：")
	fmt.Printf("  cms_comment.user_id       %d\n", result.CmsCommentUserID)
	fmt.Printf("  cms_seller_forum.user_id  %d\n", result.CmsSellerForumUserID)
	fmt.Printf("  reports.reporter_id       %d\n", result.ReportsReporterID)
	fmt.Printf("  user_blocks.blocker_id    %d\n", result.UserBlocksBlockerID)
	fmt.Printf("  合計                      %d\n", result.Total())
	return nil
}

func loadSingleStintMapping(ctx context.Context, pool *pgxpool.Pool) (postgres.SingleStintSellerMapping, int, error) {
	rows, err := pool.Query(ctx, `
		SELECT account_id, store_id, stint_count FROM (
			SELECT account_id, store_id,
			       COUNT(*) OVER (PARTITION BY account_id) AS stint_count
			FROM sellers
		) s
	`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	mapping := postgres.SingleStintSellerMapping{}
	multiStintAccounts := map[int64]bool{}
	for rows.Next() {
		var accountID, storeID, stintCount int64
		if err := rows.Scan(&accountID, &storeID, &stintCount); err != nil {
			return nil, 0, err
		}
		if stintCount == 1 {
			mapping[accountID] = storeID
		} else {
			multiStintAccounts[accountID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return mapping, len(multiStintAccounts), nil
}
