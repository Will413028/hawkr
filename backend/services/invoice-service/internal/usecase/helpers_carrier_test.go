package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestNormalizeBuyerIdentity_CarrierHiddenCodeMirrorsDisplay 守住一件在 2026-09-05
// 之前**讓每一張帶載具的統一發票都開不出來**的事：AMEGO 的 f0401 要求顯碼與隱碼相符，
// 而這裡曾經固定回 `carrierID2 = ""`。
//
// 為什麼沒有任何東西轉紅：
//   - 落庫沒問題（carrier_id2_ciphertext 可為 NULL，chk_invdoc_exclusive 不管它）
//   - 我方的讀路徑只用 carrier_id1（B04／B05 顯示、遮罩、hash 比對）
//   - 唯一會抱怨的是 AMEGO，而那是**部署之後、真的付款之後**才發生的事
//
// dev 實證（2026-09-05）：帶載具的 B2C 憑證 ISSUED = 0、ISSUE_FAILED = 2，
// provider_code 都是 3040131「載具顯碼、隱碼不相符」。
//
// 判準錨在《AMEGO 電子發票 API 總覽》§2.18.2 的實測矩陣：
// 「顯碼 ≠ 隱碼／只給顯碼、隱碼留空 → 3040131」。
func TestNormalizeBuyerIdentity_CarrierHiddenCodeMirrorsDisplay(t *testing.T) {
	const b2c = "B2C"

	cases := []struct {
		name    string
		carrier string
		value   string
	}{
		// 三種載具類型的規則相同（3040131 / 3040133 / 3040135 只是碼不同）。
		{"手機條碼", "3J0002", "/GHI+.2"},
		{"自然人憑證", "CQ0001", "AB12345678901234"},
		{"光貿會員載具", "amego", "a0911222333"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			draft := &domain.IssueDraft{CarrierType: c.carrier, CarrierValue: c.value}
			ct, id1, id2, donation := normalizeBuyerIdentity(b2c, draft, "")
			if ct != c.carrier {
				t.Errorf("carrierType = %q, want %q", ct, c.carrier)
			}
			if id1 != c.value {
				t.Errorf("顯碼 = %q, want %q", id1, c.value)
			}
			// 🔴 這一格是本測的本體：隱碼留空 ⇒ AMEGO 回 3040131，發票開不出來。
			if id2 != c.value {
				t.Errorf("隱碼 = %q, want %q（與顯碼相同）—— 留空會讓 AMEGO 回 3040131，"+
					"每一張帶載具的發票都落 ISSUE_FAILED", id2, c.value)
			}
			if donation != "" {
				t.Errorf("有載具時捐贈碼必須為空（chk_invdoc_exclusive），實得 %q", donation)
			}
		})
	}
}

// TestNormalizeBuyerIdentity_NoCarrierLeavesBothEmpty 是上一支的陰性面。
//
// 沒有載具時**兩欄都必須空**：只填其中一欄同樣是 3040131／3040133 的觸發條件，
// 而「顯碼隱碼皆空」才是 AMEGO 接受的「這張發票沒有載具」。
func TestNormalizeBuyerIdentity_NoCarrierLeavesBothEmpty(t *testing.T) {
	t.Run("捐贈", func(t *testing.T) {
		draft := &domain.IssueDraft{CarrierType: domain.AmegoCarrierNone}
		ct, id1, id2, donation := normalizeBuyerIdentity("B2C", draft, "25885")
		if ct != "" || id1 != "" || id2 != "" {
			t.Errorf("捐贈時載具三欄都要空，實得 (%q,%q,%q)", ct, id1, id2)
		}
		if donation != "25885" {
			t.Errorf("捐贈碼 = %q, want 25885", donation)
		}
	})

	t.Run("B2B 不帶載具", func(t *testing.T) {
		// 統編發票不得同時帶載具（chk_invdoc_exclusive），且 AMEGO 對 B2B 只看統編。
		draft := &domain.IssueDraft{CarrierType: "3J0002", CarrierValue: "/GHI+.2"}
		ct, id1, id2, donation := normalizeBuyerIdentity(codemeta.InvoiceDocumentKindB2b.Value, draft, "")
		if ct != "" || id1 != "" || id2 != "" || donation != "" {
			t.Errorf("B2B 四欄都要空，實得 (%q,%q,%q,%q)", ct, id1, id2, donation)
		}
	})

	t.Run("draft 為 nil", func(t *testing.T) {
		ct, id1, id2, donation := normalizeBuyerIdentity("B2C", nil, "25885")
		if ct != "" || id1 != "" || id2 != "" || donation != "" {
			t.Errorf("nil draft 四欄都要空，實得 (%q,%q,%q,%q)", ct, id1, id2, donation)
		}
	})
}
