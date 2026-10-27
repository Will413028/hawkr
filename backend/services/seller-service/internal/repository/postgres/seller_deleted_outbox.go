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

type sellerDeletedOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewSellerDeletedOutboxRepository 建立 repo。
func NewSellerDeletedOutboxRepository(pool *pgxpool.Pool) *sellerDeletedOutboxRepository {
	return &sellerDeletedOutboxRepository{pool: pool}
}

// getQueries returns a Queries instance using transaction from context if available.
func (r *sellerDeletedOutboxRepository) getQueries(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// Insert persists a new seller_deleted_outbox row and refreshes the entity's timestamps
// from the returned row (server defaults applied when zero). Participates in caller's tx.
func (r *sellerDeletedOutboxRepository) Insert(ctx context.Context, row *domain.SellerDeletedOutbox) error {
	q := r.getQueries(ctx)
	out, err := q.InsertSellerDeletedOutbox(ctx, gen.InsertSellerDeletedOutboxParams{
		ID:            row.ID,
		AccountID:     row.AccountID,
		EventType:     row.EventType,
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
// Claim (issue #2294). It deliberately does NOT use FOR UPDATE SKIP LOCKED:
// this runs under autocommit, so the row locks would be released the instant
// the SELECT returns.
func (r *sellerDeletedOutboxRepository) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*domain.SellerDeletedOutbox, error) {
	q := r.getQueries(ctx)
	rows, err := q.FetchPendingSellerDeletedOutbox(ctx, gen.FetchPendingSellerDeletedOutboxParams{
		NextAttemptAt: timestamptzParam(now),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*domain.SellerDeletedOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, sellerDeletedOutboxToDomain(row))
	}
	return out, nil
}

// Claim is the lease CAS preemption gate — see domain.SellerDeletedOutboxRepository.
func (r *sellerDeletedOutboxRepository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	q := r.getQueries(ctx)
	return q.ClaimSellerDeletedOutbox(ctx, gen.ClaimSellerDeletedOutboxParams{
		ID:              id,
		LeaseUntil:      timestamptzParam(leaseUntil),
		ClaimableBefore: timestamptzParam(claimableBefore),
	})
}

// UpdateAttempt records a retry attempt: bumps attempt_count, schedules next_attempt_at,
// and stores last_error (empty string is persisted as NULL).
func (r *sellerDeletedOutboxRepository) UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error {
	q := r.getQueries(ctx)
	return q.UpdateSellerDeletedOutboxAttempt(ctx, gen.UpdateSellerDeletedOutboxAttemptParams{
		ID:            id,
		AttemptCount:  int32(attempt),
		NextAttemptAt: timestamptzParam(nextAt),
		LastError:     nullableText(lastErr),
	})
}

// CountPending returns the number of outbox rows in 'pending' status.
func (r *sellerDeletedOutboxRepository) CountPending(ctx context.Context) (int, error) {
	q := r.getQueries(ctx)
	count, err := q.CountPendingSellerDeletedOutbox(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// OldestPendingAge returns the age of the oldest pending row's created_at
// relative to now. ok is false when there are no pending rows.
func (r *sellerDeletedOutboxRepository) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	q := r.getQueries(ctx)
	createdAt, err := q.OldestPendingSellerDeletedOutboxCreatedAt(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return now.Sub(createdAt.Time), true, nil
}

// DeleteDoneBefore deletes up to limit 'done' rows completed before cutoff.
func (r *sellerDeletedOutboxRepository) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	q := r.getQueries(ctx)
	n, err := q.DeleteDoneSellerDeletedOutboxBefore(ctx, gen.DeleteDoneSellerDeletedOutboxBeforeParams{
		Cutoff:   nullableTime(cutoff),
		RowLimit: int32(limit),
	})
	return int(n), err
}

// MarkDone transitions the row to status='done' and stamps completed_at.
func (r *sellerDeletedOutboxRepository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkSellerDeletedOutboxDone(ctx, gen.MarkSellerDeletedOutboxDoneParams{
		ID:          id,
		CompletedAt: nullableTime(now),
	})
}

// MarkFailed transitions the row to status='failed', stores last_error, and stamps completed_at.
func (r *sellerDeletedOutboxRepository) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkSellerDeletedOutboxFailed(ctx, gen.MarkSellerDeletedOutboxFailedParams{
		ID:          id,
		LastError:   nullableText(lastErr),
		CompletedAt: nullableTime(now),
	})
}

// MarkEventPublished stamps event_published_at after a successful seller.deleted publish.
func (r *sellerDeletedOutboxRepository) MarkEventPublished(ctx context.Context, id int64, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkSellerDeletedOutboxEventPublished(ctx, gen.MarkSellerDeletedOutboxEventPublishedParams{
		ID:               id,
		EventPublishedAt: nullableTime(now),
	})
}

// MarkRoleRemoved stamps role_removed_at after a successful RemoveSellerRole call.
func (r *sellerDeletedOutboxRepository) MarkRoleRemoved(ctx context.Context, id int64, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkSellerDeletedOutboxRoleRemoved(ctx, gen.MarkSellerDeletedOutboxRoleRemovedParams{
		ID:            id,
		RoleRemovedAt: nullableTime(now),
	})
}

// sellerDeletedOutboxToDomain maps a gen row to the domain entity.
func sellerDeletedOutboxToDomain(row gen.SellerDeletedOutbox) *domain.SellerDeletedOutbox {
	return &domain.SellerDeletedOutbox{
		ID:               row.ID,
		AccountID:        row.AccountID,
		EventType:        row.EventType,
		Payload:          row.Payload,
		Status:           domain.SellerDeletedOutboxStatus(row.Status),
		AttemptCount:     int(row.AttemptCount),
		NextAttemptAt:    row.NextAttemptAt.Time,
		LastError:        stringValue(row.LastError),
		CreatedAt:        row.CreatedAt.Time,
		CompletedAt:      row.CompletedAt,
		EventPublishedAt: row.EventPublishedAt,
		RoleRemovedAt:    row.RoleRemovedAt,
	}
}
