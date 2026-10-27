package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type sellerRepository struct {
	pool *pgxpool.Pool
}

func NewSellerRepository(pool *pgxpool.Pool) domain.SellerRepository {
	return &sellerRepository{pool: pool}
}

func (r *sellerRepository) GetByAccountID(ctx context.Context, accountID int64) (*domain.Seller, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	return toDomainSeller(row)
}

func (r *sellerRepository) GetCurrentByStoreID(ctx context.Context, storeID int64) (*domain.Seller, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCurrentSellerByStoreID(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	return toDomainSeller(row)
}

func (r *sellerRepository) GetProfileByAccountID(ctx context.Context, accountID int64) (*domain.SellerProfileRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerProfileByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	return r.profileRowToDomain(row)
}

func (r *sellerRepository) GetPublicProfileByStoreIDOrCode(ctx context.Context, storeIDOrCode int64) (*domain.SellerProfileRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetPublicSellerProfileByStoreIDOrCode(ctx, storeIDOrCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	return r.profileRowToDomain(gen.GetSellerProfileByAccountIDRow(row))
}

func (r *sellerRepository) GetProfileByStoreID(ctx context.Context, storeID int64) (*domain.SellerProfileRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerProfileByStoreID(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	m, err := toDomainSeller(gen.Seller{
		AccountID:               row.AccountID,
		StoreID:                 row.StoreID,
		StoreCode:               row.StoreCode,
		StoreName:               row.StoreName,
		Description:             row.Description,
		Logo:                    row.Logo,
		CoverKey:                row.CoverKey,
		Email:                   row.Email,
		ContactPhone:            row.ContactPhone,
		City:                    row.City,
		District:                row.District,
		AddressDetail:           row.AddressDetail,
		PostalCode:              row.PostalCode,
		Birthday:                row.Birthday,
		Gender:                  row.Gender,
		Prize:                   row.Prize,
		Rating:                  row.Rating,
		CurrentCertificationID:  row.CurrentCertificationID,
		SellerType:              row.SellerType,
		OnboardingCompleted:     row.OnboardingCompleted,
		CertifiedAt:             row.CertifiedAt,
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
		DeletedAt:               row.DeletedAt,
		Status:                  row.Status,
		PiiClearedAt:            row.PiiClearedAt,
		PayuniAccountUnlinkedAt: row.PayuniAccountUnlinkedAt,
		SeoTitle:                row.SeoTitle,
		SeoDescription:          row.SeoDescription,
		WebsiteUrl:              row.WebsiteUrl,
		IsProcessedFood:         row.IsProcessedFood,
		FoodBusinessRegNo:       row.FoodBusinessRegNo,
		Version:                 row.Version,
	})
	if err != nil {
		return nil, err
	}
	return &domain.SellerProfileRow{
		Seller:                   m,
		CertStatus:               row.CertStatus,
		PayuniApprovedAt:         row.PayuniApprovedAt,
		CertSellerType:           row.CertSellerType,
		PayuniMerStatus:          row.PayuniMerStatus,
		PayuniRegistrationStatus: row.PayuniRegistrationStatus,
		PayuniMemberStatus:       row.PayuniMemberStatus,
	}, nil
}

func (r *sellerRepository) GetAccountIDByStoreID(ctx context.Context, storeID int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	accountID, err := q.GetSellerAccountLinkage(ctx, storeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, pkgerrors.NewNotFound("seller not found")
		}
		return 0, err
	}
	return accountID, nil
}

func (r *sellerRepository) profileRowToDomain(row gen.GetSellerProfileByAccountIDRow) (*domain.SellerProfileRow, error) {
	m, err := toDomainSeller(gen.Seller{
		AccountID:               row.AccountID,
		StoreID:                 row.StoreID,
		StoreCode:               row.StoreCode,
		StoreName:               row.StoreName,
		Description:             row.Description,
		Logo:                    row.Logo,
		CoverKey:                row.CoverKey,
		Email:                   row.Email,
		ContactPhone:            row.ContactPhone,
		City:                    row.City,
		District:                row.District,
		AddressDetail:           row.AddressDetail,
		PostalCode:              row.PostalCode,
		Birthday:                row.Birthday,
		Gender:                  row.Gender,
		Prize:                   row.Prize,
		Rating:                  row.Rating,
		CurrentCertificationID:  row.CurrentCertificationID,
		SellerType:              row.SellerType,
		OnboardingCompleted:     row.OnboardingCompleted,
		CertifiedAt:             row.CertifiedAt,
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
		DeletedAt:               row.DeletedAt,
		Status:                  row.Status,
		PiiClearedAt:            row.PiiClearedAt,
		PayuniAccountUnlinkedAt: row.PayuniAccountUnlinkedAt,
		SeoTitle:                row.SeoTitle,
		SeoDescription:          row.SeoDescription,
		WebsiteUrl:              row.WebsiteUrl,
		IsProcessedFood:         row.IsProcessedFood,
		FoodBusinessRegNo:       row.FoodBusinessRegNo,
		Version:                 row.Version,
	})
	if err != nil {
		return nil, err
	}
	return &domain.SellerProfileRow{
		Seller:                   m,
		CertStatus:               row.CertStatus,
		PayuniApprovedAt:         row.PayuniApprovedAt,
		CertSellerType:           row.CertSellerType,
		PayuniMerStatus:          row.PayuniMerStatus,
		PayuniRegistrationStatus: row.PayuniRegistrationStatus,
		PayuniMemberStatus:       row.PayuniMemberStatus,
	}, nil
}

func (r *sellerRepository) GetByStoreIDs(ctx context.Context, storeIDs []int64) ([]*domain.Seller, error) {
	if len(storeIDs) == 0 {
		return []*domain.Seller{}, nil
	}

	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT m.account_id, m.store_id, m.store_code, m.store_name, COALESCE(m.description, ''), COALESCE(m.logo, ''),
		        COALESCE(m.cover_key, ''), COALESCE(m.rating, 0),
		        COALESCE(m.city, ''), COALESCE(m.district, '')
		 FROM sellers m
		 WHERE m.store_id = ANY($1::bigint[]) AND m.is_current AND m.deleted_at IS NULL
		   AND m.status = $3
		   AND EXISTS (
		     SELECT 1 FROM payuni_registrations pr
		     WHERE pr.store_id = m.store_id AND pr.registration_status = $2
		   )`,
		storeIDs,
		domain.PayuniStatusActive.String(),
		string(domain.SellerStatusActive),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.Seller, 0, len(storeIDs))
	for rows.Next() {
		var s domain.Seller
		if err := rows.Scan(&s.AccountID, &s.StoreID, &s.StoreCode, &s.StoreName, &s.Description, &s.Logo, &s.CoverKey, &s.Rating, &s.City, &s.District); err != nil {
			return nil, err
		}
		result = append(result, &s)
	}
	return result, rows.Err()
}

func (r *sellerRepository) GetPublicStoreIDsByCity(ctx context.Context, cityAliases []string) ([]int64, error) {
	if len(cityAliases) == 0 {
		return []int64{}, nil
	}
	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT m.store_id
		FROM sellers m
		WHERE m.city = ANY($1::text[])
		  AND m.is_current
		  AND m.deleted_at IS NULL
		  AND m.status = $2
		  AND EXISTS (
		    SELECT 1 FROM payuni_registrations pr
		    WHERE pr.store_id = m.store_id AND pr.registration_status = $3
		  )
		ORDER BY m.store_id`, cityAliases, string(domain.SellerStatusActive), domain.PayuniStatusActive.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (r *sellerRepository) GetPublicStoreIDsGroupedByCity(ctx context.Context) (map[string][]int64, error) {
	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT m.city, m.store_id
		FROM sellers m
		WHERE m.city IS NOT NULL
		  AND m.city <> ''
		  AND m.is_current
		  AND m.deleted_at IS NULL
		  AND m.status = $1
		  AND EXISTS (
		    SELECT 1 FROM payuni_registrations pr
		    WHERE pr.store_id = m.store_id AND pr.registration_status = $2
		  )
		ORDER BY m.city, m.store_id`, string(domain.SellerStatusActive), domain.PayuniStatusActive.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string][]int64)
	for rows.Next() {
		var city string
		var storeID int64
		if err := rows.Scan(&city, &storeID); err != nil {
			return nil, err
		}
		canonical, ok := twaddr.CanonicalCity(city)
		if ok {
			result[canonical] = append(result[canonical], storeID)
		}
	}
	return result, rows.Err()
}

func (r *sellerRepository) GetApprovedExcluding(ctx context.Context, excludeStoreIDs []int64, withPublicProducts domain.PublicProductStores, limit int32) ([]*domain.Seller, error) {
	if limit <= 0 {
		return []*domain.Seller{}, nil
	}
	if excludeStoreIDs == nil {
		excludeStoreIDs = []int64{}
	}

	args := []any{
		domain.PayuniStatusActive.String(),
		excludeStoreIDs,
		limit,
		string(domain.SellerStatusActive),
	}
	publicProductsClause := ""
	if withPublicProducts.Known() {
		args = append(args, withPublicProducts.IDs())
		publicProductsClause = "\n\t\t   AND m.store_id = ANY($5::bigint[])"
	}

	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT m.account_id, m.store_id, m.store_code, m.store_name, COALESCE(m.description, ''), COALESCE(m.logo, ''),
		        COALESCE(m.cover_key, ''), COALESCE(m.rating, 0),
		        COALESCE(m.city, ''), COALESCE(m.district, '')
		 FROM sellers m
		 JOIN payuni_registrations pr ON pr.store_id = m.store_id
		 WHERE pr.registration_status = $1
		   AND m.is_current AND m.deleted_at IS NULL
		   AND m.status = $4
		   AND NOT (m.store_id = ANY($2::bigint[]))`+publicProductsClause+`
		 ORDER BY m.store_id DESC
		 LIMIT $3`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.Seller, 0, limit)
	for rows.Next() {
		var s domain.Seller
		if err := rows.Scan(&s.AccountID, &s.StoreID, &s.StoreCode, &s.StoreName, &s.Description, &s.Logo, &s.CoverKey, &s.Rating, &s.City, &s.District); err != nil {
			return nil, err
		}
		result = append(result, &s)
	}
	return result, rows.Err()
}

func (r *sellerRepository) GetSellerMetaByStoreIDs(ctx context.Context, storeIDs []int64) ([]*domain.SellerMetaRow, error) {
	if len(storeIDs) == 0 {
		return []*domain.SellerMetaRow{}, nil
	}

	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT m.account_id, m.store_id, m.store_code, m.store_name, COALESCE(m.logo, ''),
		        COALESCE(c.status, 'not_started') AS cert_status,
		        COALESCE(pr.registration_status, '') AS payuni_status, m.status,
		        COALESCE(m.postal_code, ''), COALESCE(m.city, ''), COALESCE(m.district, '')
		 FROM sellers m
		 LEFT JOIN certification_applications c ON c.id = m.current_certification_id
		 LEFT JOIN payuni_registrations pr ON pr.store_id = m.store_id
		 WHERE m.store_id = ANY($1::bigint[])`,
		storeIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.SellerMetaRow, 0, len(storeIDs))
	for rows.Next() {
		var row domain.SellerMetaRow
		if err := rows.Scan(&row.AccountID, &row.StoreID, &row.StoreCode, &row.StoreName, &row.Logo, &row.CertStatus, &row.PayuniStatus, &row.Status, &row.ZipCode, &row.City, &row.District); err != nil {
			return nil, err
		}
		result = append(result, &row)
	}
	return result, rows.Err()
}

func (r *sellerRepository) GetPrincipalsByStoreIDs(ctx context.Context, ids []int64) (map[int64]int64, []int64, error) {
	if len(ids) == 0 {
		return map[int64]int64{}, nil, nil
	}

	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT input.id, s.account_id
		 FROM unnest($1::bigint[]) AS input(id)
		 LEFT JOIN sellers s ON s.store_id = input.id AND s.deleted_at IS NULL`,
		ids,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	result := make(map[int64]int64, len(ids))
	var unresolvedIDs []int64
	for rows.Next() {
		var id int64
		var accountID *int64
		if err := rows.Scan(&id, &accountID); err != nil {
			return nil, nil, err
		}
		if accountID == nil {
			result[id] = id
			unresolvedIDs = append(unresolvedIDs, id)
		} else {
			result[id] = *accountID
		}
	}
	return result, unresolvedIDs, rows.Err()
}

func (r *sellerRepository) GetStoreIDsByAccountID(ctx context.Context, accountID int64) ([]int64, error) {
	db := database.GetDBTX(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT store_id FROM sellers WHERE account_id = $1`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storeIDs := make([]int64, 0)
	for rows.Next() {
		var storeID int64
		if err := rows.Scan(&storeID); err != nil {
			return nil, err
		}
		storeIDs = append(storeIDs, storeID)
	}
	return storeIDs, rows.Err()
}

func (r *sellerRepository) Create(ctx context.Context, m *domain.Seller) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	_, err := q.CreateSeller(ctx, gen.CreateSellerParams{
		AccountID:     m.AccountID,
		StoreID:       m.StoreID,
		StoreCode:     m.StoreCode,
		StoreName:     m.StoreName,
		Description:   nullableText(m.Description),
		Logo:          nullableText(m.Logo),
		CoverKey:      nullableText(m.CoverKey),
		Email:         nullableText(m.Email),
		ContactPhone:  nullableText(m.ContactPhone),
		City:          nullableText(m.City),
		District:      nullableText(m.District),
		AddressDetail: nullableText(m.AddressDetail),
		PostalCode:    nullableText(m.PostalCode),
		Birthday:      stringToDate(m.Birthday),
		Gender:        nullableText(m.Gender),
		Prize:         nullableText(m.Prize),
		Rating:        float64Pointer(m.Rating),
		SellerType:    nullableText(m.SellerType.String()),
		CreatedAt:     nullableTime(m.CreatedAt),
		UpdatedAt:     nullableTime(m.UpdatedAt),
	})
	if err != nil {
		if cn, ok := isUniqueViolation(err); ok {
			return &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return err
	}
	return nil
}

const noVersionCAS int64 = -1

func casArg(expectedVersion *int64) int64 {
	if expectedVersion == nil {
		return noVersionCAS
	}
	return *expectedVersion
}

func casResult(rows int64, expectedVersion *int64, err error) error {
	if err != nil {
		return err
	}
	if rows == 0 && expectedVersion != nil {
		return domain.ErrVersionConflict
	}
	return nil
}

func (r *sellerRepository) Update(ctx context.Context, m *domain.Seller, expectedVersion *int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.UpdateSeller(ctx, gen.UpdateSellerParams{
		StoreID:         m.StoreID,
		StoreName:       m.StoreName,
		Description:     nullableText(m.Description),
		Logo:            nullableText(m.Logo),
		CoverKey:        nullableText(m.CoverKey),
		ContactPhone:    nullableText(m.ContactPhone),
		City:            nullableText(m.City),
		District:        nullableText(m.District),
		AddressDetail:   nullableText(m.AddressDetail),
		PostalCode:      nullableText(m.PostalCode),
		Birthday:        stringToDate(m.Birthday),
		Gender:          nullableText(m.Gender),
		Prize:           nullableText(m.Prize),
		Rating:          float64Pointer(m.Rating),
		WebsiteUrl:      nullableText(m.WebsiteURL),
		UpdatedAt:       nullableTime(m.UpdatedAt),
		ExpectedVersion: casArg(expectedVersion),
	})
	return casResult(rows, expectedVersion, err)
}

func (r *sellerRepository) UpdateLogo(ctx context.Context, storeID int64, logo string, expectedVersion *int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.UpdateSellerLogo(ctx, gen.UpdateSellerLogoParams{
		StoreID:         storeID,
		Logo:            nullableText(logo),
		UpdatedAt:       nullableTime(now()),
		ExpectedVersion: casArg(expectedVersion),
	})
	return casResult(rows, expectedVersion, err)
}

func (r *sellerRepository) UpdateCover(ctx context.Context, storeID int64, coverKey string, expectedVersion *int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.UpdateSellerCover(ctx, gen.UpdateSellerCoverParams{
		StoreID:         storeID,
		CoverKey:        nullableText(coverKey),
		UpdatedAt:       nullableTime(now()),
		ExpectedVersion: casArg(expectedVersion),
	})
	return casResult(rows, expectedVersion, err)
}

func (r *sellerRepository) UpdateSEO(ctx context.Context, accountID int64, seoTitle, seoDescription string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateSellerSEO(ctx, gen.UpdateSellerSEOParams{
		AccountID:      accountID,
		SeoTitle:       nullableText(seoTitle),
		SeoDescription: nullableText(seoDescription),
		UpdatedAt:      nullableTime(now()),
	})
}

func (r *sellerRepository) UpdateFoodBusinessRegistration(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateSellerFoodBusinessRegistration(ctx, gen.UpdateSellerFoodBusinessRegistrationParams{
		AccountID:         accountID,
		IsProcessedFood:   isProcessedFood,
		FoodBusinessRegNo: regNo,
		UpdatedAt:         nullableTime(updatedAt),
	})
}

func (r *sellerRepository) UpdateCurrentCertificationID(ctx context.Context, accountID int64, certID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateCurrentCertificationID(ctx, gen.UpdateCurrentCertificationIDParams{
		AccountID:              accountID,
		CurrentCertificationID: int64Pointer(certID),
		UpdatedAt:              nullableTime(now()),
	})
}

func (r *sellerRepository) CompleteOnboarding(ctx context.Context, accountID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CompleteOnboarding(ctx, gen.CompleteOnboardingParams{
		AccountID: accountID,
		UpdatedAt: nullableTime(now()),
	})
}

func parseStringEnum[T any](s string, parser func(string) (T, error)) (T, error) {
	if s == "" {
		var zero T
		return zero, nil
	}
	return parser(s)
}

func now() time.Time {
	return time.Now()
}

func (r *sellerRepository) UpdateRatingByStoreID(ctx context.Context, storeID int64, rating float64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateSellerRatingByStoreID(ctx, gen.UpdateSellerRatingByStoreIDParams{
		StoreID:   storeID,
		Rating:    float64Pointer(domain.RoundRating(rating)),
		UpdatedAt: nullableTime(now()),
	})
}

func (r *sellerRepository) ListForAdmin(ctx context.Context, filter domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	keyword := ""
	if filter.Keyword != nil {
		keyword = *filter.Keyword
	}

	rows, err := q.ListSellersForAdmin(ctx, gen.ListSellersForAdminParams{
		Column1: certStatusesParam(filter.CertStatuses),
		Column2: keyword,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		Column5: filter.LifecycleStatus,
	})
	if err != nil {
		return nil, err
	}

	sellers := make([]*domain.AdminSellerRow, 0, len(rows))
	for _, row := range rows {
		m, err := listRowToDomainSeller(row)
		if err != nil {
			return nil, err
		}
		sellers = append(sellers, m)
	}
	return sellers, nil
}

func certStatusesParam(statuses []string) []string {
	if statuses == nil {
		return []string{}
	}
	return statuses
}

func (r *sellerRepository) CountForAdmin(ctx context.Context, filter domain.SellerListFilter) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	keyword := ""
	if filter.Keyword != nil {
		keyword = *filter.Keyword
	}

	count, err := q.CountSellersForAdmin(ctx, gen.CountSellersForAdminParams{
		Column1: certStatusesParam(filter.CertStatuses),
		Column2: keyword,
		Column3: filter.LifecycleStatus,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sellerRepository) GetStatsForAdmin(ctx context.Context) (*domain.SellerStats, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	row, err := q.GetSellerStatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}

	return &domain.SellerStats{
		Total:      row.Total,
		Pending:    row.Pending,
		Approved:   row.Approved,
		Rejected:   row.Rejected,
		MonthlyNew: row.MonthlyNew,
	}, nil
}

func (r *sellerRepository) GetByAccountIDForUpdate(ctx context.Context, accountID int64) (*domain.Seller, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerByAccountIDForUpdate(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("seller not found")
		}
		return nil, err
	}
	return toDomainSeller(row)
}

func (r *sellerRepository) ClearPIIFields(ctx context.Context, storeID int64, clearedAt time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.SetSellerPIICleared(ctx, gen.SetSellerPIIClearedParams{
		StoreID:      storeID,
		PiiClearedAt: nullableTime(clearedAt),
	})
}

func (r *sellerRepository) ListPendingPIICleanup(ctx context.Context, before time.Time, limit int) ([]int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.ListSellersPendingPIICleanup(ctx, gen.ListSellersPendingPIICleanupParams{
		DeletedAt: nullableTime(before),
		Limit:     int32(limit),
	})
}

func (r *sellerRepository) RetireCurrentStint(ctx context.Context, accountID int64, nowTime time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.RetireCurrentSellerStint(ctx, gen.RetireCurrentSellerStintParams{
		AccountID: accountID,
		UpdatedAt: nullableTime(nowTime),
	})
}

func (r *sellerRepository) UpdateStatusByStoreID(ctx context.Context, storeID int64, status domain.SellerStatus, unlinkedAt *time.Time, deletedAt *time.Time, nowTime time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateSellerStatusByStoreID(ctx, gen.UpdateSellerStatusByStoreIDParams{
		StoreID:                 storeID,
		Status:                  string(status),
		PayuniAccountUnlinkedAt: nullableTimePointer(unlinkedAt),
		DeletedAt:               nullableTimePointer(deletedAt),
		UpdatedAt:               nullableTime(nowTime),
	})
}

func (r *sellerRepository) UpdateStatusCurrentByAccountID(ctx context.Context, accountID int64, status domain.SellerStatus, nowTime time.Time) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateSellerStatusCurrentByAccountID(ctx, gen.UpdateSellerStatusCurrentByAccountIDParams{
		AccountID: accountID,
		Status:    string(status),
		UpdatedAt: nullableTime(nowTime),
	})
}

func listRowToDomainSeller(row gen.ListSellersForAdminRow) (*domain.AdminSellerRow, error) {
	rawSellerType := stringValue(row.SellerType)
	if row.CertSellerType != "" {
		rawSellerType = row.CertSellerType
	}
	sellerType, err := parseStringEnum(rawSellerType, domain.SellerTypeString)
	if err != nil {
		return nil, err
	}
	certifiedAt := row.CertifiedAt
	if t := row.PayuniApprovedAt; t != nil {
		certifiedAt = t
	}

	var certID *int64
	if row.CurrentCertificationID != nil {
		certID = row.CurrentCertificationID
	}

	return &domain.AdminSellerRow{
		Seller: domain.Seller{
			AccountID:              row.AccountID,
			StoreID:                row.StoreID,
			Status:                 domain.SellerStatus(row.Status),
			StoreName:              row.StoreName,
			StoreCode:              row.StoreCode,
			Description:            stringValue(row.Description),
			Logo:                   stringValue(row.Logo),
			CoverKey:               stringValue(row.CoverKey),
			SeoTitle:               stringValue(row.SeoTitle),
			SeoDescription:         stringValue(row.SeoDescription),
			ContactPhone:           stringValue(row.ContactPhone),
			Email:                  stringValue(row.Email),
			City:                   stringValue(row.City),
			District:               stringValue(row.District),
			AddressDetail:          stringValue(row.AddressDetail),
			PostalCode:             stringValue(row.PostalCode),
			Rating:                 float64Value(row.Rating),
			CurrentCertificationID: certID,
			SellerType:             sellerType,
			CertifiedAt:            certifiedAt,
			CreatedAt:              timeValue(row.CreatedAt),
			UpdatedAt:              timeValue(row.UpdatedAt),
		},
		CertStatus:         row.CertStatus,
		PayuniMerStatus:    row.PayuniMerStatus,
		PayuniMemberStatus: row.PayuniMemberStatus,
	}, nil
}
