package model

import (
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"slices"
	"testing"
	"time"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func matchingSubject() RegistrationSubject {
	return RegistrationSubject{
		CompanyName: str("範例小農有限公司"), TownCode: str("63000050"), AddressDetail: str("中山路 2 號"),
		RegisterDate: day(2015, 3, 4),
	}
}

func matchingRegistrationRecord() Registration {
	return Registration{
		Name: "範例小農有限公司", OrganizationType: "有限公司", SetupDate: *day(2015, 3, 4), TownCode: "63000050", AddressDetail: "中山路２號",
	}
}

func TestNormalizeRegistrationText(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"台灣台北", "臺灣臺北"},
		{"臺灣臺北", "臺灣臺北"},
		{" 中山路 ２ 號 ", "中山路2號"},
		{"ＡＢＣ　ｄｅｆ", "ABCdef"},
		{"範例\t小農\n有限公司", "範例小農有限公司"},
		{"", ""},
	} {
		if got := NormalizeRegistrationText(tt.in); got != tt.want {
			t.Errorf("NormalizeRegistrationText(%q) = %q，預期 %q", tt.in, got, tt.want)
		}
	}
}

func TestCompareRegistration(t *testing.T) {
	for name, tt := range map[string]struct {
		subject                   func(*RegistrationSubject)
		reg                       func(*Registration)
		wantMismatch, wantUnknown []string
	}{
		"全部一致（寫法不同）": {},
		"台與臺視為相同": {
			subject: func(s *RegistrationSubject) {
				s.CompanyName, s.AddressDetail = str("台灣小農有限公司"), str("台北路 1 號")
			},
			reg: func(r *Registration) { r.Name, r.AddressDetail = "臺灣小農有限公司", "臺北路１號" },
		},
		"名稱不同": {
			subject:      func(s *RegistrationSubject) { s.CompanyName = str("範例農產有限公司") },
			wantMismatch: []string{FieldCompanyName},
		},
		"登記的組織型態不同或空白都不參與比對": {
			reg: func(r *Registration) { r.OrganizationType = "股份有限公司" },
		},
		"登記的組織型態是空的也不影響": {
			reg: func(r *Registration) { r.OrganizationType = "" },
		},
		"日期差一天": {
			subject:      func(s *RegistrationSubject) { s.RegisterDate = day(2015, 3, 5) },
			wantMismatch: []string{FieldCompanyRegisterDate},
		},
		"日期只比日曆日（時間部分不影響）": {
			subject: func(s *RegistrationSubject) {
				t := time.Date(2015, 3, 4, 15, 30, 0, 0, time.UTC)
				s.RegisterDate = &t
			},
		},
		"鄉鎮市區不同": {
			subject:      func(s *RegistrationSubject) { s.TownCode = str("63000040") },
			wantMismatch: []string{FieldCompanyAddress},
		},
		"明細不同": {
			subject:      func(s *RegistrationSubject) { s.AddressDetail = str("中山路 3 號") },
			wantMismatch: []string{FieldCompanyAddress},
		},
		"來源缺設立日期": {
			reg:         func(r *Registration) { r.SetupDate = time.Time{} },
			wantUnknown: []string{FieldCompanyRegisterDate},
		},
		"申請缺設立日期": {
			subject:     func(s *RegistrationSubject) { s.RegisterDate = nil },
			wantUnknown: []string{FieldCompanyRegisterDate},
		},
		"地址拆不開": {
			reg:         func(r *Registration) { r.TownCode, r.AddressDetail = "", "" },
			wantUnknown: []string{FieldCompanyAddress},
		},
		"申請沒有地址": {
			subject:     func(s *RegistrationSubject) { s.TownCode, s.AddressDetail = nil, nil },
			wantUnknown: []string{FieldCompanyAddress},
		},
		"同時有不一致與無法比對": {
			subject:      func(s *RegistrationSubject) { s.CompanyName = str("別家有限公司") },
			reg:          func(r *Registration) { r.SetupDate = time.Time{} },
			wantMismatch: []string{FieldCompanyName}, wantUnknown: []string{FieldCompanyRegisterDate},
		},
	} {
		subject, reg := matchingSubject(), matchingRegistrationRecord()
		if tt.subject != nil {
			tt.subject(&subject)
		}
		if tt.reg != nil {
			tt.reg(&reg)
		}
		mismatched, unverifiable := CompareRegistration(subject, reg)
		if !slices.Equal(mismatched, append([]string{}, tt.wantMismatch...)) || !slices.Equal(unverifiable, append([]string{}, tt.wantUnknown...)) {
			t.Errorf("%s：mismatched=%v unverifiable=%v，預期 %v 與 %v", name, mismatched, unverifiable, tt.wantMismatch, tt.wantUnknown)
		}
		for _, f := range slices.Concat(mismatched, unverifiable) {
			if f == "business.company_type" || !slices.Contains(RegistrationComparedFields(), f) {
				t.Errorf("%s：比對結果出現不在比對欄位內的 %q", name, f)
			}
		}
		if mismatched == nil || unverifiable == nil {
			t.Errorf("%s：結果必須是空切片，不是 nil（寫入 NOT NULL 的陣列欄位）", name)
		}
	}
}

func TestRegistrationFieldsAreKYCFields(t *testing.T) {
	for _, f := range RegistrationComparedFields() {
		if !slices.Contains(KYCFieldNames(), f) {
			t.Errorf("%s 不在 KYCFieldNames", f)
		}
	}
}

// 申請人不填組織型態：它不是申請的欄位，也不在比對欄位裡（審核員看登記資料快照的組織型態）。
func TestOrganizationTypeIsNotAnApplicationOrComparedField(t *testing.T) {
	want := []string{"business.company_name", "business.company_register_date", "business.company_address"}
	if got := RegistrationComparedFields(); !slices.Equal(got, want) {
		t.Errorf("RegistrationComparedFields = %v，預期 %v", got, want)
	}
	if slices.Contains(KYCFieldNames(), "business.company_type") {
		t.Error("business.company_type 不應在 KYCFieldNames")
	}
}

func TestNewRegistrationSplitsTheAddress(t *testing.T) {
	if r := NewRegistration(gcis.Registration{Address: "臺北市中正區重慶南路一段1號"}); r.TownCode != "63000050" || r.AddressDetail != "重慶南路一段1號" {
		t.Fatalf("registration=%+v", r)
	}
	if r := NewRegistration(gcis.Registration{Address: "新竹科學園區新竹市力行六路8號"}); r.TownCode != "" || r.AddressDetail != "" || r.Address == "" {
		t.Fatalf("registration=%+v", r)
	}
}

// 現況只在這裡分類：公司的 13 種已知文字各落在對照的分類，其餘（含空字串與未來新增的）是 UNKNOWN 並保留原文；商業登記只有代碼
// 01 是 ACTIVE。
func TestNewRegistrationClassifiesTheStatus(t *testing.T) {
	company := []struct {
		text string
		want RegistrationStatus
	}{
		{"核准設立", RegistrationStatusActive},
		{"核准設立，但已命令解散", RegistrationStatusOrderedToDissolve},
		{"解散", RegistrationStatusTerminated}, {"撤銷", RegistrationStatusTerminated}, {"破產", RegistrationStatusTerminated},
		{"合併解散", RegistrationStatusTerminated}, {"撤回登記", RegistrationStatusTerminated}, {"廢止", RegistrationStatusTerminated},
		{"廢止登記", RegistrationStatusTerminated}, {"解散已清算完結", RegistrationStatusTerminated}, {"撤銷已清算完結", RegistrationStatusTerminated},
		{"廢止已清算完結", RegistrationStatusTerminated}, {"撤回登記已清算完結", RegistrationStatusTerminated},
		{"", RegistrationStatusUnknown}, {"停業", RegistrationStatusUnknown}, {"核准設立（未來新增的說法）", RegistrationStatusUnknown},
	}
	for _, c := range company {
		r := NewRegistration(gcis.Registration{Source: gcis.SourceCompany, StatusText: c.text})
		if r.Status != c.want || r.StatusText != c.text {
			t.Errorf("公司 %q：status=%q text=%q，預期 %q", c.text, r.Status, r.StatusText, c.want)
		}
	}
	for _, c := range []struct {
		code string
		want RegistrationStatus
	}{{"01", RegistrationStatusActive}, {"02", RegistrationStatusUnknown}, {"", RegistrationStatusUnknown}} {
		r := NewRegistration(gcis.Registration{Source: gcis.SourceBusiness, StatusCode: c.code, StatusText: "原文"})
		if r.Status != c.want || r.StatusText != "原文" {
			t.Errorf("商業登記 %q：status=%q text=%q", c.code, r.Status, r.StatusText)
		}
	}
	for _, r := range []Registration{NewRegistration(gcis.Registration{}), NewRegistration(gcis.Registration{Source: gcis.SourceBusiness})} {
		if !slices.Contains(RegistrationStatusValues(), r.Status) {
			t.Errorf("狀態 %q 不是有效的分類", r.Status)
		}
	}
}

// 停業期間：迄日早於起日時兩個日期都不採用；同一天、只有一端、都沒有照樣保留。
func TestNewRegistrationDropsAnInvertedSuspensionPeriod(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	for name, tt := range map[string]struct{ from, until, wantFrom, wantUntil time.Time }{
		"顛倒":   {d(1994, 3, 31), d(1993, 11, 1), time.Time{}, time.Time{}},
		"同一天":  {d(1993, 11, 1), d(1993, 11, 1), d(1993, 11, 1), d(1993, 11, 1)},
		"只有起日": {d(1993, 11, 1), time.Time{}, d(1993, 11, 1), time.Time{}},
		"只有迄日": {time.Time{}, d(1994, 3, 31), time.Time{}, d(1994, 3, 31)},
		"正常":   {d(1993, 11, 1), d(1994, 3, 31), d(1993, 11, 1), d(1994, 3, 31)},
	} {
		r := NewRegistration(gcis.Registration{SuspendedFrom: tt.from, SuspendedUntil: tt.until})
		if !r.SuspendedFrom.Equal(tt.wantFrom) || !r.SuspendedUntil.Equal(tt.wantUntil) {
			t.Errorf("%s：from=%v until=%v", name, r.SuspendedFrom, r.SuspendedUntil)
		}
	}
}

// 查詢當天（台北日期）是否在停業期間：由模型這一處算，後台顯示端只讀結果。
func TestRegistrationSuspendedAt(t *testing.T) {
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	taipei := func(m time.Month, d, h int) time.Time { return time.Date(2026, m, d, h, 0, 0, 0, twtime.Taipei) }
	both := Registration{SuspendedFrom: day(10, 1), SuspendedUntil: day(10, 31)}
	for name, c := range map[string]struct {
		reg  Registration
		at   time.Time
		want bool
	}{
		"沒有期間":    {Registration{}, taipei(10, 11, 12), false},
		"起日當天含":   {both, taipei(10, 1, 0), true},
		"迄日當天含":   {both, taipei(10, 31, 23), true},
		"期間中":     {both, taipei(10, 11, 12), true},
		"起日之前":    {both, taipei(9, 30, 23), false},
		"迄日之後":    {both, taipei(11, 1, 0), false},
		"只有起日，之後": {Registration{SuspendedFrom: day(10, 1)}, taipei(12, 1, 0), true},
		"只有起日，之前": {Registration{SuspendedFrom: day(10, 1)}, taipei(9, 1, 0), false},
		"只有迄日，之前": {Registration{SuspendedUntil: day(10, 31)}, taipei(1, 1, 0), true},
		"只有迄日，之後": {Registration{SuspendedUntil: day(10, 31)}, taipei(11, 1, 0), false},
		"台北日期不是 UTC 日期：UTC 9/30 16:00 已是台北 10/1": {both, time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC), true},
	} {
		if got := c.reg.SuspendedAt(c.at); got != c.want {
			t.Errorf("%s：got %v，want %v", name, got, c.want)
		}
	}
}
