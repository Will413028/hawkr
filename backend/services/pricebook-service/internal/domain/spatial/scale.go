package spatial

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	ErrScaleDigestMismatch = errors.New("spatial: price scale file does not match its manifest sha256")
	ErrScaleNoRows         = errors.New("spatial: price scale file has no rows")
)

// PriceScale is one crop or group's versioned, stable price-color-band
// definition (issue #2248: "同 selection/date/horizon 重取必須穩定；不可依當次
// min/max 動態重算" — the direct opposite of Transformer's current
// _nice_contour_breaks, which recomputes per request; see design doc).
//
// Issue #3102 reuses this same struct for a second, deliberately different
// producer — DeriveAdaptiveScale (adaptive_scale.go) — whose breaks ARE
// recomputed per request from that request's own P50 statistics; see that
// function's doc comment for why determinism still holds. LoadPriceScales
// below only ever produces the #2248 fixed-scale variant.
type PriceScale struct {
	ScaleID       string
	PolicyVersion string
	Breaks        []float64
}

type scaleManifest struct {
	SHA256   string `json:"sha256"`
	RowCount int    `json:"row_count"`
}

// LoadPriceScales reads reference/price_scale.v1.csv, keyed by
// "crop:<code>" or "group:<code>" depending on the CSV's selection_kind
// column, mirroring geography.LoadMarketRegistry's sha256-manifest,
// all-or-nothing loading pattern (same rationale: a changed reference file
// without a synced manifest means the review process was bypassed).
func LoadPriceScales(csvPath, manifestPath string) (map[string]PriceScale, error) {
	payload, err := os.ReadFile(csvPath)
	if err != nil {
		return nil, fmt.Errorf("spatial: reading %s: %w", csvPath, err)
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("spatial: reading %s: %w", manifestPath, err)
	}
	var manifest scaleManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, fmt.Errorf("spatial: parsing %s: %w", manifestPath, err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != manifest.SHA256 {
		return nil, ErrScaleDigestMismatch
	}
	reader := csv.NewReader(strings.NewReader(string(payload)))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("spatial: parsing %s: %w", csvPath, err)
	}
	if len(rows) < 2 {
		return nil, ErrScaleNoRows
	}
	header := rows[0]
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}
	for _, required := range []string{"selection_kind", "selection_code", "scale_id", "policy_version", "breaks"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("spatial: price scale CSV missing required column %q", required)
		}
	}
	result := make(map[string]PriceScale, len(rows)-1)
	for lineNumber, row := range rows[1:] {
		kind := row[col["selection_kind"]]
		code := row[col["selection_code"]]
		breaksRaw := strings.Split(row[col["breaks"]], "|")
		breaks := make([]float64, 0, len(breaksRaw))
		for _, b := range breaksRaw {
			v, err := strconv.ParseFloat(strings.TrimSpace(b), 64)
			if err != nil {
				return nil, fmt.Errorf("spatial: price scale CSV row %d: invalid break %q: %w", lineNumber+2, b, err)
			}
			breaks = append(breaks, v)
		}
		for i := 1; i < len(breaks); i++ {
			if breaks[i] <= breaks[i-1] {
				return nil, fmt.Errorf("spatial: price scale CSV row %d: breaks must be strictly increasing, got %v", lineNumber+2, breaks)
			}
		}
		result[kind+":"+code] = PriceScale{
			ScaleID:       row[col["scale_id"]],
			PolicyVersion: row[col["policy_version"]],
			Breaks:        breaks,
		}
	}
	return result, nil
}
