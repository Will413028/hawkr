package httputil

import "testing"

func TestParsePageParam(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal int
		want       int
	}{
		{"正常值", "3", 1, 3},
		{"空字串", "", 1, 1},
		{"非數字", "abc", 1, 1},
		{"零值", "0", 1, 1},
		{"負數", "-1", 1, 1},
		{"大數字", "500", 20, 500},
		{"不同預設值", "", 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePageParam(tt.input, tt.defaultVal)
			if got != tt.want {
				t.Errorf("ParsePageParam(%q, %d) = %d, want %d", tt.input, tt.defaultVal, got, tt.want)
			}
		})
	}
}
