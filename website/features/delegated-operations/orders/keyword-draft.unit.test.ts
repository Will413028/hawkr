import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { keywordUrlValue } from "./filters";

/**
 * 訂單搜尋：**只打空白不算搜尋**。
 *
 * `setKeyword(keywordDraft || null)` 對 `"   "` 為 truthy，於是那幾個空白被寫進網址。
 * 後端不受影響（`repo.ts` 送出前已經 trim），會痛的是這一頁自己：react-query 的鍵吃的
 * 是**未 trim 的** keyword，空白字串換出一組全新的鍵 ⇒ 清單 ＋ 五張 KPI 共 6 支請求
 * 全部重打一次，而它們送出去的 query string 與「沒搜尋」那 6 支一模一樣。附帶把
 * `page` 重置回第 1 頁，並留下 `?keyword=%20%20%20` 這種可分享網址。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));

describe("keywordUrlValue", () => {
	it("純空白（含 tab／全形空白以外的空白字元）一律當成沒搜尋", () => {
		expect(keywordUrlValue("   ")).toBeNull();
		expect(keywordUrlValue("\t\n ")).toBeNull();
		expect(keywordUrlValue("")).toBeNull();
	});

	it("有內容時去掉前後空白再寫進網址", () => {
		expect(keywordUrlValue("  O005001417 ")).toBe("O005001417");
		expect(keywordUrlValue("梨山")).toBe("梨山");
	});

	it("中間的空白保留（品名可能真的含空白）", () => {
		expect(keywordUrlValue(" 高山 蜜梨 ")).toBe("高山 蜜梨");
	});
});

/**
 * 純函式對了但沒人用它＝沒修。這一條釘住呼叫點 —— 述詞很窄（只認函式名），
 * 但它擋得住「有人把 `.trim()` 拿掉、改回 `keywordDraft || null`」這唯一一種回歸。
 */
describe("DelegatedOrdersPage 的 debounce 寫入", () => {
	it("寫進網址前先經過 keywordUrlValue", () => {
		const source = readFileSync(
			join(
				ROOT,
				"features/delegated-operations/orders/DelegatedOrdersPage.tsx",
			),
			"utf8",
		);
		expect(source).toContain("const next = keywordUrlValue(keywordDraft)");
		expect(source).not.toContain("setKeyword(keywordDraft || null)");
	});
});
