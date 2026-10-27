package fsm

// 本檔是 §7 不變式 **I7** 宣稱的「transition helper」。
//
// 設計書 §7 不變式表逐字：
//
//	I7 | 每一次 status 轉移都有一列 invoice_status_history
//	   | transition helper 是唯一寫入點 ＋ 契約測試斷言「七個 aggregate 的每一條
//	     轉換表列都至少被一支測試走過」
//
// 🔴 **這個檔案在 2026-09-04 之前不存在**（#3109）。I7 從上線起就沒有任何東西在守：
// `internal/usecase/recheck.go` 的 `applyDecision` 三個分支承載 J23／J24／J25／J26／
// J17／J31 六條轉移，一列 history 都不寫。dev 實證是一條**自相矛盾**的軌跡——
// `→ CANCEL_REQUESTED` 的下一列直接是 `CANCELLED_BY_ADMIN →`，中間兩步整段消失，
// 照它讀這筆 job 是憑空變成已取消的。
//
// 本檔只做一件事：**在寫 history 之前，先問 fsm 這條邊存不存在**。
// 它不碰 DB、不知道 repository 長什麼樣——寫入由呼叫端用既有的 append helper 做，
// 這裡回答的是「這條邊合法嗎、該不該留痕」。

import "fmt"

// TransitionKind 是 CheckOperation／CheckDocument 的判定結果。
type TransitionKind int

const (
	// TransitionRecord ＝ 合法且狀態真的改變了 ⇒ **必須**寫一列 history。
	TransitionRecord TransitionKind = iota
	// TransitionSelfLoop ＝ from == to（例如 PROVIDER_STATUS_RECHECK 留在原地等下一輪）。
	//
	// 🔴 自轉移**不寫** history，而這不是偷懶：INV.rc 的重查迴圈最短 30 秒一輪，
	// 逐輪寫會讓一筆卡住的 operation 在 24 小時內產生上千列，把真正的狀態變化淹掉。
	// 「這一輪也沒有結果」是 next_attempt_at 與 unknown_since 在表達的事，不是歷程。
	TransitionSelfLoop
)

// CheckOperation 判定一條 M2 轉移該不該留痕，不合法時回 error。
//
// 🔴 **不合法一律回 error，不是「寫下去再說」**：能走到這裡的 from/to 都來自
// 呼叫端剛剛成功的那一句 CAS，所以「fsm 不認得這條邊」只有兩種意思——
// 轉換表漏登記，或者有人繞過轉換表加了一條邊。兩者都必須當場炸掉：
// 靜默寫進去的話，`invoice_status_history` 會慢慢累積出一份**與設計書不一致的
// 事實**，而它正是 A10 稽核與 AuditTrailDrawer 呈現給人看的東西。
//
// ⚠️ 呼叫端必須在 CAS **成功之後**才呼叫本函式（rows>0）。CAS 失敗代表狀態不是
// 你以為的那個，此時的 from 是快照值、不是真實值。
func CheckOperation(from, to string) (TransitionKind, error) {
	if from == to {
		return TransitionSelfLoop, nil
	}
	if !CanTransitionOperation(from, to) {
		return TransitionRecord, fmt.Errorf(
			"fsm: operation transition %s -> %s is not in the M2 table (I7)", from, to)
	}
	return TransitionRecord, nil
}

// CheckDocument 是 M1 的對應版本，規則與 CheckOperation 逐字相同。
func CheckDocument(from, to string) (TransitionKind, error) {
	if from == to {
		return TransitionSelfLoop, nil
	}
	if !CanTransitionDocument(from, to) {
		return TransitionRecord, fmt.Errorf(
			"fsm: document transition %s -> %s is not in the M1 table (I7)", from, to)
	}
	return TransitionRecord, nil
}
