// pkg/geocode/parse_test.go
package geocode

import "testing"

func TestExtractDetail(t *testing.T) {
	cases := []struct {
		fullAddress string
		county      string
		district    string
		want        string
	}{
		{"桃園市桃園區鎮撫街1號", "桃園市", "桃園區", "鎮撫街1號"},
		{"台北市中正區中華路一段1號", "台北市", "中正區", "中華路一段1號"},
		{"桃園市桃園區", "桃園市", "桃園區", ""},
	}

	for _, tc := range cases {
		t.Run(tc.fullAddress, func(t *testing.T) {
			got := extractDetail(tc.fullAddress, tc.county, tc.district)
			if got != tc.want {
				t.Errorf("期望 %q，got %q", tc.want, got)
			}
		})
	}
}
