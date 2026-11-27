//go:build !integration

package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

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
		{"ALLOWANCE", "FAILED_PERMANENT", false, true},
		{"ALLOWANCE", "PENDING", false, true},
		{"ALLOWANCE", "WAITING_CONFIRMATION", false, true},
		{"ALLOWANCE", "PROVIDER_STATUS_RECHECK", false, true},
		{"ALLOWANCE", "FAILED_UNKNOWN", false, true},
		{"VOID_ALLOWANCE", "READY", false, true},
		{"ALLOWANCE", "SUCCEEDED", false, false},
		{"ALLOWANCE", "NOT_APPLICABLE", false, false},
		{"ALLOWANCE", "CANCELLED_BEFORE_ISSUE", false, false},
		{"ALLOWANCE", "RESOLVED_EXTERNALLY", false, false},
		{"ALLOWANCE", "CANCELLED_BY_ADMIN", false, false},
		{"VOID", "SENDING", true, false},
		{"REISSUE", "CREATED", true, false},
		{"VOID", "SUCCEEDED", false, false},
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
	f.Doc.AllowanceTotalAmount = 300
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
	h.ops.gateHeld = true

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
