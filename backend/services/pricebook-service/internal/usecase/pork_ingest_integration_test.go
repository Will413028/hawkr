//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/porkmarket"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type stubPorkClient struct{ records []moa.PorkRecord }

func (s *stubPorkClient) FetchPork(context.Context, time.Time) ([]moa.PorkRecord, error) {
	return s.records, nil
}

func porkRegistry(t *testing.T) porkmarket.Registry {
	t.Helper()
	r, err := porkmarket.Load(
		"../../reference/pork_market_registry.v1.csv",
		"../../reference/pork_market_registry.v1.manifest.json",
	)
	require.NoError(t, err)
	return r
}

var porkDay = time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

func porkRow(market string, num, avg float64) moa.PorkRecord {
	return moa.PorkRecord{
		TransDate: moa.ToROCDateCompact(porkDay), MarketName: market,
		SpecPigNum: num, SpecPigWgt: 128.4, SpecPigAvg: avg,
	}
}

func TestPorkIngest_ZeroSentinelDoesNotLand(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("雲林縣", 333, 106.03),
		porkRow("高雄岡山", 0, 0),
	}}, pool, porkRegistry(t), newSeqTSID(900000000000000001))

	stats, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Fetched)
	require.Equal(t, 1, stats.Inserted)
	require.Equal(t, 1, stats.NoTrade)

	q := gen.New(pool)
	rows, err := q.ListCommoditiesBySource(ctx, usecase.SourcePork)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "live_animal", rows[0].PriceLayer, "毛豬必須是活體層級")
	require.Equal(t, "kg", rows[0].Unit)

	for _, l3 := range []string{"2008", "2257"} {
		mapped, merr := q.ListCommoditiesByCategory(ctx, l3)
		require.NoError(t, merr)
		require.Len(t, mapped, 1, "L3 %s 應對到毛豬序列", l3)
		require.Equal(t, rows[0].ID, mapped[0].ID)
	}

	obs, err := q.ListLatestCommodityObservationsAsOf(ctx, gen.ListLatestCommodityObservationsAsOfParams{
		CommodityID: rows[0].ID,
		Cutoff:      pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Hour), Valid: true},
		FromDate:    pgtype.Date{Time: porkDay.AddDate(0, 0, -7), Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, obs, 1, "無成交的市場不該有觀測列")
	require.Equal(t, "head", *obs[0].QuantityUnit, "毛豬的數量單位是頭不是公斤")
}

func TestPorkIngest_SameDigestSkipped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	recs := []moa.PorkRecord{porkRow("雲林縣", 333, 106.03)}
	uc := usecase.NewPorkIngestWithClient(&stubPorkClient{records: recs}, pool, porkRegistry(t), newSeqTSID(900000000000001001))

	first, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)
	require.Equal(t, 1, first.Inserted)

	second, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)
	require.Equal(t, 0, second.Inserted)
	require.Equal(t, 0, second.Revised)
	require.Equal(t, 1, second.Skipped, "digest 相同必須跳過，不可重複寫入")
}

func TestPorkIngest_ChangedDigestAppendsRevision(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	stub := &stubPorkClient{records: []moa.PorkRecord{porkRow("雲林縣", 333, 106.03)}}
	uc := usecase.NewPorkIngestWithClient(stub, pool, porkRegistry(t), newSeqTSID(900000000000002001))

	_, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)

	stub.records = []moa.PorkRecord{porkRow("雲林縣", 340, 106.55)}
	stats, err := uc.Handle(ctx, porkDay)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Revised)

	q := gen.New(pool)
	commodities, err := q.ListCommoditiesBySource(ctx, usecase.SourcePork)
	require.NoError(t, err)
	latest, err := q.GetLatestCommodityObservation(ctx, gen.GetLatestCommodityObservationParams{
		CommodityID: commodities[0].ID, MarketCode: strPtr("PORK-15"),
		TradeDate: pgtype.Date{Time: porkDay, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), latest.RevisionNo)

	// 兩次 ingest 之後 SOURCE_TERM「毛豬」仍恰好一列：登記與 upsert 同交易且可重跑。
	var sourceTerms int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM priceable_surface_form
		WHERE entity_kind = 'COMMODITY' AND form_kind = 'SOURCE_TERM' AND form_raw = '毛豬'`).Scan(&sourceTerms))
	require.Equal(t, 1, sourceTerms)
}

func TestPorkIngest_UnknownMarketFailsClosed(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	uc := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("澎湖縣", 100, 99.0),
	}}, pool, porkRegistry(t), newSeqTSID(900000000000003001))

	_, err := uc.Handle(ctx, porkDay)
	require.ErrorIs(t, err, porkmarket.ErrUnknownMarket)
}

func strPtr(s string) *string { return &s }
