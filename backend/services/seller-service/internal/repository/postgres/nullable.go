package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// nullableText applies the repository policy that an empty optional text value
// is stored as SQL NULL.
func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// stringValue maps database NULL to the domain's empty string value.
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableTime applies the repository policy that a zero optional timestamp is
// stored as SQL NULL.
func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// nullableTimePointer preserves nil and zero timestamps as SQL NULL.
func nullableTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	return value
}

// timestamptzParam is the explicit driver boundary for generated non-null
// timestamptz query parameters. Nullable timestamptz fields use *time.Time from
// sqlc instead; only non-null generated fields need this pgx representation.
func timestamptzParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func int32PointerFromInt(value int) *int32 {
	converted := int32(value)
	return &converted
}

func int32Value(value *int32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}

func int64Pointer(value int64) *int64 {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}

func boolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func float64Pointer(value float64) *float64 {
	return &value
}

func float64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
