package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Lane 是候選來源通道：PLATFORM（product-service 平台商品）或 EXTERNAL（seller-service 的
// seller_prospects 站外業者主檔）。
type Lane string

const (
	LanePlatform Lane = "PLATFORM"
	LaneExternal Lane = "EXTERNAL"
)

// MatchFact 是候選與需求之間一個「可驗證」的配對事實，呈現在候選卡上。
type MatchFact struct {
	Key   SlotKey `json:"key"`
	Label string  `json:"label"`
}

// UnverifiableFact 是一個「無法驗證」的需求維度，依 D1 動態組出具體文案
// （不是固定字串「需向賣家確認」——那句話沒有告訴買家該確認什麼）。
type UnverifiableFact struct {
	Key    SlotKey `json:"key"`
	Reason string  `json:"reason"`
}

var slotLabel = map[SlotKey]string{
	SlotItem:               "品項",
	SlotQuantity:           "數量",
	SlotUnit:               "單位",
	SlotPackaging:          "包裝",
	SlotBudgetPerUnit:      "單價預算",
	SlotDeliveryCity:       "配送城市",
	SlotDeliveryDate:       "到貨日期",
	SlotExactMatchRequired: "是否需完全相符",
}

// MatchFactFor 回傳某個 slot 在候選卡上顯示用的配對事實（命中時使用）。
func MatchFactFor(key SlotKey) MatchFact {
	return MatchFact{Key: key, Label: slotLabel[key]}
}

// NeedConfirmReason 依 D1（procurement-candidate-matching-a3 ADR）為每個無法驗證的維度
// 動態組出具體文案；slot 未填時退回通用標籤，不印出零值或 panic。
func NeedConfirmReason(key SlotKey, s Slots) string {
	switch key {
	case SlotQuantity:
		if s.Quantity != nil {
			unit := ""
			if s.Unit != nil {
				unit = " " + *s.Unit
			}
			return fmt.Sprintf("需向賣家確認是否備有 %d%s 的供應量", *s.Quantity, unit)
		}
	case SlotDeliveryCity:
		if s.DeliveryCity != nil {
			return fmt.Sprintf("需向賣家確認是否配送至%s", *s.DeliveryCity)
		}
	case SlotDeliveryDate:
		if s.DeliveryDate != nil {
			return fmt.Sprintf("需向賣家確認能否於 %s 前到貨", *s.DeliveryDate)
		}
	case SlotBudgetPerUnit:
		// 用買家自己填的金額與單位陳述，不宣稱平台已經照這個價篩過——平台手上的
		// 商品售價沒有可換算的重量基準（見 candidate_platform_lane.go 的說明）。
		if s.BudgetPerUnit != nil {
			unit := ""
			if s.Unit != nil {
				unit = "／" + *s.Unit
			}
			return fmt.Sprintf("需向賣家確認能否符合 %d 元%s 的單價", *s.BudgetPerUnit, unit)
		}
	}
	return "需向賣家確認" + slotLabel[key]
}

// EvidenceExternalUnverified 是常設揭露那一條 UnverifiableFact 的 key。
//
// 它走 SlotKey 這個既有型別，但**不是一個買家填答維度**——同 SlotItemResolution／
// SlotCropDisambiguation 兩個既有值，這個型別在候選卡這條路徑上承載的是「這一列在
// 講什麼」，不是「Slots 結構有哪些欄位」。值刻意不與任何真實 slot 相撞（見
// TestEvidenceExternalUnverified_IsNotASlot）：撞了會讓前端把來源揭露誤讀成某個
// 維度的缺口。
const EvidenceExternalUnverified SlotKey = "external_unverified"

// externalUnverifiedReason 是 doc 57 §1.6 明文指定的文案，改動要同步回該文件。
const externalUnverifiedReason = "外部名單，供應能力未經平台驗證，以賣家回覆為準"

// ExternalLaneDisclosure 是 EXTERNAL lane 每一張候選卡都必須攜帶的常設揭露
// （doc 57 §1.6）。
//
// 「EXTERNAL lane 不做事前驗證」是 2026-08-26 的決策：站外賣家只寄詢價信，不回覆
// 即自我篩除，email 往返本身就是篩選器。這個策略的代價是買家清單看起來像經過篩選
// 而其實沒有——所以這句揭露不是補充說明，是那個策略成立的**前提**。少了它，一個
// 只填品項的查詢會讓站外候選帶著空的 unverifiableFacts 回到前端，候選卡渲染成
// 「沒有額外待確認項目」（CandidateBrowser.tsx 的空列表分支），等於對一批完全沒有
// 驗證過的名單宣稱它沒有待確認事項。
func ExternalLaneDisclosure() UnverifiableFact {
	return UnverifiableFact{Key: EvidenceExternalUnverified, Reason: externalUnverifiedReason}
}

// ValidateCandidateEvidence 是 doc 57 §1.6 evidence contract 的記錄層閘門：候選被
// 呈現之前對三欄做 accept/reject。
//
// 現行規則刻意只擋「結構上不可能被正確渲染」的形狀：
//
//   - matchedFacts／unverifiableFacts 為 JSON null 或空 bytes——前端對 null 做
//     `.length` 會直接爆（兩條 lane 各自的「空的非 nil slice」註解講的就是這件事，
//     這裡是那個約定失效時的防線，不是取代它）。
//   - factsAsOf 為零值——候選卡會渲染出 0001-01-01，比不顯示更糟：那是一個看起來
//     像真的、但完全錯誤的資料時點宣稱。
//
// **刻意不擋空陣列。** doc 57 §1.6 的「不得只是空陣列」只對 EXTERNAL 明文要求常設
// 揭露（由 ExternalLaneDisclosure 從建構面保證），PLATFORM 半邊要不要也常設一條
// 來源揭露屬未定的產品決策（Will 2026-09-06 裁決：需求還會變，先寬鬆、之後好改）。
// 要收緊時改這一支函式，兩條 lane 的呼叫端都不必動。
func ValidateCandidateEvidence(matchedFactsJSON, unverifiableFactsJSON []byte, factsAsOf time.Time) error {
	if err := requireJSONArray("matchedFacts", matchedFactsJSON); err != nil {
		return err
	}
	if err := requireJSONArray("unverifiableFacts", unverifiableFactsJSON); err != nil {
		return err
	}
	if factsAsOf.IsZero() {
		return fmt.Errorf("evidence contract: factsAsOf 為零值（doc 57 §1.6：必須是資料來源時點）")
	}
	return nil
}

// requireJSONArray 只驗「這是不是一個 JSON 陣列」，不解析內容——內容形狀由各 lane
// 自己的型別保證，這一層要擋的是序列化層的 null。
func requireJSONArray(field string, raw []byte) error {
	var probe []json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("evidence contract: %s 不是 JSON 陣列（doc 57 §1.6 保證非 null）: %w", field, err)
	}
	if probe == nil {
		return fmt.Errorf("evidence contract: %s 為 null（doc 57 §1.6 保證非 null）", field)
	}
	return nil
}
