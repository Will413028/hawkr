/**
 * A07 未收到發票申訴佇列的分頁與排序定義（端點 58）。
 *
 * 放在 feature 模組是為了讓契約測試驗得到值域——五個 tab 值打錯不會報錯，後端會
 * 當成不篩而回全部，畫面看起來完全正常。
 */

export type ComplaintTabValue =
	| "AWAITING_SELLER"
	| "SELLER_RESPONDED"
	| "ADM_REVIEW"
	| "OVERDUE"
	| "CLOSED";

export interface ComplaintTab {
	value: ComplaintTabValue;
	label: string;
	/** 這個分頁在畫面上要說明的一句話（`AWAITING_SELLER` 與 `OVERDUE` 會互相包含）。 */
	note?: string;
}

/**
 * 五個分頁是「**查詢視圖**」不是 `InvoiceComplaintStatus` 的值。
 *
 * 🔴 **五者不互斥，加總 ≠ 總筆數** ⇒ **不得顯示合計、不得用「總數 − 其他四頁」去算
 * 任何一頁的筆數**。這一點與 A05 的六分頁**規則相反**（那六個互斥且窮盡，還有機械
 * 驗算 gate），兩處的規則不可互相套用。
 *
 * 後端述詞（列在這裡是為了讓人知道同一筆案件會出現在哪幾頁）：
 *   - `AWAITING_SELLER` ＝ `status IN ('OPEN','OVERDUE')`　　← **與 OVERDUE 交集**
 *   - `SELLER_RESPONDED` ＝ `status = 'SELLER_RESPONDED'`
 *   - `ADM_REVIEW`　　　 ＝ `status = 'ESCALATED_TO_ADM'`
 *   - `OVERDUE`　　　　　＝ `status = 'OVERDUE' OR (status IN ('OPEN','SELLER_RESPONDED',
 *     'ESCALATED_TO_ADM') AND due_at <= now())`　← **橫切視圖，與前三者交集**
 *   - `CLOSED`　　　　　 ＝ `status IN ('RESOLVED','VIOLATION_CONFIRMED','WITHDRAWN')`
 *
 * 兩個容易被「修正」掉的設計：
 *   1. **`AWAITING_SELLER` 含 `OVERDUE` 不是筆誤**——逾期的案子仍然在等賣家。把它移走的話，
 *      賣家那邊還在等、ADM 這邊卻看不到。
 *   2. **`OVERDUE` 的述詞是 OR 不是 `status = 'OVERDUE'`**——`OVERDUE` 由掃描器寫入且只從
 *      `OPEN` 進入，所以有三類逾期案在單純述詞下會消失：① `due_at` 已過但掃描還沒跑到
 *      ② 賣家逾期後才回覆（狀態被帶到 `SELLER_RESPONDED`）③ 已升級到 ADM 的逾期件。
 *      而那三類正是 ADM 最該看到的。
 *
 * ⚠️ `REOPENED` 既不是分頁也不是狀態：重開後案件回到 `OPEN`（`reopenCount` ＋1），
 * 出現在 `AWAITING_SELLER`。要在列表標「這是重開案」就讀 `reopenCount > 0`。
 */
export const COMPLAINT_TABS: readonly ComplaintTab[] = [
	{
		value: "AWAITING_SELLER",
		label: "待賣家處理",
		note: "含已逾期的案件——逾期的案子仍然在等賣家回覆。",
	},
	{ value: "SELLER_RESPONDED", label: "賣家已回覆" },
	{ value: "ADM_REVIEW", label: "待平台判定" },
	{
		value: "OVERDUE",
		label: "已逾期",
		note: "橫切視圖：逾期案件同時也會出現在它原本的分頁，五個分頁的筆數不可相加。",
	},
	{ value: "CLOSED", label: "已結案" },
];

export const COMPLAINT_TAB_VALUES: readonly string[] = COMPLAINT_TABS.map(
	(t) => t.value,
);

/** 端點 58 的 `sort` 值域。預設「期限最近的排前面」——ADM 先看快到期的。 */
export const COMPLAINT_SORTS: ReadonlyArray<{ value: string; label: string }> =
	[
		{ value: "due_at_asc", label: "期限由近到遠" },
		{ value: "created_at_desc", label: "建立時間由新到舊" },
	];

export const COMPLAINT_SORT_VALUES: readonly string[] = COMPLAINT_SORTS.map(
	(s) => s.value,
);
