package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestBuildReissueDocument_CarrierSurvivesReissue 問一個 2026-09-05 之前沒有人問過的問題：
// 買家的**載具**在作廢重開之後還在不在。
//
// 形狀與 #3010（法定日期三欄）完全相同：`ReviseDocumentSnapshot` 有 CarrierID1／
// CarrierID2 兩個欄位，而 `correction_intake.go` 建它的時候**從來沒有賦值**
// （全 repo 生產程式碼零賦值點）⇒ 恆為 ""。`buildReissueDocument` 照抄過去，
// `ReviseInvoiceDocumentSnapshot` 的 SQL 又是 `SET carrier_id1_ciphertext = narg(...)`
// （沒有「保持原值」的選項）⇒ R03 就地改與 R04 作廢重開**都會把載具抹掉**。
//
// 為什麼不會有任何東西轉紅：欄位可為 NULL、chk_invdoc_exclusive 允許三者皆空、
// 我方讀路徑對「沒有載具」與「有載具」一樣渲染得出來。買家只會發現
// 「重開的那張發票沒有存到我的載具」，而那要等他去財政部平台對獎才看得到。
//
// 修法：correction_intake 從**已經解析好的** `id`（⑤-b 早就呼叫 resolveInvoiceIdentity）
// 把載具四欄填進 Revised —— 解析一直都做了，只是值沒接上。
func TestBuildReissueDocument_CarrierSurvivesReissue(t *testing.T) {
	now := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	carrier := "3J0002"
	orig := domain.Document{
		ID:              111,
		IssuerID:        222,
		DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		SnapshotVersion: 1,
		TaxRateBps:      domain.TaxRateBps,
		CarrierType:     &carrier,
	}
	// revised 逐字複製 correction_intake.go 建出來的形狀（修好之後：四欄都有值）。
	const carrierValue = "/GHI+.2"
	revised := domain.ReviseDocumentSnapshot{
		RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientName:  "買家",
		RecipientTaxID: domain.B2CBuyerIdentifier,
		CarrierType:    &carrier,
		CarrierID1:     carrierValue,
		CarrierID2:     carrierValue,
	}
	lines := []domain.NewDocumentLine{{
		LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
		Description: "商品", Quantity: 1, UnitPrice: 100, NetAmount: 100,
		TaxCategory: int16(domain.TaxCategoryTaxable),
	}}

	got := buildReissueDocument(999, orig, revised, lines, "O123456789", 2, now)

	if got.CarrierType != carrier {
		t.Errorf("CarrierType = %q, want %q", got.CarrierType, carrier)
	}
	if got.CarrierID1 == "" {
		t.Error("🔴 重開票的載具顯碼是空的 —— 買家的載具在作廢重開之後消失了")
	}
	// 顯碼＝隱碼是 AMEGO 的硬要求（見 normalizeBuyerIdentity 的紅字）。
	if got.CarrierID1 != got.CarrierID2 {
		t.Errorf("顯碼 %q ≠ 隱碼 %q —— AMEGO 會回 3040131，重開票開不出來",
			got.CarrierID1, got.CarrierID2)
	}
}

// TestRevisedCarrierFromIdentity 釘住更正時載具四欄的折法。
//
// 在 2026-09-05 之前這四欄**整組沒有被賦值**，而 ⑤-b 的註解早就寫著
// 「與結帳共用同一份推導——另寫一份的症狀是『更正之後買家原本選的載具消失了』」
// ——解析真的做了（resolveInvoiceIdentity 拿到 id），只是解完只用了
// id.SelectionMode（order-service 那一側），invoice_documents 這一側四欄全空。
// **解析做了、值沒接上。**
func TestRevisedCarrierFromIdentity(t *testing.T) {
	const b2c = "B2C"
	b2b := codemeta.InvoiceDocumentKindB2b.Value

	t.Run("手機條碼：折成 AMEGO 原生詞彙並帶值", func(t *testing.T) {
		id := checkoutIdentity{
			CarrierType:  codemeta.InvoiceCarrierTypeMobileBarcode.Value,
			CarrierValue: "/GHI+.2",
			HasCarrier:   true,
		}
		ct, cv, dc := revisedCarrierFromIdentity(b2c, id)
		if ct == nil || *ct != domain.AmegoCarrierMobileBarcode {
			t.Fatalf("carrierType = %v, want %q（CodeMeta Value 必須折成原生詞彙）",
				ct, domain.AmegoCarrierMobileBarcode)
		}
		if cv != "/GHI+.2" {
			t.Errorf("🔴 carrierValue = %q —— 空的話重開票會被 AMEGO 回 3040132", cv)
		}
		if dc != nil {
			t.Errorf("有載具時捐贈碼必須為 nil（chk_invdoc_exclusive），實得 %v", *dc)
		}
	})

	t.Run("自然人憑證", func(t *testing.T) {
		id := checkoutIdentity{
			CarrierType:  codemeta.InvoiceCarrierTypeCitizenCert.Value,
			CarrierValue: "AB12345678901234",
			HasCarrier:   true,
		}
		ct, cv, _ := revisedCarrierFromIdentity(b2c, id)
		if ct == nil || *ct != domain.AmegoCarrierCitizenCert {
			t.Fatalf("carrierType = %v, want %q", ct, domain.AmegoCarrierCitizenCert)
		}
		if cv == "" {
			t.Error("carrierValue 是空的")
		}
	})

	// 🔴 陰性①：買家補打統編是端點 11 的**主要使用情境**，舊載具必須跟著消失
	// ——留著就違反 chk_invdoc_exclusive，就地 UPDATE 當場 23514。
	t.Run("B2B：四欄全清", func(t *testing.T) {
		id := checkoutIdentity{
			CarrierType:  codemeta.InvoiceCarrierTypeMobileBarcode.Value,
			CarrierValue: "/GHI+.2",
			HasCarrier:   true,
			BuyerTaxID:   "22223223",
		}
		ct, cv, dc := revisedCarrierFromIdentity(b2b, id)
		if ct != nil || cv != "" || dc != nil {
			t.Errorf("B2B 應四欄全清，實得 (%v,%q,%v)", ct, cv, dc)
		}
	})

	// 🔴 陰性②：捐贈時載具兩欄必須空——carrier_type 空而號碼有值同樣會被 AMEGO 拒。
	t.Run("捐贈：載具空、捐贈碼有值", func(t *testing.T) {
		id := checkoutIdentity{
			CarrierType:  codemeta.InvoiceCarrierTypeDonation.Value,
			DonationCode: "25885",
		}
		ct, cv, dc := revisedCarrierFromIdentity(b2c, id)
		if ct != nil {
			t.Errorf("捐贈時 carrierType 必須為 nil，實得 %q", *ct)
		}
		if cv != "" {
			t.Errorf("捐贈時 carrierValue 必須為空，實得 %q", cv)
		}
		if dc == nil || *dc != "25885" {
			t.Errorf("捐贈碼 = %v, want 25885", dc)
		}
	})
}

// TestRevisedCarrierFromIdentity_TypeAndValueGoTogether 守 chk_invdoc_carrier_pair。
//
// 🔴 那條 CHECK 是**雙向等式**：
//
//	(carrier_type IS NULL) = (carrier_id1_ciphertext IS NULL)
//
// 只填類型不填號碼 ⇒ 23514 ⇒ 整條更正路徑回 500。
//
// 這正是 2026-09-05 我自己在 dev 上踩到的：第一版修法從 `id.CarrierType` 折出
// 類型就直接填，而 **`id.CarrierValue` 對來自常用發票資料的載具是空的**
// ——resolveInvoiceIdentity 那一層不解密（checkout.go 的 draftFromIdentity 為此
// 用 carrierPresentPlaceholder 繞過）。結果是把「靜默丟失載具」換成了
// 「帶載具的發票完全不能更正」，而單元測試因為 fixture 自己填了值所以全綠。
//
// ⇒ 判準改成「兩者同進同出」，並在呼叫端補 openCarrierValueInto 取明文。
func TestRevisedCarrierFromIdentity_TypeAndValueGoTogether(t *testing.T) {
	// 明文取不到（常用發票資料的載具尚未解密）⇒ 兩欄都必須留空，
	// 不得只填類型。放行的話 DB 當場 23514。
	id := checkoutIdentity{
		CarrierType:  codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		CarrierValue: "", // ← 關鍵：解析層沒有解密
		HasCarrier:   true,
		PreferenceID: 7501935856077242368,
	}
	ct, cv, _ := revisedCarrierFromIdentity("B2C", id)
	if ct != nil || cv != "" {
		t.Errorf("🔴 載具值為空時填了類型 (%v,%q) —— chk_invdoc_carrier_pair 是雙向等式，"+
			"DB 會 23514，帶載具的發票完全不能更正", ct, cv)
	}
}
