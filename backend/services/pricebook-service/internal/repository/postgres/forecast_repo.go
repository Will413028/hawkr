package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type forecastRepo struct {
	pool *pgxpool.Pool
}

func NewForecastRepository(pool *pgxpool.Pool) domain.ForecastRepository {
	return &forecastRepo{pool: pool}
}

func numericToFloatPtr(n pgtype.Numeric, label string) (*float64, error) {
	s := NumericToPtr(n)
	if s == nil {
		return nil, nil
	}
	v, err := strconv.ParseFloat(*s, 64)
	if err != nil {
		return nil, fmt.Errorf("parse %s %q: %w", label, *s, err)
	}
	return &v, nil
}

func (r *forecastRepo) SeriesObservations(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.Observation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.SeriesObservationsAsOf(ctx, gen.SeriesObservationsAsOfParams{
		MarketCode: marketCode,
		CropCode:   cropCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
		Cutoff:     pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Observation, 0, len(rows))
	for _, row := range rows {
		obs, err := toObservation(row.MarketCode, row.CropCode, row.CategoryCode, row.MarketActive, row.CropActive,
			row.TradeDate, row.AvgPrice, row.VolumeKg, row.AvailableAt, row.RevisionNo)
		if err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, nil
}

func toObservation(marketCode, cropCode string, categoryCode *string, marketActive, cropActive bool,
	tradeDate pgtype.Date, avgPrice, volumeKg pgtype.Numeric, availableAt pgtype.Timestamptz, revisionNo int32,
) (domain.Observation, error) {
	price, err := numericToFloatPtr(avgPrice, "avg_price")
	if err != nil {
		return domain.Observation{}, err
	}
	volume, err := numericToFloatPtr(volumeKg, "volume_kg")
	if err != nil {
		return domain.Observation{}, err
	}
	return domain.Observation{
		MarketCode: marketCode, CropCode: cropCode, CategoryCode: categoryCode,
		MarketActive: marketActive, CropActive: cropActive,
		TradeDate: tradeDate.Time, AvgPrice: price, VolumeKg: volume,
		AvailableAt: availableAt.Time, RevisionNo: revisionNo,
	}, nil
}

func (r *forecastRepo) GroupObservations(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.Observation, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GroupObservationsAsOf(ctx, gen.GroupObservationsAsOfParams{
		GroupCode:  groupCode,
		MarketCode: marketCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
		Cutoff:     pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Observation, 0, len(rows))
	for _, row := range rows {
		obs, err := toObservation(row.MarketCode, row.CropCode, row.CategoryCode, row.MarketActive, row.CropActive,
			row.TradeDate, row.AvgPrice, row.VolumeKg, row.AvailableAt, row.RevisionNo)
		if err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, nil
}

func (r *forecastRepo) RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.RestDaysForMarket(ctx, gen.RestDaysForMarketParams{
		MarketCode: marketCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.RestDay, 0, len(rows))
	for _, row := range rows {
		if row.MarketType == nil {
			continue
		}
		out = append(out, domain.RestDay{
			MarketCode: row.MarketCode,
			MarketType: *row.MarketType,
			RestDate:   row.RestDate.Time,
		})
	}
	return out, nil
}

func (r *forecastRepo) RestDayCoverage(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.RestDayCoverageForMarket(ctx, marketCode)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RestDayCoverage, 0, len(rows))
	for _, row := range rows {
		if row.MarketType == nil {
			continue
		}
		out = append(out, domain.RestDayCoverage{
			MarketCode: row.MarketCode,
			MarketType: *row.MarketType,
			Year:       int(row.CalendarYear),
		})
	}
	return out, nil
}

func (r *forecastRepo) CropByCode(ctx context.Context, cropCode string) (domain.CropRef, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.FindCropByCode(ctx, cropCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CropRef{}, nil
		}
		return domain.CropRef{}, fmt.Errorf("forecastRepo.CropByCode(%s): %w", cropCode, err)
	}
	return domain.CropRef{CropCode: row.CropCode, Name: row.Name, CategoryCode: row.CategoryCode}, nil
}

func (r *forecastRepo) CropsInGroup(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.CropsInGroup(ctx, groupCode)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CropRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CropRef{
			CropCode:     row.CropCode,
			Name:         row.Name,
			VariantName:  row.VariantName,
			CategoryCode: row.CategoryCode,
			Active:       row.Active,
		})
	}
	return out, nil
}

func (r *forecastRepo) RecentVolume(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.RecentVolumeByCropMarket(ctx, gen.RecentVolumeByCropMarketParams{
		GroupCode: groupCode,
		FromDate:  pgxutil.DateParam(&from),
		ToDate:    pgxutil.DateParam(&to),
		Cutoff:    pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.CropMarketVolume, 0, len(rows))
	for _, row := range rows {
		v, err := numericToFloatPtr(row.RecentVolume, "recent_volume")
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		out = append(out, domain.CropMarketVolume{
			MarketCode: row.MarketCode,
			CropCode:   row.CropCode,
			Volume:     *v,
		})
	}
	return out, nil
}

func (r *forecastRepo) RecentVolumeForCrop(ctx context.Context, cropCode string, from, to, cutoff time.Time) ([]domain.MarketVolume, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.RecentVolumeByCrop(ctx, gen.RecentVolumeByCropParams{
		CropCode: cropCode,
		FromDate: pgxutil.DateParam(&from),
		ToDate:   pgxutil.DateParam(&to),
		Cutoff:   pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MarketVolume, 0, len(rows))
	for _, row := range rows {
		v, err := numericToFloatPtr(row.RecentVolume, "recent_volume")
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		out = append(out, domain.MarketVolume{
			MarketCode: row.MarketCode,
			Volume:     *v,
		})
	}
	return out, nil
}

func (r *forecastRepo) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ObservedPriceByCrop(ctx, gen.ObservedPriceByCropParams{
		MarketCode: marketCode,
		CropCode:   cropCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
		Cutoff:     pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ObservedPrice, 0, len(rows))
	for _, row := range rows {
		p, err := toObservedPrice(row.MarketCode, row.CropCode, row.CategoryCode, row.TradeDate, row.AvgPrice, row.UpperPrice, row.MiddlePrice, row.LowerPrice, row.VolumeKg,
			row.Source, row.SourceDigest, row.AvailableAt, row.RevisionNo)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *forecastRepo) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ObservedPriceByGroup(ctx, gen.ObservedPriceByGroupParams{
		GroupCode:  groupCode,
		MarketCode: marketCode,
		FromDate:   pgxutil.DateParam(&from),
		ToDate:     pgxutil.DateParam(&to),
		Cutoff:     pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ObservedPrice, 0, len(rows))
	for _, row := range rows {
		p, err := toObservedPrice(row.MarketCode, row.CropCode, row.CategoryCode, row.TradeDate, row.AvgPrice, row.UpperPrice, row.MiddlePrice, row.LowerPrice, row.VolumeKg,
			row.Source, row.SourceDigest, row.AvailableAt, row.RevisionNo)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func toObservedPrice(marketCode, cropCode string, categoryCode *string, tradeDate pgtype.Date, avg, upper, middle, lower, volume pgtype.Numeric,
	source, sourceDigest string, availableAt pgtype.Timestamptz, revisionNo int32,
) (domain.ObservedPrice, error) {
	avgPtr, err := numericToFloatPtr(avg, "avg_price")
	if err != nil {
		return domain.ObservedPrice{}, err
	}
	upperPtr, err := numericToFloatPtr(upper, "upper_price")
	if err != nil {
		return domain.ObservedPrice{}, err
	}
	middlePtr, err := numericToFloatPtr(middle, "middle_price")
	if err != nil {
		return domain.ObservedPrice{}, err
	}
	lowerPtr, err := numericToFloatPtr(lower, "lower_price")
	if err != nil {
		return domain.ObservedPrice{}, err
	}
	volumePtr, err := numericToFloatPtr(volume, "volume_kg")
	if err != nil {
		return domain.ObservedPrice{}, err
	}
	return domain.ObservedPrice{
		MarketCode:   marketCode,
		CropCode:     cropCode,
		CategoryCode: categoryCode,
		TradeDate:    tradeDate.Time,
		AvgPrice:     avgPtr,
		UpperPrice:   upperPtr,
		MiddlePrice:  middlePtr,
		LowerPrice:   lowerPtr,
		VolumeKg:     volumePtr,
		Source:       source,
		SourceDigest: sourceDigest,
		RevisionNo:   revisionNo,
		AvailableAt:  availableAt.Time,
	}, nil
}

func (r *forecastRepo) CropGroupsByName(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.FindCropGroupByName(ctx, name)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CropGroupMatch, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CropGroupMatch{
			GroupCode:    row.GroupCode,
			Name:         row.Name,
			CategoryCode: row.CategoryCode,
		})
	}
	return out, nil
}

func (r *forecastRepo) MarketByCode(ctx context.Context, marketCode string) (domain.MarketRef, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetMarketByCode(ctx, marketCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.MarketRef{}, nil
		}
		return domain.MarketRef{}, fmt.Errorf("forecastRepo.MarketByCode(%s): %w", marketCode, err)
	}
	return domain.MarketRef{MarketCode: row.MarketCode, Name: row.Name}, nil
}

func (r *forecastRepo) MarketsByCode(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error) {
	if len(marketCodes) == 0 {
		return nil, nil
	}
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetMarketsByCode(ctx, marketCodes)
	if err != nil {
		return nil, fmt.Errorf("forecastRepo.MarketsByCode(%d codes): %w", len(marketCodes), err)
	}
	out := make([]domain.MarketRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MarketRef{MarketCode: row.MarketCode, Name: row.Name})
	}
	return out, nil
}

func (r *forecastRepo) GroupCropOptionsByRecentVolume(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	rows, err := q.ListGroupCropOptionsByRecentVolume(ctx, gen.ListGroupCropOptionsByRecentVolumeParams{
		GroupCode: groupCode, WindowDays: windowDays, RowLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("forecastRepo.GroupCropOptionsByRecentVolume: %w", err)
	}

	out := make([]domain.CatalogSearchRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CatalogSearchRow{
			MatchKind: "crop", GroupCode: row.GroupCode, GroupName: row.GroupName,
			CropCode: row.CropCode, CropName: row.Name, VariantName: row.VariantName,
		})
	}
	return out, nil
}
