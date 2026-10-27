/**
 * 付款完成頁（B03）的兩張折疊表：`invoiceMode` → 狀態句、`documents[]` → 載具摘要。
 *
 * 為什麼不共用 B04 的 `foldOrderInvoiceState`：B04 折的是 `state`（發票走到哪裡），
 * B03 折的是 `invoiceMode`（這張發票由誰開）。付款當下 `state` 幾乎必然是
 * `PROCESSING`／`AWAITING_PAYMENT` 兩個值之一，八態橫幅在這裡沒有資訊量；買家在
 * 這一頁真正需要知道的是「接下來發票會從哪裡來」。
 *
 * 🔴 兩條硬禁止（58A §B03）：
 *   1. **絕不得出現「發票已開立」**——付款完成當下發票必然還在處理中。
 *   2. **不得繞過 `state`／`invoiceMode` 去讀 `documents[].status`**：`ISSUE_FAILED`
 *      與 `AWAITING_MANUAL` 都由後端投影成 `PROCESSING`，前端自己去讀原始狀態等於
 *      把 provider 的異常暴露給買家。
 */

import type { InvoiceDocumentSummary } from "./types";

/**
 * `InvoiceFulfillmentMode` 的 4 值 → 付款成功當下的狀態句（58A §B03 文案表逐字）。
 *
 * 🔴 **必須有 default 分支**（§0.6 C11）。後端先上線一個前端還不知道的 mode 時，
 * 落 default 顯示「系統正在處理發票」是安全的方向——它既不宣稱已開立、也不把開票
 * 責任指給錯的人；沒有 default 就是這一列整格空白，而測試用的是舊值域所以全綠。
 *
 * 參數刻意是 `string` 而不是收窄的 union：收窄會讓 default 分支在型別上不可達，
 * 而那個分支正是本函式存在的理由。
 */
export function checkoutPaidInvoiceNotice(invoiceMode: string): string {
	switch (invoiceMode) {
		case "AMEGO_AUTO":
			return "付款完成，系統正在處理發票";
		case "PLATFORM_RECEIPT":
			return "付款完成，收據將顯示在訂單詳情";
		// LEGACY_MANUAL 與 SELLER_SELF_ISSUE 的責任歸屬相同（都在賣家手上），
		// 買家可見的措辭因此一致；兩者的差別只在平台內部有沒有串接，說給買家聽
		// 沒有意義。
		case "SELLER_SELF_ISSUE":
		case "LEGACY_MANUAL":
			return "付款完成，賣家將依訂單資訊自行開立發票";
		default:
			return "付款完成，系統正在處理發票";
	}
}

/**
 * ATM 待轉帳頁在 `stateNotice` 為空時的墊底句。
 *
 * 正常路徑一律**逐字顯示後端的 `stateNotice`**（58A §B03）——那句話由後端依實際
 * 狀態產生，前端自組會和訂單詳情頁講的不一樣。這句只在後端沒給時出現，且刻意不寫
 * 「付款完成」：這一頁的錢還沒到。
 */
export const CHECKOUT_AWAITING_INVOICE_NOTICE = "完成付款後開始處理發票。";

/**
 * 「發票方式」列要顯示的載具摘要；沒有可顯示的值時回空字串（呼叫端整列不渲染）。
 *
 * ⚠️ 端點 8 的回應**沒有訂單層級的載具欄位**，唯一的來源是 `documents[].carrierSummary`
 * ——而付款完成當下 `documents` 幾乎必然是空的 ⇒ 這一列在 B03 實務上多半不會出現。
 * 這是資料面的事實，不是可以用前端補上的東西（詳見交付回報）。
 *
 * 從尾端往前取第一個非空值：作廢重開後 `documents` 會有多筆並存，順序由後端保證
 * （依 `issuedAt` 遞增），取最新的那筆才是買家現在真正在用的載具。
 */
export function checkoutInvoiceCarrier(
	documents: InvoiceDocumentSummary[],
): string {
	for (let i = documents.length - 1; i >= 0; i -= 1) {
		const summary = documents[i]?.carrierSummary ?? "";
		if (summary !== "") return summary;
	}
	return "";
}
