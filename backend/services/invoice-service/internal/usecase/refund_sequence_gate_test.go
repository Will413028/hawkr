//go:build !integration

package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// P0-1：I5 序列閘門的兩半
//
// dev 實測（2026-09-05，evidence/A6/P0-orphan/）：同一張發票上有一筆
// FAILED_PERMANENT 的折讓 operation（AMEGO 永久拒絕、等 ADM 處理）時，
// 第二筆退款會建出 adjustment `7501688835508535296`（單號 A000000000010、PENDING）
// 而 **operation 一列都沒有**，inbox 標 PROCESSED、last_error 為 NULL ⇒ 完全靜默。
//
// 兩個成因，兩邊都要修：
//   ① 應用層閘門 correctionInFlight 只涵蓋 partial 索引 operation_type 述詞
//      4 種裡的 2 種（缺 ALLOWANCE／VOID_ALLOWANCE）⇒ 決策樹放行到取號與建列。
//   ② createOperation 對 Create 回 rows=0 一律當冪等 return nil ⇒ 撞序列閘門
//      這一半被靜默吞掉。
//
// 對照組（來自同一份 dev 證據）：`operation_key='allowance:7501686487977558016'`
// 命中 0（⇒ 不是同一把 key 重送），而 ALW-05 那筆的 key 命中 1（⇒ 查詢有效）。
// ---------------------------------------------------------------------------

// gateOp 造一列佔閘門用的 operation。狀態值硬寫字面值：期望值引用 codemeta 的話，
// 碼值被改掉測試照樣綠。
func gateOp(opType, status string) domain.Operation {
	return domain.Operation{ID: 9001, DocumentID: 5101, OperationType: opType, Status: status}
}

// TestAdjustmentInFlightMatchesPartialIndexTypes 守「應用層閘門 ⊇ 索引述詞」。
//
// 🔴 `uq_invop_open_adjustment_per_document` 的 operation_type 述詞是四種
// （VOID／REISSUE／ALLOWANCE／VOID_ALLOWANCE），狀態述詞排除五個
// （SUCCEEDED／NOT_APPLICABLE／CANCELLED_BEFORE_ISSUE／RESOLVED_EXTERNALLY／
// CANCELLED_BY_ADMIN）。**FAILED_PERMANENT 不在排除清單裡** —— 那不是邊界情況，
// 而是本閘門最常見的觸發原因（AMEGO 拒絕、等 ADM，可以是好幾天）。
func TestAdjustmentInFlightMatchesPartialIndexTypes(t *testing.T) {
	cases := []struct {
		opType, status string
		wantCorrection bool
		wantAdjustment bool
	}{
		// ALLOWANCE：只有 adjustmentInFlight 看得到它。
		{"ALLOWANCE", "FAILED_PERMANENT", false, true},
		{"ALLOWANCE", "PENDING", false, true},
		{"ALLOWANCE", "WAITING_CONFIRMATION", false, true},
		{"ALLOWANCE", "PROVIDER_STATUS_RECHECK", false, true},
		{"ALLOWANCE", "FAILED_UNKNOWN", false, true},
		// VOID_ALLOWANCE 在索引述詞內，即使今天沒有路徑建得出它。
		{"VOID_ALLOWANCE", "READY", false, true},
		// 五個排除值 ⇒ 兩邊都不擋。
		{"ALLOWANCE", "SUCCEEDED", false, false},
		{"ALLOWANCE", "NOT_APPLICABLE", false, false},
		{"ALLOWANCE", "CANCELLED_BEFORE_ISSUE", false, false},
		{"ALLOWANCE", "RESOLVED_EXTERNALLY", false, false},
		{"ALLOWANCE", "CANCELLED_BY_ADMIN", false, false},
		// correction 那一半維持原樣。
		{"VOID", "SENDING", true, false},
		{"REISSUE", "CREATED", true, false},
		{"VOID", "SUCCEEDED", false, false},
		// ISSUE 不在索引述詞內 ⇒ 兩邊都不擋（它走的是註 A 那條路）。
		{"ISSUE", "SENDING", false, false},
	}
	for _, tc := range cases {
		ops := []domain.Operation{gateOp(tc.opType, tc.status)}
		if got := correctionInFlight(ops); got != tc.wantCorrection {
			t.Errorf("correctionInFlight(%s/%s) = %v，want %v",
				tc.opType, tc.status, got, tc.wantCorrection)
		}
		if got := adjustmentInFlight(ops); got != tc.wantAdjustment {
			t.Errorf("adjustmentInFlight(%s/%s) = %v，want %v —— "+
				"應用層閘門比索引窄的那一格，症狀是「建列時才撞、而且撞出來是靜默的」",
				tc.opType, tc.status, got, tc.wantAdjustment)
		}
	}
}

// TestPlanRefundAdjustment_AdjustmentInFlightRetriesLater 守決策樹的新閘門。
func TestPlanRefundAdjustment_AdjustmentInFlightRetriesLater(t *testing.T) {
	f := voidableFacts()
	f.Doc.AllowanceTotalAmount = 300 // 已有折讓 ⇒ 本來會走 B 節點建列
	f.AdjustmentInFlight = true

	got := planRefundAdjustment(f)
	if got.Kind != refundPlanRetryLater {
		t.Fatalf("Kind = %d（%s），want RETRY_LATER —— 前一筆調整還佔著序列閘門時建列必然撞 DB，"+
			"而那時折讓單號已經取走了", got.Kind, got.Reason)
	}
	if got.Reason != "ADJUSTMENT_IN_FLIGHT" {
		t.Errorf("Reason = %q，want ADJUSTMENT_IN_FLIGHT —— "+
			"它會落進 inbox 的 last_error，與 CORRECTION_IN_FLIGHT 分開才看得出誰擋住了", got.Reason)
	}
}

// TestPlanRefundAdjustment_InFlightGateDoesNotShadowSkip 是上一支的**反面**，
// 也是這次改動最容易造成的退化。
//
// 🔴 新閘門刻意放在 status switch **之後**：SUPERSEDED／VOIDED 的 document 上常常
// 也掛著佔閘門的 operation（dev 實查：8 張裡有 7 張是這兩種），而那些退款本來就
// 「不需要任何發票調整」⇒ 必須維持 SKIP。放在 CorrectionInFlight 旁邊的話，
// 它們會變成無限退避重試。
func TestPlanRefundAdjustment_InFlightGateDoesNotShadowSkip(t *testing.T) {
	for _, status := range []string{"SUPERSEDED", "VOIDED", "NOT_APPLICABLE", "RESOLVED_EXTERNALLY"} {
		f := voidableFacts()
		f.Doc.Status = status
		f.AdjustmentInFlight = true

		got := planRefundAdjustment(f)
		if got.Kind != refundPlanSkip {
			t.Errorf("document=%s 時 Kind = %d（%s），want SKIP —— "+
				"沒有可調整的發票，退款本身不受影響；判成退避重試會讓它永遠留在 inbox",
				status, got.Kind, got.Reason)
		}
	}
}

// TestCreateOperation_SequenceGateHeldIsNotIdempotent 是 ② 的核心。
//
// Create 回 rows=0 的兩種意義處置完全相反，而分辨方式只有一個：回讀 operation_key。
func TestCreateOperation_SequenceGateHeldIsNotIdempotent(t *testing.T) {
	h := newRefundHarness(t)
	ctx := context.Background()
	h.ops.gateHeld = true // ON CONFLICT 命中 partial 索引：沒有任何列被插入

	err := h.uc.createOperation(ctx, 5101, 7101,
		codemeta.InvoiceOperationTypeAllowance.Value, refundTestNow)
	if err == nil {
		t.Fatal("撞序列閘門卻回 nil —— 那會留下一張**沒有執行者**的折讓單：" +
			"adjustment 是 PENDING、operation 一列都沒有、inbox 標 PROCESSED，" +
			"而唯一的訊號是 invoice_adjustment_stuck_rows 的 24h 述詞 ＋ 告警 for:1h ⇒ 最快 25 小時")
	}
	if len(h.ops.promoted) != 0 {
		t.Errorf("沒建成卻 promote 了 %d 次", len(h.ops.promoted))
	}
}

// TestCreateOperation_SameKeyReplayStaysIdempotent 是上一支的反面：
// 同一把 key 重送（重投／併發第二個副本）**必須**維持冪等，不得因為這次修復而回 error。
func TestCreateOperation_SameKeyReplayStaysIdempotent(t *testing.T) {
	h := newRefundHarness(t)
	ctx := context.Background()

	if err := h.uc.createOperation(ctx, 5101, 7101,
		codemeta.InvoiceOperationTypeAllowance.Value, refundTestNow); err != nil {
		t.Fatalf("第一次 createOperation: %v", err)
	}
	if len(h.ops.promoted) != 1 {
		t.Fatalf("第一次沒有 promote 到 READY（promoted=%d）", len(h.ops.promoted))
	}

	// 同一組 (documentID, adjustmentID) ⇒ 同一把 operation_key，那一列存在。
	if err := h.uc.createOperation(ctx, 5101, 7101,
		codemeta.InvoiceOperationTypeAllowance.Value, refundTestNow); err != nil {
		t.Fatalf("同一把 key 重送回了 error: %v —— 重投本來就該冪等", err)
	}
	if len(h.ops.promoted) != 1 {
		t.Errorf("重送時又 promote 了一次（promoted=%d）", len(h.ops.promoted))
	}
}

// TestApply_SequenceGateHeldSurfacesAsError 驗那個 error 真的傳到 Apply 的呼叫端
// （consumer 據此把 inbox 標 FAILED ＋ 退避重試，而不是 PROCESSED）。
func TestApply_SequenceGateHeldSurfacesAsError(t *testing.T) {
	h := newRefundHarness(t)
	h.ops.gateHeld = true

	if _, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc()); err == nil {
		t.Fatal("Apply 回 nil —— consumer 會把這則退款標成 PROCESSED，" +
			"買家的折讓單就此消失且沒有任何錯誤訊息")
	}
}
