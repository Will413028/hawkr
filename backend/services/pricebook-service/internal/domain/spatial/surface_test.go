// services/pricebook-service/internal/domain/spatial/surface_test.go
package spatial_test

import (
	"math"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/stretchr/testify/require"
)

func TestBuildSurfaceCells_OneFullCell_AveragesCorners(t *testing.T) {
	grid := spatial.RegionGrid{
		XAxis:        []float64{215_000, 216_000},
		YAxis:        []float64{2_677_000, 2_678_000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{true}},
	}
	values := [][]float64{{10, 30}, {20, 40}}
	cells, err := spatial.BuildSurfaceCells(grid, values, []float64{15, 25, 35}, 3826)
	require.NoError(t, err)
	require.Len(t, cells, 1)
	cell := cells[0]
	require.InDelta(t, 25.0, cell.PriceTwdPerKg, 1e-9) // mean(10,30,40,20) = 25
	require.Equal(t, 2, cell.BandIndex)                // 25 falls in [25,35) -> band index 2 per _band()
	require.NotNil(t, cell.LowerBound)
	require.InDelta(t, 25.0, *cell.LowerBound, 1e-9)
	require.NotNil(t, cell.UpperBound)
	require.InDelta(t, 35.0, *cell.UpperBound, 1e-9)
	require.Len(t, cell.PolygonWGS84, 5) // closed ring: 4 corners + repeat of first
	require.Equal(t, cell.PolygonWGS84[0], cell.PolygonWGS84[4])
}

func TestBuildSurfaceCells_SkipsNonFullCells(t *testing.T) {
	grid := spatial.RegionGrid{
		XAxis:        []float64{215_000, 216_000},
		YAxis:        []float64{2_677_000, 2_678_000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{false}},
	}
	cells, err := spatial.BuildSurfaceCells(grid, [][]float64{{10, 30}, {20, 40}}, []float64{15, 25, 35}, 3826)
	require.NoError(t, err)
	require.Empty(t, cells)
}

func TestBuildSurfaceCells_SkipsNonFiniteCorners(t *testing.T) {
	grid := spatial.RegionGrid{
		XAxis:        []float64{215_000, 216_000},
		YAxis:        []float64{2_677_000, 2_678_000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{true}},
	}
	values := [][]float64{{10, math.NaN()}, {20, 40}}
	cells, err := spatial.BuildSurfaceCells(grid, values, []float64{15, 25, 35}, 3826)
	require.NoError(t, err)
	require.Empty(t, cells)
}

func TestBuildSurfaceCells_BelowLowestBreak_HasNilLowerBound(t *testing.T) {
	grid := spatial.RegionGrid{
		XAxis:        []float64{215_000, 216_000},
		YAxis:        []float64{2_677_000, 2_678_000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{true}},
	}
	values := [][]float64{{1, 2}, {1, 2}} // mean = 1.5, below the lowest break of 15
	cells, err := spatial.BuildSurfaceCells(grid, values, []float64{15, 25, 35}, 3826)
	require.NoError(t, err)
	require.Len(t, cells, 1)
	require.Equal(t, 0, cells[0].BandIndex)
	require.Nil(t, cells[0].LowerBound)
	require.NotNil(t, cells[0].UpperBound)
}

func TestBuildSurfaceCells_RoundTieBoundary_MatchesPython(t *testing.T) {
	// Regression test: verify round-half-to-even at tie boundary (4 decimal places).
	// Corners [0, 0, 0.0007, 0.0007] have mean 0.00035, which is exactly halfway
	// between 0.0003 and 0.0004 when rounded to 4 decimals.
	// - Python's round(0.00035, 4) → 0.0003 (round-half-to-even, ties to even)
	// - Old Go bug (math.Round): would give 0.0004 (round-half-away-from-zero)
	// This test catches regression of the rounding fix (strconv round-trip approach).
	grid := spatial.RegionGrid{
		XAxis:        []float64{215_000, 216_000},
		YAxis:        []float64{2_677_000, 2_678_000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{true}},
	}
	values := [][]float64{{0.0, 0.0}, {0.0007, 0.0007}}
	cells, err := spatial.BuildSurfaceCells(grid, values, []float64{0.0001, 0.0005, 0.001}, 3826)
	require.NoError(t, err)
	require.Len(t, cells, 1)
	require.InDelta(t, 0.0003, cells[0].PriceTwdPerKg, 1e-9)
}
