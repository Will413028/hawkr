package usecase

import "time"

func stringPointer(value string) *string {
	return &value
}

func stringOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timePointer(value time.Time) *time.Time {
	return &value
}
