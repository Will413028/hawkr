// Package model 放 procurement 對買家與賣家的型別。
package model

import (
	"time"

	"uuid"
)

// SlotKey 是需求欄位的代碼，值等於 proto SlotKey 的值名（去掉前綴）。
type SlotKey string

// MatchedFact 是候選賣家符合買家某個欄位的一項事實；Label 是伺服器的固定文字。
type MatchedFact struct {
	SlotKey SlotKey
	Label   string
}

// UnverifiableFact 是無法驗證、需向賣家確認的一項事實；Reason 是伺服器依買家填的值組出來的說明。
type UnverifiableFact struct {
	SlotKey SlotKey
	Reason  string
}

// Candidate 是買家回應裡的候選賣家（A9-D8、AD7）：只有結構化欄位。店名由呼叫端以 StoreID 讀 StoreCard 組裝，不存快照（A9-D19 S9）。
// 這個型別不得出現研究摘要、定位等自由文字；TestCandidateCarriesNoFreeText 以反射守住，唯一的例外是下面兩個伺服器產生的欄位。
type Candidate struct {
	ID                uuid.UUID
	StoreID           int64
	Score             int
	MatchedFacts      []MatchedFact
	UnverifiableFacts []UnverifiableFact
	FactsAsOf         time.Time
	Selected          bool
}

// LLMCandidate 是給 LLM 看的候選：在買家看得到的欄位之外多了研究摘要等自由文字（A9-D8：自由文字只給 LLM）。
// 它不得被序列化進買家回應；轉成買家回應只經 ForBuyer。
type LLMCandidate struct {
	Candidate
	ResearchSummary string
}

// ForBuyer 回傳買家看得到的部分，丟掉所有自由文字。
func (c LLMCandidate) ForBuyer() Candidate { return c.Candidate }
