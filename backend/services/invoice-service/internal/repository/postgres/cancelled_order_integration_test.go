//go:build integration

// invoice_cancelled_orders 的兩件事：Tier A 的雙欄授權述詞，與 ON CONFLICT DO NOTHING
// 的去重。
//
// 為什麼一定要在真 DB 上驗：
//   - 授權述詞少一欄不是編譯錯、不是 Go 端測得到的（fake 的鍵是我自己定的），
//     它只是「WHERE 少一個條件」——查詢照樣成功、回 true，而那個 true 就是洩漏。
//   - ON CONFLICT DO NOTHING 的 rows=0 只有真 DB 回得出來；fake 的 map 判斷是另一回事。
//   - PK 與 CHECK 約束（buyer_id > 0）只在真 DB 上存在。
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

func cancelledFixture(orderNo string, buyerID int64) domain.CancelledOrder {
	now := time.Now().UTC()
	return domain.CancelledOrder{
		OrderNo:      orderNo,
		BuyerID:      buyerID,
		CancelReason: "PAYMENT_TIMEOUT",
		OccurredAt:   now.Add(-time.Minute),
		Now:          now,
	}
}

// TestIntegration_CancelledOrderIsScopedToItsBuyer 是 #2904 的 Tier A 陰性測試。
//
// 🔴 述詞若只有 order_no，本測試的第二段會回 true —— 那代表任何登入買家拿別人的
// order_no 來問都能確認「這張訂單存在且已取消」，而訂單號是可枚舉的。
func TestIntegration_CancelledOrderIsScopedToItsBuyer(t *testing.T) {
	ctx := context.Background()
	repo := NewCancelledOrderRepository(testPool(t))

	const owner, stranger = int64(2623900520), int64(2688637900)
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)

	inserted, err := repo.Insert(ctx, cancelledFixture(orderNo, owner))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !inserted {
		t.Fatal("第一次 Insert 應該回 inserted=true")
	}

	// 陽性：本人查得到。
	got, err := repo.IsCancelledForBuyer(ctx, orderNo, owner)
	if err != nil {
		t.Fatalf("IsCancelledForBuyer(owner): %v", err)
	}
	if !got {
		t.Fatal("本人查不到自己的取消紀錄 —— 讀路徑會退回錯誤文案")
	}

	// 🔴 陰性：別人查不到。這一段才是本測試的理由。
	got, err = repo.IsCancelledForBuyer(ctx, orderNo, stranger)
	if err != nil {
		t.Fatalf("IsCancelledForBuyer(stranger): %v", err)
	}
	if got {
		t.Fatal("🔴 別的買家拿這個 order_no 查得到取消紀錄 —— " +
			"授權述詞漏了 buyer_id，這是一個訂單存在性 oracle（Tier A／E21）")
	}

	// 不存在的訂單同樣回 false（與越權不可區分）。
	got, err = repo.IsCancelledForBuyer(ctx, orderNo+"X", owner)
	if err != nil {
		t.Fatalf("IsCancelledForBuyer(missing): %v", err)
	}
	if got {
		t.Fatal("不存在的訂單回了 true")
	}
}

// TestIntegration_CancelledOrderInsertIsIdempotent 釘住 at-least-once 的正常路徑。
func TestIntegration_CancelledOrderInsertIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewCancelledOrderRepository(testPool(t))

	const buyerID = int64(2623900520)
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)
	in := cancelledFixture(orderNo, buyerID)

	if _, err := repo.Insert(ctx, in); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// 第二次：ON CONFLICT DO NOTHING ⇒ 不得拋 23505（那會讓整個交易 aborted 25P02，
	// 而本 handler 在同一個交易內還有 MarkProcessed 要下）。
	inserted, err := repo.Insert(ctx, in)
	if err != nil {
		t.Fatalf("重投時 Insert 拋錯：%v —— ON CONFLICT DO NOTHING 沒有生效，"+
			"整個交易會 abort（25P02）而不只是這一句失敗", err)
	}
	if inserted {
		t.Fatal("第二次 Insert 應該回 inserted=false")
	}

	// 值沒有被第二次覆蓋（DO NOTHING 不是 DO UPDATE）。
	var reason string
	if err := testPoolQueryRow(ctx, t, repo, orderNo, &reason); err != nil {
		t.Fatalf("回查：%v", err)
	}
	if reason != "PAYMENT_TIMEOUT" {
		t.Errorf("cancel_reason = %q, want PAYMENT_TIMEOUT", reason)
	}
}

// TestIntegration_CancelledOrderRejectsNonPositiveBuyer 驗 DB 端的兜底。
//
// consumer 已經在應用層擋過（fail-loud），這裡驗的是「即使有人繞過 consumer 直接
// 寫入，DB 也不收」——buyer_id 是授權述詞，一列 buyer_id=0 的資料永遠對不到人，
// 它比沒有那一列更糟（看起來像已經修好了）。
func TestIntegration_CancelledOrderRejectsNonPositiveBuyer(t *testing.T) {
	ctx := context.Background()
	repo := NewCancelledOrderRepository(testPool(t))
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)

	if _, err := repo.Insert(ctx, cancelledFixture(orderNo, 0)); err == nil {
		t.Fatal("buyer_id=0 應該被 chk_invcanord_buyer 擋下")
	}
}

// TestIntegration_CancelledOrderNullReason 驗「買家自願取消」那條路徑（reason 為空）。
func TestIntegration_CancelledOrderNullReason(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewCancelledOrderRepository(pool)
	orderNo := fmt.Sprintf("O%d", time.Now().UnixNano()%1_000_000_000)

	in := cancelledFixture(orderNo, 2623900520)
	in.CancelReason = ""
	if _, err := repo.Insert(ctx, in); err != nil {
		t.Fatalf("cancel_reason 為空是正常值（買家自願取消未付款訂單）：%v", err)
	}
	var isNull bool
	if err := pool.QueryRow(ctx,
		`SELECT cancel_reason IS NULL FROM invoice_cancelled_orders WHERE order_no = $1`,
		orderNo).Scan(&isNull); err != nil {
		t.Fatalf("回查：%v", err)
	}
	if !isNull {
		t.Error("空字串應該折成 NULL：同一件事有兩種表示會讓之後的查詢兩邊都要寫")
	}
}

func testPoolQueryRow(ctx context.Context, t *testing.T, repo *CancelledOrderRepository, orderNo string, dst *string) error {
	t.Helper()
	return repo.pool.QueryRow(ctx,
		`SELECT cancel_reason FROM invoice_cancelled_orders WHERE order_no = $1`, orderNo).Scan(dst)
}
