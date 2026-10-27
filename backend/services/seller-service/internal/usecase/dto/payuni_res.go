package dto

import "time"

type PayuniStatus struct {
	PayuniMerID     string
	Status          string
	MerStatus       string
	MemberStatus    string
	CreditStatus    string
	Reason          string // PayUni 商店審核未通過/待補件原因（≤200 字），無則為空字串
	ApplePayStatus  string
	GooglePayStatus string
	ATMStatus       string
	ErrorMessage    string
	Freshness       string
	AccountID       int64
	PaymentEnabled  bool
	RegisteredAt    *time.Time
	ApprovedAt      *time.Time
	LastSyncedAt    *time.Time
	AllowedActions  []string
	// MerchantProfileVersion and the 7 fields below mirror
	// domain.PayuniRegistration's merchant-profile columns (issue #2806
	// Phase 2), exposed read-only here so the Web edit form can GET the
	// current values + version before PATCHing.
	MerchantProfileVersion int64
	ContactName            string
	ContactMobile          string
	ContactTel             string
	MerEmail               string
	MerAddr                string
	MerAddrPost            string
	MerTel                 string
	// 以下 4 個欄位（2026-09-10 賣家可讀錯誤目錄）：由 usecase.PayuniErrorDisplayFor
	// 依當前 MerStatus/MemberStatus 解析，不是 domain.PayuniRegistration.ErrorCode
	// 的直接鏡射——見該函式 doc comment。ErrorMessage 是內部欄位，不受影響。
	ErrorCode           string
	ErrorDisplayMessage string
	ErrorRemedy         string
	RetryStep           string
}
