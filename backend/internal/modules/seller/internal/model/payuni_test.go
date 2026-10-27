package model

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// golden 是舊 seller-service 的錯誤目錄（136 筆）逐碼的處置分類，原樣搬來：分類被改動的碼會在這裡失敗，覆蓋率檢查
// 看不出「搬錯組」。我們自己的碼（INCOMPLETE_DATA 等）不在 golden 裡，另外檢查。MerTel 的三個碼（AGENT04008 至 AGENT04010）
// 與舊目錄不同：商店電話是店家的聯絡電話，賣家可以自己修正（第三欄 storeMerTel），舊目錄當成平台沒送的欄位。
func TestPayUniCatalogMatchesTheLegacyGolden(t *testing.T) {
	// golden 第三欄是舊目錄的補件步驟；它們在新目錄對應到申請欄位的群組。
	type legacyGroup struct{ fields, storeFields []string }
	legacyFields := map[string]legacyGroup{
		"": {}, "idCard": {fields: identityFields}, "companyInfo": {fields: businessFields}, "otherInfo": {storeFields: contactStoreFields},
		"start": {fields: allFields, storeFields: allStoreFields}, "storeMerTel": {storeFields: merTelStoreFields},
	}
	f, err := os.Open("testdata/payuni_error_catalog.golden")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	golden := map[string]bool{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		cols := strings.Split(s.Text(), "\t")
		if len(cols) != 3 {
			t.Fatalf("golden 行格式不合：%q", s.Text())
		}
		remedy, fields, storeFields, known := ClassifyPayUniCode(cols[0])
		want, ok := legacyFields[cols[2]]
		if !ok || !known || string(remedy) != cols[1] || !slices.Equal(fields, want.fields) || !slices.Equal(storeFields, want.storeFields) {
			t.Errorf("%s：golden 是 %s／%s，目錄是 %s／%v／%v（known %v）", cols[0], cols[1], cols[2], remedy, fields, storeFields, known)
		}
		golden[cols[0]] = true
	}
	if len(golden) != 136 {
		t.Fatalf("golden 有 %d 筆", len(golden))
	}
	own := []string{ErrIncompleteData, ErrNotConfigured, ErrUnavailable, ErrMerchantTerminal}
	for _, code := range PayUniErrorCodes() {
		if !golden[code] && !slices.Contains(own, code) {
			t.Errorf("%s 不在舊目錄也不是我們自己的碼", code)
		}
	}
	for _, code := range own {
		if _, _, _, known := ClassifyPayUniCode(code); !known {
			t.Errorf("%s 沒有分類", code)
		}
	}
}

func TestUnknownPayUniCodeFailsClosed(t *testing.T) {
	remedy, fields, storeFields, known := ClassifyPayUniCode("AGENT99999")
	if known || remedy != RemedyContactSupport || fields != nil || storeFields != nil {
		t.Fatalf("got %s %v %v %v", remedy, fields, storeFields, known)
	}
	if PayUniRetryable("AGENT99999") || !PayUniRetryable("AGENT03021") || PayUniRetryable("AGENT04017") || !PayUniAlreadyBound("AGENT04017") {
		t.Fatal("retryable / already-bound 分類不對")
	}
}

// outcome 的推導照舊 derivePayuniOutcome：終止狀態與 91 不看 MemberStatus；1、9 看 MemberStatus；認不得的 fail closed。
func TestPayUniOutcomeOf(t *testing.T) {
	for _, tc := range []struct {
		mer, member string
		want        PayUniOutcome
	}{
		{"", "", OutcomeNone},
		{"1", "1", OutcomeApproved},
		{"1", "5", OutcomePendingInfo},
		{"1", "9", OutcomeUnderReview},
		{"1", "", OutcomeUnderReview},
		{"9", "1", OutcomeUnderReview},
		{"9", "5", OutcomePendingInfo},
		{"9", "", OutcomeUnderReview},
		{"1", "8", OutcomeRejected},
		{"9", "92", OutcomeRejected},
		{"1", "99", OutcomeRejected},
		{"2", "1", OutcomeRejected},
		{"8", "", OutcomeRejected},
		{"92", "1", OutcomeRejected},
		{"99", "5", OutcomeRejected},
		{"91", "1", OutcomePendingInfo},
		{"", "5", OutcomePendingInfo},
		{"7", "5", OutcomePendingInfo},
		{"1", "7", OutcomeUnderReview},
		{"7", "1", OutcomeUnrecognized},
		{"S", "", OutcomeUnrecognized},
	} {
		if got := PayUniOutcomeOf(tc.mer, tc.member); got != tc.want {
			t.Errorf("PayUniOutcomeOf(%q, %q) = %s，預期 %s", tc.mer, tc.member, got, tc.want)
		}
	}
	if !TerminalMerStatus("2") || !TerminalMerStatus("99") || TerminalMerStatus("91") || TerminalMerStatus("1") {
		t.Fatal("終止狀態不對")
	}
}

func TestPayUniStatusCodeIsInTheCatalog(t *testing.T) {
	for _, mer := range []string{"", "1", "2", "8", "9", "91", "92", "99"} {
		for _, member := range []string{"", "1", "5", "8", "9", "92", "99"} {
			code := PayUniStatusCode(mer, member)
			if code == "" {
				if mer != "" {
					t.Errorf("(%q, %q) 沒有碼", mer, member)
				}
				continue
			}
			if _, _, _, known := ClassifyPayUniCode(code); !known {
				t.Errorf("(%q, %q) 的碼 %s 不在目錄", mer, member, code)
			}
		}
	}
}

func TestMerIDAndExecuteTime(t *testing.T) {
	if got := FormatMerID(2600000001); got != "HAWK2600000001" {
		t.Fatal(got)
	}
	if code, ok := ParseMerID("HAWK2600000001"); !ok || code != 2600000001 {
		t.Fatalf("%d %v", code, ok)
	}
	for _, bad := range []string{"", "HAWK", "daly1", "HAWK12345678901", "HAWKx", "XHAWK1", "HAWK1 "} {
		if _, ok := ParseMerID(bad); ok {
			t.Errorf("ParseMerID(%q) 應該被拒絕", bad)
		}
	}
	// ExecuteTime 是台北時間：中午 12 點是 UTC 的 04:00。
	if got, ok := ParseExecuteTime("20261002120000"); !ok || !got.Equal(time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("%v %v", got, ok)
	}
	for _, bad := range []string{"", "2026-10-02 12:00:00", "20261302120000", "2026100212"} {
		if _, ok := ParseExecuteTime(bad); ok {
			t.Errorf("ParseExecuteTime(%q) 應該被拒絕", bad)
		}
	}
}

func str(s string) *string { return &s }

func TestDeriveKYCFlow(t *testing.T) {
	reg := func(bind PayUniBindStatus, code *string, mer, member string) *PayUniRegistration {
		r := &PayUniRegistration{BindStatus: bind, BindErrorCode: code}
		if mer != "" || member != "" {
			r.Status = PayUniStatus{MerStatus: str(mer), MemberStatus: str(member), Reason: str("審核意見")}
			if mer == "" {
				r.Status.MerStatus = nil
			}
			if member == "" {
				r.Status.MemberStatus = nil
			}
		}
		r.Status.Outcome = PayUniOutcomeOf(mer, member)
		r.PaymentEnabled = bind == BindBound && r.Status.Outcome == OutcomeApproved
		r.NeedsReonboarding = TerminalMerStatus(mer) || (code != nil && *code == ErrMerchantTerminal)
		return r
	}
	for name, tc := range map[string]struct {
		status                       KYCStatus
		reg                          *PayUniRegistration
		want                         KYCFlowStatus
		resubmit, reonboard, payable bool
		problem                      string
	}{
		"草稿":        {KYCStatusDraft, nil, FlowDraft, true, false, false, ""},
		"審核中":       {KYCStatusSubmitted, nil, FlowSubmitted, false, false, false, ""},
		"被退回":       {KYCStatusRejected, nil, FlowRejected, true, false, false, ""},
		"取消":        {KYCStatusCancelled, nil, FlowNotStarted, true, false, false, ""},
		"核准、綁定中":    {KYCStatusApproved, reg(BindPending, nil, "", ""), FlowBinding, false, false, false, ""},
		"核准、還沒有註冊列": {KYCStatusApproved, nil, FlowBinding, false, false, false, ""},
		"核准、綁定失敗（指出申請欄位，能重送）": {KYCStatusApproved, reg(BindFailed, str("AGENT04003"), "", ""), FlowBindFailed, true, false, false, "AGENT04003"},
		"核准、綁定失敗（只指出店家欄位）":    {KYCStatusApproved, reg(BindFailed, str("AGENT03005"), "", ""), FlowBindFailed, false, false, false, "AGENT03005"},
		"核准、綁定失敗（暫時性，等平台）":    {KYCStatusApproved, reg(BindFailed, str(ErrUnavailable), "", ""), FlowBindFailed, false, false, false, ErrUnavailable},
		"核准、綁定、還沒有狀態":         {KYCStatusApproved, reg(BindBound, nil, "", ""), FlowForwarded, false, false, false, ""},
		"核准、審核中":              {KYCStatusApproved, reg(BindBound, nil, "9", ""), FlowUnderReview, false, false, false, ""},
		"核准、待補件":              {KYCStatusApproved, reg(BindBound, nil, "1", "5"), FlowPendingInfo, false, false, false, "MEMBER_5"},
		"核准、PayUni 核准":        {KYCStatusApproved, reg(BindBound, nil, "1", "1"), FlowApproved, false, false, true, ""},
		"核准、會員層級被拒（能重送）":      {KYCStatusApproved, reg(BindBound, nil, "1", "8"), FlowPayUniRejected, true, false, false, "MEMBER_8"},
		"核准、會員被停權（聯絡客服，不能重送）": {KYCStatusApproved, reg(BindBound, nil, "1", "99"), FlowPayUniRejected, false, false, false, "MEMBER_99"},
		"核准、商家層級終止（要重新入駐）":    {KYCStatusApproved, reg(BindBound, nil, "92", ""), FlowPayUniRejected, false, true, false, "MER_92"},
		"核准、認領時是終止狀態":         {KYCStatusApproved, reg(BindFailed, str(ErrMerchantTerminal), "2", ""), FlowBindFailed, false, true, false, ErrMerchantTerminal},
		"核准、不認得的狀態當成已送交":      {KYCStatusApproved, reg(BindBound, nil, "7", "1"), FlowForwarded, false, false, false, ""},
	} {
		got := DeriveKYCFlow(tc.status, tc.reg)
		if got.Status != tc.want || got.CanResubmit != tc.resubmit {
			t.Errorf("%s：%+v", name, got)
		}
		// PayUni 的事實不在流程上：終止、可收款與問題由註冊本身回答，賣家看到的狀態是 SellerPayUniOf。
		if tc.reg == nil {
			continue
		}
		problem := ""
		if p := ProblemOf(*tc.reg); p != nil {
			problem = p.Code
		}
		if tc.reg.NeedsReonboarding != tc.reonboard || tc.reg.PaymentEnabled != tc.payable || problem != tc.problem {
			t.Errorf("%s：註冊的 needsReonboarding=%v paymentEnabled=%v problem=%q", name, tc.reg.NeedsReonboarding, tc.reg.PaymentEnabled, problem)
		}
		if tc.status != KYCStatusApproved {
			continue
		}
		state, ok := SellerPayUniOf(tc.status, true, tc.reg)
		if !ok || (state.State == PayUniStateTerminated) != tc.reonboard || (state.State == PayUniStateApproved) != tc.payable || (state.State == PayUniStateFixable) != tc.resubmit {
			t.Errorf("%s：SellerPayUniOf %+v ok=%v", name, state, ok)
		}
	}
}

// 身分鎖：註冊有 MerID 而且 PayUni 收下（或正要收下）之後，每個流程狀態都帶 IdentityLocked；綁定失敗與沒有註冊列不鎖。
func TestDeriveKYCFlowIdentityLocked(t *testing.T) {
	for name, tc := range map[string]struct {
		status KYCStatus
		reg    *PayUniRegistration
		want   bool
	}{
		"沒有註冊列的草稿":      {KYCStatusDraft, nil, false},
		"綁定失敗後的草稿":      {KYCStatusDraft, &PayUniRegistration{BindStatus: BindFailed}, false},
		"已綁定、改回草稿":      {KYCStatusDraft, &PayUniRegistration{BindStatus: BindBound}, true},
		"已綁定、被退回":       {KYCStatusRejected, &PayUniRegistration{BindStatus: BindBound}, true},
		"綁定中":           {KYCStatusApproved, &PayUniRegistration{BindStatus: BindPending}, true},
		"核准、綁定失敗":       {KYCStatusApproved, &PayUniRegistration{BindStatus: BindFailed}, false},
		"已綁定、PayUni 拒絕": {KYCStatusApproved, &PayUniRegistration{BindStatus: BindBound, Status: PayUniStatus{Outcome: OutcomeRejected}}, true},
	} {
		if got := DeriveKYCFlow(tc.status, tc.reg).IdentityLocked; got != tc.want {
			t.Errorf("%s：IdentityLocked=%v，要 %v", name, got, tc.want)
		}
	}
}

// 賣家可修的錯誤指出的欄位都是申請上的欄位（KYCFieldNames），不會漏掉或寫錯名稱。
func TestPayUniProblemFieldsAreKYCFields(t *testing.T) {
	names := KYCFieldNames()
	for _, code := range PayUniErrorCodes() {
		remedy, fields, storeFields, _ := ClassifyPayUniCode(code)
		if remedy != RemedySellerFix && (len(fields) > 0 || len(storeFields) > 0) {
			t.Errorf("%s 不是賣家可修的錯誤卻有欄位 %v／%v", code, fields, storeFields)
		}
		for _, f := range fields {
			if !slices.Contains(names, f) {
				t.Errorf("%s 的欄位 %q 不是 KYCFieldNames", code, f)
			}
		}
		for _, f := range storeFields {
			if !slices.Contains(StoreFieldNames(), f) {
				t.Errorf("%s 的店家欄位 %q 不是 StoreFieldNames", code, f)
			}
		}
	}
}

// 賣家可修的問題要補的欄位依申請的賣家型態過濾：個人賣家沒有企業專屬的欄位，企業賣家沒有個人專屬的欄位。
func TestProblemFieldsAreFilteredBySellerType(t *testing.T) {
	code := "AGENT03001" // 身分與姓名：兩種型態各有自己的身分證欄位
	reg := PayUniRegistration{BindStatus: BindFailed, BindErrorCode: &code}
	reg.SellerType = KYCIndividual
	individual := ProblemOf(reg)
	reg.SellerType = KYCCompany
	business := ProblemOf(reg)
	if individual == nil || business == nil || individual.Remedy != RemedySellerFix {
		t.Fatalf("individual=%+v business=%+v", individual, business)
	}
	if !slices.Contains(individual.Fields, "individual.id_card_no") || slices.Contains(individual.Fields, "business.rep_id_card_no") {
		t.Errorf("個人賣家：%v", individual.Fields)
	}
	if slices.Contains(business.Fields, "individual.id_card_no") || !slices.Contains(business.Fields, "business.rep_id_card_no") || !slices.Contains(business.Fields, "real_name") {
		t.Errorf("企業賣家：%v", business.Fields)
	}
	reg.SellerType = ""
	if got := ProblemOf(reg); !slices.Contains(got.Fields, "individual.id_card_no") || !slices.Contains(got.Fields, "business.rep_id_card_no") {
		t.Errorf("沒有型態時不過濾：%v", got.Fields)
	}
}

func TestFieldAppliesTo(t *testing.T) {
	for _, name := range KYCFieldNames() {
		switch {
		case strings.HasPrefix(name, "individual."):
			if !FieldAppliesTo(name, KYCIndividual) || FieldAppliesTo(name, KYCCompany) {
				t.Errorf("%s", name)
			}
		case strings.HasPrefix(name, "business."):
			if FieldAppliesTo(name, KYCIndividual) || !FieldAppliesTo(name, KYCCompany) {
				t.Errorf("%s", name)
			}
		default:
			if !FieldAppliesTo(name, KYCIndividual) || !FieldAppliesTo(name, KYCCompany) {
				t.Errorf("%s 是兩種型態共用的欄位", name)
			}
		}
	}
}

func TestStoreFieldProblems(t *testing.T) {
	code := ErrIncompleteData
	// INCOMPLETE_DATA 只指向店家現在實際缺的欄位。
	incomplete := ProblemOf(PayUniRegistration{BindStatus: BindFailed, BindErrorCode: &code, IncompleteStoreFields: []string{StoreFieldAddress, StoreFieldSupportEmail}})
	if incomplete == nil || incomplete.Remedy != RemedySellerFix || incomplete.Code != ErrIncompleteData ||
		!slices.Equal(incomplete.StoreFields, []string{StoreFieldAddress, StoreFieldSupportEmail}) {
		t.Errorf("INCOMPLETE_DATA：%+v", incomplete)
	}
	// 缺的不是店家欄位（例如沒有網站根網址的設定）：保留碼，不指向任何欄位。
	none := ProblemOf(PayUniRegistration{BindStatus: BindFailed, BindErrorCode: &code})
	if none == nil || none.Code != ErrIncompleteData || len(none.StoreFields) != 0 {
		t.Errorf("INCOMPLETE_DATA 沒有缺的店家欄位：%+v", none)
	}
	contact := ProblemOf(PayUniRegistration{BindStatus: BindFailed, BindErrorCode: ptrTo("AGENT06006")})
	if contact == nil || len(contact.Fields) != 0 || !slices.Equal(contact.StoreFields, []string{StoreFieldContactName, StoreFieldContactMobile, StoreFieldSupportEmail}) {
		t.Errorf("聯絡類：%+v", contact)
	}
	merTel := ProblemOf(PayUniRegistration{BindStatus: BindFailed, BindErrorCode: ptrTo("AGENT04008")})
	if merTel == nil || merTel.Remedy != RemedySellerFix || !slices.Equal(merTel.StoreFields, []string{StoreFieldContactPhone}) {
		t.Errorf("MerTel：%+v", merTel)
	}
}

func TestNormalizeContactFields(t *testing.T) {
	ok := Profile{StoreName: "店", ContactName: ptrTo(" 王小明 "), ContactMobile: ptrTo("0912-345-678"), SupportEmail: ptrTo(" A@Example.COM ")}
	if err := Normalize(&ok); err != nil || *ok.ContactName != "王小明" || *ok.ContactMobile != "0912345678" || *ok.SupportEmail != "a@example.com" {
		t.Fatalf("err=%v %+v", err, ok)
	}
	for name, edit := range map[string]func(*Profile){
		"姓名超過 20 字": func(p *Profile) { p.ContactName = ptrTo(strings.Repeat("字", ContactNameMaxRunes+1)) },
		"手機是市話":     func(p *Profile) { p.ContactMobile = ptrTo("0229010572") },
		"信箱沒有網域":    func(p *Profile) { p.SupportEmail = ptrTo("user@localhost") },
		"信箱超過 50 字": func(p *Profile) {
			p.SupportEmail = ptrTo(strings.Repeat("a", SupportEmailMaxRunes-len("@example.com")+1) + "@example.com")
		},
	} {
		p := Profile{StoreName: "店"}
		edit(&p)
		if err := Normalize(&p); err == nil {
			t.Errorf("%s 應該被拒絕", name)
		}
	}
	boundary := Profile{StoreName: "店", ContactName: ptrTo(strings.Repeat("字", ContactNameMaxRunes)),
		SupportEmail: ptrTo(strings.Repeat("a", SupportEmailMaxRunes-len("@example.com")) + "@example.com")}
	if err := Normalize(&boundary); err != nil {
		t.Errorf("上限剛好應該通過：%v", err)
	}
}
