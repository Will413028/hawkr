import { describe, expect, it } from "vitest";
import {
	CHECKOUT_AWAITING_INVOICE_NOTICE,
	checkoutInvoiceCarrier,
	checkoutPaidInvoiceNotice,
} from "./checkoutInvoiceNotice";
import type { InvoiceDocumentSummary } from "./types";

/**
 * 這張 display map 守兩件事：**後端加了前端還不知道的 `invoiceMode` 時這一格不能
 * 空白**（58A §0.6 C11），以及**付款完成當下絕不得宣稱發票已開立**（§B03 兩條硬
 * 禁止之一）。後者不是措辭問題：買家看到「已開立」就不會再回訂單頁看，而那一刻
 * 發票必然還在處理中。
 */
describe("付款完成頁的 invoiceMode 狀態句", () => {
	const KNOWN_MODES = [
		"AMEGO_AUTO",
		"PLATFORM_RECEIPT",
		"SELLER_SELF_ISSUE",
		"LEGACY_MANUAL",
	];

	it("四個已知值都有自己的句子，沒有一句是空的", () => {
		for (const mode of KNOWN_MODES) {
			expect(checkoutPaidInvoiceNotice(mode)).not.toBe("");
		}
	});

	it("未知的 mode 落 default，而不是空白", () => {
		for (const mode of ["", "AMEGO_SEMI_AUTO", "未來才會有的值"]) {
			expect(checkoutPaidInvoiceNotice(mode)).not.toBe("");
		}
		// default 走的是最安全的方向：不宣稱已開立、也不把開票責任指給任何一方。
		expect(checkoutPaidInvoiceNotice("AMEGO_SEMI_AUTO")).toBe(
			checkoutPaidInvoiceNotice("AMEGO_AUTO"),
		);
	});

	it("沒有任何一句宣稱發票已開立", () => {
		for (const mode of [...KNOWN_MODES, "未知值"]) {
			expect(checkoutPaidInvoiceNotice(mode)).not.toContain("已開立");
		}
	});

	it("賣家自行開立的兩個 mode 把責任指向賣家，平台開立的兩個不指向賣家", () => {
		for (const mode of ["SELLER_SELF_ISSUE", "LEGACY_MANUAL"]) {
			expect(checkoutPaidInvoiceNotice(mode)).toContain("賣家");
		}
		for (const mode of ["AMEGO_AUTO", "PLATFORM_RECEIPT"]) {
			expect(checkoutPaidInvoiceNotice(mode)).not.toContain("賣家");
		}
	});

	it("ATM 待轉帳的墊底句不說「付款完成」——那一頁的錢還沒到", () => {
		expect(CHECKOUT_AWAITING_INVOICE_NOTICE).not.toContain("付款完成");
		expect(CHECKOUT_AWAITING_INVOICE_NOTICE).not.toBe("");
	});
});

function doc(
	overrides: Partial<InvoiceDocumentSummary>,
): InvoiceDocumentSummary {
	return {
		documentId: "1",
		documentKind: "B2C",
		status: "ISSUED",
		invoiceNumber: "",
		issuedAt: "",
		totalAmount: "0",
		carrierSummary: "",
		carrierType: "",
		relatedDocumentId: "",
		fileAvailable: false,
		fileUnavailableReason: "",
		...overrides,
	};
}

describe("付款完成頁的載具摘要", () => {
	it("沒有文件時回空字串（呼叫端整列不渲染）", () => {
		expect(checkoutInvoiceCarrier([])).toBe("");
	});

	it("全部文件都沒有載具時回空字串，不回「無載具」之類的字面值", () => {
		expect(
			checkoutInvoiceCarrier([
				doc({ documentId: "1" }),
				doc({ documentId: "2" }),
			]),
		).toBe("");
	});

	it("多筆並存時取最新的那一筆——作廢重開後買家在用的是新的載具", () => {
		expect(
			checkoutInvoiceCarrier([
				doc({ documentId: "1", carrierSummary: "/OLD+11" }),
				doc({ documentId: "2", carrierSummary: "/NEW+22" }),
			]),
		).toBe("/NEW+22");
	});

	it("最新那筆沒有載具時往前找，不會因此整列消失", () => {
		expect(
			checkoutInvoiceCarrier([
				doc({ documentId: "1", carrierSummary: "/ABC+12" }),
				doc({ documentId: "2", carrierSummary: "" }),
			]),
		).toBe("/ABC+12");
	});
});
