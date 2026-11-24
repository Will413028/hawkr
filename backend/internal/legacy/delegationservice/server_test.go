package delegationservice

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

func TestEveryDenialHasABusinessCode(t *testing.T) {
	seen := map[int32]delegation.DelegationDenial{}
	for d := delegation.DenialInvalidCommand; d <= delegation.DenialAdminNotAllowlisted; d++ {
		code, ok := bizCodes[d]
		if !ok || code < 9260 || code > 9299 {
			t.Errorf("拒絕 %d 沒有業務碼（%d）", d, code)
			continue
		}
		if other, dup := seen[code]; dup {
			t.Errorf("拒絕 %d 與 %d 共用業務碼 %d", d, other, code)
		}
		seen[code] = d
	}
}

func TestParsing(t *testing.T) {
	for s, want := range map[string]bool{"1": true, "2685914682": true, "0": false, "-1": false, "": false, "7494590778405027840x": false} {
		if _, ok := parseID(s); ok != want {
			t.Errorf("parseID(%q) = %v", s, ok)
		}
	}
	if tm, ok := parseTime("2026-09-27T12:00:00.123+08:00"); !ok || !tm.Equal(time.Date(2026, 9, 27, 4, 0, 0, 123e6, time.UTC)) {
		t.Errorf("= %v, %v", tm, ok)
	}
	if _, ok := parseTime("2026-09-27"); ok {
		t.Error("只接受 ISO 8601 的時刻")
	}
	if formatTime(time.Date(2026, 9, 27, 12, 0, 0, 999e6, time.FixedZone("TPE", 8*3600))) != "2026-09-27T04:00:00Z" {
		t.Error("對外的時間是 UTC、截到秒")
	}
	if parseOptionalInt64("O123456789") != nil || *parseOptionalInt64("13") != 13 {
		t.Error("parseOptionalInt64")
	}
}
