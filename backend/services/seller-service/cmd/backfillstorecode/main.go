// Command backfillstorecode is a one-time, idempotent job: assigns
// sellers.store_code to every row where it's still NULL, per the design spec
// (2026-07-21-store-code-design.md). Safe to re-run.
//
// Run once per environment before the store_code NOT NULL migration lands:
//
//	DATABASE_URL=<dev DB DSN> go run ./cmd/backfillstorecode
//
// Note: this job references payuni_registrations.merchant_code, which Task 6's
// migration drops. However, the job targets environments where the backfill
// hasn't run yet; the NOT NULL constraint gates the pipeline, ensuring this
// job completes before that environment reaches the DROP COLUMN migration.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	fromMerchantCode, err := postgres.BackfillStoreCodeFromMerchantCode(ctx, pool)
	if err != nil {
		return fmt.Errorf("backfill from merchant_code: %w", err)
	}
	fmt.Printf("backfilled %d sellers rows from existing payuni_registrations.merchant_code\n", fromMerchantCode)

	fresh, err := postgres.BackfillFreshStoreCodes(ctx, pool, domain.GenerateStoreCode)
	if err != nil {
		return fmt.Errorf("backfill fresh codes: %w", err)
	}
	fmt.Printf("backfilled %d sellers rows with freshly minted store_code\n", fresh)

	if err := postgres.VerifyStoreCodeBackfillComplete(ctx, pool); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	fmt.Println("backfill complete: all sellers rows have a distinct store_code")
	return nil
}
