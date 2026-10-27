package model

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"uuid"

	"golang.org/x/text/unicode/norm"

	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 公司型申請送件時對經濟部商業司公開資料的查詢快照（ADR 2026-10-19-seller-kyc-payuni-consolidated「登記資料快照」）：審核的依據
// 是送件當時查到的登記資料與比對結果，不是審核當下才查。

// RegistrationOutcome 是一次登記資料查詢的結果，與 DB 的 kyc_registration_checks_outcome_check 一致。
type RegistrationOutcome string

const (
	// RegistrationPending 還沒查完。
	RegistrationPending RegistrationOutcome = "PENDING"
	// RegistrationFound 查到登記資料。
	RegistrationFound RegistrationOutcome = "FOUND"
	// RegistrationNotFound 公司登記與商業登記都查無這個統編。
	RegistrationNotFound RegistrationOutcome = "NOT_FOUND"
	// RegistrationUnavailable 資料來源重試到最後一次仍不可用（不代表查無）。
	RegistrationUnavailable RegistrationOutcome = "UNAVAILABLE"
	// RegistrationCancelled 是 job 認領時申請已經不是這次送件審核中（退回、取消、重送過，或已不是公司型）而放棄的查詢：終態，
	// 不顯示、不比對，契約沒有這個值。
	RegistrationCancelled RegistrationOutcome = "CANCELLED"
)

// RegistrationOutcomeValues 回傳全部結果。
func RegistrationOutcomeValues() []RegistrationOutcome {
	return []RegistrationOutcome{RegistrationPending, RegistrationFound, RegistrationNotFound, RegistrationUnavailable, RegistrationCancelled}
}

// RegistrationStatus 是登記現況的分類，與 DB 的 kyc_registration_checks_found_check 的值一致。來源的現況文字不在對照表內時是
// RegistrationStatusUnknown，原文在 Registration.StatusText；空字串不是有效值。
type RegistrationStatus string

const (
	// RegistrationStatusActive 核准設立。
	RegistrationStatusActive RegistrationStatus = "ACTIVE"
	// RegistrationStatusOrderedToDissolve 核准設立，但已命令解散。
	RegistrationStatusOrderedToDissolve RegistrationStatus = "ORDERED_TO_DISSOLVE"
	// RegistrationStatusTerminated 解散、撤銷、破產、合併解散、撤回登記、廢止，以及這些之後的清算完結。
	RegistrationStatusTerminated RegistrationStatus = "TERMINATED"
	// RegistrationStatusUnknown 對照表沒有的現況。
	RegistrationStatusUnknown RegistrationStatus = "UNKNOWN"
)

// RegistrationStatusValues 回傳全部現況分類。
func RegistrationStatusValues() []RegistrationStatus {
	return []RegistrationStatus{RegistrationStatusActive, RegistrationStatusOrderedToDissolve, RegistrationStatusTerminated, RegistrationStatusUnknown}
}

// RegistrationRequestKind 是誰觸發了這次查詢，與 DB 的 kyc_registration_checks_kind_check 一致。
type RegistrationRequestKind string

const (
	// RegistrationRequestSubmit 賣家送件。
	RegistrationRequestSubmit RegistrationRequestKind = "SUBMIT"
	// RegistrationRequestAdminRequery 後台重新查詢。
	RegistrationRequestAdminRequery RegistrationRequestKind = "ADMIN_REQUERY"
)

// RegistrationRequestKindValues 回傳全部觸發來源。
func RegistrationRequestKindValues() []RegistrationRequestKind {
	return []RegistrationRequestKind{RegistrationRequestSubmit, RegistrationRequestAdminRequery}
}

// 比對的欄位（KYCFieldNames 的路徑）；與 DB 的 kyc_registration_checks_fields_check 一致。
const (
	FieldCompanyName         = "business.company_name"
	FieldCompanyAddress      = "business.company_address"
	FieldCompanyRegisterDate = "business.company_register_date"
)

// RegistrationComparedFields 回傳登記資料會比對的欄位，依 KYCFieldNames 的順序。
func RegistrationComparedFields() []string {
	return []string{FieldCompanyName, FieldCompanyRegisterDate, FieldCompanyAddress}
}

// Registration 是查詢取得的登記資料（來源的值，不是申請填的值）。沒有的欄位是零值。由 NewRegistration 從資料來源的原始資料轉出
// （現況分類、停業期間的合理性、地址拆分都在那裡）；TownCode 與 AddressDetail 是 Address 拆出的結果（拆不開時兩者都是空字串）。
type Registration struct {
	TaxID, Name string
	// Address 是來源的地址原文。
	Address string
	// SetupDate 是設立日期（日曆日，UTC 午夜），零值是來源缺漏。
	SetupDate time.Time
	// OrganizationType 是組織型態，空字串是判斷不出。
	OrganizationType string
	Status           RegistrationStatus
	// StatusText 是來源的現況原文。
	StatusText string
	// SuspendedFrom、SuspendedUntil 是停業期間（日曆日），零值是沒有；是否涵蓋某個時間點由顯示端依查詢時間判斷。
	SuspendedFrom, SuspendedUntil time.Time
	// TownCode 與 AddressDetail 是 Address 拆出的鄉鎮市區代碼與其後的地址，拆不開時是空字串。
	TownCode, AddressDetail string
}

// RegistrationCheck 是一次登記資料查詢的結果與比對（已完成的那種；PENDING 的只有 ID、Seq、Outcome、RequestedAt 與 Deadline）。
type RegistrationCheck struct {
	ID  uuid.UUID
	Seq int64
	// RequestedAt 是這筆查詢被要求的時間（送件或後台重新查詢）。
	RequestedAt time.Time
	// Deadline 只在 PENDING 時有值：伺服器保證這筆查詢在這個時間之前到終態（查到、查無、不可用，或被取代而放棄）。
	Deadline *time.Time
	// Outcome 是結果；QueriedAt 在非 PENDING 時有值。
	Outcome   RegistrationOutcome
	QueriedAt *time.Time
	// Registration 只在 Outcome 是 FOUND 時有值。
	Registration *Registration
	// MismatchedFields 是申請與登記資料不一致的欄位；UnverifiableFields 是無法比對的欄位（來源缺漏或地址拆不開）。
	MismatchedFields, UnverifiableFields []string
}

// SuspendedAt 回報 asOf（以台北日期計）是否落在登記的停業期間內：起迄都有時含起迄兩天；只有起日表示起日（含）之後、只有迄日表示迄日
// （含）之前；兩者都沒有是 false。快照只存期間，「查詢當天是否在停業」由這一處依查詢時間算，後台顯示端不自己比對日期。
func (r Registration) SuspendedAt(asOf time.Time) bool {
	if r.SuspendedFrom.IsZero() && r.SuspendedUntil.IsZero() {
		return false
	}
	day := twtime.Day(asOf)
	if !r.SuspendedFrom.IsZero() && day.Before(r.SuspendedFrom) {
		return false
	}
	return r.SuspendedUntil.IsZero() || !day.After(r.SuspendedUntil)
}

// RegistrationSubject 是申請上要與登記資料比對的欄位（公司型）。指標欄位是 nil 或空字串表示申請沒有填（送出之後必填，
// 所以只在個資已清除時發生），那一欄記為無法比對。
type RegistrationSubject struct {
	CompanyName, TownCode, AddressDetail *string
	RegisterDate                         *time.Time
}

// NormalizeRegistrationText 把名稱與地址明細化成比對用的形狀：NFKC（全形英數與相容字轉成一般字）、台視為臺、去掉所有空白。
// 這是寬鬆比對：只吸收書寫差異，不做子字串或近似比對；不一致只是提示審核員，不改申請狀態。
func NormalizeRegistrationText(s string) string {
	s = norm.NFKC.String(s)
	s = strings.NewReplacer("台", "臺").Replace(s)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// CompareRegistration 比對申請與登記資料，回傳不一致與無法比對的欄位（都依 RegistrationComparedFields 的順序，兩者不重疊）。
// reg.TownCode、reg.AddressDetail 必須已經由呼叫方拆好（拆不開時是空字串，地址記為無法比對）：
//   - 公司名稱：正規化後相同才算一致；
//   - 地址：鄉鎮市區代碼相同，而且明細正規化後相同；
//   - 設立日期：任一方缺漏無法比對，否則日曆日相同。
func CompareRegistration(subject RegistrationSubject, reg Registration) (mismatched, unverifiable []string) {
	mismatched, unverifiable = []string{}, []string{}
	record := func(field string, status int) {
		switch status {
		case cmpMismatch:
			mismatched = append(mismatched, field)
		case cmpUnverifiable:
			unverifiable = append(unverifiable, field)
		}
	}
	text := func(a *string, b string) int {
		if a == nil || NormalizeRegistrationText(*a) == "" || NormalizeRegistrationText(b) == "" {
			return cmpUnverifiable
		}
		if NormalizeRegistrationText(*a) != NormalizeRegistrationText(b) {
			return cmpMismatch
		}
		return cmpSame
	}
	record(FieldCompanyName, text(subject.CompanyName, reg.Name))
	switch {
	case subject.RegisterDate == nil || subject.RegisterDate.IsZero() || reg.SetupDate.IsZero():
		record(FieldCompanyRegisterDate, cmpUnverifiable)
	default:
		ay, am, ad := subject.RegisterDate.Date()
		by, bm, bd := reg.SetupDate.Date()
		if ay != by || am != bm || ad != bd {
			record(FieldCompanyRegisterDate, cmpMismatch)
		}
	}
	switch {
	case reg.TownCode == "" || subject.TownCode == nil || *subject.TownCode == "":
		record(FieldCompanyAddress, cmpUnverifiable)
	case *subject.TownCode != reg.TownCode:
		record(FieldCompanyAddress, cmpMismatch)
	default:
		record(FieldCompanyAddress, text(subject.AddressDetail, reg.AddressDetail))
	}
	return mismatched, unverifiable
}

const (
	cmpSame = iota
	cmpMismatch
	cmpUnverifiable
)

// companyStatuses 是公司登記現況文字（Company_Status_Desc，沒有代碼）的對照，2026-10-07 以公開資料取樣（名稱查詢端點的
// Company_Status 代碼 01–14 的 13 種；03、15 查無樣本，落在 RegistrationStatusUnknown）。現況分類只有這一份。
var companyStatuses = map[string]RegistrationStatus{
	"核准設立":        RegistrationStatusActive,
	"核准設立，但已命令解散": RegistrationStatusOrderedToDissolve,
	"解散":          RegistrationStatusTerminated,
	"撤銷":          RegistrationStatusTerminated,
	"破產":          RegistrationStatusTerminated,
	"合併解散":        RegistrationStatusTerminated,
	"撤回登記":        RegistrationStatusTerminated,
	"廢止":          RegistrationStatusTerminated,
	"廢止登記":        RegistrationStatusTerminated,
	"解散已清算完結":     RegistrationStatusTerminated,
	"撤銷已清算完結":     RegistrationStatusTerminated,
	"廢止已清算完結":     RegistrationStatusTerminated,
	"撤回登記已清算完結":   RegistrationStatusTerminated,
}

// classifyRegistrationStatus 把來源的現況分類：公司看文字，商業登記看代碼（目前只有 '01' 核准設立有樣本，其他代碼沒有樣本，一律
// UNKNOWN 並保留原文；觸發：取得商業登記非 01 的樣本時補對照）。對照以外的值是 UNKNOWN，不是空字串。
func classifyRegistrationStatus(src gcis.Registration) RegistrationStatus {
	if src.Source == gcis.SourceBusiness {
		if strings.TrimSpace(src.StatusCode) == "01" {
			return RegistrationStatusActive
		}
		return RegistrationStatusUnknown
	}
	if status, ok := companyStatuses[strings.TrimSpace(src.StatusText)]; ok {
		return status
	}
	return RegistrationStatusUnknown
}

// NewRegistration 把資料來源（platform/gcis）給的原始登記資料轉成模組的型別：互動查詢（統編帶入表單）與 KYC 送件的查詢快照共用
// 這一個入口，所以下面三件事只有一份——現況分類（classifyRegistrationStatus）、停業期間的合理性（迄日早於起日的期間意思不明，兩個
// 日期都不採用）、地址拆成鄉鎮市區代碼與明細（platform/twaddr 的 Split，拆不開時兩者是空字串）。
func NewRegistration(src gcis.Registration) Registration {
	reg := Registration{
		TaxID: src.TaxID, Name: src.Name, Address: src.Address, SetupDate: src.SetupDate, OrganizationType: src.OrganizationType,
		Status: classifyRegistrationStatus(src), StatusText: src.StatusText,
		SuspendedFrom: src.SuspendedFrom, SuspendedUntil: src.SuspendedUntil,
	}
	if !reg.SuspendedFrom.IsZero() && !reg.SuspendedUntil.IsZero() && reg.SuspendedUntil.Before(reg.SuspendedFrom) {
		reg.SuspendedFrom, reg.SuspendedUntil = time.Time{}, time.Time{}
	}
	if code, detail, ok := twaddr.Split(src.Address); ok {
		reg.TownCode, reg.AddressDetail = code, detail
	}
	return reg
}

// ValidRegistrationField 回報 field 是不是登記資料會比對的欄位。
func ValidRegistrationField(field string) bool {
	return slices.Contains(RegistrationComparedFields(), field)
}
