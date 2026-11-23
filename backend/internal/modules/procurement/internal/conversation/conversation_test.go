package conversation

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"uuid"
)

func rawToken(t *testing.T, micro int64, id uuid.UUID) string {
	t.Helper()
	b, err := json.Marshal(token{UpdatedAt: micro, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestDecodeTokenRoundTripsAndRejectsTimestampsOutsideTimestamptzRange(t *testing.T) {
	id := uuid.NewV7()
	at := time.Date(2026, 11, 23, 3, 0, 0, 123456000, time.UTC)
	got, ok := decodeToken(encodeToken(at, id))
	if !ok || got.ID != id || got.UpdatedAt != at.UnixMicro() {
		t.Fatalf("round trip = %+v %v", got, ok)
	}
	for name, micro := range map[string]int64{
		"min int64":  math.MinInt64,
		"year -5000": time.Date(-5000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMicro(),
	} {
		if _, ok := decodeToken(rawToken(t, micro, id)); ok {
			t.Errorf("%s 要被拒絕", name)
		}
	}
	if _, ok := decodeToken(rawToken(t, math.MaxInt64, id)); !ok {
		t.Error("int64 微秒的上界（294247 AD）仍在 timestamptz 範圍內，要接受")
	}
	if _, ok := decodeToken(rawToken(t, time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).UnixMicro(), id)); !ok {
		t.Error("範圍內的時間要接受")
	}
}

func TestValidTitleRejectsEveryLineBreakAndControlRune(t *testing.T) {
	for _, r := range []rune{'\n', '\r', '\u000b', '\u000c', '\u0085', ' ', ' ', '\t', 0, '\u007f'} {
		if _, err := ValidTitle("前" + string(r) + "後"); err == nil {
			t.Errorf("U+%04X 在標題中間要被拒絕", r)
		}
	}
	if got, err := ValidTitle("  蘋果　採購 "); err != nil || got != "蘋果　採購" {
		t.Errorf("一般標題 = %q %v", got, err)
	}
	if _, err := ValidTitle(strings.Repeat("字", MaxTitleRunes+1)); err == nil {
		t.Error("超過長度要被拒絕")
	}
	if _, err := ValidTitle("   "); err == nil {
		t.Error("只有分行符的標題要被拒絕")
	}
}
