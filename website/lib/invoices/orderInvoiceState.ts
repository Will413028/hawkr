/**
 * 端點 8 的 `state`（買家可見投影，9 值）→ B04 的橫幅樣態與空狀態說明句。
 *
 * 為什麼不是 CodeMeta：這 9 值刻意**不納管 CodeMeta**（58 §C.4，invoice-service
 * `orderInvoiceState*` 常數群），與 `InvoiceDocumentStatus` 的 11 值是兩套不可互
 * 相代入的字串。橫幅**標題**因此寫在這裡（58A §B04 的八態表逐字）；橫幅**內文**
 * 一律逐字用後端的 `stateNotice`，前端不自組。
 *
 * 🔴 兩條硬規則：
 *   1. **必須有 default 分支**（§0.6 C11）。後端先上線一個前端還不知道的值時，
 *      落 default 顯示「處理中」是安全的——它既不宣稱已開立、也不宣稱失敗，而
 *      內文仍然逐字來自後端。沒有 default 就是白畫面，而測試用的是舊值域所以全綠。
 *   2. `ISSUED`／`VOIDED`／`ALLOWANCE_APPLIED` **不出橫幅**：狀態已由每張文件卡
 *      自己的 Chip 表達，再加一條橫幅等於同一件事講兩次。
 *
 * ⚠️ `SELLER_WILL_ISSUE`（賣家還沒開）與 `SELLER_SELF_ISSUED`（賣家已開並回報號碼）
 * 是兩個狀態，不可合併，且後者的文案**不得寫成「平台已開立」**——開票責任歸屬在
 * 爭議時是法律問題。
 */

/** 買家可見投影的 9 值。與 `InvoiceDocumentStatus` 分離，不可互相代入。 */
export const ORDER_INVOICE_STATE = {
	AWAITING_PAYMENT: "AWAITING_PAYMENT",
	/**
	 * 訂單已取消且從未開過票（backend #2904）。
	 *
	 * 🔴 它**必須有自己的分支**，不能靠 `default` 兜：`default` 的標題是「處理中」、
	 * 空狀態句是「系統正在處理，完成後會寄送 Email」——兩句對已取消的訂單都是假的，
	 * 而後者還多承諾了一封永遠不會寄出的信。
	 *
	 * ⚠️ 值刻意不是 `NOT_APPLICABLE_CANCELLED`：`NOT_APPLICABLE` 是
	 * `InvoiceDocumentStatus` 的值，讓其中一個成為另一個的前綴等於在每一次
	 * `startsWith` 上放一顆地雷（58 §6.1.3 陷阱表）。
	 */
	ORDER_CANCELLED: "ORDER_CANCELLED",
	PROCESSING: "PROCESSING",
	SELLER_WILL_ISSUE: "SELLER_WILL_ISSUE",
	SELLER_SELF_ISSUED: "SELLER_SELF_ISSUED",
	ISSUED: "ISSUED",
	VOIDED: "VOIDED",
	ALLOWANCE_APPLIED: "ALLOWANCE_APPLIED",
	FAILED: "FAILED",
} as const;

export interface OrderInvoiceStateBanner {
	title: string;
	/** 對應既有的 `info`／`warning`／`danger` utility 三組，不新增顏色語彙。 */
	tone: "info" | "warning" | "danger";
	/** 失敗是 `alert`（要打斷讀屏），其餘是 `status`。 */
	role: "status" | "alert";
	/** 只有 `PROCESSING` 給「重新整理」文字鍵（§B-1.0.7 第 2 條）。 */
	showRefresh: boolean;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

const PROCESSING_BANNER: OrderInvoiceStateBanner = {
	title: "處理中",
	tone: "info",
	role: "status",
	showRefresh: true,
	unknown: false,
};

/**
 * 折出橫幅樣態；回 `null` 代表這個狀態不出橫幅。
 *
 * 參數刻意是 `string` 而不是收窄的 union：收窄會讓 default 分支在型別上不可達，
 * 而那個分支正是本函式存在的理由之一。
 */
export function foldOrderInvoiceState(
	state: string,
): OrderInvoiceStateBanner | null {
	switch (state) {
		case ORDER_INVOICE_STATE.AWAITING_PAYMENT:
			return {
				title: "待付款",
				tone: "info",
				role: "status",
				showRefresh: false,
				unknown: false,
			};
		case ORDER_INVOICE_STATE.ORDER_CANCELLED:
			return {
				// tone 是 `info` 不是 `danger`：訂單取消對發票模組不是異常，是一個
				// 確定的終局。紅色會讓買家以為發票出了問題而跑去申訴。
				title: "訂單已取消",
				tone: "info",
				role: "status",
				// 不給「重新整理」：這個狀態不會再變，按了也一樣。
				showRefresh: false,
				unknown: false,
			};
		case ORDER_INVOICE_STATE.PROCESSING:
			return PROCESSING_BANNER;
		case ORDER_INVOICE_STATE.SELLER_WILL_ISSUE:
			return {
				title: "賣家自行開立",
				tone: "warning",
				role: "status",
				showRefresh: false,
				unknown: false,
			};
		case ORDER_INVOICE_STATE.SELLER_SELF_ISSUED:
			return {
				title: "賣家已自行開立發票",
				tone: "warning",
				role: "status",
				showRefresh: false,
				unknown: false,
			};
		case ORDER_INVOICE_STATE.ISSUED:
		case ORDER_INVOICE_STATE.VOIDED:
		case ORDER_INVOICE_STATE.ALLOWANCE_APPLIED:
			return null;
		case ORDER_INVOICE_STATE.FAILED:
			return {
				title: "發票開立異常",
				tone: "danger",
				role: "alert",
				showRefresh: false,
				unknown: false,
			};
		default:
			return { ...PROCESSING_BANNER, unknown: true };
	}
}

/**
 * `documents` 為空時的一行說明句。
 *
 * 不用 `ScEmpty` 插圖：這是卡片內的一段，不是整頁空狀態。未知狀態沿用「系統正在
 * 處理」那句——與 default 橫幅同一個安全方向。
 */
export function orderInvoiceEmptyHint(state: string): string {
	switch (state) {
		case ORDER_INVOICE_STATE.AWAITING_PAYMENT:
			return "付款完成後開始處理。";
		case ORDER_INVOICE_STATE.ORDER_CANCELLED:
			// 🔴 這一句是 #2904 的另一半。少了它會落到 default 的「系統正在處理，
			// 完成後會寄送 Email」——對已取消的訂單那是一封永遠不會寄出的信。
			return "這筆訂單已取消，不會開立發票。";
		case ORDER_INVOICE_STATE.SELLER_WILL_ISSUE:
		case ORDER_INVOICE_STATE.SELLER_SELF_ISSUED:
			return "這筆訂單的發票由賣家自行開立；尚未收到時可從此處提出申訴。";
		case ORDER_INVOICE_STATE.FAILED:
			return "目前還沒有可查看的發票文件。";
		default:
			return "系統正在處理，完成後會寄送 Email。";
	}
}

/** `fileUnavailableReason` 三值 → 說明句。都不寫天數與 provider 原文。 */
export function invoiceFileUnavailableHint(reason: string): string {
	switch (reason) {
		case "CARRIER_NOT_WON":
			return "使用載具的發票在中獎後才會提供 PDF。";
		case "EXPIRED":
			return "這份文件已超過可查詢期限，若需要副本請提出申訴。";
		case "NOT_APPLICABLE":
			return "這類文件沒有可下載的 PDF。";
		default:
			return "目前沒有可下載的 PDF。";
	}
}
