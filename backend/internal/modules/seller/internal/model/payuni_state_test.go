package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// stateReg 組出一筆與生產路徑一致的註冊：outcome、PaymentEnabled、NeedsReonboarding 由 bind 與狀態碼推導。
func stateReg(bind PayUniBindStatus, code *string, mer, member string) *PayUniRegistration {
	r := &PayUniRegistration{BindStatus: bind, BindErrorCode: code, SellerType: KYCIndividual}
	if mer != "" || member != "" {
		r.Status = PayUniStatus{Reason: str("  審核意見\n")}
		if mer != "" {
			r.Status.MerStatus = str(mer)
		}
		if member != "" {
			r.Status.MemberStatus = str(member)
		}
	}
	r.Status.Outcome = PayUniOutcomeOf(mer, member)
	r.PaymentEnabled = bind == BindBound && r.Status.Outcome == OutcomeApproved
	r.NeedsReonboarding = TerminalMerStatus(mer) || (code != nil && *code == ErrMerchantTerminal)
	return r
}

func TestSellerPayUniOf(t *testing.T) {
	incomplete := stateReg(BindFailed, str(ErrIncompleteData), "", "")
	withMissing := stateReg(BindFailed, str(ErrIncompleteData), "", "")
	withMissing.IncompleteStoreFields = []string{StoreFieldAddress}
	for name, tc := range map[string]struct {
		app      KYCStatus
		same     bool
		reg      *PayUniRegistration
		ok       bool
		want     PayUniState
		fix      []string // 要包含的申請欄位；nil 表示必須是空的
		store    []string // 必須等於的店家欄位；nil 表示必須是空的
		reason   string
		noFields bool // 申請欄位必須是空的（含 wholeForm）
	}{
		"送審中、PayUni 拒絕的舊註冊：沒有":                        {app: KYCStatusSubmitted, same: true, reg: stateReg(BindBound, nil, "1", "8")},
		"被退回、PayUni 拒絕的舊註冊：沒有":                        {app: KYCStatusRejected, same: true, reg: stateReg(BindBound, nil, "1", "8")},
		"已取消：沒有":                                      {app: KYCStatusCancelled, same: true, reg: stateReg(BindBound, nil, "1", "8")},
		"草稿、同一份申請、PayUni 拒絕：FIXABLE（R3）":              {app: KYCStatusDraft, same: true, reg: stateReg(BindBound, nil, "1", "8"), ok: true, want: PayUniStateFixable, noFields: true, reason: "審核意見"},
		"草稿、不是同一份申請：沒有":                               {app: KYCStatusDraft, same: false, reg: stateReg(BindBound, nil, "1", "8")},
		"草稿、同一份申請、PayUni 審核中：沒有":                      {app: KYCStatusDraft, same: true, reg: stateReg(BindBound, nil, "9", "")},
		"草稿、沒有註冊：沒有":                                  {app: KYCStatusDraft, same: true},
		"草稿、同一份申請、綁定失敗指出申請欄位：FIXABLE":                 {app: KYCStatusDraft, same: true, reg: stateReg(BindFailed, str("AGENT03001"), "", ""), ok: true, want: PayUniStateFixable, fix: []string{"real_name"}},
		"核准、沒有註冊：REVIEWING":                           {app: KYCStatusApproved, same: true, ok: true, want: PayUniStateReviewing},
		"核准、綁定中：REVIEWING":                            {app: KYCStatusApproved, same: true, reg: stateReg(BindPending, nil, "", ""), ok: true, want: PayUniStateReviewing},
		"核准、已綁定沒有狀態：REVIEWING":                        {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "", ""), ok: true, want: PayUniStateReviewing},
		"核准、審核中：REVIEWING":                            {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "9", ""), ok: true, want: PayUniStateReviewing},
		"核准、補件：PENDING_INFO":                          {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "91", ""), ok: true, want: PayUniStatePendingInfo, reason: "審核意見"},
		"核准、PayUni 核准：APPROVED":                       {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "1", "1"), ok: true, want: PayUniStateApproved},
		"核准、會員層級被拒 MEMBER_8：FIXABLE、不列欄位（整份表單）":       {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "1", "8"), ok: true, want: PayUniStateFixable, noFields: true, reason: "審核意見"},
		"核准、MEMBER_99：CONTACT_SUPPORT":                {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "1", "99"), ok: true, want: PayUniStateContactSupport, reason: "審核意見"},
		"核准、MER_92：TERMINATED":                        {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "92", ""), ok: true, want: PayUniStateTerminated, reason: "審核意見"},
		"核准、MER_91 以外的終止 MER_8：TERMINATED":            {app: KYCStatusApproved, same: true, reg: stateReg(BindBound, nil, "8", ""), ok: true, want: PayUniStateTerminated, reason: "審核意見"},
		"核准、綁定失敗 AGENT03001（身分欄位，FAILED 不鎖）：FIXABLE":  {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str("AGENT03001"), "", ""), ok: true, want: PayUniStateFixable, fix: []string{"real_name", "id_card_front"}},
		"核准、綁定失敗 AGENT03005（店家欄位）：FIX_VIA_SUPPORT":    {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str("AGENT03005"), "", ""), ok: true, want: PayUniStateFixViaSupport, store: []string{StoreFieldContactName, StoreFieldContactMobile, StoreFieldSupportEmail}},
		"核准、INCOMPLETE_DATA 沒有缺的店家欄位：CONTACT_SUPPORT": {app: KYCStatusApproved, same: true, reg: incomplete, ok: true, want: PayUniStateContactSupport},
		"核准、INCOMPLETE_DATA 缺地址：FIX_VIA_SUPPORT":      {app: KYCStatusApproved, same: true, reg: withMissing, ok: true, want: PayUniStateFixViaSupport, store: []string{StoreFieldAddress}},
		"核准、PAYUNI_UNAVAILABLE：RETRY_LATER":           {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str(ErrUnavailable), "", ""), ok: true, want: PayUniStateRetryLater},
		"核准、不認得的碼：CONTACT_SUPPORT":                    {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str("AGENT99999"), "", ""), ok: true, want: PayUniStateContactSupport},
		"核准、PAYUNI_NOT_CONFIGURED：CONTACT_SUPPORT":    {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str(ErrNotConfigured), "", ""), ok: true, want: PayUniStateContactSupport},
		"核准、認領時是終止狀態：TERMINATED":                      {app: KYCStatusApproved, same: true, reg: stateReg(BindFailed, str(ErrMerchantTerminal), "2", ""), ok: true, want: PayUniStateTerminated},
	} {
		got, ok := SellerPayUniOf(tc.app, tc.same, tc.reg)
		if ok != tc.ok {
			t.Errorf("%s：ok=%v，要 %v（%+v）", name, ok, tc.ok, got)
			continue
		}
		if !ok {
			if got.State != "" || got.Reason != "" || got.FixFields != nil || got.FixStoreFields != nil {
				t.Errorf("%s：沒有值時要是零值：%+v", name, got)
			}
			continue
		}
		if got.State != tc.want || got.Reason != tc.reason {
			t.Errorf("%s：%+v，要 %s／%q", name, got, tc.want, tc.reason)
		}
		for _, f := range tc.fix {
			if !slices.Contains(got.FixFields, f) {
				t.Errorf("%s：FixFields %v 少了 %s", name, got.FixFields, f)
			}
		}
		if (tc.noFields || (tc.fix == nil && tc.want != PayUniStateFixable)) && len(got.FixFields) != 0 {
			t.Errorf("%s：FixFields 要是空的：%v", name, got.FixFields)
		}
		if !slices.Equal(got.FixStoreFields, tc.store) && (len(got.FixStoreFields) != 0 || len(tc.store) != 0) {
			t.Errorf("%s：FixStoreFields %v，要 %v", name, got.FixStoreFields, tc.store)
		}
	}
}

// 綁定失敗指出的申請欄位依賣家型態過濾：個人賣家看不到企業專屬的欄位。
func TestSellerPayUniFixFieldsFollowTheSellerType(t *testing.T) {
	reg := stateReg(BindFailed, str("AGENT04003"), "", "")
	got, _ := SellerPayUniOf(KYCStatusApproved, true, reg)
	if got.State != PayUniStateFixable || slices.Contains(got.FixFields, "business.company_name") || !slices.Contains(got.FixFields, "english_name") {
		t.Fatalf("個人：%+v", got)
	}
	reg.SellerType = KYCCompany
	got, _ = SellerPayUniOf(KYCStatusApproved, true, reg)
	if !slices.Contains(got.FixFields, "business.company_name") {
		t.Fatalf("企業：%+v", got)
	}
}

// 身分鎖：註冊已有 MerID（不是 FAILED）時，賣家改不了的身分欄位不列在可修正的欄位裡。
func TestSellerFixDropsLockedIdentityFieldsOnlyWhileLocked(t *testing.T) {
	p := &PayUniProblem{Code: "X", Remedy: RemedySellerFix, Fields: []string{"real_name", "english_name", "bank_name", "id_card_front"}}
	locked := sellerFix(&PayUniRegistration{BindStatus: BindBound}, p)
	if locked.State != PayUniStateFixable || !slices.Equal(locked.FixFields, []string{"bank_name", "id_card_front"}) {
		t.Fatalf("鎖住：%+v", locked)
	}
	unlocked := sellerFix(&PayUniRegistration{BindStatus: BindFailed}, p)
	if !slices.Equal(unlocked.FixFields, p.Fields) {
		t.Fatalf("沒鎖：%+v", unlocked)
	}
	onlyLocked := sellerFix(&PayUniRegistration{BindStatus: BindBound}, &PayUniProblem{Remedy: RemedySellerFix, Fields: []string{"real_name"}})
	if onlyLocked.State != PayUniStateContactSupport {
		t.Fatalf("可改的欄位都被鎖住時賣家什麼都改不到：%+v", onlyLocked)
	}
	if !slices.Equal(p.Fields, []string{"real_name", "english_name", "bank_name", "id_card_front"}) {
		t.Fatalf("不得改到傳入的問題：%v", p.Fields)
	}
}

// 整份表單的碼（MER_8、MEMBER_8…）給 admin 的 Fields 仍是全部欄位，賣家端不列欄位。
func TestWholeFormCodesKeepAdminFieldsAndFlagTheProblem(t *testing.T) {
	for _, code := range []string{"MER_8", "MER_91", "MER_92", "MEMBER_5", "MEMBER_8", "MEMBER_92"} {
		p := problemOf(code, nil)
		if !p.WholeForm || len(p.Fields) != len(KYCFieldNames()) || len(p.StoreFields) != len(StoreFieldNames()) {
			t.Errorf("%s：%+v", code, p)
		}
	}
	if p := problemOf("AGENT03001", nil); p.WholeForm {
		t.Errorf("AGENT03001 不是整份表單：%+v", p)
	}
}

func TestProblemReasonIsTrimmedAndEmptyIsNil(t *testing.T) {
	reg := stateReg(BindBound, nil, "1", "8")
	if p := ProblemOf(*reg); p == nil || p.Reason == nil || *p.Reason != "審核意見" {
		t.Fatalf("problem=%+v", p)
	}
	reg.Status.Reason = str(" \t\n")
	if p := ProblemOf(*reg); p == nil || p.Reason != nil {
		t.Fatalf("全是空白時沒有審核意見：%+v", p)
	}
}

// 不變式：同一份資料集（申請狀態 × 同一份申請與否 × 綁定狀態 × PayUni 狀態 × 碼）上，各個推導互相一致。
func TestSellerPayUniInvariants(t *testing.T) {
	codes := []*string{nil, str(ErrIncompleteData), str(ErrNotConfigured), str(ErrUnavailable), str(ErrMerchantTerminal), str("AGENT03001"), str("AGENT03005"), str("AGENT04003"), str("AGENT04008"), str("AGENT99999")}
	statuses := [][2]string{{"", ""}, {"9", ""}, {"1", "1"}, {"1", "5"}, {"91", ""}, {"1", "8"}, {"1", "99"}, {"1", "92"}, {"92", ""}, {"2", ""}, {"8", ""}, {"99", ""}, {"7", "1"}}
	apps := []KYCStatus{KYCStatusDraft, KYCStatusSubmitted, KYCStatusRejected, KYCStatusApproved, KYCStatusCancelled}
	n := 0
	for _, app := range apps {
		for _, same := range []bool{true, false} {
			for _, bind := range PayUniBindStatusValues() {
				for _, st := range statuses {
					for _, code := range codes {
						if (bind == BindFailed) != (code != nil) {
							continue
						}
						for _, reg := range []*PayUniRegistration{stateReg(bind, code, st[0], st[1]), nil} {
							n++
							checkInvariants(t, app, same, reg)
						}
					}
				}
			}
		}
	}
	if n < 1000 {
		t.Fatalf("資料集只有 %d 格", n)
	}
}

func checkInvariants(t *testing.T, app KYCStatus, same bool, reg *PayUniRegistration) {
	t.Helper()
	desc := func() string {
		return string(app) + "/" + map[bool]string{true: "same", false: "other"}[same] + "/" + strings.TrimSpace(strings.Join([]string{regDesc(reg)}, ""))
	}
	state, ok := SellerPayUniOf(app, same, reg)
	if app == KYCStatusApproved && !ok {
		t.Errorf("%s：已核准的申請一定有值", desc())
	}
	flow := DeriveKYCFlow(app, reg)
	var payuni *SellerPayUni
	if ok {
		payuni = &state
		if (state.State == PayUniStateTerminated) != (reg != nil && reg.NeedsReonboarding) {
			t.Errorf("%s：TERMINATED ⇔ NeedsReonboarding 不成立：%+v", desc(), state)
		}
		if (state.State == PayUniStateApproved) != (reg != nil && reg.PaymentEnabled) {
			t.Errorf("%s：APPROVED ⇔ PaymentEnabled 不成立：%+v", desc(), state)
		}
		if app == KYCStatusDraft && state.State != PayUniStateFixable {
			t.Errorf("%s：草稿只有 FIXABLE 才有值：%+v", desc(), state)
		}
		if state.State == PayUniStateFixViaSupport && (len(state.FixFields) != 0 || len(state.FixStoreFields) == 0) {
			t.Errorf("%s：FIX_VIA_SUPPORT ⇒ 只有店家欄位：%+v", desc(), state)
		}
		if state.State != PayUniStateFixable && len(state.FixFields) != 0 {
			t.Errorf("%s：只有 FIXABLE 列申請欄位：%+v", desc(), state)
		}
		if state.State != PayUniStateFixable && state.State != PayUniStateFixViaSupport && len(state.FixStoreFields) != 0 {
			t.Errorf("%s：只有 FIXABLE、FIX_VIA_SUPPORT 列店家欄位：%+v", desc(), state)
		}
		if reg != nil && IdentityLockedBy(*reg) {
			for _, f := range LockedIdentityFields() {
				if slices.Contains(state.FixFields, f) {
					t.Errorf("%s：身分鎖住時不該列 %s", desc(), f)
				}
			}
		}
	}
	wantResubmit := app == KYCStatusDraft || app == KYCStatusRejected || app == KYCStatusCancelled || (app == KYCStatusApproved && ok && state.State == PayUniStateFixable)
	if flow.CanResubmit != wantResubmit {
		t.Errorf("%s：CanResubmit=%v，要 %v", desc(), flow.CanResubmit, wantResubmit)
	}
	reasons := ResubmitUnavailableReasons(app, flow, payuni)
	// 已取消的申請不是目前的申請：流程是 NOT_STARTED、能重送（SaveKYCDraft 版本 0 會建立新的）、沒有原因，與「原因為空 ⇔ CanResubmit」一致。
	if (len(reasons) == 0) != flow.CanResubmit {
		t.Errorf("%s：原因 %v 與 CanResubmit=%v 不一致", desc(), reasons, flow.CanResubmit)
	}
	if app == KYCStatusCancelled && (!flow.CanResubmit || flow.Status != FlowNotStarted) {
		t.Errorf("%s：已取消的流程要是 NOT_STARTED、能重送：%+v", desc(), flow)
	}
}

func regDesc(r *PayUniRegistration) string {
	if r == nil {
		return "nil"
	}
	return string(r.BindStatus) + "/" + Deref(r.BindErrorCode) + "/" + Deref(r.Status.MerStatus) + "/" + Deref(r.Status.MemberStatus)
}

// PayUniPhaseOf 的階段：期望值寫死，抄自抽出之前 DeriveKYCFlow（a6f7b3815）在已核准、有註冊時的 switch：綁定狀態優先（PENDING 一律綁定中、
// FAILED 一律綁定失敗，與 outcome 無關），已綁定才看 outcome。同一張表也是 DeriveKYCFlow 的輸出（已核准的申請）。
func TestPayUniPhaseOf(t *testing.T) {
	want := map[PayUniBindStatus]map[PayUniOutcome]KYCFlowStatus{
		BindPending: {
			OutcomeNone: FlowBinding, OutcomeUnderReview: FlowBinding, OutcomePendingInfo: FlowBinding,
			OutcomeApproved: FlowBinding, OutcomeRejected: FlowBinding, OutcomeUnrecognized: FlowBinding,
		},
		BindFailed: {
			OutcomeNone: FlowBindFailed, OutcomeUnderReview: FlowBindFailed, OutcomePendingInfo: FlowBindFailed,
			OutcomeApproved: FlowBindFailed, OutcomeRejected: FlowBindFailed, OutcomeUnrecognized: FlowBindFailed,
		},
		BindBound: {
			OutcomeNone: FlowForwarded, OutcomeUnderReview: FlowUnderReview, OutcomePendingInfo: FlowPendingInfo,
			OutcomeApproved: FlowApproved, OutcomeRejected: FlowPayUniRejected, OutcomeUnrecognized: FlowForwarded,
		},
	}
	if len(want) != len(PayUniBindStatusValues()) {
		t.Fatalf("表要涵蓋全部綁定狀態：%d／%d", len(want), len(PayUniBindStatusValues()))
	}
	for bind, row := range want {
		if len(row) != len(PayUniOutcomeValues()) {
			t.Fatalf("%s：表要涵蓋全部 outcome：%d／%d", bind, len(row), len(PayUniOutcomeValues()))
		}
		for outcome, status := range row {
			reg := &PayUniRegistration{BindStatus: bind, Status: PayUniStatus{Outcome: outcome}}
			if got := PayUniPhaseOf(*reg); got != status {
				t.Errorf("%s/%s：PayUniPhaseOf %s，要 %s", bind, outcome, got, status)
			}
			if got := DeriveKYCFlow(KYCStatusApproved, reg).Status; got != status {
				t.Errorf("%s/%s：DeriveKYCFlow %s，要 %s", bind, outcome, got, status)
			}
		}
	}
}

// 流程的 CanResubmit 與賣家看到的 PayUni 狀態吃同一份註冊（賣家型態與店家缺的欄位都帶上）：註冊的問題指向的欄位依賣家型態過濾，
// 過濾後沒有欄位時 PayUni 狀態不是 FIXABLE，流程就不能重送。目錄現在每一組都含不分型態的欄位，碰不到這個差異，所以測試放進一個
// 只有企業欄位的碼；DeriveKYCFlow 若讀一份沒有賣家型態的註冊，個人賣家會是 can_resubmit 真而 payuni 不是 FIXABLE。
func TestFlowCanResubmitAndSellerPayUniReadTheSameRegistration(t *testing.T) {
	const code = "TEST_BUSINESS_ONLY"
	payuniErrorIndex[code] = payuniErrorGroup{remedy: RemedySellerFix, fields: []string{"business.company_name"}, codes: []string{code}}
	t.Cleanup(func() { delete(payuniErrorIndex, code) })
	for _, tc := range []struct {
		sellerType KYCSellerType
		want       PayUniState
	}{{KYCIndividual, PayUniStateContactSupport}, {KYCCompany, PayUniStateFixable}} {
		reg := stateReg(BindFailed, str(code), "", "")
		reg.SellerType = tc.sellerType
		state, ok := SellerPayUniOf(KYCStatusApproved, true, reg)
		flow := DeriveKYCFlow(KYCStatusApproved, reg)
		if !ok || state.State != tc.want {
			t.Fatalf("%s：%+v ok=%v，要 %s", tc.sellerType, state, ok, tc.want)
		}
		if flow.CanResubmit != (state.State == PayUniStateFixable) {
			t.Errorf("%s：CanResubmit=%v，PayUni 狀態 %s", tc.sellerType, flow.CanResubmit, state.State)
		}
		if reasons := ResubmitUnavailableReasons(KYCStatusApproved, flow, &state); (len(reasons) == 0) != flow.CanResubmit {
			t.Errorf("%s：原因 %v 與 CanResubmit=%v 不一致", tc.sellerType, reasons, flow.CanResubmit)
		}
	}
}

func TestPayUniStateExplanation(t *testing.T) {
	for _, s := range PayUniStateValues() {
		m, ok := s.Explanation(uitext.ZhTW)
		if s == PayUniStateApproved {
			if ok {
				t.Errorf("APPROVED 沒有說明：%+v", m)
			}
			continue
		}
		if !ok || strings.TrimSpace(m.Text) == "" || m.Locale != uitext.ZhTW {
			t.Errorf("%s：%+v ok=%v", s, m, ok)
		}
	}
	if _, ok := PayUniState("").Explanation(uitext.ZhTW); ok {
		t.Error("空字串不是有效的狀態")
	}
}

func TestResubmitUnavailableText(t *testing.T) {
	for _, r := range KYCResubmitUnavailableReasonValues() {
		m := r.Text(uitext.ZhTW)
		if !strings.HasSuffix(m.Text, "。") || m.Locale != uitext.ZhTW {
			t.Errorf("%s：%+v", r, m)
		}
		// 訊息是第一個原因的句子，不含後面的。
		other := ResubmitPayUniApproved
		if r == other {
			other = ResubmitUnderReview
		}
		err := ErrResubmitUnavailable([]KYCResubmitUnavailableReason{r, other}, uitext.ZhTW)
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), m.Text) || strings.Contains(err.Error(), other.Text(uitext.ZhTW).Text) {
			t.Errorf("%s：err=%v", r, err)
		}
	}
	for name, reasons := range map[string][]KYCResubmitUnavailableReason{"空": nil, "不認得": {"X"}} {
		if err := ErrResubmitUnavailable(reasons, uitext.ZhTW); err == nil || apperr.KindOf(err) == apperr.KindFailedPrecondition {
			t.Errorf("%s：到不了的分支回 internal：%v", name, err)
		}
	}
	if (KYCResubmitUnavailableReason("")).Text(uitext.ZhTW).Text != "" {
		t.Error("不認得的原因沒有句子")
	}
}

// 每個不能重送的 PayUni 狀態都有對應的原因；FIXABLE 能重送、不在表裡。
func TestEveryNonFixablePayUniStateHasAResubmitReason(t *testing.T) {
	for _, s := range PayUniStateValues() {
		flow := KYCFlow{Status: FlowApproved, CanResubmit: s == PayUniStateFixable}
		got := ResubmitUnavailableReasons(KYCStatusApproved, flow, &SellerPayUni{State: s})
		if (s == PayUniStateFixable) != (len(got) == 0) || (s != PayUniStateFixable && len(got) != 1) {
			t.Errorf("%s：%v", s, got)
		}
	}
	for locked, want := range map[bool]KYCResubmitUnavailableReason{true: ResubmitUnderReview, false: ResubmitUnderReviewCancelable} {
		got := ResubmitUnavailableReasons(KYCStatusSubmitted, KYCFlow{Status: FlowSubmitted, IdentityLocked: locked}, nil)
		if !slices.Equal(got, []KYCResubmitUnavailableReason{want}) {
			t.Errorf("送審中、鎖住=%v：%v", locked, got)
		}
	}
	// 已取消的申請不是目前的申請（讀取回 not_found，可以重新開始）：沒有原因，流程能重送。
	if got := ResubmitUnavailableReasons(KYCStatusCancelled, KYCFlow{Status: FlowNotStarted}, nil); len(got) != 0 {
		t.Errorf("已取消：%v", got)
	}
}
