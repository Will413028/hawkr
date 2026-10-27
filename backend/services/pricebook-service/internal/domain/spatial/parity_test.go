// services/pricebook-service/internal/domain/spatial/parity_test.go
package spatial_test

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tolerances come from testdata/parity/README.md's documented policy
// (design doc §6): feature layer 1e-12, price layer 1e-9. Both sides of
// this fixture emit values that have already been rounded (coordinates 6
// dp, prices 4 dp, nearest_distance_km 3 dp) by Python's _rounded() and
// Go's spatial.RoundTo(), so the tighter price-layer figure is applied
// across the board — any drift large enough to matter would have to push a
// value across a rounding boundary, which 1e-9 catches immediately.
//
// Note: the plan text paraphrased this convention as "features 1e-6,
// prices 1e-4"; no such policy exists in testdata/parity/README.md.
const parityTolerance = 1e-9

const (
	parityEPSG        = 3826
	parityMaxGridCell = 2000
)

type spatialFixture struct {
	RegionRingWGS84 [][2]float64 `json:"region_ring_wgs84"`
	Status          string       `json:"status"`
	Config          struct {
		GridResolutionM        float64   `json:"grid_resolution_m"`
		IDWPower               float64   `json:"idw_power"`
		MaxSupportDistanceKM   float64   `json:"max_support_distance_km"`
		MinSupportingNeighbors int       `json:"min_supporting_neighbors"`
		ContourBreaks          []float64 `json:"contour_breaks"`
	} `json:"config"`
	SurfaceGeojson struct {
		Features []struct {
			Geometry struct {
				Type        string         `json:"type"`
				Coordinates [][][2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				SurfaceRegion           string   `json:"surface_region"`
				PriceTwdPerKg           float64  `json:"price_twd_per_kg"`
				BandIndex               int      `json:"band_index"`
				LowerPriceTwdPerKg      *float64 `json:"lower_price_twd_per_kg"`
				UpperPriceTwdPerKg      *float64 `json:"upper_price_twd_per_kg"`
				SupportingNeighborCount int      `json:"supporting_neighbor_count"`
				NearestDistanceKM       float64  `json:"nearest_distance_km"`
			} `json:"properties"`
		} `json:"features"`
	} `json:"surface_geojson"`
	ContoursGeojson struct {
		Features []struct {
			Geometry struct {
				Type        string       `json:"type"`
				Coordinates [][2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				SurfaceRegion      string  `json:"surface_region"`
				LevelPriceTwdPerKg float64 `json:"level_price_twd_per_kg"`
			} `json:"properties"`
		} `json:"features"`
	} `json:"contours_geojson"`
	PointsGeojson struct {
		Features []struct {
			Geometry struct {
				Coordinates [2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				MarketID       string  `json:"market_id"`
				PriceTwdPerKg  float64 `json:"price_twd_per_kg"`
				VolumeKg       float64 `json:"volume_kg"`
				UsedForSurface bool    `json:"used_for_surface"`
			} `json:"properties"`
		} `json:"features"`
	} `json:"points_geojson"`
}

func loadSpatialFixture(t *testing.T) spatialFixture {
	t.Helper()
	raw, err := os.ReadFile("../../../testdata/parity/spatial/spatial-fixture.json")
	require.NoError(t, err)
	var fixture spatialFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))
	return fixture
}

// parityRegionPolygons rebuilds the fixture's single test region in TM2
// meters — the Go equivalent of interpolation.py's _project_polygon, which
// _region_domain applies before any grid work.
func parityRegionPolygons(t *testing.T, fixture spatialFixture) []spatial.Polygon {
	t.Helper()
	ring := make(spatial.Ring, len(fixture.RegionRingWGS84))
	for i, lonLat := range fixture.RegionRingWGS84 {
		x, y, err := spatial.WGS84ToTWD97TM2(lonLat[0], lonLat[1], parityEPSG)
		require.NoError(t, err)
		ring[i] = spatial.Point{X: x, Y: y}
	}
	return []spatial.Polygon{{ring}}
}

// parityLocations mirrors interpolation.py:_consolidate_locations: project
// each observation, group by the rounded (x, y) metre key, volume-weight
// the price inside each group, and order the result by that key. Ordering
// is load-bearing — IDWAt's exact-match branch takes the *first* location
// within 1e-6 m, exactly as Python's np.flatnonzero(...)[0] does.
//
// Real production counterpart: usecase.consolidateLocations, which
// deliberately DIVERGES from this mirror — it takes an unweighted mean.
// Not because Go has no volumes (aggregateMarkets does volume-weight, one
// step earlier, when it folds a group's sub-variants into one per-market
// number), but because by the time consolidateLocations runs those volumes
// are already baked into each market's single value, and two distinct
// facilities that merely project to the same metre have no meaningful
// weight left to rank them by. Python's _consolidate_locations is doing a
// different job at this point: merging raw observations that still carry
// their own volume_kg. See consolidateLocations' and aggregateMarkets' doc
// comments in internal/usecase/spatial_surface.go. That divergence is
// therefore NOT covered by this fixture; see the spatial README's
// Known-limitations section.
func parityLocations(t *testing.T, fixture spatialFixture) []spatial.Location {
	t.Helper()
	type key struct{ x, y int64 }
	type group struct {
		x, y           float64
		weightedPrice  float64
		positiveVolume float64
		priceSum       float64
		memberCount    int
	}
	groups := map[key]*group{}
	var order []key
	for _, feature := range fixture.PointsGeojson.Features {
		if !feature.Properties.UsedForSurface {
			continue
		}
		lon, lat := feature.Geometry.Coordinates[0], feature.Geometry.Coordinates[1]
		x, y, err := spatial.WGS84ToTWD97TM2(lon, lat, parityEPSG)
		require.NoError(t, err)
		k := key{int64(math.RoundToEven(x)), int64(math.RoundToEven(y))}
		g, ok := groups[k]
		if !ok {
			g = &group{x: x, y: y}
			groups[k] = g
			order = append(order, k)
		}
		volume := math.Max(feature.Properties.VolumeKg, 0)
		g.weightedPrice += feature.Properties.PriceTwdPerKg * volume
		g.positiveVolume += volume
		g.priceSum += feature.Properties.PriceTwdPerKg
		g.memberCount++
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].x != order[j].x {
			return order[i].x < order[j].x
		}
		return order[i].y < order[j].y
	})
	locations := make([]spatial.Location, 0, len(order))
	for _, k := range order {
		g := groups[k]
		value := g.priceSum / float64(g.memberCount)
		if g.positiveVolume > 0 {
			value = g.weightedPrice / g.positiveVolume
		}
		locations = append(locations, spatial.Location{X: g.x, Y: g.y, Value: value})
	}
	return locations
}

// parityNodeField holds the per-node IDW outputs Python keeps on
// _RegionGrid (values / neighbor_counts / nearest_distance_km), indexed
// [y][x] like the grid masks. Non-land nodes stay NaN / 0, matching
// _region_grid's np.full(..., np.nan) + np.zeros initialisation.
type parityNodeField struct {
	values         [][]float64
	neighborCounts [][]int
	nearestKM      [][]float64
}

func parityInterpolate(grid spatial.RegionGrid, locations []spatial.Location, cfg spatial.IDWConfig) parityNodeField {
	field := parityNodeField{
		values:         make([][]float64, len(grid.YAxis)),
		neighborCounts: make([][]int, len(grid.YAxis)),
		nearestKM:      make([][]float64, len(grid.YAxis)),
	}
	for yi := range grid.YAxis {
		field.values[yi] = make([]float64, len(grid.XAxis))
		field.neighborCounts[yi] = make([]int, len(grid.XAxis))
		field.nearestKM[yi] = make([]float64, len(grid.XAxis))
		for xi := range grid.XAxis {
			field.values[yi][xi] = math.NaN()
			field.nearestKM[yi][xi] = math.NaN()
			if !grid.LandMask[yi][xi] {
				continue
			}
			value, neighbors, nearestM, ok := spatial.IDWAt(grid.XAxis[xi], grid.YAxis[yi], locations, cfg)
			field.neighborCounts[yi][xi] = neighbors
			// Python divides by 1_000 once, at the _idw_at_targets
			// boundary; Go's IDWAt hands back metres, so the caller owns
			// that conversion. Getting it wrong here is exactly the
			// cross-function unit slip this fixture exists to catch.
			field.nearestKM[yi][xi] = nearestM / 1000
			if ok {
				field.values[yi][xi] = value
			}
		}
	}
	return field
}

// parityCellIndices replays BuildSurfaceCells' own emission order (full
// cells with four finite corners, y-major) so each emitted cell can be
// traced back to its grid indices — needed to recompute the per-cell
// neighbour/distance aggregates Python attaches as properties.
func parityCellIndices(grid spatial.RegionGrid, values [][]float64) [][2]int {
	var indices [][2]int
	for yi := 0; yi < len(grid.YAxis)-1; yi++ {
		for xi := 0; xi < len(grid.XAxis)-1; xi++ {
			if yi >= len(grid.FullCellMask) || xi >= len(grid.FullCellMask[yi]) || !grid.FullCellMask[yi][xi] {
				continue
			}
			corners := [4]float64{values[yi][xi], values[yi][xi+1], values[yi+1][xi+1], values[yi+1][xi]}
			finite := true
			for _, v := range corners {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					finite = false
					break
				}
			}
			if !finite {
				continue
			}
			indices = append(indices, [2]int{yi, xi})
		}
	}
	return indices
}

// parityProjectedSegment mirrors usecase.projectContours: BuildContours
// deliberately stops in TM2 metres, so unprojecting to WGS84 and dropping
// segments that collapse once rounded (`if left_wgs84 == right_wgs84:
// continue` in interpolation.py) is the caller's contract. This fixture
// therefore validates "domain functions + that documented caller
// contract", which is what a real caller composes — see the spatial
// README's Known limitations.
type parityProjectedSegment struct {
	left, right [2]float64
	level       float64
}

func parityProjectContours(t *testing.T, segments []spatial.ContourSegment) []parityProjectedSegment {
	t.Helper()
	out := make([]parityProjectedSegment, 0, len(segments))
	for _, seg := range segments {
		left := parityToWGS84(t, seg.Left)
		right := parityToWGS84(t, seg.Right)
		if left == right {
			continue
		}
		out = append(out, parityProjectedSegment{left: left, right: right, level: seg.LevelValue})
	}
	return out
}

func parityToWGS84(t *testing.T, p spatial.Point) [2]float64 {
	t.Helper()
	lon, lat, err := spatial.TWD97TM2ToWGS84(p.X, p.Y, parityEPSG)
	require.NoError(t, err)
	return [2]float64{spatial.RoundTo(lon, 6), spatial.RoundTo(lat, 6)}
}

// TestSpatialParity_EndToEndAgainstTransformer composes the whole
// domain/spatial pipeline the way a caller does — BuildRegionGrid → IDWAt
// per land node → BuildSurfaceCells + BuildContours — and compares every
// emitted feature, field by field, against the golden output of
// Hawkr_Transformer_Pricebook's build_spatial_surface. Tasks 1-6 each
// verified one function in isolation; this is the only check that catches
// mismatches living *between* them.
func TestSpatialParity_EndToEndAgainstTransformer(t *testing.T) {
	fixture := loadSpatialFixture(t)

	// The fixture records the config it was generated with, so the Go side
	// is driven by that rather than by hand-copied literals — the test and
	// the fixture cannot drift apart.
	require.Equal(t, "ready", fixture.Status)
	require.Equal(t, 2500.0, fixture.Config.GridResolutionM)
	require.Equal(t, []float64{15, 25, 35}, fixture.Config.ContourBreaks)
	require.Len(t, fixture.PointsGeojson.Features, 4)
	// Absolute anchors on the fixture itself. Everything below compares Go
	// against the fixture, which would happily agree with a silently
	// rescoped regeneration; these pin the scenario this test was written
	// for (49 surface + 21 contour features) so that has to be a conscious
	// edit here too.
	require.Len(t, fixture.SurfaceGeojson.Features, 49)
	require.Len(t, fixture.ContoursGeojson.Features, 21)

	breaks := fixture.Config.ContourBreaks
	polygons := parityRegionPolygons(t, fixture)
	locations := parityLocations(t, fixture)
	require.Len(t, locations, 4, "all four synthetic markets must survive consolidation")

	grid, err := spatial.BuildRegionGrid(polygons, fixture.Config.GridResolutionM, parityMaxGridCell)
	require.NoError(t, err)

	field := parityInterpolate(grid, locations, spatial.IDWConfig{
		Power:                  fixture.Config.IDWPower,
		MaxSupportDistanceM:    fixture.Config.MaxSupportDistanceKM * 1000,
		MinSupportingNeighbors: fixture.Config.MinSupportingNeighbors,
	})

	cells, err := spatial.BuildSurfaceCells(grid, field.values, breaks, parityEPSG)
	require.NoError(t, err)
	segments, err := spatial.BuildContours(grid, field.values, breaks)
	require.NoError(t, err)
	contours := parityProjectContours(t, segments)

	wantSurface := fixture.SurfaceGeojson.Features
	wantContours := fixture.ContoursGeojson.Features
	require.Len(t, cells, len(wantSurface), "surface feature count")
	require.Len(t, contours, len(wantContours), "contour feature count")

	// --- surface layer: every feature, every field ------------------------
	cellIndices := parityCellIndices(grid, field.values)
	require.Len(t, cellIndices, len(cells), "replayed cell order must match BuildSurfaceCells")

	for i, want := range wantSurface {
		got := cells[i]
		require.Equal(t, "Polygon", want.Geometry.Type)
		require.Len(t, want.Geometry.Coordinates, 1, "surface[%d]: one outer ring, no holes", i)
		wantRing := want.Geometry.Coordinates[0]
		require.Len(t, got.PolygonWGS84, len(wantRing), "surface[%d]: ring vertex count", i)
		for v := range wantRing {
			assert.InDelta(t, wantRing[v][0], got.PolygonWGS84[v][0], parityTolerance,
				"surface[%d].ring[%d].lon", i, v)
			assert.InDelta(t, wantRing[v][1], got.PolygonWGS84[v][1], parityTolerance,
				"surface[%d].ring[%d].lat", i, v)
		}

		assert.InDelta(t, want.Properties.PriceTwdPerKg, got.PriceTwdPerKg, parityTolerance,
			"surface[%d].price_twd_per_kg", i)
		assert.Equal(t, want.Properties.BandIndex, got.BandIndex, "surface[%d].band_index", i)
		assertOptionalPrice(t, want.Properties.LowerPriceTwdPerKg, got.LowerBound,
			"surface[%d].lower_price_twd_per_kg", i)
		assertOptionalPrice(t, want.Properties.UpperPriceTwdPerKg, got.UpperBound,
			"surface[%d].upper_price_twd_per_kg", i)

		// Python's _surface_features takes min(neighbour counts) and
		// max(nearest distances) over the cell's four corners.
		yi, xi := cellIndices[i][0], cellIndices[i][1]
		gotNeighbors := minInt(
			field.neighborCounts[yi][xi], field.neighborCounts[yi][xi+1],
			field.neighborCounts[yi+1][xi+1], field.neighborCounts[yi+1][xi],
		)
		gotNearestKM := spatial.RoundTo(math.Max(
			math.Max(field.nearestKM[yi][xi], field.nearestKM[yi][xi+1]),
			math.Max(field.nearestKM[yi+1][xi+1], field.nearestKM[yi+1][xi]),
		), 3)
		assert.Equal(t, want.Properties.SupportingNeighborCount, gotNeighbors,
			"surface[%d].supporting_neighbor_count", i)
		assert.InDelta(t, want.Properties.NearestDistanceKM, gotNearestKM, parityTolerance,
			"surface[%d].nearest_distance_km", i)
	}

	// --- contour layer: every feature, every field ------------------------
	for i, want := range wantContours {
		got := contours[i]
		require.Equal(t, "LineString", want.Geometry.Type)
		require.Len(t, want.Geometry.Coordinates, 2, "contour[%d]: two endpoints", i)
		assert.InDelta(t, want.Geometry.Coordinates[0][0], got.left[0], parityTolerance,
			"contour[%d].left.lon", i)
		assert.InDelta(t, want.Geometry.Coordinates[0][1], got.left[1], parityTolerance,
			"contour[%d].left.lat", i)
		assert.InDelta(t, want.Geometry.Coordinates[1][0], got.right[0], parityTolerance,
			"contour[%d].right.lon", i)
		assert.InDelta(t, want.Geometry.Coordinates[1][1], got.right[1], parityTolerance,
			"contour[%d].right.lat", i)
		assert.InDelta(t, want.Properties.LevelPriceTwdPerKg, spatial.RoundTo(got.level, 4), parityTolerance,
			"contour[%d].level_price_twd_per_kg", i)
	}
}

func assertOptionalPrice(t *testing.T, want *float64, got *float64, msg string, args ...any) {
	t.Helper()
	msgAndArgs := append([]any{msg}, args...)
	if want == nil {
		assert.Nil(t, got, msgAndArgs...)
		return
	}
	if !assert.NotNil(t, got, msgAndArgs...) {
		return
	}
	assert.InDelta(t, *want, *got, parityTolerance, msgAndArgs...)
}

func minInt(values ...int) int {
	smallest := values[0]
	for _, v := range values[1:] {
		if v < smallest {
			smallest = v
		}
	}
	return smallest
}
