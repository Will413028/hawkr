package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// B-169 SellerViolationReportRepository implementation.

type sellerViolationReportRepository struct {
	pool *pgxpool.Pool
}

// NewSellerViolationReportRepository constructs the repository.
func NewSellerViolationReportRepository(pool *pgxpool.Pool) domain.SellerViolationReportRepository {
	return &sellerViolationReportRepository{pool: pool}
}

func (r *sellerViolationReportRepository) CreateReport(ctx context.Context, in *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateSellerViolationReport(ctx, gen.CreateSellerViolationReportParams{
		ID:                in.ID,
		TargetSellerID:    in.TargetSellerID,
		TargetProductID:   in.TargetProductID,
		TargetPostID:      in.TargetPostID,
		TargetOrderNo:     in.TargetOrderNo,
		ReporterAccountID: in.ReporterAccountID,
		Category:          string(in.Category),
		Severity:          string(in.Severity),
		Description:       in.Description,
		EvidenceAssetIds:  marshalAssetIDs(in.EvidenceAssetIDs),
		Status:            string(in.Status),
		CreatedAt:         timestamptzParam(in.CreatedAt),
		UpdatedAt:         timestamptzParam(in.UpdatedAt),
	})
	if err != nil {
		// D13: calendar-day partial unique 觸發 → 回傳 sentinel error，usecase 捕捉後查既有 id 回 409。
		if cn, ok := isUniqueViolation(err); ok {
			return nil, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return nil, err
	}
	report := toDomainSellerViolationReport(row)
	return report, nil
}

func (r *sellerViolationReportRepository) GetByID(ctx context.Context, id int64) (*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerViolationReportByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation report not found")
		}
		return nil, err
	}
	report := toDomainSellerViolationReport(row)
	return report, nil
}

func (r *sellerViolationReportRepository) GetExistingByReporterAndTargetToday(ctx context.Context, reporterID, targetSellerID int64, category domain.ViolationCategory) (*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetExistingReportByReporterAndTargetToday(ctx, gen.GetExistingReportByReporterAndTargetTodayParams{
		ReporterAccountID: reporterID,
		TargetSellerID:    targetSellerID,
		Category:          string(category),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("no existing report today")
		}
		return nil, err
	}
	report := toDomainSellerViolationReport(row)
	return report, nil
}

func (r *sellerViolationReportRepository) ListByReporter(ctx context.Context, reporterID int64, since time.Time, limit, offset int32) ([]*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListSellerViolationReportsByReporter(ctx, gen.ListSellerViolationReportsByReporterParams{
		ReporterAccountID: reporterID,
		CreatedAt:         timestamptzParam(since),
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolationReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerViolationReport(row))
	}
	return out, nil
}

func (r *sellerViolationReportRepository) AdminList(ctx context.Context, filter domain.SellerViolationReportListFilter) ([]*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	params := gen.AdminListSellerViolationReportsParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.Status != nil {
		params.Status = ptrString(string(*filter.Status))
	}
	if filter.Severity != nil {
		params.Severity = ptrString(string(*filter.Severity))
	}
	if filter.Since != nil {
		params.Since = nullableTimePointer(filter.Since)
	}
	rows, err := q.AdminListSellerViolationReports(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolationReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerViolationReport(row))
	}
	return out, nil
}

func (r *sellerViolationReportRepository) AdminGet(ctx context.Context, id int64) (*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminGetSellerViolationReport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation report not found")
		}
		return nil, err
	}
	report := toDomainSellerViolationReport(row)
	return report, nil
}

func (r *sellerViolationReportRepository) AdminUpdateStatus(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.SellerViolationReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminUpdateSellerViolationReportStatus(ctx, gen.AdminUpdateSellerViolationReportStatusParams{
		ID:                     id,
		Status:                 string(status),
		InvestigatingAt:        nullableTimePointer(investigatingAt),
		ResolvedAt:             nullableTimePointer(resolvedAt),
		AdminResolutionSummary: summary,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation report not found")
		}
		return nil, err
	}
	report := toDomainSellerViolationReport(row)
	return report, nil
}

// helpers
func ptrString(s string) *string { return &s }
