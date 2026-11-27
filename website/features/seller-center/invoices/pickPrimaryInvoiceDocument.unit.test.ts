import { describe, expect, it } from "vitest";
import type { SellerInvoiceDocumentItem } from "@/lib/invoices/seller-types";
import { pickPrimaryInvoiceDocument } from "./pickPrimaryInvoiceDocument";

/** 只填本函式讀得到的欄位；其餘以 as 斷言帶過，避免測試被無關欄位綁死。 */
function doc(
	invoiceNumber: string,
	status: string,
	issuedAt: string,
	updatedAt = "",
): SellerInvoiceDocumentItem {
	return {
		summary: { invoiceNumber, status, issuedAt },
		updatedAt,
	} as unknown as SellerInvoiceDocumentItem;
}

describe("pickPrimaryInvoiceDocument", () => {
	it("空清單回 null", () => {
		expect(pickPrimaryInvoiceDocument([])).toBeNull();
	});

	it("單一文件就是它", () => {
		const only = doc("AA10000001", "ISSUED", "2026-09-01T00:00:00Z");
		expect(pickPrimaryInvoiceDocument([only])?.summary.invoiceNumber).toBe(
			"AA10000001",
		);
	});

	it("後端 DESC 順序（新在前）時取新的，不是取尾端", () => {
		const list = [
			doc("AA10001655", "ISSUED", "2026-09-03T02:00:00Z"),
			doc("AA10001515", "SUPERSEDED", "2026-09-03T01:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10001655",
		);
	});

	it("順序顛倒（舊在前）時仍取新的", () => {
		const list = [
			doc("AA10001515", "SUPERSEDED", "2026-09-03T01:00:00Z"),
			doc("AA10001655", "ISSUED", "2026-09-03T02:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10001655",
		);
	});

	it("時間戳相同時，生效中的優先於已被取代的", () => {
		const sameTime = "2026-09-03T02:00:00Z";
		const list = [
			doc("AA10001515", "SUPERSEDED", sameTime),
			doc("AA10001655", "ISSUED", sameTime),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10001655",
		);
	});

	it("issuedAt 為空時退回 updatedAt", () => {
		const list = [
			doc("AA10000001", "ISSUED", "", "2026-09-03T01:00:00Z"),
			doc("AA10000002", "ISSUED", "", "2026-09-03T03:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10000002",
		);
	});

	it("全部都是 SUPERSEDED 時仍要挑出最新的那一張，而不是回 null", () => {
		const list = [
			doc("AA10000001", "SUPERSEDED", "2026-09-03T01:00:00Z"),
			doc("AA10000002", "SUPERSEDED", "2026-09-03T03:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10000002",
		);
	});
});
