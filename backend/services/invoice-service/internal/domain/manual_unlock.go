package domain

import "time"

// A4 人工出口雙人解鎖的狀態值域（§5.1.16 `chk_invmu_status`）。
//
// 🔴 **這四個值刻意不是 CodeMeta enum**。58 號 §C.2 的 30 個新增碼表逐名清單裡沒有
// 它們，而登記一張新碼表會連動四個前端 repo 的 verbatim 拷貝（`check-copies.sh`）。
// 現況與 `invoice_status_history.actor_type`（§C.4 明列的邊界案例）同一格：值域由 DB 的
// CHECK 守、常數由本檔承載。**改判條件寫死**：ADM 一旦需要中文標籤（`ManualUnlockInfo`
// 目前沒有 `statusLabel` 欄），就必須改判納管並補登記 —— 那正是 §C.4 對
// `InvoiceDocumentTimelineEvent` 用的同一條判準。
const (
	// ManualUnlockAwaitingSecondApproval 是第一位管理員發起後、第二位尚未覆核的狀態。
	ManualUnlockAwaitingSecondApproval = "AWAITING_SECOND_APPROVAL"
	// ManualUnlockApproved 是第二位管理員覆核放行（job 隨之進 MANUAL_PROCESS_ALLOWED）。
	ManualUnlockApproved = "APPROVED"
	// ManualUnlockRejected 是第二位管理員駁回（必附理由）。
	ManualUnlockRejected = "REJECTED"
	// ManualUnlockExpired 是逾 24 小時未覆核而失效。
	//
	// ⚠️ 它有兩種產生方式，**都不是排程器**：① 端點 54 再次發起時，同一交易內把過期的
	// 那一列就地收斂（釋放 uq_invmu_open_per_operation 的名額）② 讀取面以
	// ProjectManualUnlockStatus 投影。逾期是時間函數不是事件——掃描它的 worker 要嘛是
	// 永久旗標、要嘛是沒有搶佔的裸迴圈（《多副本排程搶佔規範》）。
	ManualUnlockExpired = "EXPIRED"
)

// ManualUnlockExpiry 是解鎖請求的存活窗口（§6.1.14 定版：發起後 24 小時）。
const ManualUnlockExpiry = 24 * time.Hour

// ProjectManualUnlockStatus 把 DB 上的狀態投影成對外的狀態。
//
// 唯一的轉換是「仍掛著但已過期 ⇒ EXPIRED」。它必須存在的理由不是美觀：DB 那一列在
// 下一次端點 54 進來之前不會有人去改，而 58A §A06 靠 `status === "EXPIRED"` 決定要不要
// 顯示「解鎖請求已逾期／可重新發起」。少了這個投影，ADM 會看到一張永遠等不到人的
// 「等待覆核」卡，而按下去只會拿到 9343。
//
// 🔴 它是**唯讀投影**，不得反向用來決定寫入：寫入路徑的守衛一律是 SQL 的
// `expires_at > now`（DecideManualUnlock），不是這支函式的回傳值。
func ProjectManualUnlockStatus(status string, expiresAt, now time.Time) string {
	if status == ManualUnlockAwaitingSecondApproval && !now.Before(expiresAt) {
		return ManualUnlockExpired
	}
	return status
}
