package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// B-169 SellerNotificationLogRepository implementation.

type sellerNotificationLogRepository struct {
	pool *pgxpool.Pool
}

// NewSellerNotificationLogRepository constructs the repository.
func NewSellerNotificationLogRepository(pool *pgxpool.Pool) domain.SellerNotificationLogRepository {
	return &sellerNotificationLogRepository{pool: pool}
}

func (r *sellerNotificationLogRepository) Create(ctx context.Context, in *domain.SellerNotificationLog) (*domain.SellerNotificationLog, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateSellerNotificationLog(ctx, gen.CreateSellerNotificationLogParams{
		ID:                in.ID,
		SourceReportID:    in.SourceReportID,
		SourceReportTable: in.SourceReportTable,
		TemplateKey:       in.TemplateKey,
		SellerID:          in.SellerID,
		SentByAdminID:     in.SentByAdminID,
		MessagePayload:    in.MessagePayload,
	})
	if err != nil {
		return nil, err
	}
	return toDomainNotificationLog(row), nil
}

func (r *sellerNotificationLogRepository) ListBySourceReport(ctx context.Context, sourceTable string, sourceReportID int64) ([]*domain.SellerNotificationLog, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListNotificationsBySourceReport(ctx, gen.ListNotificationsBySourceReportParams{
		SourceReportTable: nullableText(sourceTable),
		SourceReportID:    int64Pointer(sourceReportID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerNotificationLog, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainNotificationLog(row))
	}
	return out, nil
}

func (r *sellerNotificationLogRepository) ListBySeller(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerNotificationLog, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListNotificationsBySeller(ctx, gen.ListNotificationsBySellerParams{
		SellerID: sellerID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerNotificationLog, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainNotificationLog(row))
	}
	return out, nil
}
