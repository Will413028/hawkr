package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestTerminationEffectTimingAndText(t *testing.T) {
	// 2026-10-01 23:30:45 UTC = 台北 2026-10-02 07:30:45；預告 30 日後是台北 2026-11-01 07:30。
	now := time.Date(2026, 10, 1, 23, 30, 45, 0, time.UTC)
	for _, g := range TerminationGroundValues() {
		e := NewTerminationEffect(g, now)
		start := TerminationStart(g, now)
		if g == GroundNotice {
			if e.EffectiveAt == nil || !e.EffectiveAt.Equal(start) || !start.Equal(now.Add(720*time.Hour)) {
				t.Errorf("%s：預告要排在 now 加 720 小時：%+v", g, e)
			}
			if got, want := e.Message().Text, "建立後 30 天生效（2026/11/01 07:30）。期間店家照常營業，賣家可提出申覆。"; got != want {
				t.Errorf("預告的說明句 = %q，期望 %q", got, want)
			}
			continue
		}
		if e.EffectiveAt != nil || !start.Equal(now) {
			t.Errorf("%s：其他事由立即生效：%+v", g, e)
		}
		want := "立即生效：店家暫停營業。"
		if g == GroundMaterialBreach {
			// 天數是 MaterialBreachCure（7 日）；常數改了而這裡沒改，這個測試與下面的天數測試都會紅。
			want = "立即生效：店家暫停營業。必須引用至少一筆未撤銷、已存在 7 日以上的認定。"
		}
		if g == GroundLinkedAccount {
			want = "立即生效：店家暫停營業。來源措施撤銷時，這筆會一併撤銷。賣家可對這筆單獨申覆。"
		}
		if got := e.Message().Text; got != want {
			t.Errorf("%s 的說明句 = %q，期望 %q", g, got, want)
		}
	}
	if m := NewTerminationEffect(GroundNotice, now).Message(); m.Locale != uitext.ZhTW {
		t.Errorf("staff 說明句只有 zh-TW：%+v", m)
	}
	got := NewTerminationEffects([]TerminationGround{GroundFraud, GroundNotice}, now)
	if len(got) != 2 || got[0].Ground != GroundFraud || got[1].Ground != GroundNotice {
		t.Errorf("效果清單每個事由一筆、依事由的順序：%+v", got)
	}
}

// 重大違約的說明句帶的天數是 MaterialBreachCure（驗證用的同一個常數）：句子的天數與常數對得上。
// mutation：MaterialBreachCure 改成 14 日而 Message 的 days 寫死 7 → 這個測試紅。
func TestTerminationEffectMaterialBreachDaysFollowTheCure(t *testing.T) {
	days := int(MaterialBreachCure / (24 * time.Hour))
	want := fmt.Sprintf("立即生效：店家暫停營業。必須引用至少一筆未撤銷、已存在 %d 日以上的認定。", days)
	if got := NewTerminationEffect(GroundMaterialBreach, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)).Message().Text; got != want {
		t.Errorf("說明句 = %q，期望 %q", got, want)
	}
}

// 需求值與 API 的 TerminationViolationRequirement 對得上在 httpapi（EnumMismatches）；這裡確認說明句與有狀態檢查的天數同一個來源：
// 只有 CitedFindingMinAge 大於 0 的事由有天數句，其他事由是一般的立即生效句。
// mutation：Message 改回只看 GroundMaterialBreach 而 CitedFindingMinAge 改成別的事由 → 這個測試紅。
func TestTerminationEffectDaysSentenceFollowsCitedFindingMinAge(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, g := range TerminationGroundValues() {
		if g == GroundNotice || g == GroundLinkedAccount {
			continue
		}
		got := NewTerminationEffect(g, now).Message().Text
		wantDays := fmt.Sprintf("已存在 %d 日以上", int(g.CitedFindingMinAge()/(24*time.Hour)))
		if (g.CitedFindingMinAge() > 0) != strings.Contains(got, wantDays) {
			t.Errorf("%s：說明句 %q 與 CitedFindingMinAge %s 不一致", g, got, g.CitedFindingMinAge())
		}
	}
}
