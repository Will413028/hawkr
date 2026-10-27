package usecase

import "testing"

func TestMaskIDCardNo(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"A123456789", "A1******89"}, // standard TW id: first2+last2, 6 stars
		{"", ""},
		{"AB", "**"},     // <=4 fully masked
		{"ABCD", "****"}, // <=4 fully masked
		{"ABCDE", "AB*DE"},
	}
	for _, c := range cases {
		if got := maskIDCardNo(c.in); got != c.want {
			t.Errorf("maskIDCardNo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
