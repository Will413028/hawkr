package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/ricecity"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const (
	SourceRice = "MOA_RICE"

	riceMappingVersion = "pricebook-commodity-v1"

	RiceRowsPerDay = 20
)

type RiceIngestStats struct {
	Fetched     int
	Days        int
	Inserted    int
	Revised     int
	Skipped     int
	Absent      int
	Unpublished int
	Incomplete  int
}

type riceClient interface {
	FetchRice(ctx context.Context, top, skip int) ([]moa.RiceRow, error)
}

type RiceIngest struct {
	client   riceClient
	pool     *pgxpool.Pool
	registry ricecity.Registry
	tsid     func() int64
}

func NewRiceIngest(c *moa.Client, pool *pgxpool.Pool, registry ricecity.Registry, tsid func() int64) *RiceIngest {
	return &RiceIngest{client: c, pool: pool, registry: registry, tsid: tsid}
}

func NewRiceIngestWithClient(c riceClient, pool *pgxpool.Pool, registry ricecity.Registry, tsid func() int64) *RiceIngest {
	return &RiceIngest{client: c, pool: pool, registry: registry, tsid: tsid}
}

func riceDigest(field, marketCode string, day time.Time, price float64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%.4f", field, marketCode, day.Format("2006-01-02"), price)))
	return hex.EncodeToString(sum[:])
}

func (u *RiceIngest) Handle(ctx context.Context, days, skip int) (RiceIngestStats, error) {
	stats := RiceIngestStats{}
	if days <= 0 {
		return stats, fmt.Errorf("糧價 ingest 的天數需為正整數，得到 %d", days)
	}

	rows, err := u.client.FetchRice(ctx, days*RiceRowsPerDay, skip)
	if err != nil {
		return stats, err
	}
	stats.Fetched = len(rows)

	byDate := map[string][]moa.RiceRow{}
	for _, r := range rows {
		key := r.TradeDate.Format("2006-01-02")
		byDate[key] = append(byDate[key], r)
	}
	stats.Days = len(byDate)
	dates := make([]string, 0, len(byDate))
	for k := range byDate {
		dates = append(dates, k)
	}

	sort.Strings(dates)

	q := gen.New(database.GetDBTX(ctx, u.pool))

	commodityIDs := make(map[string]int64, len(moa.RiceSpecs))
	for _, spec := range moa.RiceSpecs {
		canonicalUnit := domain.CanonicalUnitFor(spec.Unit)
		var canonicalPtr *string
		if canonicalUnit != "" {
			cu := canonicalUnit
			canonicalPtr = &cu
		}
		id, cerr := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
			ID: u.tsid(), Source: SourceRice, CommodityCode: spec.Field, Name: spec.Name,
			PriceLayer: spec.PriceLayer, Unit: spec.Unit, CanonicalUnit: canonicalPtr,
		})
		if cerr != nil {
			return stats, fmt.Errorf("寫入糧價品項失敗（%s）: %w", spec.Field, cerr)
		}
		if err := registerCommodityForms(ctx, q, id, spec.Name, spec.SourceTerms, spec.BuyerAliases); err != nil {
			return stats, err
		}
		if err := declareCommodityCategories(ctx, q, id, spec.CategoryIDs, riceMappingVersion, spec.Name); err != nil {
			return stats, err
		}
		commodityIDs[spec.Field] = id
	}

	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())
	for _, key := range dates {
		dayRows := byDate[key]
		day := dayRows[0].TradeDate
		dayParam := pgxutil.DateParam(&day)

		if err := q.DeclareCommodityIngestPartition(ctx, gen.DeclareCommodityIngestPartitionParams{
			Source: SourceRice, TradeDate: dayParam, Status: domain.PartitionStatusPending,
		}); err != nil {
			return stats, fmt.Errorf("宣告糧價攝取分區失敗（%s）: %w", key, err)
		}

		if riceDayUnpublished(dayRows) {
			stats.Unpublished++
			continue
		}

		for _, row := range dayRows {
			marketCode, rerr := u.registry.Resolve(row.CityName)
			if rerr != nil {
				return stats, fmt.Errorf("糧價縣市映射失敗（%s）: %w", key, rerr)
			}
			for _, spec := range moa.RiceSpecs {
				price, ok := row.Prices[spec.Field]
				if !ok {
					stats.Absent++
					continue
				}
				if err := u.writeOne(ctx, q, commodityIDs[spec.Field], spec, marketCode, day, dayParam, price, availableAt, &stats); err != nil {
					return stats, err
				}
			}
		}

		if len(dayRows) < RiceRowsPerDay {
			stats.Incomplete++
			continue
		}
		now := time.Now().UTC()
		if err := q.MarkCommodityIngestPartitionAttempt(ctx, gen.MarkCommodityIngestPartitionAttemptParams{
			Source: SourceRice, TradeDate: dayParam, Status: domain.PartitionStatusComplete,
			LastAttemptAt: &now, CompletedAt: &now,
		}); err != nil {
			zap.L().Warn("rice_ingest: 更新攝取分區狀態失敗", zap.String("trade_date", key), zap.Error(err))
		}
	}

	zap.L().Info("rice_ingest 完成",
		zap.Int("fetched", stats.Fetched), zap.Int("days", stats.Days),
		zap.Int("inserted", stats.Inserted), zap.Int("revised", stats.Revised),
		zap.Int("skipped", stats.Skipped), zap.Int("absent", stats.Absent),
		zap.Int("unpublished", stats.Unpublished), zap.Int("incomplete", stats.Incomplete))
	return stats, nil
}

func riceDayUnpublished(rows []moa.RiceRow) bool {
	for _, r := range rows {
		if len(r.Prices) > 0 {
			return false
		}
	}
	return true
}

func (u *RiceIngest) writeOne(
	ctx context.Context, q *gen.Queries, commodityID int64, spec moa.RiceSeriesSpec,
	marketCode string, day time.Time, dayParam pgtype.Date, price float64,
	availableAt pgtype.Timestamptz, stats *RiceIngestStats,
) error {
	digest := riceDigest(spec.Field, marketCode, day, price)
	revision := int32(1)
	latest, lerr := q.GetLatestCommodityObservation(ctx, gen.GetLatestCommodityObservationParams{
		CommodityID: commodityID, MarketCode: &marketCode, TradeDate: dayParam,
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
		return fmt.Errorf("查詢糧價既有 revision 失敗: %w", lerr)
	}

	avg, aerr := postgres.NumericFromString(fmt.Sprintf("%.4f", price))
	if aerr != nil {
		return fmt.Errorf("avg_price 轉換失敗（%s/%s）: %w", spec.Field, marketCode, aerr)
	}

	var canonical pgtype.Numeric
	if v, ok := domain.CanonicalPricePerKg(price, spec.Unit); ok {
		var cerr error
		canonical, cerr = postgres.NumericFromString(fmt.Sprintf("%.4f", v))
		if cerr != nil {
			return fmt.Errorf("canonical_price 轉換失敗（%s/%s）: %w", spec.Field, marketCode, cerr)
		}
	}

	if err := q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
		ID: u.tsid(), CommodityID: commodityID, MarketCode: &marketCode,
		TradeDate: dayParam, RevisionNo: revision, AvailableAt: availableAt,
		AvgPrice: avg, CanonicalPrice: canonical,

		Source: SourceRice, SourceDigest: digest,
	}); err != nil {
		return fmt.Errorf("寫入糧價行情失敗（%s/%s）: %w", spec.Field, marketCode, err)
	}
	if revision == 1 {
		stats.Inserted++
	} else {
		stats.Revised++
	}
	return nil
}
