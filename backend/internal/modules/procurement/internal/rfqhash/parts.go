// Package rfqhash 算詢價發布鏈的 hash（A9-D31 Q5、A9-D35）：S5 刪掉所有 hash 欄，hash 由不可變的列在 Go 重算。
// 演算法是 SHA-256 對固定欄位順序的 JSON（struct 欄位順序即輸出順序，沒有 map，所以確定），原像（preimage）以發布核准的 schema_version 為版本：
// 原像包含 schemaVersion，欄位集合依版本分派，不認得的版本回錯誤。同樣的版本與輸入永遠得到同一個值，改任何一個欄位 hash 就變。
// 新增或改變原像的欄位必須新增一個版本（新的原像型別與分派分支），不得更動既有版本的欄位集合，否則已發布的詢價 hash 會變；每個版本有 golden 測試。
// 三種 hash：受眾（Audience）、賣家可見內容（SellerFacing）與賣家回價一個版本的報價內容（SellerResponse，A9-D36，版本固定 SchemaV2，不含 ID、版本號與送出時間）。各自一個型別，不共用欄位，所以一個 hash 不會被當成另一個的值。
// 賣家可見內容的 hash 是 AcceptSellerQuote 比對的對象，原像涵蓋賣家看到的全部欄位（ADR 2026-11-23-seller-facing-hash-supersedes-snapshot-hash）。
package rfqhash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// AudienceInput 是核准的受眾：模式與指定對象的店家（順序有意義）。
type AudienceInput struct {
	SchemaVersion string
	Mode          string
	StoreIDs      []int64
}

// Audience 回傳受眾的 hash：SHA-256 的小寫十六進位。店家順序不同、少一家或多一家 hash 都不同；沒有對象時 stores 輸出空陣列（不是 null）。
func Audience(in AudienceInput) (string, error) {
	switch in.SchemaVersion {
	case SchemaV1:
		ids := in.StoreIDs
		if ids == nil {
			ids = []int64{}
		}
		return sum(audienceV1{SchemaVersion: in.SchemaVersion, Mode: in.Mode, StoreIDs: ids})
	default:
		return "", errVersion(in.SchemaVersion)
	}
}

type audienceV1 struct {
	SchemaVersion string  `json:"schemaVersion"`
	Mode          string  `json:"mode"`
	StoreIDs      []int64 `json:"storeIds"`
}

// SellerFacingInput 是賣家看得到的詢價內容（白名單欄位）；日期是 YYYY-MM-DD，沒有值的欄位輸出 null，沒有值的文字輸出空字串。
type SellerFacingInput struct {
	SchemaVersion    string
	Item             string
	ItemLabel        string
	Quantity         *int32
	Unit             *string
	Packaging        *string
	DeliveryCity     string
	DeliveryDate     *string
	ResponseDeadline *string
	BuyerNote        string
	Range            SellerRange
}

// SellerRange 是賣家看到的買家預期單價區間與計價基準。
type SellerRange struct {
	BasisQuantity    int32
	BasisUnit        string
	LowerBoundAmount int64
	UpperBoundAmount int64
}

// SellerFacing 回傳賣家可見內容的 hash：SHA-256 的小寫十六進位。
func SellerFacing(in SellerFacingInput) (string, error) {
	switch in.SchemaVersion {
	case SchemaV1:
		return sum(sellerV1{
			SchemaVersion: in.SchemaVersion, Item: in.Item, ItemLabel: in.ItemLabel, Quantity: in.Quantity, Unit: in.Unit, Packaging: in.Packaging,
			DeliveryCity: in.DeliveryCity, DeliveryDate: in.DeliveryDate, ResponseDeadline: in.ResponseDeadline, BuyerNote: in.BuyerNote,
			Range: sellerRangeV1(in.Range),
		})
	default:
		return "", errVersion(in.SchemaVersion)
	}
}

type sellerV1 struct {
	SchemaVersion    string        `json:"schemaVersion"`
	Item             string        `json:"item"`
	ItemLabel        string        `json:"itemLabel"`
	Quantity         *int32        `json:"quantity"`
	Unit             *string       `json:"unit"`
	Packaging        *string       `json:"packaging"`
	DeliveryCity     string        `json:"deliveryCity"`
	DeliveryDate     *string       `json:"deliveryDate"`
	ResponseDeadline *string       `json:"responseDeadline"`
	BuyerNote        string        `json:"buyerNote"`
	Range            sellerRangeV1 `json:"buyerRequestedRange"`
}

type sellerRangeV1 struct {
	BasisQuantity    int32  `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
	LowerBoundAmount int64  `json:"lowerBoundAmount"`
	UpperBoundAmount int64  `json:"upperBoundAmount"`
}

func errVersion(v string) error { return fmt.Errorf("rfqhash：不認得的 schema_version %q", v) }

func sum(pre any) (string, error) {
	b, err := json.Marshal(pre)
	if err != nil {
		return "", fmt.Errorf("rfqhash：序列化：%w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
