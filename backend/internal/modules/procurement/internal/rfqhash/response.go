package rfqhash

import "time"

// SellerResponseInput 是賣家回價一個版本的報價內容（SellerQuote 的全部內容欄位加幣別）；不含 ID、版本號、送出時間與冪等鍵。
// 數量是 NUMERIC(14, 3) 讀回的固定三位小數字串；時間以 UTC 的 RFC 3339（奈秒）輸出，沒有值的欄位輸出 null。
type SellerResponseInput struct {
	SchemaVersion       string
	ResponseType        string
	SpecNote            string
	UnitBasisQuantity   string
	UnitBasisUnit       string
	OfferedQuantity     string
	OfferedQuantityUnit string
	ConversionBasis     string
	Currency            string
	GoodsAmount         int64
	ShippingAmount      int64
	TaxAmount           int64
	TaxInclusionBasis   string
	TotalAmount         int64
	LeadTimeDays        *int32
	PromisedDelivery    *time.Time
	ValidUntil          time.Time
	TermsNote           string
	DeviationDisclosure string
}

// SellerResponse 回傳回價版本的內容 hash：SHA-256 的小寫十六進位（A9-D36）。
func SellerResponse(in SellerResponseInput) (string, error) {
	switch in.SchemaVersion {
	case SchemaV2:
		return sum(responseV2{
			SchemaVersion: in.SchemaVersion, ResponseType: in.ResponseType, SpecNote: in.SpecNote,
			UnitBasisQuantity: in.UnitBasisQuantity, UnitBasisUnit: in.UnitBasisUnit, OfferedQuantity: in.OfferedQuantity, OfferedQuantityUnit: in.OfferedQuantityUnit,
			ConversionBasis: in.ConversionBasis, Currency: in.Currency, GoodsAmount: in.GoodsAmount, ShippingAmount: in.ShippingAmount, TaxAmount: in.TaxAmount,
			TaxInclusionBasis: in.TaxInclusionBasis, TotalAmount: in.TotalAmount, LeadTimeDays: in.LeadTimeDays, PromisedDelivery: instant(in.PromisedDelivery),
			ValidUntil: in.ValidUntil.UTC().Format(time.RFC3339Nano), TermsNote: in.TermsNote, DeviationDisclosure: in.DeviationDisclosure,
		})
	default:
		return "", errVersion(in.SchemaVersion)
	}
}

func instant(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

type responseV2 struct {
	SchemaVersion       string  `json:"schemaVersion"`
	ResponseType        string  `json:"responseType"`
	SpecNote            string  `json:"specNote"`
	UnitBasisQuantity   string  `json:"unitBasisQuantity"`
	UnitBasisUnit       string  `json:"unitBasisUnit"`
	OfferedQuantity     string  `json:"offeredQuantity"`
	OfferedQuantityUnit string  `json:"offeredQuantityUnit"`
	ConversionBasis     string  `json:"conversionBasis"`
	Currency            string  `json:"currency"`
	GoodsAmount         int64   `json:"goodsAmount"`
	ShippingAmount      int64   `json:"shippingAmount"`
	TaxAmount           int64   `json:"taxAmount"`
	TaxInclusionBasis   string  `json:"taxInclusionBasis"`
	TotalAmount         int64   `json:"totalAmount"`
	LeadTimeDays        *int32  `json:"leadTimeDays"`
	PromisedDelivery    *string `json:"promisedDelivery"`
	ValidUntil          string  `json:"validUntil"`
	TermsNote           string  `json:"termsNote"`
	DeviationDisclosure string  `json:"deviationDisclosure"`
}
