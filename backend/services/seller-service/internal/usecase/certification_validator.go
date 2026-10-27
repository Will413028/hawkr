package usecase

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// ValidationError aggregates multiple field-level validation failures so the
// caller can present all problems to the user in one go.
//
// Error() joins messages with a full-width semicolon (U+FF1B) "；" to form a
// single string that fits cleanly into Gateway's Envelope.message field,
// requiring zero frontend changes.
type ValidationError struct {
	Messages []string
}

// Error implements the error interface. Messages are joined with "；" (U+FF1B).
func (e *ValidationError) Error() string {
	return strings.Join(e.Messages, "；")
}

// DomainCode maps ValidationError to CodeInvalidInput (400) so the gRPC error
// interceptor returns codes.InvalidArgument.
func (e *ValidationError) DomainCode() int {
	return int(pkgerrors.CodeInvalidInput)
}

// DomainMessage returns the joined message string.
func (e *ValidationError) DomainMessage() string {
	return e.Error()
}

// HasErrors returns true if any message has been collected.
func (e *ValidationError) HasErrors() bool {
	return len(e.Messages) > 0
}

// add appends a validation message.
func (e *ValidationError) add(msg string) {
	e.Messages = append(e.Messages, msg)
}

// validatePayuniFields runs all PayUni-side format checks that would otherwise
// surface as AGENT03* / AGENT04* / AGENT08* errors from the PayUni API.
//
// IMPORTANT: All checks run independently. We collect every failure into a
// single ValidationError and return at the end. Callers MUST NOT short-circuit
// on the first error — this is the contract that gives users a full problem
// list in one response.
//
// Returns nil on success, or *ValidationError if any field failed.
func validatePayuniFields(app *domain.CertificationApplication, seller *domain.Seller, email, phone string) error {
	verr := &ValidationError{}

	// Email (AGENT03009)
	if !isValidEmail(email) {
		verr.add("電子信箱格式不符")
	}

	// Contact mobile — try account-service phone first, fall back to seller.ContactPhone.
	// PayUni requires mobile OR landline, and we always send mobile.
	effectiveMobile := phone
	if !isTWMobile(effectiveMobile) {
		effectiveMobile = seller.ContactPhone
	}
	if !isTWMobile(effectiveMobile) {
		verr.add("聯絡手機格式不符，需為 09xx-xxxxxx")
	}

	// English store name (AGENT04028/04029) — required by PayUni bind_merchant
	switch {
	case app.EnglishName == "":
		verr.add("商店英文名稱不可為空")
	case !isPayuniEnglishName(app.EnglishName):
		verr.add("商店英文名稱格式不符，僅容許英數、半形空白與符號 ~!@#$%^&*()_-+={[}]\\:;\"<,>.?/'")
	case utf8.RuneCountInString(app.EnglishName) > 50:
		verr.add("商店英文名稱不可超過 50 字")
	}

	// Identity documents (both seller types) — PayUni review needs to see these.
	// finalizeStepDataTokens() best-effort-converts staged_token → asset_id and
	// silently leaves the asset_id empty on any finalize failure (media-service
	// down, expired/invalid token, ...), so this is the one place that actually
	// gates Submit on the documents being linked.
	if app.IDCardFrontAssetID == "" {
		verr.add("請上傳身分證正面照片")
	}
	if app.IDCardBackAssetID == "" {
		verr.add("請上傳身分證反面照片")
	}
	if app.BankBookAssetID == "" {
		verr.add("請上傳存摺封面照片")
	}

	if app.SellerType == "business" {
		validateBusinessFields(app, verr)
	} else {
		validateIndividualFields(app, seller, verr)
	}

	// Store name length (PayUni MerName ≤ 20)
	if seller.StoreName != "" && utf8.RuneCountInString(seller.StoreName) > 20 {
		verr.add("店名不可超過 20 字")
	}

	// Store description length (AGENT04027: MerDesc ≤ 500)
	if seller.Description != "" && utf8.RuneCountInString(seller.Description) > 500 {
		verr.add("商店簡介不可超過 500 字")
	}

	// Note: postal code validation is intentionally NOT included here.
	// pkg/twaddr.LookupPostalCode always returns a value (falls back to "100"
	// for unresolvable addresses), so a presence check is meaningless. PayUni
	// will return AGENT04023 for truly bad postcodes, which we map to 400.

	if verr.HasErrors() {
		return verr
	}
	return nil
}

// validateBusinessFields validates fields specific to business sellers.
func validateBusinessFields(app *domain.CertificationApplication, verr *ValidationError) {
	// Company name length (AGENT03029: ≤ 200)
	if app.CompanyName == "" {
		verr.add("公司名稱不可為空")
	} else if utf8.RuneCountInString(app.CompanyName) > 200 {
		verr.add("公司名稱不可超過 200 字")
	}

	// Tax ID format only (AGENT03004). The authoritative existence check is the
	// GCIS lookup the seller already ran (gateway /app/businessRegistration/{ban})
	// plus admin review + PayUni bind_merchant. We deliberately drop the local
	// checksum: GCIS is the source of truth, and re-implementing the government
	// rule only ever false-negatives valid IDs — it already drifted once
	// (mod-10 → mod-5 in 2023) and caused #1201.
	// ponytail: format-only; if bogus-but-8-digit tax IDs reaching admin review
	// becomes a problem, gate on GCIS-resolvable instead (issue #1201 option ②).
	if app.TaxID == "" {
		verr.add("統一編號不可為空")
	} else if !twTaxIDRe.MatchString(app.TaxID) {
		verr.add("統一編號格式不符")
	}

	// Responsible person ID (MCID — AGENT08004)
	if app.IDCardNo != "" && !isValidTWID(app.IDCardNo) {
		verr.add("負責人身分證字號格式不符")
	}

	// Company phone — landline format (AGENT04010)
	if app.CompanyPhone != "" && !isTWLandline(app.CompanyPhone) {
		verr.add("公司電話格式不符，需為市話格式，例如 02-12345678")
	}
}

// validateIndividualFields validates fields specific to individual sellers.
func validateIndividualFields(app *domain.CertificationApplication, seller *domain.Seller, verr *ValidationError) {
	// Personal name length (AGENT03028: ≤ 25)
	if app.RealName == "" {
		verr.add("姓名不可為空")
	} else if utf8.RuneCountInString(app.RealName) > 25 {
		verr.add("姓名不可超過 25 字")
	}

	// Individual ID card format + checksum (AGENT03004)
	if app.IDCardNo == "" {
		verr.add("身分證字號不可為空")
	} else if !isValidTWID(app.IDCardNo) {
		verr.add("身分證字號格式不符")
	}

	// Onboarding: store name and city must exist
	if seller.StoreName == "" {
		verr.add("店名不可為空，請先至店鋪資訊填寫店名")
	}
	if seller.City == "" {
		verr.add("縣市不可為空，請先至店鋪資訊填寫地址")
	} else {
		// Validate full address parses to a known city+district. This catches
		// "city only, no district" case (e.g. "桃園市" with empty district).
		if _, err := twaddr.ValidateAddress(seller.FullAddress()); err != nil {
			verr.add(err.Error())
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Field-level validators
// ─────────────────────────────────────────────────────────────────────────────

// isValidEmail checks RFC 5322 email format AND requires a domain with a dot
// (so `user@localhost` is rejected even though Go's net/mail accepts it).
func isValidEmail(s string) bool {
	if s == "" {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 0 {
		return false
	}
	return strings.Contains(addr.Address[at+1:], ".")
}

var twMobileRe = regexp.MustCompile(`^09\d{2}-?\d{6}$`)

// isTWMobile checks Taiwan mobile format: 09xx-xxxxxx or 09xxxxxxxx.
func isTWMobile(s string) bool {
	return twMobileRe.MatchString(s)
}

var twLandlineRe = regexp.MustCompile(`^0\d{1,2}-?\d{6,8}(#\d+)?$`)

// isTWLandline checks Taiwan landline format: 0X-XXXXXXX or 0XX-XXXXXXXX (with optional #ext).
// Hyphen is optional — accepts both "02-29010572" and "0229010572".
func isTWLandline(s string) bool {
	return twLandlineRe.MatchString(s)
}

var payuniEnglishNameRe = regexp.MustCompile(`^[A-Za-z0-9 ~!@#$%^&*()_\-+={[}\]\\:;"<,>.?/']+$`)

// isPayuniEnglishName checks the character set allowed by PayUni MerEngName.
// Length is checked separately by the caller.
func isPayuniEnglishName(s string) bool {
	return payuniEnglishNameRe.MatchString(s)
}

// ─────────────────────────────────────────────────────────────────────────────
// Taiwan ID card checksum
// ─────────────────────────────────────────────────────────────────────────────

// twIDLetterMap maps ID card letter prefix to a two-digit number.
// Source: Taiwan MOI ID card specification.
var twIDLetterMap = map[rune]int{
	'A': 10, 'B': 11, 'C': 12, 'D': 13, 'E': 14, 'F': 15, 'G': 16, 'H': 17,
	'I': 34, 'J': 18, 'K': 19, 'L': 20, 'M': 21, 'N': 22, 'O': 35, 'P': 23,
	'Q': 24, 'R': 25, 'S': 26, 'T': 27, 'U': 28, 'V': 29, 'W': 32, 'X': 30,
	'Y': 31, 'Z': 33,
}

var twIDRe = regexp.MustCompile(`^[A-Z][12]\d{8}$`)

// isValidTWID validates a Taiwan national ID card number, including format,
// gender digit (2nd position must be 1 or 2), and checksum.
//
// Algorithm:
//  1. Format: [A-Z] + [12] + 8 digits
//  2. Letter -> 2-digit number (via twIDLetterMap), split into tens/units
//  3. Weighted sum: tens*1 + units*9 + d1*8 + d2*7 + d3*6 + d4*5 + d5*4 + d6*3 + d7*2 + d8*1 + d9*1
//     (d1..d9 are the 9 digits after the letter, d9 is the check digit)
//  4. Valid iff sum % 10 == 0
func isValidTWID(s string) bool {
	if !twIDRe.MatchString(s) {
		return false
	}
	letterVal, ok := twIDLetterMap[rune(s[0])]
	if !ok {
		return false
	}
	tens := letterVal / 10
	units := letterVal % 10

	// Digits after the letter (9 digits)
	digits := make([]int, 9)
	for i := 0; i < 9; i++ {
		digits[i] = int(s[i+1] - '0')
	}

	// Weights for the 9 digits after the letter: 8, 7, 6, 5, 4, 3, 2, 1, 1
	weights := [9]int{8, 7, 6, 5, 4, 3, 2, 1, 1}
	sum := tens*1 + units*9
	for i, d := range digits {
		sum += d * weights[i]
	}
	return sum%10 == 0
}

// ─────────────────────────────────────────────────────────────────────────────
// Taiwan Tax ID (統一編號)
// ─────────────────────────────────────────────────────────────────────────────

// twTaxIDRe matches the syntactic form only: exactly 8 digits. The checksum is
// deliberately NOT validated here — GCIS is the source of truth (see
// validateBusinessFields and #1201).
var twTaxIDRe = regexp.MustCompile(`^\d{8}$`)
