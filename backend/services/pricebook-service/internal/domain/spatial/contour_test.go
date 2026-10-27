package spatial_test

import (
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/stretchr/testify/require"
)

// A single 2x2-node (1 cell) grid, values increasing left-to-right, all 4
// corners land and the cell fully inside — the simplest possible non-empty
// contour case. Corner order for `values` is (y0,x0),(y0,x1),(y1,x1),(y1,x0)
// (bottom-left, bottom-right, top-right, top-left), matching
// interpolation.py:912-917's `values` tuple construction.
func oneCellGrid() (spatial.RegionGrid, [][]float64) {
	grid := spatial.RegionGrid{
		XAxis:        []float64{0, 1000},
		YAxis:        []float64{0, 1000},
		LandMask:     [][]bool{{true, true}, {true, true}},
		FullCellMask: [][]bool{{true}},
	}
	values := [][]float64{
		{10, 30}, // y=0 row: (x=0,y=0)=10, (x=1000,y=0)=30
		{10, 30}, // y=1000 row: (x=0,y=1000)=10, (x=1000,y=1000)=30
	}
	return grid, values
}

func TestBuildContours_SingleCell_ProducesOneVerticalSegmentAtLevel20(t *testing.T) {
	grid, values := oneCellGrid()
	segments, err := spatial.BuildContours(grid, values, []float64{20})
	require.NoError(t, err)
	require.Len(t, segments, 1)
	seg := segments[0]
	require.Equal(t, 20.0, seg.LevelValue)
	// Values go 10->30 linearly left to right, so the level=20 crossing is
	// exactly at the cell's horizontal midpoint (x=500) on both the top and
	// bottom edges, giving a vertical segment.
	require.InDelta(t, 500.0, seg.Left.X, 1e-9)
	require.InDelta(t, 500.0, seg.Right.X, 1e-9)
	require.ElementsMatch(t, []float64{0, 1000}, []float64{seg.Left.Y, seg.Right.Y})
}

func TestBuildContours_LevelOutsideRange_ProducesNoSegments(t *testing.T) {
	grid, values := oneCellGrid()
	segments, err := spatial.BuildContours(grid, values, []float64{100})
	require.NoError(t, err)
	require.Empty(t, segments)
}

func TestBuildContours_CellNotFullyInside_Skipped(t *testing.T) {
	grid, values := oneCellGrid()
	grid.FullCellMask = [][]bool{{false}}
	segments, err := spatial.BuildContours(grid, values, []float64{20})
	require.NoError(t, err)
	require.Empty(t, segments)
}

func TestBuildContours_RejectsNonIncreasingBreaks(t *testing.T) {
	grid, values := oneCellGrid()
	_, err := spatial.BuildContours(grid, values, []float64{20, 20})
	require.Error(t, err)
	_, err = spatial.BuildContours(grid, values, []float64{30, 20})
	require.Error(t, err)
}

func TestBuildContours_EmptyBreaks_ReturnsNoSegmentsWithoutError(t *testing.T) {
	grid := spatial.RegionGrid{} // 零值 grid：YAxis/XAxis/FullCellMask 皆為 nil，迴圈自然跑 0 次
	segments, err := spatial.BuildContours(grid, nil, nil)
	if err != nil {
		t.Fatalf("BuildContours returned error for empty breaks: %v", err)
	}
	if len(segments) != 0 {
		t.Fatalf("segments = %v, want empty", segments)
	}
}
