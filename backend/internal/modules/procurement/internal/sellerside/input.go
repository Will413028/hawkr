package sellerside

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

func invalid(field, format string, args ...any) error {
	return apperr.InvalidInputOn([]string{field}, format, args...)
}

func text(field, v string, max int) (string, error) {
	v = trimtext.Trim(v)
	switch {
	case !utf8.ValidString(v):
		return "", invalid(field, "%s 不是有效的 UTF-8", field)
	case containsNUL(v):
		return "", invalid(field, "%s 不得含 NUL 字元", field)
	case utf8.RuneCountInString(v) > max:
		return "", invalid(field, "%s 不得超過 %d 個字", field, max)
	}
	return v, nil
}

func containsNUL(v string) bool {
	for _, r := range v {
		if r == 0 {
			return true
		}
	}
	return false
}

func instant(field string, t time.Time) (time.Time, error) {
	if t.IsZero() || t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}, invalid(field, "%s 不是合法的時間", field)
	}
	return t.UTC().Truncate(time.Microsecond), nil
}

func reasonCode(raw string) (*string, error) {
	v, err := text("reason_code", raw, model.MaxReasonCodeRunes)
	if err != nil || v == "" {
		return nil, err
	}
	return &v, nil
}

func nonNegative(field string, v int64) error {
	if v < 0 {
		return invalid(field, "%s 不可為負數", field)
	}
	return nil
}

// NormalizeDraft 檢查草稿的形狀與長度（不驗必填欄位與金額恆等式）並正規化：文字去頭尾空白、時間取 UTC 微秒，回傳存進 JSONB 的內容。
// 列舉只收空字串（沒填）或已知的值；金額不可為負。整份草稿的大小上限只由 seller_response_drafts_payload_size_check 量（jsonb 的文字長度），UpsertDraft 把它的違規對成 INVALID_ARGUMENT。
func NormalizeDraft(in model.QuoteInput) (model.QuoteInput, []byte, error) {
	out := in
	var err error
	texts := []struct {
		field string
		dst   *string
		max   int
	}{
		{"SpecNote", &out.SpecNote, model.MaxSpecNoteRunes},
		{"UnitBasisQuantity", &out.UnitBasisQuantity, model.MaxQuantityTextRunes},
		{"UnitBasisUnit", &out.UnitBasisUnit, model.MaxQuoteUnitRunes},
		{"OfferedQuantity", &out.OfferedQuantity, model.MaxQuantityTextRunes},
		{"OfferedQuantityUnit", &out.OfferedQuantityUnit, model.MaxQuoteUnitRunes},
		{"ConversionBasis", &out.ConversionBasis, model.MaxConversionBasisRunes},
		{"TermsNote", &out.TermsNote, model.MaxTermsNoteRunes},
		{"DeviationDisclosure", &out.DeviationDisclosure, model.MaxDeviationDisclosureRunes},
	}
	for _, t := range texts {
		if *t.dst, err = text(t.field, *t.dst, t.max); err != nil {
			return model.QuoteInput{}, nil, err
		}
	}
	if out.ResponseType != "" && !validResponseType(out.ResponseType) {
		return model.QuoteInput{}, nil, invalid("ResponseType", "ResponseType 不是有效的回價方式")
	}
	if out.TaxInclusionBasis != "" && !validTaxBasis(out.TaxInclusionBasis) {
		return model.QuoteInput{}, nil, invalid("TaxInclusionBasis", "TaxInclusionBasis 不是有效的稅別")
	}
	for _, a := range []struct {
		field string
		v     int64
	}{{"GoodsAmount", out.GoodsAmount}, {"ShippingAmount", out.ShippingAmount}, {"TaxAmount", out.TaxAmount}, {"TotalAmount", out.TotalAmount}} {
		if err := nonNegative(a.field, a.v); err != nil {
			return model.QuoteInput{}, nil, err
		}
	}
	if out.LeadTimeDays != nil && *out.LeadTimeDays < 0 {
		return model.QuoteInput{}, nil, invalid("LeadTimeDays", "LeadTimeDays 不可為負數")
	}
	if out.PromisedDelivery != nil {
		t, err := instant("PromisedDelivery", *out.PromisedDelivery)
		if err != nil {
			return model.QuoteInput{}, nil, err
		}
		out.PromisedDelivery = &t
	}
	if out.ValidUntil != nil {
		t, err := instant("ValidUntil", *out.ValidUntil)
		if err != nil {
			return model.QuoteInput{}, nil, err
		}
		out.ValidUntil = &t
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return model.QuoteInput{}, nil, fmt.Errorf("sellerside：草稿序列化：%w", err)
	}
	return out, payload, nil
}

func validResponseType(v string) bool {
	return v == model.ResponseTypeManualQuote || v == model.ResponseTypeAcceptNonTerm
}

func validTaxBasis(v string) bool { return v == model.TaxIncluded || v == model.TaxExcluded }

type normalized struct {
	Quote                    model.SellerQuote
	UnitBasisNum, OfferedNum pgtype.Numeric
}

func normalizeQuote(in model.QuoteInput) (normalized, error) {
	d, _, err := NormalizeDraft(in)
	if err != nil {
		return normalized{}, err
	}
	switch {
	case !validResponseType(d.ResponseType):
		return normalized{}, invalid("ResponseType", "ResponseType 必須是有效的回價方式")
	case !validTaxBasis(d.TaxInclusionBasis):
		return normalized{}, invalid("TaxInclusionBasis", "TaxInclusionBasis 必須是有效的稅別")
	case d.UnitBasisUnit == "":
		return normalized{}, invalid("UnitBasisUnit", "UnitBasisUnit 不能是空的")
	case d.OfferedQuantityUnit == "":
		return normalized{}, invalid("OfferedQuantityUnit", "OfferedQuantityUnit 不能是空的")
	case d.ValidUntil == nil:
		return normalized{}, invalid("ValidUntil", "ValidUntil 不能是空的")
	}
	basis, err := milli("UnitBasisQuantity", d.UnitBasisQuantity)
	if err != nil {
		return normalized{}, err
	}
	offered, err := milli("OfferedQuantity", d.OfferedQuantity)
	if err != nil {
		return normalized{}, err
	}
	sum, ok := add(d.GoodsAmount, d.ShippingAmount, d.TaxAmount)
	if !ok || sum != d.TotalAmount {
		return normalized{}, invalid("TotalAmount", "TotalAmount 必須等於 GoodsAmount + ShippingAmount + TaxAmount")
	}
	return normalized{
		Quote: model.SellerQuote{
			ResponseType: d.ResponseType, SpecNote: d.SpecNote,
			UnitBasisQuantity: decimal3(basis), UnitBasisUnit: d.UnitBasisUnit, OfferedQuantity: decimal3(offered), OfferedQuantityUnit: d.OfferedQuantityUnit,
			ConversionBasis: d.ConversionBasis, Currency: model.Currency, GoodsAmount: d.GoodsAmount, ShippingAmount: d.ShippingAmount, TaxAmount: d.TaxAmount,
			TaxInclusionBasis: d.TaxInclusionBasis, TotalAmount: d.TotalAmount, LeadTimeDays: d.LeadTimeDays, PromisedDelivery: d.PromisedDelivery,
			ValidUntil: *d.ValidUntil, TermsNote: d.TermsNote, DeviationDisclosure: d.DeviationDisclosure,
		},
		UnitBasisNum: numeric(basis), OfferedNum: numeric(offered),
	}, nil
}

func add(vs ...int64) (int64, bool) {
	var sum int64
	for _, v := range vs {
		if v > math.MaxInt64-sum {
			return 0, false
		}
		sum += v
	}
	return sum, true
}

var quantityPattern = regexp.MustCompile(`^[0-9]{1,11}(\.[0-9]{1,3})?$`)

func milli(field, raw string) (int64, error) {
	if !quantityPattern.MatchString(raw) {
		return 0, invalid(field, "%s 必須是正的十進位數（最多三位小數）", field)
	}
	whole, frac := raw, ""
	for i := range len(raw) {
		if raw[i] == '.' {
			whole, frac = raw[:i], raw[i+1:]
			break
		}
	}
	for len(frac) < 3 {
		frac += "0"
	}
	n, ok := new(big.Int).SetString(whole+frac, 10)
	if !ok || !n.IsInt64() || n.Sign() <= 0 {
		return 0, invalid(field, "%s 必須是正的十進位數（最多三位小數）", field)
	}
	return n.Int64(), nil
}

func decimal3(m int64) string { return fmt.Sprintf("%d.%03d", m/1000, m%1000) }

func numeric(m int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(m), Exp: -3, Valid: true}
}
