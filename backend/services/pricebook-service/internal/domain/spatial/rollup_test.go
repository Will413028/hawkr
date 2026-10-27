// services/pricebook-service/internal/domain/spatial/rollup_test.go
package spatial_test

import (
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/paulmach/orb"
	"github.com/stretchr/testify/require"
)

// A 1-degree-square county roughly covering central Taiwan, for a
// synthetic test independent of the real 22-county boundary data (which
// Task 10's parity fixture exercises against the real reference/ files).
func testCounty() geography.CountyRegion {
	return geography.CountyRegion{
		CountyCode:        "99999",
		CountyName:        "測試縣",
		CountyNameEN:      "Test County",
		InterpolationEPSG: 3826,
		Geometry: orb.Polygon{
			orb.Ring{{120.5, 23.5}, {121.5, 23.5}, {121.5, 24.5}, {120.5, 24.5}, {120.5, 23.5}},
		},
	}
}

// A second 1-degree-square county adjacent to testCounty (non-overlapping).
func testCountyEast() geography.CountyRegion {
	return geography.CountyRegion{
		CountyCode:        "88888",
		CountyName:        "東試縣",
		CountyNameEN:      "East Test County",
		InterpolationEPSG: 3826,
		Geometry: orb.Polygon{
			orb.Ring{{121.5, 23.5}, {122.5, 23.5}, {122.5, 24.5}, {121.5, 24.5}, {121.5, 23.5}},
		},
	}
}

func cellAt(lon, lat, price float64, bandIndex int) spatial.SurfaceCell {
	// A tiny square polygon centered at (lon,lat) — centroid math only
	// needs the ring's average, so an exact square keeps this simple.
	d := 0.001
	return spatial.SurfaceCell{
		PolygonWGS84: [][2]float64{
			{lon - d, lat - d}, {lon + d, lat - d}, {lon + d, lat + d}, {lon - d, lat + d}, {lon - d, lat - d},
		},
		PriceTwdPerKg: price,
		BandIndex:     bandIndex,
	}
}

func TestRollupByCounty_AveragesCellsInsideCounty(t *testing.T) {
	cells := []spatial.SurfaceCell{
		cellAt(120.9, 24.0, 20.0, 1),
		cellAt(121.1, 24.1, 40.0, 2),
		cellAt(125.0, 24.0, 999.0, 3), // outside the test county entirely
	}
	rollups := spatial.RollupByCounty(cells, []geography.CountyRegion{testCounty()}, []float64{15, 25, 35})
	require.Len(t, rollups, 1)
	r := rollups[0]
	require.Equal(t, "99999", r.CountyCode)
	require.InDelta(t, 30.0, r.EstimatedP50TwdPerKg, 1e-9) // mean(20,40)
	require.Equal(t, 2, r.SupportingCellCount)
}

func TestRollupByCounty_NoCellsInside_CountyOmitted(t *testing.T) {
	cells := []spatial.SurfaceCell{cellAt(125.0, 24.0, 999.0, 3)}
	rollups := spatial.RollupByCounty(cells, []geography.CountyRegion{testCounty()}, []float64{15, 25, 35})
	require.Empty(t, rollups)
}

func TestRollupByCounty_BandBoundsMatchRolledUpValue(t *testing.T) {
	cells := []spatial.SurfaceCell{cellAt(120.9, 24.0, 20.0, 1), cellAt(121.0, 24.0, 22.0, 1)}
	rollups := spatial.RollupByCounty(cells, []geography.CountyRegion{testCounty()}, []float64{15, 25, 35})
	require.Len(t, rollups, 1)
	require.InDelta(t, 21.0, rollups[0].EstimatedP50TwdPerKg, 1e-9)
	require.NotNil(t, rollups[0].BandLower)
	require.InDelta(t, 15.0, *rollups[0].BandLower, 1e-9)
	require.NotNil(t, rollups[0].BandUpper)
	require.InDelta(t, 25.0, *rollups[0].BandUpper, 1e-9)
}

func TestRollupByCounty_MultiCounty_AssignsToCorrectCounties(t *testing.T) {
	// Cells in testCounty (120.5-121.5): prices 10, 20, 30
	// Cells in testCountyEast (121.5-122.5): prices 50, 70
	// One cell outside both counties (should be ignored)
	cells := []spatial.SurfaceCell{
		cellAt(120.7, 24.0, 10.0, 1),  // in testCounty
		cellAt(120.9, 24.1, 20.0, 1),  // in testCounty
		cellAt(121.3, 23.9, 30.0, 1),  // in testCounty (near east edge)
		cellAt(121.9, 24.0, 50.0, 1),  // in testCountyEast
		cellAt(122.2, 24.1, 70.0, 1),  // in testCountyEast
		cellAt(125.0, 24.0, 999.0, 1), // outside both
	}
	rollups := spatial.RollupByCounty(cells, []geography.CountyRegion{testCounty(), testCountyEast()}, []float64{15, 25, 35})
	require.Len(t, rollups, 2)

	// Verify results are keyed by county order (testCounty first, then testCountyEast)
	west := rollups[0]
	east := rollups[1]

	require.Equal(t, "99999", west.CountyCode)
	require.Equal(t, 3, west.SupportingCellCount)
	require.InDelta(t, 20.0, west.EstimatedP50TwdPerKg, 1e-9) // mean(10,20,30)

	require.Equal(t, "88888", east.CountyCode)
	require.Equal(t, 2, east.SupportingCellCount)
	require.InDelta(t, 60.0, east.EstimatedP50TwdPerKg, 1e-9) // mean(50,70)
}
