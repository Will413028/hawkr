package model

import (
	"net/netip"
	"strings"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// AgreementKind 是賣家入駐時要同意或確認已閱讀的文件種類；儲存的代碼是 UPPER_SNAKE（ADR 2026-09-25-codemeta-retirement D3）。
type AgreementKind string

const (
	// AgreementSellerTerms 是賣家契約，開店時同意。
	AgreementSellerTerms AgreementKind = "SELLER_TERMS"
	// AgreementKYCPrivacyNotice 是身分認證的個資蒐集告知，送出 KYC 時確認已閱讀。
	AgreementKYCPrivacyNotice AgreementKind = "KYC_PRIVACY_NOTICE"
	// AgreementPayUniTerms 是 PayUni 的服務條款，送出 KYC 時同意。
	AgreementPayUniTerms AgreementKind = "PAYUNI_TERMS"
)

// AgreementKinds 回傳全部文件種類，依契約 AgreementKind 的數字順序。
func AgreementKinds() []AgreementKind {
	return []AgreementKind{AgreementSellerTerms, AgreementKYCPrivacyNotice, AgreementPayUniTerms}
}

// Agreement 是一份文件的現行版本。Version 是生效日（YYYY-MM-DD），各版本的全文以 repo 裡的正本保存；URL 是站內路徑
// 或外部的絕對網址。現行版本表 CurrentAgreements 由 website/scripts/generate-legal.ts 產生在 agreement_gen.go。
type Agreement struct {
	Version string
	URL     string
}

// 每個操作要求的種類：剛好這幾種，不多不少。
var (
	// ApplyAgreements 是 ApplyStore 要求的文件。
	ApplyAgreements = []AgreementKind{AgreementSellerTerms}
	// SubmitKYCAgreements 是 SubmitKYC 要求的文件。
	SubmitKYCAgreements = []AgreementKind{AgreementKYCPrivacyNotice, AgreementPayUniTerms}
)

// AgreementAcceptance 是賣家對一份文件的同意：Version 是 client 顯示給賣家的版本。
type AgreementAcceptance struct {
	Kind    AgreementKind
	Version string
}

// agreementsField 是欄位錯誤的欄位名（輸入型別上的欄位）。
const agreementsField = "AcceptedAgreements"

// CheckAgreements 核對賣家的同意：required 的每一種剛好出現一次、版本是現行版本。缺種類、多出不屬於這個操作的種類
// （含 UNSPECIFIED 與不認得的值）、同一種重複，回 InvalidInput（欄位 AcceptedAgreements）；種類正確但版本不是現行版本，
// 回 FailedPrecondition（AGREEMENT_OUTDATED）。
func CheckAgreements(required []AgreementKind, accepted []AgreementAcceptance) error {
	seen := make(map[AgreementKind]string, len(accepted))
	for _, a := range accepted {
		if !containsKind(required, a.Kind) {
			return apperr.InvalidInputOn([]string{agreementsField}, "同意的文件種類不屬於這個操作")
		}
		if _, dup := seen[a.Kind]; dup {
			return apperr.InvalidInputOn([]string{agreementsField}, "同一種文件只能同意一次")
		}
		if a.Version == "" {
			return apperr.InvalidInputOn([]string{agreementsField}, "版本必填")
		}
		seen[a.Kind] = a.Version
	}
	for _, k := range required {
		if _, ok := seen[k]; !ok {
			return apperr.InvalidInputOn([]string{agreementsField}, "缺少必須同意的文件")
		}
	}
	for _, k := range required {
		if seen[k] != CurrentAgreements[k].Version {
			return apperr.FailedPrecondition("文件已更新，請重新閱讀並同意").WithReason(apperr.ReasonAgreementOutdated)
		}
	}
	return nil
}

func containsKind(kinds []AgreementKind, k AgreementKind) bool {
	for _, v := range kinds {
		if v == k {
			return true
		}
	}
	return false
}

// maxUserAgentRunes 是儲存的 User-Agent 長度上限（schema 的 agreement_acceptances_user_agent_check，儲存成本的上限）。
const maxUserAgentRunes = 512

// ClientIPAddr 把來源 IP 轉成儲存的值（inet）：去掉 zone、IPv4-mapped 轉成 IPv4；取不到或不是 IP 是 nil。
func ClientIPAddr(ip string) *netip.Addr {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return nil
	}
	addr = addr.WithZone("").Unmap()
	return &addr
}

// UserAgentText 把 User-Agent 轉成儲存的值：去掉不合法的 UTF-8 位元組（HTTP header 允許，Postgres 不收），空字串是 nil，超過上限的截斷。
func UserAgentText(ua string) *string {
	ua = strings.TrimSpace(strings.ToValidUTF8(ua, ""))
	if ua == "" {
		return nil
	}
	if utf8.RuneCountInString(ua) > maxUserAgentRunes {
		ua = string([]rune(ua)[:maxUserAgentRunes])
	}
	return &ua
}
