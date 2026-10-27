package timeutil

import (
	"testing"
	"time"
)

func TestParseISO8601(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"無小數秒", "2026-03-27T08:30:00Z", false},
		{"帶毫秒", "2026-03-27T08:30:00.123Z", false},
		{"帶微秒", "2026-03-27T08:30:00.123456Z", false},
		{"帶奈秒", "2026-03-27T08:30:00.123456789Z", false},
		{"帶 offset", "2026-03-27T08:30:00+08:00", false},
		{"空字串", "", true},
		{"非法格式", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseISO8601(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseISO8601(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				want := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
				if got.Year() != want.Year() || got.Month() != want.Month() || got.Day() != want.Day() {
					t.Errorf("ParseISO8601(%q) = %v, want date 2026-03-27", tt.input, got)
				}
			}
		})
	}
}
