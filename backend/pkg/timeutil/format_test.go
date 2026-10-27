package timeutil

import (
	"testing"
	"time"
)

func TestFormatISO8601Ptr(t *testing.T) {
	tm := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	zero := time.Time{}

	tests := []struct {
		name  string
		input *time.Time
		want  string
	}{
		{"正常時間", &tm, "2026-04-04T12:00:00Z"},
		{"nil", nil, ""},
		{"零值", &zero, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatISO8601Ptr(tt.input)
			if got != tt.want {
				t.Errorf("FormatISO8601Ptr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatROCDate(t *testing.T) {
	d1 := time.Date(1993, 1, 15, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	zero := time.Time{}

	tests := []struct {
		name  string
		input *time.Time
		want  string
	}{
		{"民國 82 年", &d1, "082/01/15"},
		{"民國 115 年", &d2, "115/04/04"},
		{"nil", nil, ""},
		{"零值", &zero, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatROCDate(tt.input)
			if got != tt.want {
				t.Errorf("FormatROCDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseROCDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantErr bool
		wantY   int
		wantM   time.Month
		wantD   int
	}{
		{"正常民國年", "082/01/15", false, false, 1993, 1, 15},
		{"民國百年以上", "115/04/04", false, false, 2026, 4, 4},
		{"空字串", "", true, false, 0, 0, 0},
		{"格式錯誤—缺斜線", "0820115", false, true, 0, 0, 0},
		{"年份非三位", "82/01/15", false, true, 0, 0, 0},
		{"月份非兩位", "082/1/15", false, true, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseROCDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseROCDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseROCDate(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if tt.wantErr {
				return
			}
			if got.Year() != tt.wantY || got.Month() != tt.wantM || got.Day() != tt.wantD {
				t.Errorf("ParseROCDate(%q) = %v, want %d-%d-%d", tt.input, got, tt.wantY, tt.wantM, tt.wantD)
			}
		})
	}
}
