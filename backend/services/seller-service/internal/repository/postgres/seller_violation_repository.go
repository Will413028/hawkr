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

// B-169 SellerViolationRepository implementation.
//
// Methods that mutate state — CreateViolation / Submit/DecideAppeal / AdminRevoke / 等 — 由 usecase 透過
// pkg/database.TxManager.WithTx 包裝；method 內部用 GetDBTX(ctx, pool) 自動拿 tx。

type sellerViolationRepository struct {
	pool *pgxpool.Pool
}

// NewSellerViolationRepository constructs the repository.
func NewSellerViolationRepository(pool *pgxpool.Pool) domain.SellerViolationRepository {
	return &sellerViolationRepository{pool: pool}
}

// ========== Transactional helpers ==========

func (r *sellerViolationRepository) AcquireCreateLock(ctx context.Context, sellerID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.AcquireViolationCreateLock(ctx, sellerID)
}

func (r *sellerViolationRepository) CountMediumLast90d(ctx context.Context, sellerID int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountMediumLast90dBySeller(ctx, sellerID)
}

// ========== Mutate ==========

func (r *sellerViolationRepository) CreateViolation(ctx context.Context, in *domain.SellerViolation) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateSellerViolation(ctx, gen.CreateSellerViolationParams{
		ID:                       in.ID,
		SellerID:                 in.SellerID,
		SourceSellerReportID:     in.SourceSellerReportID,
		SourceFoodSafetyReportID: in.SourceFoodSafetyReportID,
		Category:                 string(in.Category),
		Severity:                 string(in.Severity),
		PenaltyType:              string(in.PenaltyType),
		PenaltyAmountCents:       in.PenaltyAmountCents,
		Description:              in.Description,
		PublicSummary:            in.PublicSummary,
		EffectiveAt:              timestamptzParam(in.EffectiveAt),
		ExpiresAt:                nullableTimePointer(in.ExpiresAt),
		Status:                   string(in.Status),
		CreatedByAdminID:         in.CreatedByAdminID,
		CreatedAt:                timestamptzParam(in.CreatedAt),
		UpdatedAt:                timestamptzParam(in.UpdatedAt),
	})
	if err != nil {
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

func (r *sellerViolationRepository) GetByID(ctx context.Context, id int64) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerViolationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation not found")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

func (r *sellerViolationRepository) AdminUpdateMutable(ctx context.Context, id int64, description string, publicSummary *string, expiresAt *time.Time) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminUpdateSellerViolationMutable(ctx, gen.AdminUpdateSellerViolationMutableParams{
		ID:            id,
		Description:   description,
		PublicSummary: publicSummary,
		ExpiresAt:     nullableTimePointer(expiresAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation not found or already revoked")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

func (r *sellerViolationRepository) AdminRevoke(ctx context.Context, id int64, reason domain.RevokedReason) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.AdminRevokeSellerViolation(ctx, gen.AdminRevokeSellerViolationParams{
		ID:            id,
		RevokedReason: nullableText(string(reason)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller violation not found or already revoked")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

// ========== Listing ==========

func (r *sellerViolationRepository) ListBySellerSelf(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListSellerViolationsBySellerSelf(ctx, gen.ListSellerViolationsBySellerSelfParams{
		SellerID: sellerID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerViolation(row))
	}
	return out, nil
}

func (r *sellerViolationRepository) ListPublicLast90dHigh(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListSellerViolationsPublicLast90dHigh(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolation, 0, len(rows))
	for _, row := range rows {
		// 此 query 只 select 公開白名單欄位，組成 partial domain 物件給 D8 cache 用。
		v := &domain.SellerViolation{
			ID:            row.ID,
			SellerID:      row.SellerID,
			Category:      domain.ViolationCategory(row.Category),
			Severity:      domain.ViolationSeverity(row.Severity),
			PenaltyType:   domain.PenaltyType(row.PenaltyType),
			PublicSummary: row.PublicSummary,
			EffectiveAt:   row.EffectiveAt.Time,
			ExpiresAt:     row.ExpiresAt,
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *sellerViolationRepository) AdminList(ctx context.Context, filter domain.SellerViolationListFilter) ([]*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	params := gen.AdminListSellerViolationsParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.SellerID != nil {
		params.SellerID = filter.SellerID
	}
	if filter.Severity != nil {
		params.Severity = ptrString(string(*filter.Severity))
	}
	if filter.PenaltyType != nil {
		params.PenaltyType = ptrString(string(*filter.PenaltyType))
	}
	if filter.AppealStatus != nil {
		params.AppealStatus = ptrString(string(*filter.AppealStatus))
	}
	rows, err := q.AdminListSellerViolations(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerViolation(row))
	}
	return out, nil
}

// ========== Cron ==========

func (r *sellerViolationRepository) ListExpiredFullSuspensions(ctx context.Context) ([]*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListExpiredFullSuspensions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SellerViolation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSellerViolation(row))
	}
	return out, nil
}

// HasActiveFullSuspensionBySeller 32 §13.3 recomputeSellerStatus 讀端。
func (r *sellerViolationRepository) HasActiveFullSuspensionBySeller(ctx context.Context, sellerID int64) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.HasActiveFullSuspensionBySeller(ctx, sellerID)
}

// ========== Appeal ==========

func (r *sellerViolationRepository) SubmitAppeal(ctx context.Context, id int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.SubmitAppeal(ctx, gen.SubmitAppealParams{
		ID:                     id,
		AppealReason:           nullableText(reason),
		AppealEvidenceAssetIds: marshalAssetIDs(evidenceAssetIDs),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 已申訴過 / 已 revoked / id 不存在 — usecase 透過 GetByID 區分後回對應 4xx。
			return nil, pkgerrors.NewNotFound("violation not found, not appealable, or already appealed")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

func (r *sellerViolationRepository) DecideAppealAccept(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.DecideAppealAccept(ctx, gen.DecideAppealAcceptParams{
		ID:                     id,
		AppealDecidedByAdminID: int64Pointer(adminID),
		AppealDecisionReason:   nullableText(reason),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("appeal not pending or violation not found")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}

func (r *sellerViolationRepository) DecideAppealReject(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.DecideAppealReject(ctx, gen.DecideAppealRejectParams{
		ID:                     id,
		AppealDecidedByAdminID: int64Pointer(adminID),
		AppealDecisionReason:   nullableText(reason),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("appeal not pending or violation not found")
		}
		return nil, err
	}
	v := toDomainSellerViolation(row)
	return v, nil
}
