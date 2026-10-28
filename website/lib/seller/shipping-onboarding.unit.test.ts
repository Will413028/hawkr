import { beforeEach, describe, expect, it, vi } from "vitest";
import { ShippingType } from "@/lib/codemeta/enums";
import {
	SHIPPING_TYPE,
	type ShippingOnboarding,
	submitCvsOnboarding,
	unsupportedOnboardingTypes,
} from "@/lib/seller/shipping";

const authedFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/client", () => ({ authedFetch }));

/**
 * 回歸測試：物流設定頁「靜默丟棄」。
 *
 * 該頁只寫死渲染黑貓（3652）與 7-11 店到店（3651）兩張卡。後端回第三種
 * `shippingType` 時那筆 onboarding 在 UI 上完全不存在 —— 賣家看不到、關不掉、
 * 也不知道它開著。這與「猜成別的 provider」是同一條禁令的兩面。
 */

function row(shippingType: string): ShippingOnboarding {
	return { shippingType, enabled: "true" };
}

describe("unsupportedOnboardingTypes", () => {
	it("本頁畫得出來的兩種不算 unsupported", () => {
		expect(
			unsupportedOnboardingTypes([
				row(SHIPPING_TYPE.HOME_DELIVERY),
				row(SHIPPING_TYPE.CVS),
			]),
		).toEqual([]);
	});

	it("[禁 fallback] 3654 商家自送不得被靜默吃掉", () => {
		const found = unsupportedOnboardingTypes([
			row(SHIPPING_TYPE.CVS),
			row(ShippingType.MERCHANT_DELIVERY),
		]);
		expect(found).toEqual([ShippingType.MERCHANT_DELIVERY]);
	});

	it("後端未來新增的任何值同樣會被說出來；重複只算一次", () => {
		expect(
			unsupportedOnboardingTypes([row("3999"), row("3999"), row("XYZ")]),
		).toEqual(["3999", "XYZ"]);
	});

	it("空的 shippingType 不算（那是缺欄位，不是新 provider）", () => {
		expect(unsupportedOnboardingTypes([row(""), {}])).toEqual([]);
	});
});

describe("submitCvsOnboarding", () => {
	beforeEach(() => {
		authedFetch.mockReset();
		authedFetch.mockResolvedValue({ data: {} });
	});

	it("does not post goodsType", async () => {
		await submitCvsOnboarding({
			senderName: "王小明",
			senderPhone: "0912345678",
			returnStoreCode: "123456",
		});

		expect(authedFetch).toHaveBeenCalledTimes(1);
		const [path, init] = authedFetch.mock.calls[0];
		expect(path).toBe("/app/shipping_onboardings/cvs");
		expect(init.json).not.toHaveProperty("goodsType");
	});
});
