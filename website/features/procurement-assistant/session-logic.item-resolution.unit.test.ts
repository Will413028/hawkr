import { describe, expect, it } from "vitest";
import { mapSlots } from "./session-logic";

/**
 * item_resolution 是 slots 裡唯一的巢狀物件，而它決定報價的價格基準（同一大類內
 * 品種實測差 2.96 倍）。這組測試守的是三件各自都會安靜壞掉的事：
 *
 *  1. wire 的 snake_case → camelCase 轉換不能漏欄位（漏了畫面上就少一行，沒有錯誤）
 *  2. 不合契約的內容要丟掉這個欄位、**不能**讓整包 slots 失敗
 *  3. kind／provenance 是白名單，不是 `as` 斷言
 */
describe("mapSlots 的 item_resolution", () => {
	it("CROP：整包欄位照 camelCase 轉過來", () => {
		const got = mapSlots({
			item: "高麗菜",
			item_resolution: {
				kind: "CROP",
				code: "LA1",
				label: "甘藍-初秋",
				provenance: "BUYER_CHOICE",
				display_value: "甘藍-初秋",
			},
		});

		expect(got.itemResolution).toEqual({
			kind: "CROP",
			code: "LA1",
			label: "甘藍-初秋",
			provenance: "BUYER_CHOICE",
			pricePerKg: undefined,
			displayValue: "甘藍-初秋",
		});
	});

	it("SELF_PRICED：price_per_kg 是數字、沒有 code", () => {
		const got = mapSlots({
			item: "阿嬤自種特殊品種高麗菜",
			item_resolution: {
				kind: "SELF_PRICED",
				provenance: "BUYER_CHOICE",
				price_per_kg: 95,
				display_value: "自訂品項，單價 95 元／公斤",
			},
		});

		expect(got.itemResolution?.kind).toBe("SELF_PRICED");
		expect(got.itemResolution?.pricePerKg).toBe(95);
		expect(got.itemResolution?.code).toBeUndefined();
		expect(got.itemResolution?.displayValue).toBe("自訂品項，單價 95 元／公斤");
	});

	it("尚未解析：欄位不在時是 undefined，其餘 slots 照常", () => {
		const got = mapSlots({ item: "高麗菜", quantity: 20 });
		expect(got.itemResolution).toBeUndefined();
		expect(got.item).toBe("高麗菜");
		expect(got.quantity).toBe(20);
	});

	it("內容不合契約時只丟掉這個欄位，不讓整包 slots 變空", () => {
		// 未知的 provenance——後端日後新增第三種值時就是這個形狀。
		const got = mapSlots({
			item: "高麗菜",
			quantity: 20,
			delivery_city: "台中市",
			item_resolution: {
				kind: "CROP",
				provenance: "SOMETHING_NEW",
				display_value: "甘藍-初秋",
			},
		});

		expect(got.itemResolution).toBeUndefined();
		expect(got.item).toBe("高麗菜");
		expect(got.quantity).toBe(20);
		expect(got.deliveryCity).toBe("台中市");
	});

	it("缺 display_value 就整個丟掉——顯示字串由伺服器組是這個型別存在的理由", () => {
		const got = mapSlots({
			item: "高麗菜",
			item_resolution: {
				kind: "GROUP",
				code: "moa-group:abc",
				label: "甘藍",
				provenance: "SYSTEM",
			},
		});

		expect(got.itemResolution).toBeUndefined();
		// 其餘 slots 不受影響
		expect(got.item).toBe("高麗菜");
	});

	it("未知的 kind 一樣被擋下", () => {
		const got = mapSlots({
			item_resolution: {
				kind: "WHOLE_CATEGORY",
				provenance: "SYSTEM",
				display_value: "甘藍",
			},
		});
		expect(got.itemResolution).toBeUndefined();
	});

	it("GROUP＋SYSTEM：兩個 provenance 值都認得", () => {
		const got = mapSlots({
			item_resolution: {
				kind: "GROUP",
				code: "moa-group:abc",
				label: "甘藍",
				provenance: "SYSTEM",
				display_value: "甘藍（整個大類）",
			},
		});

		expect(got.itemResolution?.provenance).toBe("SYSTEM");
		expect(got.itemResolution?.displayValue).toBe("甘藍（整個大類）");
	});
});
