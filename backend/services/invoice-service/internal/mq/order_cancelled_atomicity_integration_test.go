//go:build integration

// C3 的核心不變式：`order.cancelled` 的 handler 全程在**一個交易**內，所以
// invoice_inbox_events 上永遠不會留下一列非終態的 ORDER_CANCELLED。
//
// 🔴 為什麼一定要在真 DB 上驗：單元測試裡的 rollbackTx 是我自己寫的 snapshot/restore
// ——它證明的是「我的 fake 會還原」，不是「pgx 的交易會還原」。把 handler 改成兩個
// 獨立交易時，那個 fake 只要跟著改就照樣全綠。真 TxManager 沒有這個空間。
//
// 這一列若留得下來，它會是一列**沒有任何執行者**的孤兒：INV.dl 的兜底掃描
// （FetchDueByType）只撈 ORDER_PAID，而 M30 invoice_inbox_stranded_rows 要 7 天後
// 才看得到它——也就是說，失效與訊號之間有一週的靜默期。
package mq

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
)

func mqTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	db, _ := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// countInboxByType 回 (總列數, 非終態列數)。非終態述詞逐字取自 M30 的 sql_exporter
// 查詢與 idx_invinbox_due：RECEIVED / DEFERRED / FAILED。
func countInboxByType(ctx context.Context, t *testing.T, pool *pgxpool.Pool, eventType string) (total, nonTerminal int) {
	t.Helper()
	if err := pool.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE status IN ('RECEIVED','DEFERRED','FAILED'))
		   FROM invoice_inbox_events WHERE event_type = $1`, eventType).
		Scan(&total, &nonTerminal); err != nil {
		t.Fatalf("count inbox: %v", err)
	}
	return total, nonTerminal
}

// explodingCancelled 讓業務寫入失敗，用來觸發 rollback。它包住真的 repository，
// 所以除了 Insert 之外的一切行為都是真的。
type explodingCancelled struct {
	domain.CancelledOrderRepository
	boom bool
}

func (e *explodingCancelled) Insert(ctx context.Context, in domain.CancelledOrder) (bool, error) {
	if e.boom {
		return false, errors.New("integration: simulated downstream failure")
	}
	return e.CancelledOrderRepository.Insert(ctx, in)
}

// TestIntegration_OrderCancelledLeavesNoNonTerminalInboxRow 是本設計的載重測試。
func TestIntegration_OrderCancelledLeavesNoNonTerminalInboxRow(t *testing.T) {
	ctx := context.Background()
	pool := mqTestPool(t)
	tx := database.NewTxManager(pool)
	inbox := postgres.NewInboxRepository(pool)
	real := postgres.NewCancelledOrderRepository(pool)
	cancelled := &explodingCancelled{CancelledOrderRepository: real}

	c := NewOrderCancelledConsumer(tx, inbox, cancelled, nil)
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)
	body := producerShapedCancelledBody(t, map[string]any{"order_no": orderNo})
	evType := string(pkgmq.EventTypeOrderCancelled)

	// ① 下游失敗 ⇒ handler 回 error，而且**什麼都沒留下**。
	cancelled.boom = true
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("下游失敗時 handler 必須回 error（→ retry → DLQ）")
	}
	total, nonTerminal := countInboxByType(ctx, t, pool, evType)
	if total != 0 {
		t.Fatalf("rollback 之後 invoice_inbox_events 仍有 %d 列 ORDER_CANCELLED —— "+
			"去重鍵留下了，重投會被自己擋掉，訊息永遠進不了 DLQ", total)
	}
	if nonTerminal != 0 {
		t.Fatalf("留下 %d 列非終態 ORDER_CANCELLED —— 沒有任何執行者會處理它"+
			"（INV.dl 只掃 ORDER_PAID），它會在 7 天後才觸發 InvoiceInboxStranded", nonTerminal)
	}

	// ② 重投（pkg/mq 的 retry）必須真的重跑，而不是被去重擋掉之後假裝成功。
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("重投時下游仍然失敗，handler 卻回 nil —— 它被自己上一次留下的去重鍵擋掉了")
	}

	// ③ 下游恢復 ⇒ 同一則訊息成功，且 inbox 那一列直接是終態。
	cancelled.boom = false
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err != nil {
		t.Fatalf("下游恢復後重投應該成功：%v", err)
	}
	total, nonTerminal = countInboxByType(ctx, t, pool, evType)
	if total != 1 {
		t.Fatalf("inbox 有 %d 列，want 1", total)
	}
	if nonTerminal != 0 {
		t.Fatalf("成功之後仍有 %d 列非終態 —— MarkProcessed 沒有在同一個交易內生效", nonTerminal)
	}

	// ④ 業務事實真的落庫，而且讀路徑查得到（含 Tier A 的 buyer 綁定）。
	got, err := real.IsCancelledForBuyer(ctx, orderNo, 2623900520)
	if err != nil {
		t.Fatalf("IsCancelledForBuyer: %v", err)
	}
	if !got {
		t.Fatal("業務列沒有落庫 —— 買家仍會看到「付款完成後會立即開立發票」")
	}

	// ⑤ 再投一次（at-least-once）：不新增列、不報錯。
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err != nil {
		t.Fatalf("重複投遞應該被去重並 ACK：%v", err)
	}
	total, _ = countInboxByType(ctx, t, pool, evType)
	if total != 1 {
		t.Fatalf("重複投遞後 inbox 有 %d 列，want 1", total)
	}
}

// TestIntegration_OrderCancelledMarksProcessedInSameTx 是上一支的正向對照：
// 一次就成功的路徑，inbox 列必須是 PROCESSED 且 processed_at 非空。
//
// 🔴 少了 processed_at 的斷言，「把 MarkProcessed 拿掉」只會讓 status 停在 RECEIVED
// ——而上一支的 ④ 仍然會綠（業務列還在）。兩支合起來才蓋得住。
func TestIntegration_OrderCancelledMarksProcessedInSameTx(t *testing.T) {
	ctx := context.Background()
	pool := mqTestPool(t)
	c := NewOrderCancelledConsumer(
		database.NewTxManager(pool),
		postgres.NewInboxRepository(pool),
		postgres.NewCancelledOrderRepository(pool),
		nil,
	)
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)
	body := producerShapedCancelledBody(t, map[string]any{"order_no": orderNo})

	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var status string
	var processedAt *time.Time
	var aggregateRef *string
	if err := pool.QueryRow(ctx,
		`SELECT status, processed_at, aggregate_ref FROM invoice_inbox_events
		  WHERE event_type = $1 AND aggregate_ref = $2`,
		string(pkgmq.EventTypeOrderCancelled), orderNo).
		Scan(&status, &processedAt, &aggregateRef); err != nil {
		t.Fatalf("回查 inbox：%v", err)
	}
	if status != "PROCESSED" {
		t.Errorf("status = %q, want PROCESSED", status)
	}
	if processedAt == nil {
		t.Error("processed_at 是 NULL —— MarkProcessed 沒有真的跑")
	}
	if aggregateRef == nil || *aggregateRef != orderNo {
		t.Errorf("aggregate_ref = %v, want %q（ADM 靠它反查）", aggregateRef, orderNo)
	}
}
