package model

import (
	"sort"
	"strings"
	"unicode"
)


// CertHolderField 是業者名稱相符的欄位；值是 legacy 的 `MatchCertificateHolderResponse.matched_field`，稽核只記欄位名稱。
type CertHolderField string

const (
	// CertHolderCompanyName 與 KYC 的公司名稱相符。
	CertHolderCompanyName CertHolderField = "COMPANY_NAME"
	// CertHolderNone 沒有相符的欄位。
	CertHolderNone CertHolderField = "NONE"
)

// CertHolderMatch 是業者名稱歸屬比對的結果。Matched 與 HasReference 不是冗餘的：Matched=false 而 HasReference=true 是「跟已核准的名稱
// 比過、不符」，兩者都是 false 是「完全沒有可比的名稱」，前者是這張證書的證據，後者是賣家 KYC 狀態的證據；兩者都送人工審核，
// 但稽核紀錄要能分辨。
type CertHolderMatch struct {
	Matched      bool
	Field        CertHolderField
	HasReference bool
}

const holderNoise = ".,-_·、。"

var holderSuffixes = sortByLengthDesc([]string{
	"股份有限公司", "有限責任公司", "有限公司", "無限公司", "兩合公司", "股份公司", "(股)公司", "有限合夥",
	"企業社", "合作社", "產銷班", "工作室", "商行", "商號", "農場", "牧場", "農園", "果園", "公司",
	"company,ltd.", "companylimited", "incorporated", "corporation", "colimited", "co.,ltd.", "co.,ltd", "co.ltd.",
	"co.ltd", "l.l.c.", "company", "limited", "corp.", "ltd.", "llc.", "inc.", "corp", "ltd", "llc", "inc", "co.",
})

func sortByLengthDesc(in []string) []string {
	out := append([]string(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// NormalizeHolderName 把業者名稱化成比對用的形狀：去掉空白、全形英數標點轉半形、英文小寫、剝掉尾端的公司型態。結果可以是空字串
// （只有「有限公司」不帶身分），呼叫方一律把空字串當成「沒有可比的東西」，不是一個可以相符的值（MatchCertHolder 就是這樣）。
func NormalizeHolderName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(narrowRune(r))
	}
	return stripHolderSuffixes(strings.ToLower(b.String()))
}

func narrowRune(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	return r
}

func stripHolderSuffixes(s string) string {
	for {
		s = strings.TrimRight(s, holderNoise)
		matched := false
		for _, suffix := range holderSuffixes {
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

// MatchCertHolder 回報 holderName 是否屬於 KYC 的公司名稱 companyName（正規化後完全相同）。基準只有公司名稱（收款開通 ADR D37：平台不再收個人姓名，
// 個人賣家沒有基準）。公司名稱正規化後是空字串（個人賣家，或沒有公司名稱）時沒有基準（HasReference=false），要轉人工，不是放行也不是拒絕；
// holderName 正規化後是空字串一律不相符。
func MatchCertHolder(holderName string, companyName *string) CertHolderMatch {
	holder := NormalizeHolderName(holderName)
	company := ""
	if companyName != nil {
		company = NormalizeHolderName(*companyName)
	}
	hasReference := company != ""
	switch {
	case holder == "" || !hasReference:
		return CertHolderMatch{Field: CertHolderNone, HasReference: hasReference}
	case company == holder:
		return CertHolderMatch{Matched: true, Field: CertHolderCompanyName, HasReference: true}
	}
	return CertHolderMatch{Field: CertHolderNone, HasReference: true}
}
