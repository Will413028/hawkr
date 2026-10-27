//go:build integration

// 第四個靜默終點：invoice_documents.status = 'AWAITING_MANUAL'。
//
// 本檔跑真 SQL ＋ **monitoring/sql-exporter/sql_exporter.yml 的查詢原文**，證明一件事：
// 一份進了 AWAITING_MANUAL 的文件，法定上傳期限照樣在倒數，而**六支 invoice 指標一支都
// 數不到它**——包含唯一與罰款直接相關的那兩條告警。
//
// 與甲（outbox status='failed'）是同一個 class 的第四個實例，但更嚴重：
//
//	甲：InvoiceOutboxLag 先 firing、再於卡死那一刻 resolved（有過訊號）
//	本：**從頭到尾沒有任何告警亮過**，而後果是賣家被罰 2%／2.5%／3%
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// exporterBuckets 跑一支 exporter 查詢原文，回「label 組 → cnt」。
//
// 與 countStranded 的差別是**不加總**：M7 的三個 bucket 必須分開看，加總會把
// 「overdue 掉到 0、lt6h 升 1」這種桶間搬移看成沒有變化。
func exporterBuckets(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) map[string]float64 {
	t.Helper()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("exporter 查詢跑不起來（metric 會永久缺席，而告警對缺席的 metric 永遠 false）：%v\n查詢：\n%s", err, query)
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			t.Fatalf("讀 exporter 查詢結果：%v", err)
		}
		if len(vals) == 0 {
			t.Fatalf("exporter 查詢回了零欄的列：\n%s", query)
		}
		cnt, ok := vals[len(vals)-1].(float64)
		if !ok {
			t.Fatalf("最後一欄不是 float8：%T\n查詢：\n%s", vals[len(vals)-1], query)
		}
		key := ""
		for _, v := range vals[:len(vals)-1] {
			key += fmt.Sprintf("%v|", v)
		}
		out[key] += cnt
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate exporter 查詢結果：%v", err)
	}
	return out
}

func sumBuckets(m map[string]float64) float64 {
	t := 0.0
	for _, v := range m {
		t += v
	}
	return t
}

// seedOverdueDocument 建一份**已逾法定上傳期限**的 B2C 訂單發票。
//
// chk_invdoc_deadline 要求 statutory_deadline_at > issue_not_before ⇒ 兩個時點一起往前推。
func seedOverdueDocument(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	issuerID, buyerID int64, orderNo, status string) int64 {
	t.Helper()
	id := nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_documents (
		   id, issuer_id, issuer_name_snapshot, source_type, source_ref, trigger_type,
		   recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
		   document_kind, fulfillment_mode, status,
		   snapshot_version, issue_attempt_seq,
		   sales_amount, tax_amount, total_amount, tax_type, tax_rate_bps,
		   provider, issue_not_before, statutory_deadline_at)
		 VALUES ($1, $2, 'seed-issuer', 'ORDER', $3, 'AUTO',
		         'BUYER', $4, '消費者', '0000000000',
		         'B2C', 'AMEGO_AUTO', $5,
		         1, 1,
		         100, 0, 100, 1, 500,
		         'AMEGO', now() - interval '3 days', now() - interval '1 day')`,
		id, issuerID, orderNo, buyerID, status)
	return id
}

// seedProcessedOrderPaid 建一列已處理完的 ORDER_PAID inbox 事件（M8 的分母）。
func seedProcessedOrderPaid(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orderNo string) {
	t.Helper()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_inbox_events (
		   event_id, event_type, routing_key, exchange, business_dedup_key,
		   aggregate_ref, payload, status, occurred_at, received_at, processed_at)
		 VALUES ($1, 'ORDER_PAID', 'order.paid', 'hawkr.order', $2,
		         $3, '{}'::jsonb, 'PROCESSED',
		         now() - interval '3 days', now() - interval '3 days', now() - interval '3 days')`,
		fmt.Sprintf("evt-%d-%s", time.Now().UnixNano(), orderNo), orderNo, orderNo)
}

// TestIntegration_AwaitingManualIsInvisibleToEveryInvoiceMetric 是第四個靜默終點的證據。
//
// 🔴 鑑別力來自「同一列、只改 status」的前後對照：ISSUE_FAILED 時 M7 的 overdue 桶數得到它，
// 翻成 AWAITING_MANUAL 之後掉回 0 ——**這正是告警方向相反的定義**，而且與甲不同的是
// 之後沒有任何一支指標接手。
func TestIntegration_AwaitingManualIsInvisibleToEveryInvoiceMetric(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	m6 := exporterQuery(t, "invoice_pending_operations")
	m7 := exporterQuery(t, "invoice_statutory_deadline_pending")
	m8 := exporterQuery(t, "invoice_paid_orders_without_operation")
	m30 := exporterQuery(t, "invoice_inbox_stranded_rows")
	m31 := exporterQuery(t, "invoice_outbox_stranded_rows")
	m32 := exporterQuery(t, "invoice_documents_awaiting_manual")

	base7 := exporterBuckets(t, ctx, pool, m7)
	base6 := sumBuckets(exporterBuckets(t, ctx, pool, m6))
	base8 := sumBuckets(exporterBuckets(t, ctx, pool, m8))
	base30 := sumBuckets(exporterBuckets(t, ctx, pool, m30))
	base31 := sumBuckets(exporterBuckets(t, ctx, pool, m31))
	baseAM := exporterBuckets(t, ctx, pool, m32)

	storeID := nextIssuerRefID()
	issuerID := seedIssuerFor(ctx, t, pool, storeID)
	orderNo := fmt.Sprintf("O-AM-%d", time.Now().UnixNano())
	seedProcessedOrderPaid(ctx, t, pool, orderNo)
	docID := seedOverdueDocument(ctx, t, pool, issuerID, nextIssuerRefID(), orderNo, "ISSUE_FAILED")

	// ── 陽性對照：ISSUE_FAILED ＋ 已逾期 ⇒ M7 的 overdue 桶看得到它。
	got := exporterBuckets(t, ctx, pool, m7)
	if got["overdue|"] != base7["overdue|"]+1 {
		t.Fatalf("陽性對照失敗：ISSUE_FAILED 且已逾期，M7 overdue = %.0f，want %.0f。\n"+
			"本測試的鑑別力建立在這一步，它不成立的話後面的「掉到 0」什麼都證明不了。\n查詢：\n%s",
			got["overdue|"], base7["overdue|"]+1, m7)
	}

	if n := exporterBuckets(t, ctx, pool, m32)["overdue|"]; n != baseAM["overdue|"] {
		t.Fatalf("陽性對照失敗：文件還在 ISSUE_FAILED，M32 overdue 就已經是 %.0f —— "+
			"兩支指標重疊了，會重複告警", n)
	}

	// ── 翻成 AWAITING_MANUAL（allowed-from 逐字含 ISSUE_FAILED，見
	//    db/queries/invoice_documents.sql 的 MarkInvoiceDocumentAwaitingManual）。
	mustExec(ctx, t, pool,
		`UPDATE invoice_documents
		 SET status='AWAITING_MANUAL', manual_reason_code='MIXED_TAX_CATEGORY'
		 WHERE id=$1`, docID)

	// ① M7：唯一與罰款直接相關的兩條告警（InvoiceStatutoryDeadlineApproaching／Overdue）
	//    的資料面。述詞 status IN ('PENDING','ISSUING','ISSUE_FAILED') 不含 AWAITING_MANUAL
	//    ⇒ 該列在轉換的那一刻**退出** M7。這是預期行為（M7 的 ③ 說明了為什麼不能含它），
	//    但它必須是**交棒**而不是**掉地上** —— ② 驗的就是那一半。
	got = exporterBuckets(t, ctx, pool, m7)
	if got["overdue|"] != base7["overdue|"] {
		t.Fatalf("M7 overdue = %.0f，want %.0f：AWAITING_MANUAL 竟然還留在 M7 裡。\n"+
			"若這是刻意加進去的，那條 critical 告警會永久觸發（M7 註解 ③），"+
			"而永久紅的告警會被靜音 ⇒ 等於沒有。", got["overdue|"], base7["overdue|"])
	}

	// ② 🔴 交棒：M32 必須在同一刻接手。這一項是本檔的**主張**本身 ——
	//    2026-08-31 之前它是 0，於是 InvoiceStatutoryDeadlineOverdue（critical）
	//    在票還沒開出去的情況下由 firing 轉 resolved，之後沒有任何指標數得到那一列。
	gotAM := exporterBuckets(t, ctx, pool, m32)
	if gotAM["overdue|"] != baseAM["overdue|"]+1 {
		t.Fatalf("🔴 M32 overdue = %.0f，want %.0f：一份**已逾法定上傳期限**的文件翻進 "+
			"AWAITING_MANUAL 之後沒有任何指標接手。\n"+
			"它同時退出了 M7（③）、不建 operation 所以 M6（⑥）看不到、"+
			"invoice_issue_total 一次都不會加所以 ⑤ 與 ⑰ 也看不到 ⇒ 法定期限會在完全靜默中過去。\n查詢：\n%s",
			gotAM["overdue|"], baseAM["overdue|"]+1, m32)
	}

	// ③ M6：AWAITING_MANUAL 的建立邊**不建 operation**（usecase/issuance.go 的
	//    plan.makeOp = false）⇒ 待開票積壓指標結構上看不到這一類。
	//    這裡斷言「看不到」不是為了接受它，是為了釘住 M32 存在的理由：
	//    哪天有人以為 M6 蓋得住而想刪掉 M32，這一行會告訴他 M6 從來沒蓋住過。
	if n := sumBuckets(exporterBuckets(t, ctx, pool, m6)); n != base6 {
		t.Errorf("M6 = %.0f（base %.0f）：AWAITING_MANUAL 現在會建 operation 了？"+
			"那 M32 與 M6 的分工要重新想過（可能重複告警）", n, base6)
	}

	// ④ M8：NOT EXISTS 的對象是 invoice_documents ⇒ 只要「有一份文件」就出桶，
	//    不問那份文件是不是還需要人。同 ③，這裡釘的是「M8 蓋不住」這個事實。
	if n := sumBuckets(exporterBuckets(t, ctx, pool, m8)); n != base8 {
		t.Errorf("M8 = %.0f（base %.0f）：述詞變了，M32 的分工要重新想過", n, base8)
	}

	// ⑤⑥ M30／M31 是 inbox／outbox 兩張表，對 invoice_documents 結構性全盲。
	if n := sumBuckets(exporterBuckets(t, ctx, pool, m30)); n != base30 {
		t.Errorf("M30 意外變動：%.0f → %.0f", base30, n)
	}
	if n := sumBuckets(exporterBuckets(t, ctx, pool, m31)); n != base31 {
		t.Errorf("M31 意外變動：%.0f → %.0f", base31, n)
	}
}
