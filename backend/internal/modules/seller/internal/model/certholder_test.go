package model

import "testing"

// TestNormalize_FoldsOnlyIdentityPreservingNoise pins each individual folding rule of
// the normalizer, and pins that the rules it must NOT have (substring/fuzzy matching)
// stay absent.
func TestNormalize_FoldsOnlyIdentityPreservingNoise(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"空字串", "", ""},
		{"半形空白", " 範例 生鮮 ", "範例生鮮"},
		{"全形空白 U+3000", "　範例　生鮮　", "範例生鮮"},
		{"tab 與換行", "範例\t生鮮\n", "範例生鮮"},
		{"NBSP", "範例 生鮮", "範例生鮮"},
		{"全形英數轉半形", "ＡＢＣ１２３", "abc123"},
		{"全形括號轉半形", "範例（生鮮）", "範例(生鮮)"},
		{"英文轉小寫", "AcmeRoad", "acmeroad"},
		{"股份有限公司後綴", "範例生鮮股份有限公司", "範例生鮮"},
		{"有限公司後綴", "範例生鮮有限公司", "範例生鮮"},
		{"企業社後綴", "範例生鮮企業社", "範例生鮮"},
		{"商行後綴", "範例生鮮商行", "範例生鮮"},
		{"農場後綴", "範例生鮮農場", "範例生鮮"},
		{"工作室後綴", "範例生鮮工作室", "範例生鮮"},
		{"合作社後綴", "範例生鮮合作社", "範例生鮮"},
		{"產銷班後綴", "範例生鮮產銷班", "範例生鮮"},
		{"Co., Ltd. 後綴", "Acme Road Co., Ltd.", "acmeroad"},
		{"Co.,Ltd 後綴（無尾點）", "Acme Road Co.,Ltd", "acmeroad"},
		{"Ltd. 後綴", "Acme Road Ltd.", "acmeroad"},
		{"Inc. 後綴", "Acme Road Inc.", "acmeroad"},
		{"LLC 後綴", "Acme Road LLC", "acmeroad"},
		{"Corp. 後綴", "Acme Road Corp.", "acmeroad"},
		{"尾端逗號視為雜訊", "範例生鮮有限公司，", "範例生鮮"},
		// 只有公司型態 ⇒ 正規化後為空，代表「這個名稱不帶身分資訊」
		{"只有有限公司", "有限公司", ""},
		{"只有股份有限公司", "股份有限公司", ""},
		{"只有 LLC", "LLC", ""},
		// 以下釘住「不可以做的事」——這些若被折疊掉就是把冒用放行的破口
		{"不去除公司型態以外的詞", "範例生鮮嚴選", "範例生鮮嚴選"},
		{"不去除中間的公司型態字樣", "有限公司範例生鮮", "有限公司範例生鮮"},
		{"英文一般字不因結尾像後綴而被截", "Rock Corporate", "rockcorporate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeHolderName(c.in); got != c.want {
				t.Errorf("NormalizeHolderName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestNormalize_NeverLoops guards the strip loop's termination on inputs made
// entirely of stackable company forms.
func TestNormalize_NeverLoops(t *testing.T) {
	if got := NormalizeHolderName("農場農場有限公司"); got != "" {
		t.Errorf("NormalizeHolderName(疊加公司型態) = %q, want 空字串", got)
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		name             string
		holder           string
		companyName      string
		wantMatched      bool
		wantField        CertHolderField
		wantHasReference bool
	}{
		{
			name: "完全相同", holder: "範例生鮮嚴選", companyName: "範例生鮮嚴選",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差半形空白", holder: "範例 生鮮 嚴選", companyName: "範例生鮮嚴選",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差全形空白", holder: "範例　生鮮嚴選", companyName: "範例生鮮嚴選",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差全半形英數", holder: "ＡｃｍｅＲｏａｄ２", companyName: "AcmeRoad2",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差大小寫", holder: "ACME ROAD", companyName: "acme road",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差公司後綴（中文）", holder: "範例生鮮有限公司", companyName: "範例生鮮",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name: "只差公司後綴（英文，方向相反）", holder: "Acme Road", companyName: "Acme Road Co., Ltd.",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			// 2026-08-27 實測：真實存在的有機編號 1-009-300022 持有人是 Acme Road LLC，
			// 被填到別的賣家的文旦禮盒上而自動核准。這一列就是那個缺口。
			name:   "真實案例 1-009-300022：他人證書不得相符",
			holder: "Acme Road LLC", companyName: "範例生鮮優選有限公司",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
		{
			name:   "真實案例反向：持有人確為本賣家時仍要相符",
			holder: "Acme Road LLC", companyName: "acme road, llc.",
			wantMatched: true, wantField: CertHolderCompanyName, wantHasReference: true,
		},
		{
			name:   "公司名稱為空（個人賣家）⇒ 無可比對基準",
			holder: "Acme Road LLC", companyName: "",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: false,
		},
		{
			name:   "名稱只有空白 ⇒ 無可比對基準",
			holder: "Acme Road LLC", companyName: "  　",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: false,
		},
		{
			// company_name 只填了公司型態 ⇒ 正規化後為空，不得與任何東西相符，
			// 也不算 reference（否則「無從判斷」會被誤報成「有比過、不符」）。
			name:   "company_name 只有「有限公司」⇒ 不相符且不算 reference",
			holder: "有限公司", companyName: "有限公司",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: false,
		},
		{
			// 政府回空字串時不得因為「雙方都空」而判成相符。
			name:   "holder 為空 ⇒ 不相符",
			holder: "", companyName: "範例生鮮",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
		{
			name:   "holder 正規化後為空 ⇒ 不相符",
			holder: "股份有限公司", companyName: "範例生鮮",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
		{
			// 明令禁止子字串包含比對。
			name:   "子字串不得相符",
			holder: "甲乙丙", companyName: "甲",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
		{
			name:   "反向子字串不得相符",
			holder: "甲", companyName: "甲乙丙農場",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
		{
			// 明令禁止編輯距離模糊比對。
			name:   "一字之差不得相符",
			holder: "範例生鮮", companyName: "範例生先",
			wantMatched: false, wantField: CertHolderNone, wantHasReference: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MatchCertHolder(c.holder, &c.companyName)
			matched, field, hasReference := got.Matched, got.Field, got.HasReference
			if matched != c.wantMatched {
				t.Errorf("matched = %v, want %v", matched, c.wantMatched)
			}
			if field != c.wantField {
				t.Errorf("field = %q, want %q", field, c.wantField)
			}
			if hasReference != c.wantHasReference {
				t.Errorf("hasReference = %v, want %v", hasReference, c.wantHasReference)
			}
		})
	}
}

// TestMatchWithoutCompanyNameHasNoReference pins D37: a nil company name (an individual seller) is "no reference", never a match.
func TestMatchWithoutCompanyNameHasNoReference(t *testing.T) {
	got := MatchCertHolder("陳大明", nil)
	if got.Matched || got.Field != CertHolderNone || got.HasReference {
		t.Errorf("MatchCertHolder(nil company) = %+v, want no match and no reference", got)
	}
}

// TestCertHolderField_PinWireValues pins the constants against the strings
// MatchCertificateHolderResponse.matched_field carries; a rename here would
// silently change what the audit trail records.
func TestCertHolderField_PinWireValues(t *testing.T) {
	pins := []struct {
		got  CertHolderField
		want string
	}{
		{CertHolderCompanyName, "COMPANY_NAME"},
		{CertHolderNone, "NONE"},
	}
	for _, p := range pins {
		if string(p.got) != p.want {
			t.Errorf("CertHolderField = %q, want %q", p.got, p.want)
		}
	}
}
