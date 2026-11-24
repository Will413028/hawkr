package sellerside

import (
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var base = time.Date(2026, 11, 24, 3, 4, 5, 123456789, time.UTC)

func validInput() model.QuoteInput {
	lead := int32(0)
	until := base.Add(72 * time.Hour)
	return model.QuoteInput{
		ResponseType: model.ResponseTypeManualQuote, SpecNote: " 規格如圖 ", UnitBasisQuantity: "1", UnitBasisUnit: " 公斤 ",
		OfferedQuantity: "12.5", OfferedQuantityUnit: "公斤", GoodsAmount: 1000, ShippingAmount: 50, TaxAmount: 20,
		TaxInclusionBasis: model.TaxExcluded, TotalAmount: 1070, LeadTimeDays: &lead, ValidUntil: &until,
	}
}

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("要回欄位違規，得到 nil")
	}
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("要是 INVALID_ARGUMENT，得到 %v", err)
	}
	return err.Error()
}

func TestNormalizeQuoteCanonicalizesTextQuantitiesAndTimes(t *testing.T) {
	in := validInput()
	promised := base.In(time.FixedZone("x", 8*3600))
	in.PromisedDelivery = &promised
	n, err := normalizeQuote(in)
	if err != nil {
		t.Fatal(err)
	}
	q := n.Quote
	if q.SpecNote != "規格如圖" || q.UnitBasisUnit != "公斤" || q.UnitBasisQuantity != "1.000" || q.OfferedQuantity != "12.500" || q.Currency != "TWD" {
		t.Errorf("正規化結果 = %+v", q)
	}
	if q.PromisedDelivery.Location() != time.UTC || q.PromisedDelivery.Nanosecond() != 123456000 {
		t.Errorf("時間要是 UTC 且截到微秒：%v", q.PromisedDelivery)
	}
	if q.LeadTimeDays == nil || *q.LeadTimeDays != 0 {
		t.Errorf("交期 0 天要保留：%v", q.LeadTimeDays)
	}
}

func TestNormalizeQuoteHashEqualsTheHashOfTheStoredValues(t *testing.T) {
	n, err := normalizeQuote(validInput())
	if err != nil {
		t.Fatal(err)
	}
	h1, err := quote.PayloadHash(n.Quote)
	if err != nil {
		t.Fatal(err)
	}
	stored := n.Quote
	stored.ValidUntil = time.UnixMicro(n.Quote.ValidUntil.UnixMicro()).UTC()
	h2, err := quote.PayloadHash(stored)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("送出時算的 hash %s 與存成微秒再讀回算的 %s 不同", h1, h2)
	}
}

func TestNormalizeQuoteRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.QuoteInput)
		field  string
	}{
		{"沒有回價方式", func(q *model.QuoteInput) { q.ResponseType = "" }, "ResponseType"},
		{"未知回價方式", func(q *model.QuoteInput) { q.ResponseType = "OTHER" }, "ResponseType"},
		{"沒有稅別", func(q *model.QuoteInput) { q.TaxInclusionBasis = "" }, "TaxInclusionBasis"},
		{"基準單位空白", func(q *model.QuoteInput) { q.UnitBasisUnit = "  " }, "UnitBasisUnit"},
		{"報價單位空", func(q *model.QuoteInput) { q.OfferedQuantityUnit = "" }, "OfferedQuantityUnit"},
		{"基準數量 0", func(q *model.QuoteInput) { q.UnitBasisQuantity = "0" }, "UnitBasisQuantity"},
		{"數量四位小數", func(q *model.QuoteInput) { q.OfferedQuantity = "1.2345" }, "OfferedQuantity"},
		{"數量科學記號", func(q *model.QuoteInput) { q.OfferedQuantity = "1e3" }, "OfferedQuantity"},
		{"數量負數", func(q *model.QuoteInput) { q.OfferedQuantity = "-1" }, "OfferedQuantity"},
		{"數量超出 NUMERIC(14,3)", func(q *model.QuoteInput) { q.OfferedQuantity = "123456789012" }, "OfferedQuantity"},
		{"總額不等於三項相加", func(q *model.QuoteInput) { q.TotalAmount = 1071 }, "TotalAmount"},
		{"金額負數", func(q *model.QuoteInput) { q.ShippingAmount = -1; q.TotalAmount = 1019 }, "ShippingAmount"},
		{"金額相加溢位", func(q *model.QuoteInput) { q.GoodsAmount = 1<<63 - 1; q.ShippingAmount = 1 }, "TotalAmount"},
		{"交期負數", func(q *model.QuoteInput) { n := int32(-1); q.LeadTimeDays = &n }, "LeadTimeDays"},
		{"沒有有效期限", func(q *model.QuoteInput) { q.ValidUntil = nil }, "ValidUntil"},
		{"規格說明 1001 字", func(q *model.QuoteInput) { q.SpecNote = strings.Repeat("字", 1001) }, "SpecNote"},
		{"換算說明 501 字", func(q *model.QuoteInput) { q.ConversionBasis = strings.Repeat("a", 501) }, "ConversionBasis"},
		{"單位 41 字", func(q *model.QuoteInput) { q.OfferedQuantityUnit = strings.Repeat("a", 41) }, "OfferedQuantityUnit"},
		{"含 NUL", func(q *model.QuoteInput) { q.TermsNote = "a\x00b" }, "TermsNote"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validInput()
			c.mutate(&in)
			_, err := normalizeQuote(in)
			msg := fieldOf(t, err)
			if !strings.Contains(msg, c.field) {
				t.Errorf("訊息 %q 要指出欄位 %s", msg, c.field)
			}
		})
	}
}

func TestNormalizeQuoteAcceptsBoundaries(t *testing.T) {
	in := validInput()
	in.SpecNote = strings.Repeat("字", 1000)
	in.OfferedQuantity = "99999999999.999"
	in.GoodsAmount, in.ShippingAmount, in.TaxAmount, in.TotalAmount = 0, 0, 0, 0
	if _, err := normalizeQuote(in); err != nil {
		t.Fatalf("邊界值要通過：%v", err)
	}
}

func TestNormalizeDraftChecksShapeOnly(t *testing.T) {
	d, payload, err := NormalizeDraft(model.QuoteInput{SpecNote: "  草稿  ", GoodsAmount: 5, TotalAmount: 99})
	if err != nil {
		t.Fatalf("草稿不驗必填欄位與金額恆等式：%v", err)
	}
	if d.SpecNote != "草稿" || len(payload) == 0 {
		t.Errorf("草稿 = %+v、payload %s", d, payload)
	}
	cases := []struct {
		name   string
		mutate func(*model.QuoteInput)
		field  string
	}{
		{"規格說明 1001 字", func(q *model.QuoteInput) { q.SpecNote = strings.Repeat("字", 1001) }, "SpecNote"},
		{"數量文字 33 字", func(q *model.QuoteInput) { q.OfferedQuantity = strings.Repeat("1", 33) }, "OfferedQuantity"},
		{"未知回價方式", func(q *model.QuoteInput) { q.ResponseType = "X" }, "ResponseType"},
		{"未知稅別", func(q *model.QuoteInput) { q.TaxInclusionBasis = "X" }, "TaxInclusionBasis"},
		{"金額負數", func(q *model.QuoteInput) { q.TaxAmount = -1 }, "TaxAmount"},
		{"交期負數", func(q *model.QuoteInput) { n := int32(-1); q.LeadTimeDays = &n }, "LeadTimeDays"},
		{"零值時間", func(q *model.QuoteInput) { z := time.Time{}; q.ValidUntil = &z }, "ValidUntil"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var in model.QuoteInput
			c.mutate(&in)
			_, _, err := NormalizeDraft(in)
			if msg := fieldOf(t, err); !strings.Contains(msg, c.field) {
				t.Errorf("訊息 %q 要指出欄位 %s", msg, c.field)
			}
		})
	}
}

// 每個欄位都在自己的字數上限內，但 JSON 會把 < > & 轉成六位元組的轉義：三個 1000 字的說明加 500 字的換算說明序列化後超過 16 KiB。
func TestDraftSizeIsCappedByTheSerializedBytes(t *testing.T) {
	in := model.QuoteInput{
		SpecNote: strings.Repeat("<", 1000), TermsNote: strings.Repeat("<", 1000), DeviationDisclosure: strings.Repeat("<", 1000), ConversionBasis: strings.Repeat("<", 500),
	}
	_, _, err := NormalizeDraft(in)
	if msg := fieldOf(t, err); !strings.Contains(msg, "16384") {
		t.Errorf("訊息 %q 要指出整份草稿超過上限", msg)
	}
	in.DeviationDisclosure, in.ConversionBasis = "", ""
	if _, payload, err := NormalizeDraft(in); err != nil || len(payload) > model.MaxDraftBytes {
		t.Fatalf("上限以內的草稿要通過：%v（%d 位元組）", err, len(payload))
	}
}

func TestReasonCode(t *testing.T) {
	if r, err := reasonCode("   "); err != nil || r != nil {
		t.Errorf("空白是沒填：%v %v", r, err)
	}
	if r, err := reasonCode(" PRICE "); err != nil || r == nil || *r != "PRICE" {
		t.Errorf("要去頭尾空白：%v %v", r, err)
	}
	if _, err := reasonCode(strings.Repeat("a", 65)); err == nil {
		t.Error("65 字要被擋")
	}
	if _, err := reasonCode(strings.Repeat("a", 64)); err != nil {
		t.Errorf("64 字要通過：%v", err)
	}
}

func TestParseVersion(t *testing.T) {
	for raw, want := range map[string]int32{"": 0, "1": 1, "42": 42} {
		if got, err := ParseVersion("v", raw); err != nil || got != want {
			t.Errorf("%q = %d、%v", raw, got, err)
		}
	}
	for _, raw := range []string{"0", "-1", "x", "1.5", "99999999999"} {
		if _, err := ParseVersion("v", raw); err == nil {
			t.Errorf("%q 要被擋", raw)
		}
	}
}
