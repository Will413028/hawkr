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
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/porkmarket"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const (
	SourcePork = "MOA_PORK"

	porkCommodityCode = "SpecPig_AvgPrice"
	porkCommodityName = "毛豬（規格豬）"

	porkQuantityUnit = "head"

	porkMappingVersion = "pricebook-commodity-v1"
)

var porkCategoryIDs = []string{"2008", "2257"}

var porkSourceTerms = []string{"毛豬"}

var (
	porkQuantityUnitValue = porkQuantityUnit

	porkCanonicalUnitValue = domain.CanonicalUnitFor(domain.UnitKilogram)
)

type PorkIngestStats struct {
	TradeDate time.Time
	Fetched   int
	Inserted  int
	Revised   int
	Skipped   int
	NoTrade   int
}

type porkClient interface {
	FetchPork(ctx context.Context, date time.Time) ([]moa.PorkRecord, error)
}

type PorkIngest struct {
	client   porkClient
	pool     *pgxpool.Pool
	registry porkmarket.Registry
	tsid     func() int64
}

func NewPorkIngest(c *moa.Client, pool *pgxpool.Pool, registry porkmarket.Registry, tsid func() int64) *PorkIngest {
	return &PorkIngest{client: c, pool: pool, registry: registry, tsid: tsid}
}

func NewPorkIngestWithClient(c porkClient, pool *pgxpool.Pool, registry porkmarket.Registry, tsid func() int64) *PorkIngest {
	return &PorkIngest{client: c, pool: pool, registry: registry, tsid: tsid}
}

func PorkRecordDigest(r moa.PorkRecord) string {
	payload := fmt.Sprintf("%s|%.4f|%.3f|%.3f", r.TransDate, r.SpecPigAvg, r.SpecPigNum, r.SpecPigWgt)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (u *PorkIngest) Handle(ctx context.Context, date time.Time) (PorkIngestStats, error) {
	stats := PorkIngestStats{TradeDate: date}
	day := pgxutil.DateParam(&date)

	q := gen.New(database.GetDBTX(ctx, u.pool))
	if err := q.DeclareCommodityIngestPartition(ctx, gen.DeclareCommodityIngestPartitionParams{
		Source: SourcePork, TradeDate: day, Status: domain.PartitionStatusPending,
	}); err != nil {
		return stats, fmt.Errorf("宣告毛豬攝取分區失敗（%s）: %w", date.Format("2006-01-02"), err)
	}

	records, err := u.client.FetchPork(ctx, date)
	if err != nil {
		u.markAttempt(ctx, q, day, domain.PartitionStatusPending, nil, err)
		return stats, err
	}
	stats.Fetched = len(records)

	commodityID, err := q.UpsertCommodity(ctx, gen.UpsertCommodityParams{
		ID: u.tsid(), Source: SourcePork, CommodityCode: porkCommodityCode,
		Name: porkCommodityName, PriceLayer: string(domain.PriceLayerLiveAnimal),
		Unit: domain.UnitKilogram, CanonicalUnit: &porkCanonicalUnitValue,
	})
	if err != nil {
		return stats, fmt.Errorf("寫入毛豬品項失敗: %w", err)
	}
	if err := registerCommodityForms(ctx, q, commodityID, porkCommodityName, porkSourceTerms, nil); err != nil {
		return stats, err
	}
	if err := declareCommodityCategories(ctx, q, commodityID, porkCategoryIDs, porkMappingVersion, "毛豬活體拍賣均價"); err != nil {
		return stats, err
	}

	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())
	for _, r := range records {
		if !r.HasTrade() {
			stats.NoTrade++
			continue
		}
		marketCode, rerr := u.registry.Resolve(r.MarketName)
		if rerr != nil {
			return stats, fmt.Errorf("毛豬市場映射失敗（%s）: %w", date.Format("2006-01-02"), rerr)
		}

		digest := PorkRecordDigest(r)
		revision := int32(1)
		latest, lerr := q.GetLatestCommodityObservation(ctx, gen.GetLatestCommodityObservationParams{
			CommodityID: commodityID, MarketCode: &marketCode, TradeDate: day,
		})
		switch {
		case lerr == nil:
			if latest.SourceDigest == digest {
				stats.Skipped++
				continue
			}
			revision = latest.RevisionNo + 1
		case errors.Is(lerr, pgx.ErrNoRows):

		default:
			return stats, fmt.Errorf("查詢毛豬既有 revision 失敗: %w", lerr)
		}

		var cerr error
		avg, aerr := postgres.NumericFromString(fmt.Sprintf("%.4f", r.SpecPigAvg))
		if aerr != nil {
			return stats, fmt.Errorf("avg_price 轉換失敗（market=%s）: %w", marketCode, aerr)
		}
		qty, qerr := postgres.NumericFromString(fmt.Sprintf("%.3f", r.SpecPigNum))
		if qerr != nil {
			return stats, fmt.Errorf("quantity 轉換失敗（market=%s）: %w", marketCode, qerr)
		}

		var canonical pgtype.Numeric
		if v, ok := domain.CanonicalPricePerKg(r.SpecPigAvg, domain.UnitKilogram); ok {
			canonical, cerr = postgres.NumericFromString(fmt.Sprintf("%.4f", v))
			if cerr != nil {
				return stats, fmt.Errorf("canonical_price 轉換失敗（market=%s）: %w", marketCode, cerr)
			}
		}
		if err := q.InsertCommodityObservation(ctx, gen.InsertCommodityObservationParams{
			ID: u.tsid(), CommodityID: commodityID, MarketCode: &marketCode,
			TradeDate: day, RevisionNo: revision, AvailableAt: availableAt,
			AvgPrice: avg, CanonicalPrice: canonical, Quantity: qty,
			QuantityUnit: &porkQuantityUnitValue,
			Source:       SourcePork, SourceDigest: digest,
		}); err != nil {
			return stats, fmt.Errorf("寫入毛豬行情失敗（market=%s）: %w", marketCode, err)
		}
		if revision == 1 {
			stats.Inserted++
		} else {
			stats.Revised++
		}
	}

	now := time.Now().UTC()
	u.markAttempt(ctx, q, day, domain.PartitionStatusComplete, &now, nil)
	zap.L().Info("pork_ingest 完成",
		zap.String("trade_date", date.Format("2006-01-02")),
		zap.Int("fetched", stats.Fetched), zap.Int("inserted", stats.Inserted),
		zap.Int("revised", stats.Revised), zap.Int("skipped", stats.Skipped),
		zap.Int("no_trade", stats.NoTrade))
	return stats, nil
}

func (u *PorkIngest) markAttempt(ctx context.Context, q *gen.Queries, day pgtype.Date, status int16, completedAt *time.Time, cause error) {
	var lastErr *string
	if cause != nil {
		s := cause.Error()
		lastErr = &s
	}
	now := time.Now().UTC()
	if err := q.MarkCommodityIngestPartitionAttempt(ctx, gen.MarkCommodityIngestPartitionAttemptParams{
		Source: SourcePork, TradeDate: day, Status: status,
		LastAttemptAt: &now,
		CompletedAt:   completedAt, LastError: lastErr,
	}); err != nil {
		zap.L().Warn("pork_ingest: 更新攝取分區狀態失敗", zap.Error(err))
	}
}
