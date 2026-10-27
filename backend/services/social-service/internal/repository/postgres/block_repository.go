package postgres

import (
	"context"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BlockRepository struct {
	q *gen.Queries
}

func NewBlockRepository(pool *pgxpool.Pool) *BlockRepository {
	return &BlockRepository{q: gen.New(pool)}
}

func (r *BlockRepository) Create(ctx context.Context, id, blockerID, blockedID int64) (*gen.UserBlock, error) {
	row, err := r.q.CreateBlock(ctx, gen.CreateBlockParams{
		ID:        id,
		BlockerID: blockerID,
		BlockedID: blockedID,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *BlockRepository) Delete(ctx context.Context, blockerID, blockedID int64) error {
	return r.q.DeleteBlock(ctx, gen.DeleteBlockParams{
		BlockerID: blockerID,
		BlockedID: blockedID,
	})
}

func (r *BlockRepository) Get(ctx context.Context, blockerID, blockedID int64) (*gen.UserBlock, error) {
	row, err := r.q.GetBlock(ctx, gen.GetBlockParams{
		BlockerID: blockerID,
		BlockedID: blockedID,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *BlockRepository) ListByBlocker(ctx context.Context, blockerID int64, limit, offset int64) ([]*gen.UserBlock, error) {
	rows, err := r.q.ListBlocksByBlocker(ctx, gen.ListBlocksByBlockerParams{
		BlockerID: blockerID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*gen.UserBlock, len(rows))
	for i := range rows {
		row := rows[i]
		out[i] = &row
	}
	return out, nil
}
