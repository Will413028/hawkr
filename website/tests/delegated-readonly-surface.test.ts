import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { mock, test } from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
	delegatedProductsSubtitle,
	productRowActionHint,
	productRowActionLabel,
} from "../features/seller-center/products/productsCapabilityCopy";
import { withAppRouter } from "./mocks/app-router";

/**
 * 「有 read、沒有 write」的工作階段，畫面必須真的是唯讀（§5.3 導航顯示規則：
 * 頁面呈唯讀，不出現 write CTA 或 handler；§6.4：批次操作只在具備
 * `ADMIN_PRODUCT_BULK_OPERATE` 時出現）。
 *
 * dev 上那個只核發四個 read capability 的工作階段（`OPS-1KXURWYWIQRK0`）截下來的
 * 兩張畫面就是這條規則的反例：商店設定整份是可輸入的白底欄位＋兩顆換圖鈕＋一顆
 * 深綠「檢視變更並儲存」，商品頁副標寫著「本工作階段可編輯與發布商品」、16 列裡
 * 15 列給「編輯」、每一列都畫著勾選框。**全部要按下去才會被後端擋。**
 *
 * ⚠️ 這一組同時守另一半：**賣家自己的 `/seller-center` 一個字都不能變**。這兩支元件
 * 是賣家中心與代行工作區共用的薄殼，新加的每一個判斷在非代行模式下都必須等價於
 * 現況（`scope.hasCapability` 在賣家路徑恆回 true，且每處都再以 `!delegated` 短路）。
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

const PROFILE = "features/seller-center/profile/SellerProfileClient.tsx";
const PRODUCTS = "features/seller-center/products/ProductsPage.tsx";
const BRANDING = "features/seller/components/SellerProfileBranding.tsx";

/**
 * 逐個吐出某個 JSX 元件的開頭標籤，**註解已剝除**。
 *
 * 兩個坑與 `seller-workspace-ui-invariants.test.ts` 的 `openingAnchorTags` 相同：
 * 屬性裡的 `onChange={(e) => …}` 含 `>`，所以要追大括號深度；而標籤上方的註解常常
 * 就寫著要斷言的那個字，不剝掉的話把屬性整行刪掉測試照樣是綠的。
 */
function openingTags(text: string, name: string): string[] {
	const out: string[] = [];
	for (const m of text.matchAll(new RegExp(`<${name}(?=[\\s/>])`, "g"))) {
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
				out.push(`${code}>`);
				break;
			}
			code += c;
			i += 1;
		}
	}
	return out;
}

/* ============================================================
   一、換頭像／換封面：真的渲染，不是讀原始碼
   ============================================================ */

/**
 * 一份夠用的注入 scope。`SellerProfileBranding` 只讀 `mode`／`hasCapability`
 * （外加寫入時才會碰到的 `confirmMutation`／`repos`），其餘欄位給得出型別即可。
 */
function delegatedScope(capabilities: string[]) {
	const keys = {
		storefront: () => ["storefront"],
		products: () => ["products"],
		posts: () => ["posts"],
		orders: () => ["orders"],
		order: () => ["order"],
		orderCounts: () => ["orderCounts"],
	};
	return {
		mode: "delegated" as const,
		keys,
		canRead: () => true,
		ownerStoreId: "2685914682",
		confirmMutation: async () => ({ ok: true as const }),
		delegatedCommands: null,
		hasCapability: (capability: string) => capabilities.includes(capability),
	};
}

/**
 * 渲染商店設定右側那塊預覽（兩顆換圖入口的家）。
 *
 * `capabilities === null` ＝ **不注入 provider**，也就是賣家自己的 `/seller-center`。
 */
async function renderBranding(capabilities: string[] | null): Promise<string> {
	mock.reset();
	mock.module("@/lib/hooks/useQueryIdentity", {
		exports: {
			useQueryIdentity: () => ({ accountId: null, activeRole: null }),
		},
	});
	const [{ SellerProfileBranding }, scopeModule] = await Promise.all([
		import("../features/seller/components/SellerProfileBranding"),
		import("../lib/seller-data/scope"),
	]);
	const element = React.createElement(SellerProfileBranding, {
		logoUrl: null,
		coverUrl: null,
		storeName: "測試",
		accountId: "7487768729113814280",
		onSaved: () => undefined,
	});
	return renderToStaticMarkup(
		withAppRouter(
			capabilities === null
				? element
				: React.createElement(
						scopeModule.SellerDataScopeProvider,
						{
							scope: {
								...scopeModule.sellerDataRepos,
								repos: scopeModule.sellerDataRepos,
								...delegatedScope(capabilities),
							} as never,
						},
						element,
					),
		),
	);
}

const AVATAR_CTA = "點此換大頭貼";
const COVER_CTA = "點此換背景";

test("唯讀工作階段：商店設定的兩顆換圖入口與檔案輸入完全不渲染", async () => {
	const html = await renderBranding([
		"orders.read",
		"storefront.read",
		"products.read",
		"posts.read",
	]);
	// 預覽本身必須還在 —— 唯讀工作階段要看的正是這個。
	assert.ok(html.includes("商店外觀預覽"), "預覽被一起拿掉了");
	assert.ok(
		!html.includes(AVATAR_CTA),
		"缺 storefront.write 卻仍畫「點此換大頭貼」⇒ 按下去才被後端擋",
	);
	assert.ok(
		!html.includes(COVER_CTA),
		"缺 storefront.write 卻仍畫「點此換背景」",
	);
	// 兩顆鈕只是觸發器，真正的入口是那兩個 file input；留著它們等於留著 handler。
	assert.ok(
		!html.includes('type="file"'),
		"寫入 handler（file input）仍在 ⇒ §5.3 的「不出現 handler」沒有做到",
	);
});

test("具備 storefront.write 的代行工作階段：兩顆換圖入口照舊", async () => {
	const html = await renderBranding([
		"storefront.read",
		"storefront.write",
		"products.read",
	]);
	// 反向鑑別：把判斷寫死成 false（＝有權限的人也被拿掉功能）會在這裡紅。
	assert.ok(
		html.includes(AVATAR_CTA),
		"有 storefront.write 卻拿不到換頭像入口",
	);
	assert.ok(html.includes(COVER_CTA), "有 storefront.write 卻拿不到換封面入口");
	assert.ok(html.includes('type="file"'));
});

test("賣家自己的 /seller-center：沒有 provider 就沒有 capability 這回事", async () => {
	const html = await renderBranding(null);
	assert.ok(
		html.includes(AVATAR_CTA) && html.includes(COVER_CTA),
		"賣家路徑憑空少掉換圖入口 —— 那裡的 hasCapability 恆為 true，不該有任何分支打到它",
	);
	assert.ok(html.includes('type="file"'));
});

/* ============================================================
   二、商店設定表單本體（效果驅動的 state，無法 SSR 渲染 ⇒ 逐個標籤比對源碼）
   ============================================================ */

test("商店設定：每個可編輯欄位在無 storefront.write 時都不可輸入", () => {
	const text = source(PROFILE);
	assert.match(
		text,
		/const canWrite = delegated\s*\?\s*scope\.hasCapability\("storefront\.write"\)\s*:\s*hasCapability\(serverSelf, SellerCapability\.STOREFRONT_WRITE\)/,
		"canWrite 代行路徑必須來自 storefront.write，賣家路徑來自 GetMyStore 的 STOREFRONT_WRITE",
	);

	// 登入資訊那兩格本來就是 readOnly（唯讀常數），它們不參與這條判準。
	const editable = [
		...openingTags(text, "Input"),
		...openingTags(text, "Textarea"),
	].filter((tag) => tag.includes("onChange"));
	assert.ok(
		editable.length >= 6,
		`可編輯欄位只找到 ${editable.length} 個，解析壞了`,
	);
	// `readOnly={!canWrite}` 或 `readOnly={!canWrite || <更多限制>}` 都算過關 ——
	// 後者是**更嚴**（PayUni 鎖定的店名／商店介紹就是這一種），代行唯讀的保證不變。
	//
	// 只放行 `||` 不放行 `&&`：`!canWrite && x` 會讓沒權限的人在 x 為 false 時
	// 反而可以輸入，那正是這條判準要擋的東西。開頭的 `!canWrite` 也不能省，
	// 否則整個代行唯讀契約就沒了。
	const READ_ONLY_AT_LEAST_CAN_WRITE = /readOnly=\{!canWrite\s*(\}|\|\|)/;
	for (const tag of editable) {
		assert.match(
			tag,
			READ_ONLY_AT_LEAST_CAN_WRITE,
			`這個欄位在唯讀工作階段仍可輸入：${tag.replace(/\s+/g, " ").slice(0, 90)}`,
		);
	}

	// 下拉不是 <input>，沒有 readOnly；唯讀下只能 disabled。縣市／鄉鎮市區兩個下拉由
	// TownFields 渲染：這一頁要把 disabled 交給它，它要把 disabled 交給兩個下拉。
	const townFields = openingTags(text, "TownFields");
	assert.equal(townFields.length, 1, "縣市／鄉鎮市區由一個 TownFields 渲染");
	for (const tag of townFields) {
		assert.match(
			tag,
			/disabled=\{!canWrite\b/,
			`這組下拉在唯讀工作階段仍可改：${tag.replace(/\s+/g, " ").slice(0, 90)}`,
		);
	}
	const townFieldsSource = source(
		"features/account/screens/addresses/TownFields.tsx",
	);
	const townDropdowns = openingTags(townFieldsSource, "Dropdown");
	assert.equal(townDropdowns.length, 2, "縣市／鄉鎮市區兩個下拉");
	for (const tag of townDropdowns) {
		assert.match(
			tag,
			/disabled=\{disabled\b/,
			`TownFields 的下拉沒有吃 disabled：${tag.replace(/\s+/g, " ").slice(0, 90)}`,
		);
	}

	// 送出路徑本身也要擋：儲存鍵不渲染之後，表單的隱式送出仍到得了 handleSubmit。
	assert.match(
		text,
		/e\.preventDefault\(\);[\s\S]{0,240}?if \(!canWrite\) return;/,
		"handleSubmit 缺少 canWrite 守衛 ⇒ 「不出現 handler」只擋得住滑鼠",
	);
});

test("商店設定：唯讀時不渲染儲存鍵，提示也不再承諾寫入", () => {
	const text = source(PROFILE);
	const submit = openingTags(text, "Button").filter((tag) =>
		tag.includes('type="submit"'),
	);
	assert.equal(submit.length, 1, "商店設定只有一顆送出鍵");
	// **不渲染**而不是 disabled：disabled 與「尚未修改」在畫面上完全一樣。
	assert.ok(
		!submit[0].includes("disabled={!canWrite"),
		"儲存鍵用 disabled 表達沒權限 ⇒ 與「還沒改東西」分不出來",
	);
	const index = text.indexOf(submit[0]);
	const before = text.slice(0, index);
	assert.match(
		before.slice(-80),
		/\{canWrite && \(\s*$/,
		"儲存鍵必須整顆包在 {canWrite && (…)} 之內",
	);

	// 旁邊那句提示不得再指向一個不存在的動作。
	assert.match(
		text,
		/\{!canWrite\s*\?\s*delegated\s*\?\s*"本工作階段沒有核發店面編輯權限[^"]*"/,
		"唯讀時的提示仍寫著「將以平台管理員身分寫入」⇒ 指向一顆已經不在的按鈕",
	);
});

/* ============================================================
   三、商品頁：副標與每一列的動作字（純函式，逐組合驗）
   ============================================================ */

test("商品頁副標：四種 capability 組合各說各的實話", () => {
	assert.equal(
		delegatedProductsSubtitle(true, true),
		"本工作階段可編輯與發布商品",
	);
	assert.equal(
		delegatedProductsSubtitle(false, false),
		"本工作階段僅可檢視商品",
		"唯讀工作階段仍宣稱可編輯與發布 ⇒ 截圖 C-readonly-W3-products.png 的那一句",
	);
	// 兩個 capability 是分開核發的，只講其中一個就會在另一半上說謊。
	assert.equal(
		delegatedProductsSubtitle(true, false),
		"本工作階段可編輯商品，不可上架或下架",
	);
	assert.equal(
		delegatedProductsSubtitle(false, true),
		"本工作階段可上架或下架商品，不可編輯內容",
	);
	// 四句互不相同，否則上面四條有一半是白測的。
	assert.equal(
		new Set(
			[
				[true, true],
				[true, false],
				[false, true],
				[false, false],
			].map(([w, p]) => delegatedProductsSubtitle(w, p)),
		).size,
		4,
	);
});

test("商品列的動作字：權限與商品狀態是兩把獨立的鎖", () => {
	// 賣家路徑／全權代行：只有上架中的商品因為 3019 而變成「檢視」。
	assert.equal(productRowActionLabel(true, false), "編輯");
	assert.equal(productRowActionLabel(true, true), "檢視");
	// 缺 products.write：草稿與未啟用也進不了編輯，字面不得再寫「編輯」。
	assert.equal(
		productRowActionLabel(false, false),
		"檢視",
		"唯讀工作階段的草稿列仍寫「編輯」⇒ 點進去是一份全部鎖住的表單",
	);
	assert.equal(productRowActionLabel(false, true), "檢視");

	// 說明要講對原因：「請先下架」在一個連下架都做不到的工作階段是一句撞牆的指示。
	assert.equal(productRowActionHint(true, false), undefined);
	assert.equal(
		productRowActionHint(true, true),
		"上架中無法編輯內容，請先下架",
	);
	assert.match(productRowActionHint(false, true) ?? "", /沒有核發商品編輯權限/);
	assert.match(
		productRowActionHint(false, false) ?? "",
		/沒有核發商品編輯權限/,
	);
});

test("商品頁：勾選欄只在具備 products.publish 時渲染", () => {
	const text = source(PRODUCTS);
	assert.match(
		text,
		/const canPublish =\s*!delegated \|\| scope\.hasCapability\("products\.publish"\)/,
		"canPublish 必須以 !delegated 短路，賣家路徑才會逐位元不變",
	);

	// 表頭與資料列各一格，兩格都要被同一個條件包住。
	const cells = [
		...text.matchAll(/<(th|td)\s+className="seller-products-table__check"/g),
	];
	assert.equal(cells.length, 2, "勾選欄應該只有表頭與資料列兩格");
	for (const cell of cells) {
		// 兩格之間只准隔著註解（JSX 註解或 biome-ignore 那一行），不准隔著別的條件。
		const before = text.slice(0, cell.index ?? 0);
		assert.match(
			before.slice(-320),
			/\{canPublish && \(\s*(?:(?:\{\/\*[\s\S]*?\*\/\}|\/\/[^\n]*)\s*)*$/,
			"勾選欄沒有被 {canPublish && (…)} 包住 ⇒ 勾了不會有任何東西出現（批次列的條件同樣是 canPublish）",
		);
	}
	// 停用的框仍在宣稱「勾選這件事有意義」——不得退回那個寫法。
	assert.ok(
		!text.includes("disabled={delegated && !canPublish}"),
		"勾選框改回 disabled ⇒ 又是一條死路 affordance",
	);
	// 少畫一欄，空狀態的跨欄數要跟著少一欄。
	assert.match(
		text,
		/colSpan=\{\(delegated \? 8 : 10\) \+ \(canPublish \? 1 : 0\)\}/,
		"空狀態的 colSpan 沒有跟著勾選欄走",
	);
	// 表寬與捲動提示的門檻靠這個 class 分辨兩種欄數。
	assert.ok(
		text.includes("seller-products-table--noselect"),
		"少一欄卻沒有掛 --noselect ⇒ 表格底線多留 56px，捲動提示會說一句不成立的話",
	);
});

test("賣家路徑：預覽與商品頁短路成現況，商店設定看 GetMyStore 的能力", () => {
	// 商店設定與商品頁走 `!delegated ||`；預覽元件沒有 `delegated` 這個區域變數，
	// 用 `scope.mode !== "delegated"` 表達同一件事。
	assert.match(
		source(BRANDING),
		/const canWrite =\s*!readOnly &&\s*\(scope\.mode !== "delegated" \|\| scope\.hasCapability\("storefront\.write"\)\)/,
		"預覽元件的 canWrite 少了賣家路徑短路或 readOnly",
	);
	for (const [file, expected] of [
		[PROFILE, 0],
		[PRODUCTS, 2],
	] as const) {
		const guards = [
			...source(file).matchAll(/!delegated \|\| scope\.hasCapability\(/g),
		];
		assert.equal(
			guards.length,
			expected,
			`${file}：以 !delegated 短路的 capability 判斷數量變了`,
		);
	}
	// 賣家路徑的答案就是現況：可編輯與發布、非上架中的列給「編輯」。
	assert.equal(
		delegatedProductsSubtitle(true, true),
		"本工作階段可編輯與發布商品",
	);
	assert.equal(productRowActionLabel(true, false), "編輯");
	assert.equal(productRowActionHint(true, false), undefined);
});
