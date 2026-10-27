/**
 * `InvoiceOperationStatus`（**20 值**）→ 賣家看到的 Chip 標籤與顏色（S08 ②）。
 *
 * 🔴 中文一律取自 CodeMeta（§0.4 ⑤），前端只寫「值 → 顏色」那一半。
 * 🔴 **必須有 default 分支**（§0.6 C11／§B-2.0 T13）：後端先上線一個前端還不知道的
 * 值時，落 default 顯示原始碼是安全的；沒有 default 就是白畫面，而測試用的是舊值域
 * 所以全綠。反向風險也要防——default 不得寫成把已知值一併吃掉的萬用分支，所以本檔
 * **逐值列舉 20 個**，`unknown` 旗標讓測試驗得出「拿掉某個已知值時它才會落 default」。
 *
 * ⚠️ 與 `./invoiceDocumentDisplay.ts` 是**兩個不同的值域**：那一份是文件狀態
 * （`InvoiceDocumentStatus`，11 值，「這張發票現在是什麼」），本份是 operation 狀態
 * （「對 AMEGO 的那一次呼叫現在是什麼」）。一張 ISSUED 的文件底下可以有一筆
 * SUCCEEDED 的 operation，也可以有一筆 CANCELLED_BY_ADMIN 的舊 operation。
 */

import type { ChipColor } from "@/components/ui/Chip";
import {
	INVOICE_OPERATION_STATUS_LABEL,
	InvoiceOperationStatus,
} from "@/lib/codemeta/enums";

export interface InvoiceOperationDisplay {
	label: string;
	color: ChipColor;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

/** 語意群 → 顏色。五群窮盡 20 個值。 */
const COLOR_BY_STATUS: Record<string, ChipColor> = {
	// 已完成
	[InvoiceOperationStatus.SUCCEEDED]: "success",
	[InvoiceOperationStatus.RESOLVED_EXTERNALLY]: "success",
	// 排程中／進行中
	[InvoiceOperationStatus.CREATED]: "default",
	[InvoiceOperationStatus.AWAITING_TRIGGER]: "default",
	[InvoiceOperationStatus.READY]: "default",
	[InvoiceOperationStatus.SENDING]: "warning",
	[InvoiceOperationStatus.RETRY_SCHEDULED]: "warning",
	[InvoiceOperationStatus.WAITING_CONFIRMATION]: "warning",
	[InvoiceOperationStatus.PROVIDER_STATUS_RECHECK]: "warning",
	[InvoiceOperationStatus.HELD]: "warning",
	[InvoiceOperationStatus.CANCEL_REQUESTED]: "warning",
	// 失敗
	[InvoiceOperationStatus.FAILED_PERMANENT]: "danger",
	[InvoiceOperationStatus.FAILED_RETRY_EXHAUSTED]: "danger",
	[InvoiceOperationStatus.FAILED_UNKNOWN]: "danger",
	[InvoiceOperationStatus.CANCEL_BLOCKED_UNKNOWN]: "danger",
	// 待賣家／已解鎖人工
	[InvoiceOperationStatus.SELLER_ACTION_REQUIRED]: "danger",
	[InvoiceOperationStatus.MANUAL_PROCESS_ALLOWED]: "primary",
	// 已終止（非失敗）
	[InvoiceOperationStatus.CANCELLED_BY_ADMIN]: "default",
	[InvoiceOperationStatus.NOT_APPLICABLE]: "default",
	[InvoiceOperationStatus.CANCELLED_BEFORE_ISSUE]: "default",
};

export function invoiceOperationDisplay(
	status: string,
): InvoiceOperationDisplay {
	const color = COLOR_BY_STATUS[status];
	if (!color) {
		// 值域外：顏色中性、label 直接印原始碼。印原始碼而不是「未知狀態」是刻意的
		// ——賣家截圖給客服時，那串碼是唯一能定位的東西。
		return { label: status || "—", color: "default", unknown: true };
	}
	return {
		label: INVOICE_OPERATION_STATUS_LABEL[status] ?? status,
		color,
		unknown: false,
	};
}

/** ② 的「這筆已經處理完成，不需要你做任何事」。 */
export function isInvoiceOperationDone(status: string): boolean {
	return status === InvoiceOperationStatus.SUCCEEDED;
}

/** ⑤a：這筆由賣家自行開立並回報完成。 */
export function isInvoiceOperationResolvedExternally(status: string): boolean {
	return status === InvoiceOperationStatus.RESOLVED_EXTERNALLY;
}
