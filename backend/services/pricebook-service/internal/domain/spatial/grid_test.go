package spatial_test

import (
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/stretchr/testify/require"
)

// A simple 10km x 10km square in TM2 meters, single outer ring, no holes.
func squarePolygon() spatial.Polygon {
	return spatial.Polygon{
		spatial.Ring{
			{X: 0, Y: 0}, {X: 10_000, Y: 0}, {X: 10_000, Y: 10_000}, {X: 0, Y: 10_000}, {X: 0, Y: 0},
		},
	}
}

func TestBuildRegionGrid_SquareRegion_LandMaskCoversInterior(t *testing.T) {
	grid, err := spatial.BuildRegionGrid([]spatial.Polygon{squarePolygon()}, 2_500, 1_000)
	require.NoError(t, err)
	// Axis must span [0, 10000] at 2500m spacing: 0, 2500, 5000, 7500, 10000.
	require.Equal(t, []float64{0, 2_500, 5_000, 7_500, 10_000}, grid.XAxis)
	require.Equal(t, []float64{0, 2_500, 5_000, 7_500, 10_000}, grid.YAxis)
	require.Len(t, grid.LandMask, 5)
	require.Len(t, grid.LandMask[0], 5)
	// A node strictly inside the square (index 2,2 = x=5000,y=5000) must be land.
	require.True(t, grid.LandMask[2][2])
	// Every node on the square's own boundary is land too (point-on-segment counts as inside).
	require.True(t, grid.LandMask[0][0])
	require.True(t, grid.LandMask[4][4])
}

func TestBuildRegionGrid_FullCellMask_OnlyCellsEntirelyInsideRegion(t *testing.T) {
	grid, err := spatial.BuildRegionGrid([]spatial.Polygon{squarePolygon()}, 2_500, 1_000)
	require.NoError(t, err)
	// FullCellMask has one fewer row/col than the node grid (cells between nodes).
	require.Len(t, grid.FullCellMask, 4)
	require.Len(t, grid.FullCellMask[0], 4)
	// Verified against the real Python _rectangle_fully_inside_region (interpolation.py:610-646):
	// a cell whose edge is collinear with the region's own boundary edge counts as
	// "touching," not "fully inside" — only the 4 strictly-interior cells (not touching
	// the square's outer boundary at all) are True; the 12 perimeter cells are False.
	want := [][]bool{
		{false, false, false, false},
		{false, true, true, false},
		{false, true, true, false},
		{false, false, false, false},
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			require.Equal(t, want[y][x], grid.FullCellMask[y][x], "cell (%d,%d)", x, y)
		}
	}
}

func TestBuildRegionGrid_ExceedsMaxGridCells_ReturnsError(t *testing.T) {
	_, err := spatial.BuildRegionGrid([]spatial.Polygon{squarePolygon()}, 100, 10)
	require.Error(t, err)
}

func TestBuildRegionGrid_TriangleRegion_PartialCellsExcludedFromFullCellMask(t *testing.T) {
	triangle := spatial.Polygon{
		spatial.Ring{{X: 0, Y: 0}, {X: 12_000, Y: 0}, {X: 0, Y: 13_000}, {X: 0, Y: 0}},
	}
	grid, err := spatial.BuildRegionGrid([]spatial.Polygon{triangle}, 3_000, 100)
	require.NoError(t, err)
	// Verified against the real Python _rectangle_fully_inside_region: cell (x-index 1,
	// y-index 1), spanning x:[3000,6000] y:[3000,6000], is comfortably inside the
	// triangle's hypotenuse (13000*x + 12000*y <= 156_000_000 at all 4 corners) — full.
	require.True(t, grid.FullCellMask[1][1])
	// Cell (x-index 2, y-index 1), spanning x:[6000,9000] y:[3000,6000], straddles the
	// hypotenuse: 3 corners inside, corner (9000,6000) outside (13000*9000+12000*6000 =
	// 189,000,000 > 156,000,000) — partial, must be excluded.
	require.False(t, grid.FullCellMask[1][2])
}
