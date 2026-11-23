package model

import (
	"time"

	"uuid"
)

// 賣家回價一個版本的狀態（等於 proto SellerResponseStatus 的值名去掉前綴）。
const (
	ResponseActive     = "ACTIVE"
	ResponseSuperseded = "SUPERSEDED"
	ResponseWithdrawn  = "WITHDRAWN"
	ResponseExpired    = "EXPIRED"
)

// 買家能不能接受一份回價（等於 proto QuoteAcceptability 的值名去掉前綴）。
const (
	AcceptabilityAcceptable = "ACCEPTABLE"
	AcceptabilityExpired    = "EXPIRED"
	AcceptabilityWithdrawn  = "WITHDRAWN"
	AcceptabilitySuperseded = "SUPERSEDED"
	AcceptabilityIneligible = "INELIGIBLE"
)

// 接受報價之後建單的進度（等於 proto OrderProvisioningState 的值名去掉前綴，也是 buyer_quote_acceptances 的 CHECK 值）。
const (
	ProvisioningCreating     = "CREATING_ORDER"
	ProvisioningOrderCreated = "ORDER_CREATED"
	ProvisioningOrderFailed  = "ORDER_FAILED"
)

// SellerQuote 是賣家回價一個版本的報價內容；數量是固定三位小數的十進位字串。
type SellerQuote struct {
	ResponseType        string
	QualityGrade        string
	InspectionNotes     string
	UnitBasisQuantity   string
	UnitBasisUnit       string
	OfferedQuantity     string
	OfferedQuantityUnit string
	ConversionBasis     string
	Currency            string
	GoodsAmount         int64
	ShippingAmount      int64
	TaxAmount           int64
	TaxInclusionBasis   string
	TotalAmount         int64
	LeadTimeDays        *int32
	PromisedDelivery    *time.Time
	ValidUntil          time.Time
	TermsNote           string
	DeviationDisclosure string
}

// BuyerSellerResponse 是買家看到的一份賣家回價（目前版本）。
type BuyerSellerResponse struct {
	ID          uuid.UUID
	Version     int32
	Status      string
	PayloadHash string
	SubmittedAt time.Time
	StoreID     int64
	Quote       SellerQuote
	// OutsideBuyerRequestedRange 對發布核准凍結的單價區間現算（A9-D35(2)）。
	OutsideBuyerRequestedRange bool
	Shortlisted                bool
	Acceptability              string
}

// SellerResponsePage 是 ListSellerResponses 的一頁。
type SellerResponsePage struct {
	Responses     []BuyerSellerResponse
	NextPageToken string
}

// ShortlistEntry 是入圍清單的現況：Present 為 true 表示這則回價在清單上，Version 是掛著的版本。
type ShortlistEntry struct {
	SellerResponseID uuid.UUID
	Version          int32
	Present          bool
}

// Acceptance 是買家接受報價的結果；Replayed 為 true 表示這是冪等的重送，沒有建立新的接受單。
type Acceptance struct {
	ID                uuid.UUID
	SellerResponseID  uuid.UUID
	Version           int32
	AcceptedDealPrice int64
	AcceptedAt        time.Time
	Provisioning      string
	OrderNo           *string
	PaymentDeadline   *time.Time
	Replayed          bool
}
