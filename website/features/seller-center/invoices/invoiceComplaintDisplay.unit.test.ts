import { describe, expect, it } from "vitest";
import {
	INVOICE_COMPLAINT_STATUS_LABEL,
	InvoiceComplaintStatus,
} from "@/lib/codemeta/enums";
import { foldInvoiceComplaintStatus } from "@/lib/invoices/complaintStatus";
import {
	invoiceComplaintDisplay,
	isComplaintClosed,
	isComplaintOverdue,
	normalizeSellerComplaintView,
	SELLER_COMPLAINT_VIEWS,
} from "./invoiceComplaintDisplay";

const ALL = Object.values(InvoiceComplaintStatus);

describe("invoiceComplaintDisplay", () => {
	it("covers every InvoiceComplaintStatus value without falling to default", () => {
		expect(ALL).toHaveLength(7);
		for (const status of ALL) {
			const display = invoiceComplaintDisplay(status);
			expect(display.unknown, `${status} fell to the default branch`).toBe(
				false,
			);
			expect(display.label).toBe(INVOICE_COMPLAINT_STATUS_LABEL[status]);
		}
	});

	it("falls to the default branch for a value the backend has not shipped to us yet", () => {
		const display = invoiceComplaintDisplay("SOME_FUTURE_COMPLAINT_STATUS");
		expect(display.unknown).toBe(true);
		expect(display.color).toBe("default");
		expect(display.label).toBe("SOME_FUTURE_COMPLAINT_STATUS");
	});

	it("does not fold VIOLATION_CONFIRMED into RESOLVED the way the buyer view does", () => {
		const buyerResolved = foldInvoiceComplaintStatus(
			InvoiceComplaintStatus.RESOLVED,
		);
		const buyerViolation = foldInvoiceComplaintStatus(
			InvoiceComplaintStatus.VIOLATION_CONFIRMED,
		);
		expect(buyerResolved.title).toBe(buyerViolation.title);

		const sellerResolved = invoiceComplaintDisplay(
			InvoiceComplaintStatus.RESOLVED,
		);
		const sellerViolation = invoiceComplaintDisplay(
			InvoiceComplaintStatus.VIOLATION_CONFIRMED,
		);
		expect(sellerResolved.label).not.toBe(sellerViolation.label);
	});

	it("keeps the three views mutually exclusive and exhaustive over the 7 DB values", () => {
		const COVERAGE: Record<string, readonly string[]> = {
			AWAITING_SELLER: [
				InvoiceComplaintStatus.OPEN,
				InvoiceComplaintStatus.OVERDUE,
			],
			SELLER_RESPONDED: [
				InvoiceComplaintStatus.SELLER_RESPONDED,
				InvoiceComplaintStatus.ESCALATED_TO_ADM,
			],
			CLOSED: [
				InvoiceComplaintStatus.RESOLVED,
				InvoiceComplaintStatus.VIOLATION_CONFIRMED,
				InvoiceComplaintStatus.WITHDRAWN,
			],
		};
		expect(SELLER_COMPLAINT_VIEWS.map((view) => view.value)).toEqual(
			Object.keys(COVERAGE),
		);
		const covered = Object.values(COVERAGE).flat();
		expect(covered).toHaveLength(7);
		expect([...covered].sort()).toEqual([...ALL].sort());
		expect(new Set(covered).size).toBe(7);
	});

	it("rejects a view name the API does not accept", () => {
		expect(normalizeSellerComplaintView("CLOSED")).toBe("CLOSED");
		expect(normalizeSellerComplaintView("OPEN")).toBe("AWAITING_SELLER");
		expect(normalizeSellerComplaintView("")).toBe("AWAITING_SELLER");
	});

	it("treats a passed dueAt as overdue even before the scanner flips the status", () => {
		expect(isComplaintOverdue(InvoiceComplaintStatus.OPEN, "-1")).toBe(true);
		expect(isComplaintOverdue(InvoiceComplaintStatus.OVERDUE, "2")).toBe(true);
		expect(isComplaintOverdue(InvoiceComplaintStatus.OPEN, "2")).toBe(false);
		expect(isComplaintOverdue(InvoiceComplaintStatus.OPEN, "")).toBe(false);
	});

	it("marks exactly the three closed statuses as closed", () => {
		const closed = ALL.filter(isComplaintClosed);
		expect([...closed].sort()).toEqual(
			[
				InvoiceComplaintStatus.RESOLVED,
				InvoiceComplaintStatus.VIOLATION_CONFIRMED,
				InvoiceComplaintStatus.WITHDRAWN,
			].sort(),
		);
	});
});
