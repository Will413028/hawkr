package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

// ResponseDeadlineLayout 是 responseDeadline 在 API、proto 與 DB DATE 欄位之間
// 唯一的字串形式：只有日期，沒有時間、沒有時區。
//
// 放在 domain 是因為驗證（usecase）與 DB 參數轉換（repository）都要用它，而兩邊
// 對「什麼算合法」必須逐字一致——各自留一份字面值就是讓它們有機會分家。
const ResponseDeadlineLayout = "2006-01-02"

type SellerFacingReferenceRange struct {
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
}

type SellerFacingRequestedRange struct {
	Currency         string `json:"currency"`
	PriceComponent   string `json:"priceComponent"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
}

// SellerFacingPayload 是賣家報價時看得到的全部內容。⛔ 白名單型別：新增欄位前先問
// 「賣家報價需要它嗎」與「它是不是買家私有」。budgetPerUnit 永遠不進來。
type SellerFacingPayload struct {
	// 賣家要知道買家沒指定品相，否則會自己假設一個等級去報價（docx V.7.5 的
	// seller-facing 排除清單不含本欄）。
	QualityKnowledgeState string  `json:"qualityKnowledgeState"`
	Item                  string  `json:"item"`
	ItemCode              *string `json:"itemCode,omitempty"`
	ItemLabel             string  `json:"itemLabel,omitempty"`
	Quantity              *int    `json:"quantity,omitempty"`
	Unit                  *string `json:"unit,omitempty"`
	Packaging             *string `json:"packaging,omitempty"`
	DeliveryCity          *string `json:"deliveryCity,omitempty"`
	DeliveryDate          *string `json:"deliveryDate,omitempty"`
	// ResponseDeadline 是 publication 的屬性，由 preview 的參數帶入，不從 canonical
	// payload 讀（doc 64 §3.1：confirm 當下它還不存在）。賣家要知道何時前要回覆，
	// 所以它屬於 seller-facing 內容、並因此被 seller_facing_payload_hash 涵蓋。
	ResponseDeadline *string `json:"responseDeadline,omitempty"`
	// BuyerNote 是買家自己打字的補充說明，原文轉呈。⛔ 不是品相欄位——伺服器不
	// 解析、不比對、不由它推導等級。omitempty 是刻意的：沒填時這個鍵完全不出現，
	// 序列化結果與本欄加入前逐位元相同 ⇒ 既有 payload_hash 全部不受影響。
	BuyerNote           *string                     `json:"buyerNote,omitempty"`
	ReferencePriceRange *SellerFacingReferenceRange `json:"referencePriceRange,omitempty"`
	BuyerRequestedRange *SellerFacingRequestedRange `json:"buyerRequestedRange,omitempty"`
}

// canonicalRfqPayload 是 confirmed snapshot 的 canonical_payload_json 反序列化目標。
// 欄位對齊 usecase.confirmedRfqPayload；這裡只挑賣家要看的那些——刻意不是完整鏡像，
// 漏掉的欄位就是**不該**流進賣家視野的欄位（pricingMode 是買家端的分支旗標，
// budgetPerUnit 一開始就不在 canonical payload 裡）。
type canonicalRfqPayload struct {
	QualityKnowledgeState string  `json:"qualityKnowledgeState"`
	Item                  *string `json:"item"`
	ItemResolution        *struct {
		Kind  string  `json:"kind"`
		Code  *string `json:"code"`
		Label string  `json:"label"`
	} `json:"itemResolution"`
	Quantity            *int    `json:"quantity"`
	Unit                *string `json:"unit"`
	Packaging           *string `json:"packaging"`
	DeliveryCity        *string `json:"deliveryCity"`
	DeliveryDate        *string `json:"deliveryDate"`
	BuyerNote           *string `json:"buyerNote"`
	ReferencePriceRange *struct {
		Lower         *float64 `json:"lower"`
		Upper         *float64 `json:"upper"`
		BasisQuantity string   `json:"basisQuantity"`
		BasisUnit     string   `json:"basisUnit"`
	} `json:"referencePriceRange"`
	BuyerRequestedRange *struct {
		Currency         string `json:"currency"`
		PriceComponent   string `json:"priceComponent"`
		BasisQuantity    string `json:"basisQuantity"`
		BasisUnit        string `json:"basisUnit"`
		LowerBoundAmount string `json:"lowerBoundAmount"`
		UpperBoundAmount string `json:"upperBoundAmount"`
	} `json:"buyerRequestedRange"`
}

// CompileSellerFacingPayload 由 confirmed snapshot 的 canonical payload 編譯出賣家視角
// 的內容。⛔ 只讀傳入的 JSON，不得查任何其他表——canonical_payload_json 之外的內容
// 不受 payload_hash 保護，join 進來等於引入一條無法 CAS 的路徑（doc 64 §1.2）。
func CompileSellerFacingPayload(canonicalPayloadJSON []byte, responseDeadline *string) (SellerFacingPayload, error) {
	var src canonicalRfqPayload
	if err := json.Unmarshal(canonicalPayloadJSON, &src); err != nil {
		return SellerFacingPayload{}, fmt.Errorf("unmarshal canonical rfq payload: %w", err)
	}

	out := SellerFacingPayload{
		QualityKnowledgeState: src.QualityKnowledgeState,
		Quantity:              src.Quantity, Unit: src.Unit, Packaging: src.Packaging,
		DeliveryCity: src.DeliveryCity, DeliveryDate: src.DeliveryDate,
		ResponseDeadline: responseDeadline, BuyerNote: src.BuyerNote,
	}
	if src.Item != nil {
		out.Item = *src.Item
	}
	if src.ItemResolution != nil {
		out.ItemCode = src.ItemResolution.Code
		out.ItemLabel = src.ItemResolution.Label
	}
	if r := src.ReferencePriceRange; r != nil && (r.Lower != nil || r.Upper != nil) {
		out.ReferencePriceRange = &SellerFacingReferenceRange{
			LowerBoundAmount: optionalAmount(r.Lower),
			UpperBoundAmount: optionalAmount(r.Upper),
			BasisQuantity:    r.BasisQuantity,
			BasisUnit:        r.BasisUnit,
		}
	}
	if r := src.BuyerRequestedRange; r != nil && (r.LowerBoundAmount != "" || r.UpperBoundAmount != "") {
		out.BuyerRequestedRange = &SellerFacingRequestedRange{
			Currency:         r.Currency,
			PriceComponent:   r.PriceComponent,
			BasisQuantity:    r.BasisQuantity,
			BasisUnit:        r.BasisUnit,
			LowerBoundAmount: r.LowerBoundAmount,
			UpperBoundAmount: r.UpperBoundAmount,
		}
	}
	return out, nil
}

func optionalAmount(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// Hash 是這份 payload 的內容指紋，供 execute 的 CAS 與 #2538 的買家接受使用。
func (p SellerFacingPayload) Hash() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal seller-facing payload: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
