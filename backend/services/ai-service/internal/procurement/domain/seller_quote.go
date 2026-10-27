package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 綁到 CodeMeta 生成的 var（非 const——生成型別是 struct literal），手法同 artifact.go
// 的 ArtifactType：編譯期綁死，不留第二份字面值。
var (
	SellerResponseTypeManualQuote                  = codemeta.ProcurementSellerResponseTypeManualQuote.Value
	SellerResponseTypeAcceptNonPriceTermsWithQuote = codemeta.ProcurementSellerResponseTypeAcceptNonPriceTermsWithQuote.Value

	TaxInclusionBasisIncluded = codemeta.ProcurementTaxInclusionBasisTaxIncluded.Value
	TaxInclusionBasisExcluded = codemeta.ProcurementTaxInclusionBasisTaxExcluded.Value

	SellerResponseStatusActive     = codemeta.ProcurementSellerResponseStatusActive.Value
	SellerResponseStatusSuperseded = codemeta.ProcurementSellerResponseStatusSuperseded.Value
	SellerResponseStatusWithdrawn  = codemeta.ProcurementSellerResponseStatusWithdrawn.Value
	SellerResponseStatusExpired    = codemeta.ProcurementSellerResponseStatusExpired.Value

	QuoteAcceptabilityAcceptable = codemeta.ProcurementQuoteAcceptabilityAcceptable.Value
	QuoteAcceptabilityExpired    = codemeta.ProcurementQuoteAcceptabilityExpired.Value
	QuoteAcceptabilityWithdrawn  = codemeta.ProcurementQuoteAcceptabilityWithdrawn.Value
	QuoteAcceptabilitySuperseded = codemeta.ProcurementQuoteAcceptabilitySuperseded.Value
	QuoteAcceptabilityIneligible = codemeta.ProcurementQuoteAcceptabilityIneligible.Value
)

const CurrencyTWD = "TWD"

var (
	ErrQuoteAmountInconsistent = errors.New("procurement: total amount is not goods + shipping + tax")
	ErrQuoteQualityRequired    = errors.New("procurement: offered quality grade and inspection notes are required")
	ErrQuoteFieldInvalid       = errors.New("procurement: seller quote field is invalid")
)

// SellerQuotePayload 是賣家回價的 wire 形狀（#2538 wire 契約 §6.1，十七欄）。
// 金額與數量在 wire 上一律 decimal string；轉成型別由 Normalize 負責，這一層不
// 接受 float。
type SellerQuotePayload struct {
	ResponseType        string         `json:"responseType"`
	OfferedQuality      OfferedQuality `json:"offeredQuality"`
	UnitBasis           UnitBasis      `json:"unitBasis"`
	OfferedQuantity     string         `json:"offeredQuantity"`
	OfferedQuantityUnit string         `json:"offeredQuantityUnit"`
	ConversionBasis     string         `json:"conversionBasis"`
	Currency            string         `json:"currency"`
	GoodsAmount         string         `json:"goodsAmount"`
	ShippingAmount      string         `json:"shippingAmount"`
	TaxAmount           string         `json:"taxAmount"`
	TaxInclusionBasis   string         `json:"taxInclusionBasis"`
	TotalAmount         string         `json:"totalAmount"`
	LeadTimeDays        string         `json:"leadTimeDays"`
	PromisedDeliveryAt  string         `json:"promisedDeliveryAt"`
	ValidUntil          string         `json:"validUntil"`
	TermsNote           string         `json:"termsNote"`
	DeviationDisclosure string         `json:"deviationDisclosure"`
}

type OfferedQuality struct {
	Grade           string `json:"grade"`
	InspectionNotes string `json:"inspectionNotes"`
}

type UnitBasis struct {
	Quantity string `json:"quantity"`
	Unit     string `json:"unit"`
}

// NormalizedSellerQuote 是通過驗證後的回價，金額已是「元」的整數、時間已是 UTC。
type NormalizedSellerQuote struct {
	Payload            SellerQuotePayload
	GoodsAmount        int64
	ShippingAmount     int64
	TaxAmount          int64
	TotalAmount        int64
	LeadTimeDays       *int32
	PromisedDeliveryAt *time.Time
	ValidUntil         time.Time
	PayloadHash        string
}

// Normalize 驗證並轉換一份正式回價。
//
// ⛔ 兩個 responseType 的必填欄位完全相同：「接受非價格條款」不是少填金額的捷徑
// （v6.0 逐字要求兩者都要建立含實際品相、unit basis、goods／shipping／tax／total
// 與 validUntil 的 immutable 版本）。
func (p SellerQuotePayload) Normalize(now time.Time) (NormalizedSellerQuote, error) {
	switch p.ResponseType {
	case SellerResponseTypeManualQuote, SellerResponseTypeAcceptNonPriceTermsWithQuote:
	default:
		return NormalizedSellerQuote{}, fmt.Errorf("%w: responseType %q", ErrQuoteFieldInvalid, p.ResponseType)
	}

	if strings.TrimSpace(p.OfferedQuality.Grade) == "" || strings.TrimSpace(p.OfferedQuality.InspectionNotes) == "" {
		return NormalizedSellerQuote{}, ErrQuoteQualityRequired
	}

	switch p.TaxInclusionBasis {
	case TaxInclusionBasisIncluded, TaxInclusionBasisExcluded:
	default:
		return NormalizedSellerQuote{}, fmt.Errorf("%w: taxInclusionBasis %q", ErrQuoteFieldInvalid, p.TaxInclusionBasis)
	}

	if p.Currency != CurrencyTWD {
		return NormalizedSellerQuote{}, fmt.Errorf("%w: currency %q（v1 恆為 TWD）", ErrQuoteFieldInvalid, p.Currency)
	}

	out := NormalizedSellerQuote{Payload: p}
	var err error
	if out.GoodsAmount, err = parseTWDAmount("goodsAmount", p.GoodsAmount); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if out.ShippingAmount, err = parseTWDAmount("shippingAmount", p.ShippingAmount); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if out.TaxAmount, err = parseTWDAmount("taxAmount", p.TaxAmount); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if out.TotalAmount, err = parseTWDAmount("totalAmount", p.TotalAmount); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if out.TotalAmount != out.GoodsAmount+out.ShippingAmount+out.TaxAmount {
		return NormalizedSellerQuote{}, ErrQuoteAmountInconsistent
	}

	if err := requireDecimal("unitBasis.quantity", p.UnitBasis.Quantity); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if err := requireDecimal("offeredQuantity", p.OfferedQuantity); err != nil {
		return NormalizedSellerQuote{}, err
	}
	if strings.TrimSpace(p.UnitBasis.Unit) == "" || strings.TrimSpace(p.OfferedQuantityUnit) == "" {
		return NormalizedSellerQuote{}, fmt.Errorf("%w: unit 不得為空", ErrQuoteFieldInvalid)
	}

	if p.LeadTimeDays != "" {
		days, convErr := strconv.ParseInt(p.LeadTimeDays, 10, 32)
		if convErr != nil || days < 0 {
			return NormalizedSellerQuote{}, fmt.Errorf("%w: leadTimeDays %q", ErrQuoteFieldInvalid, p.LeadTimeDays)
		}
		v := int32(days)
		out.LeadTimeDays = &v
	}
	if p.PromisedDeliveryAt != "" {
		at, convErr := time.Parse(time.RFC3339, p.PromisedDeliveryAt)
		if convErr != nil {
			return NormalizedSellerQuote{}, fmt.Errorf("%w: promisedDeliveryAt %q", ErrQuoteFieldInvalid, p.PromisedDeliveryAt)
		}
		utc := at.UTC()
		out.PromisedDeliveryAt = &utc
	}

	validUntil, err := time.Parse(time.RFC3339, p.ValidUntil)
	if err != nil {
		return NormalizedSellerQuote{}, fmt.Errorf("%w: validUntil %q", ErrQuoteFieldInvalid, p.ValidUntil)
	}
	out.ValidUntil = validUntil.UTC()
	if !out.ValidUntil.After(now) {
		return NormalizedSellerQuote{}, fmt.Errorf("%w: validUntil 已過期", ErrQuoteFieldInvalid)
	}

	if out.PayloadHash, err = p.Hash(); err != nil {
		return NormalizedSellerQuote{}, err
	}
	return out, nil
}

// Hash 是這一版回價的 canonical hash，同 SellerFacingPayload.Hash 的做法
// （encoding/json 的欄位順序由 struct 宣告決定，因此是 canonical）。
func (p SellerQuotePayload) Hash() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// parseTWDAmount 收「元」的整數字面值。⛔ 不接受小數點與科學記號：金額禁止浮點運算
// （CLAUDE.md §4），而放行小數字串等於把捨入決定交給下游。
func parseTWDAmount(field, s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("%w: %s 不得為空", ErrQuoteFieldInvalid, field)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%w: %s %q 必須是非負整數（元）", ErrQuoteFieldInvalid, field, s)
	}
	return v, nil
}

// requireDecimal 只擋空字串與科學記號；實際精度由 pgxutil.NumericFromString 在
// repository 邊界把關（該函式對含 e/E 的輸入直接拒絕，避免走上 float64 路徑）。
func requireDecimal(field, s string) error {
	if s == "" {
		return fmt.Errorf("%w: %s 不得為空", ErrQuoteFieldInvalid, field)
	}
	if strings.ContainsAny(s, "eE") {
		return fmt.Errorf("%w: %s 不接受科學記號", ErrQuoteFieldInvalid, field)
	}
	return nil
}

// SellerResponseVersionStatus 推導一個版本的生命週期狀態（wire 契約 §5.5：持久層
// 刻意沒有 status 欄，三個狀態全部推導，因為「永不 UPDATE」是本票的核心不變式）。
func SellerResponseVersionStatus(version, latestVersion int32, withdrawnVersion *int32, validUntil, now time.Time) string {
	if withdrawnVersion != nil && *withdrawnVersion == version {
		return SellerResponseStatusWithdrawn
	}
	if version < latestVersion {
		return SellerResponseStatusSuperseded
	}
	if now.After(validUntil) {
		return SellerResponseStatusExpired
	}
	return SellerResponseStatusActive
}

// QuoteAcceptability 是買家端的 typed 判定（wire 契約 §6.3）。前端不得自行推導。
// writeClosed 代表該賣家已 decline 或 publication 已停止寫入——那些版本仍留在
// audit，但不可接受。
func QuoteAcceptability(status string, writeClosed bool) string {
	switch status {
	case SellerResponseStatusWithdrawn:
		return QuoteAcceptabilityWithdrawn
	case SellerResponseStatusSuperseded:
		return QuoteAcceptabilitySuperseded
	case SellerResponseStatusExpired:
		return QuoteAcceptabilityExpired
	}
	if writeClosed {
		return QuoteAcceptabilityIneligible
	}
	return QuoteAcceptabilityAcceptable
}
