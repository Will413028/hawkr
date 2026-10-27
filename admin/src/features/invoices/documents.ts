import {
	codeLabel,
	INVOICE_DOCUMENT_STATUS_LABEL,
	INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL,
	SHIPPING_STATUS_LABEL,
} from "@/lib/codemeta";

/**
 * A12／A13 的顯示規則（顏色表在 `badge-variants.ts`，這裡放查表與判定）。
 */

/**
 * 文件歷程一列的中文。
 *
 * 🔴 **這一支存在的唯一理由是契約與規格不一致**（詳見 `types.ts` 的
 * `InvoiceTimelineItem`）：58A 說 `timeline[].event` 走
 * `INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL`，而 invoice-service 實際填進去的是
 * 狀態歷程的 `to_status`，值域是 11 個 `InvoiceDocumentStatus`。
 *
 * ⇒ 先查文件狀態表、再退回 timeline event 表、都查不到就**原樣顯示碼值**。
 * 兩張表都是 CodeMeta 生成物，這裡一個中文字都沒有手寫（§0.4 ⑤）。
 *
 * ⚠️ **default 分支不可省**（陷阱 C11）：後端哪天在 `invoice_status_history` 加一個
 * 新的 to_status，這裡回碼值、畫面照常；exhaustive switch 則是整頁白畫面，而測試
 * 用的是舊值域所以全綠。
 */
export function documentTimelineLabel(event: string): string {
	if (event === "") return "—";
	return (
		INVOICE_DOCUMENT_STATUS_LABEL[event] ??
		INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL[event] ??
		event
	);
}

/**
 * `fileAvailable === false` 時，列表要在 `.cell-sub` 說的那句話。
 *
 * ⚠️ **不寫中文對照表**：`fileUnavailableReason` 的值域
 * （`CARRIER_NOT_WON` / `EXPIRED` / `NOT_APPLICABLE`，見 `invoice.proto` 的
 * `file_unavailable_reason` 註解）**不是 CodeMeta enum**，沒有 SSOT。手寫一張三筆的
 * 對照表就是一份沒有來源的第二碼表（同 A03 對 `TaxDocumentInfo.kind` 的處置）。
 * ⇒ 前綴一句人看得懂的話 ＋ 原樣附上碼值，讓人在列表就知道點進去也載不到。
 */
export function fileUnavailableText(reason: string): string {
	return reason === "" ? "沒有可下載的檔案" : `沒有可下載的檔案（${reason}）`;
}

/**
 * 出貨狀態欄的三態（backend#3019）。
 *
 * 🔴 **`none` 與 `degraded` 是兩件事，不得塌成同一格。**
 * 後端在 shipping-service 不可用時整批回空字串並把 `shipmentStatusDegraded` 設 true；
 * 只看 `code` 的實作會讓每一列都畫成「還沒出貨」——一句我們無從得知的話。
 * 這一欄在 backend#3019 之前不敢畫，就是為了避免說這句話；修完之後它會換一個形式重演。
 *
 * ⚠️ 判定順序不可對調：`degraded` 必須在 `code === ""` 之前判，
 * 否則降級的空字串會先被歸成 `none`。
 */
export type ShipmentStatusDisplay =
	| { kind: "degraded" }
	| { kind: "none" }
	| { kind: "status"; label: string };

export function shipmentStatusDisplay(
	code: string,
	degraded: boolean,
): ShipmentStatusDisplay {
	if (degraded) {
		return { kind: "degraded" };
	}
	if (code === "") {
		return { kind: "none" };
	}
	// 值域是 ShippingStatus 的 value（數字字串）⇒ 走 CodeMeta 查表，不手寫中文。
	return { kind: "status", label: codeLabel(SHIPPING_STATUS_LABEL, code) };
}
