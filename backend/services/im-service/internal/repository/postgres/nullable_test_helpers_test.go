//go:build integration

package postgres

import "time"

func stringPointer(value string) *string {
	return &value
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}
