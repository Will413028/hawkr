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

// B-169 FoodSafetyReportRepository implementation.

type foodSafetyReportRepository struct {
	pool *pgxpool.Pool
}

// NewFoodSafetyReportRepository constructs the repository.
func NewFoodSafetyReportRepository(pool *pgxpool.Pool) domain.FoodSafetyReportRepository {
	return &foodSafetyReportRepository{pool: pool}
}

func (r *foodSafetyReportRepository) Create(ctx context.Context, in *domain.FoodSafetyReport) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateFoodSafetyReport(ctx, gen.CreateFoodSafetyReportParams{
		ID:               in.ID,
		TargetSellerID:   in.TargetSellerID,
		TargetProductID:  in.TargetProductID,
		TargetOrderNo:    in.TargetOrderNo,
		ReporterPhone:    in.ReporterPhone,
		ReporterEmail:    in.ReporterEmail,
		ReporterName:     in.ReporterName,
		Description:      in.Description,
		EvidenceAssetIds: marshalAssetIDs(in.EvidenceAssetIDs),
		Status:           string(in.Status),
		CreatedAt:        timestamptzParam(in.CreatedAt),
		UpdatedAt:        timestamptzParam(in.UpdatedAt),
	})
	if err != nil {
		// B1: ISO-week partial unique 觸發 → silent drop sentinel；usecase 用此 error 走 silent drop 路徑。
		if cn, ok := isUniqueViolation(err); ok {
			return nil, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) GetByID(ctx context.Context, id int64) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetFoodSafetyReportByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("food safety report not found")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) GetExistingByPhoneAndSellerThisWeek(ctx context.Context, phone string, targetSellerID int64) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetExistingFoodSafetyByPhoneAndSellerThisWeek(ctx, gen.GetExistingFoodSafetyByPhoneAndSellerThisWeekParams{
		ReporterPhone:  phone,
		TargetSellerID: int64Pointer(targetSellerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("no existing report this week")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) AdminList(ctx context.Context, filter domain.FoodSafetyReportListFilter) ([]*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	params := gen.AdminListFoodSafetyReportsParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.Status != nil {
		params.Status = ptrString(string(*filter.Status))
	}
	if filter.Since != nil {
		params.Since = nullableTimePointer(filter.Since)
	}
	if filter.UnackedOnly != nil {
		params.UnackedOnly = filter.UnackedOnly
	}
	rows, err := q.AdminListFoodSafetyReports(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.FoodSafetyReport, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainFoodSafetyReport(row))
	}
	return out, nil
}

func (r *foodSafetyReportRepository) AdminGet(ctx context.Context, id int64) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminGetFoodSafetyReport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("food safety report not found")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) AdminUpdateStatus(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminUpdateFoodSafetyReportStatus(ctx, gen.AdminUpdateFoodSafetyReportStatusParams{
		ID:                     id,
		Status:                 string(status),
		InvestigatingAt:        nullableTimePointer(investigatingAt),
		ResolvedAt:             nullableTimePointer(resolvedAt),
		AdminResolutionSummary: summary,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("food safety report not found")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) Acknowledge(ctx context.Context, id, adminID int64) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AcknowledgeFoodSafetyReport(ctx, gen.AcknowledgeFoodSafetyReportParams{
		ID:                    id,
		AcknowledgedByAdminID: int64Pointer(adminID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 已 ack 過或 record 不存在 — usecase 透過 GetByID 區分後回 409 idempotent friendly 或 404。
			return nil, pkgerrors.NewNotFound("food safety report not found or already acknowledged")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) MarkFDAReported(ctx context.Context, id int64, channel domain.FDAReportChannel, refNo string) (*domain.FoodSafetyReport, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.MarkFoodSafetyFDAReported(ctx, gen.MarkFoodSafetyFDAReportedParams{
		ID:               id,
		FdaReportChannel: nullableText(string(channel)),
		FdaReferenceNo:   nullableText(refNo),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("food safety report not found or already FDA reported")
		}
		return nil, err
	}
	report := toDomainFoodSafetyReport(row)
	return report, nil
}

func (r *foodSafetyReportRepository) AnonymizeOldPhones(ctx context.Context) ([]domain.AnonymizedFoodSafetyRecord, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.AnonymizeOldFoodSafetyPhones(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.AnonymizedFoodSafetyRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AnonymizedFoodSafetyRecord{
			ID:               row.ID,
			EvidenceAssetIDs: unmarshalAssetIDs(row.EvidenceAssetIds),
		})
	}
	return out, nil
}
