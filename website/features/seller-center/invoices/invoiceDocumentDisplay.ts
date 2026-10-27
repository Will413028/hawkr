/**
 * `InvoiceDocumentStatus`（11 值）→ 賣家看到的 Chip 標籤與顏色。
 *
 * 🔴 **與買家面的 `lib/invoices/documentStatus.ts` 是兩張不同的表，不可共用。**
 * 買家看到的是**折疊投影**（`PENDING`／`ISSUING`／`ISSUE_FAILED`／`AWAITING_MANUAL`
 * ／`VOIDING` 五值全折成「處理中」，因為法規要求 provider 異常時不對買家宣稱失敗）；
 * 賣家看到的是 **11 個原值**，因為只有賣家處理得了異常。共用那一份會讓賣家永遠看不到
 * 「開立失敗」，也就永遠不知道該去處理（58A §S06 的顏色表）。
 *
 * 🔴 中文一律取自 CodeMeta（§0.4 ⑤），前端只寫「值 → 顏色」那一半。
 * 🔴 **必須有 default 分支**（§0.6 C11／§B-2.0 T13）：後端先上線一個前端還不知道的
 * 值時，落 default 顯示原始碼是安全的；沒有 default 就是白畫面，而測試用的是舊值域
 * 所以全綠。反向風險也要防——default 不得寫成把已知值一併吃掉的萬用分支，所以本檔
 * 逐值列舉，`unknown` 旗標讓測試驗得出「拿掉某個已知值時它才會落 default」。
 */

import type { ChipColor } from "@/components/ui/Chip";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/enums";

export interface InvoiceDocumentDisplay {
	label: string;
	color: ChipColor;
	/** 落到 default 分支（後端值域比前端新）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

/** 語意群 → 顏色。四群窮盡 11 個值（§S06 的表）。 */
const COLOR_BY_STATUS: Record<string, ChipColor> = {
	// 已開立／已完成
	[InvoiceDocumentStatus.ISSUED]: "success",
	[InvoiceDocumentStatus.RESOLVED_EXTERNALLY]: "success",
	// 處理中／待開立
	[InvoiceDocumentStatus.PENDING]: "warning",
	[InvoiceDocumentStatus.ISSUING]: "warning",
	[InvoiceDocumentStatus.VOIDING]: "warning",
	// 失敗／異常／待人工
	[InvoiceDocumentStatus.ISSUE_FAILED]: "danger",
	[InvoiceDocumentStatus.AWAITING_MANUAL]: "danger",
	// 已作廢／已取代／不適用／開立前取消
	[InvoiceDocumentStatus.VOIDED]: "default",
	[InvoiceDocumentStatus.SUPERSEDED]: "default",
	[InvoiceDocumentStatus.NOT_APPLICABLE]: "default",
	[InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE]: "default",
};

export function invoiceDocumentDisplay(status: string): InvoiceDocumentDisplay {
	const color = COLOR_BY_STATUS[status];
	if (!color) {
		// 值域外：顏色中性、label 直接印原始碼。印原始碼而不是「未知狀態」是刻意的
		// ——賣家截圖給客服時，那串碼是唯一能定位的東西。
		return { label: status || "—", color: "default", unknown: true };
	}
	return {
		label: INVOICE_DOCUMENT_STATUS_LABEL[status] ?? status,
		color,
		unknown: false,
	};
}

/** 賣家眼中「這份文件現在有異常」——決定 S07 要不要出 warning banner。 */
export function isInvoiceDocumentAbnormal(status: string): boolean {
	return (
		status === InvoiceDocumentStatus.ISSUE_FAILED ||
		status === InvoiceDocumentStatus.AWAITING_MANUAL
	);
}

/** 賣家眼中「還在跑」——決定 S07 要不要出「系統正在開立發票」那一句。 */
export function isInvoiceDocumentProcessing(status: string): boolean {
	return (
		status === InvoiceDocumentStatus.PENDING ||
		status === InvoiceDocumentStatus.ISSUING ||
		status === InvoiceDocumentStatus.VOIDING
	);
}
