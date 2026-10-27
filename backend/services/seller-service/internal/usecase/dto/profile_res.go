package dto

import "time"

type SellerShippingProfile struct {
	StoreID       string
	SenderName    string
	SenderPhone   string
	City          string
	District      string
	AddressDetail string
	ZipCode       string
	MerID         string
	ContactEmail  string
}

type SellerProfile struct {
	CertifiedAt                *time.Time
	CurrentCertificationID     *int64
	Prize                      string
	CoverKey                   string
	ContactPhone               string
	City                       string
	District                   string
	AddressDetail              string
	PostalCode                 string
	Birthday                   string
	Gender                     string
	Phone                      string
	Email                      string
	Logo                       string
	SellerType                 string
	Nickname                   string
	ApprovalStatus             string
	OverallCertificationStatus string
	StoreName                  string
	Description                string
	SeoTitle                   string
	SeoDescription             string
	WebsiteURL                 string
	FoodBusinessRegNo          string
	Status                     string
	Rating                     float64
	PayuniStatus               int
	// PayuniRegistrationStatus 是 payuni_registrations.registration_status
	// （active/pending/failed），決定賣家能不能收款，與上面那個 mer_status 轉的 int 不同。
	PayuniRegistrationStatus string
	AccountID                int64
	// StoreID is the seller aggregate id (the id embedded in the PAYUNi
	// MerID). Grandfathered == AccountID today; diverges on a store delete+reopen
	// incarnation (Plan 3). Callers that need the current PayUni registration must
	// key on this, not AccountID.
	StoreID int64
	// StoreCode is the display code exposed to end users (YY+8 亂數，10 碼,
	// domain.GenerateStoreCode) — replaces surfacing StoreID directly.
	StoreCode int64
	// Version is the optimistic-lock version the delegated storefront editor
	// reads and sends back as expectedVersion (56 §6.1.7).
	Version             int64
	AccountType         int32
	OnboardingCompleted bool
	IsProcessedFood     bool
}
