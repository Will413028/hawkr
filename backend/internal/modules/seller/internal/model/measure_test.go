package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestStateOfAtTheBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 12, 8, 0, 0, 123456789, time.UTC)
	micro := now.Truncate(time.Microsecond)
	before, after := micro.Add(-time.Microsecond), micro.Add(time.Microsecond)
	tests := []struct {
		name   string
		status MeasureStatus
		kind   MeasureKind
		starts time.Time
		ends   *time.Time
		want   MeasureState
	}{
		{"警告永遠是已發出", MeasureStatusEnforced, MeasureWarning, before, nil, StateIssued},
		{"撤銷的警告是已撤銷", MeasureStatusRevoked, MeasureWarning, before, nil, StateRevoked},
		{"沒有結束時間的停權生效中", MeasureStatusEnforced, MeasureSuspension, before, nil, StateActive},
		{"建立時間恰是此刻就生效", MeasureStatusEnforced, MeasureSuspension, micro, nil, StateActive},
		{"開始時間在此刻之後尚未生效", MeasureStatusEnforced, MeasureSuspension, after, nil, StateScheduled},
		{"終止合作：開始時間恰是此刻就生效，沒有結束時間永遠生效", MeasureStatusEnforced, MeasureTermination, micro, nil, StateActive},
		{"終止合作：開始時間之前一微秒也已生效", MeasureStatusEnforced, MeasureTermination, before, nil, StateActive},
		{"終止合作：開始時間在此刻之後是預定生效", MeasureStatusEnforced, MeasureTermination, after, nil, StateScheduled},
		{"預告期間內店家關閉：結束時間早於開始時間，是已結束，不是預定生效", MeasureStatusEnforced, MeasureTermination, after, &before, StateEnded},
		{"預告期間內店家關閉：結束時間恰是此刻也是已結束", MeasureStatusEnforced, MeasureTermination, after, &micro, StateEnded},
		{"預定生效而結束時間在未來仍是預定生效", MeasureStatusEnforced, MeasureTermination, after, &after, StateScheduled},
		{"終止合作生效後店家關閉：結束時間到了就是已結束", MeasureStatusEnforced, MeasureTermination, before, &micro, StateEnded},
		{"撤銷的預定終止是已撤銷", MeasureStatusRevoked, MeasureTermination, after, nil, StateRevoked},
		{"結束時間恰是此刻就已結束", MeasureStatusEnforced, MeasureSuspension, before, &micro, StateEnded},
		{"結束時間在此刻之後仍生效", MeasureStatusEnforced, MeasureSuspension, before, &after, StateActive},
		{"結束時間在此刻之前已結束", MeasureStatusEnforced, MeasureSuspension, before, &before, StateEnded},
		{"撤銷的停權是已撤銷", MeasureStatusRevoked, MeasureSuspension, before, nil, StateRevoked},
	}
	for _, tt := range tests {
		if got := StateOf(tt.status, tt.kind, tt.starts, tt.ends, now); got != tt.want {
			t.Errorf("%s：got=%s want=%s", tt.name, got, tt.want)
		}
	}
}

// 不認得的種類不能默默落進任何時間分支（包括 ACTIVE）。
func TestStateOfPanicsOnAnUnknownKind(t *testing.T) {
	for _, kind := range []MeasureKind{"", "SUSPEND", "termination"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("kind=%q 應該 panic", kind)
				}
			}()
			StateOf(MeasureStatusEnforced, kind, time.Unix(0, 0), nil, time.Unix(100, 0))
		}()
	}
	if got := StateOf(MeasureStatusRevoked, "", time.Unix(0, 0), nil, time.Unix(100, 0)); got != StateRevoked {
		t.Errorf("撤銷先於種類的判斷：got=%s", got)
	}
}

// 常數與 SQL 的 CHECK、RetireDueStores 的 hours 參數是同一組數字。
func TestTerminationTimeConstants(t *testing.T) {
	if NoticePeriod != 720*time.Hour || AppealWindow != 4416*time.Hour || StoreRetention != 4416*time.Hour || MaterialBreachCure != 7*24*time.Hour {
		t.Fatalf("notice=%s window=%s retention=%s cure=%s", NoticePeriod, AppealWindow, StoreRetention, MaterialBreachCure)
	}
	if AppealWindowHours != 4416 || StoreRetentionHours != 4416 || StoreRetireAfter != 720*time.Hour || StoreRetireAfterHours != 720 {
		t.Fatalf("hours=%d/%d/%d", AppealWindowHours, StoreRetentionHours, StoreRetireAfterHours)
	}
	if AppealWindow < 184*24*time.Hour {
		t.Fatalf("申覆期間 %s 短於任何連續六個曆月的最大長度（184 天）", AppealWindow)
	}
	// 關店後的個資保留期不短於任何連續六個曆月的最大長度（詐欺犯罪危害防制條例第 37 條第 1 項的六個月）：最長的六個曆月（7～12 月、12～5 月）是 184 天。
	if StoreRetention < 184*24*time.Hour {
		t.Fatalf("保留期 %s 短於任何連續六個曆月的最大長度（184 天）", StoreRetention)
	}
}

func TestMeasureActionPredicates(t *testing.T) {
	appeal := func(state RequestState) []MeasureRequest {
		return []MeasureRequest{{Kind: RequestKindAppeal, State: state}}
	}
	remediation := func(state RequestState) []MeasureRequest {
		return []MeasureRequest{{Kind: RequestKindRemediation, State: state}}
	}
	type can struct{ update, extend, shorten, end, revoke, review, decide, appeal, remediate, decideRemediation bool }
	active := func(basis Basis, requests []MeasureRequest) Measure {
		return Measure{Kind: MeasureSuspension, Status: MeasureStatusEnforced, State: StateActive, Basis: basis, Requests: requests}
	}
	termination := func(state MeasureState, basis Basis, requests []MeasureRequest) Measure {
		return Measure{Kind: MeasureTermination, Status: MeasureStatusEnforced, State: state, Basis: basis, Requests: requests}
	}
	tests := []struct {
		name string
		m    Measure
		want can
	}{
		{"生效中的 RISK 停權", active(BasisRisk, nil), can{true, true, true, true, true, true, false, true, true, false}},
		{"生效中的違規停權", active(BasisViolation, nil), can{true, true, true, false, true, false, false, true, false, false}},
		{"待裁定的申訴凍結理由與延長、撤銷只經裁定", active(BasisViolation, appeal(RequestPending)),
			can{false, false, true, false, false, false, true, false, false, false}},
		{"RISK 的申訴待裁定時不能補正", active(BasisRisk, appeal(RequestPending)),
			can{false, false, true, true, false, true, true, false, false, false}},
		{"駁回之後仍凍結、可撤銷、不能再申訴", active(BasisViolation, appeal(RequestRejected)),
			can{false, false, true, false, true, false, false, false, false, false}},
		{"RISK 的申訴被駁回之後仍可補正", active(BasisRisk, appeal(RequestRejected)),
			can{false, false, true, true, true, true, false, false, true, false}},
		{"待處理的補正凍結理由與延長、不能結束、可縮短與撤銷、不能申訴與再補正", active(BasisRisk, remediation(RequestPending)),
			can{false, false, true, false, true, true, false, false, false, true}},
		{"補正被駁回後可再補正、可申訴", active(BasisRisk, remediation(RequestRejected)),
			can{true, true, true, true, true, true, false, true, true, false}},
		{"補正以 CLOSED 結案後不再凍結", active(BasisRisk, remediation(RequestClosed)),
			can{true, true, true, true, true, true, false, true, true, false}},
		{"措施已結束而待處理的補正沒有標的：可申訴、不能裁定、不能補正", Measure{Kind: MeasureSuspension, Status: MeasureStatusEnforced, State: StateEnded, Basis: BasisRisk, Requests: remediation(RequestPending)},
			can{false, false, false, false, true, false, false, true, false, false}},
		{"已結束的停權不能改也不能延長，仍可撤銷與申訴", Measure{Kind: MeasureSuspension, Status: MeasureStatusEnforced, State: StateEnded, Basis: BasisViolation}, can{false, false, false, false, true, false, false, true, false, false}},
		{"警告可改理由、可撤銷、可申訴", Measure{Kind: MeasureWarning, Status: MeasureStatusEnforced, State: StateIssued, Basis: BasisViolation}, can{true, false, false, false, true, false, false, true, false, false}},
		{"已撤銷的措施什麼都不能做", Measure{Kind: MeasureSuspension, Status: MeasureStatusRevoked, State: StateRevoked, Basis: BasisViolation}, can{}},
		{"預定生效的終止：可改理由、可撤銷、可申訴，不能延長縮短結束與補正", termination(StateScheduled, BasisContract, nil),
			can{true, false, false, false, true, false, false, true, false, false}},
		{"生效中的終止（契約依據）：同上", termination(StateActive, BasisContract, nil), can{true, false, false, false, true, false, false, true, false, false}},
		{"生效中的終止（認定依據）：同上，也不能結束", termination(StateActive, BasisViolation, nil), can{true, false, false, false, true, false, false, true, false, false}},
		{"已結束的終止（預告期間內店家關閉）：不能改理由，仍可撤銷與申訴", termination(StateEnded, BasisContract, nil),
			can{false, false, false, false, true, false, false, true, false, false}},
		{"預定生效的終止申訴待裁定：凍結理由，只經裁定", termination(StateScheduled, BasisContract, appeal(RequestPending)),
			can{false, false, false, false, false, false, true, false, false, false}},
	}
	for _, tt := range tests {
		got := can{
			tt.m.CanUpdateStatement(), tt.m.CanExtend(), tt.m.CanShorten(), tt.m.CanEnd(), tt.m.CanRevoke(), tt.m.CanScheduleReview(),
			tt.m.CanDecideAppeal(), tt.m.CanAppeal(), tt.m.CanRequestRemediation(), tt.m.CanDecideRemediation(),
		}
		if got != tt.want {
			t.Errorf("%s：got=%+v want=%+v", tt.name, got, tt.want)
		}
	}
}

func TestCooldownAfterRejections(t *testing.T) {
	day := 24 * time.Hour
	for n, want := range []time.Duration{0, 0, 0, 7 * day, 14 * day, 28 * day, 28 * day, 28 * day} {
		if got := CooldownAfterRejections(n); got != want {
			t.Errorf("n=%d：got=%s want=%s", n, got, want)
		}
	}
	if CooldownAfterRejections(-1) != 0 {
		t.Error("負數沒有冷卻")
	}
}

func TestRemediationCooldownCountsRejectionsOfEveryKind(t *testing.T) {
	day := 24 * time.Hour
	read := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	rejected := func(kind RequestKind, decided time.Time) MeasureRequest {
		return MeasureRequest{Kind: kind, State: RequestRejected, DecidedAt: &decided}
	}
	closed := func(decided time.Time) MeasureRequest {
		cause := CloseCauseEnded
		return MeasureRequest{Kind: RequestKindRemediation, State: RequestClosed, DecidedAt: &decided, CloseCause: &cause}
	}
	// 新到舊：n 次駁回，第一次是申覆，其餘是解除請求；最近一次在 read 前 1 天。
	build := func(n int, extra ...MeasureRequest) []MeasureRequest {
		var out []MeasureRequest
		for i := n; i >= 1; i-- {
			kind := RequestKindRemediation
			if i == 1 {
				kind = RequestKindAppeal
			}
			out = append(out, rejected(kind, read.Add(-day-time.Duration(n-i)*100*day)))
		}
		return append(out, extra...)
	}
	cases := []struct {
		name         string
		requests     []MeasureRequest
		wantNext     time.Duration
		wantWarning  time.Duration
		wantCanAsked bool
	}{
		{"沒有請求", nil, 0, 0, true},
		{"一次駁回", build(1), 0, 0, true},
		{"兩次駁回還沒有冷卻，但下一次被駁回會冷卻 7 天", build(2), 0, 7 * day, true},
		{"三次駁回：最近一次起算 7 天，下一次 14 天", build(3), 6 * day, 14 * day, false},
		{"四次駁回：14 天，下一次 28 天", build(4), 13 * day, 28 * day, false},
		{"五次駁回：28 天", build(5), 27 * day, 28 * day, false},
		{"六次駁回：仍是 28 天（上限）", build(6), 27 * day, 28 * day, false},
		{"CLOSED 不計：兩次駁回加兩筆結案", build(2, closed(read.Add(-day)), closed(read.Add(-2*day))), 0, 7 * day, true},
		{"待處理的請求不計入駁回，但佔住名額", append([]MeasureRequest{{Kind: RequestKindRemediation, State: RequestPending}}, build(2)...), 0, 7 * day, false},
	}
	for _, c := range cases {
		m := Measure{Status: MeasureStatusEnforced, State: StateActive, Basis: BasisRisk, ReadAt: read, Requests: c.requests}
		next := m.NextRemediationTime()
		switch {
		case c.wantNext == 0 && next != nil:
			t.Errorf("%s：不該有冷卻，got=%s", c.name, next)
		case c.wantNext != 0 && (next == nil || !next.Equal(read.Add(c.wantNext))):
			t.Errorf("%s：next=%v want=%s", c.name, next, read.Add(c.wantNext))
		}
		if got := m.RejectionCooldown(); got != c.wantWarning {
			t.Errorf("%s：RejectionCooldown=%s want=%s", c.name, got, c.wantWarning)
		}
		if got := m.CanRequestRemediation(); got != c.wantCanAsked {
			t.Errorf("%s：CanRequestRemediation=%v want=%v", c.name, got, c.wantCanAsked)
		}
	}

	three := build(3)
	m := Measure{Status: MeasureStatusEnforced, State: StateActive, Basis: BasisRisk, Requests: three, ReadAt: read.Add(7 * day)}
	if m.NextRemediationTime() != nil || !m.CanRequestRemediation() {
		t.Error("冷卻期滿的那一刻（read + 6 天已過）可以再提出")
	}
	m.ReadAt = read.Add(6 * day)
	if m.NextRemediationTime() != nil || !m.CanRequestRemediation() {
		t.Error("冷卻結束時間恰好是此刻就已結束")
	}
	m.ReadAt = read.Add(6*day - time.Microsecond)
	if m.NextRemediationTime() == nil || m.CanRequestRemediation() {
		t.Error("冷卻結束前一微秒仍在冷卻中")
	}
	violation := Measure{Status: MeasureStatusEnforced, State: StateActive, Basis: BasisViolation, Requests: three, ReadAt: read}
	if violation.NextRemediationTime() != nil || violation.RejectionCooldown() != 0 || violation.CanRequestRemediation() {
		t.Error("違規依據的措施沒有補正，也不適用冷卻")
	}
}

func TestRemediationCooldownFieldsOnlyOnActiveRiskMeasures(t *testing.T) {
	day := 24 * time.Hour
	read := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	var requests []MeasureRequest
	for i := 0; i < 3; i++ {
		decided := read.Add(-day - time.Duration(i)*100*day)
		requests = append(requests, MeasureRequest{Kind: RequestKindRemediation, State: RequestRejected, DecidedAt: &decided})
	}
	cases := []struct {
		name  string
		basis Basis
		state MeasureState
		want  bool
	}{
		{"RISK ACTIVE", BasisRisk, StateActive, true},
		{"RISK ENDED", BasisRisk, StateEnded, false},
		{"RISK REVOKED", BasisRisk, StateRevoked, false},
		{"VIOLATION ACTIVE", BasisViolation, StateActive, false},
	}
	for _, c := range cases {
		m := Measure{Status: MeasureStatusEnforced, State: c.state, Basis: c.basis, ReadAt: read, Requests: requests}
		if got := m.NextRemediationTime() != nil; got != c.want {
			t.Errorf("%s：NextRemediationTime 有值=%v want=%v", c.name, got, c.want)
		}
		if got := m.RejectionCooldown() != 0; got != c.want {
			t.Errorf("%s：RejectionCooldown 有值=%v want=%v", c.name, got, c.want)
		}
	}
}

// 請求對外的狀態只有 EffectiveRequest 一個推導點：措施已不在生效中而儲存為 PENDING 的解除請求輸出 CLOSED（ENDED），結案時間是措施的結束時間、沒有
// 裁定的管理員；其餘（生效中措施上的解除請求、已結束措施上的申覆、已裁定的請求）照儲存值；輸入不被修改。
func TestEffectiveRequestClosesAMootRemediationAtTheMeasureEnd(t *testing.T) {
	read := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	ends := read.Add(-time.Hour)
	submitted := read.Add(-48 * time.Hour)
	admin, reason := int64(9), "舊的"
	pendingRemediation := MeasureRequest{Kind: RequestKindRemediation, State: RequestPending, SubmittedAt: submitted, DecidedByAdminID: &admin, DecisionReason: &reason}
	pendingAppeal := MeasureRequest{Kind: RequestKindAppeal, State: RequestPending, SubmittedAt: submitted}
	rejected := MeasureRequest{Kind: RequestKindRemediation, State: RequestRejected, SubmittedAt: submitted, DecidedAt: &ends, DecidedByAdminID: &admin, DecisionReason: &reason}

	ended := Measure{Status: MeasureStatusEnforced, State: StateEnded, Basis: BasisRisk, EndsAt: &ends, ReadAt: read,
		Requests: []MeasureRequest{pendingRemediation, rejected}}
	got := ended.EffectiveRequests()
	if g := got[0]; g.State != RequestClosed || g.CloseCause == nil || *g.CloseCause != CloseCauseEnded || g.DecidedAt == nil || !g.DecidedAt.Equal(ends) ||
		g.DecidedByAdminID != nil || g.DecisionReason != nil || g.Kind != RequestKindRemediation {
		t.Errorf("已結束措施上待處理的補正：%+v", g)
	}
	if g := got[1]; g.State != RequestRejected || g.DecidedByAdminID == nil || g.CloseCause != nil {
		t.Errorf("已裁定的請求照儲存值：%+v", g)
	}
	if ended.Requests[0].State != RequestPending || ended.Requests[0].CloseCause != nil || ended.Requests[0].DecidedAt != nil {
		t.Error("EffectiveRequests 不得修改儲存的請求")
	}
	if ended.Pending() != nil || ended.PendingRemediation() != nil || ended.MootRemediation() == nil {
		t.Error("失去標的的補正不算待處理，但要能找出來讓寫入路徑結案")
	}

	active := ended
	active.State = StateActive
	if g := active.EffectiveRequests()[0]; g.State != RequestPending || g.DecidedAt != nil || active.Pending() == nil || active.MootRemediation() != nil {
		t.Errorf("生效中措施上的補正仍待處理：%+v", g)
	}
	appealOnEnded := ended
	appealOnEnded.Requests = []MeasureRequest{pendingAppeal}
	if g := appealOnEnded.EffectiveRequests()[0]; g.State != RequestPending || appealOnEnded.Pending() == nil || appealOnEnded.MootRemediation() != nil {
		t.Errorf("已結束措施上的申訴仍待處理：%+v", g)
	}
	noEnd := ended
	noEnd.EndsAt = nil
	if g := noEnd.EffectiveRequests()[0]; g.DecidedAt == nil || !g.DecidedAt.Equal(submitted) {
		t.Errorf("沒有結束時間時用提出時間，結案時間不得早於提出時間：%+v", g)
	}
}

// can_request_remediation 與 remediation_unavailable_reasons 來自同一份規則：每一格（狀態、依據、待處理的請求、冷卻）空清單 ⇔ 可以請求，
// 原因依優先序（依據、狀態、待處理的請求、冷卻），而且與不經推導、直接照規則寫的期望相同。
func TestRemediationUnavailableReasonsAgreeWithCanRequestRemediationOnEveryCell(t *testing.T) {
	day := 24 * time.Hour
	read := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	var cooling []MeasureRequest
	for i := 0; i < 3; i++ {
		decided := read.Add(-day - time.Duration(i)*100*day)
		cooling = append(cooling, MeasureRequest{Kind: RequestKindRemediation, State: RequestRejected, DecidedAt: &decided})
	}
	ends := read.Add(-time.Hour)
	type cell struct {
		name   string
		status MeasureStatus
		state  MeasureState
	}
	states := []cell{
		{"生效中", MeasureStatusEnforced, StateActive}, {"已結束", MeasureStatusEnforced, StateEnded},
		{"已撤銷", MeasureStatusRevoked, StateRevoked}, {"警告", MeasureStatusEnforced, StateIssued},
	}
	pendings := map[string]*MeasureRequest{
		"沒有待處理": nil,
		"申訴待處理": {Kind: RequestKindAppeal, State: RequestPending},
		"補正待處理": {Kind: RequestKindRemediation, State: RequestPending},
	}
	for _, st := range states {
		for _, basis := range []Basis{BasisRisk, BasisViolation} {
			for pendingName, pending := range pendings {
				for _, cooldown := range []bool{false, true} {
					var requests []MeasureRequest
					if pending != nil {
						requests = append(requests, *pending)
					}
					if cooldown {
						requests = append(requests, cooling...)
					}
					m := Measure{Status: st.status, State: st.state, Basis: basis, ReadAt: read, EndsAt: &ends, Requests: requests}
					name := st.name + "/" + string(basis) + "/" + pendingName
					if cooldown {
						name += "/冷卻"
					}

					// 期望：照規則逐條寫。已結束措施上待處理的解除請求沒有標的，不算待處理。
					var want []RemediationUnavailableReason
					if basis != BasisRisk {
						want = append(want, ReasonNotRisk)
					}
					if st.state != StateActive {
						want = append(want, ReasonNotActive)
					}
					switch {
					case pending == nil:
					case pending.Kind == RequestKindAppeal:
						want = append(want, ReasonAppealPending)
					case st.state == StateActive:
						want = append(want, ReasonRemediationPending)
					}
					if cooldown && st.state == StateActive && basis == BasisRisk {
						want = append(want, ReasonCooldown)
					}

					got := m.RemediationUnavailableReasons()
					if !slices.Equal(got, want) {
						t.Errorf("%s：reasons=%v want=%v", name, got, want)
					}
					if can := m.CanRequestRemediation(); can != (len(got) == 0) || can != (len(want) == 0) {
						t.Errorf("%s：can=%v reasons=%v", name, can, got)
					}
				}
			}
		}
	}
}

// 伺服器句子用到的名詞（終止事由、風險類型）由共用文案目錄供應：model 的每個值在每個語系與受眾都要查得到，新增值而目錄沒有文字就紅。
func TestCatalogCoversModuleValues(t *testing.T) {
	audiences := map[string]uitext.Text{
		"zh-TW":    uitext.For(uitext.ZhTW),
		"en":       uitext.For(uitext.EN),
		"zh-TW 後台": uitext.Staff(),
	}
	for name, ut := range audiences {
		for _, g := range TerminationGroundValues() {
			if text, ok := ut.SellerTerminationGround(string(g)); !ok || text == "" {
				t.Errorf("%s：目錄沒有終止事由 %s 的名稱", name, g)
			}
		}
		for _, g := range RiskGroundValues() {
			if text, ok := ut.SellerRiskGround(string(g)); !ok || text == "" {
				t.Errorf("%s：目錄沒有風險類型 %s 的名稱", name, g)
			}
		}
		if text, ok := ut.SellerRiskGround("UNKNOWN"); ok || text != "" {
			t.Errorf("%s：不認得的風險類型不得有名稱：%q", name, text)
		}
		if text, ok := ut.SellerTerminationGround("UNKNOWN"); ok || text != "" {
			t.Errorf("%s：不認得的終止事由不得有名稱：%q", name, text)
		}
	}
}

// 不能申覆的原因的順序與邊界：已撤銷、已申覆過、申覆期間已過（終止合作，讀取時間 ≥ 期限）、有待處理的請求；CanAppeal 就是原因為空。
func TestAppealUnavailableReasons(t *testing.T) {
	start := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	deadline := start.Add(AppealWindow)
	termination := func(read time.Time, status MeasureStatus, requests ...MeasureRequest) Measure {
		return Measure{Kind: MeasureTermination, Status: status, StartsAt: start, ReadAt: read, Requests: requests, State: StateActive}
	}
	appeal := func(state RequestState) MeasureRequest { return MeasureRequest{Kind: RequestKindAppeal, State: state} }
	pendingRemediation := MeasureRequest{Kind: RequestKindRemediation, State: RequestPending}
	for _, tc := range []struct {
		name string
		m    Measure
		want []AppealUnavailableReason
	}{
		{"期限前一微秒可申訴", termination(deadline.Add(-time.Microsecond), MeasureStatusEnforced), nil},
		{"預定生效（開始之前）可申訴", termination(start.Add(-time.Hour), MeasureStatusEnforced), nil},
		{"期限那一刻不可", termination(deadline, MeasureStatusEnforced), []AppealUnavailableReason{AppealReasonWindowEnded}},
		{"期限之後不可", termination(deadline.Add(time.Hour), MeasureStatusEnforced), []AppealUnavailableReason{AppealReasonWindowEnded}},
		{"已撤銷且期滿", termination(deadline, MeasureStatusRevoked), []AppealUnavailableReason{AppealReasonRevoked, AppealReasonWindowEnded}},
		{"已申訴過（駁回）", termination(start, MeasureStatusEnforced, appeal(RequestRejected)), []AppealUnavailableReason{AppealReasonAlreadyAppealed}},
		{"申訴待處理", termination(start, MeasureStatusEnforced, appeal(RequestPending)), []AppealUnavailableReason{AppealReasonAlreadyAppealed, AppealReasonRequestPending}},
		{"已申訴且期滿", termination(deadline, MeasureStatusEnforced, appeal(RequestRejected)), []AppealUnavailableReason{AppealReasonAlreadyAppealed, AppealReasonWindowEnded}},
		{"待處理的補正", termination(start, MeasureStatusEnforced, pendingRemediation), []AppealUnavailableReason{AppealReasonRequestPending}},
		{"停權沒有期限", Measure{Kind: MeasureSuspension, Status: MeasureStatusEnforced, StartsAt: start, ReadAt: deadline.Add(1000 * time.Hour)}, nil},
	} {
		got := tc.m.AppealUnavailableReasons()
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s：reasons=%v want %v", tc.name, got, tc.want)
		}
		if tc.m.CanAppeal() != (len(tc.want) == 0) {
			t.Errorf("%s：CanAppeal=%v，應該等於原因為空", tc.name, tc.m.CanAppeal())
		}
	}
	if d := (Measure{Kind: MeasureSuspension, StartsAt: start}).AppealDeadline(); d != nil {
		t.Errorf("停權沒有申覆期限：%v", d)
	}
	if d := (Measure{Kind: MeasureTermination, StartsAt: start}).AppealDeadline(); d == nil || !d.Equal(deadline) {
		t.Errorf("deadline=%v", d)
	}
}

// 終止事由的可用性（建立終止合作的前置條件）：開著的店所有事由（關聯帳號除外，它要來源措施）；已關閉的店沒有預告終止，其他事由只在帳號最近的 stint；
// 已關閉而且不是最近的 stint 時都不行。
func TestAvailableTerminationGroundsByTarget(t *testing.T) {
	var all, noNotice []TerminationGround
	for _, g := range TerminationGroundValues() {
		if g != GroundLinkedAccount {
			all = append(all, g)
		}
		if g != GroundLinkedAccount && g != GroundNotice {
			noNotice = append(noNotice, g)
		}
	}
	for _, tc := range []struct {
		name   string
		target TerminationTarget
		want   []TerminationGround
	}{
		{"開著", TerminationTarget{LatestOfAccount: true}, all},
		{"開著（較新 stint 的旗標不影響）", TerminationTarget{}, all},
		{"已關閉且最近", TerminationTarget{Closed: true, LatestOfAccount: true}, noNotice},
		{"已關閉但有較新的 stint", TerminationTarget{Closed: true}, nil},
	} {
		if got := AvailableTerminationGrounds(tc.target); !slices.Equal(got, tc.want) {
			t.Errorf("%s：%v，期望 %v", tc.name, got, tc.want)
		}
	}
	if (TerminationTarget{Closed: true, LatestOfAccount: true}).BlockFor(GroundNotice) != TerminationBlockStoreClosed ||
		(TerminationTarget{Closed: true}).BlockFor(GroundFraud) != TerminationBlockStoreSuperseded ||
		(TerminationTarget{Closed: true}).BlockFor(GroundNotice) != TerminationBlockStoreClosed {
		t.Error("BlockFor 的原因與優先序")
	}
}

// 關聯終止的來源與可用性：來源要是仍生效、第 21.3 條事由的終止合作、在另一個帳號；可用性再加上沒有生效中的關聯終止與目標店家允許。
func TestLinkedTerminationAvailability(t *testing.T) {
	ground := func(g TerminationGround) *TerminationGround { return &g }
	eligible := LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, Ground: ground(GroundFraud), AccountID: 1}
	open := TerminationTarget{LatestOfAccount: true}
	closed := TerminationTarget{Closed: true, LatestOfAccount: true}
	for _, tc := range []struct {
		name    string
		source  LinkedSource
		linked  bool
		target  TerminationTarget
		block   LinkedSourceBlock
		allowed bool
	}{
		{"可以", eligible, false, open, "", true},
		{"目標店家已關閉仍可以", eligible, false, closed, "", true},
		{"已建立", eligible, true, open, "", false},
		{"來源已撤銷", LinkedSource{Status: MeasureStatusRevoked, Kind: MeasureTermination, Ground: ground(GroundFraud), AccountID: 1}, false, open, LinkedSourceRevoked, false},
		{"來源是停權", LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureSuspension, AccountID: 1}, false, open, LinkedSourceNotEligible, false},
		{"來源是預告終止", LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, Ground: ground(GroundNotice), AccountID: 1}, false, open, LinkedSourceNotEligible, false},
		{"來源是關聯終止", LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, Ground: ground(GroundLinkedAccount), AccountID: 1}, false, open, LinkedSourceNotEligible, false},
		{"終止沒有事由", LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, AccountID: 1}, false, open, LinkedSourceNotEligible, false},
		{"同一個帳號", LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, Ground: ground(GroundFraud), AccountID: 2}, false, open, LinkedSourceSameAccount, false},
		{"目標有較新的 stint", eligible, false, TerminationTarget{Closed: true}, "", false},
	} {
		if got := tc.source.BlockFor(2); got != tc.block && tc.name != "同一個帳號" {
			t.Errorf("%s：BlockFor=%q，期望 %q", tc.name, got, tc.block)
		}
		if got := CanCreateLinkedTermination(tc.source, 2, tc.linked, tc.target); got != tc.allowed {
			t.Errorf("%s：Can=%v，期望 %v", tc.name, got, tc.allowed)
		}
	}
	same := LinkedSource{Status: MeasureStatusEnforced, Kind: MeasureTermination, Ground: ground(GroundFraud), AccountID: 2}
	if same.BlockFor(2) != LinkedSourceSameAccount || CanCreateLinkedTermination(same, 2, false, open) {
		t.Error("來源與目標同一個帳號")
	}
}

// ApplyStore 的前置條件：終止先於停權（終止撤銷之前，解除停權也開不了店），原因各自帶 reason 與正確的訊息。
func TestOpeningBlockSeparatesTerminationFromSuspension(t *testing.T) {
	suspension := EffectiveMeasure{MeasureID: uuid.NewV7(), Kind: MeasureSuspension}
	termination := EffectiveMeasure{MeasureID: uuid.NewV7(), Kind: MeasureTermination}
	if OpeningBlock(uitext.ZhTW, OpeningState{AccountActive: true, Sanctions: NoSanctions()}) != nil {
		t.Fatal("沒有措施可以開店")
	}
	for _, tc := range []struct {
		name string
		s    *Sanctions
		want apperr.Reason
		text string
	}{
		{"停權", SanctionedBy(suspension), apperr.ReasonStoreOpeningSuspended, "帳號停權中，解除停權前不能開店"},
		{"終止", SanctionedBy(termination), apperr.ReasonStoreOpeningTerminated, "此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。"},
		{"停權加終止", SanctionedBy(suspension, termination), apperr.ReasonStoreOpeningTerminated, "此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。"},
		{"沒有推導過（fail-closed）", nil, apperr.ReasonStoreOpeningSuspended, "帳號停權中，解除停權前不能開店"},
	} {
		var ae *apperr.Error
		err := OpeningBlock(uitext.ZhTW, OpeningState{AccountActive: true, Sanctions: tc.s})
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != tc.want || ae.Message != tc.text {
			t.Errorf("%s：%v", tc.name, err)
		}
	}
}

// 開店入口的原因清單：依優先序（終止合作、停權、帳號正在刪除、已有店家），終止與停權同時存在時兩個都列、終止在前；只有終止時不再標成停權；
// 沒有推導過（nil）時 fail-closed 標成停權。可以開店 ⇔ 清單為空。
func TestOpeningBlockersAreOrderedAndExact(t *testing.T) {
	suspension := EffectiveMeasure{MeasureID: uuid.NewV7(), Kind: MeasureSuspension}
	termination := EffectiveMeasure{MeasureID: uuid.NewV7(), Kind: MeasureTermination}
	for name, tc := range map[string]struct {
		in   OpeningState
		want []OpeningBlocker
	}{
		"沒有任何原因": {OpeningState{AccountActive: true, Sanctions: NoSanctions()}, nil},
		"停權":     {OpeningState{AccountActive: true, Sanctions: SanctionedBy(suspension)}, []OpeningBlocker{OpeningSuspended}},
		"只有終止":   {OpeningState{AccountActive: true, Sanctions: SanctionedBy(termination)}, []OpeningBlocker{OpeningTerminated}},
		"終止加停權":  {OpeningState{AccountActive: true, Sanctions: SanctionedBy(suspension, termination)}, []OpeningBlocker{OpeningTerminated, OpeningSuspended}},
		"沒有推導過":  {OpeningState{AccountActive: true}, []OpeningBlocker{OpeningSuspended}},
		"帳號正在刪除": {OpeningState{Sanctions: NoSanctions()}, []OpeningBlocker{OpeningAccountDeleting}},
		"已有店家":   {OpeningState{AccountActive: true, HasStore: true, Sanctions: NoSanctions()}, []OpeningBlocker{OpeningStoreExists}},
		"全部":     {OpeningState{HasStore: true, Sanctions: SanctionedBy(suspension, termination)}, []OpeningBlocker{OpeningTerminated, OpeningSuspended, OpeningAccountDeleting, OpeningStoreExists}},
	} {
		got := OpeningBlockers(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%s：%v，要 %v", name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s：%v，要 %v", name, got, tc.want)
				break
			}
		}
		if (OpeningBlock(uitext.ZhTW, tc.in) == nil) != (len(tc.want) == 0) {
			t.Errorf("%s：OpeningBlock 與清單不一致", name)
		}
	}
}

// 空值或不認得的原因不是「已有店家」：Err 回沒有分類的錯誤，不當成任何一種有效的原因。
func TestUnknownOpeningBlockerIsNotAValidReason(t *testing.T) {
	for _, b := range []OpeningBlocker{"", "SOMETHING_ELSE"} {
		err := b.Err(uitext.ZhTW)
		if err == nil || apperr.KindOf(err) == apperr.KindAlreadyExists || apperr.KindOf(err) == apperr.KindFailedPrecondition {
			t.Errorf("%q：%v", b, err)
		}
	}
}

// legacyActionPredicates 是改成原因清單之前的四個布林判斷，逐字照舊；資料集測試用它當對照，鎖住「哪些狀態允許哪個動作」沒有改變。
func legacyActionPredicates(m Measure) (update, extend, shorten, revoke bool) {
	update = m.Status == MeasureStatusEnforced && (m.State == StateIssued || m.State == StateActive || m.State == StateScheduled) && m.Appeal() == nil && m.PendingRemediation() == nil
	extend = m.Kind == MeasureSuspension && m.State == StateActive && m.Appeal() == nil && m.PendingRemediation() == nil
	shorten = m.Kind == MeasureSuspension && m.State == StateActive
	pending := m.Pending()
	revoke = m.Status == MeasureStatusEnforced && !(pending != nil && pending.Kind == RequestKindAppeal)
	return update, extend, shorten, revoke
}

func TestActionUnavailableReasonsAgreeWithTheLegacyPredicatesOnEveryCell(t *testing.T) {
	read := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	type cell struct {
		status MeasureStatus
		kind   MeasureKind
		state  MeasureState
	}
	// 只取 StateOf 推得出的組合：撤銷一定是 REVOKED，警告未撤銷一定是 ISSUED，停權與終止不會是 ISSUED。
	var cells []cell
	for _, kind := range MeasureKindValues() {
		cells = append(cells, cell{MeasureStatusRevoked, kind, StateRevoked})
		if kind == MeasureWarning {
			cells = append(cells, cell{MeasureStatusEnforced, kind, StateIssued})
			continue
		}
		for _, state := range []MeasureState{StateActive, StateEnded, StateScheduled} {
			cells = append(cells, cell{MeasureStatusEnforced, kind, state})
		}
	}
	if len(cells) != 10 {
		t.Fatalf("cells=%d", len(cells))
	}
	requestSets := map[string][]MeasureRequest{
		"沒有請求":     nil,
		"申訴待裁定":    {{Kind: RequestKindAppeal, State: RequestPending}},
		"申訴被駁回":    {{Kind: RequestKindAppeal, State: RequestRejected}},
		"申訴被接受":    {{Kind: RequestKindAppeal, State: RequestAccepted}},
		"補正待處理":    {{Kind: RequestKindRemediation, State: RequestPending}},
		"補正被駁回":    {{Kind: RequestKindRemediation, State: RequestRejected}},
		"補正已結案":    {{Kind: RequestKindRemediation, State: RequestClosed}},
		"駁回的申訴加補正": {{Kind: RequestKindAppeal, State: RequestRejected}, {Kind: RequestKindRemediation, State: RequestPending}},
	}
	checked := 0
	for _, c := range cells {
		for name, requests := range requestSets {
			m := Measure{Kind: c.kind, Status: c.status, State: c.state, Basis: BasisViolation, ReadAt: read, Requests: requests}
			label := string(c.kind) + "/" + string(c.status) + "/" + string(c.state) + "/" + name
			wantUpdate, wantExtend, wantShorten, wantRevoke := legacyActionPredicates(m)
			for action, check := range map[string]struct {
				act     MeasureAction
				reasons []MeasureActionUnavailableReason
				can     bool
				want    bool
			}{
				"改理由": {MeasureActionUpdateStatement, m.UpdateStatementUnavailableReasons(), m.CanUpdateStatement(), wantUpdate},
				"延長":  {MeasureActionExtend, m.ExtendUnavailableReasons(), m.CanExtend(), wantExtend},
				"縮短":  {MeasureActionShorten, m.ShortenUnavailableReasons(), m.CanShorten(), wantShorten},
				"撤銷":  {MeasureActionRevoke, m.RevokeUnavailableReasons(), m.CanRevoke(), wantRevoke},
			} {
				if (len(check.reasons) == 0) != check.want || check.can != check.want {
					t.Errorf("%s %s：reasons=%v can=%v want=%v", label, action, check.reasons, check.can, check.want)
				}
				known := MeasureActionUnavailableReasonValues()
				for _, r := range check.reasons {
					if !slices.Contains(known, r) {
						t.Errorf("%s %s：不認得的原因 %q", label, action, r)
					}
					if ActionUnavailableText(r, check.act).Text == "" {
						t.Errorf("%s %s：原因 %q 沒有文字", label, action, r)
					}
				}
				if !slices.Equal(m.ActionUnavailableReasons(check.act), check.reasons) {
					t.Errorf("%s %s：ActionUnavailableReasons 與各動作的清單不一致", label, action)
				}
			}
			checked++
		}
	}
	if checked != len(cells)*len(requestSets) {
		t.Fatalf("checked=%d", checked)
	}
}

// legacyCanEnd 是改成原因清單之前的結束判斷，逐字照舊：只限生效中的 RISK 措施，而且沒有待處理的解除請求。
func legacyCanEnd(m Measure) bool {
	return m.State == StateActive && m.Basis == BasisRisk && m.PendingRemediation() == nil
}

// 結束的原因清單與舊判斷在每一格都一致（空 ⇔ 可以結束），清單的內容與順序也固定：這是 can_end、End 的前置檢查與 end_unavailabilities 的同一個規則。
func TestEndUnavailableReasonsAgreeWithTheLegacyPredicateOnEveryCell(t *testing.T) {
	read := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	pendingRemediation := []MeasureRequest{{Kind: RequestKindRemediation, State: RequestPending}}
	checked := 0
	for _, kind := range MeasureKindValues() {
		for _, basis := range []Basis{BasisRisk, BasisViolation, BasisContract} {
			for _, st := range []struct {
				status MeasureStatus
				state  MeasureState
			}{{MeasureStatusRevoked, StateRevoked}, {MeasureStatusEnforced, StateIssued}, {MeasureStatusEnforced, StateActive}, {MeasureStatusEnforced, StateEnded}, {MeasureStatusEnforced, StateScheduled}} {
				for name, requests := range map[string][]MeasureRequest{"沒有請求": nil, "補正待處理": pendingRemediation} {
					m := Measure{Kind: kind, Basis: basis, Status: st.status, State: st.state, ReadAt: read, Requests: requests}
					label := string(kind) + "/" + string(basis) + "/" + string(st.status) + "/" + string(st.state) + "/" + name
					reasons := m.EndUnavailableReasons()
					if (len(reasons) == 0) != legacyCanEnd(m) || m.CanEnd() != legacyCanEnd(m) {
						t.Errorf("%s：reasons=%v can=%v want=%v", label, reasons, m.CanEnd(), legacyCanEnd(m))
					}
					if !slices.Equal(m.ActionUnavailableReasons(MeasureActionEnd), reasons) {
						t.Errorf("%s：ActionUnavailableReasons 與結束的清單不一致", label)
					}
					for _, r := range reasons {
						if !slices.Contains(MeasureActionUnavailableReasonValues(), r) || ActionUnavailableText(r, MeasureActionEnd).Text == "" {
							t.Errorf("%s：原因 %q 不認得或沒有文字", label, r)
						}
					}
					checked++
				}
			}
		}
	}
	if checked != len(MeasureKindValues())*3*5*2 {
		t.Fatalf("checked=%d", checked)
	}
	// 內容與順序（獨立於實作的預期表）。
	active := Measure{Kind: MeasureSuspension, Basis: BasisRisk, Status: MeasureStatusEnforced, State: StateActive, ReadAt: read}
	for name, tc := range map[string]struct {
		m    Measure
		want []MeasureActionUnavailableReason
	}{
		"生效中的風險停權":    {active, nil},
		"依據違規的停權":     {Measure{Kind: MeasureSuspension, Basis: BasisViolation, Status: MeasureStatusEnforced, State: StateActive, ReadAt: read}, []MeasureActionUnavailableReason{ActionReasonViolationBasis}},
		"終止合作":        {Measure{Kind: MeasureTermination, Basis: BasisContract, Status: MeasureStatusEnforced, State: StateActive, ReadAt: read}, []MeasureActionUnavailableReason{ActionReasonTerminationRecreate}},
		"風險停權有待處理的補正": {Measure{Kind: MeasureSuspension, Basis: BasisRisk, Status: MeasureStatusEnforced, State: StateActive, ReadAt: read, Requests: pendingRemediation}, []MeasureActionUnavailableReason{ActionReasonRemediationPending}},
		"已撤銷的違規停權":    {Measure{Kind: MeasureSuspension, Basis: BasisViolation, Status: MeasureStatusRevoked, State: StateRevoked, ReadAt: read}, []MeasureActionUnavailableReason{ActionReasonRevoked, ActionReasonNotActive, ActionReasonViolationBasis}},
	} {
		if got := tc.m.EndUnavailableReasons(); !slices.Equal(got, tc.want) {
			t.Errorf("%s：%v，要 %v", name, got, tc.want)
		}
	}
	// 違規依據的句子與原本 admin 狀態卡寫死的字相同（搬到伺服器，不是新文案）。
	if got := ActionUnavailableText(ActionReasonViolationBasis, MeasureActionEnd).Text; got != "因違規處分停權；違規認定只供查閱，撤銷要對這個措施進行。" {
		t.Errorf("違規依據的句子：%q", got)
	}
}

// 文字：每個有效原因至少在一個動作下有句子；UNSPECIFIED 與不認得的原因、凍結搭配不會凍結的動作都不是有效組合（空字串）；每句有主詞，不含 client 的導覽名稱。
func TestActionUnavailableTextCoversEveryValidCombination(t *testing.T) {
	for _, r := range MeasureActionUnavailableReasonValues() {
		any := false
		for _, a := range MeasureActionValues() {
			if text := ActionUnavailableText(r, a).Text; text != "" {
				any = true
				if strings.Contains(text, "「") {
					t.Errorf("%s/%s：文字不寫 client 的導覽名稱：%q", r, a, text)
				}
			}
		}
		if !any {
			t.Errorf("原因 %q 沒有任何文字", r)
		}
	}
	for _, bad := range []MeasureActionUnavailableReason{"", "UNSPECIFIED", "SOMETHING_ELSE"} {
		for _, a := range MeasureActionValues() {
			if ActionUnavailableText(bad, a).Text != "" {
				t.Errorf("%q 不是有效的原因，不能有文字", bad)
			}
		}
	}
	for _, a := range []MeasureAction{MeasureActionShorten, MeasureActionRevoke, ""} {
		if ActionUnavailableText(ActionReasonRequestPendingFrozen, a).Text != "" {
			t.Errorf("凍結只出現在改理由與延長，%q 不該有文字", a)
		}
	}
	if (Measure{}).ActionUnavailableReasons("") != nil {
		t.Error("不認得的動作沒有原因")
	}
}

func TestActionUnavailableReasonsAreExactAndOrdered(t *testing.T) {
	appeal := func(state RequestState) []MeasureRequest {
		return []MeasureRequest{{Kind: RequestKindAppeal, State: state}}
	}
	remediation := []MeasureRequest{{Kind: RequestKindRemediation, State: RequestPending}}
	suspension := func(state MeasureState, requests []MeasureRequest) Measure {
		return Measure{Kind: MeasureSuspension, Status: MeasureStatusEnforced, State: state, Requests: requests}
	}
	revoked := Measure{Kind: MeasureSuspension, Status: MeasureStatusRevoked, State: StateRevoked, Requests: appeal(RequestPending)}
	warning := Measure{Kind: MeasureWarning, Status: MeasureStatusEnforced, State: StateIssued}
	termination := func(state MeasureState) Measure {
		return Measure{Kind: MeasureTermination, Status: MeasureStatusEnforced, State: state, Basis: BasisContract}
	}
	type reasons = []MeasureActionUnavailableReason
	tests := []struct {
		name                            string
		m                               Measure
		update, extend, shorten, revoke reasons
	}{
		{"生效中的停權", suspension(StateActive, nil), nil, nil, nil, nil},
		{"申訴待裁定：凍結、只能縮短、撤銷改經裁定", suspension(StateActive, appeal(RequestPending)),
			reasons{ActionReasonRequestPendingFrozen}, reasons{ActionReasonRequestPendingFrozen}, nil, reasons{ActionReasonAppealPending}},
		{"申訴已駁回：仍凍結，可撤銷", suspension(StateActive, appeal(RequestRejected)),
			reasons{ActionReasonRequestPendingFrozen}, reasons{ActionReasonRequestPendingFrozen}, nil, nil},
		{"補正待處理：凍結，可撤銷", suspension(StateActive, remediation),
			reasons{ActionReasonRequestPendingFrozen}, reasons{ActionReasonRequestPendingFrozen}, nil, nil},
		{"已結束的停權：結構性原因在前，凍結在最後", suspension(StateEnded, appeal(RequestRejected)),
			reasons{ActionReasonNotActive, ActionReasonRequestPendingFrozen}, reasons{ActionReasonNotActive, ActionReasonRequestPendingFrozen},
			reasons{ActionReasonNotActive}, nil},
		{"已撤銷且有申訴待裁定：全部列出", revoked,
			reasons{ActionReasonRevoked, ActionReasonNotActive, ActionReasonRequestPendingFrozen},
			reasons{ActionReasonRevoked, ActionReasonNotActive, ActionReasonRequestPendingFrozen},
			reasons{ActionReasonRevoked, ActionReasonNotActive},
			reasons{ActionReasonRevoked, ActionReasonAppealPending}},
		{"已撤銷的警告", Measure{Kind: MeasureWarning, Status: MeasureStatusRevoked, State: StateRevoked},
			reasons{ActionReasonRevoked, ActionReasonNotActive},
			reasons{ActionReasonRevoked, ActionReasonKindHasNoEndTime, ActionReasonNotActive},
			reasons{ActionReasonRevoked, ActionReasonKindHasNoEndTime, ActionReasonNotActive},
			reasons{ActionReasonRevoked}},
		{"警告", warning, nil, reasons{ActionReasonKindHasNoEndTime, ActionReasonNotActive}, reasons{ActionReasonKindHasNoEndTime, ActionReasonNotActive}, nil},
		{"預定生效的終止", termination(StateScheduled), nil,
			reasons{ActionReasonTerminationRecreate, ActionReasonNotActive}, reasons{ActionReasonTerminationRecreate, ActionReasonNotActive}, nil},
		{"生效中的終止", termination(StateActive), nil, reasons{ActionReasonTerminationRecreate}, reasons{ActionReasonTerminationRecreate}, nil},
		{"已結束的終止", termination(StateEnded), reasons{ActionReasonNotActive},
			reasons{ActionReasonTerminationRecreate, ActionReasonNotActive}, reasons{ActionReasonTerminationRecreate, ActionReasonNotActive}, nil},
		{"已撤銷的終止合作：不是撤銷後重建，只有已撤銷與不在生效中",
			Measure{Kind: MeasureTermination, Status: MeasureStatusRevoked, State: StateRevoked, Basis: BasisContract},
			reasons{ActionReasonRevoked, ActionReasonNotActive}, reasons{ActionReasonRevoked, ActionReasonNotActive},
			reasons{ActionReasonRevoked, ActionReasonNotActive}, reasons{ActionReasonRevoked}},
		{"預定生效的停權", suspension(StateScheduled, nil), nil, reasons{ActionReasonNotActive}, reasons{ActionReasonNotActive}, nil},
	}
	for _, tt := range tests {
		for action, pair := range map[string][2]reasons{
			"改理由": {tt.m.UpdateStatementUnavailableReasons(), tt.update}, "延長": {tt.m.ExtendUnavailableReasons(), tt.extend},
			"縮短": {tt.m.ShortenUnavailableReasons(), tt.shorten}, "撤銷": {tt.m.RevokeUnavailableReasons(), tt.revoke},
		} {
			if !slices.Equal(pair[0], pair[1]) {
				t.Errorf("%s %s：got=%v want=%v", tt.name, action, pair[0], pair[1])
			}
		}
	}
}

// CanRevokeFindings：依據是 VIOLATION，而且措施自己建立（發動措施是它）了至少一筆尚未撤銷的認定；只引用別人的、或自己的都已撤銷、或不是 VIOLATION 都不行。
func TestCanRevokeFindings(t *testing.T) {
	id, other := uuid.NewV7(), uuid.NewV7()
	own := Violation{OriginMeasureID: id, Status: ViolationStatusEnforced}
	ownRevoked := Violation{OriginMeasureID: id, Status: ViolationStatusRevoked}
	cited := Violation{OriginMeasureID: other, Status: ViolationStatusEnforced}
	for name, tt := range map[string]struct {
		basis    Basis
		findings []Violation
		want     bool
	}{
		"自己建立的認定尚未撤銷":     {BasisViolation, []Violation{own}, true},
		"自己建立的加引用的":       {BasisViolation, []Violation{own, cited}, true},
		"引用的在前、自己建立的在後":   {BasisViolation, []Violation{cited, own}, true},
		"只引用別的措施建立的認定":    {BasisViolation, []Violation{cited}, false},
		"自己建立的已撤銷":        {BasisViolation, []Violation{ownRevoked}, false},
		"自己建立的已撤銷，引用的仍生效": {BasisViolation, []Violation{ownRevoked, cited}, false},
		"沒有載入認定":          {BasisViolation, nil, false},
		"RISK 依據":         {BasisRisk, []Violation{own}, false},
		"CONTRACT 依據":     {BasisContract, []Violation{own}, false},
	} {
		if got := (Measure{ID: id, Basis: tt.basis, Findings: tt.findings}).CanRevokeFindings(); got != tt.want {
			t.Errorf("%s：got %v want %v", name, got, tt.want)
		}
	}
}

// 員工看到的原因句與核可的字逐字相同（句子在模組文字來源的 staff 受眾），語系只有 zh-TW。
func TestActionUnavailableTextIsTheApprovedStaffSentences(t *testing.T) {
	for _, tc := range []struct {
		reason MeasureActionUnavailableReason
		action MeasureAction
		want   string
	}{
		{ActionReasonRevoked, MeasureActionRevoke, "這筆措施已撤銷"},
		{ActionReasonNotActive, MeasureActionRevoke, "這筆措施不在生效中"},
		{ActionReasonKindHasNoEndTime, MeasureActionExtend, "警告沒有結束時間"},
		{ActionReasonTerminationRecreate, MeasureActionShorten, "終止合作要改期間，請撤銷後重建"},
		{ActionReasonRequestPendingFrozen, MeasureActionUpdateStatement, "賣家提出過申覆，或有待處理的解除請求，給賣家的理由不能再改"},
		{ActionReasonRequestPendingFrozen, MeasureActionExtend, "賣家提出過申覆，或有待處理的解除請求，結束時間不能再延長"},
		{ActionReasonAppealPending, MeasureActionRevoke, "申覆待裁定，請先裁定"},
	} {
		got := ActionUnavailableText(tc.reason, tc.action)
		if got.Text != tc.want || got.Locale != uitext.ZhTW {
			t.Errorf("%s/%s = %+v，期望 %q（zh-TW）", tc.reason, tc.action, got, tc.want)
		}
	}
}
