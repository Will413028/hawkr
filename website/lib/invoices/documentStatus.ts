/**
 * `documents[].status`（`InvoiceDocumentStatus`，11 值）→ 買家看得到的標籤與色調。
 * B04 的文件卡 Chip 與 B05 的詳情共用這一份，不各寫一份（58A §B04 §6）。
 *
 * 折疊規則（58A §B04 的狀態 Chip 列）：
 *   PENDING / ISSUING / ISSUE_FAILED / AWAITING_MANUAL / VOIDING → 「處理中」
 *   SUPERSEDED                                                  → 與 VOIDED 同語
 *   其餘五值                                                      → 各自對應
 *
 * 為什麼要折：法規要求 provider 異常時對買家只顯示「處理中」——把
 * `ISSUE_FAILED` 原樣印出去，買家會以為發票開不出來了，而實際上系統仍在重試。
 *
 * 🔴 兩條硬規則：
 *   1. **中文一律取自 CodeMeta**（§0.4 ⑤）。唯一的例外是折疊後的「處理中」——
 *      它不是任何一個 `InvoiceDocumentStatus` 的 label，而是五個值合併之後的
 *      買家投影，CodeMeta 沒有、也不該有這個值。
 *   2. **必須有 default 分支**（§0.6 C11）。後端先上線一個前端還不知道的值時，
 *      落 default 顯示「處理中」是安全的（既不宣稱已開立、也不宣稱失敗）；
 *      沒有 default 就是白畫面，而測試用的是舊值域所以全綠。
 */

import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/generated";

/** Chip 色調。對應既有的 `Chip` color 值域，不新增顏色語彙。 */
export type InvoiceDocumentTone = "default" | "success" | "warning" | "danger";

export interface InvoiceDocumentStatusDisplay {
	label: string;
	tone: InvoiceDocumentTone;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

/** 折疊後的「處理中」桶——見檔頭第 1 條例外。 */
const PROCESSING_LABEL = "處理中";

const PROCESSING: InvoiceDocumentStatusDisplay = {
	label: PROCESSING_LABEL,
	tone: "default",
	unknown: false,
};

function labelOf(status: string): string {
	return INVOICE_DOCUMENT_STATUS_LABEL[status] ?? PROCESSING_LABEL;
}

/**
 * 這份文件現在還有沒有「通知」可以重寄（端點 71／32 的前置條件）。
 *
 * 🔴 三個值逐字取自 invoice-service 的 `resendableStatus`
 * （`services/invoice-service/internal/usecase/notification_resend.go`），
 * **不是**前端自己推導的「看起來已經開好了」。落在三值之外時後端回的是
 * `9301` ＋「這份文件尚未開立完成，沒有可重新寄送的通知」——與「文件不存在」
 * 同一個碼（Tier A），前端分不出來 ⇒ 渲染那顆鍵等於保證買家按到一個看不懂的錯誤。
 *
 * ⚠️ `VOIDED` 在內是刻意的：作廢本身也要通知得出去。而 `SUPERSEDED` 不在內
 * ——被取代的那一張已經沒有要寄的東西了。
 */
export function canResendInvoiceNotification(status: string): boolean {
	switch (status) {
		case InvoiceDocumentStatus.ISSUED:
		case InvoiceDocumentStatus.RESOLVED_EXTERNALLY:
		case InvoiceDocumentStatus.VOIDED:
			return true;
		default:
			return false;
	}
}

/**
 * 把一個 `documents[].status` 折成買家看得到的標籤與色調。
 *
 * 參數刻意是 `string` 而不是 `InvoiceDocumentStatusValue`：收窄成 union 會讓
 * default 分支在型別上不可達，而那個分支正是本函式存在的理由之一。
 */
export function foldInvoiceDocumentStatus(
	status: string,
): InvoiceDocumentStatusDisplay {
	switch (status) {
		case InvoiceDocumentStatus.PENDING:
		case InvoiceDocumentStatus.ISSUING:
		case InvoiceDocumentStatus.ISSUE_FAILED:
		case InvoiceDocumentStatus.AWAITING_MANUAL:
		case InvoiceDocumentStatus.VOIDING:
			return PROCESSING;
		case InvoiceDocumentStatus.ISSUED:
			return {
				label: labelOf(InvoiceDocumentStatus.ISSUED),
				tone: "success",
				unknown: false,
			};
		// 「已被重開取代」對買家的意思就是這張已經不算數了，與已作廢同語——
		// 借 VOIDED 的 CodeMeta label，不手抄第二份中文。
		case InvoiceDocumentStatus.SUPERSEDED:
		case InvoiceDocumentStatus.VOIDED:
			return {
				label: labelOf(InvoiceDocumentStatus.VOIDED),
				tone: "warning",
				unknown: false,
			};
		case InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE:
			return {
				label: labelOf(InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE),
				tone: "default",
				unknown: false,
			};
		case InvoiceDocumentStatus.NOT_APPLICABLE:
			return {
				label: labelOf(InvoiceDocumentStatus.NOT_APPLICABLE),
				tone: "default",
				unknown: false,
			};
		case InvoiceDocumentStatus.RESOLVED_EXTERNALLY:
			return {
				label: labelOf(InvoiceDocumentStatus.RESOLVED_EXTERNALLY),
				tone: "warning",
				unknown: false,
			};
		default:
			return { ...PROCESSING, unknown: true };
	}
}
