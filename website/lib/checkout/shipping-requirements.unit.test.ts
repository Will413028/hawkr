import { describe, expect, it } from "vitest";
import {
	isShippingDestinationReady,
	shippingDestinationKind,
} from "@/lib/checkout/shipping-requirements";
import { ShippingMethod } from "@/lib/codemeta/enums";

/**
 * 回歸測試：購物車／結帳的「未知配送方式 fail-open」。
 *
 * 舊行為是三元的最後一支 `: true` —— 沒見過的配送方式就完全不驗收件資訊。
 * 這裡的斷言一律是「未知值**不得**取得與已知 provider 相同的放行結果」。
 */

/**
 * client 不支援的 provider：還沒有人定義過收件目標的新通路，加上已刪除的
 * Uber 即時配送（ADR 2026-10-27-general-ecommerce-scope-cut D4；值域仍在
 * CodeMeta，後端仍可能回這個值）。
 */
const NEW_PROVIDERS = [
	ShippingMethod.UBER_INSTANT,
	ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE,
	ShippingMethod.MERCHANT_DELIVERY,
] as const;

const UNKNOWN_VALUES = ["", "3999", "SOME_NEW_CARRIER", undefined] as const;

describe("shippingDestinationKind", () => {
	it("已知兩種各自對到自己的收件目標", () => {
		expect(shippingDestinationKind(ShippingMethod.TAQBIN)).toBe("address");
		expect(
			shippingDestinationKind(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP),
		).toBe("store-pickup");
	});

	it("[禁 fallback] 新通路與未知值一律 unsupported，不得歸成 address 或 store-pickup", () => {
		for (const method of [...NEW_PROVIDERS, ...UNKNOWN_VALUES]) {
			expect(shippingDestinationKind(method)).toBe("unsupported");
		}
	});
});

describe("isShippingDestinationReady", () => {
	/** 兩個目標都沒填 —— 已知 provider 應該被擋。 */
	const nothingPicked = { hasAddress: false, hasStorePickup: false };
	/** 兩個目標都填了 —— 已知 provider 應該放行。 */
	const bothPicked = { hasAddress: true, hasStorePickup: true };

	it("黑貓看地址、7-11 看門市", () => {
		expect(
			isShippingDestinationReady(ShippingMethod.TAQBIN, {
				hasAddress: true,
				hasStorePickup: false,
			}),
		).toBe(true);
		expect(
			isShippingDestinationReady(ShippingMethod.TAQBIN, {
				hasAddress: false,
				hasStorePickup: true,
			}),
		).toBe(false);
		expect(
			isShippingDestinationReady(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, {
				hasAddress: true,
				hasStorePickup: false,
			}),
		).toBe(false);
		expect(
			isShippingDestinationReady(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, {
				hasAddress: false,
				hasStorePickup: true,
			}),
		).toBe(true);
	});

	it("[禁 fallback] 未知配送方式一律擋住 —— 連兩個目標都填齊了也不放行", () => {
		for (const method of [...NEW_PROVIDERS, ...UNKNOWN_VALUES]) {
			expect(isShippingDestinationReady(method, bothPicked)).toBe(false);
			expect(isShippingDestinationReady(method, nothingPicked)).toBe(false);
		}
		// 對照：已知 provider 在同一組輸入下是放行的，證明上面的 false 不是
		// 「這個函式對什麼都回 false」。
		expect(isShippingDestinationReady(ShippingMethod.TAQBIN, bothPicked)).toBe(
			true,
		);
	});
});
