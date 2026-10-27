package usecase

import (
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 🔴 收據線的對外文案不得出現「發票」字樣（#3026，dev 實測 O481452453 抓到）。
//
// 這是 §5 L11 要防的那件事，方向與 #3022 相反：#3022 是「依法不該開發票的賣家被說成
// 開立失敗」，這裡是「拿到收據的買家被說成拿到發票」。兩者都是畫面對使用者做出
// 與事實不符的告知，而收據與發票在稅務上不能互相冒充。
func TestReceiptStateNotices_NeverSayInvoice(t *testing.T) {
	receiptKind := codemeta.InvoiceDocumentKindReceipt.Value
	for state := range receiptStateNotices {
		got := StateNoticeFor(receiptKind, state)
		if strings.Contains(got, "發票") {
			t.Errorf("收據 state=%s 的文案含「發票」：%q", state, got)
		}
		if got == "" {
			t.Errorf("收據 state=%s 沒有文案", state)
		}
	}
	if strings.Contains(ShippingFeeNoticeFor(receiptKind), "發票") {
		t.Errorf("收據的運費說明含「發票」：%q", ShippingFeeNoticeFor(receiptKind))
	}
}

// 陰性對照：**發票線不得被一起改掉**。收據那份是覆蓋不是取代。
func TestInvoiceStateNotices_Unchanged(t *testing.T) {
	for _, kind := range []string{codemeta.InvoiceDocumentKindB2c.Value, codemeta.InvoiceDocumentKindB2b.Value, ""} {
		if got := StateNoticeFor(kind, orderInvoiceStateIssued); !strings.Contains(got, "發票") {
			t.Errorf("kind=%q 的已開立文案不再提發票：%q", kind, got)
		}
		if got := ShippingFeeNoticeFor(kind); got != ShippingFeeNotice {
			t.Errorf("kind=%q 的運費說明被改動：%q", kind, got)
		}
	}
}

// 🔴 收據沒有載具。載具是電子發票上傳財政部平台的憑證，收據根本不上傳
// ⇒ 顯示它等於暗示了一件沒發生的事（dev 實測收據上掛著 "/DEF5678"）。
func TestCarrierSummary_SuppressedForReceipt(t *testing.T) {
	const mobileBarcode = domain.AmegoCarrierMobileBarcode
	if got, ct := carrierPresentationOf(
		codemeta.InvoiceDocumentKindReceipt.Value, mobileBarcode, "/DEF5678", "0000000000"); got != "" || ct != "" {
		t.Errorf("收據仍顯示載具 summary=%q type=%q", got, ct)
	}
	// 陰性對照：發票的載具必須照常顯示，否則買家看不到自己歸戶到哪。
	if got, ct := carrierPresentationOf(
		codemeta.InvoiceDocumentKindB2c.Value, mobileBarcode, "/DEF5678", "0000000000"); got != "/DEF5678" ||
		ct != codemeta.InvoiceCarrierTypeMobileBarcode.Value {
		t.Errorf("B2C 的載具被誤擋：summary=%q type=%q", got, ct)
	}
	if got, ct := carrierPresentationOf(
		codemeta.InvoiceDocumentKindB2b.Value, "", "", "77777177"); got != "77777177" || ct != "" {
		t.Errorf("B2B 的統編摘要被誤擋：summary=%q type=%q", got, ct)
	}
}

// TestCarrierPresentation_TaxIdIsNotACarrier 是 #3084 的核心斷言。
//
// 🔴 沒有載具的 B2B：summary 退回買受人統編（**行為維持不變** —— 裁決 B 選的是
// 「加判別欄」而不是「把統編從 summary 拿掉」，因為後者會讓 B2B 列表上看不到
// 開給哪個統編），但 carrierType **必須為空**。
//
// 少了這一條，前端拿不到任何東西可以分辨，只能把整欄標成「載具」
// ⇒ dev 實測：document 7501121593846792192 的 carrier_type／carrier_id1_ciphertext
// 皆為 NULL（真的沒有載具），而詳情頁同時出現「買受人統編 22223223」與
// 「載具 22223223」兩列、值一模一樣。
func TestCarrierPresentation_TaxIdIsNotACarrier(t *testing.T) {
	summary, carrierType := carrierPresentationOf(
		codemeta.InvoiceDocumentKindB2b.Value,
		"",         // AMEGO 對打統編不帶 CarrierType（domain.AmegoCarrierNone）
		"",         // 沒有載具值
		"22223223", // 買受人統編
	)
	if summary != "22223223" {
		t.Errorf("summary = %q，want 統編 —— 裁決 B 明訂統編摘要要留著", summary)
	}
	if carrierType != "" {
		t.Fatalf("carrierType = %q，want 空字串 —— 統編不是載具。"+
			"非空會讓前端把「買受人統編」標成「載具」，那正是 #3084", carrierType)
	}
}

// TestCarrierTypeFromAmegoCoversEveryCarrier 守 proto 註解裡那個「這個欄位擋不住的」窗口：
// AMEGO 日後新增載具類型而 CarrierTypeFromAmego 忘了對映時，carrier_type 會回空
// ⇒ 前端把載具標成統編。這支測試讓那個遺漏**在加值域的當下**就轉紅。
func TestCarrierTypeFromAmegoCoversEveryCarrier(t *testing.T) {
	for _, ct := range codemeta.AllInvoiceCarrierTypes {
		if !domain.CarrierUsesAmegoCarrierField(ct) {
			continue // TAX_ID／DONATION 在 AMEGO 端不帶 CarrierType，本來就不該有反向
		}
		amego, ok := domain.CarrierTypeToAmego(ct)
		if !ok || amego == "" {
			t.Fatalf("%s 宣稱會帶 AMEGO CarrierType，卻換不出詞彙", ct.Value)
		}
		back, ok := domain.CarrierTypeFromAmego(amego)
		if !ok || back.Value != ct.Value {
			t.Errorf("%s → %q → 換不回來（got %q ok=%v）—— "+
				"carrier_type 對這種載具會回空，前端會把它標成買受人統編",
				ct.Value, amego, back.Value, ok)
		}
	}
}
