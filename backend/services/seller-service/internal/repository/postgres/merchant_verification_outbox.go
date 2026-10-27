package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type merchantVerificationOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewMerchantVerificationOutboxRepository 建立 repo。
func NewMerchantVerificationOutboxRepository(pool *pgxpool.Pool) *merchantVerificationOutboxRepository {
	return &merchantVerificationOutboxRepository{pool: pool}
}

// getQueries returns a Queries instance using transaction from context if available.
func (r *merchantVerificationOutboxRepository) getQueries(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// Insert persists a new merchant_verification_outbox row and refreshes the
// entity's timestamps from the returned row (server defaults applied when
// zero). Participates in caller's tx.
func (r *merchantVerificationOutboxRepository) Insert(ctx context.Context, row *domain.MerchantVerificationOutbox) error {
	q := r.getQueries(ctx)
	out, err := q.InsertMerchantVerificationOutbox(ctx, gen.InsertMerchantVerificationOutboxParams{
		ID:            row.ID,
		AccountID:     row.AccountID,
		EventType:     row.EventType,
		Reason:        nullableText(row.Reason),
		Payload:       row.Payload,
		NextAttemptAt: timestamptzParam(row.NextAttemptAt),
		CreatedAt:     timestamptzParam(row.CreatedAt),
	})
	if err != nil {
		return err
	}

	row.CreatedAt = out.CreatedAt.Time
	row.NextAttemptAt = out.NextAttemptAt.Time

	return nil
}

// FetchPendingDue returns up to limit pending outbox rows whose next_attempt_at <= now.
//
// This is only the candidate query — it grants no exclusivity, and two replicas
// ticking together will get the same rows. Cross-replica exclusivity comes from
// Claim. It deliberately does NOT use FOR UPDATE SKIP LOCKED: this runs under
// autocommit, so the row locks would be released the instant the SELECT returns.
func (r *merchantVerificationOutboxRepository) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*domain.MerchantVerificationOutbox, error) {
	q := r.getQueries(ctx)
	rows, err := q.FetchPendingMerchantVerificationOutbox(ctx, gen.FetchPendingMerchantVerificationOutboxParams{
		NextAttemptAt: timestamptzParam(now),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*domain.MerchantVerificationOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, merchantVerificationOutboxToDomain(row))
	}
	return out, nil
}

// Claim is the lease CAS preemption gate — see domain.MerchantVerificationOutboxRepository.
func (r *merchantVerificationOutboxRepository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	q := r.getQueries(ctx)
	return q.ClaimMerchantVerificationOutbox(ctx, gen.ClaimMerchantVerificationOutboxParams{
		ID:              id,
		LeaseUntil:      timestamptzParam(leaseUntil),
		ClaimableBefore: timestamptzParam(claimableBefore),
	})
}

// UpdateAttempt records a retry attempt: bumps attempt_count, schedules next_attempt_at,
// and stores last_error (empty string is persisted as NULL).
func (r *merchantVerificationOutboxRepository) UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error {
	q := r.getQueries(ctx)
	return q.UpdateMerchantVerificationOutboxAttempt(ctx, gen.UpdateMerchantVerificationOutboxAttemptParams{
		ID:            id,
		AttemptCount:  int32(attempt),
		NextAttemptAt: timestamptzParam(nextAt),
		LastError:     nullableText(lastErr),
	})
}

// MarkDone transitions the row to status='done' and stamps completed_at.
func (r *merchantVerificationOutboxRepository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkMerchantVerificationOutboxDone(ctx, gen.MarkMerchantVerificationOutboxDoneParams{
		ID:          id,
		CompletedAt: nullableTime(now),
	})
}

// MarkFailed transitions the row to status='failed', stores last_error, and stamps completed_at.
func (r *merchantVerificationOutboxRepository) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkMerchantVerificationOutboxFailed(ctx, gen.MarkMerchantVerificationOutboxFailedParams{
		ID:          id,
		LastError:   nullableText(lastErr),
		CompletedAt: nullableTime(now),
	})
}

// CountPending returns the number of outbox rows in 'pending' status.
func (r *merchantVerificationOutboxRepository) CountPending(ctx context.Context) (int, error) {
	q := r.getQueries(ctx)
	count, err := q.CountPendingMerchantVerificationOutbox(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// OldestPendingAge returns the age of the oldest pending row's created_at
// relative to now. ok is false when there are no pending rows.
func (r *merchantVerificationOutboxRepository) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	q := r.getQueries(ctx)
	createdAt, err := q.OldestPendingMerchantVerificationOutboxCreatedAt(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return now.Sub(createdAt.Time), true, nil
}

// DeleteDoneBefore deletes up to limit 'done' rows completed before cutoff.
func (r *merchantVerificationOutboxRepository) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	q := r.getQueries(ctx)
	n, err := q.DeleteDoneMerchantVerificationOutboxBefore(ctx, gen.DeleteDoneMerchantVerificationOutboxBeforeParams{
		Cutoff:   nullableTime(cutoff),
		RowLimit: int32(limit),
	})
	return int(n), err
}

// merchantVerificationOutboxToDomain maps a gen row to the domain entity.
func merchantVerificationOutboxToDomain(row gen.MerchantVerificationOutbox) *domain.MerchantVerificationOutbox {
	return &domain.MerchantVerificationOutbox{
		ID:            row.ID,
		AccountID:     row.AccountID,
		EventType:     row.EventType,
		Reason:        stringValue(row.Reason),
		Payload:       row.Payload,
		Status:        domain.MerchantVerificationOutboxStatus(row.Status),
		AttemptCount:  int(row.AttemptCount),
		NextAttemptAt: row.NextAttemptAt.Time,
		LastError:     stringValue(row.LastError),
		CreatedAt:     row.CreatedAt.Time,
		CompletedAt:   row.CompletedAt,
	}
}
