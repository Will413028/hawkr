package usecase

import (
	"testing"
	"time"
)

func TestTimestamptzParamPreservesDriverValidity(t *testing.T) {
	if got := timestamptzParam(time.Time{}); got.Valid {
		t.Fatal("timestamptzParam(zero).Valid = true, want false")
	}

	value := time.Date(2026, time.August, 27, 12, 34, 56, 0, time.UTC)
	got := timestamptzParam(value)
	if !got.Valid || !got.Time.Equal(value) {
		t.Fatalf("timestamptzParam(%v) = %+v, want valid driver timestamp", value, got)
	}
}
