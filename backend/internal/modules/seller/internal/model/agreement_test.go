package model

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func acceptances(kinds ...AgreementKind) []AgreementAcceptance {
	out := make([]AgreementAcceptance, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, AgreementAcceptance{Kind: k, Version: CurrentAgreements[k].Version})
	}
	return out
}

func TestCheckAgreementsAcceptsExactlyTheRequiredKindsAtTheCurrentVersion(t *testing.T) {
	if err := CheckAgreements(ApplyAgreements, acceptances(AgreementSellerTerms)); err != nil {
		t.Fatal(err)
	}
	// 順序不拘。
	if err := CheckAgreements(SubmitKYCAgreements, acceptances(AgreementPayUniTerms, AgreementKYCPrivacyNotice)); err != nil {
		t.Fatal(err)
	}
}

func TestCheckAgreementsRejectsWrongKindsAsInvalidInputOnTheAgreementsField(t *testing.T) {
	cur := func(k AgreementKind) AgreementAcceptance { return acceptances(k)[0] }
	for name, c := range map[string]struct {
		required []AgreementKind
		got      []AgreementAcceptance
	}{
		"沒有任何同意":           {ApplyAgreements, nil},
		"缺一種":              {SubmitKYCAgreements, acceptances(AgreementKYCPrivacyNotice)},
		"多出不屬於操作的種類":       {ApplyAgreements, acceptances(AgreementSellerTerms, AgreementPayUniTerms)},
		"整組是別的操作的種類":       {ApplyAgreements, acceptances(AgreementKYCPrivacyNotice, AgreementPayUniTerms)},
		"同一種兩次":            {SubmitKYCAgreements, []AgreementAcceptance{cur(AgreementKYCPrivacyNotice), cur(AgreementKYCPrivacyNotice), cur(AgreementPayUniTerms)}},
		"UNSPECIFIED（空字串）": {ApplyAgreements, []AgreementAcceptance{cur(AgreementSellerTerms), {Kind: "", Version: "2026-10-07"}}},
		"不認得的種類":           {ApplyAgreements, []AgreementAcceptance{{Kind: "99", Version: "2026-10-07"}}},
		"空的種類取代必要的種類":      {ApplyAgreements, []AgreementAcceptance{{Kind: "", Version: CurrentAgreements[AgreementSellerTerms].Version}}},
	} {
		err := CheckAgreements(c.required, c.got)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AcceptedAgreements" || ae.Reason != (apperr.Reason{}) {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

func TestCheckAgreementsReportsAnOutdatedVersionAsFailedPrecondition(t *testing.T) {
	for name, version := range map[string]string{"舊版": "2026-01-01", "條款 17.4 改版前的版本": "2026-10-07", "新版": "2999-01-01"} {
		err := CheckAgreements(ApplyAgreements, []AgreementAcceptance{{Kind: AgreementSellerTerms, Version: version}})
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonAgreementOutdated {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	// 種類錯誤先於版本：版本也過時的壞請求仍是 invalid_argument。
	err := CheckAgreements(ApplyAgreements, []AgreementAcceptance{{Kind: AgreementPayUniTerms, Version: "2026-01-01"}})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("err=%v", err)
	}
}

func TestEveryKindHasACurrentVersionAndURL(t *testing.T) {
	if len(CurrentAgreements) != len(AgreementKinds()) {
		t.Fatalf("CurrentAgreements 有 %d 筆，種類有 %d 個", len(CurrentAgreements), len(AgreementKinds()))
	}
	for _, k := range AgreementKinds() {
		a := CurrentAgreements[k]
		if a.Version == "" || (!strings.HasPrefix(a.URL, "/") && !strings.HasPrefix(a.URL, "https://")) {
			t.Errorf("%s：%+v", k, a)
		}
	}
	for _, k := range append(append([]AgreementKind{}, ApplyAgreements...), SubmitKYCAgreements...) {
		if _, ok := CurrentAgreements[k]; !ok {
			t.Errorf("%s 沒有現行版本", k)
		}
	}
}

func TestSourceTextsNormalizeToStoredValues(t *testing.T) {
	for in, want := range map[string]string{"203.0.113.9": "203.0.113.9", " 203.0.113.9 ": "203.0.113.9", "::ffff:203.0.113.9": "203.0.113.9", "2001:db8::1": "2001:db8::1", "fe80::1%eth0": "fe80::1"} {
		if got := ClientIPAddr(in); got == nil || got.String() != want {
			t.Errorf("ClientIPAddr(%q) = %v，預期 %q", in, got, want)
		}
	}
	for _, in := range []string{"", "  ", "unknown", "203.0.113.9, 10.0.0.1", "203.0.113.9:443"} {
		if got := ClientIPAddr(in); got != nil {
			t.Errorf("ClientIPAddr(%q) = %v，預期 nil", in, got)
		}
	}
	if got := UserAgentText("  "); got != nil {
		t.Errorf("空白的 UA = %q", *got)
	}
	if got := UserAgentText(strings.Repeat("界", 600)); got == nil || len([]rune(*got)) != 512 {
		t.Errorf("過長的 UA 沒有截斷：%v", got)
	}
}

// 版本是必填：空字串是 invalid_argument（欄位 AcceptedAgreements），不是版本過時。
func TestCheckAgreementsRejectsAnEmptyVersionAsInvalidInput(t *testing.T) {
	err := CheckAgreements(ApplyAgreements, []AgreementAcceptance{{Kind: AgreementSellerTerms, Version: ""}})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AcceptedAgreements" || ae.Reason != (apperr.Reason{}) {
		t.Fatalf("err=%v", err)
	}
}

// User-Agent 可以含非 UTF-8 位元組（HTTP header 允許）：去掉之後再存，Postgres 才收得下。
func TestUserAgentTextDropsInvalidUTF8(t *testing.T) {
	got := UserAgentText("hk/1.0 \xff\xfe(測試)\x80")
	if got == nil || *got != "hk/1.0 (測試)" || !utf8.ValidString(*got) {
		t.Fatalf("got=%v", got)
	}
	if UserAgentText("\xff\xfe") != nil {
		t.Fatal("只有不合法位元組的 UA 應該是 nil")
	}
	if got := UserAgentText(strings.Repeat("a", 511) + "\xff" + "界界"); got == nil || !utf8.ValidString(*got) || utf8.RuneCountInString(*got) != 512 {
		t.Fatalf("先清理再截斷：%v", got)
	}
}
