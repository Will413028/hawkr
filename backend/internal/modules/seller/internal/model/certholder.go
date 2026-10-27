package model

import (
	"sort"
	"strings"
	"unicode"
)

// 業者名稱歸屬比對（設計書 19 §8.5.1）：政府證書查驗回的業者名稱，是不是屬於送出證書的賣家。名字是個資（身分證上的姓名），
// 所以比對留在 seller 模組，呼叫方（legacy 的 product-service）只拿到判定與哪個欄位相符，拿不到名字。
//
// 規則照設計書：正規化是寬鬆比對，寬鬆的方向是減少誤判為不符（誤判不符只多一筆人工審核，代價可控；不符一律轉人工，不是拒絕）；
// 去掉空白、全半形、大小寫與公司型態後綴，後綴表照設計書明列（含農場、商行、工作室等商號常用詞），所以「陽光農場」與「陽光果園」會
// 正規化成同一個名字、判為相符；除此之外不做子字串包含，也不做編輯距離。比對基準只有已通過人工核准的 KYC 名稱（賣家自己填、
// 平台沒驗過的名稱不能當基準）。

// CertHolderField 是業者名稱相符的欄位；值是 legacy 的 `MatchCertificateHolderResponse.matched_field`，稽核只記欄位名稱。
type CertHolderField string

const (
	// CertHolderRealName 與 KYC 的負責人姓名相符。
	CertHolderRealName CertHolderField = "REAL_NAME"
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

// holderNoise 是去掉公司型態之後留在尾端的標點（只從右邊修剪）。
const holderNoise = ".,-_·、。"

// holderSuffixes 是從名稱尾端依序剝掉的公司型態與商號用詞（設計書 §8.5.1 的後綴表），寫成已正規化的形狀（小寫、半形、無空白），
// 最長的先比。只剝後綴、不剝前綴（這些詞夠短，剝前綴會弄壞一般名稱）；單獨的 "co" 刻意不在裡面（很多一般英文字的結尾），縮寫 "co." 沒有歧義。
var holderSuffixes = sortByLengthDesc([]string{
	// 中文
	"股份有限公司", "有限責任公司", "有限公司", "無限公司", "兩合公司", "股份公司", "(股)公司", "有限合夥",
	"企業社", "合作社", "產銷班", "工作室", "商行", "商號", "農場", "牧場", "農園", "果園", "公司",
	// 英文（正規化後已無空白，所以含連寫的變體）
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

// narrowRune 把全形的 ASCII（U+FF01 到 U+FF5E）轉成半形，其餘（含中文）不動。
func narrowRune(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	return r
}

// stripHolderSuffixes 從尾端依序剝掉公司型態直到沒有可剝的；每一輪都修剪尾端標點，所以 "co.,ltd" 拆成兩輪也剝得掉。每一輪都讓
// 字串變短，迴圈一定結束。
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

// MatchCertHolder 回報 holderName 是否屬於 KYC 的姓名 realName 或公司名稱 companyName（正規化後完全相同）。先比 realName（已核准的
// 申請裡 real_name 的覆蓋率遠高於 company_name），兩者都相符時才看得出順序。兩個名稱正規化後都是空字串時沒有基準
// （HasReference=false），要轉人工，不是放行也不是拒絕；holderName 正規化後是空字串一律不相符。
func MatchCertHolder(holderName string, realName, companyName *string) CertHolderMatch {
	holder := NormalizeHolderName(holderName)
	personal, company := "", ""
	if realName != nil {
		personal = NormalizeHolderName(*realName)
	}
	if companyName != nil {
		company = NormalizeHolderName(*companyName)
	}
	hasReference := personal != "" || company != ""
	switch {
	case holder == "" || !hasReference:
		return CertHolderMatch{Field: CertHolderNone, HasReference: hasReference}
	case personal == holder:
		return CertHolderMatch{Matched: true, Field: CertHolderRealName, HasReference: true}
	case company == holder:
		return CertHolderMatch{Matched: true, Field: CertHolderCompanyName, HasReference: true}
	}
	return CertHolderMatch{Field: CertHolderNone, HasReference: true}
}
