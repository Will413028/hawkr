//go:build !integration

package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/domain/fsm"
)

// ---------------------------------------------------------------------------
// P0-2：ADM 取消折讓 job 之後，operation 必須終結
//
// dev 實測（2026-09-05，evidence/A6/ALW-03/g15-experiment.txt）的症狀：
// 端點 53 取消一筆 PENDING 的折讓 job ⇒ operation CANCEL_REQUESTED／adjustment CANCELLED
// （兩者同一交易，正確）。24 小時後 INV.rc 把它 claim 進 PROVIDER_STATUS_RECHECK ⇒
// ConfirmAllowance ⇒ allowance_query 回 71 ⇒ scheduleAllowanceRecheck 的 G15 ⇒
// 第一句 MarkFailedUnknown 的 allowed-from 只有 WAITING_CONFIRMATION ⇒ rows=0 ⇒
// **整個 WithTx rollback** ⇒ operation 停在 PROVIDER_STATUS_RECHECK、next_attempt_at
// 只被 claim 的租約推兩分鐘 ⇒ 每 ~2 分鐘真的打一次 AMEGO，永遠。
// 佐證：invoice_status_history 在該時點之後零列、invoice_provider_audit_logs 9 筆外呼。
//
// 這條軸的正解不是「讓 adjustment 也能被推」（CANCELLED → FAILED_UNKNOWN 在 M3 上是
// 一條假的邊），而是走 operation 自己該走的那條終結邊 —— M2 的 J24。
// ---------------------------------------------------------------------------

// alwCancelUnknownSince 是「已經未明 25 小時」的事實時間戳（testNow = 2026-08-30 02:00Z）。
// 硬寫字面值而不是 testNow.Add(-recheckGiveUpAfter-1h)：期望值引用被測物引用的常數時，
// 把那個常數改掉測試照樣綠（§0.3 第二種形態）。
var alwCancelUnknownSince = time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)

// alwAdminCancelSetup 把 harness 擺成「ADM 取消在途 ＋ INV.rc 已 claim」的真實時序，
// 回傳要傳進 ConfirmAllowance 的**快照**（claim 之前那一份）。
func alwAdminCancelSetup(h *alwHarness) domain.Operation {
	// 端點 53 的結果：operation CANCEL_REQUESTED、adjustment CANCELLED（同一交易）。
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusCancelled.Value

	requestedAt := time.Date(2026, 8, 29, 0, 30, 0, 0, time.UTC)
	requestedBy := int64(2600000022)
	unknownSince := alwCancelUnknownSince

	h.ops.op.CancelRequestedAt = &requestedAt
	h.ops.op.CancelRequestedBy = &requestedBy
	h.ops.op.CancelConfirmedAt = nil
	h.ops.op.UnknownSince = &unknownSince

	snapshot := h.ops.op
	snapshot.Status = codemeta.InvoiceOperationStatusCancelRequested.Value
	// ClaimInvoiceRecheck 已經把 DB 上那一列蓋成 PROVIDER_STATUS_RECHECK（J22）。
	h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	return snapshot
}

// TestAllowance_AdminCancelWithProviderQuery71ReachesCancelledByAdmin 守 J24。
//
// 🔴 三個斷言缺一都會讓 P0-2 復活：
//   - operation 必須離開 PROVIDER_STATUS_RECHECK（不然就是熱迴圈本身）
//   - cancel_evidence_kind 必須是 PROVIDER_QUERY_71（chk_invop_admin_cancel_needs_confirm
//     放行 J24 的唯一依據，J24 沒有第二個人 ⇒ 不帶就是 23514）
//   - cancel_confirmed_at 必須有值（不然「ADM 取消在途」判別式永遠成立）
func TestAllowance_AdminCancelWithProviderQuery71ReachesCancelledByAdmin(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	snapshot := alwAdminCancelSetup(h)

	h.prov.allowanceQueryFn = func(string) (*amego.AllowanceDetail, error) {
		return nil, &amego.APIError{
			Endpoint: "/json/allowance_query", Code: "71",
			Class: amego.ClassPermanent, NotFound: true, KnownCode: true, Msg: "查無資料",
		}
	}

	if err := h.uc.ConfirmAllowance(ctx, snapshot); err != nil {
		t.Fatalf("ConfirmAllowance: %v —— 回 error 代表整個交易 rollback，"+
			"而那正是 dev 上那個每 2 分鐘打一次 AMEGO 的熱迴圈", err)
	}

	if got := h.ops.op.Status; got != "CANCELLED_BY_ADMIN" {
		t.Fatalf("operation = %q，want CANCELLED_BY_ADMIN —— 停在 PROVIDER_STATUS_RECHECK "+
			"就是永久熱迴圈：ADM 的重試／取消兩顆按鈕都是灰的，而 "+
			"invoice_adjustment_stuck_rows 看的是 adjustment（已終態）⇒ 零告警", got)
	}
	if h.ops.gotResolved == nil {
		t.Fatal("沒有經 MarkRecheckResolved 收尾")
	}
	got := *h.ops.gotResolved
	if got.CancelEvidenceKind == nil || *got.CancelEvidenceKind != "PROVIDER_QUERY_71" {
		t.Errorf("cancel_evidence_kind = %v，want PROVIDER_QUERY_71 —— "+
			"J24 沒有第二個人（cancel_confirmed_by 恆 NULL），"+
			"這個值是 chk_invop_admin_cancel_needs_confirm 放行它的唯一依據，不帶就是 23514",
			got.CancelEvidenceKind)
	}
	if got.CancelConfirmedAt == nil {
		t.Errorf("cancel_confirmed_at 沒有寫入 —— 「ADM 取消在途」判別式" +
			"（cancel_requested_at IS NOT NULL AND cancel_confirmed_at IS NULL）會永遠成立")
	}
	if got.CompletedAt == nil {
		t.Errorf("completed_at 沒有寫入")
	}

	// adjustment 那一半刻意不動：它已經是 CANCELLED（M3 終態），硬推只會 rows=0，
	// 而把那個 rows=0 當錯誤正是本缺陷的成因。
	if s := h.adjs.adj.Status; s != codemeta.InvoiceAdjustmentStatusCancelled.Value {
		t.Errorf("adjustment = %q，want 留在 CANCELLED", s)
	}
	if h.prov.allowanceCalls != 0 {
		t.Errorf("取消在途卻重送了 %d 次折讓", h.prov.allowanceCalls)
	}
}

// TestAllowance_AdminCancelStillUnknownGoesCancelBlockedUnknown 守 J26。
//
// 71 以外的失敗（逾時、51、憑證解不開）**不是**「確定沒開」。把它們一起判成沒開，
// 會在折讓其實已經成立時把 operation 收掉；正解是進 CANCEL_BLOCKED_UNKNOWN
// 等 ADM 雙人確認（J28），而 J27 會把它推回重查。
func TestAllowance_AdminCancelStillUnknownGoesCancelBlockedUnknown(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()
	snapshot := alwAdminCancelSetup(h)

	h.prov.allowanceQueryFn = func(string) (*amego.AllowanceDetail, error) {
		return nil, &amego.APIError{
			Endpoint: "/json/allowance_query", Code: "51",
			Class: amego.ClassReconcile, KnownCode: true, Msg: "系統忙碌",
		}
	}

	if err := h.uc.ConfirmAllowance(ctx, snapshot); err != nil {
		t.Fatalf("ConfirmAllowance: %v", err)
	}
	if got := h.ops.op.Status; got != "CANCEL_BLOCKED_UNKNOWN" {
		t.Fatalf("operation = %q，want CANCEL_BLOCKED_UNKNOWN", got)
	}
	if h.ops.recheckUnresolvedCnt != 1 {
		t.Errorf("MarkRecheckUnresolved 呼叫 %d 次，want 1", h.ops.recheckUnresolvedCnt)
	}
	if h.ops.gotResolved != nil {
		t.Errorf("不確定卻走了 MarkRecheckResolved（J24）—— 那會在折讓其實已成立時把它收掉")
	}
	// 25h 已過階梯頂（>= 2h ⇒ 30 分鐘），期望值硬寫。
	want := time.Date(2026, 8, 30, 2, 30, 0, 0, time.UTC)
	if !h.ops.gotUnresolvedNextAt.Equal(want) {
		t.Errorf("next_attempt_at = %v，want %v（退避階梯觸頂 30m）",
			h.ops.gotUnresolvedNextAt, want)
	}
}

// TestAllowanceAdminCancelEdgesExistInM2 是「我走的邊真的在轉換表上」的機械證據。
//
// 🔴 字面值硬寫，**不從 codemeta 取**：那樣寫的話碼值被改掉測試照樣綠，
// 而這支要驗的正是「§7.3.2 的 J24／J26 是不是這兩條」。
func TestAllowanceAdminCancelEdgesExistInM2(t *testing.T) {
	for _, tc := range []struct{ code, from, to string }{
		{"J24", "PROVIDER_STATUS_RECHECK", "CANCELLED_BY_ADMIN"},
		{"J26", "PROVIDER_STATUS_RECHECK", "CANCEL_BLOCKED_UNKNOWN"},
	} {
		tr, ok := fsm.OperationTransitionByCode(tc.code)
		if !ok {
			t.Fatalf("%s 不在 OperationTransitions 上", tc.code)
		}
		if tr.To != tc.to {
			t.Errorf("%s.To = %q，want %q", tc.code, tr.To, tc.to)
		}
		found := false
		for _, f := range tr.From {
			if f == tc.from {
				found = true
			}
		}
		if !found {
			t.Errorf("%s.From = %v，不含 %q", tc.code, tr.From, tc.from)
		}
		if !fsm.CanTransitionOperation(tc.from, tc.to) {
			t.Errorf("%s → %s 不是 M2 上的合法邊", tc.from, tc.to)
		}
	}
}

// TestAllowance_GiveUpToleratesSettledAdjustment 守 G15 的第二個實例。
//
// 就算沒有 ADM 取消在途，同樣的 rollback 也會發生：G15 放棄過一次之後 operation 是
// FAILED_UNKNOWN，J20（週期重查）把它推回 PROVIDER_STATUS_RECHECK 再走一次 G15，
// 而 adjustment 早已是 FAILED_UNKNOWN ⇒ MarkFailedUnknown 的 allowed-from
// （只有 WAITING_CONFIRMATION）對不上 ⇒ rows=0 ⇒ 同一個熱迴圈。
//
// 不變式是「**operation 必須終結**」：adjustment 那一半推不動時要放行。
func TestAllowance_GiveUpToleratesSettledAdjustment(t *testing.T) {
	for _, adjStatus := range []string{
		"FAILED_UNKNOWN", "CANCELLED", "COMPLETED", "FAILED_PERMANENT",
	} {
		t.Run(adjStatus, func(t *testing.T) {
			h := newAllowanceHarness(t)
			ctx := context.Background()
			h.adjs.adj.Status = adjStatus
			unknownSince := alwCancelUnknownSince
			h.ops.op.UnknownSince = &unknownSince

			snapshot := h.ops.op
			snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
			h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value

			h.prov.allowanceQueryFn = func(string) (*amego.AllowanceDetail, error) {
				return &amego.AllowanceDetail{}, nil // 查無
			}

			err := h.uc.ConfirmAllowance(ctx, snapshot)
			if adjStatus == "COMPLETED" {
				// COMPLETED 在 ConfirmAllowance 開頭就走 finishOperation，不進 G15。
				if err != nil {
					t.Fatalf("ConfirmAllowance: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConfirmAllowance: %v —— adjustment 已在 %s，"+
					"這一半沒有事情要做；回 error 會讓整個 WithTx rollback ⇒ "+
					"operation 停在 PROVIDER_STATUS_RECHECK ⇒ 每 ~2 分鐘打一次 AMEGO",
					err, adjStatus)
			}
			if got := h.ops.op.Status; got != "FAILED_UNKNOWN" {
				t.Fatalf("operation = %q，want FAILED_UNKNOWN —— "+
					"operation 沒有終結就是熱迴圈本身", got)
			}
			if got := h.adjs.adj.Status; got != adjStatus {
				t.Errorf("adjustment = %q，want 留在 %q（M3 上沒有那條邊）", got, adjStatus)
			}
		})
	}
}

// TestAllowance_GiveUpStillFailsWhenAdjustmentDrifted 是上一支的**反面**：
// adjustment 停在非終態、也不是目標狀態（PENDING／SENDING／BLOCKED_*）時，
// 那是真的狀態漂移，fail-closed 必須保留。
//
// 少了這一支，「把 rows=0 一律放行」與「分辨後放行」在測試上長得一模一樣。
func TestAllowance_GiveUpStillFailsWhenAdjustmentDrifted(t *testing.T) {
	for _, adjStatus := range []string{"PENDING", "SENDING", "BLOCKED_PENDING_CORRECTION"} {
		t.Run(adjStatus, func(t *testing.T) {
			h := newAllowanceHarness(t)
			ctx := context.Background()
			h.adjs.adj.Status = adjStatus
			unknownSince := alwCancelUnknownSince
			h.ops.op.UnknownSince = &unknownSince

			snapshot := h.ops.op
			snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
			h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value

			h.prov.allowanceQueryFn = func(string) (*amego.AllowanceDetail, error) {
				return &amego.AllowanceDetail{}, nil
			}

			if err := h.uc.ConfirmAllowance(ctx, snapshot); err == nil {
				t.Fatalf("adjustment 停在 %s（非終態）卻放行 —— "+
					"那是真的狀態漂移，放行會讓 operation 與 adjustment 各說各話", adjStatus)
			}
		})
	}
}
