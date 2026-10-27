package postgres

import "errors"

// ErrNotFound is returned when a record is not found in the database.
var ErrNotFound = errors.New("not found")
