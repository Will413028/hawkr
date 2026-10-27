package grpcclient

import (
	"testing"
	"time"
)

// TestTimeoutConstants_MatchCurrentCallSiteValues 鎖住這三個常數的實際數值。
// 這份 refactor 的硬性限制是「不改變任何既有 timeout 數值」——這支測試防止未來
// 手滑把常數定義改成別的秒數卻沒人發現（三個值原樣對應 Global Constraints 記錄的
// 3*time.Second / 5*time.Second / 10*time.Second 三種既有用法）。
func TestTimeoutConstants_MatchCurrentCallSiteValues(t *testing.T) {
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"Timeout3s", Timeout3s, 3 * time.Second},
		{"Timeout5s", Timeout5s, 5 * time.Second},
		{"Timeout10s", Timeout10s, 10 * time.Second},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
