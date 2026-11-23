package rfqhash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// 價格預覽、受眾與賣家可見內容的 hash（A9-D31 Q5）：演算法與 Snapshot 相同（SHA-256 對固定欄位順序的 JSON，
// 原像含 schemaVersion、欄位集合依版本分派、不認得的版本回錯誤），差別只在原像的內容；各自一個型別，不共用欄位，
// 所以一個 hash 不會被當成另一個的值。

// PricePreviewInput 是價格預覽的內容：預覽本身、它對應的需求版本與價格。
type PricePreviewInput struct {
	SchemaVersion     string
	PricePreviewID    string
	RequestRevisionID string
	Price             Price
}

// PricePreview 回傳價格預覽的 hash：SHA-256 的小寫十六進位。
func PricePreview(in PricePreviewInput) (string, error) {
	switch in.SchemaVersion {
	case SchemaV1:
		return sum(previewV1{
			SchemaVersion: in.SchemaVersion, PricePreviewID: in.PricePreviewID, RequestRevisionID: in.RequestRevisionID, Price: v1Price(in.Price),
		})
	default:
		return "", errVersion(in.SchemaVersion)
	}
}

type previewV1 struct {
	SchemaVersion     string  `json:"schemaVersion"`
	PricePreviewID    string  `json:"pricePreviewId"`
	RequestRevisionID string  `json:"requestRevisionId"`
	Price             v1Price `json:"price"`
}

// AudienceInput 是核准的受眾：模式、公開池的政策與指定對象的店家（順序有意義）。
type AudienceInput struct {
	SchemaVersion string
	Mode          string
	PolicyID      *string
	PolicyVersion *int32
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
		return sum(audienceV1{SchemaVersion: in.SchemaVersion, Mode: in.Mode, PolicyID: in.PolicyID, PolicyVersion: in.PolicyVersion, StoreIDs: ids})
	default:
		return "", errVersion(in.SchemaVersion)
	}
}

type audienceV1 struct {
	SchemaVersion string  `json:"schemaVersion"`
	Mode          string  `json:"mode"`
	PolicyID      *string `json:"policyId"`
	PolicyVersion *int32  `json:"policyVersion"`
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
