// services/pricebook-service/internal/domain/spatial/rollup.go
package spatial

import "github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"

// CountyRollup is one county's area-weighted price, ready to become one
// boundaryPriceGeojson feature (issue #2248's regionCode/estimatedP50TwdPerKg
// contract — the usecase layer adds regionName/status/sampleCellCount).
type CountyRollup struct {
	CountyCode           string
	CountyName           string
	EstimatedP50TwdPerKg float64
	BandLower, BandUpper *float64
	SupportingCellCount  int
}

func centroid(ring [][2]float64) (lon, lat float64) {
	// Ring is closed (first point repeated as last) — average over the
	// distinct points only, matching a plain polygon centroid for the
	// small, roughly-square cells BuildSurfaceCells produces (not a
	// general area-weighted polygon centroid, which isn't needed here
	// since every cell is the same small rectangle shape).
	n := len(ring) - 1
	for i := 0; i < n; i++ {
		lon += ring[i][0]
		lat += ring[i][1]
	}
	return lon / float64(n), lat / float64(n)
}

// RollupByCounty assigns each surface cell to the county whose polygon
// contains its centroid, then averages assigned cells' PriceTwdPerKg per
// county. Every cell BuildSurfaceCells emits comes from one uniform grid,
// so all cells assigned to a county cover the same area — which makes the
// unweighted mean below *already* the area-weighted result, not an
// approximation of one. That is also why the wire label the handler emits
// is "surface_cell_area_weighted/v1" (internal/grpc/handler.go's
// spatialAggregationMethod): issue #2248 requires the label to name the
// method actually used. Counties with no assigned cells are omitted — the
// caller must not synthesize a value for them (issue #2248: "缺資料行政區回
// null + status/reason，不得補 0 或全國平均").
func RollupByCounty(cells []SurfaceCell, counties []geography.CountyRegion, breaks []float64) []CountyRollup {
	sums := make(map[string]float64, len(counties))
	counts := make(map[string]int, len(counties))
	names := make(map[string]string, len(counties))
	for _, cell := range cells {
		lon, lat := centroid(cell.PolygonWGS84)
		for _, county := range counties {
			if county.Contains(lon, lat) {
				sums[county.CountyCode] += cell.PriceTwdPerKg
				counts[county.CountyCode]++
				names[county.CountyCode] = county.CountyName
				break // first-match-wins, mirrors CountyContaining's linear scan
			}
		}
	}
	var rollups []CountyRollup
	for _, county := range counties {
		count := counts[county.CountyCode]
		if count == 0 {
			continue
		}
		value := sums[county.CountyCode] / float64(count)
		_, lower, upper := band(value, breaks)
		rollups = append(rollups, CountyRollup{
			CountyCode:           county.CountyCode,
			CountyName:           names[county.CountyCode],
			EstimatedP50TwdPerKg: RoundTo(value, 4),
			BandLower:            lower,
			BandUpper:            upper,
			SupportingCellCount:  count,
		})
	}
	return rollups
}
