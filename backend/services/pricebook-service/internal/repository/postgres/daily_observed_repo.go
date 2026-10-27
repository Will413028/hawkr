package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func (r *forecastRepo) DailyObservedPrices(ctx context.Context, cropCodes []string, marketCode string, from, to, cutoff time.Time) ([]domain.DailyObservedPrice, error) {
	if cutoff.IsZero() {
		return nil, fmt.Errorf("daily observed prices: cutoff must not be zero")
	}
	rows, err := gen.New(database.GetDBTX(ctx, r.pool)).DailyObservedPrices(ctx, gen.DailyObservedPricesParams{
		CropCodes:  cropCodes,
		MarketCode: marketCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
		Cutoff:     pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.DailyObservedPrice, 0, len(rows))
	for _, row := range rows {
		price := NumericToPtr(row.AvgPrice)
		if price == nil {
			return nil, fmt.Errorf("daily observed prices: aggregate has no price")
		}
		out = append(out, domain.DailyObservedPrice{
			CropCode:             row.CropCode,
			TradeDate:            row.TradeDate.Time,
			AvgPrice:             *price,
			VolumeKg:             NumericToPtr(row.VolumeKg),
			MarketCodes:          row.MarketCodes,
			SourceDigests:        row.SourceDigests,
			AvailableAt:          row.AvailableAt.Time.UTC(),
			VolumeWeightFallback: row.VolumeWeightFallback,
		})
	}
	return out, nil
}
