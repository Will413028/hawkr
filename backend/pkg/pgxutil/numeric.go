package pgxutil

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func NumericFromString(s string) (pgtype.Numeric, error) {
	if strings.ContainsAny(s, "eE") {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: 不接受科學記號（該路徑會經過 float64 而失真）", s)
	}
	var n pgtype.Numeric
	if err := n.ScanScientific(s); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: %w", s, err)
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return pgtype.Numeric{}, fmt.Errorf("parse numeric %q: 不接受非有限值（NaN／Infinity）", s)
	}
	return n, nil
}

func NumericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	b, err := n.MarshalJSON()
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), `"`)
}

// NumericToCanonicalString 去掉小數尾隨的零，讓回傳值與呼叫端當初送進來的字面值一致
// ——欄位的 scale 是儲存細節，不該出現在對外 JSON 上。
func NumericToCanonicalString(n pgtype.Numeric) string {
	s := NumericToString(n)
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
