package usecase

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

var recheckLadder = []struct {
	after   time.Duration
	backoff time.Duration
}{
	{0, 30 * time.Second},
	{2 * time.Minute, time.Minute},
	{5 * time.Minute, 2 * time.Minute},
	{15 * time.Minute, 5 * time.Minute},
	{45 * time.Minute, 10 * time.Minute},
	{2 * time.Hour, 30 * time.Minute},
}

const recheckGiveUpAfter = 24 * time.Hour

func recheckBackoff(elapsed time.Duration) time.Duration {
	out := recheckLadder[0].backoff
	for _, step := range recheckLadder {
		if elapsed >= step.after {
			out = step.backoff
		}
	}
	return out
}

// RecheckProbe 是一次外部查詢的結果，已經由呼叫端翻譯成本層看得懂的三種形態。
//
// 刻意不讓決策函式碰 amego 的 DTO：那會把「AMEGO 回什麼」與「我方怎麼判」綁在一起，
// 而前者會隨對方改版變動、後者是我方的業務契約。
type RecheckProbe struct {
	// Found ＝ invoice_query code=0 或 invoice_list 比對到（J23）。
	Found bool
	// NotIssued ＝ code=71「查無資料」（J24／J25 的輸入）。
	NotIssued bool
	// InvoiceNumber 在 Found 時回填。
	InvoiceNumber string
	// ProviderCode／ProviderMessage 原樣留存，供稽核與 ADM 診斷。
	ProviderCode    *string
	ProviderMessage *string
	// WithinProviderRetention ＝ 發票日期在 180 天內。
	//
	// 🔴 J25 的條件逐字含這一項：超過 180 天時 AMEGO 的 71 **不可信**（invoice_query
	// 只查得到 180 天內），把它當成「確定沒開」會導致重複開票。
	WithinProviderRetention bool
}

// RecheckDecision 是決策結果。呼叫端依 Kind 選 repository 方法。
type RecheckDecision struct {
	Kind          RecheckOutcomeKind
	NextStatus    string
	NextAttemptAt time.Time
	// CancelEvidenceKind／CancelConfirmedAt 只在 J24 有值，且**同進同出**。
	//
	// 🔴 兩者都必須真的寫進 invoice_operations，缺任一個的後果不同但都無聲：
	//   - 少了 evidence_kind ⇒ §6.4「人工出口四樣紀律」少了證據那一樣。J24 沒有第二個人
	//     （cancel_confirmed_by 恆為 NULL），evidence_kind 是 chk_invop_admin_cancel_needs_confirm
	//     放行它的**唯一**依據；而那道 CHECK 在 2026-09-05 之前是三值邏輯的 no-op
	//     （兩欄皆 NULL ⇒ `FALSE OR FALSE OR NULL` = NULL ⇒ PostgreSQL 放行）⇒ 沒有任何訊號。
	//   - 少了 confirmed_at ⇒ 「ADM 取消在途」判別式
	//     （cancel_requested_at IS NOT NULL AND cancel_confirmed_at IS NULL）**永遠成立**
	//     ⇒ 這筆在資料上一直看起來像還沒處理完。
	//
	// dev 實證（op 7500463275386077184，2026-09-05）：兩欄皆 NULL —— 值在這裡算出來，
	// 而 domain.MarkRecheckResult 沒有對應欄位，applyDecision 把它整個丟掉。
	CancelEvidenceKind *string
	CancelConfirmedAt  *time.Time
}

// RecheckOutcomeKind 決定呼叫端要用哪一支 repository 方法寫回。
type RecheckOutcomeKind int

const (
	// RecheckResolved ＝ 對帳有結論（J23／J24／J25）⇒ MarkRecheckResolved。
	RecheckResolved RecheckOutcomeKind = iota
	// RecheckUnresolvedTerminal ＝ 仍不確定且要換狀態（J26／J17）⇒ MarkRecheckUnresolved。
	RecheckUnresolvedTerminal
	// RecheckStillWaiting ＝ 仍不確定且留在重查迴圈（J31）⇒ UpdateRecheckAttempt。
	RecheckStillWaiting
)

func decideRecheck(op domain.Operation, probe RecheckProbe, now time.Time) RecheckDecision {
	cancelPending := op.CancelRequestedAt != nil && op.CancelConfirmedAt == nil

	if probe.Found {
		return RecheckDecision{Kind: RecheckResolved, NextStatus: codemeta.InvoiceOperationStatusSucceeded.Value, NextAttemptAt: now}
	}

	if probe.NotIssued {
		if cancelPending {
			ev := codemeta.InvoiceCancelEvidenceKindProviderQuery71.Value
			confirmed := now
			return RecheckDecision{
				Kind:               RecheckResolved,
				NextStatus:         codemeta.InvoiceOperationStatusCancelledByAdmin.Value,
				NextAttemptAt:      now,
				CancelEvidenceKind: &ev,
				CancelConfirmedAt:  &confirmed,
			}
		}
		if probe.WithinProviderRetention {
			return RecheckDecision{Kind: RecheckResolved, NextStatus: codemeta.InvoiceOperationStatusRetryScheduled.Value, NextAttemptAt: now}
		}
	}

	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	next := now.Add(recheckBackoff(elapsed))

	if cancelPending {
		return RecheckDecision{Kind: RecheckUnresolvedTerminal, NextStatus: codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value, NextAttemptAt: next}
	}
	if elapsed > recheckGiveUpAfter {
		return RecheckDecision{Kind: RecheckUnresolvedTerminal, NextStatus: codemeta.InvoiceOperationStatusFailedUnknown.Value, NextAttemptAt: next}
	}
	return RecheckDecision{Kind: RecheckStillWaiting, NextStatus: codemeta.InvoiceOperationStatusWaitingConfirmation.Value, NextAttemptAt: next}
}
