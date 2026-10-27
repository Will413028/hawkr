package postgres

import (
	"testing"
	"time"
)

func TestNullableTextPreservesEmptyAsNull(t *testing.T) {
	if got := nullableText(""); got != nil {
		t.Fatalf("nullableText(\"\") = %q, want nil", *got)
	}

	const value = "seller@example.com"
	got := nullableText(value)
	if got == nil || *got != value {
		t.Fatalf("nullableText(%q) = %v, want pointer to original value", value, got)
	}
}

func TestStringValueMapsDatabaseNullToDomainEmpty(t *testing.T) {
	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty string", got)
	}

	value := ""
	if got := stringValue(&value); got != "" {
		t.Fatalf("stringValue(pointer to empty string) = %q, want empty string", got)
	}

	value = "present"
	if got := stringValue(&value); got != value {
		t.Fatalf("stringValue(%q) = %q, want original value", value, got)
	}
}

func TestNullableTimePreservesZeroAsNull(t *testing.T) {
	if got := nullableTime(time.Time{}); got != nil {
		t.Fatalf("nullableTime(zero) = %v, want nil", *got)
	}

	value := time.Date(2026, time.August, 26, 12, 34, 56, 0, time.UTC)
	got := nullableTime(value)
	if got == nil || !got.Equal(value) {
		t.Fatalf("nullableTime(%v) = %v, want pointer to original value", value, got)
	}
}

func TestNullableTimePointerPreservesNilAndZeroAsNull(t *testing.T) {
	if got := nullableTimePointer(nil); got != nil {
		t.Fatalf("nullableTimePointer(nil) = %v, want nil", *got)
	}

	zero := time.Time{}
	if got := nullableTimePointer(&zero); got != nil {
		t.Fatalf("nullableTimePointer(zero) = %v, want nil", *got)
	}

	value := time.Date(2026, time.August, 26, 12, 34, 56, 0, time.UTC)
	got := nullableTimePointer(&value)
	if got != &value || !got.Equal(value) {
		t.Fatalf("nullableTimePointer(%v) = %v, want the original pointer", value, got)
	}
}

func TestInt32PointerFromIntPreservesZero(t *testing.T) {
	zero := int32PointerFromInt(0)
	if zero == nil || *zero != 0 {
		t.Fatalf("int32PointerFromInt(0) = %v, want pointer to zero", zero)
	}

	value := int32PointerFromInt(42)
	if value == nil || *value != 42 {
		t.Fatalf("int32PointerFromInt(42) = %v, want pointer to 42", value)
	}
}

func TestNullablePrimitivePointersPreserveValues(t *testing.T) {
	if got := int64Pointer(0); got == nil || *got != 0 {
		t.Fatalf("int64Pointer(0) = %v, want pointer to zero", got)
	}
	if got := boolPointer(false); got == nil || *got {
		t.Fatalf("boolPointer(false) = %v, want pointer to false", got)
	}
	if got := boolValue(nil); got {
		t.Fatal("boolValue(nil) = true, want false")
	}
	if got := float64Pointer(0); got == nil || *got != 0 {
		t.Fatalf("float64Pointer(0) = %v, want pointer to zero", got)
	}
	if got := float64Value(nil); got != 0 {
		t.Fatalf("float64Value(nil) = %v, want zero", got)
	}
	if got := timeValue(nil); !got.IsZero() {
		t.Fatalf("timeValue(nil) = %v, want zero time", got)
	}
}

func TestTimestamptzParamPreservesDriverValidity(t *testing.T) {
	zero := timestamptzParam(time.Time{})
	if zero.Valid {
		t.Fatal("timestamptzParam(zero).Valid = true, want false")
	}

	value := time.Date(2026, time.August, 26, 12, 34, 56, 0, time.UTC)
	converted := timestamptzParam(value)
	if !converted.Valid || !converted.Time.Equal(value) {
		t.Fatalf("timestamptzParam(%v) = %+v, want valid driver timestamp", value, converted)
	}
}
