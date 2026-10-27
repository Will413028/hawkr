package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

var ErrCommodityNotFound = errors.New("usecase: 查無 commodity")

type CommodityForecastResult struct {
	CommodityID   int64
	Name          string
	PriceLayer    domain.PriceLayer
	Unit          string
	Status        forecast.PredictionStatus
	Derivation    string
	SourceDate    time.Time
	TargetDate    time.Time
	P50           *float64
	CanonicalUnit string
	OriginalP50   *float64
	MarketCount   int
	QuantityUnit  string
	TotalQuantity *float64
}

type CommodityForecast struct{ pool *pgxpool.Pool }

func NewCommodityForecast(pool *pgxpool.Pool) *CommodityForecast {
	return &CommodityForecast{pool: pool}
}

func (u *CommodityForecast) Handle(ctx context.Context, commodityCode string, cutoff time.Time) (CommodityForecastResult, error) {
	id, err := strconv.ParseInt(commodityCode, 10, 64)
	if err != nil {
		return CommodityForecastResult{}, fmt.Errorf("%w: commodity code %q 不是合法 id", ErrCommodityNotFound, commodityCode)
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))
	c, err := q.GetCommodityByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommodityForecastResult{}, fmt.Errorf("%w: id=%d", ErrCommodityNotFound, id)
		}
		return CommodityForecastResult{}, fmt.Errorf("查詢 commodity 失敗: %w", err)
	}

	out := CommodityForecastResult{
		CommodityID: c.ID,
		Name:        c.Name,
		PriceLayer:  domain.PriceLayer(c.PriceLayer),
		Unit:        c.Unit,
		Derivation:  domain.ForecastDerivationSourcePassthrough,
		Status:      forecast.PredictionStatusInsufficientData,
	}

	rows, err := q.ListCommodityObservationsOnLatestDate(ctx, gen.ListCommodityObservationsOnLatestDateParams{
		CommodityID: id,
		Cutoff:      pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return out, fmt.Errorf("查詢 commodity 觀測失敗: %w", err)
	}
	if len(rows) == 0 {
		return out, nil
	}

	agg, aerr := aggregateCommodityDay(toCommodityDayRows(rows))
	if aerr != nil {
		return out, aerr
	}
	if agg.MarketCount == 0 {
		return out, nil
	}
	p50 := agg.Price

	sourceDate := rows[0].TradeDate.Time
	out.Status = forecast.PredictionStatusReady
	if c.CanonicalUnit != nil {
		out.CanonicalUnit = *c.CanonicalUnit
		out.P50 = &p50

		if kg, ok := domain.KgPerUnit(c.Unit); ok {
			orig := p50 * kg
			out.OriginalP50 = &orig
		}
	} else {
		orig := p50
		out.OriginalP50 = &orig
	}
	out.SourceDate = sourceDate

	out.TargetDate = sourceDate.AddDate(0, 0, 1)
	out.MarketCount = agg.MarketCount
	out.QuantityUnit = agg.QuantityUnit
	out.TotalQuantity = agg.TotalQuantity
	return out, nil
}

type commodityDayRow struct {
	CanonicalPrice pgtype.Numeric
	AvgPrice       pgtype.Numeric
	Quantity       pgtype.Numeric
	QuantityUnit   *string
}

func toCommodityDayRows(rows []gen.ListCommodityObservationsOnLatestDateRow) []commodityDayRow {
	out := make([]commodityDayRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, commodityDayRow{
			CanonicalPrice: r.CanonicalPrice, AvgPrice: r.AvgPrice,
			Quantity: r.Quantity, QuantityUnit: r.QuantityUnit,
		})
	}
	return out
}

type commodityDayAggregate struct {
	Price         float64
	MarketCount   int
	TotalQuantity *float64
	QuantityUnit  string
}

func aggregateCommodityDay(rows []commodityDayRow) (commodityDayAggregate, error) {
	var out commodityDayAggregate
	var weighted, weightSum, plain, totalQty float64
	var plainCount int

	for _, r := range rows {
		price, perr := numericFloat(r.CanonicalPrice)
		if perr != nil {
			return out, fmt.Errorf("canonical_price 轉換失敗: %w", perr)
		}
		if price == nil {
			price, perr = numericFloat(r.AvgPrice)
			if perr != nil {
				return out, fmt.Errorf("avg_price 轉換失敗: %w", perr)
			}
		}
		if price == nil {
			continue
		}
		out.MarketCount++
		plain += *price
		plainCount++
		if r.QuantityUnit != nil {
			out.QuantityUnit = *r.QuantityUnit
		}
		qty, qerr := numericFloat(r.Quantity)
		if qerr != nil {
			return out, fmt.Errorf("quantity 轉換失敗: %w", qerr)
		}
		if qty != nil && *qty > 0 {
			weighted += *price * *qty
			weightSum += *qty
			totalQty += *qty
		}
	}
	if out.MarketCount == 0 {
		return out, nil
	}
	if weightSum > 0 {
		out.Price = weighted / weightSum
		out.TotalQuantity = &totalQty
		return out, nil
	}

	out.Price = plain / float64(plainCount)
	return out, nil
}

func numericFloat(n pgtype.Numeric) (*float64, error) {
	s := postgres.NumericToPtr(n)
	if s == nil {
		return nil, nil
	}
	v, err := strconv.ParseFloat(*s, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type CommodityObservedPoint struct {
	TradeDate time.Time
	Price     float64
}

type CommodityObservedSeriesResult struct {
	CommodityID   int64
	Name          string
	PriceLayer    domain.PriceLayer
	Unit          string
	CanonicalUnit string
	Points        []CommodityObservedPoint
}

func (u *CommodityForecast) ObservedSeries(ctx context.Context, commodityID int64, from, to, cutoff time.Time) (CommodityObservedSeriesResult, error) {
	if cutoff.IsZero() {
		return CommodityObservedSeriesResult{}, fmt.Errorf("usecase: cutoff 不可為零值（零值 time.Time 會靜默變成 SQL NULL，每一支 as-of 查詢都回 0 列且 err=nil）")
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))
	c, err := q.GetCommodityByID(ctx, commodityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommodityObservedSeriesResult{}, fmt.Errorf("%w: id=%d", ErrCommodityNotFound, commodityID)
		}
		return CommodityObservedSeriesResult{}, fmt.Errorf("查詢 commodity 失敗: %w", err)
	}

	out := CommodityObservedSeriesResult{
		CommodityID: c.ID, Name: c.Name,
		PriceLayer: domain.PriceLayer(c.PriceLayer), Unit: c.Unit,
	}
	if c.CanonicalUnit != nil {
		out.CanonicalUnit = *c.CanonicalUnit
	}

	rows, err := q.ListLatestCommodityObservationsAsOf(ctx, gen.ListLatestCommodityObservationsAsOfParams{
		CommodityID: commodityID,
		Cutoff:      pgxutil.TimestamptzParam(cutoff),
		FromDate:    pgxutil.DateParam(&from),
	})
	if err != nil {
		return out, fmt.Errorf("查詢 commodity 觀測序列失敗: %w", err)
	}

	byDate := map[string][]commodityDayRow{}
	dates := map[string]time.Time{}
	for _, r := range rows {
		day := r.TradeDate.Time
		if day.After(to) {
			continue
		}
		key := day.Format("2006-01-02")
		dates[key] = day
		byDate[key] = append(byDate[key], commodityDayRow{
			CanonicalPrice: r.CanonicalPrice, AvgPrice: r.AvgPrice,
			Quantity: r.Quantity, QuantityUnit: r.QuantityUnit,
		})
	}

	keys := make([]string, 0, len(byDate))
	for k := range byDate {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	scale := 1.0
	if out.CanonicalUnit != "" {
		if kg, ok := domain.KgPerUnit(c.Unit); ok {
			scale = kg
		}
	}

	for _, key := range keys {
		agg, aerr := aggregateCommodityDay(byDate[key])
		if aerr != nil {
			return out, aerr
		}
		if agg.MarketCount == 0 {
			continue
		}
		out.Points = append(out.Points, CommodityObservedPoint{
			TradeDate: dates[key], Price: agg.Price * scale,
		})
	}
	return out, nil
}
