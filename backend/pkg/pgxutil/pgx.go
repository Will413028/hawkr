// Package pgxutil contains the few explicit pgx value conversions that cannot
// be expressed by sqlc's generated Go pointer types.
package pgxutil

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// DateParam converts an optional domain date to the pgx value required by a
// generated query parameter. Nil and zero dates are represented as SQL NULL.
// Ordinary nullable columns should use the pointers emitted by sqlc instead.
func DateParam(value *time.Time) pgtype.Date {
	if value == nil || value.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *value, Valid: true}
}

// DateValue converts a pgx date result to the domain's optional time value.
func DateValue(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

// TimestamptzParam converts a domain timestamp to the pgx value required by
// a generated query parameter. A zero timestamp is represented as SQL NULL.
// Ordinary nullable columns should use the pointers emitted by sqlc instead.
func TimestamptzParam(value time.Time) pgtype.Timestamptz {
	if value.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value, Valid: true}
}

// JSONBytes preserves the json.RawMessage type required for pgx's json/jsonb
// codec while applying the repository convention that empty data is NULL.
func JSONBytes(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return value
}

// JSONRawMessage converts database bytes to the typed json.RawMessage value
// expected by generated json/jsonb fields. Empty data is represented as nil.
func JSONRawMessage(value []byte) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return json.RawMessage(value)
}
