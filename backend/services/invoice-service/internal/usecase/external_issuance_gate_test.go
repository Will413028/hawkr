//go:build !integration

// statementOf 住在 fail_reason_visibility_test.go（同樣掛 !integration）。

package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔守 #3166 / DEC-20260905-221900 選 A 的那條新邊，兩個方向各一支：
//
//	① 判準本身對不對（canReportExternalIssuance 的四種輸入）
//	② **三份判準有沒有漂移**（usecase 守衛 ／ 兩句 SQL 的 allowed-from）
//
// ② 是重點。三份分別住在 Go 與兩個 .sql 檔，改一份不會讓另兩份轉紅：
//   - usecase 多開一格 ⇒ 回 200 然後在 SQL 撞 rows=0（泛用漂移錯誤，看起來像併發）
//   - usecase 少一格   ⇒ 合法回報被 9349 擋掉，而那正是 #3166 的原始症狀

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
		// ① J30：ADM 人工出口解鎖的產物，mode 不拘。
		{"人工解鎖 ＋ AMEGO_AUTO", unlocked, auto, true},
		{"人工解鎖 ＋ 自行開立", unlocked, selfIssue, true},

		// ② 新邊：平台從一開始就不開票的兩個 mode。
		{"NOT_APPLICABLE ＋ 賣家自行開立", na, selfIssue, true},
		{"NOT_APPLICABLE ＋ 尚未核准（LEGACY_MANUAL）", na, legacy, true},

		// 🔴 陰性：AMEGO_AUTO 的 NOT_APPLICABLE 是零元訂單／切點前訂單，
		// 平台判定「這一單不開票」而不是「賣家自己開」⇒ 不得開放回報一個發票號碼。
		{"NOT_APPLICABLE ＋ AMEGO_AUTO（零元／切點前）", na, auto, false},

		// 其餘狀態一律不行（回報只在這兩個入口）。
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

	// 兩句都必須認 NOT_APPLICABLE，且都必須夾住那兩個 mode。
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

	// 🔴 document 那句必須同時清 not_applicable_reason：chk_invdoc_na_reason 是雙向等式
	// (status='NOT_APPLICABLE') = (not_applicable_reason IS NOT NULL)
	// ⇒ 只改 status 不清 reason，這條新邊每一次都當場 23514。
	if !strings.Contains(docStmt, "not_applicable_reason = NULL") {
		t.Error("🔴 MarkInvoiceDocumentResolvedExternally 沒有清 not_applicable_reason —— " +
			"chk_invdoc_na_reason 是雙向等式，從 NOT_APPLICABLE 出發時當場 23514")
	}

	// 陰性對照：兩句都必須保留原本那條入口，否則 A4 人工解鎖流程（端點 54–56）會斷掉。
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
	// 這個常數只准出現在三個地方：helper 自己、以及兩處與回報無關的用途
	// （62R 的唯讀述詞、job 清單分組）。任何**新的** `op.Status != ...` 比對都是漂移。
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
