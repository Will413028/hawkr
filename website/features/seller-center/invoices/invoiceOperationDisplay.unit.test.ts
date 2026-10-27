import { describe, expect, it } from "vitest";
import {
	INVOICE_OPERATION_STATUS_LABEL,
	InvoiceOperationStatus,
} from "@/lib/codemeta/enums";
import {
	invoiceOperationDisplay,
	isInvoiceOperationDone,
	isInvoiceOperationResolvedExternally,
} from "./invoiceOperationDisplay";

const ALL = Object.values(InvoiceOperationStatus);

describe("invoiceOperationDisplay", () => {
	it("covers every InvoiceOperationStatus value without falling to default", () => {
		// 20 值全部有歸屬。這一條是 default 分支的**鑑別力**測試：default 若被寫成
		// 萬用分支，拿掉任一個已知值都不會有人發現——`unknown` 旗標讓它會。
		expect(ALL).toHaveLength(20);
		for (const status of ALL) {
			const display = invoiceOperationDisplay(status);
			expect(display.unknown, `${status} fell to the default branch`).toBe(
				false,
			);
			expect(display.label).toBe(INVOICE_OPERATION_STATUS_LABEL[status]);
		}
	});

	it("falls to the default branch for a value the backend has not shipped to us yet", () => {
		const display = invoiceOperationDisplay("SOME_FUTURE_OPERATION_STATUS");
		expect(display.unknown).toBe(true);
		expect(display.color).toBe("default");
		expect(display.label).toBe("SOME_FUTURE_OPERATION_STATUS");
	});

	it("does not collapse failure into a neutral tone", () => {
		// S08 存在的理由就是「賣家看得到失敗」。三個失敗值都必須是 danger，
		// 否則這一頁與「處理中」長得一樣。
		for (const status of [
			InvoiceOperationStatus.FAILED_PERMANENT,
			InvoiceOperationStatus.FAILED_RETRY_EXHAUSTED,
			InvoiceOperationStatus.FAILED_UNKNOWN,
		]) {
			expect(invoiceOperationDisplay(status).color).toBe("danger");
		}
	});

	it("keeps the two terminal-ish states apart", () => {
		expect(isInvoiceOperationDone(InvoiceOperationStatus.SUCCEEDED)).toBe(true);
		expect(
			isInvoiceOperationDone(InvoiceOperationStatus.RESOLVED_EXTERNALLY),
		).toBe(false);
		expect(
			isInvoiceOperationResolvedExternally(
				InvoiceOperationStatus.RESOLVED_EXTERNALLY,
			),
		).toBe(true);
		expect(
			isInvoiceOperationResolvedExternally(InvoiceOperationStatus.SUCCEEDED),
		).toBe(false);
	});

	it("is a different value domain from the document status map", () => {
		// InvoiceOperationStatus 與 InvoiceDocumentStatus 只有四個字串長得一樣
		// （RESOLVED_EXTERNALLY／NOT_APPLICABLE／CANCELLED_BEFORE_ISSUE 三個重疊，
		// ISSUED 只在文件那一份、SUCCEEDED 只在本份）。拿其中一份去查另一份的表
		// 會查不到而顯示原始碼，這一條把「兩份不是同一個值域」寫成機械斷言。
		expect(invoiceOperationDisplay("ISSUED").unknown).toBe(true);
	});
});
