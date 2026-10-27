package phone

import "testing"

func TestNormalizeToLocal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"空字串", "", ""},
		{"+886 國際格式", "+886912345678", "0912345678"},
		{"886 無加號", "886912345678", "0912345678"},
		{"已是本地格式", "0912345678", "0912345678"},
		{"含空白", "+886 912 345 678", "0912345678"},
		{"含橫線", "+886-912-345-678", "0912345678"},
		{"含括號", "(+886)912345678", "0912345678"},
		{"混合符號", "+886 (912) 345-678", "0912345678"},
		{"市話不變", "0223456789", "0223456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeToLocal(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeToLocal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
