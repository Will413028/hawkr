package spatial

import (
	"fmt"
	"math"
)

// Point, Ring, Polygon are planar (already-projected, TM2 meters)
// geometry types used only inside this package for grid-domain
// construction — mirrors interpolation.py's tuple-of-(x,y)-tuples
// convention, not GeoJSON (that conversion happens in surface.go/contour.go
// via TWD97TM2ToWGS84).
type Point struct{ X, Y float64 }
type Ring []Point
type Polygon []Ring // Polygon[0] = outer ring, Polygon[1:] = holes

const pointTolerance = 1e-7

// pointOnSegment mirrors interpolation.py:305-317 (_point_on_segment).
func pointOnSegment(x, y float64, left, right Point) bool {
	cross := (x-left.X)*(right.Y-left.Y) - (y-left.Y)*(right.X-left.X)
	if math.Abs(cross) > pointTolerance {
		return false
	}
	return math.Min(left.X, right.X)-pointTolerance <= x && x <= math.Max(left.X, right.X)+pointTolerance &&
		math.Min(left.Y, right.Y)-pointTolerance <= y && y <= math.Max(left.Y, right.Y)+pointTolerance
}

// pointInRing mirrors interpolation.py:320-329 (_point_in_ring) — ray
// casting with a point-on-boundary short circuit.
func pointInRing(x, y float64, r Ring) bool {
	inside := false
	for i := 0; i < len(r)-1; i++ {
		left, right := r[i], r[i+1]
		if pointOnSegment(x, y, left, right) {
			return true
		}
		if (left.Y > y) != (right.Y > y) {
			intersectionX := left.X + (y-left.Y)*(right.X-left.X)/(right.Y-left.Y)
			if intersectionX > x {
				inside = !inside
			}
		}
	}
	return inside
}

// pointInPolygon mirrors interpolation.py:332-339 (_point_in_polygon) —
// inside the outer ring, and not inside any hole.
func pointInPolygon(x, y float64, p Polygon) bool {
	if !pointInRing(x, y, p[0]) {
		return false
	}
	for _, hole := range p[1:] {
		if pointInRing(x, y, hole) {
			return false
		}
	}
	return true
}

// pointInRegion mirrors interpolation.py:342-347 (_point_in_region) — a
// region can be a MultiPolygon, so containment is "in any member polygon."
func pointInRegion(x, y float64, polygons []Polygon) bool {
	for _, p := range polygons {
		if pointInPolygon(x, y, p) {
			return true
		}
	}
	return false
}

// orientation mirrors interpolation.py:567-574 (_orientation).
func orientation(left, middle, right Point) float64 {
	return (middle.Y-left.Y)*(right.X-middle.X) - (middle.X-left.X)*(right.Y-middle.Y)
}

// segmentsIntersect mirrors interpolation.py:577-607 (_segments_intersect)
// — standard orientation-based segment intersection with an on-segment
// tolerance fallback for collinear/touching cases.
func segmentsIntersect(firstLeft, firstRight, secondLeft, secondRight Point) bool {
	o0 := orientation(firstLeft, firstRight, secondLeft)
	o1 := orientation(firstLeft, firstRight, secondRight)
	o2 := orientation(secondLeft, secondRight, firstLeft)
	o3 := orientation(secondLeft, secondRight, firstRight)
	if ((o0 > pointTolerance && o1 < -pointTolerance) || (o0 < -pointTolerance && o1 > pointTolerance)) &&
		((o2 > pointTolerance && o3 < -pointTolerance) || (o2 < -pointTolerance && o3 > pointTolerance)) {
		return true
	}
	return (math.Abs(o0) <= pointTolerance && pointOnSegment(secondLeft.X, secondLeft.Y, firstLeft, firstRight)) ||
		(math.Abs(o1) <= pointTolerance && pointOnSegment(secondRight.X, secondRight.Y, firstLeft, firstRight)) ||
		(math.Abs(o2) <= pointTolerance && pointOnSegment(firstLeft.X, firstLeft.Y, secondLeft, secondRight)) ||
		(math.Abs(o3) <= pointTolerance && pointOnSegment(firstRight.X, firstRight.Y, secondLeft, secondRight))
}

// rectangleFullyInsideRegion mirrors interpolation.py:610-646
// (_rectangle_fully_inside_region) — all 4 corners inside some member
// polygon AND no boundary edge crosses the rectangle's interior.
func rectangleFullyInsideRegion(x0, x1, y0, y1 float64, polygons []Polygon) bool {
	corners := [4]Point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	edges := [4][2]Point{{corners[0], corners[1]}, {corners[1], corners[2]}, {corners[2], corners[3]}, {corners[3], corners[0]}}
	for _, p := range polygons {
		allCornersInside := true
		for _, c := range corners {
			if !pointInPolygon(c.X, c.Y, p) {
				allCornersInside = false
				break
			}
		}
		if !allCornersInside {
			continue
		}
		crosses := false
		for _, r := range p {
			for _, vertex := range r[:len(r)-1] {
				if x0 < vertex.X && vertex.X < x1 && y0 < vertex.Y && vertex.Y < y1 {
					crosses = true
					break
				}
			}
			if crosses {
				break
			}
			for i := 0; i < len(r)-1 && !crosses; i++ {
				boundaryLeft, boundaryRight := r[i], r[i+1]
				for _, edge := range edges {
					if segmentsIntersect(edge[0], edge[1], boundaryLeft, boundaryRight) {
						crosses = true
						break
					}
				}
			}
			if crosses {
				break
			}
		}
		if !crosses {
			return true
		}
	}
	return false
}

// axisRange mirrors interpolation.py:670-681's np.arange(floor(min/step)*step,
// ceil(max/step)*step + step*0.5, step) — count-based rather than
// cumulative-addition to avoid float drift across many iterations.
func axisRange(minCoord, maxCoord, spacing float64) []float64 {
	start := math.Floor(minCoord/spacing) * spacing
	stop := math.Ceil(maxCoord/spacing)*spacing + spacing*0.5
	count := int(math.Ceil((stop - start) / spacing))
	if count < 1 {
		count = 1
	}
	axis := make([]float64, count)
	for i := range axis {
		axis[i] = start + float64(i)*spacing
	}
	return axis
}

// RegionGrid is the fixed node/cell grid over one interpolation region,
// mirroring interpolation.py's _RegionGrid (minus the interpolated
// `values`/`neighbor_counts`/`nearest_distance_km` fields, which
// surface.go/contour.go compute separately by calling IDWAt per land node
// — keeping grid construction and interpolation as separate, independently
// testable steps rather than one large function like Python's).
type RegionGrid struct {
	XAxis, YAxis []float64
	// LandMask[y][x] reports whether node (XAxis[x], YAxis[y]) is inside
	// the region polygons.
	LandMask [][]bool
	// FullCellMask[y][x] reports whether the cell whose corners are nodes
	// (x,y),(x+1,y),(x+1,y+1),(x,y+1) is entirely inside the region
	// polygons (not just its 4 corners — the whole rectangle).
	FullCellMask [][]bool
}

// BuildRegionGrid mirrors interpolation.py:649-712 (_region_domain, minus
// the @lru_cache — pricebook-service usecase-level caching, if needed, is
// a separate concern from this pure function; see design doc Followup).
func BuildRegionGrid(polygons []Polygon, gridResolutionM float64, maxGridCells int) (RegionGrid, error) {
	var minX, minY, maxX, maxY float64
	first := true
	for _, p := range polygons {
		for _, r := range p {
			for _, pt := range r {
				if first {
					minX, maxX, minY, maxY = pt.X, pt.X, pt.Y, pt.Y
					first = false
					continue
				}
				minX = math.Min(minX, pt.X)
				maxX = math.Max(maxX, pt.X)
				minY = math.Min(minY, pt.Y)
				maxY = math.Max(maxY, pt.Y)
			}
		}
	}
	xAxis := axisRange(minX, maxX, gridResolutionM)
	yAxis := axisRange(minY, maxY, gridResolutionM)
	if len(xAxis)*len(yAxis) > maxGridCells {
		return RegionGrid{}, fmt.Errorf("spatial: grid exceeds max_grid_cells_per_region (%d nodes > %d)", len(xAxis)*len(yAxis), maxGridCells)
	}
	landMask := make([][]bool, len(yAxis))
	for yi, y := range yAxis {
		row := make([]bool, len(xAxis))
		for xi, x := range xAxis {
			row[xi] = pointInRegion(x, y, polygons)
		}
		landMask[yi] = row
	}
	fullCellMask := make([][]bool, max(len(yAxis)-1, 0))
	for yi := 0; yi < len(yAxis)-1; yi++ {
		row := make([]bool, max(len(xAxis)-1, 0))
		for xi := 0; xi < len(xAxis)-1; xi++ {
			if !landMask[yi][xi] || !landMask[yi][xi+1] || !landMask[yi+1][xi+1] || !landMask[yi+1][xi] {
				continue
			}
			row[xi] = rectangleFullyInsideRegion(xAxis[xi], xAxis[xi+1], yAxis[yi], yAxis[yi+1], polygons)
		}
		fullCellMask[yi] = row
	}
	return RegionGrid{XAxis: xAxis, YAxis: yAxis, LandMask: landMask, FullCellMask: fullCellMask}, nil
}
