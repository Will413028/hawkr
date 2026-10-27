package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// ScheduledPostRepository 是排程發布 worker 的資料存取層（56 §5.1.5 / §7.6）。
type ScheduledPostRepository struct {
	q    *gen.Queries
	pool *pgxpool.Pool
}

func NewScheduledPostRepository(pool *pgxpool.Pool) *ScheduledPostRepository {
	return &ScheduledPostRepository{q: gen.New(pool), pool: pool}
}

func (r *ScheduledPostRepository) ConvergeDeleted(ctx context.Context, now time.Time) (int64, error) {
	return r.q.CancelSchedulesForDeletedPosts(ctx, timestamptzParam(now))
}

func (r *ScheduledPostRepository) ConvergeHidden(ctx context.Context, now time.Time) (int64, error) {
	return r.q.CancelSchedulesForHiddenPosts(ctx, timestamptzParam(now))
}

func (r *ScheduledPostRepository) ConvergeExpired(ctx context.Context, now time.Time, windowHours int) (int64, error) {
	return r.q.ExpireSchedulesOutsideWindow(ctx, gen.ExpireSchedulesOutsideWindowParams{
		Now:         timestamptzParam(now),
		WindowHours: int32(windowHours),
	})
}

func (r *ScheduledPostRepository) ListDue(ctx context.Context, now time.Time, windowHours, limit int) ([]int64, error) {
	return r.q.ListDueScheduledPosts(ctx, gen.ListDueScheduledPostsParams{
		Now:         timestamptzParam(now),
		WindowHours: int32(windowHours),
		RowLimit:    int32(limit),
	})
}

// Claim 回 nil, nil 表示沒搶到，這是預期結果不是錯誤：另一個 pod 先到、租約還在別人
// 手上、或該列在候選 SELECT 與這一句之間被刪除／下架／收斂成終態。
func (r *ScheduledPostRepository) Claim(ctx context.Context, id int64, owner string, now, leaseExpiresAt time.Time, windowHours int) (*gen.CmsPost, error) {
	row, err := r.q.ClaimScheduledPost(ctx, gen.ClaimScheduledPostParams{
		Owner:          owner,
		LeaseExpiresAt: timestamptzParam(leaseExpiresAt),
		Now:            timestamptzParam(now),
		ID:             id,
		WindowHours:    int32(windowHours),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// Publish 把終寫 CAS 與 sameTx 綁在同一個交易裡。
//
// CAS 輸掉時直接 rollback 並回 false：sameTx 的副作用（hashtag 計數 +1）以「這則貼文
// 剛剛才變成公開可見」為前提，而 rows = 0 正是那個前提不成立。
func (r *ScheduledPostRepository) Publish(
	ctx context.Context,
	id int64,
	owner string,
	publishStatus int16,
	now time.Time,
	sameTx func(ctx context.Context, q gen.DBTX) error,
) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := gen.New(tx).PublishScheduledPost(ctx, gen.PublishScheduledPostParams{
		PublishStatus: publishStatus,
		Now:           timestamptzParam(now),
		ID:            id,
		Owner:         owner,
	})
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, nil
	}
	if sameTx != nil {
		if err := sameTx(ctx, tx); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *ScheduledPostRepository) ReleaseLease(ctx context.Context, id int64, owner string, now, nextAttemptAt time.Time) (bool, error) {
	rows, err := r.q.ReleaseScheduledPostLease(ctx, gen.ReleaseScheduledPostLeaseParams{
		NextAttemptAt: timestamptzParam(nextAttemptAt),
		Now:           timestamptzParam(now),
		ID:            id,
		Owner:         owner,
	})
	return rows > 0, err
}

func (r *ScheduledPostRepository) ExhaustRetries(ctx context.Context, id int64, owner string, now time.Time) (bool, error) {
	rows, err := r.q.ExpireExhaustedScheduledPost(ctx, gen.ExpireExhaustedScheduledPostParams{
		Now:   timestamptzParam(now),
		ID:    id,
		Owner: owner,
	})
	return rows > 0, err
}

func (r *ScheduledPostRepository) IsLeaseHeldBy(ctx context.Context, id int64, owner string) (bool, error) {
	return r.q.IsScheduledPostLeaseHeldBy(ctx, gen.IsScheduledPostLeaseHeldByParams{
		ID:    id,
		Owner: owner,
	})
}

func (r *ScheduledPostRepository) CancelByRevocation(ctx context.Context, id int64, owner string, now time.Time) (bool, error) {
	rows, err := r.q.CancelScheduledPostByRevocation(ctx, gen.CancelScheduledPostByRevocationParams{
		Now:   timestamptzParam(now),
		ID:    id,
		Owner: owner,
	})
	return rows > 0, err
}
