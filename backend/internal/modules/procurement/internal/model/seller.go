package model

import (
	"time"

	"uuid"
)

// 賣家寫入的上限（字元數，proto 與 DB CHECK 同值）。
const (
	MaxSpecNoteRunes            = 1000
	MaxTermsNoteRunes           = 1000
	MaxDeviationDisclosureRunes = 1000
	MaxConversionBasisRunes     = 500
	MaxQuoteUnitRunes           = 40
	MaxQuantityTextRunes        = 32
	MaxReasonCodeRunes          = 64
	// MaxDraftBytes 是一份草稿序列化後的上限（與 seller_response_drafts_payload_check 同值）。
	MaxDraftBytes = 16384
	// Currency 是賣家回價的幣別：目前只有新台幣。
	Currency = "TWD"
)

// 賣家回價的方式、稅的關係與停止寫入原因的代碼（proto enum 值名去掉前綴，也是 seller_response_versions 與 seller_responses 的 CHECK 值）。
const (
	ResponseTypeManualQuote   = "MANUAL_QUOTE"
	ResponseTypeAcceptNonTerm = "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE"
	TaxIncluded               = "TAX_INCLUDED"
	TaxExcluded               = "TAX_EXCLUDED"
	WriteClosedDeclined       = "DECLINED"
)

// QuoteInput 是賣家填的報價內容（草稿與正式回價共用的輸入）：欄位照使用者填的原樣，沒填的是零值；草稿以這個形狀存成 JSONB（零值省略）。
// 正式回價送出時由 sellerside 驗證並正規化成 SellerQuote。
type QuoteInput struct {
	ResponseType        string     `json:"responseType,omitempty"`
	SpecNote            string     `json:"specNote,omitempty"`
	UnitBasisQuantity   string     `json:"unitBasisQuantity,omitempty"`
	UnitBasisUnit       string     `json:"unitBasisUnit,omitempty"`
	OfferedQuantity     string     `json:"offeredQuantity,omitempty"`
	OfferedQuantityUnit string     `json:"offeredQuantityUnit,omitempty"`
	ConversionBasis     string     `json:"conversionBasis,omitempty"`
	GoodsAmount         int64      `json:"goodsAmount,omitempty"`
	ShippingAmount      int64      `json:"shippingAmount,omitempty"`
	TaxAmount           int64      `json:"taxAmount,omitempty"`
	TaxInclusionBasis   string     `json:"taxInclusionBasis,omitempty"`
	TotalAmount         int64      `json:"totalAmount,omitempty"`
	LeadTimeDays        *int32     `json:"leadTimeDays,omitempty"`
	PromisedDelivery    *time.Time `json:"promisedDelivery,omitempty"`
	ValidUntil          *time.Time `json:"validUntil,omitempty"`
	TermsNote           string     `json:"termsNote,omitempty"`
	DeviationDisclosure string     `json:"deviationDisclosure,omitempty"`
}

// SellerPublicationSummary 是賣家看到的一次發布的摘要；Version 與 SellerFacingHash 是賣家送出報價時要帶回的版本與 hash。
type SellerPublicationSummary struct {
	PublicationID    uuid.UUID
	Version          int32
	SellerFacingHash string
	DeliveryState    string
	PublishedAt      time.Time
}

// SellerDraft 是賣家自己的回價草稿。
type SellerDraft struct {
	Version   int32
	UpdatedAt time.Time
	Quote     QuoteInput
}

// SellerResponseVersion 是賣家送出的一個正式回價版本；SupersededByVersion 為 0 表示仍是最新版。
type SellerResponseVersion struct {
	ResponseID          uuid.UUID
	Version             int32
	PublicationID       uuid.UUID
	Status              string
	PayloadHash         string
	SubmittedAt         time.Time
	SupersededByVersion int32
	OutsideRange        bool
	Quote               SellerQuote
	Replayed            bool
}

// SellerPublicationItem 是賣家的詢價清單的一列；LatestResponseStatus 為 nil 表示還沒有正式回價。
type SellerPublicationItem struct {
	Summary              SellerPublicationSummary
	Payload              SellerFacing
	LatestResponseStatus *string
	HasDraft             bool
}

// SellerPublicationPage 是 ListSellerPublications 的一頁。
type SellerPublicationPage struct {
	Items         []SellerPublicationItem
	NextPageToken string
}

// SellerPublicationDetail 是 GetSellerPublication 的結果；Draft 與 Latest 沒有時為 nil。
type SellerPublicationDetail struct {
	Summary SellerPublicationSummary
	Payload SellerFacing
	Draft   *SellerDraft
	Latest  *SellerResponseVersion
}

// ResponseClosure 是回價被撤回或婉拒之後的結案狀態；WithdrawnVersion 為 0 表示沒有撤回，WriteClosedAt 為 nil 表示仍可寫入。
type ResponseClosure struct {
	ResponseID        uuid.UUID
	PublicationID     uuid.UUID
	LatestVersion     int32
	WithdrawnVersion  int32
	WriteClosedAt     *time.Time
	WriteClosedReason string
}
