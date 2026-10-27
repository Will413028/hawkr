import { describe, expect, it } from "vitest";
import {
	formatCurrencyUnitLabel,
	formatPriceLayerLabel,
	formatUnitLabel,
} from "./insightsFormat";

describe("formatUnitLabel", () => {
	it("對應後端送來的單位代碼", () => {
		expect(formatUnitLabel("kg")).toBe("每公斤");
		expect(formatUnitLabel("catty")).toBe("每台斤");
		expect(formatUnitLabel("100kg")).toBe("每百公斤");
	});

	// 未知單位不得被當成公斤——那是把「我不知道」渲染成一個具體的錯誤主張。
	it("未知或缺席的單位回單位待確認，不猜成公斤", () => {
		for (const unit of [undefined, null, "", "box", "KG", "公斤"]) {
			expect(formatUnitLabel(unit)).toBe("單位待確認");
		}
	});
});

describe("formatCurrencyUnitLabel", () => {
	it("表格 caption 用的寫法", () => {
		expect(formatCurrencyUnitLabel("kg")).toBe("新臺幣／公斤");
		expect(formatCurrencyUnitLabel("catty")).toBe("新臺幣／台斤");
		expect(formatCurrencyUnitLabel("unknown")).toBe("單位待確認");
	});
});

describe("formatPriceLayerLabel", () => {
	it("五個價格層級各有標籤", () => {
		expect(formatPriceLayerLabel("wholesale_auction")).toBe("批發拍賣");
		expect(formatPriceLayerLabel("farmgate")).toBe("產地價");
		expect(formatPriceLayerLabel("live_animal")).toBe("活體拍賣");
		expect(formatPriceLayerLabel("retail")).toBe("零售價");
		expect(formatPriceLayerLabel("wholesale_bulk")).toBe("躉售價");
	});

	// 回 null 而非空字串：呼叫端要能分辨「沒有層級資訊」與「層級是空的」，
	// 前者應該整塊不渲染，後者是資料錯誤。
	it("未知層級回 null 讓呼叫端決定不渲染", () => {
		expect(formatPriceLayerLabel(undefined)).toBeNull();
		expect(formatPriceLayerLabel("futures")).toBeNull();
	});
});
