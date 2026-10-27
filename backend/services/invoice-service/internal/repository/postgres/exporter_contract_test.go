// 本檔**沒有 build tag**，所以在一般 `go test ./...` 就會跑。
//
// 這是刻意的：它守的是「某個狀態值沒有任何指標數得到」這一個 class，而該 class 的四個
// 已知實例（inbox RECEIVED 無掃描器／refund DEFERRED 無執行者／outbox 'failed' 永久旗標
// ＋告警反向／document AWAITING_MANUAL 全盲）沒有一個是編譯錯或測試紅——全部要靠一支
// 專門去問「還有誰沒被數到」的檢查才看得見。掛在 integration tag 下的話，它只在有
// Docker 的環境跑，而這裡要驗的東西一個 DB 都不需要。
package postgres

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoFile 讀 repo 內的檔案（相對本 package 目錄）。
func repoFile(t *testing.T, rel ...string) string {
	t.Helper()
	path := filepath.Join(rel...)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 %s：%v", path, err)
	}
	return string(raw)
}

// exporterQuery 從 monitoring/sql-exporter/sql_exporter.yml 抽出某個 metric 的 query。
//
// 🔴 找不到一律 t.Fatalf 不 t.Skip：檔案被搬走或 metric 被改名時，SKIP 會讓這道檢查
// 靜默消失，而它守的正是「那支查詢從來沒有被執行過」。
func exporterQuery(t *testing.T, metricName string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "..", "monitoring", "sql-exporter", "sql_exporter.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 %s：%v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "- metric_name: "+metricName {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("sql_exporter.yml 裡找不到 metric %q", metricName)
	}
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "query: |" {
			continue
		}
		indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		var body []string
		for j := i + 1; j < len(lines); j++ {
			cur := lines[j]
			if strings.TrimSpace(cur) == "" {
				break
			}
			if len(cur)-len(strings.TrimLeft(cur, " ")) <= indent {
				break
			}
			body = append(body, strings.TrimLeft(cur, " "))
		}
		if len(body) == 0 {
			t.Fatalf("metric %q 的 query 區塊是空的", metricName)
		}
		return strings.Join(body, "\n")
	}
	t.Fatalf("metric %q 之後找不到 `query: |` 區塊", metricName)
	return ""
}

// checkConstraintValues 從 db/schema/schema.sql 抽某條 CHECK ... IN ('a','b',…) 的值域。
//
// 🔴 值域**從 schema 推導不從這裡列舉**：手寫一份清單的話，新增狀態時沒有任何東西會提醒
// 你回來改它 —— 而那正是本檔要防的失效形狀（丙：eventProducers 硬寫 7 項的同一個病）。
func checkConstraintValues(t *testing.T, constraint string) []string {
	t.Helper()
	schema := repoFile(t, "..", "..", "..", "db", "schema", "schema.sql")
	idx := strings.Index(schema, `"`+constraint+`"`)
	if idx < 0 {
		t.Fatalf("schema.sql 裡找不到 CHECK 約束 %q —— 它被改名或刪除了，"+
			"而本檔的整個覆蓋率論證建立在它身上", constraint)
	}
	// 取該約束之後第一組小括號內的字面值。
	rest := schema[idx:]
	end := strings.Index(rest, "\n")
	if end < 0 {
		end = len(rest)
	}
	vals := regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(rest[:end], -1)
	if len(vals) == 0 {
		t.Fatalf("CHECK %q 解不出任何字面值（寫法變了？）：%s", constraint, rest[:end])
	}
	out := make([]string, 0, len(vals))
	for _, m := range vals {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

// statusesIn 回傳 query 裡出現過的狀態字面值（用於判斷某支指標的述詞含哪些狀態）。
func statusesIn(query string, domain []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range domain {
		if strings.Contains(query, "'"+s+"'") {
			out[s] = true
		}
	}
	return out
}

// TestDocumentStatusCoverageHasNoSilentTerminal 是「第四個靜默終點」（AWAITING_MANUAL）
// 的結構性防線：invoice_documents 的每一個狀態值，都必須明確落進三類之一。
//
// 🔴 它要防的**不是**已經修好的那一個，是**第五個**。四個已知實例的共同形狀是
// 「新增／改動一個狀態，而沒有人回頭問『現在誰數得到它』」——那個問題沒有任何
// 編譯器、linter 或既有測試會替你問。本測試就是那個問題的機械形式。
//
// ⚠️ 新增狀態值時本測試會紅，**那是它在工作**。修法是把該值歸類並寫下理由，
//
//	不是把它塞進 closedStatuses 了事：塞進去代表你宣稱「這個狀態不需要任何人再看它」。
//
// 🔴 **這道 gate 擋不住什麼**（誠實標明邊界，`.claude/rules/06-no-small-tails.md` §3.10）：
// 它的觀測窗是**狀態值**，看不到查詢內部的其他濾網。已知的一個實例：
// M7 的述詞除了 status 還帶 `statutory_deadline_at IS NOT NULL`，而
// **RECEIPT 的該欄恆為 NULL**（`chk_invdoc_deadline_presence` 強制；收據不申報、
// 沒有法定上傳期限）⇒ 一張停在 `ISSUE_FAILED` 的收據，status 這一格算「被 M7 覆蓋」，
// 實際上 M7 的 series 裡一列都沒有。同理它也看不到 M6 的
// `invoice_pending_operations` 刻意排除 `FAILED_PERMANENT` 這件事。
//
// ⇒ 那類 row 今天的唯一能見度是 ADM 的 NEEDS_ATTENTION 分頁（人要去看才看得到），
// 以及比率型的 `InvoiceIssueFailureRateHigh`（事件過後回 0，而列還卡著）。
// 要把它變成「有 gauge 隨時間累積」得動告警口徑（多一個桶或放寬 M7 的濾網），
// 那會改變既有告警的語意，屬於需要功能擁有者點頭的事，**不在本測試的職權內**。
// 2026-09-05 dev 實查：`ISSUE_FAILED` 兩列皆 B2B，這個洞尚未有真實實例。
func TestDocumentStatusCoverageHasNoSilentTerminal(t *testing.T) {
	domain := checkConstraintValues(t, "chk_invdoc_status")

	// ① 還在自動線上 —— 由 M7 invoice_statutory_deadline_pending 承擔（③ 兩條告警）。
	onAutomaticLine := statusesIn(exporterQuery(t, "invoice_statutory_deadline_pending"), domain)
	// ② 在等人 —— 由 M32 invoice_documents_awaiting_manual 承擔（㉕／㉖ 兩條告警）。
	awaitingHuman := statusesIn(exporterQuery(t, "invoice_documents_awaiting_manual"), domain)

	// ③ 已結案或由 operation 軸承擔。每一項都必須寫下「為什麼沒有人需要再看它」。
	closedStatuses := map[string]string{
		"ISSUED":                 "發票已開出，法定義務已履行；後續作廢／折讓各有自己的 operation 與指標",
		"VOIDED":                 "作廢已完成並經 invoice_query 確認（C0501）",
		"SUPERSEDED":             "已被重開的新文件取代，新文件自己會落進 ① 或 ②",
		"CANCELLED_BEFORE_ISSUE": "ADM 雙人確認本單不需開立，invoice_admin_audit_logs 留有理由",
		"NOT_APPLICABLE":         "合法的「不開票」路徑（零元／免用發票賣家／切點前訂單），not_applicable_reason 必填",
		"RESOLVED_EXTERNALLY":    "賣家已自行開立並回報號碼，平台不再有開立義務",
		// 🔴 VOIDING 是**唯一**靠 operation 軸兜底的一個：它必然有一筆非終態的 VOID
		//    operation 驅動，而 M6 invoice_pending_operations 數得到那筆 operation。
		//    它落在這裡的前提是「VOID operation 進終態時 document 一定同步離開 VOIDING」
		//    （成功→VOIDED、被 provider 拒絕→RestoreInvoiceDocumentToIssued 回 ISSUED）。
		//    ✅ **§7.2.2 落地時（#3073）已重驗，前提成立**：`Voiding.settleVoidPermanent`
		//    在 `doc.Status == VOIDING` 時呼叫 `RestoreToIssued`，與 MarkFailed 同一交易
		//    ⇒ 不存在「operation 進 FAILED_PERMANENT 而 document 留在 VOIDING」的路徑。
		//    退款作廢（本波新增的 VOID adjustment）走的是同一條路徑，不變式維持。
		"VOIDING": "由驅動它的 VOID operation 在 M6 上代表；前提已於 #3073 重驗成立（見上方註解）",
	}

	var uncovered []string
	for _, s := range domain {
		n := 0
		if onAutomaticLine[s] {
			n++
		}
		if awaitingHuman[s] {
			n++
		}
		if _, ok := closedStatuses[s]; ok {
			n++
		}
		if n == 0 {
			uncovered = append(uncovered, s)
		}
	}
	if len(uncovered) > 0 {
		t.Fatalf("🔴 invoice_documents 的狀態 %v 沒有任何指標數得到，也沒有被宣告為已結案。\n"+
			"一份停在這些狀態的文件，法定上傳期限照樣在倒數，而沒有任何告警會亮。\n"+
			"這正是 AWAITING_MANUAL 在 2026-08-31 之前的處境（M6／M7／M8／M30／M31 與 "+
			"invoice_issue_total 六者全盲）。\n"+
			"修法二選一：① 納入 M7 或 M32 的述詞 ② 在 closedStatuses 寫下「為什麼沒有人需要再看它」。",
			uncovered)
	}

	// 反向：closedStatuses 不得列出值域裡不存在的狀態 —— 那種條目看起來像已經想過了，
	// 實際上是一個永遠不會被檢查的字串（同 ⑤ 的 selector 值域對帳）。
	for s := range closedStatuses {
		found := false
		for _, d := range domain {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("closedStatuses 列了 %q，但 chk_invdoc_status 的值域沒有它 —— "+
				"這條理由從來沒有被套用到任何東西上", s)
		}
	}

	// ①②必須互斥：同一個狀態被兩支指標數到會重複告警，而且代表兩者的分工沒想清楚。
	for _, s := range domain {
		if onAutomaticLine[s] && awaitingHuman[s] {
			t.Errorf("狀態 %q 同時落在 M7 與 M32 —— 兩者是**交棒**不是重疊", s)
		}
	}
}

// TestStatutoryDeadlineHandoverIsLossless 守「交棒不中斷」這一個不變式本身。
//
// 🔴 這支與上面那支的差別：上面驗「每個狀態都有歸屬」，這支驗**兩支指標的述詞真的形成
// 一條完整的鏈** —— 具體來說，MarkInvoiceDocumentAwaitingManual 的 allowed-from 裡
// 每一個狀態都必須在 M7 的述詞內，而它的目標狀態必須在 M32 的述詞內。
// 少了任何一半，那條合法轉換就是一次「告警由 firing 轉 resolved 而問題還在」。
func TestStatutoryDeadlineHandoverIsLossless(t *testing.T) {
	domain := checkConstraintValues(t, "chk_invdoc_status")
	m7 := statusesIn(exporterQuery(t, "invoice_statutory_deadline_pending"), domain)
	m32 := statusesIn(exporterQuery(t, "invoice_documents_awaiting_manual"), domain)

	if !m32["AWAITING_MANUAL"] {
		t.Fatal("M32 的述詞不含 AWAITING_MANUAL —— 那正是它存在的唯一理由")
	}

	// 從 SQL 原文取 allowed-from，不在這裡抄一份。
	queries := repoFile(t, "..", "..", "..", "db", "queries", "invoice_documents.sql")
	idx := strings.Index(queries, "name: MarkInvoiceDocumentAwaitingManual")
	if idx < 0 {
		t.Fatal("找不到 MarkInvoiceDocumentAwaitingManual —— 它被改名了，本檢查的錨失效")
	}
	stmt := queries[idx:]
	if end := strings.Index(stmt, ";"); end > 0 {
		stmt = stmt[:end]
	}
	whereIdx := strings.Index(stmt, "AND status IN")
	if whereIdx < 0 {
		t.Fatalf("MarkInvoiceDocumentAwaitingManual 的 allowed-from 解不出來（寫法變了）：\n%s", stmt)
	}
	from := statusesIn(stmt[whereIdx:], domain)
	if len(from) == 0 {
		t.Fatalf("allowed-from 是空集合？\n%s", stmt[whereIdx:])
	}

	for s := range from {
		if !m7[s] {
			t.Errorf("🔴 %q → AWAITING_MANUAL 是合法轉換，但 %q **不在** M7 的述詞內。\n"+
				"這代表該列在轉換前就已經沒有法定期限告警在看它了 —— 交棒的上游是斷的。", s, s)
		}
	}
}

// TestComplaintStatusCoverageHasNoSilentTerminal 是同一條規則套在第二張表上。
//
// 🔴 `invoice_complaints` 的四個非終態全都是「在等某個人」，而 2026-08-31 盤點時
// 整張表在 sql_exporter 與 business-alerts 的命中數**皆為 0** —— 與 AWAITING_MANUAL
// 同一個 root cause，只是換一張表。其中 ESCALATED_TO_ADM 最嚴重：它連一個到期時間欄
// 都沒有（sla_due_at 屬 OPEN、buyer_review_due_at 屬 SELLER_RESPONDED、overdue_at 屬
// OVERDUE），所以連「寫一支掃描器來撈它」都無從下手。
//
// ⚠️ 本測試與 document 那支刻意寫成兩份而不是一個 table-driven 迴圈：兩張表的「已結案」
//
//	理由完全不同，合併之後那些理由會被擠成一個 map 而失去可讀性——而那些理由正是
//	下一個人判斷「這格能不能塞進 closed」的唯一依據。
func TestComplaintStatusCoverageHasNoSilentTerminal(t *testing.T) {
	domain := checkConstraintValues(t, "chk_invcomp_status")
	unresolved := statusesIn(exporterQuery(t, "invoice_complaints_unresolved"), domain)

	closedStatuses := map[string]string{
		"RESOLVED":            "已結案；chk_invcomp_resolved_pair 保證 resolved_at 非空",
		"VIOLATION_CONFIRMED": "已裁決為違規並計入 90 天 3 次停權；chk_invcomp_violation_needs_decider 保證有裁決者",
		"WITHDRAWN":           "買家自行撤回，平台無後續義務",
	}

	var uncovered []string
	for _, s := range domain {
		if unresolved[s] {
			continue
		}
		if _, ok := closedStatuses[s]; ok {
			continue
		}
		uncovered = append(uncovered, s)
	}
	if len(uncovered) > 0 {
		t.Fatalf("🔴 invoice_complaints 的狀態 %v 沒有任何指標數得到，也沒有被宣告為已結案。\n"+
			"一則卡住的申訴 ＝ 一位買家始終沒拿到法定憑證，而賣家的違規計數也停在未定。\n"+
			"修法二選一：① 納入 M33 invoice_complaints_unresolved 的述詞 "+
			"② 在 closedStatuses 寫下「為什麼沒有人需要再看它」。", uncovered)
	}

	for s := range closedStatuses {
		found := false
		for _, d := range domain {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("closedStatuses 列了 %q，但 chk_invcomp_status 的值域沒有它", s)
		}
	}

	// ESCALATED_TO_ADM 單獨釘一次：它是本表最容易被漏掉的一格（沒有任何到期時間欄，
	// 所以任何「依到期欄掃描」的設計都會系統性略過它）。
	if !unresolved["ESCALATED_TO_ADM"] {
		t.Error("🔴 M33 的述詞不含 ESCALATED_TO_ADM —— 那是本表唯一連時間欄都沒有的等人狀態，" +
			"少了它，一則升級到 ADM 的申訴可以無限期躺著而沒有任何訊號")
	}
}

// TestAdjustmentStatusCoverageHasNoSilentTerminal 是 invoice_adjustments 的版本（#3073）。
//
// 🔴 這支在 #3073 之前**不需要存在**：那張表全表零列，因為 AdjustmentApplier 是
// Unimplemented。折讓執行軸落地之後它開始有列，而 ADM 對 invoice_adjustments
// **零可見性**（api/proto 與 internal/grpc 都零命中）⇒ 一個沒有指標數得到的狀態，
// 就是一筆「錢退了、憑證沒開、沒有人會知道」的資料。
func TestAdjustmentStatusCoverageHasNoSilentTerminal(t *testing.T) {
	domain := checkConstraintValues(t, "chk_invadj_status")

	// 卡住的由 invoice_adjustment_stuck_rows 承擔（告警 InvoiceAdjustmentStuck）。
	stuck := statusesIn(exporterQuery(t, "invoice_adjustment_stuck_rows"), domain)

	// 其餘每一項都必須寫下「為什麼沒有人需要再看它」。
	closed := map[string]string{
		"COMPLETED": "折讓／作廢已生效，累計已落帳（I4 的單句 CAS 與逐行 allowanced_amount）",
		"CANCELLED": "已決定不做；uq_invadj_* 的述詞都排除它 ⇒ 同一筆退款可以重新建列",

		// 這兩個有自己的自動執行者，停在那裡是正常過程而不是卡住。
		"SENDING":              "INV.op 的第三條分派軸持有租約中；租約到期會自己回到候選集合",
		"WAITING_CONFIRMATION": "INV.rc 的 ConfirmAllowance 在查；超過 24h 會自轉 FAILED_UNKNOWN 而落進 stuck 指標",

		// 🔴 這兩個今天**建不出來**：chk_invadj_allowance_shape 要求 ALLOWANCE 帶齊
		// 六欄，而它們的觸發條件正是「那六欄還算不出來」。退款路徑改走退避重試
		// （usecase/refund_adjustment.go 的 refundPlanRetryLater）。
		// 它們留在值域裡是因為 schema 的 CHECK 還列著；哪天有人讓它們建得出來，
		// 這一格的說明就變成假的 —— 那時 stuck 指標的述詞要一起加上它們。
		"BLOCKED_UNKNOWN_DOCUMENT":   "全 repo 零寫入者（chk_invadj_allowance_shape 讓它建不出來）",
		"BLOCKED_PENDING_CORRECTION": "同上",
	}

	var uncovered []string
	for _, s := range domain {
		if !stuck[s] {
			if _, ok := closed[s]; !ok {
				uncovered = append(uncovered, s)
			}
		}
	}
	if len(uncovered) > 0 {
		t.Fatalf("🔴 invoice_adjustments 的狀態 %v 沒有任何指標數得到，也沒有被宣告為已結案。\n"+
			"停在這些狀態的一列 ＝ 一筆退款的錢已經退了、稅務憑證沒開，而沒有任何告警會亮。\n"+
			"ADM 對這張表零可見性，所以「有人會看到」不成立。\n"+
			"修法二選一：① 納入 invoice_adjustment_stuck_rows 的述詞 "+
			"② 在 closed 寫下「為什麼沒有人需要再看它」。", uncovered)
	}
}
