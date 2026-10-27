import { describe, expect, it } from "vitest";
import {
	isRevokedOrExpired,
	RECEIPT_INELIGIBLE_REASON,
	receiptAuthGateCopy,
} from "./receiptAuthGate";

describe("receiptAuthGateCopy", () => {
	it("三個已知理由全部有歸屬，沒有一個落 default", () => {
		for (const reason of Object.values(RECEIPT_INELIGIBLE_REASON)) {
			expect(
				receiptAuthGateCopy(reason).unknown,
				`${reason} 落到 default`,
			).toBe(false);
		}
	});

	it("三句描述互不相同——處置一樣不代表文案可以合併", () => {
		// 合併之後賣家不知道自己是「沒核定過」「被撤銷」還是「到期」，
		// 而三者的下一步實際上不同。
		const descriptions = Object.values(RECEIPT_INELIGIBLE_REASON).map(
			(r) => receiptAuthGateCopy(r).description,
		);
		expect(new Set(descriptions).size).toBe(3);
	});

	it("撤銷的文案要說出「既有收據不受影響」", () => {
		// 少了這句，賣家會以為已經寄出去的收據也失效了。
		expect(
			receiptAuthGateCopy(RECEIPT_INELIGIBLE_REASON.APPROVAL_REVOKED)
				.description,
		).toContain("既有的收據不受影響");
	});

	it("NOT_APPROVED 的文案要對得上它現在真正的母體", () => {
		// DEC-20260905-015836 之後，審核進行中的四個狀態拿到的是 eligible=true
		// ⇒ 落到 NOT_APPROVED 的只剩草稿／未通過／逾期／核定為應開立統一發票。
		// 舊文案「只開放給已核定免用統一發票的商店」對這批人全部是錯的。
		const copy = receiptAuthGateCopy(RECEIPT_INELIGIBLE_REASON.NOT_APPROVED);
		expect(copy.description).not.toContain("只開放給");
		expect(copy.description).not.toContain("提交核定證明");
		// 正面：要說出「送出審核之後就能先簽」這條新的下一步。
		expect(copy.description).toContain("送出稅務身分審核");
	});

	it("值域外落 default 而不是白畫面", () => {
		const copy = receiptAuthGateCopy("SOME_FUTURE_REASON");
		expect(copy.unknown).toBe(true);
		expect(copy.title.length).toBeGreaterThan(0);
		expect(copy.ctaHref).toBe("/seller-center/invoices/tax-status");
	});

	it("撤銷／到期算「曾經核定過」，未核定不算", () => {
		expect(isRevokedOrExpired(RECEIPT_INELIGIBLE_REASON.APPROVAL_REVOKED)).toBe(
			true,
		);
		expect(isRevokedOrExpired(RECEIPT_INELIGIBLE_REASON.APPROVAL_EXPIRED)).toBe(
			true,
		);
		expect(isRevokedOrExpired(RECEIPT_INELIGIBLE_REASON.NOT_APPROVED)).toBe(
			false,
		);
	});
});
