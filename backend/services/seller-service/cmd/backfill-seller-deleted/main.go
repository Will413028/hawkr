// Command backfill-seller-deleted is a one-off job that re-emits seller.deleted
// events for sellers that were already soft-deleted before the off-shelf pipeline
// existed. product-service consumes these and off-shelves the sellers' still-ACTIVE
// products (idempotent: re-emitting for an already-processed seller is a no-op since
// off-shelf only touches ACTIVE products).
//
// Run once per environment after deploy:
//
//	DB_HOST=... DB_PASSWORD=... RABBITMQ_URL=... go run ./cmd/backfill-seller-deleted
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	pub, err := pkgmq.NewPublisher(cfg.RabbitMQ.URL)
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	defer func() { _ = pub.Close() }()

	// All soft-deleted sellers (deleted_at covers rows predating the status column).
	rows, err := pool.Query(ctx,
		`SELECT account_id FROM sellers WHERE deleted_at IS NOT NULL ORDER BY account_id`)
	if err != nil {
		return fmt.Errorf("query deleted sellers: %w", err)
	}
	defer rows.Close()

	var accountIDs []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		accountIDs = append(accountIDs, accountID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	log.Printf("found %d soft-deleted sellers; publishing seller.deleted...", len(accountIDs))

	var published, failed int
	for _, accountID := range accountIDs {
		event := &pkgmq.SellerDeletedEvent{
			BaseEvent: pkgmq.NewBaseEvent(pkgmq.EventTypeSellerDeleted, "seller-service-backfill"),
			AccountID: accountID,
		}
		if err := pub.PublishSellerDeleted(ctx, event); err != nil {
			log.Printf("publish failed accountID=%d: %v", accountID, err)
			failed++
			continue
		}
		published++
	}

	log.Printf("backfill done: published=%d failed=%d total=%d", published, failed, len(accountIDs))
	if failed > 0 {
		return fmt.Errorf("%d publishes failed; re-run to retry (idempotent)", failed)
	}
	return nil
}
