//go:build !integration

package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
)

// TestFailDocumentPersistsReasonForSeller 守「賣家看得到為什麼卡住」。
//
// 賣家收到 `invoice.issue_failed.seller`，文案逐字寫著「請至賣家中心的『發票異常作業』
// **查看原因**並處理」。他到那裡看到的唯一「為什麼」是
// `InvoiceJobInfo.manual_reason_code`，而它讀的是 `invoice_documents.manual_reason_code`。
//
// 🔴 2026-09-05 實查：三個 settle* 都算好了 CodeMeta 理由碼傳進 failDocument，
// 而它只落 `invoice_status_history.reason_code`——那張表**不在任何賣家可見的 DTO 裡**
// ⇒ document 那一欄恆為 NULL、P2 事件的 FailureReasonCode 也恆為空
// （後者讀的是載入時的 doc 快照，那時還沒寫）。畫面上只剩一個狀態 badge。
//
// 最嚴重的是 SELLER_TRACK_INSUFFICIENT（AMEGO 3040111）：它的處置與其他失敗**相反**
// ——重試多久都不會好，賣家必須去財政部配字軌。看不到原因的賣家只會反覆按重試，
// 而每按一次只是燒掉一次 attempt。
//
// ⚠️ 本測試守後端這一半；前端有沒有把它渲染出來由 website 的
// `manualReasonNotice.unit.test.ts` 守。**兩者缺一，賣家都看不到原因。**
func TestFailDocumentPersistsReasonForSeller(t *testing.T) {
	cases := []struct {
		name string
		err  *amego.APIError
		want string
	}{
		{
			"字軌不足（J32）—— 處置與其他失敗相反，最需要被看到",
			&amego.APIError{Code: "3040111", Class: amego.ClassPermanent,
				SellerActionRequired: true, Msg: "字軌不足"},
			codemeta.InvoiceManualReasonSellerTrackInsufficient.Value,
		},
		{
			"provider 永久拒絕（J11）",
			&amego.APIError{Code: "3040123", Class: amego.ClassPermanent, Msg: "統編錯誤"},
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := submitHarness(t, nil)
			h.provider.err = tc.err
			if err := h.uc.Submit(context.Background(), 900002); err != nil {
				t.Fatalf("Submit: %v", err)
			}

			if len(h.docs.issueFailedReasons) != 1 {
				t.Fatalf("MarkIssueFailed 被呼叫 %d 次，want 1", len(h.docs.issueFailedReasons))
			}
			if got := h.docs.issueFailedReasons[0]; got != tc.want {
				t.Errorf("落到 document 的理由 = %q, want %q\n"+
					"空字串 ⇒ InvoiceJobInfo.manual_reason_code 恆為空 ⇒ 賣家中心只剩狀態 badge，"+
					"而通知逐字叫他去那裡「查看原因」", got, tc.want)
			}

			// P2 事件也必須帶它：ADM 與下游消費者靠 FailureReasonCode 分流。
			if len(h.outbox.inserted) != 1 {
				t.Fatalf("outbox 列數 = %d, want 1", len(h.outbox.inserted))
			}
			if body := string(h.outbox.inserted[0].Payload); !strings.Contains(body, tc.want) {
				t.Errorf("P2 payload 沒帶理由碼 %q（讀了載入時的 doc 快照？）：%s", tc.want, body)
			}
		})
	}
}

// TestMarkIssuedClearsStaleFailureReason 是上一支的**必要配對**。
//
// 🔴 在 ISSUE_FAILED 開始寫 manual_reason_code 之前，本欄只有 AWAITING_MANUAL 會設，
// 而離開它的唯一路徑（ResumeInvoiceDocumentFromManual）已經清了 ⇒ 到得了 ISSUED 時
// 它必然是 NULL。**加了新的寫入者就必須回頭看清除者**：J18／J19（ADM 重試）與
// J33（字軌恢復自動回 READY）都不經過那三個清除者 ⇒ 不在 ISSUED 清掉的話，
// 一張重試後成功的發票會永遠掛著上一次的失敗理由，而賣家中心與 ADM 都會在一個
// SUCCEEDED 的作業旁邊顯示「原因：賣家字軌不足」。
//
// 判準錨在 SQL 本文而不是跑一次 UPDATE：清除與否是那一句的性質，
// 而這一層的 fake 不會重現 SQL 的 SET 子句。
func TestMarkIssuedClearsStaleFailureReason(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "db", "queries", "invoice_documents.sql"))
	if err != nil {
		t.Fatalf("讀不到 db/queries/invoice_documents.sql：%v", err)
	}
	body := string(raw)

	stmt := statementOf(t, body, "MarkInvoiceDocumentIssued")
	if !strings.Contains(stmt, "manual_reason_code = NULL") {
		t.Error("MarkInvoiceDocumentIssued 沒有清 manual_reason_code —— " +
			"重試後成功的發票會掛著上一次的失敗理由，畫面上會出現「已開立 ＋ 原因：賣家字軌不足」")
	}
	// 陰性對照：失敗那一句必須**寫**它（否則本檔第一支測試守的東西就不存在了）。
	failStmt := statementOf(t, body, "MarkInvoiceDocumentIssueFailed")
	if !strings.Contains(failStmt, "manual_reason_code = sqlc.arg(manual_reason_code)") {
		t.Error("MarkInvoiceDocumentIssueFailed 沒有寫 manual_reason_code")
	}
}

// statementOf 取某支 query 的 SQL 本文。找不到一律 t.Fatalf 不 t.Skip：
// query 被改名時 SKIP 會讓這道檢查靜默消失。
func statementOf(t *testing.T, body, name string) string {
	t.Helper()
	marker := "-- name: " + name + " "
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("db/queries 裡找不到 %q —— 它被改名或刪掉了", name)
	}
	rest := body[i+len(marker):]
	if j := strings.Index(rest, "-- name: "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
