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
// 事實不入庫：MatchedFacts、UnverifiableFacts 讀取時由需求的現行版本組出（A9-D29(3)），FactsAsOf 是這次搜尋完成的時間。
// 這個型別不得出現研究摘要、定位等自由文字；TestCandidateCarriesNoFreeText 以反射守住，唯一的例外是下面兩個伺服器產生的欄位。
type Candidate struct {
	ID                uuid.UUID
	StoreID           int64
	Rank              int
	MatchedFacts      []MatchedFact
	UnverifiableFacts []UnverifiableFact
	FactsAsOf         time.Time
	Selected          bool
}
