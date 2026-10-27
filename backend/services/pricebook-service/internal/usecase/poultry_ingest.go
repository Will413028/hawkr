package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const (
	SourcePoultryChickenEgg = "MOA_POULTRY_CHICKEN_EGG"
	SourcePoultryGooseDuck  = "MOA_POULTRY_GOOSE_DUCK"
	poultryMappingVersion   = "pricebook-commodity-v1"
)

type PoultryIngestStats struct {
	Days     int
	Inserted int
	Revised  int
	Skipped  int
	Absent   int
}

type poultryClient interface {
	FetchPoultry(ctx context.Context, ep moa.PoultryEndpoint, start, end time.Time) ([]moa.PoultryRow, error)
}

type PoultryIngest struct {
	client   poultryClient
	pool     *pgxpool.Pool
	tsid     func() int64
	endpoint moa.PoultryEndpoint
	source   string
}

type PoultryChickenEggIngest struct{ *PoultryIngest }

type PoultryGooseDuckIngest struct{ *PoultryIngest }

func NewPoultryChickenEggIngest(c *moa.Client, pool *pgxpool.Pool, tsid func() int64) *PoultryChickenEggIngest {
	return &PoultryChickenEggIngest{&PoultryIngest{
		client: c, pool: pool, tsid: tsid,
		endpoint: moa.PoultryChickenEgg, source: SourcePoultryChickenEgg,
	}}
}

func NewPoultryGooseDuckIngest(c *moa.Client, pool *pgxpool.Pool, tsid func() int64) *PoultryGooseDuckIngest {
	return &PoultryGooseDuckIngest{&PoultryIngest{
		client: c, pool: pool, tsid: tsid,
		endpoint: moa.PoultryGooseDuck, source: SourcePoultryGooseDuck,
	}}
}

func NewPoultryIngestWithClient(c poultryClient, pool *pgxpool.Pool, tsid func() int64, ep moa.PoultryEndpoint, source string) *PoultryIngest {
	return &PoultryIngest{client: c, pool: pool, tsid: tsid, endpoint: ep, source: source}
}

func poultryDigest(field string, day time.Time, price float64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.4f", field, day.Format("2006-01-02"), price)))
	return hex.EncodeToString(sum[:])
}

func (u *PoultryIngest) Handle(ctx context.Context, start, end time.Time) (PoultryIngestStats, error) {
	stats := PoultryIngestStats{}
	q := gen.New(database.GetDBTX(ctx, u.pool))

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		day := d
		if err := q.DeclareCommodityIngestPartition(ctx, gen.DeclareCommodityIngestPartitionParams{
			Source: u.source, TradeDate: pgxutil.DateParam(&day), Status: domain.PartitionStatusPending,
		}); err != nil {
			return stats, fmt.Errorf("宣告家禽攝取分區失敗（%s）: %w", day.Format("2006-01-02"), err)
		}
	}

	rows, err := u.client.FetchPoultry(ctx, u.endpoint, start, end)
	if err != nil {
		return stats, err
	}
	stats.Days = len(rows)

	commodityIDs := make(map[string]int64, len(u.endpoint.Specs))
	canonicalUnit := domain.CanonicalUnitFor(domain.UnitCatty)
	for _, spec := range u.endpoint.Specs {
		id, cerr := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
			ID: u.tsid(), Source: u.source, CommodityCode: spec.Field, Name: spec.Name,
			PriceLayer: spec.PriceLayer, Unit: domain.UnitCatty,
			CanonicalUnit: &canonicalUnit,
		})
		if cerr != nil {
			return stats, fmt.Errorf("寫入家禽品項失敗（%s）: %w", spec.Field, cerr)
		}
		if err := registerCommodityForms(ctx, q, id, spec.Name, spec.SourceTerms, nil); err != nil {
			return stats, err
		}
		if err := declareCommodityCategories(ctx, q, id, spec.CategoryIDs, poultryMappingVersion, spec.Name); err != nil {
			return stats, err
		}
		commodityIDs[spec.Field] = id
	}

	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())
	for _, row := range rows {
		day := row.TradeDate
		dayParam := pgxutil.DateParam(&day)
		for _, spec := range u.endpoint.Specs {
			price, ok := row.Prices[spec.Field]
			if !ok {
				stats.Absent++
				continue
			}
			if err := u.writeOne(ctx, q, commodityIDs[spec.Field], spec, day, dayParam, price, availableAt, &stats); err != nil {
				return stats, err
			}
		}
		now := time.Now().UTC()
		if err := q.MarkCommodityIngestPartitionAttempt(ctx, gen.MarkCommodityIngestPartitionAttemptParams{
			Source: u.source, TradeDate: dayParam, Status: domain.PartitionStatusComplete,
			LastAttemptAt: &now, CompletedAt: &now,
		}); err != nil {
			zap.L().Warn("poultry_ingest: 更新攝取分區狀態失敗", zap.Error(err))
		}
	}

	zap.L().Info("poultry_ingest 完成",
		zap.String("source", u.source), zap.Int("days", stats.Days),
		zap.Int("inserted", stats.Inserted), zap.Int("revised", stats.Revised),
		zap.Int("skipped", stats.Skipped), zap.Int("absent", stats.Absent))
	return stats, nil
}

func (u *PoultryIngest) writeOne(
	ctx context.Context, q *gen.Queries, commodityID int64, spec moa.PoultrySeriesSpec,
	day time.Time, dayParam pgtype.Date, price float64,
	availableAt pgtype.Timestamptz, stats *PoultryIngestStats,
) error {
	digest := poultryDigest(spec.Field, day, price)
	revision := int32(1)
	latest, lerr := q.GetLatestCommodityObservation(ctx, gen.GetLatestCommodityObservationParams{
		CommodityID: commodityID, MarketCode: nil, TradeDate: dayParam,
	})
	switch {
	case lerr == nil:
		if latest.SourceDigest == digest {
			stats.Skipped++
			return nil
		}
		revision = latest.RevisionNo + 1
	case errors.Is(lerr, pgx.ErrNoRows):
	default:
		return fmt.Errorf("查詢家禽既有 revision 失敗: %w", lerr)
	}

	avg, aerr := postgres.NumericFromString(fmt.Sprintf("%.4f", price))
	if aerr != nil {
		return fmt.Errorf("avg_price 轉換失敗（%s）: %w", spec.Field, aerr)
	}

	var canonical pgtype.Numeric
	if v, ok := domain.CanonicalPricePerKg(price, domain.UnitCatty); ok {
		var cerr error
		canonical, cerr = postgres.NumericFromString(fmt.Sprintf("%.4f", v))
		if cerr != nil {
			return fmt.Errorf("canonical_price 轉換失敗（%s）: %w", spec.Field, cerr)
		}
	}

	if err := q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
		ID: u.tsid(), CommodityID: commodityID, MarketCode: nil,
		TradeDate: dayParam, RevisionNo: revision, AvailableAt: availableAt,
		AvgPrice: avg, CanonicalPrice: canonical,

		Source: u.source, SourceDigest: digest,
	}); err != nil {
		return fmt.Errorf("寫入家禽行情失敗（%s）: %w", spec.Field, err)
	}
	if revision == 1 {
		stats.Inserted++
	} else {
		stats.Revised++
	}
	return nil
}
