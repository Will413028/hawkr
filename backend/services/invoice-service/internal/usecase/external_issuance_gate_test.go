//go:build !integration

package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

func TestCanReportExternalIssuance(t *testing.T) {
	selfIssue := codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value
	legacy := codemeta.InvoiceFulfillmentModeLegacyManual.Value
	auto := codemeta.InvoiceFulfillmentModeAmegoAuto.Value
	unlocked := codemeta.InvoiceOperationStatusManualProcessAllowed.Value
	na := codemeta.InvoiceOperationStatusNotApplicable.Value

	cases := []struct {
		name   string
		opStat string
		mode   string
		want   bool
	}{
		{"人工解鎖 ＋ AMEGO_AUTO", unlocked, auto, true},
		{"人工解鎖 ＋ 自行開立", unlocked, selfIssue, true},

		{"NOT_APPLICABLE ＋ 賣家自行開立", na, selfIssue, true},
		{"NOT_APPLICABLE ＋ 尚未核准（LEGACY_MANUAL）", na, legacy, true},

		{"NOT_APPLICABLE ＋ AMEGO_AUTO（零元／切點前）", na, auto, false},

		{"SENDING", "SENDING", selfIssue, false},
		{"SUCCEEDED", "SUCCEEDED", selfIssue, false},
		{"FAILED_PERMANENT", "FAILED_PERMANENT", legacy, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := canReportExternalIssuance(
				domain.Operation{Status: c.opStat},
				domain.Document{FulfillmentMode: c.mode},
			)
			if got != c.want {
				t.Errorf("canReportExternalIssuance(op=%s, mode=%s) = %v, want %v",
					c.opStat, c.mode, got, c.want)
			}
		})
	}
}

// TestExternalIssuanceAllowedFromMatchesGuard 守三份判準不漂移。
//
// 判準錨在 SQL 本文：allowed-from 是那兩句的性質，usecase 層的替身重現不了 WHERE 子句。
func TestExternalIssuanceAllowedFromMatchesGuard(t *testing.T) {
	read := func(f string) string {
		raw, err := os.ReadFile(filepath.Join("..", "..", "db", "queries", f))
		if err != nil {
			t.Fatalf("讀不到 db/queries/%s：%v", f, err)
		}
		return string(raw)
	}

	docStmt := statementOf(t, read("invoice_documents.sql"), "MarkInvoiceDocumentResolvedExternally")
	opStmt := statementOf(t, read("invoice_operations.sql"), "MarkInvoiceOperationResolvedExternally")

	for name, stmt := range map[string]string{
		"MarkInvoiceDocumentResolvedExternally":  docStmt,
		"MarkInvoiceOperationResolvedExternally": opStmt,
	} {
		if !strings.Contains(stmt, "NOT_APPLICABLE") {
			t.Errorf("🔴 %s 的 allowed-from 不含 NOT_APPLICABLE —— "+
				"尚未核准的賣家自行開票後回報不了（#3166 的原始症狀）", name)
		}
		if !strings.Contains(stmt, "'SELLER_SELF_ISSUE'") || !strings.Contains(stmt, "'LEGACY_MANUAL'") {
			t.Errorf("🔴 %s 沒有夾住 fulfillment_mode —— "+
				"AMEGO_AUTO 的零元訂單／切點前訂單也會被開放回報發票號碼", name)
		}
	}

	if !strings.Contains(docStmt, "not_applicable_reason = NULL") {
		t.Error("🔴 MarkInvoiceDocumentResolvedExternally 沒有清 not_applicable_reason —— " +
			"chk_invdoc_na_reason 是雙向等式，從 NOT_APPLICABLE 出發時當場 23514")
	}

	if !strings.Contains(opStmt, "MANUAL_PROCESS_ALLOWED") {
		t.Error("MarkInvoiceOperationResolvedExternally 掉了 MANUAL_PROCESS_ALLOWED —— A4 流程會斷")
	}
	if !strings.Contains(docStmt, "'ISSUE_FAILED'") || !strings.Contains(docStmt, "'AWAITING_MANUAL'") {
		t.Error("MarkInvoiceDocumentResolvedExternally 掉了原本的 allowed-from")
	}
}

// TestExternalIssuanceGateHasSingleImplementation 守「同一個判準只有一份實作」。
//
// 🔴 這支測試的存在理由是一次真實的漏改（2026-09-05）：`canReportExternalIssuance`
// 有兩個呼叫端——端點 56（ADM，admin_job.go）與端點 62（賣家，seller_job.go）——
// 而修 #3166 時我只改了前者。CI 全綠、單元測試全綠、變異驗證也轉紅了，
// **而 dev 實測仍回 9349**：賣家那一份還在自己比對
// `op.Status != MANUAL_PROCESS_ALLOWED`。
//
// 判準錨在**原始碼裡有沒有裸比對**，不是「有沒有呼叫 helper」：後者對
// 「呼叫了 helper 但又自己補一個 if」無感，而那正是漂移最常見的形狀。
func TestExternalIssuanceGateHasSingleImplementation(t *testing.T) {
	files := []string{"admin_job.go", "seller_job.go"}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("讀不到 %s：%v", f, err)
		}
		body := string(raw)
		if strings.Contains(body, "op.Status != codemeta.InvoiceOperationStatusManualProcessAllowed.Value") {
			t.Errorf("🔴 %s 自己比對 MANUAL_PROCESS_ALLOWED —— "+
				"回報資格的判準必須走 canReportExternalIssuance，"+
				"兩份實作改一份不會讓另一份轉紅（#3166 的漏改就是這樣過關的）", f)
		}
		if !strings.Contains(body, "canReportExternalIssuance(") {
			t.Errorf("🔴 %s 沒有呼叫 canReportExternalIssuance —— "+
				"端點 56／62 兩條路都必須走同一個判準", f)
		}
	}
}

// TestExternalIssuanceListablePredicateCoversNotApplicable 守讀取面跟得上寫入面。
//
// 端點 62 打得通、而 62R 的清單看不到那筆 ⇒ 賣家在畫面上找不到入口。
// 這是「後端開了路、前端沒有門」的形狀，兩側都綠、沒有任何東西會報。
func TestExternalIssuanceListablePredicateCoversNotApplicable(t *testing.T) {
	want := map[string]bool{
		codemeta.InvoiceOperationStatusManualProcessAllowed.Value: true,
		codemeta.InvoiceOperationStatusNotApplicable.Value:        true,
		codemeta.InvoiceOperationStatusResolvedExternally.Value:   true,
	}
	if len(externalIssuanceStatuses) != len(want) {
		t.Fatalf("62R 述詞有 %d 值，want %d", len(externalIssuanceStatuses), len(want))
	}
	for _, s := range externalIssuanceStatuses {
		if !want[s] {
			t.Errorf("62R 述詞多了 %q", s)
		}
		delete(want, s)
	}
	for s := range want {
		t.Errorf("🔴 62R 述詞缺少 %q —— 賣家在清單上看不到可回報的案件", s)
	}
}
