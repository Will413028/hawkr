import { InvoiceOperationStatus } from "@/lib/codemeta";

/**
 * A05 發票工作與異常佇列的分頁定義（端點 50）。
 *
 * 放在 feature 模組而不是頁面裡，是為了讓契約測試驗得到「六個集合聯集 ＝ 20 值、
 * 兩兩交集 ＝ 空」。這件事打錯**不會報錯**：
 *   - 漏一個值 ⇒ 那個狀態的 job 在六頁裡都看不到（不是報錯，是消失）
 *   - 多一個值 ⇒ 同一筆出現在兩頁，ADM 會處理兩次
 *
 * ⚠️ **集合本身是後端的查詢定義，前端只送 `tab` 字串。** 列在這裡不是為了在前端
 * 過濾，是為了讓「同一筆案件會出現在哪一頁」這件事有一份可被測試守住的宣告——
 * 以及讓 CodeMeta 值域改動時計算鍵直接編譯錯（字面值只會靜靜落空）。
 */

export type JobTabValue =
	| "PENDING"
	| "RETRYING"
	| "NEEDS_ATTENTION"
	| "NEEDS_MANUAL"
	| "COMPLETED"
	| "CANCELLED";

export interface JobTab {
	value: JobTabValue;
	label: string;
	/** 該分頁涵蓋的 `InvoiceOperationStatus` 值（SSOT ＝ 58 §5 `chk_invop_status` 的 20 值）。 */
	statuses: readonly string[];
}

/**
 * 六個分頁。**順序即處理優先序**（待處理 → 重試中 → 需關注 → 需人工 → 已完成 → 已取消）。
 *
 * 三個歸類判斷特別容易寫反，都寫在這裡而不是散在頁面註解：
 *
 * 1. **`WAITING_CONFIRMATION` 歸 `NEEDS_MANUAL`，不歸 `RETRYING`**——它不是在重試，
 *    是「送出去了但不知道結果」。歸錯會讓 ADM 以為系統正在自己處理，實際上它在等
 *    人去向 AMEGO 查證；而在結果未明時貿然重試就是開出第二張發票。
 *
 * 2. **`SELLER_ACTION_REQUIRED` 歸 `NEEDS_ATTENTION`**——不歸 `NEEDS_MANUAL`（那一頁的
 *    語意是「**平台**要動手」，而本值要動手的是**賣家**：去財政部申請丁種字軌，配到
 *    之後系統會自己接回自動線）；也不歸 `RETRYING`（它已停止重試）。
 *    ⇒ `NEEDS_ATTENTION` 同時裝「ADM 要動手」與「賣家要動手」兩類，唯一的區分依據是
 *    badge 中文，所以它的 badge variant **必須與 `FAILED_PERMANENT` 不同**
 *    （見 `badge-variants.ts`：warning vs danger；有測試守住）。
 *
 * 3. **`CANCEL_REQUESTED` 歸 `NEEDS_MANUAL` 不歸 `CANCELLED`**——取消還沒完成，
 *    而且它可能卡在「provider 結果未明」那條路上。
 *
 * ⚠️ `NEEDS_MANUAL` 的後端述詞還多一個 OR：該 job 所屬 document 的
 * `status = 'AWAITING_MANUAL'`。那是 **document** 的欄位，**不是 job 狀態** ⇒
 * 前端不得為它造一個假的 `AWAITING_MANUAL` job 狀態（回應的 `status` 仍吐 job 自己的
 * 狀態，`manualReasonCode` 由 document 帶上來）。
 *
 * ⚠️ `REVIEW_REQUIRED` 不是狀態值、不是分頁：它是 58 §5A 對「對帳耗盡／轉人工」這個
 * **結果**的簡稱。為它造分頁或 badge 會變成一個永久旗標，且與 58 §7 J17 構成兩套會
 * 分岔的守衛。
 */
export const JOB_TABS: readonly JobTab[] = [
	{
		value: "PENDING",
		label: "待處理",
		statuses: [
			InvoiceOperationStatus.CREATED,
			InvoiceOperationStatus.AWAITING_TRIGGER,
			InvoiceOperationStatus.READY,
			InvoiceOperationStatus.HELD,
		],
	},
	{
		value: "RETRYING",
		label: "重試中",
		statuses: [
			InvoiceOperationStatus.RETRY_SCHEDULED,
			InvoiceOperationStatus.SENDING,
		],
	},
	{
		value: "NEEDS_ATTENTION",
		label: "需關注",
		statuses: [
			InvoiceOperationStatus.FAILED_PERMANENT,
			InvoiceOperationStatus.FAILED_RETRY_EXHAUSTED,
			InvoiceOperationStatus.SELLER_ACTION_REQUIRED,
		],
	},
	{
		value: "NEEDS_MANUAL",
		label: "需人工",
		statuses: [
			InvoiceOperationStatus.WAITING_CONFIRMATION,
			InvoiceOperationStatus.PROVIDER_STATUS_RECHECK,
			InvoiceOperationStatus.FAILED_UNKNOWN,
			InvoiceOperationStatus.CANCEL_REQUESTED,
			InvoiceOperationStatus.CANCEL_BLOCKED_UNKNOWN,
			InvoiceOperationStatus.CANCELLED_BY_ADMIN,
			InvoiceOperationStatus.MANUAL_PROCESS_ALLOWED,
		],
	},
	{
		value: "COMPLETED",
		label: "已完成",
		statuses: [
			InvoiceOperationStatus.SUCCEEDED,
			InvoiceOperationStatus.RESOLVED_EXTERNALLY,
		],
	},
	{
		value: "CANCELLED",
		label: "已取消／不適用",
		statuses: [
			InvoiceOperationStatus.CANCELLED_BEFORE_ISSUE,
			InvoiceOperationStatus.NOT_APPLICABLE,
		],
	},
];

export const JOB_TAB_VALUES: readonly string[] = JOB_TABS.map((t) => t.value);
