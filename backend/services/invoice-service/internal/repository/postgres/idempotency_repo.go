package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// IdempotencyRepository 是冪等鍵的落地（#3015 裁決 A）。
//
// 🔴 它**必須**與業務寫入共用同一個交易：`r.q(ctx)` 會取出 ctx 裡的 tx，
// 這是全 repository 層的既有約定。少了那一點，這張表就退化成「另一次獨立寫入」，
// 而兩段寫入之間的窗口正是它要消滅的東西。
type IdempotencyRepository struct{ baseRepo }

// NewIdempotencyRepository 建 repository。
func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.IdempotencyRepository = (*IdempotencyRepository)(nil)

// Claim 認領一把鍵，回「表上最終那一列」＋這次是不是第一次。
func (r *IdempotencyRepository) Claim(ctx context.Context, in domain.ClaimIdempotency) (domain.IdempotencyRecord, bool, error) {
	row, err := r.q(ctx).ClaimIdempotencyKey(ctx, gen.ClaimIdempotencyKeyParams{
		Scope:       in.Scope,
		ActorID:     in.ActorID,
		IdemKey:     in.Key,
		RequestHash: in.RequestHash,
		Now:         ts(in.Now),
	})
	if err != nil {
		return domain.IdempotencyRecord{}, false, err
	}
	rec := domain.IdempotencyRecord{
		Scope:       row.Scope,
		ActorID:     row.ActorID,
		Key:         row.IdemKey,
		RequestHash: row.RequestHash,
		ResultRef:   row.ResultRef,
	}
	return rec, row.Claimed, nil
}

// SetResult 把第一次成功的業務主鍵補上，供之後的重送回放。
func (r *IdempotencyRepository) SetResult(ctx context.Context, scope string, actorID int64, key string, resultRef int64) error {
	_, err := r.q(ctx).SetIdempotencyResult(ctx, gen.SetIdempotencyResultParams{
		ResultRef: &resultRef,
		Scope:     scope,
		ActorID:   actorID,
		IdemKey:   key,
	})
	return err
}
