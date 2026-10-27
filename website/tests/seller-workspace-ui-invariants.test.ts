import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 賣家工作區與代行工作區的「被切掉／讀不出」不變式。
 *
 * 這一組全部來自 56 號 dev 驗收的 188 張截圖人工覆核，共同性質是**壞掉時完全沒有
 * 訊號**：tsc 過、biome 過、單元測試過、build 過，只有真的把畫面截下來看才發現。
 * 每一條都對應一張已存證的截圖，修法各自寫在對應的 CSS 註解裡。
 *
 * ⚠️ 這裡刻意比對**源碼文字**而不是渲染結果：這些缺陷全部是 CSS 特異度／繼承／
 * 溢出，jsdom 不算版面，渲染出來也判不出來。要換成真的量測就得起瀏覽器，那屬於
 * `tests/e2e`。文字比對前先把空白收斂，才不會被 biome 的換行決定弄紅。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function css(path: string): string {
	return readFileSync(join(ROOT, path), "utf8")
		.replace(/\/\*[\s\S]*?\*\//g, " ")
		.replace(/\s+/g, " ");
}

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

const DELEGATED_CSS = "styles/features/delegated-operations.css";
const SELLER_CSS = "styles/features/seller-center.css";
const SELLER_SHARE_CARD_CSS = "styles/features/seller-share-card.css";

/**
 * A2 — 「新增圖片」那一格也是 `<li>` ＋ 直接子 `<button>`，所以移除鈕那條
 * `> li > button`（0-1-2）會壓過 `.adm-product-edit__image-add > button`（0-1-1），
 * 把 120×120 的上傳鈕變成右上角 28×28 的絕對定位小圓，內容被 `overflow: hidden`
 * 切成「新增圖」。截圖：`lane-W/W4-create-via-page-button.png`（三張穩定重現）。
 */
test("移除圖片鈕的樣式不得套到「新增圖片」那一格", () => {
	const text = css(DELEGATED_CSS);
	assert.ok(
		text.includes(
			".adm-product-edit__images > li:not(.adm-product-edit__image-add) > button",
		),
		"這條規則必須排除 .adm-product-edit__image-add，否則上傳鈕會被縮成右上角小圓並被切字",
	);
	assert.ok(
		!/\.adm-product-edit__images > li > button/.test(text),
		"未排除 add 格的 `> li > button` 選擇器不得存在",
	);
});

/**
 * A3 — `bordered` 的 hover 是 `bg-[var(--hkui-primary-50)]`，而那個 token 沒有 dark
 * 值（近白 `#f3f6ed`）。dark 補丁只改了 border 與 color，於是 hover 變成米黃字疊近白底
 * （實測約 1.05:1）。截圖：`lane-W/W18-end-dialog-sidebar-dark.png`。
 */
test("adm-quiet-button 的 dark 補丁必須連 hover 底色一起補", () => {
	const text = css(DELEGATED_CSS);
	const hover = text.match(
		/\[data-theme="dark"\] \.adm-quiet-button:hover \{([^}]*)\}/,
	);
	assert.ok(
		hover,
		"缺少 dark hover 規則 ⇒ hover 會落回近白的 --hkui-primary-50",
	);
	assert.match(
		hover[1],
		/background:/,
		"dark hover 必須自己指定底色，不能沿用 --hkui-primary-50",
	);
});

/**
 * A4 — `[data-theme="dark"] body a:not([data-hkui-button])`（0-2-2）會壓過
 * `.seller-orders__btn--primary` 的白字（0-1-0），實心森林綠底配橄欖綠字（實測約
 * 4.4:1）。`<button>` 版本不受影響，只有 `<Link>`／`<a>` 版本要宣告自己是按鈕。
 * 截圖：`lane-S/shots/S-chen-products-1440-dark.png`。
 *
 * ⚠️ **這一條的掃描範圍是全 repo 的 `.tsx`，而且 CTA class 是從 CSS 推導出來的**，
 * 兩件事都是刻意的：
 *
 *   1. 病因那條規則是 **`body a`**，作用域是整個站，不分賣家／代行／買家。原本這條
 *      只讀 `features/seller-center/products/ProductsPage.tsx` 一個檔、只認
 *      `seller-orders__btn--primary` 一個 class —— 於是它對另外 10 顆同形狀的
 *      CTA 連結**系統性失明**：代行工作區的 `.adm-topnav__cta`／`.adm-blocked__link`
 *      （5 顆，其中 `.adm-topnav__cta` 與 ProductsPage 那顆是**同一個畫面上**的
 *      同名按鈕）、賣家自己的 `.seller-orders__action`（3 顆）、買家面的
 *      `.post-viewer__follow`／`.chat-context__cta`（2 顆）。全部是實心底＋白字的
 *      連結，dark 下白字全被換成 `#b7cc89`，而測試是綠的。
 *      守一條全站規則就得用全站的掃描面。
 *   2. class 清單若手寫，下一顆新的實心 CTA class 依然不會被看見（就是上面那個
 *      失明的形狀）。改成從 CSS 推導：**自帶不透明 `background` ＋ 自帶淺色前景
 *      token** 的單一 class 選擇器，就是「這個 class 擁有自己的表面與字色」，任何
 *      套上它的 `<a>` 都必須 opt out。
 *
 * ⚠️ 判準刻意**不含**「只宣告前景、背景來自祖先」的情形（例：`.adm-rail__link`
 * 在固定深色側欄上宣告白字）。那一類 dark 下也會變橄欖綠，但落在深色底上仍可讀
 * （`#b7cc89` on `#2d4023` ＝ 6.4:1），要不要跟著改是設計決定，不是可讀性缺陷；
 * 混進來只會讓這條測試開始否決正常的內文連結。
 *
 * 與 `tests/cart-feedback-contract.test.ts` 互補：那一支守 `components/ui/Button.tsx`
 * 這個共用元件本身有沒有帶標記，這一支守**沒有走 Button 元件的手刻 CTA 連結**。
 */

/** 淺色／反色前景 token —— 這些值代表「我這顆的字是要壓在深色實心底上的」。 */
const INVERSE_FOREGROUND =
	/color\s*:\s*(?:var\(\s*--(?:color-neutral-white|color-neutral-cream|hkui-primary-foreground|hk-text-inverse)\b|#fff\b|#ffffff\b|white\b|rgb\(\s*var\(\s*--color-neutral-white-rgb)/i;

const SKIP_DIRS = new Set([
	"node_modules",
	".next",
	".git",
	".open-next",
	"dist",
	"coverage",
	"playwright-report",
	"test-results",
]);

function walk(dir: string, ext: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(join(ROOT, dir), { withFileTypes: true })) {
		if (SKIP_DIRS.has(entry.name)) continue;
		const rel = join(dir, entry.name);
		if (entry.isDirectory()) out.push(...walk(rel, ext));
		else if (entry.name.endsWith(ext)) out.push(rel);
	}
	return out;
}

/**
 * 從樣式表推導「自帶實心表面＋自帶淺色字」的 class 名。
 *
 * 只收**單一 class 選擇器**（`.foo`）：複合（`.a.b`）與後代（`.a b`）選擇器的
 * 命中條件不是「元素帶了這個 class」，拿來當清單會誤判。
 */
function solidCtaClasses(): Set<string> {
	const found = new Set<string>();
	for (const file of walk("styles", ".css")) {
		const text = css(file);
		for (const rule of text.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
			const body = rule[2];
			if (
				!/(?:^|;)\s*background(?:-color)?\s*:\s*(?!none|transparent)/.test(body)
			)
				continue;
			if (!INVERSE_FOREGROUND.test(body)) continue;
			for (const selector of rule[1].split(",")) {
				const one = selector.trim();
				if (/^\.[A-Za-z0-9_-]+$/.test(one)) found.add(one.slice(1));
			}
		}
	}
	return found;
}

/**
 * 逐個吐出 `<a …>` / `<Link …>` 的開頭標籤，**註解已剝除**。
 *
 * 兩個坑，都踩過：
 *
 *   1. ⚠️ 不能用 `/<Link\b[\s\S]*?>/`：屬性裡的 `onClick={(e) => …}`、`ref={(el) => …}`
 *      與 `{a > b}` 都含 `>`，正則會在那裡截斷，後面的 `className` 就看不到 ⇒ **漏報**。
 *      實測本 repo 有 6 個 `<a>`／`<Link>` 會被那個正則截掉 className，其中一個正是
 *      `components/ui/Button.tsx`。所以這裡追蹤大括號深度與引號，只在 depth 0 的 `>` 收尾。
 *   2. ⚠️ **註解必須剝掉**：這些標籤上方通常就寫著「少了 `data-hkui-button` 會怎樣」，
 *      若把註解算進標籤文字，`tag.includes("data-hkui-button")` 會被那句說明滿足 ——
 *      把屬性整行刪掉，測試照樣是綠的（本條第一版就是這樣，變異測試當場抓到）。
 *      剝除同時解決第二個問題：註解裡的引號、反引號與 `>`（例如寫到 `<button>`）
 *      本來會擾亂上面那個狀態機。
 */
function openingAnchorTags(text: string): { index: number; tag: string }[] {
	const out: { index: number; tag: string }[] = [];
	for (const m of text.matchAll(/<(?:a|Link)(?=[\s/>])/g)) {
		const start = m.index ?? 0;
		let i = start + m[0].length;
		let depth = 0;
		let quote: string | null = null;
		let code = m[0];
		while (i < text.length) {
			const c = text[i];
			if (quote) {
				code += c;
				if (c === "\\") {
					code += text[i + 1] ?? "";
					i += 2;
					continue;
				}
				if (c === quote) quote = null;
				i += 1;
				continue;
			}
			// 註解：整段丟掉，換成一個空白以免把兩個 token 黏起來。
			if (c === "/" && text[i + 1] === "/") {
				const end = text.indexOf("\n", i);
				i = end === -1 ? text.length : end;
				code += " ";
				continue;
			}
			if (c === "/" && text[i + 1] === "*") {
				const end = text.indexOf("*/", i + 2);
				i = end === -1 ? text.length : end + 2;
				code += " ";
				continue;
			}
			if (c === '"' || c === "'" || c === "`") quote = c;
			else if (c === "{") depth += 1;
			else if (c === "}") depth -= 1;
			else if (c === ">" && depth === 0) {
				out.push({ index: start, tag: `${code}>` });
				break;
			}
			code += c;
			i += 1;
		}
	}
	return out;
}

test("實心主 CTA 只要是連結就必須帶 data-hkui-button（全 repo）", () => {
	const ctaClasses = solidCtaClasses();
	// 鑑別力①：推導本身要有結果。token 改名或 CSS 搬檔會讓集合塌成空的，
	// 那時掃描永遠通過 —— 這條把它變紅。
	assert.ok(
		ctaClasses.size >= 20,
		`從 styles/ 推導出的實心 CTA class 只有 ${ctaClasses.size} 個，判準大概壞了`,
	);
	for (const expected of [
		"seller-orders__btn--primary",
		"seller-orders__action",
		"adm-topnav__cta",
		"adm-blocked__link",
	]) {
		assert.ok(
			ctaClasses.has(expected),
			`.${expected} 應該被判為實心 CTA，卻沒進集合`,
		);
	}

	const hasClass = (tag: string, name: string) =>
		new RegExp(`(?<![A-Za-z0-9_-])${name}(?![A-Za-z0-9_-])`).test(tag);
	/** 必須是**屬性**位置，不是被提到。註解已由 `openingAnchorTags` 剝掉，這是第二道。 */
	const optsOut = (tag: string) =>
		/(?:^|\s)data-hkui-button(?=[\s=/>])/.test(tag);

	const offenders: string[] = [];
	let guarded = 0;
	const tsx = ["app", "components", "features"].flatMap((dir) =>
		walk(dir, ".tsx"),
	);
	for (const file of tsx) {
		const text = source(file);
		for (const { index, tag } of openingAnchorTags(text)) {
			const hits = [...ctaClasses].filter((name) => hasClass(tag, name));
			if (hits.length === 0) continue;
			if (optsOut(tag)) {
				guarded += 1;
				continue;
			}
			const line = text.slice(0, index).split("\n").length;
			offenders.push(`${file}:${line} (${hits.sort().join(", ")})`);
		}
	}

	assert.deepEqual(
		offenders,
		[],
		`實心 CTA 連結少了 data-hkui-button ⇒ dark 下白字被換成 #b7cc89：\n  ${offenders.join("\n  ")}`,
	);
	// 鑑別力②：掃描真的看得到那些連結。選擇器改名／JSX 改寫後這條會先轉紅，
	// 而不是讓「零命中」被當成「全部合規」。
	assert.ok(
		guarded >= 10,
		`只掃到 ${guarded} 顆帶標記的實心 CTA 連結，掃描面大概斷了`,
	);
});

/**
 * A5 — 分享名片只顯示商店 ID；專屬網址留在 QR、分享與複製流程中，
 * 不再以容易擠壓版面的長字串出現在名片預覽。
 */
test("名片不顯示網址，並保留置中的商店 ID", () => {
	const text = css(SELLER_SHARE_CARD_CSS);
	const storeIdBlock = text.match(
		/\.seller-business-card__store-id \{([^}]*)\}/,
	);
	assert.ok(storeIdBlock, "找不到名片商店 ID 規則");
	assert.doesNotMatch(text, /\.seller-business-card__link(?:\s|\{|\.)/);
	assert.match(storeIdBlock[1], /text-align:\s*center/);
	assert.match(storeIdBlock[1], /font-variant-numeric:\s*tabular-nums/);
});

/**
 * A11 — 代行商品表的可用寬度會被情境列與側欄吃掉（實測 1440／1536 兩個視窗下卡片各只
 * 剩 851／839px，而表寬底線 956px），「操作」欄整欄落在裁切線外只露出「操」一個字。
 * 那一欄是單筆編輯與預覽的唯一入口，所以釘在右側。
 * 截圖：`lane-E2E/cache-partition/01-store-A.png`、`lane-W/C-partial-W3-products.png`。
 */
test("代行商品表的「操作」欄釘在右側", () => {
	const text = css(DELEGATED_CSS);
	// 錨點是 `:last-child` 而不是第 9 欄：勾選欄只在具備 `products.publish` 時渲染
	// （§6.4），欄數因此有 9 與 8 兩種，寫死序號會讓唯讀工作階段釘到「狀態」欄上。
	const block = text.match(
		/\.seller-products-table--delegated thead th:last-child, \.seller-products-table--delegated tbody td:last-child:not\(\.seller-orders__empty\) \{([^}]*)\}/,
	);
	assert.ok(block, "缺少釘住最後一欄（操作）的規則");
	assert.match(block[1], /position:\s*sticky/);
	assert.match(block[1], /right:\s*0/);
	assert.match(
		block[1],
		/background:/,
		"sticky 格必須有不透明底色，否則捲過去的內容會從底下透出來",
	);
	// 提示列的文案必須跟著改：叫人去捲一個不會動的欄位就是另一個假訊息。
	const page = source("features/seller-center/products/ProductsPage.tsx");
	assert.ok(
		!/請左右捲動查看「狀態」與「操作」欄/.test(page),
		"「操作」已釘住，提示不得再叫人捲它",
	);
});

/**
 * A12 — ADM 窄欄在 ≤960px 轉成橫條後，三塊固定短詞跟一串 email 擠在同一列，而 flex
 * 子項可以縮到 min-content ——中文的 min-content 是一個字，實測「操作紀錄」直排成四行。
 * 截圖：`lane-W/W2-dashboard-375.png`。
 */
test("窄視窗的 ADM 橫條：品牌與連結不換行、不被壓縮", () => {
	const text = css(DELEGATED_CSS);
	const narrow = text.match(
		/@media \(max-width: 960px\) \{[\s\S]*?\.adm-topnav/,
	);
	assert.ok(narrow, "找不到 ADM 窄欄的窄視窗區塊");
	const block = narrow[0].match(
		/\.adm-rail__brand, \.adm-rail__link \{([^}]*)\}/,
	);
	assert.ok(block, "缺少品牌與連結的 nowrap 規則");
	assert.match(block[1], /white-space:\s*nowrap/);
	assert.match(block[1], /flex:\s*none/);
});

/**
 * F — 縮圖框的底色曾是兩層近乎不透明的白（0.7／0.9），把 `--hkui-content2` 完全蓋掉，
 * 於是 dark 模式下一整排無圖商品變成刺眼的亮白方塊，而框裡什麼都沒有。
 * 截圖：`lane-W/W3-products-dark-1536.png`。
 */
test("商品縮圖框的底色要透得出主題色，且無圖時有圖示", () => {
	const text = css(SELLER_CSS);
	// 同名選擇器有兩份（另一份是 ≤960px 只改 width／height 的縮圖尺寸），取宣告底色的那一份。
	const block = [
		...text.matchAll(/\.seller-products-table__thumb \{([^}]*)\}/g),
	].find((m) => m[1].includes("background:"));
	assert.ok(block, "找不到宣告 .seller-products-table__thumb 底色的那條規則");
	assert.ok(
		!/rgba\(255,\s*255,\s*255,\s*0\.[5-9]/.test(block[1]),
		"近乎不透明的白會蓋掉 --hkui-content2 ⇒ dark 下是一排亮白方塊",
	);
	assert.match(
		block[1],
		/var\(--hkui-content2\)/,
		"底色必須以主題 token 收尾，才會跟著 dark 走",
	);
	const page = source("features/seller-center/products/ProductsPage.tsx");
	assert.ok(
		/<PostImageIcon size=\{20\} \/>/.test(page),
		"無圖時要畫圖示，不留純色空方塊（讀起來像壞掉）",
	);
});

/** 只剝註解、保留換行 —— 要靠縮排分辨「頂層規則」與「@media 內的規則」。 */
function cssKeepLines(path: string): string {
	return readFileSync(join(ROOT, path), "utf8").replace(
		/\/\*[\s\S]*?\*\//g,
		"",
	);
}

/** 取頂層（非 @media 內）那一份規則本體：頂層規則的收尾 `}` 在第 0 欄。 */
function topLevelBlock(text: string, selector: string): string | null {
	const re = new RegExp(
		`^${selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} \\{\\n([\\s\\S]*?)\\n\\}`,
		"m",
	);
	return text.match(re)?.[1] ?? null;
}

/** 把 `background` 的值切成一層一層（只在括號深度 0 的逗號切）。 */
function backgroundLayers(block: string): string[] {
	const value = block.match(/(?:^|;)\s*background:\s*([^;]*)/)?.[1];
	if (!value) return [];
	const layers: string[] = [];
	let depth = 0;
	let current = "";
	for (const c of value) {
		if (c === "(") depth += 1;
		else if (c === ")") depth -= 1;
		if (c === "," && depth === 0) {
			layers.push(current.trim());
			current = "";
			continue;
		}
		current += c;
	}
	if (current.trim()) layers.push(current.trim());
	return layers;
}

/**
 * A1 — 設計書兩條常駐列都不常駐。
 *
 * 設計書把情境列的元件名寫成 **Persistent** Context Bar（§5.2），§6.3「ADM 差異：
 * 上方金色情境列固定可見」與 §7.1「進入 /operate/overview，Context Bar 固定可見」
 * 各再要求一次；圖 5 的儲存列則是覆蓋在預覽卡上的固定列（水梨圖被切掉下半）。
 * 實作兩條都會捲走：`.adm-context-bar` 完全沒有 `position`，而
 * `.seller-profile__save-bar` 的 sticky 被包在 `@media (max-width: 540px)` 內。
 * 店面設定頁 1536 寬下整頁 1843px（約 1.8 屏）⇒ 代行中狀態、倒數、capability
 * chips、**結束代行**（唯一中止出口）與代行寫入警語會一起離開視窗。
 * 截圖：`lane-W/W14-storefront-light-1536.png`。
 */
test("金色情境列必須釘在視窗頂端，且底下要有一層不透明色", () => {
	const text = css(DELEGATED_CSS);
	const bar = text.match(/\.adm-context-bar \{([^}]*)\}/);
	assert.ok(bar, "找不到 .adm-context-bar 規則");
	assert.match(
		bar[1],
		/position:\s*sticky/,
		"設計書要求 Context Bar 固定可見（§5.2／§6.3／§7.1）",
	);
	assert.match(bar[1], /top:\s*0/, "釘在頂端才叫「上方…固定可見」");

	// 貼頂之後表單會從它底下捲過去，兩層半透明金色薄膜擋不住。
	const layers = backgroundLayers(bar[1]);
	assert.ok(
		layers.length >= 2,
		`情境列的 background 只有 ${layers.length} 層 ⇒ 沒有墊底色，捲過去的文字會透出來`,
	);
	const base = layers[layers.length - 1];
	assert.ok(
		!/gradient\(/.test(base) && !/^(transparent|none)$/.test(base),
		`最底層必須是不透明色，實際是 \`${base}\``,
	);
	assert.match(
		base,
		/^var\(--[a-z0-9-]+\)$/,
		"墊底色要引用既有色票，不要寫死字面值",
	);
});

test("情境列與 ADM 窄欄的疊序要明確訂出來，不靠預設值", () => {
	const text = css(DELEGATED_CSS);
	const railZ = text
		.match(/\.adm-rail \{([^}]*)\}/)?.[1]
		.match(/z-index:\s*(-?\d+)/)?.[1];
	const barZ = text
		.match(/\.adm-context-bar \{([^}]*)\}/)?.[1]
		.match(/z-index:\s*(-?\d+)/)?.[1];
	assert.ok(railZ, ".adm-rail 少了 z-index");
	assert.ok(
		barZ,
		"sticky 元素若不宣告 z-index，頁面內任何 positioned 卡片都可能疊在它上面",
	);
	assert.ok(Number(barZ) > 0, "情境列要壓在捲過去的內容之上");
	assert.ok(
		Number(barZ) < Number(railZ),
		`ADM 窄欄是外層 chrome，兩者相碰時該在上面（rail=${railZ}, bar=${barZ}）`,
	);
});

test("店面設定的儲存列在所有寬度都釘住，不只 ≤540px", () => {
	const lines = cssKeepLines(SELLER_CSS);
	const base = topLevelBlock(lines, ".seller-profile__save-bar");
	assert.ok(base, "找不到頂層的 .seller-profile__save-bar 規則");
	assert.match(
		base,
		/position:\s*sticky/,
		"設計書圖 5 的儲存列是固定列；桌機長表單捲到預覽區時送出鈕與代行警語會一起消失",
	);
	assert.match(base, /bottom:\s*0/, "釘在視窗底端");
	assert.match(
		base,
		/background:/,
		"sticky 之後表單會從它下面捲過去，沒有底色就是一層透光的疊層",
	);
	// 代行時那句警語是「這次寫入不是賣家自己做的」的唯一提示，跟送出鈕同一列。
	const client = source(
		"features/seller-center/profile/SellerProfileClient.tsx",
	);
	assert.ok(
		/此修改將以平台管理員身分寫入並通知賣家/.test(client),
		"儲存列上的代行警語不得移除",
	);
});

/**
 * A1（續）—— sticky 最惡劣的壞法是**靜默**：祖先鏈上只要有一個捲動容器，元素照樣
 * 排版、就是不釘，而 tsc／biome／單元測試／build 全綠。所以把那條鏈本身變成不變式。
 *
 * `overflow-x: clip` 是允許的（它不建立捲動容器，`.page-wrapper` 的既有註解就是為此
 * 特地選 clip 而不是 hidden）；`hidden`／`auto`／`scroll` 一律不行。
 */
const STICKY_ANCESTORS = [
	"page-wrapper",
	"page-content",
	"page-transition",
	"adm-workspace",
	"adm-workspace__main",
	"seller-shell",
	"seller-main",
	"seller-profile",
	"seller-profile__form",
];

test("兩條常駐列的祖先鏈上不得出現捲動容器", () => {
	const offenders: string[] = [];
	let scanned = 0;
	for (const file of walk("styles", ".css")) {
		const text = css(file);
		for (const rule of text.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
			for (const selector of rule[1].split(",")) {
				const one = selector.trim();
				const hit = STICKY_ANCESTORS.find((name) =>
					new RegExp(`\\.${name}(?![A-Za-z0-9_-])\\s*$`).test(one),
				);
				if (!hit) continue;
				scanned += 1;
				for (const d of rule[2].matchAll(
					/(?:^|;)\s*(overflow(?:-x|-y)?)\s*:\s*([a-z]+)/g,
				)) {
					const [, prop, value] = d;
					if (value === "visible" || value === "clip") continue;
					if (prop === "overflow-x" && value === "hidden") {
						offenders.push(`${file} [${one}] ${prop}: ${value}（改用 clip）`);
						continue;
					}
					offenders.push(`${file} [${one}] ${prop}: ${value}`);
				}
			}
		}
	}
	// 鑑別力：掃描面要真的掃得到那些選擇器，零命中不能被當成全部合規。
	assert.ok(scanned >= STICKY_ANCESTORS.length, `只掃到 ${scanned} 條祖先規則`);
	assert.deepEqual(
		offenders,
		[],
		`祖先鏈出現捲動容器 ⇒ 情境列／儲存列會靜默不釘：\n  ${offenders.join("\n  ")}`,
	);
});

test("html/body 的 overflow-x 最後一手必須是 clip 而不是 hidden", () => {
	// `decorations-overlays.css` 在 ≤768px 留了一份 `html, body { overflow-x: hidden }`
	// —— body 的 overflow-y 會因此算成 auto，body 變捲動容器，兩條 sticky 一起失效。
	// 它現在被較晚 import 的 `floating-navigation.css` 的 clip 蓋掉；這條測試守的是
	// **import 順序**，因為兩者特異度相同、誰後誰贏。
	const imports = [
		...cssKeepLines("styles/globals.css").matchAll(/@import\s+"\.\/([^"]+)"/g),
	].map((m) => `styles/${m[1]}`);
	assert.ok(imports.length > 5, "解析不到 globals.css 的 @import 順序");

	const declared: { file: string; value: string }[] = [];
	for (const file of imports) {
		for (const rule of css(file).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
			const subjects = rule[1].split(",").map((s) => s.trim());
			if (!subjects.some((s) => s === "html" || s === "body")) continue;
			for (const d of rule[2].matchAll(
				/(?:^|;)\s*overflow-x\s*:\s*([a-z]+)/g,
			)) {
				declared.push({ file, value: d[1] });
			}
		}
	}
	assert.ok(
		declared.length > 0,
		"html/body 完全沒有 overflow-x 宣告？判準大概壞了",
	);
	const last = declared[declared.length - 1];
	assert.equal(
		last.value,
		"clip",
		`最後生效的是 ${last.file} 的 overflow-x: ${last.value}；hidden 會讓 body 變捲動容器`,
	);
});
