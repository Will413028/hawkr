package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/social-service/internal/domain"
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
		fmt.Fprintln(os.Stderr, "用法：DATABASE_URL=<social DSN> SELLER_DATABASE_URL=<seller DSN> go run ./cmd/backfill-comment-seller-snapshot [-apply]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  DATABASE_URL：連線 hawkr_social（本服務自己的 DB，cms_comment 在這裡）")
		fmt.Fprintln(os.Stderr, "  SELLER_DATABASE_URL：唯讀連線 hawkr_seller（讀 sellers 表解析現任店名/Logo）")
		fmt.Fprintln(os.Stderr, "  未加 -apply：dry-run，只報告會更新幾筆，不寫入。")
	}
	flag.Parse()

	ctx := context.Background()

	socialDSN := os.Getenv("DATABASE_URL")
	if socialDSN == "" {
		return fmt.Errorf("錯誤：未設定 DATABASE_URL（hawkr_social，本服務自己的 DB）")
	}
	sellerDSN := os.Getenv("SELLER_DATABASE_URL")
	if sellerDSN == "" {
		return fmt.Errorf("錯誤：未設定 SELLER_DATABASE_URL（唯讀連線 hawkr_seller）")
	}

	socialPool, err := pgxpool.New(ctx, socialDSN)
	if err != nil {
		return fmt.Errorf("連線 hawkr_social 失敗: %w", err)
	}
	defer socialPool.Close()

	sellerPool, err := pgxpool.New(ctx, sellerDSN)
	if err != nil {
		return fmt.Errorf("連線 hawkr_seller 失敗: %w", err)
	}
	defer sellerPool.Close()

	role := domain.ActorKindStore

	brokenUserIDs, err := postgres.ListBrokenSellerCommentUserIDs(ctx, socialPool, role)
	if err != nil {
		return fmt.Errorf("查詢壞掉的留言 user_id 失敗: %w", err)
	}
	fmt.Printf("受影響的賣家 user_id 數：%d\n", len(brokenUserIDs))

	mapping, err := loadSellerSnapshotMapping(ctx, sellerPool, brokenUserIDs)
	if err != nil {
		return fmt.Errorf("解析賣家現任名稱/Logo 失敗: %w", err)
	}
	unresolved := len(brokenUserIDs) - len(mapping)
	for _, userID := range brokenUserIDs {
		if _, ok := mapping[userID]; !ok {
			fmt.Printf("  略過（無法解析，維持 NULL 待日後重試）：user_id=%d\n", userID)
		}
	}
	fmt.Printf("成功解析：%d，無法解析：%d\n", len(mapping), unresolved)

	mode := "DRY-RUN（未加 -apply，不寫入）"
	if *apply {
		mode = "APPLY（實際寫入）"
	}
	fmt.Printf("模式：%s\n", mode)

	if !*apply {
		fmt.Println("\n加 -apply 才會實際執行 UPDATE。")
		return nil
	}

	affected, err := postgres.BackfillCommentSnapshots(ctx, socialPool, role, mapping)
	if err != nil {
		return fmt.Errorf("backfill 失敗: %w", err)
	}
	fmt.Printf("\n已寫入 cms_comment 筆數：%d\n", affected)
	return nil
}

func loadSellerSnapshotMapping(ctx context.Context, pool *pgxpool.Pool, userIDs []int64) (map[int64]postgres.BrokenCommentSnapshot, error) {
	mapping := make(map[int64]postgres.BrokenCommentSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return mapping, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT DISTINCT input.id, cur.store_name, cur.logo
		FROM unnest($1::bigint[]) AS input(id)
		JOIN sellers s ON s.account_id = input.id OR s.store_id = input.id
		JOIN sellers cur ON cur.account_id = s.account_id AND cur.is_current = true`,
		userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		var logo *string
		if err := rows.Scan(&id, &name, &logo); err != nil {
			return nil, err
		}
		if name == "" {
			// store_name is NOT NULL in schema (required at onboarding) — this
			// should be unreachable, but stay symmetric with the "don't write
			// an ambiguous blank" rule the rest of this backfill follows.
			continue
		}
		snap := postgres.BrokenCommentSnapshot{UserName: name}
		if logo != nil {
			snap.UserAvatar = *logo
		}
		mapping[id] = snap
	}
	return mapping, rows.Err()
}
