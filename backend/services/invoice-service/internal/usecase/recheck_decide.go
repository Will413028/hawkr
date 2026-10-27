package usecase

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// INV.rc 的決策核心（§7.3 J15／J17／J20／J22–J27／J31、§4.3.3 ⑥⑦、§7.5.4）
//
// 抽成純函式的理由：這支 worker 的全部正確性都在「查到什麼 ⇒ 轉去哪個狀態」這張
// 表上，而那張表不需要 context、DB 或 AMEGO。把它與 IO 分開，五條分支才驗得到。
//
// 🔴 §4.3.3 line 1005 明列三個前置「少任何一件這支 worker 都不可實作」：
//   ① 退避階梯持久化在 next_attempt_at，階梯位置由**事實時間戳** unknown_since 推導
//      ——**不另設計數器**（計數器在「claim 成功但副本崩潰」之後會失真，時間戳不會）
//   ② claim 之後的落點由 cancel_requested_at／cancel_confirmed_at 判別，
//      **不新增「來源狀態」欄位**
//   ③ **一次 claim 只做一次外部呼叫**（一次 invoice_query 或 invoice_list 的一頁），
//      分頁游標存 operation.recheck_list_page 跨輪續用。第 ③ 條是 60s 租約唯一的
//      成立理由——沒有它，一次 claim 內翻 N 頁最壞 N×20s，N≥4 就超過租約
//
// 本檔負責 ① 與 ②；③ 由呼叫端保證（每輪只打一次，游標寫回 RecheckListPage）。
// ---------------------------------------------------------------------------

// recheckLadder 是退避階梯（§7.5.3 逐字 30s→1m→2m→5m→10m→30m）。
//
// 🔴 階梯位置由 `now − unknown_since` 推導，**不是**由 attempt_count。
// 用計數器的話，「claim 成功但副本在寫回之前崩潰」會讓計數停住而時間繼續走，
// 於是那一列會用最短的間隔重查到天荒地老。
var recheckLadder = []struct {
	after   time.Duration // unknown_since 起算已經過多久
	backoff time.Duration
}{
	{0, 30 * time.Second},
	{2 * time.Minute, time.Minute},
	{5 * time.Minute, 2 * time.Minute},
	{15 * time.Minute, 5 * time.Minute},
	{45 * time.Minute, 10 * time.Minute},
	{2 * time.Hour, 30 * time.Minute},
}

// recheckGiveUpAfter 是轉 FAILED_UNKNOWN 的門檻（J17／J31 逐字：unknown_since > 24h）。
//
// ⚠️ **這裡關掉了一個設計書標示為未定的點。** §9.3.3 對 `INVOICE_RECHECK_MAX_ATTEMPTS=12`
// 的註記逐字寫著：它與「unknown_since > 24h 才轉 FAILED_UNKNOWN」是**兩個不同的終止
// 條件**，照本階梯第 12 次約在 3.8h 就到，24h 那條會變成永不觸發的死規則，**兩者必須
// 擇一定版**。
//
// 選 24h 的理由不是偏好，是**與前置 ① 一致**：① 明令階梯位置由時間戳推導、不另設
// 計數器；終止條件若改用次數，就得維護一個 ① 說不要的計數器，而那個計數器正好在
// 「claim 成功但副本崩潰」時失真 ⇒ 提前放棄一筆其實還在飛的開票。
// **這個推導可被推翻**——要改成次數制的話，① 也要一起改，不能只改這一個常數。
const recheckGiveUpAfter = 24 * time.Hour

// recheckBackoff 回這一輪要排多久之後再查（§7.5.4 第 1 條）。
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

// decideRecheck 把「查到什麼」映射成「轉去哪個狀態」。
//
// 分支逐字對應 §7.3 的 J 表：
//
//	J23  查到已開立                          → SUCCEEDED（取消請求被回絕）
//	J24  code=71 ＋ 有 ADM 取消在途           → CANCELLED_BY_ADMIN（必帶 PROVIDER_QUERY_71）
//	J25  code=71 ＋ 無取消 ＋ 180 天內         → RETRY_SCHEDULED（可安全補送）
//	J26  仍不確定 ＋ 有 ADM 取消在途           → CANCEL_BLOCKED_UNKNOWN
//	J17  仍不確定 ＋ 無取消 ＋ 超過 24h        → FAILED_UNKNOWN
//	J31  仍不確定 ＋ 無取消 ＋ 未超過 24h      → WAITING_CONFIRMATION
//
// 🔴 「有沒有 ADM 取消在途」一律看 `cancel_requested_at`／`cancel_confirmed_at`
// （前置 ②），**不新增來源狀態欄位**。
//
// 🔴 J25 少了 180 天判定的後果：AMEGO 的 71 在超過查詢窗口時對「開過的發票」也回
// 「查無」⇒ 我方判定「確定沒開」而重送 ⇒ **同一筆訂單開出第二張發票**。
func decideRecheck(op domain.Operation, probe RecheckProbe, now time.Time) RecheckDecision {
	cancelPending := op.CancelRequestedAt != nil && op.CancelConfirmedAt == nil

	if probe.Found {
		// J23：查到已開立。取消請求被回絕——已經開出去的發票不能用「取消 job」抹掉。
		return RecheckDecision{Kind: RecheckResolved, NextStatus: codemeta.InvoiceOperationStatusSucceeded.Value, NextAttemptAt: now}
	}

	if probe.NotIssued {
		if cancelPending {
			// J24：確定沒開 ＋ ADM 要取消 ⇒ 取消成立，且證據就是這次查詢。
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
			// J25：確定沒開 ＋ 沒人要取消 ＋ 71 可信 ⇒ 安全補送。
			return RecheckDecision{Kind: RecheckResolved, NextStatus: codemeta.InvoiceOperationStatusRetryScheduled.Value, NextAttemptAt: now}
		}
		// 超過 180 天：71 不可信，當成「仍不確定」往下走，**不得**補送。
	}

	// 仍不確定。
	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	next := now.Add(recheckBackoff(elapsed))

	if cancelPending {
		// J26：有 ADM 取消在途 ⇒ 卡在 CANCEL_BLOCKED_UNKNOWN，等 ADM 雙人確認（J28）。
		// unknown_since 保留、排入退避重查（J27 會再把它推回 PROVIDER_STATUS_RECHECK）。
		return RecheckDecision{Kind: RecheckUnresolvedTerminal, NextStatus: codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value, NextAttemptAt: next}
	}
	if elapsed > recheckGiveUpAfter {
		// J17：超過 24h 仍不確定 ⇒ 交給人（outbox review_required 由呼叫端發）。
		return RecheckDecision{Kind: RecheckUnresolvedTerminal, NextStatus: codemeta.InvoiceOperationStatusFailedUnknown.Value, NextAttemptAt: next}
	}
	// J31：留在重查迴圈。unknown_since **不重寫**——它是階梯的基準。
	return RecheckDecision{Kind: RecheckStillWaiting, NextStatus: codemeta.InvoiceOperationStatusWaitingConfirmation.Value, NextAttemptAt: next}
}
