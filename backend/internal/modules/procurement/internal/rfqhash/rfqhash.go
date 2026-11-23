// Package rfqhash 算詢價確認快照的 hash（A9-D31 Q5）：S5 刪掉所有 hash 欄，hash 由不可變的列在 Go 重算。
// 演算法是 SHA-256 對固定欄位順序的 JSON（struct 欄位順序即輸出順序，沒有 map，所以確定），原像（preimage）以發布的 schema_version 為版本：
// 原像包含 schemaVersion，欄位集合依版本分派，不認得的版本回錯誤。同樣的版本與輸入永遠得到同一個值，改任何一個欄位 hash 就變。
// 新增或改變原像的欄位必須新增一個版本（新的原像型別與分派分支），不得更動既有版本的欄位集合，否則已發布的詢價 hash 會變；每個版本有 golden 測試。
// A9-3c 的 ConfirmRfqSnapshot 以同一個函式算 payload_hash，讀取端（PublishedRfq.rfq_snapshot_hash）與它必須是同一個值。
package rfqhash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaV1 是第一版原像的 schema_version。
const SchemaV1 = "v1"

// Input 是確認快照的內容：發布的 schema_version、快照本身、它確認的需求版本與價格預覽。ID 是 UUID 的字串形式。
type Input struct {
	SchemaVersion          string
	ConfirmedRfqSnapshotID string
	RequestRevisionID      string
	PricePreviewID         string
	BuyerNote              string
	Requirement            Slots
	Price                  Price
}

// Slots 是需求版本的欄位。日期是 YYYY-MM-DD；沒有值的欄位輸出 null。
type Slots struct {
	Item               *string
	Quantity           *int32
	Unit               *string
	Packaging          *string
	BudgetPerUnit      *int64
	DeliveryCity       *string
	DeliveryDate       *string
	ExactMatchRequired *bool
}

// Price 是價格預覽：單價區間與依據的數量單位，以及總價區間（沒有總價時為 null）。
type Price struct {
	RangeLowerAmount int64
	RangeUpperAmount int64
	BasisQuantity    int32
	BasisUnit        string
	TotalLowerAmount *int64
	TotalUpperAmount *int64
	TotalQuantity    *int32
	TotalUnit        *string
}

// Date 把日期欄位轉成 Slots.DeliveryDate 的字串。
func Date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

// Snapshot 回傳快照的 hash：SHA-256 的小寫十六進位。schema_version 不認得回錯誤。
func Snapshot(in Input) (string, error) {
	var pre any
	switch in.SchemaVersion {
	case SchemaV1:
		pre = preimageV1(in)
	default:
		return "", fmt.Errorf("rfqhash：不認得的 schema_version %q", in.SchemaVersion)
	}
	b, err := json.Marshal(pre)
	if err != nil {
		return "", fmt.Errorf("rfqhash：序列化：%w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// preimageV1 是 v1 原像的欄位集合與順序，固定不變。
func preimageV1(in Input) v1 {
	return v1{
		SchemaVersion: in.SchemaVersion, ConfirmedRfqSnapshotID: in.ConfirmedRfqSnapshotID, RequestRevisionID: in.RequestRevisionID,
		PricePreviewID: in.PricePreviewID, BuyerNote: in.BuyerNote, Requirement: v1Slots(in.Requirement), Price: v1Price(in.Price),
	}
}

type v1 struct {
	SchemaVersion          string  `json:"schemaVersion"`
	ConfirmedRfqSnapshotID string  `json:"confirmedRfqSnapshotId"`
	RequestRevisionID      string  `json:"requestRevisionId"`
	PricePreviewID         string  `json:"pricePreviewId"`
	BuyerNote              string  `json:"buyerNote"`
	Requirement            v1Slots `json:"requirement"`
	Price                  v1Price `json:"price"`
}

type v1Slots struct {
	Item               *string `json:"item"`
	Quantity           *int32  `json:"quantity"`
	Unit               *string `json:"unit"`
	Packaging          *string `json:"packaging"`
	BudgetPerUnit      *int64  `json:"budgetPerUnit"`
	DeliveryCity       *string `json:"deliveryCity"`
	DeliveryDate       *string `json:"deliveryDate"`
	ExactMatchRequired *bool   `json:"exactMatchRequired"`
}

type v1Price struct {
	RangeLowerAmount int64   `json:"rangeLowerAmount"`
	RangeUpperAmount int64   `json:"rangeUpperAmount"`
	BasisQuantity    int32   `json:"basisQuantity"`
	BasisUnit        string  `json:"basisUnit"`
	TotalLowerAmount *int64  `json:"totalLowerAmount"`
	TotalUpperAmount *int64  `json:"totalUpperAmount"`
	TotalQuantity    *int32  `json:"totalQuantity"`
	TotalUnit        *string `json:"totalUnit"`
}
