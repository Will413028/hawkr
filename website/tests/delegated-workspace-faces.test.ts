import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import test from "node:test";
import {
	DELEGATED_AREA_CAPABILITY,
	delegatedHref,
} from "../features/delegated-operations/nav-policy";
import type { ApiEnvelope } from "../lib/api/client";
import type { RepoFetcher } from "../lib/api/delegated";
import {
	createDelegatedPostRepo,
	createDelegatedProductRepo,
	createDelegatedStorefrontRepo,
	DelegatedRepoNotWiredError,
	mapDelegatedProductItem,
} from "../lib/seller-data/delegated-repos";

/**
 * 代行工作區三個面的資料層（56 號 §6.1.7／§6.1.8／§6.1.9）。
 *
 * 這裡驗的是**線上真的送出去了什麼**，不是「函式有沒有被呼叫」：代行的失敗形狀幾乎
 * 都是「請求完全合法、回 code: 0，而賣家的資料被改壞」——
 * - 少送 `operationReason` ⇒ 9260，表單完全不能用
 * - PATCH 把沒改過的欄位一起送 ⇒ 靜默覆寫賣家自己填的內容
 * - 上下架送 `status` 值而不是打命令端點 ⇒ 端點根本不存在
 * 所以每一條斷言都盯著 **path ＋ body 的實際內容**。
 */

const STORE_ID = "2685914682";
const META = { operationReason: "協助完成新品上架", notifySeller: true };

interface Sent {
	path: string;
	method: string;
	json?: Record<string, unknown>;
	idempotencyKey?: string;
}

function recorder(data: unknown = {}) {
	const sent: Sent[] = [];
	const fetcher = (async <T>(
		path: string,
		opts: {
			method?: string;
			json?: unknown;
			idempotencyKey?: string;
		} = {},
	) => {
		sent.push({
			path,
			method: opts.method ?? "GET",
			json: opts.json as Record<string, unknown> | undefined,
			idempotencyKey: opts.idempotencyKey,
		});
		return { code: 0, data: data as T } satisfies ApiEnvelope<T>;
	}) as RepoFetcher;
	return { sent, fetcher };
}

test("店面：請求照樣送出，503 原樣交給呼叫端，不轉換任何資料", async () => {
	const unavailable = new Error("503");
	const sent: Sent[] = [];
	const fetcher = (async (
		path: string,
		opts: { method?: string; json?: unknown; idempotencyKey?: string } = {},
	) => {
		sent.push({
			path,
			method: opts.method ?? "GET",
			json: opts.json as Record<string, unknown> | undefined,
			idempotencyKey: opts.idempotencyKey,
		});
		throw unavailable;
	}) as RepoFetcher;
	const repo = createDelegatedStorefrontRepo(fetcher, STORE_ID);

	await assert.rejects(() => repo.fetchMyStore(), unavailable);
	await assert.rejects(
		() => repo.updateMyStore({ etag: "1", storeName: "梨山盛花園" }, META),
		unavailable,
	);
	await assert.rejects(
		() => repo.updateStoreLogo("1", "stg_logo_1", META),
		unavailable,
	);
	await assert.rejects(
		() => repo.updateStoreCover("1", "stg_cover_1", META),
		unavailable,
	);

	assert.deepEqual(
		sent.map((s) => `${s.method} ${s.path}`),
		[
			`GET /storefront/${STORE_ID}`,
			`PATCH /storefront/${STORE_ID}`,
			`POST /storefront/${STORE_ID}/logo`,
			`POST /storefront/${STORE_ID}/cover`,
		],
	);
	// 沒有欄位轉換：PATCH 只有共通欄位，店名之類的輸入不會被轉成代行 DTO 送出。
	assert.deepEqual(sent[1].json, {
		operationReason: "協助完成新品上架",
		notifySeller: true,
	});
	assert.equal(sent[2].json?.stagedToken, "stg_logo_1");
});

test("店面：萬一回應成功也不讀它（還沒有契約的形狀不猜）", async () => {
	const { fetcher } = recorder({ storeId: STORE_ID, name: "梨山盛花園" });
	const repo = createDelegatedStorefrontRepo(fetcher, STORE_ID);

	await assert.rejects(() => repo.fetchMyStore(), DelegatedRepoNotWiredError);
	await assert.rejects(
		() => repo.updateMyStore({ etag: "1", prize: "x" }, META),
		DelegatedRepoNotWiredError,
	);
});

test("寫入缺 operationReason：當場 throw，一個請求都不送", async () => {
	const { sent, fetcher } = recorder();
	const storefront = createDelegatedStorefrontRepo(fetcher, STORE_ID);
	const products = createDelegatedProductRepo(fetcher, STORE_ID);
	const posts = createDelegatedPostRepo(fetcher, STORE_ID);

	await assert.rejects(() =>
		storefront.updateMyStore({ etag: "1", prize: "x" }),
	);
	await assert.rejects(() => products.updateProductStatus("1", "4004"));
	await assert.rejects(() => posts.createSellerPost({ content: "x" }));
	await assert.rejects(() =>
		storefront.updateMyStore(
			{ etag: "1", prize: "x" },
			{ operationReason: "   " },
		),
	);

	assert.equal(sent.length, 0);
});

test("商品：上下架打的是兩支命令端點，body 只有共通三欄", async () => {
	const { sent, fetcher } = recorder();
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	await repo.updateProductStatus("905", "4004", {
		...META,
		expectedVersion: "12",
	});
	await repo.updateProductStatus("905", "4003", {
		...META,
		expectedVersion: "13",
	});

	assert.deepEqual(
		sent.map((call) => `${call.method} ${call.path}`),
		[
			`POST /products/${STORE_ID}/905/publish`,
			`POST /products/${STORE_ID}/905/unpublish`,
		],
	);
	// `status` 是 PATCH 明令拒收的欄位；命令端點也只收共通三欄。
	assert.deepEqual(sent[0].json, {
		expectedVersion: "12",
		operationReason: META.operationReason,
		notifySeller: true,
	});
});

test("商品：刪除與編輯在代行沒有出口，會 reject 而不是靜默打賣家端點", async () => {
	const { sent, fetcher } = recorder();
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	await assert.rejects(() => repo.deleteProduct("905"));
	await assert.rejects(() => repo.fetchSellerProductEdit("905"));
	await assert.rejects(() => repo.updateProduct("905", {}));
	assert.equal(sent.length, 0);
});

test("商品：建立打 POST .../products，body 是代行契約的形狀", async () => {
	// 建立回應與其他寫入同形（`{auditLogId, version, resource}`）。mock 成頂層
	// `{productId}` 會替「讀不到 id ⇒ 接續的 /publish 被靜默跳過」背書 —— 貼文那一面
	// 真的這樣壞過。
	const { sent, fetcher } = recorder({
		auditLogId: "8100",
		version: "0",
		resource: { productId: "9100", version: "0" },
	});
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	const created = await repo.createProduct(
		{
			name: "梨山高山高麗菜",
			categoryId: "527",
			origin: "台灣 梨山",
			description: {
				version: "1.0",
				blocks: [
					{
						type: "image",
						imageKey: "product/a.jpg",
						// 後端由 imageKey 組出來的顯示值。回送它 ⇒ DisallowUnknownFields ⇒ 9260。
						imageUrl: "https://cdn.example/i/original/product/a.jpg",
					},
				],
			},
			images: [{ stagedToken: "stg_1" }],
			skus: [
				{
					skuName: "3 台斤裝",
					price: "300",
					stockQty: "8",
					unit: "3302",
					weight: "1.8",
					lengthCm: "30",
					widthCm: "20",
					heightCm: "15",
				},
			],
		},
		META,
	);

	assert.equal(sent[0].method, "POST");
	assert.equal(sent[0].path, `/products/${STORE_ID}`);
	assert.ok(sent[0].idempotencyKey);
	assert.deepEqual(sent[0].json, {
		operationReason: META.operationReason,
		notifySeller: true,
		name: "梨山高山高麗菜",
		categoryId: "527",
		description: {
			version: "1.0",
			blocks: [{ type: "image", imageKey: "product/a.jpg" }],
		},
		images: [{ stagedToken: "stg_1" }],
		skus: [
			{
				// `skuName` ↔ `spec`：整條鏈上最容易接錯的一欄。
				spec: "3 台斤裝",
				price: "300",
				salePrice: "",
				stockQty: "8",
				unit: "3302",
				weight: "1.8",
				lengthCm: "30",
				widthCm: "20",
				heightCm: "15",
			},
		],
		origin: "台灣 梨山",
	});
	// 建立沒有版本可帶；帶了就是 9260（create 的 DTO 根本沒有這個欄位）。
	assert.ok(!("expectedVersion" in (sent[0].json ?? {})));
	// 賣家端 body 專有的三欄一個都不能漏出去。
	for (const forbidden of ["status", "updateHashtags", "idempotencyKey"]) {
		assert.ok(!(forbidden in (sent[0].json ?? {})), forbidden);
	}
	// 接著上架的 `expectedVersion` 只有這兩個值拿得到。
	assert.equal(created.id, "9100");
	assert.equal(created.version, "0");
});

test("商品：建立時帶了 expectedVersion 也不會漏進 body", async () => {
	const { sent, fetcher } = recorder({ resource: { productId: "1" } });
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	await repo.createProduct(
		{ name: "x", categoryId: "527", skus: [] },
		{ ...META, expectedVersion: "12" },
	);

	assert.ok(!("expectedVersion" in (sent[0].json ?? {})));
});

test("商品列表：DTO 的字串數字換算成列表元件吃的形狀，version 一起帶出來", async () => {
	const { sent, fetcher } = recorder({
		records: [
			{
				productId: "9001",
				version: "12",
				name: "梨山高山高麗菜",
				status: "4004",
				priceMin: "120",
				priceMax: "120",
				stockQty: "18",
				salesCount: "42",
				imageKey: "product/a.jpg",
				categoryName: "蔬菜",
			},
		],
		total: "12",
		page: "1",
		pageSize: "20",
	});
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	const result = await repo.fetchSellerProducts({ page: 2, pageSize: 10 });

	assert.equal(sent[0].path, `/products/${STORE_ID}?page=2&page_size=10`);
	assert.equal(result.total, 12);
	assert.deepEqual(mapDelegatedProductItem({ productId: "1" }).id, "1");
	assert.equal(result.records[0].id, "9001");
	assert.equal(result.records[0].version, "12");
	assert.equal(result.records[0].stock, 18);
	assert.equal(result.records[0].main_image, "product/a.jpg");
});

test("商品批次下架：整批一個旗標、逐項 expectedVersion、上限 50", async () => {
	const { sent, fetcher } = recorder({
		requested: "2",
		succeeded: "1",
		failed: "1",
		results: [
			{ productId: "1", ok: true, code: 0, version: "13", auditLogId: "a1" },
			{ productId: "2", ok: false, code: 9266, version: "", auditLogId: "" },
		],
	});
	const repo = createDelegatedProductRepo(fetcher, STORE_ID);

	const result = await repo.batchUnpublish(
		[
			{ productId: "1", expectedVersion: "12" },
			{ productId: "2", expectedVersion: "7" },
		],
		META,
	);

	assert.equal(sent[0].path, `/products/${STORE_ID}/batch-unpublish`);
	assert.deepEqual(sent[0].json, {
		operationReason: META.operationReason,
		notifySeller: true,
		items: [
			{ productId: "1", expectedVersion: "12" },
			{ productId: "2", expectedVersion: "7" },
		],
	});
	// 部分失敗時頂層 code 仍是 0：成敗只能看 failed，逐項結果要留得住。
	assert.equal(result.failed, 1);
	assert.equal(result.results[1].code, 9266);

	await assert.rejects(() =>
		repo.batchUnpublish(
			Array.from({ length: 51 }, (_, i) => ({
				productId: String(i),
				expectedVersion: "1",
			})),
			META,
		),
	);
});

test("貼文：建立一律是未發布，發布是第二支端點", async () => {
	// ⚠️ 回應形狀必須與 gateway 的 `respondDelegatedMutation` 一致
	// （`{auditLogId, version, resource}`，`admin_delegated_common.go:214`）。
	// 這裡曾經 mock 成頂層 `{postId}`，於是「repo 讀不到 postId ⇒ 第二支 /publish
	// 被靜默跳過」這個真實缺陷一路測得過 —— 假形狀的 mock 會替錯誤背書。
	const { sent, fetcher } = recorder({
		auditLogId: "8001",
		version: "0",
		resource: { postId: "7001", version: "0" },
	});
	const repo = createDelegatedPostRepo(fetcher, STORE_ID);

	const created = await repo.createSellerPost(
		{
			content: "秋意漸濃",
			stagedToken: "stg_1",
			productId: "9001",
			tags: ["梨山"],
		},
		META,
	);
	// `/publish` 必填 `expectedVersion`，值只有建立那一次的回應拿得到。
	assert.equal(created.version, "0");
	await repo.publish(
		String(created.id),
		{ ...META, expectedVersion: created.version },
		{ publishMode: "immediate" },
	);

	assert.equal(sent[0].path, `/posts/${STORE_ID}`);
	assert.deepEqual(sent[0].json, {
		operationReason: META.operationReason,
		notifySeller: true,
		content: "秋意漸濃",
		stagedTokens: ["stg_1"],
		tags: ["梨山"],
		productIds: ["9001"],
	});
	// 建立 body 不得出現 publishMode／scheduledPublishAt（出現即 9260）。
	assert.ok(!("publishMode" in (sent[0].json ?? {})));
	assert.equal(sent[1].path, `/posts/${STORE_ID}/7001/publish`);
	assert.equal(sent[1].json?.publishMode, "immediate");
	assert.equal(sent[1].json?.expectedVersion, "0");
	// 兩支呼叫 ＝ 兩筆稽核列，冪等鍵必須各自一把。
	assert.notEqual(sent[0].idempotencyKey, sent[1].idempotencyKey);
});

test("貼文：刪除在代行沒有端點（能做的是下架）", async () => {
	const { sent, fetcher } = recorder();
	const repo = createDelegatedPostRepo(fetcher, STORE_ID);
	await assert.rejects(() => repo.deleteSellerPost("7001"));
	assert.equal(sent.length, 0);
});

/**
 * 逐面路由是薄殼：內容一律 import 既有的賣家中心元件。
 *
 * 這條守的是 A28 的核心（工作區自動跟上賣家中心的演進）：一旦有人在代行路由裡另外
 * 抄一份畫面，兩邊就會各自演化，而「代行看到的與賣家看到的是同一個畫面」這個前提
 * 沒有任何東西會告訴你它已經不成立了。
 */
test("逐面路由是薄殼，內容 import 既有賣家中心元件", () => {
	const faces: [string, string][] = [
		["storefront", "features/seller-center/profile/SellerProfileClient"],
		["products", "features/seller-center/products"],
		["posts", "features/seller-center/posts/SellerPostsClient"],
		["posts/new", "features/seller-center/posts/PostComposeClient"],
	];
	for (const [face, source] of faces) {
		const file = readFileSync(
			`app/(delegated)/adm/[storeId]/${face}/page.tsx`,
			"utf8",
		);
		assert.match(file, new RegExp(source.replace(/\//g, "\\/")), face);
		// 薄殼不自己取資料：任何 fetch／repo import 都代表畫面開始分岔。
		assert.doesNotMatch(
			file,
			/useQuery|fetch\(|authedFetch|delegatedFetch/u,
			face,
		);
	}
});

/**
 * 代行工作區不得沿用登入身分的快取鍵（§4.5 定案第 4 條）。
 *
 * 沿用的後果是靜默的：代行的瀏覽器在 www 上沒有登入態，`accountId`／`activeRole`
 * 恆為 `null` ⇒ 先後代行 A 店與 B 店共用同一個快取槽，第二家直接看到第一家的資料。
 */
test("代行工作區的 source 不出現 useQueryIdentity／queryKeys.seller／queryKeys.buyer", () => {
	const files = [
		"features/delegated-operations/state/DelegatedSellerDataProvider.tsx",
		"lib/seller-data/delegated-repos.ts",
		"app/(delegated)/adm/[storeId]/storefront/page.tsx",
		"app/(delegated)/adm/[storeId]/products/page.tsx",
		"app/(delegated)/adm/[storeId]/posts/page.tsx",
		"app/(delegated)/adm/[storeId]/posts/new/page.tsx",
	];
	for (const file of files) {
		const source = readFileSync(file, "utf8");
		assert.doesNotMatch(source, /useQueryIdentity/u, file);
		assert.doesNotMatch(source, /queryKeys\.seller\./u, file);
		assert.doesNotMatch(source, /queryKeys\.buyer\./u, file);
	}
});

/**
 * 「可操作」與「有具名路由」必須是同一個集合（§6.1.14 第 1 列）。
 *
 * 分岔的兩個方向都是靜默的：
 * - `nav-policy` 說可操作、卻沒有具名路由 ⇒ 請求落到同層的 `[...area]` catch-all，
 *   側欄那一項是 active、主區卻寫著「不在本次代行授權範圍」（總覽曾經如此，而
 *   `[...area]` 的註解列舉未開放模組時根本沒提到它）
 * - 有具名路由、`nav-policy` 卻沒宣告 ⇒ 一個側欄到不了、也沒有被任何一份表承認的入口
 */
test("nav-policy 的可操作段 ↔ 工作區的具名路由目錄，雙向對得起來", () => {
	const workspaceRoot = "app/(delegated)/adm/[storeId]";
	const prefix = "/adm/1/";
	const operableSegments = Object.keys(DELEGATED_AREA_CAPABILITY).map((href) =>
		delegatedHref(href, "1").slice(prefix.length),
	);
	assert.ok(operableSegments.length > 0, "可操作段不應為空");

	for (const segment of operableSegments) {
		assert.ok(
			existsSync(`${workspaceRoot}/${segment}/page.tsx`),
			`${segment}：nav-policy 宣告可操作，卻沒有具名路由 ⇒ 會落到空狀態頁`,
		);
	}

	const named = readdirSync(workspaceRoot, { withFileTypes: true })
		.filter((entry) => entry.isDirectory() && entry.name !== "[...area]")
		.map((entry) => entry.name);
	for (const segment of named) {
		assert.ok(
			operableSegments.includes(segment),
			`${segment}：有具名路由，但 nav-policy 沒有宣告它可操作`,
		);
	}
});

/**
 * 建立商品的入口與它背後的三樣東西必須同時存在（否則是一顆死按鈕）。
 *
 * 上一輪這顆 CTA 被整顆移除，理由是「按下去沒有東西接」——移除入口等於讓一個已實作、
 * 已有 capability、已進 boot 斷言 registry 的後端能力永遠用不到。正解是接線，而
 * 「接線」是四件事同時成立：資料層真的送出去、具名路由存在、兩個落點用同一個
 * capability。任一件退回去，這條就轉紅。
 */
test("建立商品：兩個入口、具名路由、資料層，四者對得起來", () => {
	const topNav = readFileSync(
		"features/delegated-operations/components/AdmTopNav.tsx",
		"utf8",
	);
	const list = readFileSync(
		"features/seller-center/products/ProductsPage.tsx",
		"utf8",
	);

	// ① 頂部導覽的 CTA：存在、指向具名路由、由 products.write 決定渲染。
	assert.match(topNav, /＋ 上架新商品/u);
	assert.match(topNav, /products\/new/u);
	assert.match(topNav, /hasCapability\("products\.write"\)/u);

	// ② 列表的同一顆按鈕：同一個 capability，不是 disabled 而是不渲染。
	assert.match(list, /＋ 上架新商品/u);
	assert.match(list, /hasCapability\("products\.write"\)/u);
	assert.match(list, /delegated && canWrite/u);

	// ③ 具名路由存在（少了它會落進 `[...area]` 的「不在本次代行授權範圍」）。
	assert.ok(
		existsSync("app/(delegated)/adm/[storeId]/products/new/page.tsx"),
		"缺具名路由 ⇒ CTA 會落到空狀態頁",
	);

	// ④ 資料層不再 throw NotWired（那正是 ② 當初不渲染的依據）。
	const repos = readFileSync("lib/seller-data/delegated-repos.ts", "utf8");
	assert.doesNotMatch(
		repos,
		/createProduct\(\)\s*\{\s*return Promise\.reject/u,
		"createProduct 又變回 throw ⇒ 入口會變成死按鈕",
	);
});

/**
 * 版本衝突（9266）的 UI 契約：**兩個出口**，兩條寫入路徑都要有（L5346／mockup §3.4）。
 *
 * 只給「重新載入最新版本」＝ 操作者剛打的內容白打；只給「複製我的內容」＝ 覆寫賣家
 * 較新的修改。而單筆上下架原本連分支都沒有，只吐一行「代行請求失敗（代碼 9266）」，
 * 那句話唯一能引導出的動作是再按一次 —— 再按一次仍然是 9266。
 */
test("版本衝突 9266：編輯頁與單筆上下架都提供兩個出口", () => {
	const edit = readFileSync(
		"features/delegated-operations/products/DelegatedProductEditPage.tsx",
		"utf8",
	);
	const list = readFileSync(
		"features/seller-center/products/ProductsPage.tsx",
		"utf8",
	);

	for (const [name, source] of [
		["編輯頁", edit],
		["商品列表", list],
	] as const) {
		assert.match(source, /重新載入最新版本/u, name);
		assert.match(source, /複製我的內容/u, name);
	}

	// 列表要真的判得出 9266（與一般錯誤分開呈現），不是把它併進 setError。
	assert.match(list, /e\.code === 9266/u);
	assert.match(list, /setStatusConflict/u);
});

/**
 * `/adm/{storeId}` 的導向必須留在 HTTP 層。
 *
 * 它原本是一支 server-component `redirect()` 別名頁。root layout 會 streaming，
 * 等 page 呼叫 redirect() 時回應已經開始送，Next 於是把它降級成 client-side 導向
 * ——實測（2026-08-20，本機 next dev）`GET /adm/2685914682` 回 200 ＋ 69 KB HTML
 * 而且**沒有 Location**，導向藏在 RSC payload 裡。訪客要下載兩份文件，HTTP client
 * 與爬蟲則拿到一份內容不對的 200。next.config.ts 的別名群組（/seasonal、
 * /account/find-sellers…）早就為同一個形狀定了規矩，這一支當時沒跟上。
 *
 * 這條守的是「不要搬回去」：把 page.tsx 加回來不會讓任何測試轉紅，症狀也只在
 * 真正發 HTTP 請求時才看得見（瀏覽器點連結一路走 client 導航，完全正常）。
 * 實際的 307 由 tests/e2e/site-links.spec.ts 的 delegated 區塊斷言。
 */
test("/adm/{storeId} 的導向在 next.config，不在 server component", () => {
	assert.ok(
		!existsSync("app/(delegated)/adm/[storeId]/page.tsx"),
		"別名頁回來了 ⇒ 導向又會降級成 200 包 RSC payload",
	);
	const config = readFileSync("next.config.ts", "utf8");
	// `(\d+)` 是必要的：redirects() 在 filesystem routes 之前求值，裸 `:storeId`
	// 會把 /adm/enter（交接碼兌換，代行唯一入口）與 /adm/unavailable 一起吃掉。
	assert.match(
		config,
		/source:\s*"\/adm\/:storeId\(\\\\d\+\)",\s*destination:\s*"\/adm\/:storeId\/products",\s*permanent:\s*false/u,
	);
});
