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
	// backend#3066 裁決 B：收據與發票在賣家中心是兩個並列的東西，文案各自為政。
	// 這一組測試的重點是**陰性**那一半：不該切換的情況不得切換。

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
		// 尚未載入：undefined / null 不得誤判成收據
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
		// 載入中／停權／查詢失敗 ⇒ 不得把一般賣家的側欄改成「收據」
		expect(isReceiptOnlySeller(null)).toBe(false);
	});

	it("審核進行中的賣家不是收據賣家（陰性；與 showsReceiptAuth 刻意分家）", () => {
		// DEC-20260905-015836 放寬的是「能不能簽收據代製授權」，不是「他會不會有發票」。
		// 這四個狀態最後都可能被核定為「應開立統一發票」⇒ 現在就把側欄改成「收據與
		// 稅籍」，是在對他宣稱一件可能為假的事。
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
		// 非收據賣家：原樣，且必須是同一個物件參考（不得無謂重建）
		expect(framedNavItem(invoices, false)).toBe(invoices);
		// 其他項：即使是收據賣家也不得改
		expect(framedNavItem(orders, true)).toBe(orders);
	});

	it("documentsPageLabel：收據賣家看不到「發票」二字（陰性）", () => {
		expect(documentsPageLabel(true)).toBe("收據紀錄");
		expect(documentsPageLabel(true)).not.toContain("發票");
		// 一般賣家那一頁真的兩種都列得出來 ⇒ 名稱不得被收窄
		expect(documentsPageLabel(false)).toBe("發票與收據紀錄");
	});
});
