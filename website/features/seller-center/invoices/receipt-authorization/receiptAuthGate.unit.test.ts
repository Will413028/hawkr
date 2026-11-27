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
		const descriptions = Object.values(RECEIPT_INELIGIBLE_REASON).map(
			(r) => receiptAuthGateCopy(r).description,
		);
		expect(new Set(descriptions).size).toBe(3);
	});

	it("撤銷的文案要說出「既有收據不受影響」", () => {
		expect(
			receiptAuthGateCopy(RECEIPT_INELIGIBLE_REASON.APPROVAL_REVOKED)
				.description,
		).toContain("既有的收據不受影響");
	});

	it("NOT_APPROVED 的文案要對得上它現在真正的母體", () => {
		const copy = receiptAuthGateCopy(RECEIPT_INELIGIBLE_REASON.NOT_APPROVED);
		expect(copy.description).not.toContain("只開放給");
		expect(copy.description).not.toContain("提交核定證明");
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
