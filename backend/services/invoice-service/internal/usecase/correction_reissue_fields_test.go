package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/timeutil"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// #3010：作廢重開產生的新票必須自己有 invoice_date_taipei／tax_period_code／
// statutory_deadline_at，而且**是重開當下算的、不是原票複製的**。
//
// 為什麼這支測試值得存在：兩個各自正確的決定合起來變成沒有人算——
// buildReissueDocument 的註解說「留 nil 讓建票路徑依現在算」，
// 而 settleIssued 的註解說「原值帶回、不以 provider 回傳值覆蓋」。
// 兩段都對，組合起來 3/3 全 NULL，而且不編譯錯、不測試紅、DB CHECK 也允許 NULL。
//
// 危害不只是期別彙總漏算：tax_period_code 為 NULL 會讓 GatherCorrectionFacts 的
// WithinVoidDeadline fail-closed ⇒ 那張新票再也不能被更正，買家看到
// 「已超過法定作廢期限」（#3020 同源）。
func TestBuildReissueDocument_LegalDateFieldsComputedAtReissue(t *testing.T) {
	tpe := timeutil.LocationTaipei
	// 原票：一個**不同期別**的舊日期（115 年第 3 期），期限也早就過了。
	origDate := time.Date(2026, 6, 15, 0, 0, 0, 0, tpe)
	origDeadline := time.Date(2026, 6, 17, 23, 59, 59, 0, tpe).UTC()
	origPeriod := "11503"
	// 重開當下：跨到 115 年第 5 期。
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)

	orig := domain.Document{
		ID:                  111,
		IssuerID:            222,
		DocumentKind:        codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode:     codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		SnapshotVersion:     1,
		TaxRateBps:          domain.TaxRateBps,
		InvoiceDateTaipei:   &origDate,
		TaxPeriodCode:       &origPeriod,
		StatutoryDeadlineAt: &origDeadline,
	}
	revised := domain.ReviseDocumentSnapshot{
		RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientName:  "買家",
		RecipientTaxID: domain.B2CBuyerIdentifier,
	}
	lines := []domain.NewDocumentLine{{
		LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
		Description: "商品", Quantity: 1, UnitPrice: 100, NetAmount: 100,
		TaxCategory: int16(domain.TaxCategoryTaxable),
	}}

	got := buildReissueDocument(999, orig, revised, lines, "O123456789", 2, now)

	wantDate := domain.InvoiceDateTaipei(now)
	wantPeriod := domain.TaxPeriodCode(wantDate)

	// ---- 陽性：三欄都有值，且是重開當下算出來的 ----
	if got.InvoiceDateTaipei == nil {
		t.Fatal("InvoiceDateTaipei 為 nil —— 重開票沒有法定開立日，期別彙總會漏掉它，" +
			"且該票再也不能被更正（#3010）")
	}
	if !got.InvoiceDateTaipei.Equal(wantDate) {
		t.Errorf("InvoiceDateTaipei = %v, want %v（重開當下）", *got.InvoiceDateTaipei, wantDate)
	}
	if got.TaxPeriodCode != wantPeriod {
		t.Errorf("TaxPeriodCode = %q, want %q", got.TaxPeriodCode, wantPeriod)
	}
	if got.StatutoryDeadlineAt == nil {
		t.Fatal("StatutoryDeadlineAt 為 nil —— chk_invdoc_deadline_presence 會當場 23514")
	}

	// ---- 陰性①：不得是原票的值（只斷言「非 nil」的話，複製原值也會通過）----
	if got.InvoiceDateTaipei.Equal(origDate) {
		t.Errorf("InvoiceDateTaipei 複製了原票的 %v —— 新票會落在已申報的期別裡", origDate)
	}
	if got.TaxPeriodCode == origPeriod {
		t.Errorf("TaxPeriodCode 複製了原票的 %q", origPeriod)
	}
	if got.StatutoryDeadlineAt.Equal(origDeadline) {
		t.Errorf("StatutoryDeadlineAt 複製了原票的 %v", origDeadline)
	}

	// ---- 陰性②：chk_invdoc_deadline 是 statutory_deadline_at > issue_not_before。
	// 複製原票期限時這條在原票開立超過期限天數後必然違反 ⇒ 建列當下 23514。----
	if !got.StatutoryDeadlineAt.After(got.IssueNotBefore) {
		t.Errorf("StatutoryDeadlineAt(%v) 不晚於 IssueNotBefore(%v) —— chk_invdoc_deadline 會 23514",
			*got.StatutoryDeadlineAt, got.IssueNotBefore)
	}
}

// 買受人從個人改成公司統編時，期限天數要跟著 document_kind 走（B2C 2 日／B2B 7 日）。
// 這一格是「複製原票期限」最隱蔽的錯法：kind 已經重算了，期限卻還是舊 kind 的天數。
func TestBuildReissueDocument_DeadlineFollowsRecomputedKind(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	origDate := time.Date(2026, 9, 1, 0, 0, 0, 0, timeutil.LocationTaipei)
	origPeriod := "11505"
	origDeadline := time.Date(2026, 9, 3, 23, 59, 59, 0, timeutil.LocationTaipei).UTC()

	orig := domain.Document{
		ID: 111, IssuerID: 222,
		DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		SnapshotVersion: 1, TaxRateBps: domain.TaxRateBps,
		InvoiceDateTaipei: &origDate, TaxPeriodCode: &origPeriod,
		StatutoryDeadlineAt: &origDeadline,
	}
	// 改成公司統編 ⇒ kind 變 B2B ⇒ 期限天數 2 → 7。
	revised := domain.ReviseDocumentSnapshot{
		RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientName:  "某某公司",
		RecipientTaxID: "77777177",
	}
	lines := []domain.NewDocumentLine{{
		LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
		Description: "商品", Quantity: 1, UnitPrice: 105, NetAmount: 105,
		TaxCategory: int16(domain.TaxCategoryTaxable),
	}}

	got := buildReissueDocument(999, orig, revised, lines, "O123456789", 2, now)

	if got.DocumentKind != codemeta.InvoiceDocumentKindB2b.Value {
		t.Fatalf("DocumentKind = %q, want B2B（前置條件沒成立，本測試就沒有鑑別力）", got.DocumentKind)
	}
	wantDeadline, ok := domain.StatutoryDeadline(domain.InvoiceDateTaipei(now),
		codemeta.InvoiceDocumentKindB2b.Value)
	if !ok {
		t.Fatal("StatutoryDeadline 對 B2B 回 false")
	}
	if got.StatutoryDeadlineAt == nil || !got.StatutoryDeadlineAt.Equal(wantDeadline) {
		t.Errorf("StatutoryDeadlineAt = %v, want %v（B2B 是 7 個自然日，不是原票 B2C 的 2 日）",
			got.StatutoryDeadlineAt, wantDeadline)
	}
}

// #3019：重開票的 paid_at 要**複製原票**——同一筆訂單的同一次收款，錢沒有再進來一次。
//
// 這一欄與三個法定日期欄的方向相反（那三個依重開當下重算），兩者放在同一個函式裡
// 特別容易被一起改錯。
func TestBuildReissueDocument_PaidAtIsCopiedNotRecomputed(t *testing.T) {
	tpe := timeutil.LocationTaipei
	paid := time.Date(2026, 6, 15, 3, 21, 0, 0, time.UTC)
	origDate := time.Date(2026, 6, 15, 0, 0, 0, 0, tpe)
	origPeriod := "11503"
	origDeadline := time.Date(2026, 6, 17, 23, 59, 59, 0, tpe).UTC()
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)

	orig := domain.Document{
		ID: 111, IssuerID: 222,
		DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		SnapshotVersion: 1, TaxRateBps: domain.TaxRateBps,
		InvoiceDateTaipei: &origDate, TaxPeriodCode: &origPeriod,
		StatutoryDeadlineAt: &origDeadline,
		PaidAt:              &paid,
	}
	revised := domain.ReviseDocumentSnapshot{
		RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientName:  "買家",
		RecipientTaxID: domain.B2CBuyerIdentifier,
	}
	lines := []domain.NewDocumentLine{{
		LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
		Description: "商品", Quantity: 1, UnitPrice: 100, NetAmount: 100,
		TaxCategory: int16(domain.TaxCategoryTaxable),
	}}

	got := buildReissueDocument(999, orig, revised, lines, "O123456789", 2, now)

	if got.PaidAt == nil {
		t.Fatal("PaidAt 為 nil —— 賣家在 S06 看不到這張重開票對應的款項何時入帳（#3019）")
	}
	if !got.PaidAt.Equal(paid) {
		t.Errorf("PaidAt = %v, want %v（原票的收款時點）", *got.PaidAt, paid)
	}
	// 陰性：不得跟著重開當下走——那會把收款日講成今天。
	if got.PaidAt.Equal(now) || got.PaidAt.Equal(got.IssueNotBefore) {
		t.Errorf("PaidAt 被重算成重開當下（%v）—— 它問的是錢什麼時候進來，不是憑證什麼時候開",
			*got.PaidAt)
	}
}
