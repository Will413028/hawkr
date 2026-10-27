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

type payuniMerchantProfileOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewPayuniMerchantProfileOutboxRepository 建立 repo。
func NewPayuniMerchantProfileOutboxRepository(pool *pgxpool.Pool) *payuniMerchantProfileOutboxRepository {
	return &payuniMerchantProfileOutboxRepository{pool: pool}
}

// getQueries 取 context 上的交易（有的話），這是 Insert 能參與 UpdateProfile
// 交易的關鍵——沒有它，outbox 就退化成 at-most-once。
func (r *payuniMerchantProfileOutboxRepository) getQueries(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// Insert 寫入一筆 pending 任務，並用回傳列刷新時戳（欄位為零值時套用 server
// default）。參與呼叫端的交易。
func (r *payuniMerchantProfileOutboxRepository) Insert(ctx context.Context, row *domain.PayuniMerchantProfileOutbox) error {
	q := r.getQueries(ctx)
	out, err := q.InsertPayuniMerchantProfileOutbox(ctx, gen.InsertPayuniMerchantProfileOutboxParams{
		ID:            row.ID,
		StoreID:       row.StoreID,
		AccountID:     row.AccountID,
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

// FetchPendingDue 回傳至多 limit 筆 next_attempt_at <= now 的 pending 列。
//
// 這只是候選查詢，不提供任何互斥性：兩個 replica 同時 tick 會拿到同一批列。
// 跨 replica 的互斥由 Claim 負責。刻意不用 FOR UPDATE SKIP LOCKED——這支在
// 自動提交下跑，SELECT 一回傳鎖就釋放。
func (r *payuniMerchantProfileOutboxRepository) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*domain.PayuniMerchantProfileOutbox, error) {
	q := r.getQueries(ctx)
	rows, err := q.FetchPendingPayuniMerchantProfileOutbox(ctx, gen.FetchPendingPayuniMerchantProfileOutboxParams{
		NextAttemptAt: timestamptzParam(now),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*domain.PayuniMerchantProfileOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, payuniMerchantProfileOutboxToDomain(row))
	}
	return out, nil
}

// Claim 是 lease CAS 搶佔閘門——見 domain.PayuniMerchantProfileOutboxRepository。
func (r *payuniMerchantProfileOutboxRepository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	q := r.getQueries(ctx)
	return q.ClaimPayuniMerchantProfileOutbox(ctx, gen.ClaimPayuniMerchantProfileOutboxParams{
		ID:              id,
		LeaseUntil:      timestamptzParam(leaseUntil),
		ClaimableBefore: timestamptzParam(claimableBefore),
	})
}

// UpdateAttempt 記錄一次重試：累加 attempt_count、排下次時間、存 last_error
// （空字串寫成 NULL）。
func (r *payuniMerchantProfileOutboxRepository) UpdateAttempt(ctx context.Context, id int64, attempt int, nextAt time.Time, lastErr string) error {
	q := r.getQueries(ctx)
	return q.UpdatePayuniMerchantProfileOutboxAttempt(ctx, gen.UpdatePayuniMerchantProfileOutboxAttemptParams{
		ID:            id,
		AttemptCount:  int32(attempt),
		NextAttemptAt: timestamptzParam(nextAt),
		LastError:     nullableText(lastErr),
	})
}

// MarkDone 把列轉成 status='done' 並蓋 completed_at。
func (r *payuniMerchantProfileOutboxRepository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkPayuniMerchantProfileOutboxDone(ctx, gen.MarkPayuniMerchantProfileOutboxDoneParams{
		ID:          id,
		CompletedAt: nullableTime(now),
	})
}

// MarkFailed 把列轉成 status='failed'，存 last_error 並蓋 completed_at。
func (r *payuniMerchantProfileOutboxRepository) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	q := r.getQueries(ctx)
	return q.MarkPayuniMerchantProfileOutboxFailed(ctx, gen.MarkPayuniMerchantProfileOutboxFailedParams{
		ID:          id,
		LastError:   nullableText(lastErr),
		CompletedAt: nullableTime(now),
	})
}

// CountPending 回傳 'pending' 狀態的列數。
func (r *payuniMerchantProfileOutboxRepository) CountPending(ctx context.Context) (int, error) {
	q := r.getQueries(ctx)
	count, err := q.CountPendingPayuniMerchantProfileOutbox(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// OldestPendingAge 回傳最舊 pending 列的 created_at 相對 now 的年齡。
// 沒有 pending 列時 ok=false——那代表佇列是空的，不是故障。
func (r *payuniMerchantProfileOutboxRepository) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	q := r.getQueries(ctx)
	createdAt, err := q.OldestPendingPayuniMerchantProfileOutboxCreatedAt(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return now.Sub(createdAt.Time), true, nil
}

// DeleteDoneBefore 刪除至多 limit 筆在 cutoff 之前完成的 'done' 列。
func (r *payuniMerchantProfileOutboxRepository) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	q := r.getQueries(ctx)
	n, err := q.DeleteDonePayuniMerchantProfileOutboxBefore(ctx, gen.DeleteDonePayuniMerchantProfileOutboxBeforeParams{
		Cutoff:   nullableTime(cutoff),
		RowLimit: int32(limit),
	})
	return int(n), err
}

// payuniMerchantProfileOutboxToDomain 把 gen 列映射成 domain 實體。
func payuniMerchantProfileOutboxToDomain(row gen.PayuniMerchantProfileOutbox) *domain.PayuniMerchantProfileOutbox {
	return &domain.PayuniMerchantProfileOutbox{
		ID:            row.ID,
		StoreID:       row.StoreID,
		AccountID:     row.AccountID,
		Payload:       row.Payload,
		Status:        domain.PayuniMerchantProfileOutboxStatus(row.Status),
		AttemptCount:  int(row.AttemptCount),
		NextAttemptAt: row.NextAttemptAt.Time,
		LastError:     stringValue(row.LastError),
		CreatedAt:     row.CreatedAt.Time,
		CompletedAt:   row.CompletedAt,
	}
}
