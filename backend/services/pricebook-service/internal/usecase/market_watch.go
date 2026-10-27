package usecase

import (
	"context"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"strings"
)

type MarketWatch struct {
	repo  domain.MarketWatchRepository
	newID func() int64
}

func NewMarketWatch(repo domain.MarketWatchRepository, newID func() int64) *MarketWatch {
	return &MarketWatch{repo: repo, newID: newID}
}
func (u *MarketWatch) Create(ctx context.Context, accountID int64, cropCode string, marketCode *string) (domain.MarketWatchTargetResult, error) {
	if accountID <= 0 || strings.TrimSpace(cropCode) == "" || (marketCode != nil && strings.TrimSpace(*marketCode) == "") {
		return domain.MarketWatchTargetResult{}, domain.ErrInvalidWatchTarget
	}
	market := ""
	if marketCode != nil {
		market = *marketCode
	}
	return u.repo.Create(ctx, accountID, u.newID(), cropCode, market)
}
func (u *MarketWatch) Delete(ctx context.Context, accountID int64, watchID string) error {
	id, err := positiveDecimal(watchID)
	if err != nil || accountID <= 0 {
		return domain.ErrInvalidWatchTarget
	}
	return u.repo.Delete(ctx, accountID, id)
}
