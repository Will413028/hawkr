import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { SlaCountdown } from "./SlaCountdown";

/**
 * 折疊表本身已在 `lib/invoices/slaCountdown.unit.test.ts` 測過；這裡測的是
 * **元件真的把折疊結果接上去了**——兩者是不同的失敗：折疊表對、元件忘了掛
 * `aria-live`，症狀是讀屏使用者永遠聽不到期限變化，而畫面看起來完全正常。
 */
describe("SlaCountdown", () => {
	it("掛 aria-live=polite，且倒數文字在 DOM 裡（不是只有顏色）", () => {
		const markup = renderToStaticMarkup(
			createElement(SlaCountdown, {
				remainingBusinessDays: "2",
				overdueBusinessDays: "",
			}),
		);
		expect(markup).toContain('aria-live="polite"');
		expect(markup).toContain("剩餘 2 個工作天");
	});

	it("後端算不出來時顯示「處理期限計算中」，不是空白也不是 NaN", () => {
		const markup = renderToStaticMarkup(
			createElement(SlaCountdown, {
				remainingBusinessDays: "",
				overdueBusinessDays: "",
			}),
		);
		expect(markup).toContain("處理期限計算中");
		expect(markup).not.toContain("NaN");
	});

	it("逾期顯示正數天數，不顯示負號", () => {
		const markup = renderToStaticMarkup(
			createElement(SlaCountdown, {
				remainingBusinessDays: "-3",
				overdueBusinessDays: "3",
			}),
		);
		expect(markup).toContain("已逾期 3 個工作天");
	});
});
