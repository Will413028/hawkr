import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 「分類與認證」在建立頁與編輯頁必須是**同一個面**（C6 定案：兩者都開，與設計稿
 * image6 一致）。
 *
 * 這一組守三件在型別與純函式測試裡都看不到的事：
 * ① 兩頁不再各自為政 —— 編輯頁曾經是唯讀 `dl`、建立頁卻可選且必填，同一個欄位在兩個
 *    入口有兩套政策
 * ② **唯讀工作階段（缺 `products.write`／上架中 3019）不得拿到可操作的控制項**。
 *    這一頁的 capability 判斷收斂成 `editable`，任何新加的下拉／編輯器漏掛它，就是一顆
 *    按下去才被後端擋的鈕（`tests/delegated-readonly-surface.test.ts` 的同一條規則）
 * ③ 分類下拉的 items 走共用 mapper，不再各頁一份
 *
 * ⚠️ 以「整段 JSX 元素」為單位比對，不是全檔 `includes`：全檔搜尋在「三個下拉裡只有
 * 一個掛了 `!editable`」的情況下照樣是綠的。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const EDIT =
	"features/delegated-operations/products/DelegatedProductEditPage.tsx";
const CREATE =
	"features/delegated-operations/products/DelegatedProductCreatePage.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

/**
 * 逐個吐出某個自閉合 JSX 元素的完整原始碼。
 *
 * 以「自成一行的 `/>`」當結束記號：屬性裡的箭頭函式含 `>`，用第一個 `>` 當結束會把
 * 元素切一半，於是後半段的 `disabled` 永遠找不到、斷言恆綠。
 */
function elements(source: string, name: string): string[] {
	return [
		...source.matchAll(new RegExp(`<${name}[\\s\\S]*?\\n\\s*/>`, "g")),
	].map((m) => m[0]);
}

const edit = read(EDIT);
const create = read(CREATE);

describe("編輯頁：分類與認證已開放編輯，且仍受 capability 約束", () => {
	it("鑑別力：解析真的抓得到元素（抓不到的話底下每一條都會空過）", () => {
		expect(elements(edit, "Dropdown")).toHaveLength(3);
		expect(elements(edit, "CertificationEditor")).toHaveLength(1);
	});

	it("三個分類下拉全部掛 !editable", () => {
		for (const tag of elements(edit, "Dropdown")) {
			expect(tag).toContain("!editable");
		}
	});

	it("認證編輯器掛 disabled={!editable}", () => {
		expect(elements(edit, "CertificationEditor")[0]).toContain(
			"disabled={!editable}",
		);
	});

	it("不再宣稱分類與認證只能由賣家本人維護", () => {
		// 這句話與新的可編輯面直接矛盾；留著它比沒有說明更糟。
		expect(edit).not.toContain("分類與認證標章由賣家本人維護");
	});

	it("認證的顯示走 certLabel，不再直接印碼值", () => {
		// 舊寫法是 `{cert.name}`，畫面上會出現 `3701`。
		expect(edit).not.toContain("{cert.name}");
	});
});

describe("建立頁與編輯頁的面一致", () => {
	it("兩頁的區塊標題都是「分類與認證」", () => {
		expect(edit).toContain("<h2>分類與認證</h2>");
		expect(create).toContain("<h2>分類與認證</h2>");
	});

	it("兩頁都有三層分類下拉", () => {
		expect(
			elements(create, "Dropdown").filter((t) => t.includes("分類")),
		).toHaveLength(3);
		expect(elements(edit, "Dropdown")).toHaveLength(3);
	});

	it("兩頁都掛認證編輯器", () => {
		expect(elements(create, "CertificationEditor")).toHaveLength(1);
		expect(elements(edit, "CertificationEditor")).toHaveLength(1);
	});

	it("兩頁的分類選項走同一個共用 mapper，不各自寫一份", () => {
		for (const [file, text] of [
			[EDIT, edit],
			[CREATE, create],
		] as const) {
			expect(text, file).toContain("categoryDropdownItems");
			expect(text, file).not.toContain(
				"nodes.map((node) => ({ value: String(node.id)",
			);
		}
	});
});
