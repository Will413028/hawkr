package usecase

import (
	"context"
	"encoding/json"
)

// PayuniClient defines the interface for PAYUNi merchant registration operations.
type PayuniClient interface {
	BindSeller(ctx context.Context, req *BindSellerRequest) (*PayuniResponse, error)
	GetSellerStatus(ctx context.Context, merID string) (*SellerStatusResponse, error)
	UpdateMerchant(ctx context.Context, req *UpdateMerchantRequest) (*PayuniResponse, error)
}

// BindSellerRequest contains all data needed to register a merchant with PAYUNi.
type BindSellerRequest struct {
	// SellerID is PayUni's MerID (format "HAWK{store_id}") — goes in the
	// MerID field of the bind_merchant call.
	SellerID string `json:"seller_id"`
	// StoreID is the raw internal store_id (no "HAWK" prefix). Informational
	// only — populated for registration bookkeeping, but not currently read
	// by any URL builder; the public storefront URL (MerURL) is built from
	// StoreCode below, not StoreID. Kept for possible future use.
	StoreID string `json:"store_id"`
	// StoreCode is the seller's public display code (YY+8 digits), used to
	// build the public storefront URL (MerURL) at /sellers/{store_code} on
	// the web app since 2026-07-23. Must NOT reuse SellerID (PayUni MerID) —
	// that 404s the URL PayUni's manual review visits.
	StoreCode        string `json:"store_code"`
	WebsiteURL       string `json:"website_url"`
	SellerType       string `json:"seller_type"`
	RealName         string `json:"real_name"`
	IDCardNo         string `json:"id_card_no"`
	CompanyName      string `json:"company_name"`
	TaxID            string `json:"tax_id"`
	CompanyPhone     string `json:"company_phone"`
	CompanyAddress   string `json:"company_address"`
	EnglishName      string `json:"english_name"`
	Email            string `json:"email"`
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
	ContactPhone     string `json:"contact_phone"`
	Address          string `json:"address"`
	PostalCode       string `json:"postal_code"`
}

// UpdateMerchantRequest is the usecase-level mirror of the PAYUni
// update_merchant allowlist (issue #2806 Phase 2). MerID is resolved
// server-side from the caller's store_id — it is never accepted from an
// HTTP/gRPC request. Deliberately does not declare Payment/Fee/PaymentOff/
// CloseApplySwitch/OpenApplySwitch.
type UpdateMerchantRequest struct {
	MerID         string
	ContactName   string
	ContactMobile string
	ContactTel    string
	MerEmail      string
	MerAddr       string
	MerAddrPost   string
	MerTel        string
}

// PayuniResponse represents a generic PAYUNi API response.
//
// PayUni uses the `Status` field for both success ("SUCCESS") and the error
// code (e.g. "AGENT04017") — the Chinese description goes in `Message`. The
// adapter MUST propagate the raw Status into `ErrorCode` on failure so the
// usecase layer can classify errors without string-matching the localized
// message.
type PayuniResponse struct {
	Message     string
	MerID       string
	ErrorCode   string // PayUni error code from Status field (e.g. "AGENT04017"). Empty on success or transport error.
	ErrorMsg    string
	RawResponse json.RawMessage
	Success     bool
}

// SellerStatusResponse represents the merchant status from PAYUNi.
type SellerStatusResponse struct {
	MerID           string
	MerStatus       string
	Reason          string // 商店審核未通過原因, propagated to MQ verification events
	MemberStatus    string
	CreditStatus    string
	ApplePayStatus  string
	GooglePayStatus string
	ATMStatus       string
	RawResponse     json.RawMessage
}
