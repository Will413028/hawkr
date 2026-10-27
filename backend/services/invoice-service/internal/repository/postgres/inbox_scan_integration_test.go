//go:build integration

// invoice_inbox_events 的兜底掃描三件套：FetchDueByType 的候選述詞、ClaimInboxEvent 的
// 租約 CAS、UpdateInboxEventAttempt／MarkInboxEventFailed 對候選集合的影響。
//
// 為什麼一定要在真 DB 上驗：這三支的每一種失效都**不是編譯錯也不是 Go 端測得到的**——
// CAS 少一個述詞照樣回 rows=1（於是兩個副本各跑一次 §7.1.2 步驟 5–9）、
// 述詞漏一個狀態值只是候選集合變小（於是一批列永遠不再被重試），兩者都完全靜默。
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// scanInbox 造一列 invoice_inbox_events。next_attempt_at 由呼叫端決定——它同時是候選
// 述詞的門檻與租約欄，本檔每一支測試都在調它。
func scanInbox(eventType string, nextAttemptAt time.Time) domain.NewInboxEvent {
	now := time.Now().UTC()
	stamp := fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano())
	return domain.NewInboxEvent{
		EventID:          "evt-" + stamp,
		EventType:        eventType,
		RoutingKey:       eventType,
		Exchange:         "hawkr.order",
		BusinessDedupKey: "biz-" + stamp,
		Payload:          json.RawMessage(`{"orderNo":"O1"}`),
		Status:           "RECEIVED",
		NextAttemptAt:    nextAttemptAt,
		OccurredAt:       now,
		Now:              now,
	}
}

func containsInboxID(evs []domain.InboxEvent, id int64) bool {
	for _, e := range evs {
		if e.ID == id {
			return true
		}
	}
	return false
}

func inboxStatusOf(ctx context.Context, t *testing.T, repo *InboxRepository, id int64) string {
	t.Helper()
	// 走 FetchDueByType 拿不到已離開候選集合的列，所以直接讀 DB。
	var status string
	if err := repo.pool.QueryRow(ctx, `SELECT status FROM invoice_inbox_events WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	return status
}

// ---------------------------------------------------------------------------
// ⑦ claim 是租約不是永久旗標（《多副本排程搶佔規範》§二 六步判準）
// ---------------------------------------------------------------------------

// TestIntegration_Inbox_ClaimIsALeaseNotAPermanentFlag 一次守三件事：
//
//	① CAS 有效——第二次 claim 必須失敗（拿掉 `next_attempt_at <= claimable_before`
//	   這一項，兩個副本會各拿到 rows=1，然後同時跑 §7.1.2 步驟 5–9）
//	② claim 把該列推出候選集合（否則根本不是搶佔）
//	③ 租約到期後該列**自己回到候選集合**——這一條把它與永久旗標分開：持有者中途
//	   崩潰（進程消失、沒有任何 catch 會跑）時，那張訂單的發票不會永遠開不出來
//
// 另外驗 claim **不得動 status**：狀態欄是可觀測資訊，不是閘門。
func TestIntegration_Inbox_ClaimIsALeaseNotAPermanentFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewInboxRepository(testPool(t))

	now := time.Now().UTC()
	id, inserted, err := repo.Insert(ctx, scanInbox("order.paid", now.Add(-time.Minute)))
	if err != nil || !inserted {
		t.Fatalf("Insert: id=%d inserted=%v err=%v", id, inserted, err)
	}

	due, err := repo.FetchDueByType(ctx, "order.paid", now, 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if !containsInboxID(due, id) {
		t.Fatal("已到期的 RECEIVED 列不在候選集合裡")
	}

	lease := 5 * time.Minute
	ok, err := repo.Claim(ctx, id, now.Add(lease), now)
	if err != nil || !ok {
		t.Fatalf("第一次 claim 應成功：ok=%v err=%v", ok, err)
	}

	// ① CAS：同一個 claimable_before 再 claim 一次必須失敗。
	if ok, err := repo.Claim(ctx, id, now.Add(lease), now); err != nil || ok {
		t.Fatalf("第二次 claim 應失敗（ok=%v err=%v）—— 租約 CAS 沒有生效，"+
			"兩個副本會同時重跑同一列", ok, err)
	}

	// ② 已離開候選集合。
	due, err = repo.FetchDueByType(ctx, "order.paid", now, 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if containsInboxID(due, id) {
		t.Fatal("claim 之後該列仍在候選集合裡 —— 這不是搶佔，兩個 pod 都會撈到")
	}

	// ③ 租約到期後自己回來（這是 lease 與永久旗標唯一的差別）。
	due, err = repo.FetchDueByType(ctx, "order.paid", now.Add(lease+time.Second), 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if !containsInboxID(due, id) {
		t.Fatal("租約到期後該列沒有回到候選集合 —— 這是永久旗標，持有者崩潰即永久卡死")
	}

	// claim 不得動 status。
	if got := inboxStatusOf(ctx, t, repo, id); got != "RECEIVED" {
		t.Fatalf("claim 動了 status（%s）：狀態欄是可觀測資訊不是閘門", got)
	}
}

// TestIntegration_Inbox_ClaimRejectsRowsThatLeftTheCandidateSet 守 CAS 的**另一項**述詞。
//
// 上一支只驗得到租約欄那一項：claim 之後 next_attempt_at 已被推到未來，所以就算
// `status IN (...)` 整個被拿掉，第二次 claim 一樣會失敗 —— 兩項的鑑別力必須分開驗。
//
// 這裡把 status 推成 PROCESSED 而**不動** next_attempt_at（MarkInboxEventProcessed 本來
// 就不碰它，它仍停在過去）⇒ 只剩狀態那一項擋得住。擋不住的後果是：一列在被撈出來之後、
// claim 之前被 consumer 的反向認領標成 PROCESSED，worker 仍然 claim 成功並重跑一次
// §7.1.2 步驟 5–9。
func TestIntegration_Inbox_ClaimRejectsRowsThatLeftTheCandidateSet(t *testing.T) {
	ctx := context.Background()
	repo := NewInboxRepository(testPool(t))

	now := time.Now().UTC()
	id, _, err := repo.Insert(ctx, scanInbox("order.paid", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if ok, err := repo.MarkProcessed(ctx, id, now); err != nil || !ok {
		t.Fatalf("MarkProcessed: ok=%v err=%v", ok, err)
	}

	// 前提：next_attempt_at 仍在過去（MarkProcessed 不得碰租約欄）。前提不成立的話，
	// 下面那個斷言會因為租約而通過，變成一支沒有鑑別力的測試。
	var nextAttemptAt time.Time
	if err := repo.pool.QueryRow(ctx,
		`SELECT next_attempt_at FROM invoice_inbox_events WHERE id=$1`, id).Scan(&nextAttemptAt); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !nextAttemptAt.Before(now) {
		t.Fatalf("MarkProcessed 動了租約欄（next_attempt_at=%v）—— 本測試失去鑑別力", nextAttemptAt)
	}

	if ok, err := repo.Claim(ctx, id, now.Add(5*time.Minute), now); err != nil || ok {
		t.Fatalf("已離開候選集合（PROCESSED）的列仍 claim 成功（ok=%v err=%v）—— "+
			"CAS 少了 status 那一項", ok, err)
	}
}

// TestIntegration_Inbox_FetchDueByTypeFiltersInSQL 守「型別過濾在 SQL 不在 Go」。
//
// 在 Go 端撈全部再 skip 的話，被跳過的列每一輪都會重新佔掉 batch 名額而永遠不推進
// ⇒ 一種事件的積壓會靜默吃光另一種的處理量。
func TestIntegration_Inbox_FetchDueByTypeFiltersInSQL(t *testing.T) {
	ctx := context.Background()
	repo := NewInboxRepository(testPool(t))

	now := time.Now().UTC()
	paidID, _, err := repo.Insert(ctx, scanInbox("order.paid", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Insert paid: %v", err)
	}
	refundID, _, err := repo.Insert(ctx, scanInbox("order.refund.finalized", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Insert refund: %v", err)
	}

	due, err := repo.FetchDueByType(ctx, "order.paid", now, 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if !containsInboxID(due, paidID) {
		t.Fatal("order.paid 的列不在候選集合裡")
	}
	if containsInboxID(due, refundID) {
		t.Fatal("order.refund.finalized 的列被撈進來了 —— 它的 AdjustmentApplier 仍是 " +
			"Unimplemented，掃它只會空轉燒 attempt 並把 DEFERRED 翻成 FAILED")
	}
}

// TestIntegration_Inbox_UpdateAttemptBacksOffWithoutTouchingStatus 守 UpdateAttempt 與
// MarkFailed 的分工：前者是「這一輪失敗、待會再試」，不得動 status。
func TestIntegration_Inbox_UpdateAttemptBacksOffWithoutTouchingStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewInboxRepository(testPool(t))

	now := time.Now().UTC()
	id, _, err := repo.Insert(ctx, scanInbox("order.paid", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	msg := "order-service unavailable"
	ok, err := repo.UpdateAttempt(ctx, id, 1, now.Add(2*time.Minute), &msg)
	if err != nil || !ok {
		t.Fatalf("UpdateAttempt: ok=%v err=%v", ok, err)
	}

	if got := inboxStatusOf(ctx, t, repo, id); got != "RECEIVED" {
		t.Fatalf("UpdateAttempt 動了 status（%s）—— 第一次失敗就變 FAILED，ADM 工作台會看到"+
			"一批其實還在自動重試的「失敗」", got)
	}

	var attempts int32
	var lastError *string
	if err := repo.pool.QueryRow(ctx,
		`SELECT attempt_count, last_error FROM invoice_inbox_events WHERE id=$1`, id).Scan(&attempts, &lastError); err != nil {
		t.Fatalf("scan: %v", err)
	}
	// 絕對值：Worker 傳的是它自己算好的 newAttempt，SQL 不得寫成 attempt_count + 1。
	if attempts != 1 {
		t.Errorf("attempt_count = %d，want 1（絕對值）", attempts)
	}
	if lastError == nil || *lastError != msg {
		t.Errorf("last_error = %v，want %q", lastError, msg)
	}

	// 退避期間不得再被撈到；退避到期才回來。
	if due, err := repo.FetchDueByType(ctx, "order.paid", now, 10); err != nil || containsInboxID(due, id) {
		t.Fatalf("退避期間仍在候選集合裡（err=%v）", err)
	}
	if due, err := repo.FetchDueByType(ctx, "order.paid", now.Add(3*time.Minute), 10); err != nil || !containsInboxID(due, id) {
		t.Fatalf("退避到期後沒有回到候選集合（err=%v）", err)
	}
}

// TestIntegration_Inbox_FailedStaysInCandidateSet 守 dlParkInterval 存在的前提。
//
// 🔴 候選述詞含 'FAILED' ⇒ **本表沒有真正的終態**。MarkFailed 若把 next_attempt_at 寫成
// now，下一個 tick 就會重撈、重打兩支 peer gRPC，變成沒有退避的熱迴圈；正因如此
// deadlineInboxRepo.MarkFailed 才寫 `now + parkInterval` 而不是透傳 now。
// 這一支釘住的是那個前提本身（FAILED 仍是候選）——它一旦不成立，parkInterval 就變成
// 一段沒有理由的程式碼，而且沒人會發現。
func TestIntegration_Inbox_FailedStaysInCandidateSet(t *testing.T) {
	ctx := context.Background()
	repo := NewInboxRepository(testPool(t))

	now := time.Now().UTC()
	id, _, err := repo.Insert(ctx, scanInbox("order.paid", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	park := 6 * time.Hour
	if ok, err := repo.MarkFailed(ctx, id, "gave up", now.Add(park)); err != nil || !ok {
		t.Fatalf("MarkFailed: ok=%v err=%v", ok, err)
	}
	if got := inboxStatusOf(ctx, t, repo, id); got != "FAILED" {
		t.Fatalf("status = %s，want FAILED", got)
	}
	if due, err := repo.FetchDueByType(ctx, "order.paid", now, 10); err != nil || containsInboxID(due, id) {
		t.Fatalf("停泊期間仍在候選集合裡（err=%v）", err)
	}
	if due, err := repo.FetchDueByType(ctx, "order.paid", now.Add(park+time.Second), 10); err != nil || !containsInboxID(due, id) {
		t.Fatalf("FAILED 的列在停泊到期後沒有回到候選集合（err=%v）—— 發票是法定義務，"+
			"「永遠不再試」沒有合法終點", err)
	}
}
