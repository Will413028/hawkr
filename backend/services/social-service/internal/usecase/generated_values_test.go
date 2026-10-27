package usecase

import "time"

func testStringPointer(value string) *string { return &value }

func testTimePointer(value time.Time) *time.Time { return &value }

func nullableTestText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableTestInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
