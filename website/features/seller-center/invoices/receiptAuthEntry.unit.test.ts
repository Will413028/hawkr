import { describe, expect, it } from "vitest";
import { SellerTaxReviewStatus } from "@/lib/codemeta/enums";
import { showsReceiptAuth } from "./SellerInvoiceOverviewPage";

/**
 * S01 上 S05 入口的可見性（DEC-20260905-015836 選 A）。
 *
 * 🔴 這份清單必須逐值鏡射端點 29 的 allowed-from（invoice-service
 * `receiptAuthEligibleStatuses`）。少一個值的後果特別難查：那個狀態的賣家永遠找不到
 * 入口，畫面上什麼錯誤都不會有——所以最後一個案例把「恰好 5 個為真」釘住，
 * 拿掉任一個值都會轉紅。
 */
const ALLOWED_FROM: string[] = [
	SellerTaxReviewStatus.SUBMITTED,
	SellerTaxReviewStatus.UNDER_REVIEW,
	SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
	SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
];

const OTHER_MODE = "AMEGO_AUTO";

describe("showsReceiptAuth", () => {
	it("審核進行中的四個狀態就開入口，不必等核定", () => {
		// 這四個是本次放寬的重點：核定完成到賣家簽授權之間的空窗要靠它消滅。
		for (const status of [
			SellerTaxReviewStatus.SUBMITTED,
			SellerTaxReviewStatus.UNDER_REVIEW,
			SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
			SellerTaxReviewStatus.SELF_ISSUE_PENDING,
		]) {
			expect(
				showsReceiptAuth({
					taxReviewStatus: status,
					invoiceMode: OTHER_MODE,
				}),
				status,
			).toBe(true);
		}
	});

	it("已核定免用統一發票照舊開入口（授權是冪等開關，核定後仍要能改）", () => {
		expect(
			showsReceiptAuth({
				taxReviewStatus: SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
				invoiceMode: OTHER_MODE,
			}),
		).toBe(true);
	});

	it("不在 allowed-from 的狀態不開入口（陰性）", () => {
		// 草稿／被駁回／逾期／已撤銷／核定為應開立統一發票 —— 這些人點進去只會撞
		// 端點 28 的 gate。
		for (const status of [
			SellerTaxReviewStatus.NOT_STARTED,
			SellerTaxReviewStatus.DRAFT,
			SellerTaxReviewStatus.REJECTED,
			SellerTaxReviewStatus.EXPIRED,
			SellerTaxReviewStatus.REVOKED,
			SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED,
		]) {
			expect(
				showsReceiptAuth({
					taxReviewStatus: status,
					invoiceMode: OTHER_MODE,
				}),
				status,
			).toBe(false);
		}
	});

	it("已經在走平台代製收據時一律開，與 taxReviewStatus 無關", () => {
		expect(
			showsReceiptAuth({
				taxReviewStatus: SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED,
				invoiceMode: "PLATFORM_RECEIPT",
			}),
		).toBe(true);
	});

	it("11 個稅務狀態中恰好這 5 個為真（漂移就轉紅）", () => {
		const truthy = Object.values(SellerTaxReviewStatus).filter((status) =>
			showsReceiptAuth({ taxReviewStatus: status, invoiceMode: OTHER_MODE }),
		);
		expect([...truthy].sort()).toEqual([...ALLOWED_FROM].sort());
	});
});
