package usecase

import (
	"context"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

const (
	// defaultListGroupCropOptionsLimit mirrors defaultSearchCropCatalogLimit
	// — the two RPCs feed the same picker, so a caller that omits the limit
	// gets the same amount of material from either.
	defaultListGroupCropOptionsLimit = 20

	// listGroupCropOptionsWindowDays is the trailing window the popularity
	// ranking counts trades in. Six months rather than a month: MOA produce
	// is seasonal, and a one-month window would rank a crop that is simply
	// out of season below one that barely trades at all year-round. It is
	// deliberately not a cutoff — see the SQL's own comment on why crops
	// with zero rows in the window are kept and sorted last.
	listGroupCropOptionsWindowDays = 180
)

// ListGroupCropOptions returns one crop_group's crops as picker rows,
// most-recently-traded first, for a caller that already resolved a group
// and now needs to offer its sub-variants (the procurement assistant's
// 品項 drill-down).
//
// It lives on SearchCropCatalog rather than in a struct of its own because
// it is the same catalog concern over the same three dependencies, and it
// reuses rowToMatch/marketsForCrop/resolveMarketNames verbatim — a separate
// usecase would have to either duplicate those or export them.
//
// HasForecast per row comes from rowToMatch, i.e. whether that specific crop
// has a trained series. Rows where it is false are returned, not filtered:
// this method reports what the group contains, and which of those a caller
// considers eligible is the caller's rule (ai-service's ResolveCrop drops
// them). Filtering here would make the two RPCs disagree about the same
// crop, since SearchCropCatalog returns unforecastable crops too.
func (u *SearchCropCatalog) ListGroupCropOptions(ctx context.Context, groupCode string, limit int32) ([]CatalogMatch, error) {
	if limit <= 0 {
		limit = defaultListGroupCropOptionsLimit
	}

	rows, err := u.repo.GroupCropOptionsByRecentVolume(ctx, groupCode, listGroupCropOptionsWindowDays, limit)
	if err != nil {
		return nil, err
	}

	out := make([]CatalogMatch, 0, len(rows))
	for _, row := range rows {
		out = append(out, u.rowToMatch(ctx, row))
	}
	// Same ordering as SearchCropCatalog's own call: after the row set is
	// final, so nothing is looked up for a row that got dropped.
	if err := u.resolveMarketNames(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// rowToMatch converts one recent-volume row into a picker row; the same
// builders SearchCropCatalog.hitToMatch uses, so the two RPCs can't drift.
func (u *SearchCropCatalog) rowToMatch(ctx context.Context, row domain.CatalogSearchRow) CatalogMatch {
	if row.MatchKind == "group" {
		return u.groupMatchRow(ctx, row.GroupCode, row.GroupName)
	}
	return u.cropMatchRow(row.GroupCode, row.GroupName, row.CropCode, row.CropName, row.VariantName)
}
