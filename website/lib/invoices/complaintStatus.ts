/**
 * `InvoiceComplaintStatus`（7 值）→ B07 案件橫幅的樣態、標題與內文。
 *
 * 為什麼標題與內文寫在這裡而不是取 CodeMeta 的 label：CodeMeta 的
 * `INVOICE_COMPLAINT_STATUS_LABEL` 是**跨三端**的狀態名（`VIOLATION_CONFIRMED`
 * ＝「判定賣家違規」、`OVERDUE` ＝「已逾期」），而買家面要的是「發生了什麼、
 * 你現在該做什麼」的兩句話，且**買家面不得出現「違規」二字**（違規是平台對
 * 賣家的治理結論，向買家揭露一家店的違規紀錄既無必要也有法律風險）——
 * `RESOLVED` 與 `VIOLATION_CONFIRMED` 因此用完全相同的兩句。
 *
 * 🔴 三條硬規則：
 *   1. **必須有 default 分支**（§0.6 C11）。後端先上線一個前端還不知道的狀態時，
 *      落 default 顯示「處理中」是安全的——它既不宣稱已解決、也不宣稱逾期，
 *      而動作列在 unknown 時一顆都不渲染（見 `complaintActions`）。
 *   2. **`OVERDUE` 必須有自己的分支**，不可與 `OPEN` 合併：「賣家逾期了、平台會
 *      接手」是買家最需要知道的一句。
 *   3. 逾期時仍是 `role="status"` **不是** `alert`——逾期是賣家的問題，對買家是
 *      資訊不是警報。
 */

import { InvoiceComplaintStatus } from "@/lib/codemeta/generated";

export type InvoiceComplaintTone = "info" | "warning" | "success" | "neutral";

export interface InvoiceComplaintBanner {
	title: string;
	body: string;
	tone: InvoiceComplaintTone;
	/** 只有 `OPEN`／`OVERDUE`／`SELLER_RESPONDED` 顯示倒數；已結案顯示倒數是噪音。 */
	showCountdown: boolean;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

/** 已解決與已判定違規對買家是**同一件事**：文件補上了。 */
const RESOLVED: InvoiceComplaintBanner = {
	title: "申訴已完成",
	body: "文件已補開或補寄，請確認你的電子信箱與上方的文件清單。",
	tone: "success",
	showCountdown: false,
	unknown: false,
};

const PROCESSING: InvoiceComplaintBanner = {
	title: "處理中",
	body: "這件申訴正在處理，有進度會通知你。",
	tone: "info",
	showCountdown: false,
	unknown: false,
};

/**
 * 折出案件橫幅。參數刻意是 `string` 而不是收窄的 union：收窄會讓 default 分支
 * 在型別上不可達，而那個分支正是本函式存在的理由之一。
 */
export function foldInvoiceComplaintStatus(
	status: string,
): InvoiceComplaintBanner {
	switch (status) {
		case InvoiceComplaintStatus.OPEN:
			return {
				title: "已提出申訴",
				body: "我們已通知賣家，賣家需於 3 個工作天內處理。",
				tone: "warning",
				showCountdown: true,
				unknown: false,
			};
		case InvoiceComplaintStatus.OVERDUE:
			// 不寫「賣家違規」——逾期是否構成違規由平台裁定，在買家面預告治理
			// 結果等於承諾一件還沒發生的事。
			return {
				title: "賣家尚未處理",
				body: "已超過處理期限，平台會接手這件案子，有結果會通知你。",
				tone: "warning",
				showCountdown: true,
				unknown: false,
			};
		case InvoiceComplaintStatus.SELLER_RESPONDED:
			return {
				title: "賣家已回覆",
				body: "請查看下方回覆內容；若仍未收到文件，案件會由平台接手。",
				tone: "info",
				showCountdown: true,
				unknown: false,
			};
		case InvoiceComplaintStatus.ESCALATED_TO_ADM:
			return {
				title: "平台處理中",
				body: "案件已交由平台客服處理，有結果會通知你。",
				tone: "info",
				showCountdown: false,
				unknown: false,
			};
		case InvoiceComplaintStatus.RESOLVED:
		case InvoiceComplaintStatus.VIOLATION_CONFIRMED:
			return RESOLVED;
		case InvoiceComplaintStatus.WITHDRAWN:
			return {
				title: "申訴已撤回",
				body: "如仍未收到文件，可再次提出申訴。",
				tone: "neutral",
				showCountdown: false,
				unknown: false,
			};
		default:
			return { ...PROCESSING, unknown: true };
	}
}

/** 重開上限（`invoice_complaints.reopen_count` 的值域是 0–2）。 */
export const COMPLAINT_REOPEN_LIMIT = 2;

export interface InvoiceComplaintActions {
	/** 端點 63「這樣沒有解決」。 */
	canReject: boolean;
	/** 端點 64「撤回申訴」。 */
	canWithdraw: boolean;
	/**
	 * 端點 65「重新提出」是否**出現**。冷卻中與已達上限仍然渲染（disabled ＋
	 * 說明），案件已在畫面上、按鈕突然消失比看得到按不動更難理解。
	 *
	 * ⚠️ 但「這個案子根本沒有重開邊」與「暫時還不能按」是兩件事：前者一顆按鈕都
	 * 不該有（沒有任何說明句能解釋一顆永遠按不下去的鍵），判準是
	 * `reopenAllowedAfter === ""`。
	 */
	showReopen: boolean;
	/** 冷卻未到或已達上限時為 false。 */
	canReopen: boolean;
	/** 已達 2 次上限。 */
	reopenExhausted: boolean;
}

/**
 * 三顆動作鍵的可見與可按（§B07 三支動作端點共通規則第 4 條）。
 *
 * 🔴 **一律由 DTO 欄位決定，不由錯誤碼決定**：重開讀 `reopenCount` ＋
 * `reopenAllowedAfter` 兩欄自行判斷（後端不下發 `canReopen` 布林），9336 只是
 * 競態時的兜底。
 *
 * 🔴 **`showReopen` 也吃 `reopenAllowedAfter`，不是只看 `status`**（#2911）。
 * 規則第 4 條逐字就是「重開鍵讀 `reopenCount` ＋ `reopenAllowedAfter` 兩欄自行
 * 判斷」——本函式原本只把 `reopenAllowedAfter` 用在 `canReopen`、`showReopen` 單看
 * `status === RESOLVED`，那本來就沒把規則第 4 條做完，而它在後端把
 * `VIOLATION_CONFIRMED` 投影成 `RESOLVED` 之後會直接外顯成 bug：終態案件會冒出
 * 一顆重開鍵。
 *
 * 那一欄是後端對「這個案子還有沒有重開邊」的**權威回答**，不是啟發式：
 * invoice-service `db/queries/invoice_complaints.sql` 的 K13（買家重開）述詞含
 * `reopen_allowed_after IS NOT NULL`，三條進 `RESOLVED` 的邊（K06 買家異議期屆滿／
 * K08 平台判定已解決／K12 系統偵測已開立）一律寫 `now + 24h`，而 K09
 * （判定賣家違規）逐字「傳 NULL（終態，沒有重開邊）」。
 *
 * ⚠️ 所以這個條件對**真正的** `RESOLVED` 恆真 ⇒ 加上它不改變今天的任何行為，
 * 前端可以先上、後端投影後上，中間沒有窗口。
 *
 * ⚠️ `reopenCount` 是**數字字串**，`Number()` 之前先判空——空字串 `Number("")`
 * 是 0，會讓「後端沒給」看起來像「一次都沒重開過」。
 */
export function complaintActions(
	status: string,
	reopenCount: string,
	reopenAllowedAfter: string,
	nowMs: number,
): InvoiceComplaintActions {
	const resolved = status === InvoiceComplaintStatus.RESOLVED;
	// 有沒有重開邊 = 後端有沒有給重開時間（見上方 K13 述詞）。
	const hasReopenEdge = resolved && reopenAllowedAfter !== "";
	const parsedCount = reopenCount === "" ? 0 : Number(reopenCount);
	const count = Number.isFinite(parsedCount) ? parsedCount : 0;
	const exhausted = count >= COMPLAINT_REOPEN_LIMIT;
	const allowedAtMs =
		reopenAllowedAfter === "" ? Number.NaN : Date.parse(reopenAllowedAfter);
	const cooledDown = Number.isFinite(allowedAtMs) && nowMs >= allowedAtMs;
	return {
		canReject: status === InvoiceComplaintStatus.SELLER_RESPONDED,
		canWithdraw:
			status === InvoiceComplaintStatus.OPEN ||
			status === InvoiceComplaintStatus.OVERDUE ||
			status === InvoiceComplaintStatus.SELLER_RESPONDED,
		showReopen: hasReopenEdge,
		canReopen: hasReopenEdge && !exhausted && cooledDown,
		reopenExhausted: exhausted,
	};
}

/** 三支動作端點的輸入欄規則。 */
export interface ComplaintActionFieldRule {
	/** 有沒有 `<Textarea>`。 */
	hasField: boolean;
	/** 空白時是否 disable 送出鍵。 */
	required: boolean;
}

/**
 * 63／64／65 的輸入欄規則（§B07 的動作表）。
 *
 * 🔴 **三者各不相同，而且 63 與建案（端點 13）完全相反**——照抄任何一個到另一個
 * 都會錯，而且錯了不會編譯錯、不會測試紅：
 *
 *   - 建案 13 的 `description` **選填** ⇒ 照抄既有 `AppealDialog` 的
 *     `disabled={text.trim().length === 0}` 會讓買家送不出最常見的那個情況
 *     （「就是沒收到，沒別的好說」），而按鈕只是灰的、沒有任何錯誤訊息
 *   - 63 的 `reason` **必填** ⇒ 反過來忘了 disable，後端回 400
 *   - 64 **沒有輸入欄**（端點層與前端都不自行加嚴）
 *   - 65 的 `reason` 選填
 */
export function complaintActionFieldRule(
	kind: "reject" | "withdraw" | "reopen",
): ComplaintActionFieldRule {
	switch (kind) {
		case "reject":
			return { hasField: true, required: true };
		case "reopen":
			return { hasField: true, required: false };
		default:
			return { hasField: false, required: false };
	}
}

/**
 * 三支動作端點的失敗要顯示哪一種話。
 *
 * 🔴 9322 與 9336 的處置**相反**，而且兩者都是 409、看 HTTP status 分不出來：
 *
 *   - **9322** 的碼名是稅務側的（`SELLER_TAX_PROFILE_VERSION_CONFLICT`），後端
 *     message 講的是「稅務資格資料已被更新」。申訴只是借用這個碼做樂觀鎖 ⇒
 *     **不得顯示後端 message**，否則買家讀到一句與他正在做的事完全無關的話。
 *   - **9336** 後端對三種守衛（狀態不允許／重開次數用盡／冷卻未到）給三句不同的
 *     話 ⇒ **直接顯示後端 message**，前端不自行推導是哪一種。
 */
export type ComplaintActionErrorKind =
	| "notFound"
	| "versionConflict"
	| "backendMessage"
	| "idempotencyMismatch"
	| "generic";

export function complaintActionErrorKind(
	code: number,
): ComplaintActionErrorKind {
	switch (code) {
		case 9309:
			return "notFound";
		case 9322:
			return "versionConflict";
		case 9336:
			return "backendMessage";
		case 9350:
			return "idempotencyMismatch";
		default:
			return "generic";
	}
}
