package forecast

import (
	"fmt"
	"math"
)

// OrderedNonNegative ports forecasting.py's _ordered_non_negative
// (forecasting.py:111-119): add each residual quantile to the ridge/seasonal
// center, clip below at zero, then take a running maximum left-to-right so
// p10 <= p50 <= p90 always holds — in that order. Swapping the clip and the
// running max changes the result whenever center+residual goes negative for
// more than one quantile (see TestOrderedNonNegativeEnforcesMonotonicityWhenMiddleDips).
func OrderedNonNegative(center float64, residualQuantiles [3]float64) (p10, p50, p90 float64, err error) {
	raw := [3]float64{
		center + residualQuantiles[0],
		center + residualQuantiles[1],
		center + residualQuantiles[2],
	}
	for _, v := range raw {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, 0, fmt.Errorf("forecast: forecast produced a non-finite price")
		}
	}
	running := math.Inf(-1)
	ordered := [3]float64{}
	for i, v := range raw {
		if v < 0 {
			v = 0
		}
		if v > running {
			running = v
		}
		ordered[i] = running
	}
	return ordered[0], ordered[1], ordered[2], nil
}
