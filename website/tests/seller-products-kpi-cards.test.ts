import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { nextSummaryTabIndex } from "../features/seller-center/components/SummaryStatCard";

/**
 * 商品管理摘要列的「未上架（4003）」卡必須在**賣家側也渲染**（#2725）。
 *
 * 代行建立的商品一律落 4003。那張卡原本包在 `{delegated && …}` 裡，於是賣家打開
 * 自己的商品頁時，四張卡（總商品數／上架中／庫存預警／草稿）沒有一格收得下 4003 ——
 * 那批商品看得到、算不進任何一格、也**沒有任何篩選器選得到**（這一頁沒有狀態下拉，
 * 卡片就是唯一的篩選器）。
 *
 * ⚠️ 這裡掃的是**源碼結構**而不是渲染結果：這一頁是 `"use client"` 元件，整棵樹掛
 * 著 scope／認證閘／zustand／分類樹，要渲染得先立起半個工作區，而要守的東西只有
 * 一句「那張卡不在 `delegated` 守衛底下」。掃描器自己的鑑別力由下面第二支測試釘住
 * ——它要求「庫存預警」那張**必須**被偵測為 `!delegated` 守衛，所以第一支測到的空
 * 集合不可能是掃描器壞掉的產物。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const PRODUCTS = "features/seller-center/products/ProductsPage.tsx";
const SUMMARY_STAT_CARD =
	"features/seller-center/components/SummaryStatCard.tsx";
const SELLER_CSS = "styles/features/seller-center.css";

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

/** 摘要列那一段 JSX（第一張卡 → 底下的商品面板之前）。 */
function statsRegion(text: string): string {
	const start = text.indexOf("<SummaryStatCard");
	const end = text.indexOf('<section className="seller-products__panel">');
	assert.ok(start !== -1 && end > start, "找不到摘要列那一段 JSX");
	return text.slice(start, end);
}

/**
 * 某個位置外圍還開著哪些 `{… && …}` 守衛（只回與 `delegated` 有關的那些）。
 *
 * 兩個坑：JSX 註解 `{/* … *\/}` 本身也是一對大括號，而註解裡就寫著 `delegated &&`
 * 這幾個字（正是被禁止的那個寫法），不剝掉的話刪掉守衛測試照樣是紅的；屬性裡的
 * `onClick={() => …}` 也是大括號，所以只能追深度、不能用正則抓。
 */
function delegatedGuardsAt(region: string, index: number): string[] {
	const stack: string[] = [];
	let i = 0;
	let quote: string | null = null;
	while (i < index) {
		const c = region[i];
		if (quote) {
			if (c === "\\") {
				i += 2;
				continue;
			}
			if (c === quote) quote = null;
			i += 1;
			continue;
		}
		if (region.startsWith("{/*", i)) {
			const end = region.indexOf("*/}", i);
			i = end === -1 ? region.length : end + 3;
			continue;
		}
		if (c === '"' || c === "'" || c === "`") {
			quote = c;
			i += 1;
			continue;
		}
		if (c === "{") {
			stack.push(region.slice(i + 1, i + 60));
			i += 1;
			continue;
		}
		if (c === "}") {
			stack.pop();
			i += 1;
			continue;
		}
		i += 1;
	}
	return stack.filter((head) => /^\s*!?\s*delegated\s*&&/.test(head));
}

/** 每張 `<SummaryStatCard>` 的位置與它外圍的 delegated 守衛。 */
function statCards(region: string): { at: number; guards: string[] }[] {
	return [...region.matchAll(/<SummaryStatCard/g)].map((m) => ({
		at: m.index ?? 0,
		guards: delegatedGuardsAt(region, m.index ?? 0),
	}));
}

test("「未上架」卡不得被 delegated 守衛擋在賣家側之外", () => {
	const region = statsRegion(source(PRODUCTS));
	const at = region.indexOf("PRODUCT_STATUS_LABEL[ProductStatus.INACTIVE]");
	assert.ok(at !== -1, "摘要列裡找不到未上架（4003）那張卡");
	assert.deepEqual(
		delegatedGuardsAt(region, at),
		[],
		"未上架卡包在 delegated 守衛裡 ⇒ 賣家在自己的商品頁沒有任何篩選器找得到 4003",
	);
});

test("掃描器認得出守衛：「庫存預警」確實只在賣家側渲染", () => {
	// 這支不是在守庫存預警，是在守上面那支的鑑別力：守衛偵測若失效，上面會靜默轉綠。
	const region = statsRegion(source(PRODUCTS));
	const at = region.indexOf('label="庫存預警"');
	assert.ok(at !== -1, "摘要列裡找不到庫存預警那張卡");
	const guards = delegatedGuardsAt(region, at);
	assert.equal(guards.length, 1, "庫存預警應剛好被一層 delegated 守衛包住");
	assert.match(guards[0], /^\s*!\s*delegated\s*&&/);
});

test("賣家側的未上架數字要真的去數 4003，不是恆為 0", () => {
	const text = source(PRODUCTS);
	const from = text.indexOf("const summaryStats = React.useMemo(");
	assert.ok(from !== -1, "找不到 summaryStats 的計算");
	const memo = text.slice(from, text.indexOf("\treturn (", from));
	assert.ok(
		/off:\s*source\.filter\(\(p\) => isOffStatus\(p\.status\)\)\.length/.test(
			memo,
		),
		"賣家分支必須從清單數 4003；寫死 0 會讓卡片永遠顯示 0 而看起來像「沒有下架商品」",
	);
	const predicate = text.slice(text.indexOf("function isOffStatus"));
	assert.match(
		predicate.slice(0, 160),
		/"4003"/,
		"isOffStatus 必須認 4003（另認舊資料的數字碼 2）",
	);
});

test("五張卡在桌面維持五欄，手機則採用訂單同款緊湊單列", () => {
	const text = source(PRODUCTS);
	const cards = statCards(statsRegion(text));
	const forSeller = cards.filter(
		(c) => !c.guards.some((g) => /^\s*delegated\s*&&/.test(g)),
	).length;
	const forDelegated = cards.filter(
		(c) => !c.guards.some((g) => /^\s*!\s*delegated\s*&&/.test(g)),
	).length;
	assert.equal(forSeller, 5, "賣家側應有五張卡");
	assert.equal(forDelegated, 4, "代行側應維持四張卡");
	assert.match(
		text,
		/const statCardCount = delegated \? 4 : 5;/,
		"卡片數若與實際渲染的張數對不上，格線欄數就會跟著錯",
	);
	assert.ok(
		text.includes('statCardCount === 5 ? " seller-inventory__stats--5"'),
		"五張卡的那一列必須帶上 --5 變體，否則第五張會自己佔滿第二列的 1/4 寬",
	);
	assert.match(
		text,
		/seller-inventory__stats seller-summary-rail seller-summary-rail--compact-phone/,
		"商品摘要列必須加入手機緊湊變體，避免五張大卡只露出兩張",
	);

	const css = readFileSync(join(ROOT, SELLER_CSS), "utf8");
	const variant = css.indexOf(".seller-inventory__stats--5 {");
	assert.ok(variant !== -1, "缺少 .seller-inventory__stats--5");
	assert.match(
		css.slice(variant, variant + 160),
		/grid-template-columns:\s*repeat\(5, minmax\(0, 1fr\)\);/,
	);
	// 桌面變體先被 ≤1100px 的兩欄規則覆蓋；≤960px 再由 opt-in
	// 水平滑軌接手。三段順序不能顛倒，否則窄螢幕會被扳回五欄或兩欄。
	const narrow = css.search(
		/@media \(max-width: 1100px\) \{\s*\.seller-inventory__stats \{/,
	);
	assert.ok(narrow !== -1, "找不到 ≤1100px 的兩欄規則");
	const mobileRail = css.indexOf(
		"/* Seller summary cards use a compact multi-browse rail",
	);
	assert.ok(mobileRail !== -1, "找不到 ≤960px 的摘要卡水平滑軌規則");
	assert.ok(
		variant < narrow && narrow < mobileRail,
		"桌面五欄、窄版兩欄與手機水平滑軌的 CSS 順序不正確",
	);
	const phoneTiles = css.indexOf(
		"/* Product and inventory summaries keep the existing 186px browse rail",
	);
	assert.ok(phoneTiles > mobileRail, "手機緊湊覆寫必須放在通用滑軌之後");
	assert.match(
		css.slice(phoneTiles),
		/@media \(max-width: 540px\)[\s\S]*seller-summary-rail--compact-phone\.seller-inventory__stats--5\s*\{[^}]*grid-template-columns:\s*repeat\(5, minmax\(64px, 1fr\)\)/,
		"手機賣家側的五張卡必須維持單列，不得掉到第二列",
	);
});

test("商品摘要 tablist 使用 roving tabIndex，並宣告水平鍵盤操作", () => {
	const text = source(PRODUCTS);
	assert.match(text, /aria-orientation="horizontal"/);
	assert.match(text, /onKeyDown=\{handleSummaryTabKeyDown\}/);
	assert.match(text, /nextSummaryTabIndex\(/);
	assert.match(text, /nextTab\.focus\(\);\s*nextTab\.click\(\);/s);
	assert.equal(
		[...statsRegion(text).matchAll(/tabIndex=\{statusFilter ===/g)].length,
		5,
		"五張可能出現的商品狀態卡都必須由選取狀態控制 Tab 停駐點",
	);

	const card = source(SUMMARY_STAT_CARD);
	assert.match(
		card,
		/role="tab"[\s\S]*aria-selected=\{active\}[\s\S]*tabIndex=\{tabIndex\}/,
	);
});

test("商品摘要方向鍵會換卡並在首尾循環，Home／End 直達兩端", () => {
	assert.equal(nextSummaryTabIndex("ArrowRight", 1, 5), 2);
	assert.equal(nextSummaryTabIndex("ArrowRight", 4, 5), 0);
	assert.equal(nextSummaryTabIndex("ArrowLeft", 3, 5), 2);
	assert.equal(nextSummaryTabIndex("ArrowLeft", 0, 5), 4);
	assert.equal(nextSummaryTabIndex("Home", 3, 5), 0);
	assert.equal(nextSummaryTabIndex("End", 1, 5), 4);
	assert.equal(nextSummaryTabIndex("Enter", 1, 5), null);
	assert.equal(nextSummaryTabIndex("ArrowRight", -1, 5), null);
	assert.equal(nextSummaryTabIndex("ArrowRight", 0, 0), null);
});
