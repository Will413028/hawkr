import { describe, expect, it } from "vitest";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/enums";
import {
	invoiceDocumentDisplay,
	isInvoiceDocumentAbnormal,
	isInvoiceDocumentProcessing,
} from "./invoiceDocumentDisplay";

const ALL = Object.values(InvoiceDocumentStatus);

describe("invoiceDocumentDisplay", () => {
	it("covers every InvoiceDocumentStatus value without falling to default", () => {
		// 11 值全部有歸屬。這一條是 default 分支的**鑑別力**測試：如果 default 被寫成
		// 萬用分支（例如 `return {label: status, color: "default", unknown: false}`），
		// 拿掉任一個已知值都不會有人發現——`unknown` 旗標讓它會。
		expect(ALL).toHaveLength(11);
		for (const status of ALL) {
			const display = invoiceDocumentDisplay(status);
			expect(display.unknown, `${status} fell to the default branch`).toBe(
				false,
			);
			expect(display.label).toBe(INVOICE_DOCUMENT_STATUS_LABEL[status]);
		}
	});

	it("falls to the default branch for a value the backend has not shipped to us yet", () => {
		// §0.6 C11：後端先上線一個前端還不知道的值時，落 default 而不是白畫面。
		const display = invoiceDocumentDisplay("SOME_FUTURE_STATUS");
		expect(display.unknown).toBe(true);
		expect(display.color).toBe("default");
		// label 印原始碼而不是「未知狀態」——賣家截圖給客服時那串碼是唯一線索。
		expect(display.label).toBe("SOME_FUTURE_STATUS");
	});

	it("keeps the seller projection distinct from the buyer's folded one", () => {
		// 買家面把 ISSUE_FAILED 折進「處理中」；賣家面**必須**看得到失敗，否則
		// 永遠不知道該去處理。
		const failed = invoiceDocumentDisplay(InvoiceDocumentStatus.ISSUE_FAILED);
		expect(failed.color).toBe("danger");
		expect(failed.label).not.toBe("處理中");
	});

	it("separates abnormal from processing", () => {
		expect(isInvoiceDocumentAbnormal(InvoiceDocumentStatus.ISSUE_FAILED)).toBe(
			true,
		);
		expect(
			isInvoiceDocumentAbnormal(InvoiceDocumentStatus.AWAITING_MANUAL),
		).toBe(true);
		expect(isInvoiceDocumentAbnormal(InvoiceDocumentStatus.ISSUING)).toBe(
			false,
		);

		expect(isInvoiceDocumentProcessing(InvoiceDocumentStatus.ISSUING)).toBe(
			true,
		);
		expect(isInvoiceDocumentProcessing(InvoiceDocumentStatus.PENDING)).toBe(
			true,
		);
		expect(
			isInvoiceDocumentProcessing(InvoiceDocumentStatus.ISSUE_FAILED),
		).toBe(false);
	});
});
