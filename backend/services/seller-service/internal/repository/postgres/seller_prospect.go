package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

type sellerProspectRepository struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
	newID     func() int64
}

func NewSellerProspectRepository(pool *pgxpool.Pool, txManager database.TxManager, newID func() int64) domain.SellerProspectRepository {
	return &sellerProspectRepository{pool: pool, txManager: txManager, newID: newID}
}

func (r *sellerProspectRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func (r *sellerProspectRepository) Create(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {

	var factsAsOf *time.Time
	if !in.UpdatedAt.IsZero() {
		t := in.UpdatedAt
		factsAsOf = &t
	}
	row, err := r.q(ctx).CreateSellerProspect(ctx, gen.CreateSellerProspectParams{
		ID:                 r.newID(),
		DisplayName:        in.DisplayName,
		VendorType:         in.VendorType,
		Status:             in.Status,
		OutreachStatus:     in.OutreachStatus,
		TaxID:              in.TaxID,
		MatchEmail:         in.MatchKeys.Email,
		MatchPhone:         in.MatchKeys.Phone,
		MatchDomain:        in.MatchKeys.Domain,
		MatchSocial:        in.MatchKeys.Social,
		PublicEmail:        in.PublicEmail,
		PublicPhone:        in.PublicPhone,
		PublicWebsite:      in.PublicWebsite,
		PublicSocial:       in.PublicSocial,
		Category:           in.Category,
		Region:             in.Region,
		ProductSummary:     in.ProductSummary,
		Positioning:        in.Positioning,
		ResearchSummary:    in.ResearchSummary,
		PriorityLetter:     in.PriorityLetter,
		PriorityStars:      int32(in.PriorityStars),
		CreatedByAccountID: in.CreatedByAccountID,
		UpdatedByAccountID: in.UpdatedByAccountID,
		FactsAsOf:          factsAsOf,
	})
	if err != nil {

		if cn, ok := isUniqueViolation(err); ok {
			return domain.SellerProspect{}, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return domain.SellerProspect{}, err
	}
	return toSellerProspectEntity(row), nil
}

func (r *sellerProspectRepository) Search(ctx context.Context, params domain.ProspectSearchParams) ([]domain.ProspectSearchHit, error) {
	limit := params.Limit
	if limit <= 0 {

		limit = 500
	}
	categoryIDs := params.CategoryIDs
	if categoryIDs == nil {
		categoryIDs = []string{}
	}
	rows, err := r.q(ctx).SearchSellerProspects(ctx, gen.SearchSellerProspectsParams{
		Category:    params.Category,
		Region:      params.Region,
		Keywords:    params.Keywords,
		CategoryIds: categoryIDs,
		RowLimit:    int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProspectSearchHit, len(rows))
	for i, row := range rows {
		out[i] = toProspectSearchHit(row)
	}
	return out, nil
}

func toProspectSearchHit(row gen.SearchSellerProspectsRow) domain.ProspectSearchHit {
	return domain.ProspectSearchHit{
		Prospect:       toSellerProspectEntityFromSearchRow(row),
		HitCategory:    boolValue(row.HitCategory),
		HitRegion:      boolValue(row.HitRegion),
		HitKeywords:    boolValue(row.HitKeywords),
		HitCategoryIDs: boolValue(row.HitCategoryIds),
	}
}

func (r *sellerProspectRepository) FindByTaxID(ctx context.Context, taxID string) (domain.SellerProspect, error) {
	row, err := r.q(ctx).FindSellerProspectByTaxID(ctx, &taxID)
	if err != nil {
		return domain.SellerProspect{}, err
	}
	return toSellerProspectEntity(row), nil
}

func (r *sellerProspectRepository) FindByMatchKeys(ctx context.Context, keys domain.ProspectMatchKeys) ([]domain.SellerProspect, error) {

	if keys.Email == "" && keys.Phone == "" && keys.Domain == "" && keys.Social == "" {
		return nil, nil
	}
	rows, err := r.q(ctx).FindSellerProspectsByMatchKeys(ctx, gen.FindSellerProspectsByMatchKeysParams{
		MatchEmail:  keys.Email,
		MatchPhone:  keys.Phone,
		MatchDomain: keys.Domain,
		MatchSocial: keys.Social,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.SellerProspect, len(rows))
	for i, row := range rows {
		out[i] = toSellerProspectEntity(row)
	}
	return out, nil
}

func (r *sellerProspectRepository) MarkConverted(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) (int64, error) {
	rows, err := r.q(ctx).MarkSellerProspectConverted(ctx, gen.MarkSellerProspectConvertedParams{
		ID:                 prospectID,
		ConvertedStoreID:   &storeID,
		UpdatedByAccountID: operatorAccountID,
	})
	if err != nil {

		if cn, ok := isUniqueViolation(err); ok {
			return 0, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return 0, err
	}
	return rows, nil
}

func (r *sellerProspectRepository) FindUsableInviteByToken(ctx context.Context, token string) (domain.ProspectInvite, error) {
	row, err := r.q(ctx).FindUsableProspectInviteByToken(ctx, token)
	if err != nil {
		return domain.ProspectInvite{}, err
	}
	return toProspectInviteEntity(row), nil
}

func (r *sellerProspectRepository) ConsumeInvite(ctx context.Context, inviteID int64) error {
	rows, err := r.q(ctx).ConsumeProspectInvite(ctx, inviteID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("consume prospect invite %d: already consumed or not found", inviteID)
	}
	return nil
}

func (r *sellerProspectRepository) ListForAdmin(ctx context.Context, params domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	switch {
	case pageSize <= 0:
		pageSize = 20
	case pageSize > 500:
		pageSize = 500
	}

	rows, err := r.q(ctx).ListSellerProspectsForAdmin(ctx, gen.ListSellerProspectsForAdminParams{
		Status:         params.Status,
		OutreachStatus: params.OutreachStatus,
		VendorType:     params.VendorType,
		Category:       params.Category,
		Region:         params.Region,
		Keyword:        params.Keyword,
		PageOffset:     int32((page - 1) * pageSize),
		PageSize:       int32(pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.SellerProspect, len(rows))
	var total int64
	for i, row := range rows {
		out[i] = toSellerProspectEntityFromAdminListRow(row)
		total = row.TotalCount
	}

	if len(rows) == 0 {
		total, err = r.q(ctx).CountSellerProspectsForAdmin(ctx, gen.CountSellerProspectsForAdminParams{
			Status:         params.Status,
			OutreachStatus: params.OutreachStatus,
			VendorType:     params.VendorType,
			Category:       params.Category,
			Region:         params.Region,
			Keyword:        params.Keyword,
		})
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *sellerProspectRepository) GetByID(ctx context.Context, id int64) (domain.SellerProspect, error) {
	row, err := r.q(ctx).GetSellerProspectByID(ctx, id)
	if err != nil {
		return domain.SellerProspect{}, err
	}
	return toSellerProspectEntity(row), nil
}

func (r *sellerProspectRepository) Update(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	row, err := r.q(ctx).UpdateSellerProspect(ctx, gen.UpdateSellerProspectParams{
		ID:                 in.ID,
		DisplayName:        in.DisplayName,
		VendorType:         in.VendorType,
		OutreachStatus:     in.OutreachStatus,
		TaxID:              in.TaxID,
		MatchEmail:         in.MatchKeys.Email,
		MatchPhone:         in.MatchKeys.Phone,
		MatchDomain:        in.MatchKeys.Domain,
		MatchSocial:        in.MatchKeys.Social,
		PublicEmail:        in.PublicEmail,
		PublicPhone:        in.PublicPhone,
		PublicWebsite:      in.PublicWebsite,
		PublicSocial:       in.PublicSocial,
		Category:           in.Category,
		Region:             in.Region,
		ProductSummary:     in.ProductSummary,
		Positioning:        in.Positioning,
		ResearchSummary:    in.ResearchSummary,
		PriorityLetter:     in.PriorityLetter,
		PriorityStars:      int32(in.PriorityStars),
		UpdatedByAccountID: in.UpdatedByAccountID,
	})
	if err != nil {

		if cn, ok := isUniqueViolation(err); ok {
			return domain.SellerProspect{}, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return domain.SellerProspect{}, err
	}
	return toSellerProspectEntity(row), nil
}

func (r *sellerProspectRepository) SetArchived(ctx context.Context, id int64, archived bool, operatorID *int64) (int64, error) {
	rows, err := r.q(ctx).SetSellerProspectArchived(ctx, gen.SetSellerProspectArchivedParams{
		Archived:          archived,
		OperatorAccountID: operatorID,
		ID:                id,
	})
	if err != nil {
		if cn, ok := isUniqueViolation(err); ok {
			return 0, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return 0, err
	}
	return rows, nil
}

func (r *sellerProspectRepository) UpdateMedia(ctx context.Context, in domain.ProspectMediaUpdate) (domain.SellerProspect, error) {
	row, err := r.q(ctx).UpdateSellerProspectMedia(ctx, gen.UpdateSellerProspectMediaParams{
		ID:                   in.ProspectID,
		Kind:                 in.Kind,
		ObjectKey:            in.ObjectKey,
		SourceUrl:            in.SourceURL,
		ExpectedMediaVersion: in.ExpectedMediaVersion,
		OperatorAccountID:    in.OperatorAccountID,
	})
	if err != nil {
		// 零列是述詞擋下來的，不是「查不到」：id 不存在、status 不是 ACTIVE、
		// 已被合併、或版本不符，四種都落在這裡。呼叫端拿 GetByID 分辨不存在，
		// 其餘一律要 Admin 重新讀取後再換一次圖。
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SellerProspect{}, domain.ErrProspectMediaConflict
		}
		return domain.SellerProspect{}, err
	}
	return toSellerProspectEntity(row), nil
}

func (r *sellerProspectRepository) Merge(ctx context.Context, winnerID, loserID int64, operatorID *int64) (domain.SellerProspect, error) {
	if winnerID == loserID {
		return domain.SellerProspect{}, fmt.Errorf("%w: winner and loser are the same prospect (%d)", domain.ErrProspectMergeNotAllowed, winnerID)
	}

	var winner domain.SellerProspect
	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		rows, err := r.q(txCtx).LockSellerProspectsForMerge(txCtx, []int64{winnerID, loserID})
		if err != nil {
			return err
		}
		if len(rows) != 2 {
			return fmt.Errorf("%w: expected 2 prospects, found %d", domain.ErrProspectMergeNotAllowed, len(rows))
		}
		for _, row := range rows {
			if row.Status != "ACTIVE" {
				return fmt.Errorf("%w: prospect %d is %s, only ACTIVE rows can be merged", domain.ErrProspectMergeNotAllowed, row.ID, row.Status)
			}
		}

		var winnerTaxID, loserTaxID *string
		for _, row := range rows {
			switch row.ID {
			case winnerID:
				winnerTaxID = row.TaxID
			case loserID:
				loserTaxID = row.TaxID
			}
		}
		if loserTaxID != nil {
			if _, err := r.q(txCtx).ClearLoserProspectTaxID(txCtx, loserID); err != nil {
				return err
			}
			if winnerTaxID == nil {
				affected, err := r.q(txCtx).CarryProspectTaxIDToWinner(txCtx, gen.CarryProspectTaxIDToWinnerParams{
					TaxID:             *loserTaxID,
					OperatorAccountID: operatorID,
					WinnerID:          winnerID,
				})
				if err != nil {
					return err
				}

				if affected != 1 {
					return fmt.Errorf("carry tax id to winner %d affected %d rows, want 1", winnerID, affected)
				}
			}
		}

		if _, err := r.q(txCtx).MoveProspectInvitesToWinner(txCtx, gen.MoveProspectInvitesToWinnerParams{
			WinnerID: winnerID, LoserID: loserID,
		}); err != nil {
			return err
		}
		if _, err := r.q(txCtx).RepointMergedProspects(txCtx, gen.RepointMergedProspectsParams{
			WinnerID: winnerID, LoserID: loserID,
		}); err != nil {
			return err
		}
		affected, err := r.q(txCtx).ArchiveLoserProspect(txCtx, gen.ArchiveLoserProspectParams{
			WinnerID: winnerID, LoserID: loserID, OperatorAccountID: operatorID,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("%w: loser %d was not ACTIVE at write time", domain.ErrProspectMergeNotAllowed, loserID)
		}

		winner, err = r.GetByID(txCtx, winnerID)
		return err
	})
	if err != nil {

		if cn, ok := isUniqueViolation(err); ok {
			return domain.SellerProspect{}, &domain.ErrDuplicate{ConstraintName: cn, Underlying: err}
		}
		return domain.SellerProspect{}, err
	}
	return winner, nil
}

func (r *sellerProspectRepository) ListDuplicatePairs(ctx context.Context) ([]domain.ProspectDuplicatePair, error) {
	rows, err := r.q(ctx).ListDuplicateProspectPairs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProspectDuplicatePair, len(rows))
	for i, row := range rows {
		out[i] = domain.ProspectDuplicatePair{
			Left:  toSellerProspectEntity(row.SellerProspect),
			Right: toSellerProspectEntity(row.SellerProspect_2),
		}
	}
	return out, nil
}

func (r *sellerProspectRepository) CurrentStoreExists(ctx context.Context, storeID int64) (bool, error) {
	return r.q(ctx).CurrentStoreExists(ctx, storeID)
}

func (r *sellerProspectRepository) ListTaxIDMatches(ctx context.Context) ([]domain.ProspectTaxIDMatch, error) {
	rows, err := r.q(ctx).ListProspectTaxIDMatches(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProspectTaxIDMatch, len(rows))
	for i, row := range rows {
		out[i] = domain.ProspectTaxIDMatch{
			Prospect:                 toSellerProspectEntity(row.SellerProspect),
			StoreID:                  row.StoreID,
			StoreName:                row.StoreName,
			CertificationCompanyName: stringValue(row.CompanyName),
			CertificationStatus:      row.CertificationStatus,
		}
	}
	return out, nil
}

func (r *sellerProspectRepository) CreateInvite(ctx context.Context, prospectID int64, token string, expiresAt time.Time) (domain.ProspectInvite, error) {
	row, err := r.q(ctx).CreateProspectInvite(ctx, gen.CreateProspectInviteParams{
		ID:         r.newID(),
		ProspectID: prospectID,
		Token:      token,
		ExpiresAt:  timestamptzParam(expiresAt),
	})
	if err != nil {
		return domain.ProspectInvite{}, err
	}
	return toProspectInviteEntity(row), nil
}

func (r *sellerProspectRepository) RevokeInvite(ctx context.Context, prospectID, inviteID int64) (int64, error) {
	return r.q(ctx).RevokeProspectInvite(ctx, gen.RevokeProspectInviteParams{
		ProspectID: prospectID,
		ID:         inviteID,
	})
}

func (r *sellerProspectRepository) ListInvites(ctx context.Context, prospectID int64) ([]domain.ProspectInvite, error) {
	rows, err := r.q(ctx).ListProspectInvites(ctx, prospectID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProspectInvite, len(rows))
	for i, row := range rows {
		out[i] = toProspectInviteEntity(row)
	}
	return out, nil
}

func toProspectInviteEntity(row gen.SellerProspectInvite) domain.ProspectInvite {
	return domain.ProspectInvite{
		ID:         row.ID,
		ProspectID: row.ProspectID,
		ExpiresAt:  row.ExpiresAt.Time,
		SentAt:     row.SentAt,
		ClickedAt:  row.ClickedAt,
		ConsumedAt: row.ConsumedAt,
		RevokedAt:  row.RevokedAt,

		CreatedAt: row.CreatedAt.Time,
	}
}

func toSellerProspectEntity(row gen.SellerProspect) domain.SellerProspect {
	return domain.SellerProspect{
		ID:               row.ID,
		DisplayName:      row.DisplayName,
		VendorType:       row.VendorType,
		Status:           row.Status,
		OutreachStatus:   row.OutreachStatus,
		TaxID:            row.TaxID,
		ConvertedStoreID: row.ConvertedStoreID,
		ConvertedAt:      row.ConvertedAt,
		MatchKeys: domain.ProspectMatchKeys{
			TaxID:  stringValue(row.TaxID),
			Email:  row.MatchEmail,
			Phone:  row.MatchPhone,
			Domain: row.MatchDomain,
			Social: row.MatchSocial,
		},
		PublicEmail:        row.PublicEmail,
		PublicPhone:        row.PublicPhone,
		PublicWebsite:      row.PublicWebsite,
		PublicSocial:       row.PublicSocial,
		Category:           row.Category,
		Region:             row.Region,
		ProductSummary:     row.ProductSummary,
		Positioning:        row.Positioning,
		ResearchSummary:    row.ResearchSummary,
		PriorityLetter:     row.PriorityLetter,
		PriorityStars:      int(row.PriorityStars),
		CreatedByAccountID: row.CreatedByAccountID,
		UpdatedByAccountID: row.UpdatedByAccountID,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,

		MergedIntoProspectID: row.MergedIntoProspectID,

		LogoKey:        row.LogoKey,
		CoverKey:       row.CoverKey,
		LogoSourceURL:  row.LogoSourceUrl,
		CoverSourceURL: row.CoverSourceUrl,
		MediaVersion:   row.MediaVersion,
	}
}

func toSellerProspectEntityFromSearchRow(row gen.SearchSellerProspectsRow) domain.SellerProspect {
	return toSellerProspectEntity(gen.SellerProspect{
		ID: row.ID, DisplayName: row.DisplayName, VendorType: row.VendorType,
		Status: row.Status, OutreachStatus: row.OutreachStatus, TaxID: row.TaxID,
		ConvertedStoreID: row.ConvertedStoreID, ConvertedAt: row.ConvertedAt,
		MatchEmail: row.MatchEmail, MatchPhone: row.MatchPhone,
		MatchDomain: row.MatchDomain, MatchSocial: row.MatchSocial,
		PublicEmail: row.PublicEmail, PublicPhone: row.PublicPhone,
		PublicWebsite: row.PublicWebsite, PublicSocial: row.PublicSocial,
		Category: row.Category, Region: row.Region,
		ProductSummary: row.ProductSummary, Positioning: row.Positioning,
		ResearchSummary: row.ResearchSummary,
		PriorityLetter:  row.PriorityLetter, PriorityStars: row.PriorityStars,
		CreatedByAccountID: row.CreatedByAccountID, UpdatedByAccountID: row.UpdatedByAccountID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		LogoKey:  row.LogoKey,
		CoverKey: row.CoverKey,
	})
}

func toSellerProspectEntityFromAdminListRow(row gen.ListSellerProspectsForAdminRow) domain.SellerProspect {
	return toSellerProspectEntity(gen.SellerProspect{
		ID: row.ID, DisplayName: row.DisplayName, VendorType: row.VendorType,
		Status: row.Status, OutreachStatus: row.OutreachStatus, TaxID: row.TaxID,
		ConvertedStoreID: row.ConvertedStoreID, ConvertedAt: row.ConvertedAt,
		MatchEmail: row.MatchEmail, MatchPhone: row.MatchPhone,
		MatchDomain: row.MatchDomain, MatchSocial: row.MatchSocial,
		PublicEmail: row.PublicEmail, PublicPhone: row.PublicPhone,
		PublicWebsite: row.PublicWebsite, PublicSocial: row.PublicSocial,
		Category: row.Category, Region: row.Region,
		ProductSummary: row.ProductSummary, Positioning: row.Positioning,
		ResearchSummary: row.ResearchSummary,
		PriorityLetter:  row.PriorityLetter, PriorityStars: row.PriorityStars,
		CreatedByAccountID: row.CreatedByAccountID, UpdatedByAccountID: row.UpdatedByAccountID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		MergedIntoProspectID: row.MergedIntoProspectID,
		LogoKey:              row.LogoKey,
		CoverKey:             row.CoverKey,
		LogoSourceUrl:        row.LogoSourceUrl,
		CoverSourceUrl:       row.CoverSourceUrl,
		MediaVersion:         row.MediaVersion,
	})
}
