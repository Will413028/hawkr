package model

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func ptrTo(s string) *string { return &s }

func TestNormalizeProspect(t *testing.T) {
	valid := func() ProspectFields {
		return ProspectFields{DisplayName: " 範例小農 ", VendorType: VendorBusiness}
	}
	t.Run("去空白、空字串是沒有值、進度預設沒聯絡", func(t *testing.T) {
		f := valid()
		f.Region, f.TaxID, f.Category = ptrTo("  "), ptrTo(" 12345678 "), ptrTo(" 水果 ")
		if err := NormalizeProspect(&f, true); err != nil {
			t.Fatal(err)
		}
		if f.DisplayName != "範例小農" || f.Region != nil || *f.TaxID != "12345678" || *f.Category != "水果" || f.OutreachStatus != OutreachNotContacted {
			t.Fatalf("fields=%+v", f)
		}
	})
	for name, mutate := range map[string]func(*ProspectFields){
		"沒有名稱":        func(f *ProspectFields) { f.DisplayName = "  " },
		"名稱太長":        func(f *ProspectFields) { f.DisplayName = strings.Repeat("字", 256) },
		"業者類型不合法":     func(f *ProspectFields) { f.VendorType = "individual" },
		"招商狀態不合法":     func(f *ProspectFields) { f.OutreachStatus = "MAYBE" },
		"統編不是 8 碼":    func(f *ProspectFields) { f.TaxID = ptrTo("1234567") },
		"統編含字母":       func(f *ProspectFields) { f.TaxID = ptrTo("1234567a") },
		"priority 太長": func(f *ProspectFields) { f.PriorityLetter = ptrTo("ABCDE") },
		"星等為負":        func(f *ProspectFields) { f.PriorityStars = -1 },
		"管理員不能手動設已入駐": func(f *ProspectFields) { f.OutreachStatus = OutreachConverted },
		"研究摘要超過上限":    func(f *ProspectFields) { f.ResearchSummary = ptrTo(strings.Repeat("字", prospectTextMax+1)) },
	} {
		t.Run(name, func(t *testing.T) {
			f := valid()
			mutate(&f)
			if err := NormalizeProspect(&f, true); apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Fatalf("err=%v", err)
			}
		})
	}
	t.Run("內部寫入可以是已入駐", func(t *testing.T) {
		f := valid()
		f.OutreachStatus = OutreachConverted
		if err := NormalizeProspect(&f, false); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMergeProspectKeepsNilAndClearsEmpty(t *testing.T) {
	current := ProspectFields{DisplayName: "原名", VendorType: VendorBusiness, Positioning: ptrTo("精品"), Category: ptrTo("水果"), PriorityStars: 3}
	merged := MergeProspect(current, UpdateProspectInput{Positioning: ptrTo(""), Region: ptrTo(" 台南 ")})
	if merged.Positioning != nil || *merged.Region != "台南" || *merged.Category != "水果" || merged.DisplayName != "原名" || merged.PriorityStars != 3 {
		t.Fatalf("merged=%+v", merged)
	}
}

func TestNormalizeCategoryIDs(t *testing.T) {
	got, err := NormalizeCategoryIDs([]string{" b ", "a", "b"})
	if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, bad := range [][]string{{""}, {"  "}, {strings.Repeat("x", 65)}} {
		if _, err := NormalizeCategoryIDs(bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%v: %v", bad, err)
		}
	}
}

func TestNormalizeContacts(t *testing.T) {
	got, err := NormalizeContacts([]ContactPoint{
		{ContactPhone, " 02-1234-5678 ", false},
		{ContactEmail, "b@x.tw", true},
		{ContactEmail, "a@x.tw", false},
		{ContactEmail, "b@x.tw", false},
		{ContactWebsite, "https://x.tw", false},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []ContactPoint{
		{ContactEmail, "a@x.tw", false}, {ContactEmail, "b@x.tw", true},
		{ContactPhone, "02-1234-5678", true}, {ContactWebsite, "https://x.tw", true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	if empty, err := NormalizeContacts(nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty=%v err=%v", empty, err)
	}
	for name, bad := range map[string][]ContactPoint{
		"類型不合法":       {{"FAX", "1", true}},
		"值是空白":        {{ContactEmail, "  ", true}},
		"email 太長":    {{ContactEmail, strings.Repeat("a", 256), true}},
		"電話太長":        {{ContactPhone, strings.Repeat("1", 51), true}},
		"同類型多列都沒有主要的": {{ContactEmail, "a@x.tw", false}, {ContactEmail, "b@x.tw", false}},
		"同類型兩列都是主要的":  {{ContactEmail, "a@x.tw", true}, {ContactEmail, "b@x.tw", true}},
		"超過安全上限":      manyContacts(maxContacts + 1),
	} {
		if _, err := NormalizeContacts(bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func manyContacts(n int) []ContactPoint {
	out := make([]ContactPoint, n)
	for i := range out {
		out[i] = ContactPoint{ContactEmail, strconv.Itoa(i) + "@x.tw", i == 0}
	}
	return out
}

func TestSameContactsIgnoresOrderButNotThePrimary(t *testing.T) {
	a := []ContactPoint{{ContactEmail, "a@x.tw", true}, {ContactEmail, "b@x.tw", false}}
	b := []ContactPoint{{ContactEmail, "b@x.tw", false}, {ContactEmail, "a@x.tw", true}}
	if !SameContacts(a, b) {
		t.Fatal("order mattered")
	}
	if SameContacts(a, []ContactPoint{{ContactEmail, "a@x.tw", false}, {ContactEmail, "b@x.tw", true}}) {
		t.Fatal("the primary flag was ignored")
	}
}

func TestProspectMatchFoundCarriesTheReasonAndSortedIDs(t *testing.T) {
	err := error(NewProspectMatchFound([]int64{12, 3, 9}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonProspectMatchFound ||
		ae.Metadata[ProspectIDsMetadataKey] != "3,9,12" || len(ae.Metadata) != 1 {
		t.Fatalf("err=%+v", ae)
	}
	if ProspectIDsMetadataKey != "prospectIds" {
		t.Fatal("metadata key 是契約")
	}
}
