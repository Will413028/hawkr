//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type stubCategoryTree struct {
	tree *categorypb.GetCategoryTreeResponse
}

func (s *stubCategoryTree) GetCategoryTree(context.Context, string, bool) (*categorypb.GetCategoryTreeResponse, error) {
	return s.tree, nil
}

func treeNode(id, parentID string, children ...*categorypb.CategoryNode) *categorypb.CategoryNode {
	return &categorypb.CategoryNode{Id: id, ParentId: parentID, Status: "ACTIVE", Children: children}
}

func rollupOrigin() time.Time { return time.Now().UTC().Add(time.Minute) }

func rollupTestConfig() *config.Config {
	return &config.Config{Inference: config.InferenceConfig{
		MinRidgeSampleCount: 100, FreshWithinHours: 36, StaleWithinHours: 72,
		GroupVolumeWindowDays: 28, ObservedWindowDays: 30,
		GroupMinMarketSupportCount: forecast.DefaultGroupMinMarketSupportCount,
	}}
}

func TestRollupBatch_CommodityMappedL3GetsPriceWithItsOwnLayerAndUnit(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	day1 := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	ingest := usecase.NewPoultryIngestWithClient(&stubPoultryClient{rows: []moa.PoultryRow{
		{TradeDate: day1, Prices: map[string]float64{"egg_Producer_Price": 38.0}},
		{TradeDate: day2, Prices: map[string]float64{"egg_Producer_Price": 39.5}},
	}}, pool, newSeqTSID(970000000000000001), moa.PoultryChickenEgg, usecase.SourcePoultryChickenEgg)
	_, err := ingest.Handle(ctx, day1, day2)
	require.NoError(t, err)

	logger := zap.NewNop()
	batch := usecase.NewRollupBatch(
		nil, usecase.NewCommodityForecast(pool), forecast.ModelArtifact{},
		postgres.NewCategoryMappingRepository(pool, logger),
		postgres.NewCategoryPriceRollupRepository(pool),

		&stubCategoryTree{tree: &categorypb.GetCategoryTreeResponse{
			Categories: []*categorypb.CategoryNode{
				treeNode("3", "", treeNode("190", "3", treeNode("1999", "190"))),
			},
		}},
		rollupTestConfig(), logger, func() time.Time { return rollupOrigin() },
	)

	stats, err := batch.Handle(ctx, rollupOrigin())
	require.NoError(t, err)
	require.Equal(t, 3, stats.CategoriesWritten, "L3 + L2 + L1 各一列")

	repo := postgres.NewCategoryPriceRollupRepository(pool)
	leaf, err := repo.Get(ctx, "1999")
	require.NoError(t, err)
	require.NotNil(t, leaf)
	require.Equal(t, domain.RollupStatusReady, leaf.Status)

	require.NotNil(t, leaf.PriceLayer)
	require.Equal(t, domain.PriceLayerFarmgate, *leaf.PriceLayer)
	require.NotNil(t, leaf.Unit)
	require.Equal(t, domain.UnitCatty, *leaf.Unit, "畫面以官方公布的台斤價為主")

	require.NotNil(t, leaf.LatestPrice)
	require.InDelta(t, 39.5, *leaf.LatestPrice, 0.001)
	require.NotNil(t, leaf.PrevPrice)
	require.InDelta(t, 38.0, *leaf.PrevPrice, 0.001)
	require.Len(t, leaf.Points, 2)
	require.Equal(t, "2026-09-10", leaf.Points[0].TradeDate, "由舊到新")
	require.Equal(t, "2026-09-11", leaf.Points[1].TradeDate)
	require.EqualValues(t, 1, leaf.SourceGroupCount)

	require.Len(t, leaf.ForecastPoints, 1, "不得把同一個數字複製成七日平線")
	require.Equal(t, 1, leaf.ForecastPoints[0].HorizonDays)
	require.Equal(t, "2026-09-12", leaf.ForecastPoints[0].TargetDate)
	require.NotNil(t, leaf.ForecastPoints[0].P50)
	require.InDelta(t, 39.5, *leaf.ForecastPoints[0].P50, 0.001)
	require.Nil(t, leaf.ForecastPoints[0].P10, "來源沒有公布不確定性，填了就是替它宣稱")
	require.Nil(t, leaf.ForecastPoints[0].P90)

	for _, id := range []string{"190", "3"} {
		row, gerr := repo.Get(ctx, id)
		require.NoError(t, gerr)
		require.NotNil(t, row, "categoryId=%s", id)
		require.Equal(t, domain.RollupStatusReady, row.Status, "categoryId=%s", id)
		require.NotNil(t, row.LatestPrice)
		require.InDelta(t, 39.5, *row.LatestPrice, 0.001, "categoryId=%s", id)
		require.NotNil(t, row.PriceLayer)
		require.Equal(t, domain.PriceLayerFarmgate, *row.PriceLayer, "categoryId=%s", id)
		require.NotNil(t, row.Unit)
		require.Equal(t, domain.UnitCatty, *row.Unit, "categoryId=%s", id)
	}
}

func TestRollupBatch_OneCommodityFansOutToMultipleL3(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	ingest := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("雲林縣", 333, 106.03),
	}}, pool, porkRegistry(t), newSeqTSID(970000000000001001))
	_, err := ingest.Handle(ctx, porkDay)
	require.NoError(t, err)

	logger := zap.NewNop()
	batch := usecase.NewRollupBatch(
		nil, usecase.NewCommodityForecast(pool), forecast.ModelArtifact{},
		postgres.NewCategoryMappingRepository(pool, logger),
		postgres.NewCategoryPriceRollupRepository(pool),
		&stubCategoryTree{tree: &categorypb.GetCategoryTreeResponse{
			Categories: []*categorypb.CategoryNode{
				treeNode("4", "", treeNode("198", "4",
					treeNode("2008", "198"), treeNode("2257", "198"))),
			},
		}},
		rollupTestConfig(), logger, func() time.Time { return rollupOrigin() },
	)

	_, err = batch.Handle(ctx, rollupOrigin())
	require.NoError(t, err)

	repo := postgres.NewCategoryPriceRollupRepository(pool)
	for _, id := range []string{"2008", "2257"} {
		row, gerr := repo.Get(ctx, id)
		require.NoError(t, gerr)
		require.NotNil(t, row, "categoryId=%s 沒有 rollup 列", id)
		require.Equal(t, domain.RollupStatusReady, row.Status, "categoryId=%s", id)
		require.NotNil(t, row.LatestPrice)
		require.InDelta(t, 106.03, *row.LatestPrice, 0.001, "categoryId=%s", id)
		require.NotNil(t, row.PriceLayer)

		require.Equal(t, domain.PriceLayerLiveAnimal, *row.PriceLayer, "categoryId=%s", id)
	}

	l2, err := repo.Get(ctx, "198")
	require.NoError(t, err)
	require.NotNil(t, l2)
	require.Equal(t, domain.RollupStatusReady, l2.Status)
	require.InDelta(t, 106.03, *l2.LatestPrice, 0.001)
	require.EqualValues(t, 2, l2.SourceGroupCount, "兩個子節點都有來源")
}

func TestRollupBatch_InactiveCommodityFallsToUnavailable(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	ingest := usecase.NewPorkIngestWithClient(&stubPorkClient{records: []moa.PorkRecord{
		porkRow("雲林縣", 333, 106.03),
	}}, pool, porkRegistry(t), newSeqTSID(970000000000002001))
	_, err := ingest.Handle(ctx, porkDay)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "UPDATE commodity SET active = FALSE")
	require.NoError(t, err)

	logger := zap.NewNop()
	batch := usecase.NewRollupBatch(
		nil, usecase.NewCommodityForecast(pool), forecast.ModelArtifact{},
		postgres.NewCategoryMappingRepository(pool, logger),
		postgres.NewCategoryPriceRollupRepository(pool),
		&stubCategoryTree{tree: &categorypb.GetCategoryTreeResponse{
			Categories: []*categorypb.CategoryNode{
				treeNode("4", "", treeNode("198", "4", treeNode("2008", "198"))),
			},
		}},
		rollupTestConfig(), logger, func() time.Time { return rollupOrigin() },
	)

	_, err = batch.Handle(ctx, rollupOrigin())
	require.NoError(t, err)

	repo := postgres.NewCategoryPriceRollupRepository(pool)
	row, err := repo.Get(ctx, "2008")
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, domain.RollupStatusUnavailable, row.Status)
	require.Nil(t, row.LatestPrice, "停用的序列不得沿用舊值")
	require.Nil(t, row.PriceLayer)
}
