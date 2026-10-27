package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

func TestSearchPriceableEntities_DedupesAcrossVariantsKeepingBestTier(t *testing.T) {
	calls := 0
	surface := &fakeSurfaceFormRepo{
		searchFn: func(_ context.Context, q string, _ int32) ([]domain.SurfaceFormHit, error) {
			calls++
			if q == "白肉雞" {
				return []domain.SurfaceFormHit{{EntityKind: domain.EntityKindCommodity, EntityCode: "7", FormRaw: "白肉雞", FormKind: domain.FormKindSourceTerm, Tier: 0, CommodityName: "白肉雞（2.0 公斤以上）產地價"}}, nil
			}
			return []domain.SurfaceFormHit{{EntityKind: domain.EntityKindCommodity, EntityCode: "7", FormRaw: "白肉雞（2.0 公斤以上）產地價", FormKind: domain.FormKindCanonical, Tier: 2, CommodityName: "白肉雞（2.0 公斤以上）產地價"}}, nil
		},
		categoryFn: func(context.Context, []domain.SurfaceFormHit) (map[string][]string, error) {
			return map[string][]string{"COMMODITY|7": {"2011", "2258"}}, nil
		},
	}
	u := NewSearchPriceableEntities(surface, &fakeForecastRepo{}, forecast.ModelArtifact{})

	got, err := u.Get(context.Background(), []string{"白肉雞", "雞"}, 10)

	require.NoError(t, err)
	require.Equal(t, 2, calls, "每個變體各查一次")
	require.Len(t, got, 1)
	require.Equal(t, domain.EntityKindCommodity, got[0].Kind)
	require.Equal(t, "7", got[0].Code)
	require.Equal(t, "白肉雞（2.0 公斤以上）產地價", got[0].Name)
	require.Equal(t, int32(0), got[0].Tier, "兩個變體命中同一實體，保留最好的 tier")
	require.Equal(t, "白肉雞", got[0].MatchedForm)
	require.Equal(t, domain.FormKindSourceTerm, got[0].FormKind)
	require.Equal(t, []string{"2011", "2258"}, got[0].CategoryIDs)
	require.Equal(t, int32(1), got[0].Rank)
}

func TestSearchPriceableEntities_OrdersByTierThenFormKindThenFirstSeen(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{
				{EntityKind: domain.EntityKindCrop, EntityCode: "C-sub", FormRaw: "糯米飯", FormKind: domain.FormKindCanonical, Tier: 2, CropName: "糯米飯"},
				{EntityKind: domain.EntityKindCropGroup, EntityCode: "G-alias", FormRaw: "糯米", FormKind: domain.FormKindAlias, Tier: 0, GroupName: "米"},
				{EntityKind: domain.EntityKindCommodity, EntityCode: "9", FormRaw: "糯米", FormKind: domain.FormKindSourceTerm, Tier: 0, CommodityName: "白米零售價（圓糯）"},
			}, nil
		},
	}
	u := NewSearchPriceableEntities(surface, &fakeForecastRepo{}, forecast.ModelArtifact{})

	got, err := u.Get(context.Background(), []string{"糯米"}, 10)

	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "9", got[0].Code, "同 tier 0 內 SOURCE_TERM 先於 ALIAS")
	require.Equal(t, "G-alias", got[1].Code)
	require.Equal(t, "C-sub", got[2].Code, "tier 2 墊底")
	require.Equal(t, []int32{1, 2, 3}, []int32{got[0].Rank, got[1].Rank, got[2].Rank})
}

func TestSearchPriceableEntities_LimitCountsDistinctEntities(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return []domain.SurfaceFormHit{
				{EntityKind: domain.EntityKindCropGroup, EntityCode: "A", FormRaw: "A", FormKind: domain.FormKindCanonical, Tier: 0, GroupName: "A"},
				{EntityKind: domain.EntityKindCropGroup, EntityCode: "A", FormRaw: "A別名", FormKind: domain.FormKindAlias, Tier: 0, GroupName: "A"},
				{EntityKind: domain.EntityKindCropGroup, EntityCode: "B", FormRaw: "B", FormKind: domain.FormKindCanonical, Tier: 1, GroupName: "B"},
				{EntityKind: domain.EntityKindCropGroup, EntityCode: "C", FormRaw: "C", FormKind: domain.FormKindCanonical, Tier: 2, GroupName: "C"},
			}, nil
		},
	}
	u := NewSearchPriceableEntities(surface, &fakeForecastRepo{}, forecast.ModelArtifact{})

	got, err := u.Get(context.Background(), []string{"x"}, 2)

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, []string{"A", "B"}, []string{got[0].Code, got[1].Code})
}

func TestSearchPriceableEntities_EmptyVariantsIsInvalid(t *testing.T) {
	u := NewSearchPriceableEntities(&fakeSurfaceFormRepo{}, &fakeForecastRepo{}, forecast.ModelArtifact{})

	_, err := u.Get(context.Background(), []string{"", "  "}, 10)

	require.ErrorIs(t, err, ErrNoQueryVariants)
}

func TestSearchPriceableEntities_NoHitsIsEmptyNotError(t *testing.T) {
	u := NewSearchPriceableEntities(&fakeSurfaceFormRepo{}, &fakeForecastRepo{}, forecast.ModelArtifact{})

	got, err := u.Get(context.Background(), []string{"水泥"}, 10)

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestSearchPriceableEntities_IndexErrorIsReturned(t *testing.T) {
	surface := &fakeSurfaceFormRepo{
		searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
			return nil, context.DeadlineExceeded
		},
	}
	u := NewSearchPriceableEntities(surface, &fakeForecastRepo{}, forecast.ModelArtifact{})

	_, err := u.Get(context.Background(), []string{"x"}, 10)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSearchPriceableEntities_FillsHasForecastAndGroupCode(t *testing.T) {
	surface := &fakeSurfaceFormRepo{searchFn: func(context.Context, string, int32) ([]domain.SurfaceFormHit, error) {
		return []domain.SurfaceFormHit{
			{EntityKind: domain.EntityKindCrop, EntityCode: "LA1", FormRaw: "甘藍-初秋", FormKind: domain.FormKindCanonical, Tier: 0, GroupCode: "G-CAB", GroupName: "甘藍", CropCode: "LA1", CropName: "甘藍-初秋"},
			{EntityKind: domain.EntityKindCrop, EntityCode: "LA9", FormRaw: "甘藍-無模型", FormKind: domain.FormKindCanonical, Tier: 1, GroupCode: "G-CAB", GroupName: "甘藍", CropCode: "LA9", CropName: "甘藍-無模型"},
			{EntityKind: domain.EntityKindCropGroup, EntityCode: "G-CAB", FormRaw: "甘藍", FormKind: domain.FormKindCanonical, Tier: 0, GroupCode: "G-CAB", GroupName: "甘藍"},
			{EntityKind: domain.EntityKindCommodity, EntityCode: "7", FormRaw: "毛豬", FormKind: domain.FormKindSourceTerm, Tier: 0, CommodityName: "毛豬（規格豬）"},
		}, nil
	}}
	repo := &fakeForecastRepo{cropsInGroupFn: func(_ context.Context, groupCode string) ([]domain.CropRef, error) {
		return []domain.CropRef{{CropCode: "LA1", Active: true}, {CropCode: "LA9", Active: true}}, nil
	}}
	artifact := testArtifact(testSeriesModel("moa:109:LA1", "moa:109", "moa:LA1", "", 50, 10))
	u := NewSearchPriceableEntities(surface, repo, artifact)

	got, err := u.Get(context.Background(), []string{"甘藍"}, 10)

	require.NoError(t, err)
	byCode := map[string]PriceableEntityMatch{}
	for _, m := range got {
		byCode[m.Code] = m
	}
	require.True(t, byCode["LA1"].HasForecast)
	require.Equal(t, "G-CAB", byCode["LA1"].GroupCode)
	require.False(t, byCode["LA9"].HasForecast, "artifact 沒有 LA9 的序列")
	require.True(t, byCode["G-CAB"].HasForecast, "任一子品項有序列 ⇒ 大類有")
	require.Equal(t, "G-CAB", byCode["G-CAB"].GroupCode)
	require.True(t, byCode["7"].HasForecast, "commodity 恆為可取價")
	require.Equal(t, "", byCode["7"].GroupCode)
}
