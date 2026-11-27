import { describe, expect, it } from "vitest";
import {
	docNoun,
	documentsPageLabel,
	framedNavItem,
	invoiceSectionLabel,
	isReceiptKind,
	isReceiptOnlySeller,
} from "./receiptFraming";

describe("receiptFraming（backend#3066 裁決 B）", () => {

	it("RECEIPT 走收據措辭", () => {
		expect(isReceiptKind("RECEIPT")).toBe(true);
		expect(docNoun("RECEIPT")).toBe("收據");
		expect(invoiceSectionLabel("RECEIPT")).toBe("收據與稅籍");
	});

	it("B2B／B2C 與未知值一律走發票措辭（陰性）", () => {
		for (const kind of ["B2B", "B2C", "", "receipt", "UNKNOWN"]) {
			expect(isReceiptKind(kind), kind).toBe(false);
			expect(docNoun(kind), kind).toBe("發票");
		}
		expect(docNoun(undefined)).toBe("發票");
		expect(docNoun(null)).toBe("發票");
	});

	it("isReceiptOnlySeller：兩個判準各自成立即可", () => {
		expect(
			isReceiptOnlySeller({
				taxReviewStatus: "APPROVED_INVOICE_EXEMPT",
				invoiceMode: "AMEGO_AUTO",
			}),
		).toBe(true);
		expect(
			isReceiptOnlySeller({
				taxReviewStatus: "APPROVED_INVOICE_REQUIRED",
				invoiceMode: "PLATFORM_RECEIPT",
			}),
		).toBe(true);
	});

	it("isReceiptOnlySeller：一般賣家與 null 皆為 false（陰性）", () => {
		expect(
			isReceiptOnlySeller({
				taxReviewStatus: "APPROVED_INVOICE_REQUIRED",
				invoiceMode: "AMEGO_AUTO",
			}),
		).toBe(false);
		expect(isReceiptOnlySeller(null)).toBe(false);
	});

	it("審核進行中的賣家不是收據賣家（陰性；與 showsReceiptAuth 刻意分家）", () => {
		for (const status of [
			"SUBMITTED",
			"UNDER_REVIEW",
			"SUPPLEMENT_REQUIRED",
			"SELF_ISSUE_PENDING",
		]) {
			expect(
				isReceiptOnlySeller({
					taxReviewStatus: status,
					invoiceMode: "AMEGO_AUTO",
				}),
				status,
			).toBe(false);
		}
	});

	it("framedNavItem 只動發票那一項，其餘原樣（陰性）", () => {
		const invoices = { href: "/seller-center/invoices", label: "發票與稅籍" };
		const orders = { href: "/seller-center/orders", label: "訂單" };

		expect(framedNavItem(invoices, true).label).toBe("收據與稅籍");
		expect(framedNavItem(invoices, false)).toBe(invoices);
		expect(framedNavItem(orders, true)).toBe(orders);
	});

	it("documentsPageLabel：收據賣家看不到「發票」二字（陰性）", () => {
		expect(documentsPageLabel(true)).toBe("收據紀錄");
		expect(documentsPageLabel(true)).not.toContain("發票");
		expect(documentsPageLabel(false)).toBe("發票與收據紀錄");
	});
});
