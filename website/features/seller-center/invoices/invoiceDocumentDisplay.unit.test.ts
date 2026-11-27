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
		const display = invoiceDocumentDisplay("SOME_FUTURE_STATUS");
		expect(display.unknown).toBe(true);
		expect(display.color).toBe("default");
		expect(display.label).toBe("SOME_FUTURE_STATUS");
	});

	it("keeps the seller projection distinct from the buyer's folded one", () => {
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
