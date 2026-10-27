package domain_test

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestGenerateStoreCode_YYPrefixAndRange(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	code, err := domain.GenerateStoreCode(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code < 2_600_000_000 || code > 2_699_999_999 {
		t.Fatalf("code %d not in expected 2026 range [2600000000, 2699999999]", code)
	}
}

func TestGenerateStoreCode_DifferentYearPrefix(t *testing.T) {
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	code, err := domain.GenerateStoreCode(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code < 2_700_000_000 || code > 2_799_999_999 {
		t.Fatalf("code %d not in expected 2027 range [2700000000, 2799999999]", code)
	}
}

func TestGenerateStoreCode_ProducesDistinctValues(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	seen := map[int64]bool{}
	for i := 0; i < 20; i++ {
		code, err := domain.GenerateStoreCode(now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[code] = true
	}
	if len(seen) < 15 {
		t.Fatalf("expected mostly-distinct codes across 20 draws from a 1e8 space, got %d unique", len(seen))
	}
}
