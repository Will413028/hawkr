package model

import (
	"time"

	"uuid"
)

// PublicationStatePublished 是發布目前唯一的狀態（rfq_publications.state 的 CHECK）。
const PublicationStatePublished = "PUBLISHED"

// QuoteTotal 是依單價區間與需求數量算出的詢價總額區間；Quantity 與 Unit 是計算採用的數量與計價單位。
type QuoteTotal struct {
	LowerBoundAmount, UpperBoundAmount int64
	Quantity                           int32
	Unit                               string
}

// PriceQuote 是 PreviewRfqPrice 的結果（純計算，不存）：單價區間與計價基準（BasisQuantity 個 BasisUnit）、總額區間（需求沒有數量時為 nil）。
type PriceQuote struct {
	LowerBoundAmount, UpperBoundAmount int64
	BasisQuantity                      int32
	BasisUnit                          string
	Total                              *QuoteTotal
}

// SellerFacing 是賣家看得到的詢價內容（白名單欄位）；日期欄位是當日（UTC 午夜）。
type SellerFacing struct {
	Item, ItemLabel            string
	Quantity                   *int32
	Unit, Packaging            *string
	DeliveryCity               string
	DeliveryDate, ResponseDate *time.Time
	BuyerNote                  string
	BasisQuantity              int32
	BasisUnit                  string
	LowerBoundAmount           int64
	UpperBoundAmount           int64
}

// PublicationPreview 是 PreviewRfqPublication 的結果。
type PublicationPreview struct {
	ApprovalID       uuid.UUID
	ApprovalVersion  int32
	AudienceHash     string
	SellerFacingHash string
	Payload          SellerFacing
	ExpiresAt        time.Time
	RecipientCount   int32
}

// Publication 是 ExecuteRfqPublication 的結果；Replayed 為 true 表示這是冪等的重送，沒有建立新的發布。
type Publication struct {
	ID               uuid.UUID
	Version          int32
	State            string
	AudienceHash     string
	SellerFacingHash string
	PublishedAt      time.Time
	Replayed         bool
}

// RfqInvite 是發布時發給一家店的邀請。
type RfqInvite struct {
	InviteID uuid.UUID
	StoreID  int64
}

// RfqPublished 在詢價發布成立的 transaction 內發出（ExecuteRfqPublication）：發布與邀請都已寫入，通知（A9-3c 的 relay 與 A8）消費它。
// 邀請列就是賣家可見性的唯一依據（A9-D44）。重送冪等的發布不再發。
type RfqPublished struct {
	PublicationID, RequestID, ApprovalID uuid.UUID
	BuyerID                              int64
	PublishedAt                          time.Time
	Invites                              []RfqInvite
}
