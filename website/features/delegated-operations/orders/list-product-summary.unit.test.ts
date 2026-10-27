import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { hasMoreThanOneItem } from "./display";
import { createDelegatedOrderRepo } from "./repo";

/**
 * 代行訂單搜尋比對 `order_items.product_name`／`sku_name`（`OrderRepository.java` 的
 * `SELLER_ORDER_FILTER`），而列表的「商品」欄只印得出「共 N 件」⇒ 搜一個品名拿到四筆、
 * 四列全是「共 2 件」，操作者**無法從畫面判斷是哪一件命中**，而 placeholder 早就寫著
 * 「搜尋訂單編號、收件人或商品名稱…」——我們承諾會比對品名，卻不給他看品名。
 *
 * 這與 `list-recipient-line.unit.test.ts`（收件人副行）是同一個病的第二個欄位：
 * **結果正確、但畫面無法自我解釋**，而操作者的第一反應是「搜尋壞了」。
 *
 * ⚠️ 原始碼斷言刻意切到**那一個 `<td>` 之內**，不是整檔 `includes`：把品名搬去別欄
 * （或只加在明細面板）時，整檔比對照樣是綠的，而那正是「欄位對不上」會發生的形狀。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const PAGE = "features/delegated-operations/orders/DelegatedOrdersPage.tsx";

const source = readFileSync(join(ROOT, PAGE), "utf8");

/** 「商品」那一格的完整原始碼（`<td className="seller-table__product">` → `</td>`）。 */
function productCell(): string {
	const start = source.indexOf('<td className="seller-table__product">');
	if (start < 0) return "";
	const end = source.indexOf("</td>", start);
	return end < 0 ? "" : source.slice(start, end);
}

/** 主行那顆 `<strong>` 的**內容**。 */
function primaryLine(): string {
	const m = productCell().match(/<strong>([\s\S]*?)<\/strong>/);
	return m ? m[1].trim() : "";
}

/**
 * 副行那顆 `<small>` 的**內容**。
 *
 * ⚠️ 不要用「整格 includes 等」代替它：格子上方的說明註解裡也有「等 N 件」四個字，
 * 而註解在 `<td>` 之外只是**目前**如此——有人把註解移進格內，那條斷言就會在副行被
 * 整段刪掉之後照樣是綠的。切到元素內容才與註解無關。
 */
function extraCountLine(): string {
	const m = productCell().match(/<small>([\s\S]*?)<\/small>/);
	return m ? m[1].trim() : "";
}

describe("hasMoreThanOneItem：第二行該不該出現", () => {
	it("單一品項不顯示——「等 1 件」對畫面沒有新增資訊，只會淹掉真正多品項的那幾列", () => {
		expect(hasMoreThanOneItem("1")).toBe(false);
	});

	it("多品項才顯示", () => {
		expect(hasMoreThanOneItem("2")).toBe(true);
		expect(hasMoreThanOneItem("12")).toBe(true);
	});

	/**
	 * 這三個是「不顯示」那一側的邊界。改成 `itemCount !== "1"` 會讓它們全部翻面，
	 * 於是一筆聚合失敗的訂單長出一行「等  件」。
	 */
	it("空值／零／非數字一律不顯示", () => {
		expect(hasMoreThanOneItem("")).toBe(false);
		expect(hasMoreThanOneItem("0")).toBe(false);
		expect(hasMoreThanOneItem("這不是數字")).toBe(false);
	});
});

describe("列表資料層：firstProductName", () => {
	function fetcherReturning(records: unknown[]) {
		return async () => ({
			data: {
				records,
				total: String(records.length),
				page: "1",
				pageSize: "10",
			},
		});
	}

	async function listWith(records: unknown[]) {
		const repo = createDelegatedOrderRepo(
			"2685914682",
			fetcherReturning(records) as never,
		);
		return (await repo.list()).records;
	}

	it("讀得到後端下發的第一件品項名", async () => {
		const [row] = await listWith([
			{ orderNo: "O001", firstProductName: "有機小松菜", itemCount: "3" },
		]);
		expect(row.firstProductName).toBe("有機小松菜");
	});

	/**
	 * 舊回應（或聚合失敗）沒有這個鍵時必須是空字串而不是 `undefined`：`undefined`
	 * 會讓 `order.firstProductName || "—"` 之外的任何字串操作在別處炸開，而型別上
	 * 它宣告的是 `string`。
	 */
	it("後端沒帶這個鍵時回空字串，不是 undefined", async () => {
		const [row] = await listWith([{ orderNo: "O002", itemCount: "1" }]);
		expect(row.firstProductName).toBe("");
	});

	/**
	 * 長品名**原樣帶過**，不在前端截斷。截斷會剛好切掉 keyword 命中的那一段，
	 * 於是又回到「搜到了卻看不出為什麼」——正是這張票要修的東西。要控寬度是 CSS
	 * 的事，不是資料層的。
	 */
	it("長品名原樣保留，不在資料層截斷", async () => {
		const long = `${"日光溫室小番茄禮盒".repeat(6)}特選 2 公斤裝`;
		const [row] = await listWith([
			{ orderNo: "O003", firstProductName: long, itemCount: "2" },
		]);
		expect(row.firstProductName).toBe(long);
		expect(row.firstProductName.length).toBeGreaterThan(50);
	});
});

describe("代行訂單列表：商品欄", () => {
	it("鑑別力：真的切得到那一格（切不到的話底下每一條都會空過）", () => {
		const cell = productCell();
		expect(cell).not.toBe("");
		expect(cell).toContain("order.firstProductName");
		expect(cell.length).toBeGreaterThan(80);
	});

	it("鑑別力：切得到主行與副行本身", () => {
		expect(primaryLine()).not.toBe("");
		expect(extraCountLine()).not.toBe("");
	});

	it("品名顯示在商品欄之內，而不是別的地方", () => {
		expect(primaryLine()).toContain("order.firstProductName");
	});

	it("品名為空走破折號，與同表其他欄位同一個 fallback", () => {
		expect(primaryLine()).toMatch(/order\.firstProductName \|\| "—"/);
	});

	/**
	 * 第二行綁 `hasMoreThanOneItem`：這一條擋的是日後有人改成 `order.itemCount !== "1"`
	 * 或無條件渲染——前者讓空字串長出「等  件」，後者讓每一列都掛一行「等 1 件」。
	 */
	it("第二行是「等 N 件」，且只在多品項時出現", () => {
		expect(extraCountLine()).toContain("order.itemCount");
		expect(extraCountLine()).toContain("等");
		expect(extraCountLine()).toContain("件");
		expect(productCell()).toContain("hasMoreThanOneItem(order.itemCount)");
	});

	/**
	 * 沿用既有的兩行式 pattern（`styles/features/seller-center.css` 把
	 * `.seller-table__buyer` 與 `.seller-table__product` 併在同一條規則），
	 * 不新畫、不新增 token。這一條同時擋住「改用寫死 px／色碼」——本 repo 的
	 * `check-theme-tokens` 只掃 class，掃不到行內樣式。
	 */
	it("用既有 strong／small 元素承載兩行，不自帶行內樣式", () => {
		const cell = productCell();
		expect(cell).toContain("<strong>");
		expect(cell).toContain("<small>");
		expect(cell).not.toContain("style={{");
	});

	/**
	 * 樣式必須與買家欄**共用同一條規則**而不是各複製一份宣告：兩份宣告會各自漂移，
	 * 而它們是同一張表的相鄰兩欄，漂移一眼看得出來卻沒有任何測試會轉紅。
	 */
	it("兩行式樣式與買家欄共用同一條 CSS 規則", () => {
		const css = readFileSync(
			join(ROOT, "styles/features/seller-center.css"),
			"utf8",
		);
		expect(css).toContain(
			".seller-table__buyer strong,\n.seller-table__product strong {",
		);
		expect(css).toContain(
			".seller-table__buyer small,\n.seller-table__product small {",
		);
	});
});
