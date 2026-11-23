// Package rfqhash 算詢價確認快照的 hash（A9-D31 Q5）：S5 刪掉所有 hash 欄，hash 由不可變的列在 Go 重算。
// 演算法是 SHA-256 對固定欄位順序的 JSON（struct 欄位順序即輸出順序，沒有 map，所以確定）；
// 同樣的輸入永遠得到同一個值，改任何一個欄位 hash 就變。A9-3c 的 ConfirmRfqSnapshot 以同一個函式算 payload_hash，
// 讀取端（PublishedRfq.rfq_snapshot_hash）與它必須是同一個值；要加欄位只能在這裡加，並讓測試的 golden 值跟著改。
package rfqhash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Input 是確認快照的內容：快照本身、它確認的需求版本與價格預覽。ID 是 UUID 的字串形式。
type Input struct {
	ConfirmedRfqSnapshotID string `json:"confirmedRfqSnapshotId"`
	RequestRevisionID      string `json:"requestRevisionId"`
	PricePreviewID         string `json:"pricePreviewId"`
	BuyerNote              string `json:"buyerNote"`
	Requirement            Slots  `json:"requirement"`
	Price                  Price  `json:"price"`
}

// Slots 是需求版本的欄位。日期是 YYYY-MM-DD；沒有值的欄位輸出 null。
type Slots struct {
	Item               *string `json:"item"`
	Quantity           *int32  `json:"quantity"`
	Unit               *string `json:"unit"`
	Packaging          *string `json:"packaging"`
	BudgetPerUnit      *int64  `json:"budgetPerUnit"`
	DeliveryCity       *string `json:"deliveryCity"`
	DeliveryDate       *string `json:"deliveryDate"`
	ExactMatchRequired *bool   `json:"exactMatchRequired"`
}

// Price 是價格預覽：單價區間與依據的數量單位，以及總價區間（沒有總價時為 null）。
type Price struct {
	RangeLowerAmount int64   `json:"rangeLowerAmount"`
	RangeUpperAmount int64   `json:"rangeUpperAmount"`
	BasisQuantity    int32   `json:"basisQuantity"`
	BasisUnit        string  `json:"basisUnit"`
	TotalLowerAmount *int64  `json:"totalLowerAmount"`
	TotalUpperAmount *int64  `json:"totalUpperAmount"`
	TotalQuantity    *int32  `json:"totalQuantity"`
	TotalUnit        *string `json:"totalUnit"`
}

// Date 把日期欄位轉成 Slots.DeliveryDate 的字串。
func Date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

// Snapshot 回傳快照的 hash：SHA-256 的小寫十六進位。
func Snapshot(in Input) (string, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("rfqhash：序列化：%w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
