//go:build integration

// 本檔跑真的 SQL，守「停泊而非終結」（甲）的兩半：
//
//	① 程式面：Park 之後那一列**仍在 status='pending'** ⇒ CountPending 與
//	   OldestPendingCreatedAt 還看得到它 ⇒ 告警 InvoiceOutboxLag 持續亮著；
//	   而 next_attempt_at 被推到未來 ⇒ 不是每 tick 重撈的熱迴圈，到期會自己回來。
//	② 監控面：**直接執行 monitoring/sql-exporter/sql_exporter.yml 裡那一支查詢**，
//	   不是自己抄一份。抄一份的話兩端都是我寫的、自己跟自己對必然全綠，而那正是
//	   2026-08-30 四個 P0 的共同結構性成因。
package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestIntegration_OutboxPark_KeepsTheRowInTheBacklogAndBringsItBack 是「永久旗標 ＋
// 告警方向相反」在**真 SQL** 上的證據。
//
// 舊行為 `SET status='failed', completed_at=now()` 會讓三件事同時發生，而三件都是靜默的：
// 該列退出候選集合（FetchPending／Claim 的述詞是 'pending'）、退出積壓指標
// （CountPending／OldestPendingCreatedAt 同一個述詞）、且永遠不會被清掉
// （DeleteDoneBefore 只刪 'done'）。
func TestIntegration_OutboxPark_KeepsTheRowInTheBacklogAndBringsItBack(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOutboxRepository(pool)

	rk := fmt.Sprintf("test.park.%d", time.Now().UnixNano())
	ev, err := repo.Insert(ctx, newOutbox(rk))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	const park = time.Hour
	now := time.Now().UTC()
	parked, err := repo.Park(ctx, ev.ID, now.Add(park), "publish hawkr.invoice/…: broker unreachable")
	if err != nil {
		t.Fatalf("Park: %v", err)
	}
	if !parked {
		t.Fatal("Park 回 rows=0：那一列不在 'pending'，本測試的前提就不成立")
	}

	// ① 仍在積壓指標裡 —— 這是告警不熄滅的**充要條件**。
	n, err := repo.CountPending(ctx)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if n < 1 {
		t.Fatal("停泊之後 CountPending 看不到那一列 —— outbox_pending_rows 歸零、" +
			"outbox_oldest_pending_age_seconds 一併消失，InvoiceOutboxLag 會在卡死那一刻 resolved")
	}
	oldest, err := repo.OldestPendingCreatedAt(ctx)
	if err != nil {
		t.Fatalf("OldestPendingCreatedAt: %v", err)
	}
	if oldest == nil {
		t.Fatal("停泊之後最舊未派送列變成 nil —— 積壓年齡會被寫成 0，告警反向")
	}

	// ② 停泊期間不重撈（沒有熱迴圈）。
	due, err := repo.FetchPending(ctx, now.Add(park-time.Minute), 100)
	if err != nil {
		t.Fatalf("FetchPending: %v", err)
	}
	if containsOutbox(due, ev.ID) {
		t.Fatal("停泊未到期就被撈回來了 —— 那是一條沒有退避的熱迴圈")
	}

	// ③ 到期之後自己回到候選集合 —— 這是「不是永久旗標」的定義。
	due, err = repo.FetchPending(ctx, now.Add(park+time.Minute), 100)
	if err != nil {
		t.Fatalf("FetchPending: %v", err)
	}
	if !containsOutbox(due, ev.ID) {
		t.Fatal("停泊到期之後那一列沒有回到候選集合 —— 那就是永久旗標，" +
			"broker 修好之後也不會自己排空")
	}

	// ④ last_error 有落地（維運唯一需要的線索）。
	var lastErr *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM invoice_outbox_events WHERE id=$1`, ev.ID).Scan(&lastErr); err != nil {
		t.Fatalf("read last_error: %v", err)
	}
	if lastErr == nil || !strings.Contains(*lastErr, "broker unreachable") {
		t.Fatalf("last_error = %v —— 耗盡重試那一刻正是維運唯一需要那句話的時候", lastErr)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_outbox_events WHERE routing_key=$1`, rk)
	})
}

// TestIntegration_ExporterStrandedQueriesRunAndDiscriminate 直接跑 sql_exporter.yml 裡
// M30／M31 那兩支查詢。
//
// 🔴 兩件事一起驗，缺一都會讓這支變成裝飾：
//   - **能跑**：查詢是手寫在 YAML 裡的，沒有任何東西保證它對得上 schema。打錯一個欄名
//     的下場是那個 metric 永久缺席，而 `sum(...) > 0` 對「metric 不存在」永遠是 false
//     ⇒ 告警看起來安安靜靜。
//   - **有鑑別力**：塞一列「應該被數到」的資料進去，count 必須 > 0。只驗「能跑」的話，
//     一支 `WHERE false` 也會通過。
//
// M30 一起驗是因為它就是 M31 的來源：M31 存在的理由正是 M30 的 FROM 只有 inbox，
// 對 outbox 表結構性全盲。兩支並排跑，那個盲區才不會再長回來。
func TestIntegration_ExporterStrandedQueriesRunAndDiscriminate(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	outboxQ := exporterQuery(t, "invoice_outbox_stranded_rows")
	inboxQ := exporterQuery(t, "invoice_inbox_stranded_rows")

	// 鑑別力自檢：兩支必須真的打在不同的表上（M31 的存在理由）。
	if !strings.Contains(outboxQ, "invoice_outbox_events") {
		t.Fatalf("invoice_outbox_stranded_rows 的 FROM 不是 invoice_outbox_events：\n%s", outboxQ)
	}
	if !strings.Contains(inboxQ, "invoice_inbox_events") {
		t.Fatalf("invoice_inbox_stranded_rows 的 FROM 不是 invoice_inbox_events：\n%s", inboxQ)
	}

	rk := fmt.Sprintf("test.stranded.%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_outbox_events WHERE routing_key=$1`, rk)
	})

	base := countStranded(t, ctx, pool, outboxQ)

	// (a) 維運 SQL 隔離出來的 'failed' —— 程式已不再寫這個值，而本表沒有任何查詢會撈它。
	repo := NewOutboxRepository(pool)
	quarantined, err := repo.Insert(ctx, newOutbox(rk))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_outbox_events SET status='failed' WHERE id=$1`, quarantined.ID); err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	if got := countStranded(t, ctx, pool, outboxQ); got != base+1 {
		t.Fatalf("隔離一列之後 M31 = %.0f，want %.0f。\n"+
			"這一列結構上沒有任何執行者（FetchPending／Claim 都是 status='pending'，"+
			"DeleteDoneBefore 只刪 'done'），M31 是它唯一的能見度。查詢：\n%s", got, base+1, outboxQ)
	}

	// (b) 停泊了一整週還沒好的 'pending'。
	longParked, err := repo.Insert(ctx, newOutbox(rk))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_outbox_events SET created_at = now() - interval '8 days' WHERE id=$1`,
		longParked.ID); err != nil {
		t.Fatalf("age the row: %v", err)
	}
	if got := countStranded(t, ctx, pool, outboxQ); got != base+2 {
		t.Fatalf("再加一列停泊超過 7 天之後 M31 = %.0f，want %.0f。查詢：\n%s", got, base+2, outboxQ)
	}

	// (c) 陰性：剛建立、還在正常退避的 'pending' **不得**被數進來，否則本指標會恆紅、
	//     然後被調靜音 —— 那比沒有指標更糟。
	fresh, err := repo.Insert(ctx, newOutbox(rk))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	_ = fresh
	if got := countStranded(t, ctx, pool, outboxQ); got != base+2 {
		t.Fatalf("一列剛建立的 pending 被算成 stranded（M31 = %.0f，want %.0f）—— "+
			"指標會恆紅，而恆紅的指標等於沒有指標", got, base+2)
	}

	// M30 只驗「跑得起來」：它的鑑別力屬於 inbox 那一側，這裡要的是它沒有壞掉。
	countStranded(t, ctx, pool, inboxQ)
}

func containsOutbox(rows []domain.OutboxEvent, id int64) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// countStranded 跑一支 exporter 查詢並把各 label 組的 cnt 加總。
//
// 這裡刻意用 `pool.Query` 跑**原文**，不做任何改寫：改寫過的查詢驗的是我的改寫，
// 不是 exporter 真的會送出去的那一句。
func countStranded(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) float64 {
	t.Helper()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("exporter 查詢跑不起來（metric 會永久缺席，而 `sum(...) > 0` 對缺席的 "+
			"metric 永遠是 false ⇒ 告警看起來安安靜靜）：%v\n查詢：\n%s", err, query)
	}
	defer rows.Close()
	total := 0.0
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
			t.Fatalf("最後一欄不是 float8（sql-exporter 的 values 只吃數值）：%T\n查詢：\n%s",
				vals[len(vals)-1], query)
		}
		total += cnt
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate exporter 查詢結果：%v", err)
	}
	return total
}
