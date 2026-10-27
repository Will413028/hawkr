package spatial

import (
	"math"
	"strconv"
)

// SurfaceCell is one price-banded grid cell, ready for GeoJSON Polygon
// serialization (PolygonWGS84 is a single closed ring, [lon,lat] pairs).
//
// Python's reference cell additionally exposes supporting_neighbor_count
// and nearest_distance_km; both were deliberately dropped here, not
// forgotten — issue #2248's wire contract names neither, and nothing
// downstream (the usecase's banding/rollup, the handler's GeoJSON, the
// client) reads them today. IDWAt still computes both (they are asserted
// in the Go↔Python parity fixture), so restoring them is a matter of
// plumbing the values through — but check first whether a real consumer
// needs them, because carrying them per cell inflates every surface
// response by two numbers × ~1000 cells for no current reader.
type SurfaceCell struct {
	PolygonWGS84  [][2]float64
	PriceTwdPerKg float64
	BandIndex     int
	LowerBound    *float64
	UpperBound    *float64
}

// band mirrors interpolation.py:560-564 (_band): breaks are strictly
// increasing upper bounds; band 0 is "below breaks[0]", the last band is
// "at or above breaks[len-1]".
func band(value float64, breaks []float64) (index int, lower, upper *float64) {
	if len(breaks) == 0 {
		return 0, nil, nil
	}
	for i, b := range breaks {
		if value < b {
			if i == 0 {
				return i, nil, floatPtr(b)
			}
			return i, floatPtr(breaks[i-1]), floatPtr(b)
		}
	}
	return len(breaks), floatPtr(breaks[len(breaks)-1]), nil
}

func floatPtr(v float64) *float64 { return &v }

// BuildSurfaceCells mirrors interpolation.py:768-838 (_surface_features)
// for a single region's grid — the caller loops regions and concatenates.
// values must have the same [y][x] shape as grid.LandMask.
func BuildSurfaceCells(grid RegionGrid, values [][]float64, breaks []float64, epsg int) ([]SurfaceCell, error) {
	var cells []SurfaceCell
	for yi := 0; yi < len(grid.YAxis)-1; yi++ {
		for xi := 0; xi < len(grid.XAxis)-1; xi++ {
			if yi >= len(grid.FullCellMask) || xi >= len(grid.FullCellMask[yi]) || !grid.FullCellMask[yi][xi] {
				continue
			}
			corners := [4]float64{values[yi][xi], values[yi][xi+1], values[yi+1][xi+1], values[yi+1][xi]}
			allFinite := true
			for _, v := range corners {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					allFinite = false
					break
				}
			}
			if !allFinite {
				continue
			}
			value := (corners[0] + corners[1] + corners[2] + corners[3]) / 4
			bandIndex, lower, upper := band(value, breaks)
			x0, x1 := grid.XAxis[xi], grid.XAxis[xi+1]
			y0, y1 := grid.YAxis[yi], grid.YAxis[yi+1]
			ringTM2 := [5]Point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}, {x0, y0}}
			ring := make([][2]float64, 5)
			for i, p := range ringTM2 {
				lon, lat, err := TWD97TM2ToWGS84(p.X, p.Y, epsg)
				if err != nil {
					return nil, err
				}
				ring[i] = [2]float64{RoundTo(lon, 6), RoundTo(lat, 6)}
			}
			cells = append(cells, SurfaceCell{
				PolygonWGS84:  ring,
				PriceTwdPerKg: RoundTo(value, 4),
				BandIndex:     bandIndex,
				LowerBound:    lower,
				UpperBound:    upper,
			})
		}
	}
	return cells, nil
}

// RoundTo mirrors interpolation.py:37-39 (_rounded) — round to `digits`
// decimal places using round-half-to-even (banker's rounding, matching Python's
// builtin round()), collapsing -0.0 to 0.0. Uses string round-trip to achieve
// true parity with Python's correctly-rounded decimal conversion semantics.
func RoundTo(value float64, digits int) float64 {
	formatted := strconv.FormatFloat(value, 'f', digits, 64)
	rounded, err := strconv.ParseFloat(formatted, 64)
	if err != nil {
		return value // unreachable: FormatFloat's own output is always valid input to ParseFloat
	}
	if rounded == 0 {
		return 0
	}
	return rounded
}
