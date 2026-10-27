import { describe, expect, it } from "vitest";
import { sectionsSentence } from "./DelegatedDashboardPage";

/**
 * 總覽說明列必須跟著實際渲染出來的區塊走。
 *
 * dev 實測（只核發 `products.read` 的工作階段）畫面上只有「商品」一塊，說明卻寫死
 * 「本次代行三支唯讀清單的總數」——讀的人會去找另外兩塊，找不到只能推論是壞了，而
 * 真相是這次沒有核發那兩項讀取權限。這與本輪其他缺陷同一個形狀：畫面替目標店家的
 * 狀態作證，而我們根本沒讀過。
 */
describe("sectionsSentence", () => {
	it("三塊全開時講三支，並列出是哪三支", () => {
		const text = sectionsSentence(["訂單", "商品", "動態"]);
		expect(text).toContain("三支");
		expect(text).toContain("訂單、商品、動態");
	});

	it("只核發商品讀取時不得再宣稱三支", () => {
		const text = sectionsSentence(["商品"]);
		expect(text).toContain("一支");
		expect(text).not.toContain("三支");
		expect(text).not.toContain("兩支");
		expect(text).toContain("商品");
		expect(text).not.toContain("訂單");
		expect(text).not.toContain("動態");
	});

	it("兩塊時講兩支", () => {
		const text = sectionsSentence(["訂單", "動態"]);
		expect(text).toContain("兩支");
		expect(text).toContain("訂單、動態");
	});

	/**
	 * 一項讀取權限都沒有時整頁走另一個空狀態（`本次代行沒有可彙總的資料`），這句話
	 * 根本不會被渲染 —— 回空字串而不是「零支」。
	 */
	it("零塊時回空字串", () => {
		expect(sectionsSentence([])).toBe("");
	});
});
