package geography_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
)

func realGeographyPaths(t *testing.T) (registryCSV, registryManifest, boundaryGeoJSON, boundaryManifest string) {
	t.Helper()
	rc, rm := realRegistryPaths(t)
	bg, bm := realBoundaryPaths(t)
	return rc, rm, bg, bm
}

func TestLoad_RealFiles_ResolvesMarket400ByCategoryToDifferentFacilities(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	on := forecast.NewDate(2026, 8, 8)

	produce, ok := registry.ResolveFacility("400", "N04", on)
	require.True(t, ok)
	require.Equal(t, "moa-facility-400-produce", produce.FacilityID)

	flower, ok := registry.ResolveFacility("400", "N06", on)
	require.True(t, ok)
	require.Equal(t, "moa-facility-400-flower", flower.FacilityID)

	require.NotEqual(t, produce.FacilityID, flower.FacilityID, "same market_code, different category, must resolve to different physical facilities")
}

func TestLoad_RealFiles_LegacyMarket100DoesNotResolve(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	on := forecast.NewDate(2026, 8, 8)
	_, ok := registry.ResolveFacility("100", "N05", on)
	require.False(t, ok, "catalog_only_legacy rows must not be resolvable")
}

func TestLoad_RealFiles_UnknownMarketDoesNotResolve(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	on := forecast.NewDate(2026, 8, 8)
	_, ok := registry.ResolveFacility("999999", "N04", on)
	require.False(t, ok)
}

func TestLoad_RealFiles_AllActiveFacilitiesResolveToACounty(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	facilities := registry.ResolvedFacilities()
	require.Len(t, facilities, 24, "market_registry.v1.csv 目前應有 24 個 active 設施；若此斷言失敗，代表資料變了但測試沒有同步更新")

	for _, f := range facilities {
		require.NotEmpty(t, f.CountyCode, "facility_id=%s (market_code=%s) should resolve to a county — every currently-mirrored active facility is on Taiwan's main island or a covered outlying island; if a future re-mirror adds one that doesn't, document the exception here rather than silently skipping it", f.FacilityID, f.MarketCode)
	}
}

func TestLoad_RealFiles_RegionsInScopeReturns22Counties(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	require.Len(t, registry.RegionsInScope(), 22)
}

// spatial-surface 的 lineage 需要「這張圖是哪一版界線畫的」，而界線只有
// Registry 這一份——沒有這兩個 accessor，消費端只能再讀一次檔案自己算。
func TestLoad_RealFiles_ExposesBoundaryVersionAndDigest(t *testing.T) {
	rc, rm, bg, bm := realGeographyPaths(t)
	registry, err := geography.Load(rc, rm, bg, bm)
	require.NoError(t, err)

	boundary, err := geography.LoadBoundary(bg, bm)
	require.NoError(t, err)
	require.Equal(t, boundary.Version, registry.BoundaryVersion())
	require.Equal(t, boundary.Digest, registry.BoundaryDigest())
	require.NotEmpty(t, registry.BoundaryDigest())
}
