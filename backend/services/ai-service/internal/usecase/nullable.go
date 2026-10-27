package usecase

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// timestamptzParam is the explicit driver boundary for generated non-null
// timestamptz parameters. Nullable query columns are represented as pointers
// by sqlc and do not use this helper.
func timestamptzParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}
