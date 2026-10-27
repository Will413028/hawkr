package model

import (
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var textLocales = []uitext.Locale{uitext.ZhTW, uitext.EN}

// 每個不能申覆、不能請求解除、不能開店的原因，在每個語系都有非空的句子（en 缺字時退回 zh-TW）。
func TestEveryUnavailableReasonHasTextInEveryLocale(t *testing.T) {
	read := time.Date(2026, 10, 17, 12, 0, 0, 0, twtime.Taipei)
	cooldown := cooldownMeasure(read.Add(time.Hour))
	for _, l := range textLocales {
		for _, r := range AppealUnavailableReasonValues() {
			if appealUnavailableText(l, r).Text == "" {
				t.Errorf("%s：AppealUnavailableReason %s 沒有文字", l, r)
			}
		}
		for _, r := range RemediationUnavailableReasonValues() {
			for _, status := range []MeasureStatus{MeasureStatusEnforced, MeasureStatusRevoked} {
				m := cooldown
				m.Status = status
				if m.remediationUnavailableText(l, r).Text == "" {
					t.Errorf("%s：RemediationUnavailableReason %s（%s）沒有文字", l, r, status)
				}
			}
		}
		for _, b := range OpeningBlockerValues() {
			if b.Text(l).Text == "" {
				t.Errorf("%s：OpeningBlocker %s 沒有文字", l, b)
			}
		}
	}
	// Message.Locale 是實際使用的語系：請求 en 而缺字時，文字是 zh-TW 的就必須標 zh-TW，不能標 en（ADR D3）。
	checkLocale := func(what string, zh, got uitext.Message) {
		t.Helper()
		if zh.Locale != uitext.ZhTW {
			t.Errorf("%s：zh-TW 的 Message.Locale = %q", what, zh.Locale)
		}
		if got.Text == zh.Text && got.Locale != uitext.ZhTW {
			t.Errorf("%s：en 退回 zh-TW 的文字，locale 卻標成 %q", what, got.Locale)
		}
	}
	for _, r := range AppealUnavailableReasonValues() {
		checkLocale("appeal "+string(r), appealUnavailableText(uitext.ZhTW, r), appealUnavailableText(uitext.EN, r))
	}
	for _, r := range RemediationUnavailableReasonValues() {
		checkLocale("remediation "+string(r), cooldown.remediationUnavailableText(uitext.ZhTW, r), cooldown.remediationUnavailableText(uitext.EN, r))
	}
	for _, b := range OpeningBlockerValues() {
		checkLocale("opening "+string(b), b.Text(uitext.ZhTW), b.Text(uitext.EN))
	}
	if appealUnavailableText(uitext.ZhTW, "").Text != "" || cooldown.remediationUnavailableText(uitext.ZhTW, "").Text != "" || OpeningBlocker("").Text(uitext.ZhTW).Text != "" {
		t.Error("空值或不認得的原因不得有文字")
	}
}

// 錯誤的訊息就是可用性清單第一筆的句子（同一個函式），開店也一樣；不同的原因是不同的句子。
func TestOpeningErrorMessageEqualsTheAvailabilityMessage(t *testing.T) {
	seen := map[string]OpeningBlocker{}
	for _, b := range OpeningBlockerValues() {
		msg := b.Text(uitext.ZhTW).Text
		if prev, dup := seen[msg]; dup {
			t.Errorf("%s 與 %s 是同一句：%q", b, prev, msg)
		}
		seen[msg] = b
		err := b.Err(uitext.ZhTW)
		if err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%s：錯誤 %v 不含可用性句子 %q", b, err, msg)
		}
	}
}

func cooldownMeasure(end time.Time) Measure {
	latest := end.Add(-CooldownAfterRejections(RemediationFreeRejections + 1))
	var requests []MeasureRequest
	for range RemediationFreeRejections + 1 {
		decided := latest
		requests = append(requests, MeasureRequest{Kind: RequestKindRemediation, State: RequestRejected, DecidedAt: &decided})
	}
	return Measure{Status: MeasureStatusEnforced, State: StateActive, Basis: BasisRisk, ReadAt: end.Add(-time.Hour), Requests: requests}
}

// COOLDOWN 的句子帶台北時間，秒數進位到分鐘：15:05:01 顯示 15:06，15:05:00 顯示 15:05，奈秒的餘數也進位；時區換成 UTC 輸入也一樣。
func TestCooldownMessageShowsTaipeiMinuteCeil(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  time.Time
		want string
	}{
		{"秒數進位", time.Date(2026, 10, 17, 15, 5, 1, 0, twtime.Taipei), "2026/10/17 15:06"},
		{"整分鐘不動", time.Date(2026, 10, 17, 15, 5, 0, 0, twtime.Taipei), "2026/10/17 15:05"},
		{"奈秒進位", time.Date(2026, 10, 17, 15, 5, 0, 1, twtime.Taipei), "2026/10/17 15:06"},
		{"UTC 輸入換成台北", time.Date(2026, 10, 17, 7, 5, 1, 0, time.UTC), "2026/10/17 15:06"},
		{"跨日", time.Date(2026, 10, 17, 23, 59, 30, 0, twtime.Taipei), "2026/10/18 00:00"},
	} {
		m := cooldownMeasure(tc.end)
		us := m.RemediationUnavailabilities(uitext.ZhTW)
		if len(us) != 1 || us[0].Reason != ReasonCooldown {
			t.Fatalf("%s：%+v", tc.name, us)
		}
		if want := "解除請求目前在冷卻期內，" + tc.want + " 後可再提出"; us[0].Message.Text != want {
			t.Errorf("%s：%q，期望 %q", tc.name, us[0].Message.Text, want)
		}
	}
}

// NOT_ACTIVE 依措施是否撤銷選句；其他原因的句子與 §7(a) 核可的最終句逐字相同。
func TestUnavailabilityMessagesAreTheApprovedSentences(t *testing.T) {
	for r, want := range map[AppealUnavailableReason]string{
		AppealReasonRevoked:         "這筆措施已撤銷，不需要申覆",
		AppealReasonAlreadyAppealed: "這筆措施已提出過申覆，每筆措施只能申覆一次",
		AppealReasonWindowEnded:     "終止合作的申覆期限已過",
		AppealReasonRequestPending:  "這筆措施有待處理的解除請求，審核完成後才能申覆",
	} {
		if got := appealUnavailableText(uitext.ZhTW, r).Text; got != want {
			t.Errorf("%s：%q，期望 %q", r, got, want)
		}
	}
	m := cooldownMeasure(time.Date(2026, 10, 17, 15, 5, 1, 0, twtime.Taipei))
	for r, want := range map[RemediationUnavailableReason]string{
		ReasonRemediationPending: "這筆措施的解除請求審核中，審核完成前不能再提出",
		ReasonAppealPending:      "這筆措施的申覆審核中，審核完成前不能請求解除",
		ReasonNotRisk:            "只有依據風險類型的措施可以請求解除",
		ReasonNotActive:          "這筆措施目前不在生效中",
	} {
		if got := m.remediationUnavailableText(uitext.ZhTW, r).Text; got != want {
			t.Errorf("%s：%q，期望 %q", r, got, want)
		}
	}
	m.Status = MeasureStatusRevoked
	if got := m.remediationUnavailableText(uitext.ZhTW, ReasonNotActive).Text; got != "這筆措施已撤銷" {
		t.Errorf("已撤銷的 NOT_ACTIVE：%q", got)
	}
	for b, want := range map[OpeningBlocker]string{
		OpeningTerminated:      "此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。",
		OpeningSuspended:       "帳號停權中，解除停權前不能開店",
		OpeningAccountDeleting: "帳號正在刪除，不能申請開店",
		OpeningStoreExists:     "你已經是賣家，不需要重複申請",
	} {
		if got := b.Text(uitext.ZhTW).Text; got != want {
			t.Errorf("%s：%q，期望 %q", b, got, want)
		}
	}
}

// 可用性清單與判斷是同一份規則：清單為空 ⇔ CanAppeal／CanRequestRemediation，順序與原因清單相同，每一筆都有句子。
func TestUnavailabilitiesAgreeWithTheReasonLists(t *testing.T) {
	read := time.Date(2026, 10, 17, 12, 0, 0, 0, twtime.Taipei)
	cooling := cooldownMeasure(read.Add(time.Hour))
	cooling.ReadAt = read
	revoked := cooling
	revoked.Status = MeasureStatusRevoked
	revoked.State = StateRevoked
	violation := cooling
	violation.Basis = BasisViolation
	open := Measure{Status: MeasureStatusEnforced, State: StateActive, Basis: BasisRisk, ReadAt: read}
	pendingAppeal := open
	pendingAppeal.Requests = []MeasureRequest{{Kind: RequestKindAppeal, State: RequestPending}}
	for name, m := range map[string]Measure{"冷卻": cooling, "已撤銷": revoked, "違規依據": violation, "可以": open, "申覆待處理": pendingAppeal} {
		appeals, reasons := m.AppealUnavailabilities(uitext.ZhTW), m.AppealUnavailableReasons()
		if (len(appeals) == 0) != m.CanAppeal() || len(appeals) != len(reasons) {
			t.Errorf("%s：appeal 清單 %v 與 CanAppeal=%v 不一致", name, appeals, m.CanAppeal())
		}
		for i, u := range appeals {
			if u.Reason != reasons[i] || u.Message.Text == "" {
				t.Errorf("%s：appeal[%d]=%+v", name, i, u)
			}
		}
		rems, rreasons := m.RemediationUnavailabilities(uitext.ZhTW), m.RemediationUnavailableReasons()
		if (len(rems) == 0) != m.CanRequestRemediation() || len(rems) != len(rreasons) {
			t.Errorf("%s：remediation 清單 %v 與 CanRequestRemediation=%v 不一致", name, rems, m.CanRequestRemediation())
		}
		for i, u := range rems {
			if u.Reason != rreasons[i] || u.Message.Text == "" {
				t.Errorf("%s：remediation[%d]=%+v", name, i, u)
			}
		}
	}
}
