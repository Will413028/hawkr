// Package certholder implements the 業者名稱歸屬比對 rules of
// docs/詳細設計書/19-農業認證驗證（Certification Verification）詳細設計書.md §8.5.1 —
// deciding whether the 業者名稱 a government certificate lookup returns actually
// belongs to the seller who submitted that certificate number.
//
// Why this lives inside seller-service: real_name is PII (docs/modules/seller.md §13 lists it as a
// plaintext-retained column) and must not cross a service boundary. Callers send a
// store_id plus the holder name and get back a boolean and the *name* of the field
// that matched — never the value.
//
// Comparison is deliberately lenient in one direction only. A false "does not match"
// costs one extra manual review; a false "matches" hands out a platform-endorsed
// certification badge for somebody else's certificate. So the normalizer folds away
// noise that cannot change identity (whitespace, full-/half-width, letter case,
// company-form suffixes) and nothing else: no substring containment, no edit distance.
package certholder

import (
	"sort"
	"strings"
	"unicode"
)

// MatchField reports which field the holder name matched. The values are the wire
// values of MatchCertificateHolderResponse.matched_field; audit records the field
// name only, never the name itself (19 號 §8.5.1).
type MatchField string

const (
	MatchRealName    MatchField = "REAL_NAME"
	MatchCompanyName MatchField = "COMPANY_NAME"
	MatchNone        MatchField = "NONE"
)

// trailingNoise is punctuation that carries no identity once a company form has been
// peeled off ("台灣 Hawkr，" / "abc co.," ). Trimmed from the right only.
const trailingNoise = ".,-_·、。"

// companySuffixes are company/organisation forms stripped from the tail of a
// normalized name, written in already-normalized shape (lower-case, half-width,
// whitespace removed). Longest match wins — see sortByLenDesc.
//
// Only suffixes are stripped, never prefixes: these tokens are short enough that
// prefix stripping would corrupt ordinary names ("Corporate Foods" → "orateFoods"),
// and the spec's "LLC 可能有前置空格" concern is already handled by whitespace removal.
//
// Bare "co" is deliberately absent (it is a two-letter tail shared by many ordinary
// words); the abbreviated form "co." is unambiguous and is included.
var companySuffixes = sortByLenDesc([]string{
	// 中文
	"股份有限公司",
	"有限責任公司",
	"有限公司",
	"無限公司",
	"兩合公司",
	"股份公司",
	"(股)公司",
	"有限合夥",
	"企業社",
	"合作社",
	"產銷班",
	"工作室",
	"商行",
	"商號",
	"農場",
	"牧場",
	"農園",
	"果園",
	"公司",
	// 英文（正規化後已無空白，故含連寫變體）
	"company,ltd.",
	"companylimited",
	"incorporated",
	"corporation",
	"colimited",
	"co.,ltd.",
	"co.,ltd",
	"co.ltd.",
	"co.ltd",
	"l.l.c.",
	"company",
	"limited",
	"corp.",
	"ltd.",
	"llc.",
	"inc.",
	"corp",
	"ltd",
	"llc",
	"inc",
	"co.",
})

func sortByLenDesc(in []string) []string {
	out := append([]string(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// Normalize reduces a business name to the form used for comparison:
// whitespace removed, full-width ASCII narrowed, letters lower-cased, company-form
// suffixes stripped.
//
// The result may legitimately be the empty string — "有限公司" on its own carries no
// identity. Callers must treat an empty result as "nothing to compare", never as a
// value that can match (Match does exactly that).
func Normalize(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		// unicode.IsSpace covers ' ', tab, newline, NBSP (U+00A0) and the
		// ideographic space U+3000, so full-width spaces need no special case.
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(narrow(r))
	}
	return stripCompanyForms(strings.ToLower(b.String()))
}

// narrow maps full-width ASCII (U+FF01–U+FF5E: Ａ-Ｚ ａ-ｚ ０-９ and punctuation such
// as （）．，) onto its half-width counterpart. Every other rune, CJK included, is
// returned unchanged.
func narrow(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	return r
}

// stripCompanyForms peels company-form suffixes off the tail, longest first, until
// none applies. Each pass also trims trailing punctuation so that combinations like
// "co.,ltd" survive being split across two passes. Every pass shortens the string, so
// the loop terminates.
func stripCompanyForms(s string) string {
	for {
		s = strings.TrimRight(s, trailingNoise)
		matched := false
		for _, suffix := range companySuffixes {
			if strings.HasSuffix(s, suffix) {
				s = s[:len(s)-len(suffix)]
				matched = true
				break
			}
		}
		if !matched {
			return s
		}
	}
}

// Match reports whether holderName — the 業者名稱 returned by the government lookup —
// belongs to the seller identified by the KYC names realName and companyName.
//
// It returns:
//
//	matched      the holder name equals one of the seller's names after normalization
//	field        which field matched; MatchNone when nothing did
//	hasReference the seller has at least one non-empty name to compare against
//
// hasReference=false means "no basis for judgement", which the caller must route to
// manual review rather than treat as a rejection or an approval (19 號 §8.5.1).
//
// REAL_NAME is checked first: dev data shows real_name present on 97.7% of approved
// KYC records versus 27.3% for company_name, so it is the primary identity and
// company_name the supplement. The order is only observable when both fields
// normalize to the same value, in which case both answers would be true anyway.
func Match(holderName, realName, companyName string) (bool, MatchField, bool) {
	holder := Normalize(holderName)
	// not named "real": that shadows the complex-number builtin.
	personal := Normalize(realName)
	company := Normalize(companyName)

	hasReference := personal != "" || company != ""
	if holder == "" || !hasReference {
		return false, MatchNone, hasReference
	}
	if personal != "" && personal == holder {
		return true, MatchRealName, true
	}
	if company != "" && company == holder {
		return true, MatchCompanyName, true
	}
	return false, MatchNone, hasReference
}
