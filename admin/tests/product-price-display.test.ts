import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";
import { formatProductPriceRange } from "../src/lib/product-price";

/**
 * 後端契約：價格欄位的 `"0"` ＝ 無價格（入站 `parsePrice("0") → null`、
 * 出站 `formatAmount(null) → "0"`）。ADM 兩張列出商品價格的表格必須照這個
 * 語意讀，否則管理員會看到「NT$ 0」而以為商家把商品標成零元。
 *
 * 為什麼要有這支測試：`"0"` 是 truthy，`if (!min && !max)` 這種 falsy 判斷
 * 完全擋不住它，而 TypeScript 也不會有意見（型別就是 string）。沒有斷言釘住，
 * 這個判斷隨時會被改回 falsy 版本而沒有任何機械保證會轉紅。
 */

test('"0" 是無價格的表達，不得印成 NT$ 0', () => {
	assert.equal(formatProductPriceRange("0", "0"), "--");
	assert.equal(formatProductPriceRange("0", undefined), "--");
	assert.equal(formatProductPriceRange(undefined, "0"), "--");
	// 單邊為 "0"：那一邊視為缺席，只印有價格的那一邊，不得出現 "NT$ 0" 字樣
	assert.equal(formatProductPriceRange("0", "250"), "NT$ 250");
	assert.equal(formatProductPriceRange("250", "0"), "NT$ 250");
});

test("兩端皆缺席時回佔位符而非空白或 undefined", () => {
	assert.equal(formatProductPriceRange(undefined, undefined), "--");
	assert.equal(formatProductPriceRange(null, null), "--");
	// 鍵存在但不是數字（後端漏欄或型別漂移）同樣不得把原字串直出
	assert.equal(formatProductPriceRange("abc", "abc"), "--");
	assert.equal(formatProductPriceRange("", ""), "--");
});

test("負數同樣視為無價格", () => {
	assert.equal(formatProductPriceRange("-100", "-50"), "--");
});

test("單一價格與區間的格式，含千分位", () => {
	assert.equal(formatProductPriceRange("100", "100"), "NT$ 100");
	assert.equal(formatProductPriceRange("100", "250"), "NT$ 100 - NT$ 250");
	assert.equal(
		formatProductPriceRange("1200", "35000"),
		"NT$ 1,200 - NT$ 35,000",
	);
	assert.equal(formatProductPriceRange("1200", "1200"), "NT$ 1,200");
	// 數字型別也要接受（後端未來若改回 number 不應該靜默壞掉）
	assert.equal(formatProductPriceRange(1200, 1200), "NT$ 1,200");
});

/**
 * 上面的單元測試只守住 helper 自己。真正會壞的是「頁面根本不呼叫 helper」——
 * `merchants/[id]` 原本就是裸插值 `NT$ {p.priceMin}~{p.priceMax}`，helper 修得
 * 再對也照不到它。所以這裡反向斷言：兩張表格都不得再出現裸插值的價格輸出。
 */
const PAGES = [
	"src/app/admin/products/page.tsx",
	"src/app/admin/merchants/[id]/page.tsx",
];

test("兩張商品表格都走同一個 helper，沒有裸插值", () => {
	for (const rel of PAGES) {
		const src = readFileSync(join(process.cwd(), rel), "utf8");
		assert.ok(
			src.includes("formatProductPriceRange(p.priceMin, p.priceMax)"),
			`${rel} 沒有透過 formatProductPriceRange 輸出價格區間`,
		);
		assert.ok(
			!/NT\$\s*\{?\s*p\.price(Min|Max)/.test(src),
			`${rel} 仍有裸插值的價格輸出（NT$ {p.priceMin...}），"0" 會被印成 NT$ 0`,
		);
		// 第二份同責 helper ＝ 兩頁再度漂移的起點
		assert.ok(
			!/function\s+formatPriceRange/.test(src),
			`${rel} 不應自己再定義一份價格區間 formatter`,
		);
	}
});
