package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type sellerDeletionAuditRepository struct {
	pool *pgxpool.Pool
}

// NewSellerDeletionAuditRepository 建立 repo。
func NewSellerDeletionAuditRepository(pool *pgxpool.Pool) *sellerDeletionAuditRepository {
	return &sellerDeletionAuditRepository{pool: pool}
}

// Insert 寫入一筆稽核（參與 caller 的 tx：GetDBTX(ctx)）。
func (r *sellerDeletionAuditRepository) Insert(ctx context.Context, a *domain.SellerDeletionAudit) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.InsertSellerDeletionAudit(ctx, gen.InsertSellerDeletionAuditParams{
		ID:               a.ID,
		AccountID:        a.AccountID,
		OperatorID:       a.OperatorID,
		ActiveOrderCount: int32(a.ActiveOrderCount),
		Reason:           nullableText(a.Reason),
		CreatedAt:        timestamptzParam(a.CreatedAt),
	})
}
