import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 代行訂單搜尋比對 `recipient_name`（`OrderRepository.java` 的 keyword 條件），而列表
 * 先前只顯示買家暱稱。代訂很常見，於是搜「周勾」跳出四筆、四列買家全叫 Cora Chou ——
 * **結果正確，但畫面無法自我解釋**，操作者的第一反應是「搜尋壞了」。
 *
 * #2478 的核心論證是「會騙人的搜尋比沒有搜尋更糟」（針對假陰性）；這是它的溫和版本。
 * 而 placeholder 早就寫著「搜尋訂單編號、收件人或商品名稱…」⇒ 我們已經告訴操作者會比對
 * 收件人，卻不給他看收件人。
 *
 * ⚠️ 斷言刻意切到**那一個 `<td>` 之內**，不是整檔 `includes`：把收件人搬去別欄
 * （或誤放進明細面板）時，整檔比對照樣是綠的，而那正是「欄位對不上」會發生的形狀。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const PAGE = "features/delegated-operations/orders/DelegatedOrdersPage.tsx";

const source = readFileSync(join(ROOT, PAGE), "utf8");

/** 「買家」那一格的完整原始碼（`<td className="seller-table__buyer">` → `</td>`）。 */
function buyerCell(): string {
	const start = source.indexOf('<td className="seller-table__buyer">');
	if (start < 0) return "";
	const end = source.indexOf("</td>", start);
	return end < 0 ? "" : source.slice(start, end);
}

/**
 * 副行那顆 `<small>` 的**內容**。
 *
 * ⚠️ 不要用「整格 includes 收件人：」代替它：本檔上方的說明註解裡也有這五個字，
 * 於是把副行整段刪掉之後那條斷言照樣是綠的。這個洞是實際踩到的。
 */
function recipientLine(): string {
	const m = buyerCell().match(/<small>([\s\S]*?)<\/small>/);
	return m ? m[1].trim() : "";
}

describe("代行訂單列表：買家欄的收件人副行", () => {
	it("鑑別力：真的切得到那一格（切不到的話底下每一條都會空過）", () => {
		const cell = buyerCell();
		expect(cell).not.toBe("");
		expect(cell).toContain("order.buyer.displayName");
		expect(cell.length).toBeGreaterThan(120);
	});

	it("鑑別力：切得到副行本身（切不到的話前綴那條會空過）", () => {
		expect(recipientLine()).not.toBe("");
	});

	it("收件人顯示在買家欄之內，而不是別的地方", () => {
		expect(buyerCell()).toContain("order.recipient.name");
	});

	it("帶「收件人：」前綴——兩行都是人名，不標分不出誰是誰", () => {
		// 斷言的是那顆 <small> 的內容，不是「整格出現過這五個字」（本檔註解裡也有）。
		expect(recipientLine()).toContain("收件人：");
		expect(recipientLine()).toContain("order.recipient.name");
	});

	it("空值走破折號，與明細面板同一個 fallback", () => {
		expect(recipientLine()).toMatch(/order\.recipient\.name \|\| "—"/);
	});

	/**
	 * **永遠顯示**是定版決策（不是「只在收件人≠買家時顯示」）：條件式會讓同一張表在
	 * 不同資料下長不一樣，QA 取樣得涵蓋兩種、操作者也會困惑。這一條擋的是日後有人
	 * 為了「資訊精簡」把它改成條件式。
	 */
	it("不是條件式顯示：買家欄內沒有拿收件人與買家比對的分支", () => {
		const cell = buyerCell();
		expect(cell).not.toMatch(/recipient\.name\s*!==\s*buyer/);
		expect(cell).not.toMatch(/buyer\.displayName\s*!==\s*.*recipient/);
	});

	/**
	 * 沿用既有的兩行式 pattern（`styles/features/seller-center.css` 的
	 * `.seller-table__buyer strong` / `small`），不新畫、不新增 token。
	 * 這一條同時擋住「改用寫死 px／色碼」——本 repo 的 `check-theme-tokens` 只掃 class，
	 * 掃不到行內樣式。
	 */
	it("用既有 small 元素承載副行，不自帶行內樣式", () => {
		const cell = buyerCell();
		expect(cell).toContain("<small>");
		expect(cell).not.toContain("style={{");
	});
});
