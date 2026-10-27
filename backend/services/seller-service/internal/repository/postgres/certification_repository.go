package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type certificationRepository struct {
	pool *pgxpool.Pool
	enc  *crypto.Cipher
}

func NewCertificationRepository(pool *pgxpool.Pool, enc *crypto.Cipher) domain.CertificationRepository {
	return &certificationRepository{pool: pool, enc: enc}
}

func (r *certificationRepository) GetApplicationByID(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetApplicationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("certification application not found")
		}
		return nil, err
	}
	return toDomainCertificationApplication(row, r.enc)
}

func (r *certificationRepository) GetApplicationByNo(ctx context.Context, no string) (*domain.CertificationApplication, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetApplicationByNo(ctx, no)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("certification application not found")
		}
		return nil, err
	}
	return toDomainCertificationApplication(row, r.enc)
}

func (r *certificationRepository) GetApplicationsBySeller(ctx context.Context, accountID int64) ([]*domain.CertificationApplication, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetApplicationsBySeller(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CertificationApplication, 0, len(rows))
	for _, row := range rows {
		app, err := toDomainCertificationApplication(row, r.enc)
		if err != nil {
			return nil, err
		}
		result = append(result, app)
	}
	return result, nil
}

func (r *certificationRepository) GetCurrentApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCurrentApplication(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("certification application not found")
		}
		return nil, err
	}
	return toDomainCertificationApplication(row, r.enc)
}

func (r *certificationRepository) GetDraftApplication(ctx context.Context, accountID int64) (*domain.CertificationApplication, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetDraftApplication(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("certification application not found")
		}
		return nil, err
	}
	return toDomainCertificationApplication(row, r.enc)
}

func (r *certificationRepository) GetCertHolderNames(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCertHolderNamesByStoreID(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CertHolderNames{}, false, nil
		}
		return domain.CertHolderNames{}, false, err
	}
	return domain.CertHolderNames{
		RealName:    stringValue(row.RealName),
		CompanyName: stringValue(row.CompanyName),
	}, true, nil
}

func (r *certificationRepository) CreateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	params, err := toCreateApplicationParams(app, r.enc)
	if err != nil {
		return err
	}
	_, err = q.CreateApplication(ctx, params)
	return err
}

func (r *certificationRepository) UpdateApplication(ctx context.Context, app *domain.CertificationApplication) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	params, err := toUpdateApplicationParams(app, r.enc)
	if err != nil {
		return err
	}
	n, err := q.UpdateApplication(ctx, params)
	if err != nil {
		return err
	}
	if n == 0 {
		return pkgerrors.NewConflict("認證資料已被其他操作更新，請重新載入")
	}
	app.Version++
	return nil
}

func (r *certificationRepository) GetReviewsByApplication(ctx context.Context, appID int64) ([]*domain.CertificationReview, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetReviewsByApplication(ctx, appID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CertificationReview, 0, len(rows))
	for _, row := range rows {
		review, err := toDomainCertificationReview(row)
		if err != nil {
			return nil, err
		}
		result = append(result, review)
	}
	return result, nil
}

func (r *certificationRepository) GetLatestReview(ctx context.Context, appID int64) (*domain.CertificationReview, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetLatestReview(ctx, appID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("certification review not found")
		}
		return nil, err
	}
	return toDomainCertificationReview(row)
}

func (r *certificationRepository) CreateReview(ctx context.Context, review *domain.CertificationReview) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateReview(ctx, gen.CreateReviewParams{
		ApplicationID:   review.ApplicationID,
		ReviewerID:      review.ReviewerID,
		ReviewerName:    nullableText(review.ReviewerName),
		Action:          review.Action.String(),
		Result:          nullableText(review.Result.String()),
		Comment:         nullableText(review.Comment),
		RejectionReason: nullableText(review.RejectionReason),
		RequiredFields:  pgxutil.JSONBytes(review.RequiredFields),
		CreatedAt:       nullableTime(review.CreatedAt),
	})
	if err != nil {
		return err
	}
	review.ID = row.ID
	return nil
}

func (r *certificationRepository) ListForAdmin(ctx context.Context, filter domain.CertificationListFilter) ([]domain.AdminCertificationItem, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	status := ""
	if filter.Status != nil {
		status = *filter.Status
	}
	keyword := ""
	if filter.Keyword != nil {
		keyword = *filter.Keyword
	}

	rows, err := q.ListCertificationsForAdmin(ctx, gen.ListCertificationsForAdminParams{
		Column1: status,
		Column2: keyword,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]domain.AdminCertificationItem, 0, len(rows))
	for _, row := range rows {
		var docAssetIDs []string
		if len(row.DocumentAssetIds) > 0 {
			if err := json.Unmarshal(row.DocumentAssetIds, &docAssetIDs); err != nil {
				docAssetIDs = []string{}
			}
		}

		items = append(items, domain.AdminCertificationItem{
			ID:            row.ID,
			AccountID:     row.AccountID,
			ApplicationNo: row.ApplicationNo,
			Status:        row.Status,
			SellerType:    row.SellerType,
			RealName:      stringValue(row.RealName),
			StoreName:     stringValue(row.StoreName),
			CurrentStep:   int32Value(row.CurrentStep),
			TotalSteps:    int32Value(row.TotalSteps),
			SubmittedAt:   row.SubmittedAt,
			RejectedAt:    row.RejectedAt,
			CreatedAt:     timeValue(row.CreatedAt),
			UpdatedAt:     timeValue(row.UpdatedAt),

			IDCardFrontAssetID: stringValue(row.IDCardFrontAssetID),
			IDCardBackAssetID:  stringValue(row.IDCardBackAssetID),
			BankBookAssetID:    stringValue(row.BankBookAssetID),
			DocumentAssetIDs:   docAssetIDs,
		})
	}
	return items, nil
}

func (r *certificationRepository) CountForAdmin(ctx context.Context, filter domain.CertificationListFilter) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	status := ""
	if filter.Status != nil {
		status = *filter.Status
	}
	keyword := ""
	if filter.Keyword != nil {
		keyword = *filter.Keyword
	}

	return q.CountCertificationsForAdmin(ctx, gen.CountCertificationsForAdminParams{
		Column1: status,
		Column2: keyword,
	})
}

func (r *certificationRepository) GetStatsForAdmin(ctx context.Context) (*domain.CertificationStats, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	row, err := q.GetCertificationStatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}

	return &domain.CertificationStats{
		Total:     row.Total,
		Submitted: row.Submitted,
		Approved:  row.Approved,
		Rejected:  row.Rejected,
	}, nil
}
