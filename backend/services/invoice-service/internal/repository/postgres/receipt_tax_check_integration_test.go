//go:build integration

// RECEIPT 的稅額護欄（#3073 收尾）。
//
// 🔴 這條 CHECK 在 2026-09-03 之前**不存在**：`chk_invdoc_b2c` 管 B2C 的
// `tax_amount = 0`，而 RECEIPT 走的是 `BuildHeader` 的同一個 else 分支、落一樣的 0，
// 卻**只有應用層守著**。40 條 `chk_invdoc*` 裡沒有一條管 RECEIPT + tax。
//
// 少了它的後果不是「資料不漂亮」：收據 PDF 的欄位表直接印 document 上的數字，
// 而任何繞過 `BuildHeader` 的寫入路徑（維運 SQL、日後新增的 usecase）都能在一張
// **免用統一發票**的收據上寫出一個營業稅額 —— 那是對外的不實記載。
//
// 本檔就是 §2.1.2 要求的那件事：加完 gate 要**弄壞它守的東西、看它真的擋下來**，
// 而不是「設定檔裡有這個條目」。
package postgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

// timeNowPlus2Days 是 B2C／B2B 的法定期限佔位值。
func timeNowPlus2Days() time.Time { return time.Now().Add(48 * time.Hour) }

// TestReceiptTaxMustBeZero 驗 chk_invdoc_receipt_tax 真的擋得住。
//
// 三個案例缺一不可：
//   - RECEIPT + tax=0  ⇒ 通過（否則這條 CHECK 把正常路徑也擋了）
//   - RECEIPT + tax≠0  ⇒ 23514（這是它存在的理由）
//   - B2B    + tax≠0   ⇒ 通過（證明它只管 RECEIPT，沒有誤傷 B2B 的拆稅）
func TestReceiptTaxMustBeZero(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)

	insert := func(kind string, sales, tax, total int64, receiptNo, taxID *string) error {
		id := nextID()
		_, err := pool.Exec(ctx,
			`INSERT INTO invoice_documents (
			   id, issuer_id, issuer_name_snapshot, source_type, source_ref, trigger_type,
			   recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
			   document_kind, fulfillment_mode, status,
			   snapshot_version, issue_attempt_seq,
			   sales_amount, tax_amount, total_amount, tax_type, tax_rate_bps,
			   receipt_no, issue_not_before, statutory_deadline_at)
			 VALUES ($1, $2, 'seed-issuer', 'ORDER', $3, 'AUTO',
			         'BUYER', 2623900520, '消費者', $4,
			         $5, $6, 'PENDING',
			         1, 1,
			         $7, $8, $9, 1, 500,
			         $10, now() - interval '1 day', $11)`,
			id, issuerID, "O"+itoa(id%1_000_000_000), taxID,
			kind, modeOf(kind),
			sales, tax, total, receiptNo, deadlineFor(kind))
		return err
	}

	receiptNo := "R" + itoa(nextID()%1_000_000_000)
	zeros := "0000000000"
	b2bTaxID := "12345678"

	t.Run("RECEIPT 稅額 0 ⇒ 通過", func(t *testing.T) {
		if err := insert("RECEIPT", 100, 0, 100, &receiptNo, &zeros); err != nil {
			t.Fatalf("正常的收據被擋下來了：%v —— 這條 CHECK 誤傷了正常路徑", err)
		}
	})

	t.Run("RECEIPT 稅額非 0 ⇒ 23514", func(t *testing.T) {
		no2 := "R" + itoa(nextID()%1_000_000_000)
		err := insert("RECEIPT", 95, 5, 100, &no2, &zeros)
		if err == nil {
			t.Fatal("收據寫進了一個營業稅額而 DB 放行 —— " +
				"收據是免用統一發票的憑證，那個數字會直接印進 PDF，屬對外不實記載")
		}
		if !strings.Contains(err.Error(), "chk_invdoc_receipt_tax") {
			t.Fatalf("擋下來的是別條約束：%v —— 本測試要驗的是 chk_invdoc_receipt_tax", err)
		}
	})

	t.Run("B2B 稅額非 0 ⇒ 通過（沒有誤傷拆稅）", func(t *testing.T) {
		if err := insert("B2B", 95, 5, 100, nil, &b2bTaxID); err != nil {
			t.Fatalf("B2B 的拆稅被擋下來了：%v —— 這條 CHECK 只該管 RECEIPT", err)
		}
	})
}

// deadlineFor 回傳該 document_kind 合法的 statutory_deadline_at。
//
// `chk_invdoc_deadline_presence` 是**雙向等式**：
// `(document_kind IN ('B2C','B2B')) = (statutory_deadline_at IS NOT NULL)`
// —— 收據不是統一發票，沒有法定上傳期限，給它一個時間戳當場 23514。
// （這一格是寫本測試時踩到的，正好說明那些 CHECK 真的在守東西。）
func deadlineFor(kind string) any {
	if kind == "RECEIPT" {
		return nil
	}
	return timeNowPlus2Days()
}

// modeOf 回傳該 document_kind 合法的 fulfillment_mode（chk_invdoc_kind_mode）。
func modeOf(kind string) string {
	if kind == "RECEIPT" {
		return "PLATFORM_RECEIPT"
	}
	return "AMEGO_AUTO"
}
