import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 管理員看得到的文案不得外洩後端服務名與傳輸層術語。
 *
 * 「重量與長寬高由 product-service 驗證『必須大於 0』」講的規則是對的，但它讓一個
 * 內部服務的名字出現在營運人員的畫面上 —— 那既不可行動（他無從得知 product-service
 * 是什麼），也是一份不該外流的部署資訊。「後端不接受…」同理，只是把責任推給一個
 * 讀者看不到的角色。
 *
 * ⚠️ 掃描前先剝註解：本目錄的檔頭大量引用 `product-service`／`9260` 來解釋為什麼要
 * 這樣寫，那是給工程師看的說明，不是畫面上的字。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const CREATE_PAGE =
	"features/delegated-operations/products/DelegatedProductCreatePage.tsx";

/**
 * 逐行剝註解：只剝**自成一行**的註解。
 *
 * ⚠️ 不能用「找出所有區塊註解並整段刪掉」的正則。這一頁的檔案輸入有
 * `accept="image` ＋ 萬用字元，那個斜線加星號會被當成註解開頭，一路吃到下一個
 * 結束記號為止，把整段 JSX 連同要檢查的文案一起吞掉 —— 於是每一條
 * `not.toContain` 都會空過（本測試第一版即如此，被「剝完仍留著 JSX 文字」那條
 * 當場擋下）。改成逐行判斷後，字串裡的斜線星號不在行首，因此不成立。
 */
function stripComments(source: string): string {
	const kept: string[] = [];
	let inBlock = false;
	for (const line of source.split("\n")) {
		const trimmed = line.trim();
		if (inBlock) {
			if (trimmed.includes("*/")) inBlock = false;
			continue;
		}
		if (trimmed.startsWith("//")) continue;
		if (trimmed.startsWith("/*") || trimmed.startsWith("{/*")) {
			if (!trimmed.includes("*/")) inBlock = true;
			continue;
		}
		kept.push(line);
	}
	return kept.join("\n");
}

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

const raw = read(CREATE_PAGE);
const rendered = stripComments(raw);

/** 畫面上不得出現的內部用語。 */
const FORBIDDEN = ["product-service", "後端"];

describe("代行建立商品頁：管理員可見文案", () => {
	it("剝註解後仍留著 JSX 文字（否則底下每一條都會空過）", () => {
		expect(rendered).toContain("發布狀態");
		expect(rendered).toContain("重量與長寬高");
	});

	for (const token of FORBIDDEN) {
		it(`畫面文案不出現「${token}」`, () => {
			expect(rendered).not.toContain(token);
		});
	}

	it("鑑別力：同一組判定式在未剝註解的原始碼上會命中", () => {
		// 註解裡本來就寫著 product-service（解釋逐條驗、第一條失敗就 return），
		// 命中它證明這組 token 搜得到東西，`not.toContain` 不是因為搜錯而空過。
		expect(raw).toContain("product-service");
	});

	it("規則本身沒有被刪掉，只是換成業務語言", () => {
		expect(rendered).toContain("必須大於 0");
	});
});

/**
 * 建立商品的兩條路徑必須都是**畫面上看得見的按鈕**。
 *
 * 這一頁曾經只在文案裡寫「建立後一律是草稿」，把「要不要上架」變成一個操作者無從
 * 選擇、也無從察覺的隱藏預設。兩顆按鈕在的時候，操作者知道自己剛才做了哪一種；
 * 只剩一顆的時候，他只會以為那就是唯一的做法。
 */
describe("代行建立商品頁：兩條路徑都要顯性", () => {
	it("兩顆按鈕的文字都在 JSX 裡（不是只寫在文案或註解）", () => {
		expect(rendered).toContain("儲存，暫不上架");
		expect(rendered).toContain("儲存並上架");
	});

	it("「暫不上架」是主要按鈕，「並上架」是次要", () => {
		// 主／次靠 variant 表達：次要那顆帶 bordered，主要那顆不帶。把上架畫成主要
		// 會讓一次誤點就對買家可見，而那是 products.publish 才有的權力。
		// 兩個字串在「發布狀態」卡片的說明裡也各出現一次，所以取**最後**一次 ——
		// 底部工具列在檔案最下方，那才是按鈕本身。
		const publish = rendered.lastIndexOf("儲存並上架");
		expect(publish).toBeGreaterThan(-1);
		const publishButton = rendered.slice(
			rendered.lastIndexOf("<Button", publish),
			publish,
		);
		expect(publishButton).toContain('variant="bordered"');

		const draft = rendered.lastIndexOf("儲存，暫不上架");
		expect(draft).toBeGreaterThan(-1);
		const draftButton = rendered.slice(
			rendered.lastIndexOf("<Button", draft),
			draft,
		);
		expect(draftButton).not.toContain("variant=");
	});

	it("缺 products.publish 時補一句說明，而不是靜默少一顆按鈕", () => {
		// 上架鍵在缺 capability 時**不渲染**（L3857：停用的按鈕仍在宣稱這件事做得到），
		// 但少了說明，畫面就退回「只有一顆按鈕」＝ 看起來又沒有選擇。
		expect(rendered).toMatch(/\{canPublish \? null : \(/);
		expect(rendered).toContain("本次工作階段沒有「商品上架」授權");
	});

	it("不得改回 disabled（推翻 L3857 的裁決）", () => {
		expect(raw).not.toContain("disabled={!canPublish}");
		expect(raw).not.toContain("disabled={delegated && !canPublish}");
	});
});
