package repository

import (
	"testing"
	"time"
)

func TestNullableTextPreservesEmptyAsNullPolicy(t *testing.T) {
	if got := nullableText(""); got != nil {
		t.Fatalf("nullableText(\"\") = %q, want nil", *got)
	}

	value := "台南"
	got := nullableText(value)
	if got == nil || *got != value {
		t.Fatalf("nullableText(%q) = %v, want pointer to value", value, got)
	}
}

func TestStringValueMapsNullToDomainEmptyValue(t *testing.T) {
	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty string", got)
	}

	value := "台南"
	if got := stringValue(&value); got != value {
		t.Fatalf("stringValue(&value) = %q, want %q", got, value)
	}
}

func TestNullableTimeMapsZeroToNull(t *testing.T) {
	if got := nullableTime(time.Time{}); got != nil {
		t.Fatalf("nullableTime(zero) = %v, want nil", got)
	}

	value := time.Date(2026, time.August, 27, 12, 34, 56, 0, time.UTC)
	got := nullableTime(value)
	if got == nil || !got.Equal(value) {
		t.Fatalf("nullableTime(%v) = %v, want pointer to value", value, got)
	}
}

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

func TestNullableInt64RoundTripsNullAsDomainZero(t *testing.T) {
	if got := int64Value(nil); got != 0 {
		t.Fatalf("int64Value(nil) = %d, want 0", got)
	}

	value := int64(42)
	if got := int64Value(&value); got != value {
		t.Fatalf("int64Value(&value) = %d, want %d", got, value)
	}
}
