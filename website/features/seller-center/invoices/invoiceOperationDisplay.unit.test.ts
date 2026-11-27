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
		expect(invoiceOperationDisplay("ISSUED").unknown).toBe(true);
	});
});
