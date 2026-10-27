package usecase

import (
	"testing"
	"time"
)

func TestParseROCDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantYear  int
		wantMonth int
		wantDay   int
		wantErr   bool
	}{
		{
			name:      "valid ROC date - 082/03/14",
			input:     "082/03/14",
			wantYear:  1993,
			wantMonth: 3,
			wantDay:   14,
			wantErr:   false,
		},
		{
			name:      "valid ROC date - 105/03/16",
			input:     "105/03/16",
			wantYear:  2016,
			wantMonth: 3,
			wantDay:   16,
			wantErr:   false,
		},
		{
			name:      "valid ROC date - 111/01/01",
			input:     "111/01/01",
			wantYear:  2022,
			wantMonth: 1,
			wantDay:   1,
			wantErr:   false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format - missing parts",
			input:   "082/03",
			wantErr: true,
		},
		{
			name:    "invalid format - too many parts",
			input:   "082/03/14/99",
			wantErr: true,
		},
		{
			name:    "invalid format - year is not fixed width",
			input:   "82/03/14",
			wantErr: true,
		},
		{
			name:    "invalid year",
			input:   "abc/03/14",
			wantErr: true,
		},
		{
			name:    "invalid month",
			input:   "082/ab/14",
			wantErr: true,
		},
		{
			name:    "invalid day",
			input:   "082/03/ab",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseROCDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseROCDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Year() != tt.wantYear {
					t.Errorf("parseROCDate() year = %v, want %v", got.Year(), tt.wantYear)
				}
				if int(got.Month()) != tt.wantMonth {
					t.Errorf("parseROCDate() month = %v, want %v", got.Month(), tt.wantMonth)
				}
				if got.Day() != tt.wantDay {
					t.Errorf("parseROCDate() day = %v, want %v", got.Day(), tt.wantDay)
				}
				// Check timezone is UTC
				if got.Location() != time.UTC {
					t.Errorf("parseROCDate() location = %v, want UTC", got.Location())
				}
			}
		})
	}
}
