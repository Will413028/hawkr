package spatial

import "math"

// Location is one already-projected (TWD97 TM2 meters) input point for IDW
// interpolation — mirrors interpolation.py's _SurfaceLocation, minus the
// bookkeeping fields (longitude/latitude/volume/market_ids) that belong to
// the caller, not the pure interpolation math.
type Location struct {
	X, Y  float64
	Value float64
}

// IDWConfig mirrors the subset of interpolation.py's SpatialConfig that
// idw.go needs: idw_power, max_support_distance_km (converted to meters by
// the caller), min_supporting_neighbors.
type IDWConfig struct {
	Power                  float64
	MaxSupportDistanceM    float64
	MinSupportingNeighbors int
}

// IDWAt computes the inverse-distance-weighted value at (targetX, targetY)
// from locations, mirroring interpolation.py:423-455 (_idw_at_targets, one
// target at a time rather than numpy-vectorized over an array — grid.go
// calls this once per grid node). Returns ok=false when there are no
// locations, or fewer than cfg.MinSupportingNeighbors fall within
// cfg.MaxSupportDistanceM (Python's np.nan case). nearestDistanceM is
// returned in meters (not kilometers).
func IDWAt(targetX, targetY float64, locations []Location, cfg IDWConfig) (value float64, neighborCount int, nearestDistanceM float64, ok bool) {
	if len(locations) == 0 {
		return 0, 0, 0, false
	}
	nearestDistanceM = math.Inf(1)
	type supported struct {
		distance float64
		value    float64
	}
	var inRange []supported
	// First pass: scan all locations, compute nearestDistanceM and build inRange list
	for _, loc := range locations {
		distance := math.Hypot(targetX-loc.X, targetY-loc.Y)
		if distance < nearestDistanceM {
			nearestDistanceM = distance
		}
		if distance <= cfg.MaxSupportDistanceM {
			inRange = append(inRange, supported{distance: distance, value: loc.Value})
		}
	}
	neighborCount = len(inRange)
	// Check minimum neighbor threshold before considering exact match or interpolation
	if neighborCount < cfg.MinSupportingNeighbors {
		return 0, neighborCount, nearestDistanceM, false
	}
	// Check for exact match among neighbors that passed the threshold
	for _, s := range inRange {
		if s.distance <= 1e-6 {
			// Python: exact = np.flatnonzero(distances[row_index] <= 1e-6);
			// if exact.size: output = values[exact[0]] — first exact match wins.
			return s.value, neighborCount, s.distance, true
		}
	}
	// No exact match; compute weighted average over supporting neighbors
	var weightedSum, weightSum float64
	for _, s := range inRange {
		weight := 1 / math.Pow(s.distance, cfg.Power)
		weightedSum += weight * s.value
		weightSum += weight
	}
	return weightedSum / weightSum, neighborCount, nearestDistanceM, true
}
