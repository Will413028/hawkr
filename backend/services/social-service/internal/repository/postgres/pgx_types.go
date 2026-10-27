package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// timestamptzParam is intentionally limited to repository query parameters.
// sqlc's pgx/v5 generator still emits pgtype.Timestamptz for non-null
// timestamptz parameters and nullable expressions; ordinary nullable columns
// use generated Go pointers instead.
func timestamptzParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
