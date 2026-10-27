package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BackfillStoreCodeFromMerchantCode copies payuni_registrations.merchant_code
// into sellers.store_code for every seller that already has a bound PayUni
// registration — that merchant_code is a live, bound MerID and must not
// change, so the display code has to match it exactly rather than getting a
// freshly minted value. Idempotent: only touches rows where store_code IS
// NULL. Returns the number of rows updated.
//
// Note: this function still references merchant_code because it runs in
// environments where the backfill hasn't completed yet. Task 6's migration
// (20260721062000_drop_payuni_merchant_code.sql) drops the column, but Task 4's
// NOT NULL constraint gates the migration pipeline, ensuring this backfill runs first.
func BackfillStoreCodeFromMerchantCode(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE sellers s
		SET store_code = p.merchant_code
		FROM payuni_registrations p
		WHERE p.store_id = s.store_id AND s.store_code IS NULL
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// BackfillFreshStoreCodes mints a store_code for every remaining
// store_code-IS-NULL seller (i.e. sellers with no payuni_registrations row —
// BackfillStoreCodeFromMerchantCode must run first). The generate parameter
// is injected (production call site passes domain.GenerateStoreCode) rather
// than called directly, so tests can force a collision or exhaust all retry
// attempts with a fake — mirroring the usecase layer's constructor-injected
// storeCode func() (int64, error) for the same reason. Called with the
// seller's own created_at (instead of time.Now()) so a backfilled code's
// year prefix reflects when the store actually opened. Retries per-row on
// unique-constraint collision. Returns the number of rows updated.
func BackfillFreshStoreCodes(ctx context.Context, pool *pgxpool.Pool, generate func(time.Time) (int64, error)) (int64, error) {
	rows, err := pool.Query(ctx, `SELECT store_id, created_at FROM sellers WHERE store_code IS NULL`)
	if err != nil {
		return 0, err
	}
	type target struct {
		storeID   int64
		createdAt time.Time
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.storeID, &t.createdAt); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	const maxAttempts = 5
	for _, t := range targets {
		var lastErr error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			// generate takes any time.Time — passing the seller's own
			// created_at (instead of time.Now()) makes a backfilled code's
			// year prefix reflect when the store actually opened, with no
			// separate implementation needed.
			code, genErr := generate(t.createdAt)
			if genErr != nil {
				return 0, genErr
			}
			tag, execErr := pool.Exec(ctx, `UPDATE sellers SET store_code = $1 WHERE store_id = $2 AND store_code IS NULL`, code, t.storeID)
			if execErr == nil && tag.RowsAffected() == 1 {
				lastErr = nil
				break
			}
			lastErr = execErr
			if lastErr == nil {
				lastErr = fmt.Errorf("update affected %d rows, want 1 (store_code collision)", tag.RowsAffected())
			}
		}
		if lastErr != nil {
			return 0, fmt.Errorf("store_id=%d: %w", t.storeID, lastErr)
		}
	}
	return int64(len(targets)), nil
}

// VerifyStoreCodeBackfillComplete errors if any sellers row still has a NULL
// store_code, or if any two rows share a store_code value. Callers must run
// this — and get a nil error — before adding the NOT NULL constraint.
func VerifyStoreCodeBackfillComplete(ctx context.Context, pool *pgxpool.Pool) error {
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM sellers WHERE store_code IS NULL`).Scan(&remaining); err != nil {
		return err
	}
	if remaining != 0 {
		return fmt.Errorf("backfill incomplete: %d sellers rows still have NULL store_code", remaining)
	}

	var total, distinct int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(DISTINCT store_code) FROM sellers`).Scan(&total, &distinct); err != nil {
		return err
	}
	if total != distinct {
		return fmt.Errorf("backfill produced duplicates: %d rows but only %d distinct store_code values", total, distinct)
	}
	return nil
}
