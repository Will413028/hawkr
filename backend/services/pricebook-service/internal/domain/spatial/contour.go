package spatial

import (
	"fmt"
	"math"
)

// ContourSegment is one line segment of an isoline at LevelValue, in the
// same TM2-meter space as the RegionGrid it was built from.
type ContourSegment struct {
	Left, Right Point
	LevelValue  float64
}

// edgeCoordinate mirrors interpolation.py:841-874 (_edge_coordinate) —
// linear interpolation along one of the 4 cell edges to find where `level`
// crosses it. Edge indices: 0=bottom, 1=right, 2=top, 3=left.
func edgeCoordinate(edge int, level float64, values [4]float64, x0, x1, y0, y1 float64) Point {
	points := [4][2]Point{
		{{x0, y0}, {x1, y0}}, // bottom
		{{x1, y0}, {x1, y1}}, // right
		{{x0, y1}, {x1, y1}}, // top
		{{x0, y0}, {x0, y1}}, // left
	}
	valuePairs := [4][2]float64{
		{values[0], values[1]},
		{values[1], values[2]},
		{values[3], values[2]},
		{values[0], values[3]},
	}
	left, right := points[edge][0], points[edge][1]
	leftValue, rightValue := valuePairs[edge][0], valuePairs[edge][1]
	fraction := 0.5
	if rightValue != leftValue {
		fraction = math.Min(1.0, math.Max(0.0, (level-leftValue)/(rightValue-leftValue)))
	}
	return Point{
		X: left.X + fraction*(right.X-left.X),
		Y: left.Y + fraction*(right.Y-left.Y),
	}
}

// caseSegmentTable is the non-saddle half of the marching-squares lookup
// (cases 5 and 10 need a runtime branch, so they live in caseSegments'
// switch instead). Package-level rather than a literal inside caseSegments
// because BuildContours calls that function once per cell per contour
// level — on the real Taiwan grid that is tens of thousands of calls per
// request, each of which would otherwise allocate and populate a fresh
// 14-entry map. Treat as immutable: the returned slices are shared, so no
// caller may append to or write through them.
var caseSegmentTable = map[int][][2]int{
	0: nil, 1: {{3, 0}}, 2: {{0, 1}}, 3: {{3, 1}},
	4: {{1, 2}}, 6: {{0, 2}}, 7: {{3, 2}}, 8: {{2, 3}},
	9: {{0, 2}}, 11: {{1, 2}}, 12: {{1, 3}}, 13: {{0, 1}},
	14: {{3, 0}}, 15: nil,
}

// caseSegments mirrors interpolation.py:877-898 (_case_segments) — the
// standard 16-case marching-squares lookup table, with cases 5 and 10
// (diagonal saddles) disambiguated by the cell's average value vs level.
func caseSegments(caseIndex int, centerIsHigh bool) [][2]int {
	switch caseIndex {
	case 5:
		if centerIsHigh {
			return [][2]int{{0, 1}, {2, 3}}
		}
		return [][2]int{{3, 0}, {1, 2}}
	case 10:
		if centerIsHigh {
			return [][2]int{{0, 3}, {1, 2}}
		}
		return [][2]int{{0, 1}, {2, 3}}
	default:
		return caseSegmentTable[caseIndex]
	}
}

// BuildContours mirrors interpolation.py:901-972 (_contour_features),
// minus the WGS84 projection and GeoJSON wrapping (the caller does that —
// see internal/usecase/spatial_surface.go — keeping this function's output
// in the same TM2-meter space as its RegionGrid input for testability).
// values must have the same [y][x] shape as grid.LandMask.
func BuildContours(grid RegionGrid, values [][]float64, breaks []float64) ([]ContourSegment, error) {
	for i := 1; i < len(breaks); i++ {
		if breaks[i] <= breaks[i-1] {
			return nil, fmt.Errorf("spatial: contour breaks must be strictly increasing, got %v", breaks)
		}
	}
	var segments []ContourSegment
	for _, level := range breaks {
		for yi := 0; yi < len(grid.YAxis)-1; yi++ {
			for xi := 0; xi < len(grid.XAxis)-1; xi++ {
				if yi >= len(grid.FullCellMask) || xi >= len(grid.FullCellMask[yi]) || !grid.FullCellMask[yi][xi] {
					continue
				}
				cellValues := [4]float64{values[yi][xi], values[yi][xi+1], values[yi+1][xi+1], values[yi+1][xi]}
				allFinite := true
				for _, v := range cellValues {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						allFinite = false
						break
					}
				}
				if !allFinite {
					continue
				}
				caseIndex := 0
				for bit, v := range cellValues {
					if v >= level {
						caseIndex |= 1 << bit
					}
				}
				centerIsHigh := (cellValues[0]+cellValues[1]+cellValues[2]+cellValues[3])/4 >= level
				x0, x1 := grid.XAxis[xi], grid.XAxis[xi+1]
				y0, y1 := grid.YAxis[yi], grid.YAxis[yi+1]
				for _, edgePair := range caseSegments(caseIndex, centerIsHigh) {
					left := edgeCoordinate(edgePair[0], level, cellValues, x0, x1, y0, y1)
					right := edgeCoordinate(edgePair[1], level, cellValues, x0, x1, y0, y1)
					if math.Hypot(left.X-right.X, left.Y-right.Y) <= 1e-7 {
						continue
					}
					segments = append(segments, ContourSegment{Left: left, Right: right, LevelValue: level})
				}
			}
		}
	}
	return segments, nil
}
