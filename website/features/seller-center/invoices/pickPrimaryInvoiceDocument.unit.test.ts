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

	// 🔴 本檔的核心：後端是 DESC（最新在第一筆），原本的 `items[items.length - 1]`
	// 取到的是最舊那一張。dev 第 3 輪實測到賣家因此看到「已被重開取代」的舊票。
	it("後端 DESC 順序（新在前）時取新的，不是取尾端", () => {
		const list = [
			doc("AA10001655", "ISSUED", "2026-09-03T02:00:00Z"),
			doc("AA10001515", "SUPERSEDED", "2026-09-03T01:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10001655",
		);
	});

	// 鑑別力：順序反過來也要拿到同一張，才叫「不依賴後端排序」。
	// 少了這一條，把實作寫成 `items[0]` 也會通過上一條。
	it("順序顛倒（舊在前）時仍取新的", () => {
		const list = [
			doc("AA10001515", "SUPERSEDED", "2026-09-03T01:00:00Z"),
			doc("AA10001655", "ISSUED", "2026-09-03T02:00:00Z"),
		];
		expect(pickPrimaryInvoiceDocument(list)?.summary.invoiceNumber).toBe(
			"AA10001655",
		);
	});

	// 作廢重開是同一個事務內完成的，兩張的時間戳可能完全相同 ⇒ 只比時間會挑錯。
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
