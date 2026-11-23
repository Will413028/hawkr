package model

import (
	"time"

	"uuid"
)

// RequestSnapshot 是 GetSnapshot 的讀取模型：以需求為鍵，目前版本的欄位加上 run、中斷與發布結果。
// 狀態、run 類型是 proto enum 值名去掉前綴的代碼。
type RequestSnapshot struct {
	RequestID        uuid.UUID
	RevisionID       uuid.UUID
	Status           string
	Slots            RequirementSlots
	MissingRequired  []SlotKey
	Runs             []RunSummary
	ActiveRunID      *uuid.UUID
	OpenConfirmation *OpenConfirmation
	PublishedRfq     *PublishedRfq
}

// RequirementSlots 是需求版本的欄位；沒有值的欄位是 nil。
type RequirementSlots struct {
	Item               *string
	Quantity           *int32
	Unit               *string
	Packaging          *string
	BudgetPerUnit      *int64
	DeliveryCity       *string
	DeliveryDate       *time.Time
	ExactMatchRequired *bool
	ItemResolution     *ItemResolution
}

// ItemResolution 是品項原話判定成分類樹的哪個節點。Kind 是 proto ItemResolutionKind 值名去掉前綴的代碼；DisplayValue 是伺服器組好的顯示字串。
type ItemResolution struct {
	Kind         string
	Label        string
	DisplayValue string
}

// ConversationSummary 是對話列表的一列。
type ConversationSummary struct {
	ID                 uuid.UUID
	Title              string
	LastMessagePreview string
	UpdateTime         time.Time
}

// ConversationPage 是對話列表的一頁；NextPageToken 為空字串表示最後一頁。
type ConversationPage struct {
	Conversations []ConversationSummary
	NextPageToken string
}

// ConversationMessages 是 GetConversationMessages 的讀取模型，訊息由舊到新。
type ConversationMessages struct {
	ConversationID uuid.UUID
	RequestID      uuid.UUID
	Status         string
	Title          string
	Messages       []ConversationMessage
}

// CandidateSearch 是 GetCandidateSearch 的讀取模型：run 的狀態（RunStatus 代碼）、失敗時的原因（RunFailureReason 代碼）與候選。
// 只有 SUCCEEDED 的搜尋有候選；其餘狀態 Candidates 為空。CandidateSetID 從 run 建立時就有。
type CandidateSearch struct {
	RequestID      uuid.UUID
	RunID          uuid.UUID
	CandidateSetID uuid.UUID
	Status         string
	FailureReason  string
	Candidates     []Candidate
}
