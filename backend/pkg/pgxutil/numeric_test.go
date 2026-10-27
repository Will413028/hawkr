package pgxutil

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNumericRoundTrip(t *testing.T) {
	for _, s := range []string{"0", "1", "30", "1.5", "0.001", "12345678901.234", "-3.5"} {
		n, err := NumericFromString(s)
		if err != nil {
			t.Fatalf("NumericFromString(%q) error: %v", s, err)
		}
		if got := NumericToString(n); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
}

func TestNumericFromStringRejects(t *testing.T) {
	for _, s := range []string{"1e5", "1E5", "NaN", "Infinity", "-Infinity", "", "abc"} {
		if _, err := NumericFromString(s); err == nil {
			t.Errorf("NumericFromString(%q) should have failed", s)
		}
	}
}

func TestNumericToStringNull(t *testing.T) {
	var n pgtype.Numeric
	if got := NumericToString(n); got != "" {
		t.Errorf("NULL -> %q, want empty", got)
	}
}

func TestNumericToCanonicalString(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"200.000", "200"},
		{"1.000", "1"},
		{"0.500", "0.5"},
		{"0.000", "0"},
		{"200", "200"},
		{"0.125", "0.125"},
	} {
		n, err := NumericFromString(tc.in)
		if err != nil {
			t.Fatalf("NumericFromString(%q): %v", tc.in, err)
		}
		if got := NumericToCanonicalString(n); got != tc.want {
			t.Errorf("NumericToCanonicalString(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := NumericToCanonicalString(pgtype.Numeric{}); got != "" {
		t.Errorf("NULL = %q, want empty", got)
	}
}
