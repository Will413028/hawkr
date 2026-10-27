import assert from "node:assert/strict";
import { mock, test } from "node:test";
import { delegatedPreviewProduct } from "../features/delegated-operations/products/preview-props";
import type { DelegatedProductDetail } from "../features/delegated-operations/products/repo";
import { deriveDetailLayoutProps } from "../features/product/deriveDetailLayoutProps";

/**
 * 代行商品預覽的欄名契約（56 號 §2.2 out-of-scope 第 13 列）。
 *
 * 預覽以**代行 GET 的 DTO** 自繪買家視角，而代行 DTO 與買家 PDP 的欄名在三處不同。
 * 這三處的共同特徵是 **接錯不會報錯**：`deriveDetailLayoutProps` 對讀不到的欄位一律
 * 給空值或 fallback，畫面照樣渲染 —— 圖庫空一片、規格叫「規格 1」。所以每一處各釘
 * 一條，而且**斷言的是 derive 之後的結果**（畫面真正吃到的東西），不是 mapper 的中間
 * 產物：只驗中間產物的話，derive 哪天改讀別的名字，這裡照樣是綠的。
 */

const DETAIL: DelegatedProductDetail = {
	productId: "7495464541774413824",
	version: "19",
	name: "梨山高冷茶",
	status: "4001",
	statusLabel: "草稿",
	purchasable: false,
	publicationState: "draft",
	categoryId: "301",
	categoryName: "茶葉",
	description: { version: "1.0", blocks: [] },
	images: [
		{ assetId: "a1", imageKey: "public/product/one.jpg" },
		{ assetId: "a2", imageKey: "public/product/two.jpg" },
	],
	origin: "台中市和平區",
	certifications: [{ name: "3701", certNo: "ORG-2024-001" }],
	hashtags: ["高山茶"],
	skus: [
		{
			skuId: "9001",
			spec: "四兩裝",
			price: "600",
			salePrice: "",
			stockQty: "12",
			unit: "3301",
			weight: "150",
			lengthCm: "10",
			widthCm: "10",
			heightCm: "10",
			shippingMethods: [
				{
					type: "3001",
					temperature: "3201",
					freeThreshold: "1200",
					feePayer: "3101",
				},
			],
		},
		{
			skuId: "9002",
			spec: "一斤裝",
			price: "2200",
			salePrice: "1980",
			stockQty: "3",
			unit: "3301",
			weight: "600",
			lengthCm: "20",
			widthCm: "20",
			heightCm: "20",
			shippingMethods: [
				{
					type: "3002",
					temperature: "3202",
					freeThreshold: "1500",
					feePayer: "3101",
				},
				// 與上一筆逐欄相同 → 聚合時去重（公開端的商品層 shippingMethods 是聯集）。
				{
					type: "3001",
					temperature: "3201",
					freeThreshold: "1200",
					feePayer: "3101",
				},
			],
		},
	],
	priceMin: "600",
	priceMax: "2200",
	stockQty: "15",
	salesCount: "0",
};

function derive(detail: DelegatedProductDetail = DETAIL) {
	return deriveDetailLayoutProps({
		product: delegatedPreviewProduct(detail, "2685914682"),
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
}

/* ── 三處改名 ────────────────────────────────────────────────────────── */

test("預覽映射：images[].imageKey → imageKeys（漏掉＝圖庫整片空白）", () => {
	assert.deepEqual(derive().thumbnails, [
		"public/product/one.jpg",
		"public/product/two.jpg",
	]);
});

test("預覽映射：skus[].skuId → id（漏掉＝規格鍵退成索引字串）", () => {
	assert.deepEqual(
		derive().skus.map((sku) => sku.id),
		["9001", "9002"],
	);
	// 反向：derive 對缺 id 的 SKU 會填索引 "0"/"1"，而那正是「看起來有值」的失敗形狀。
	const unmapped = deriveDetailLayoutProps({
		product: { skus: DETAIL.skus },
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(
		unmapped.skus.map((sku) => sku.id),
		["0", "1"],
		"derive 的 id fallback 已改變，這條反向斷言失去鑑別力",
	);
});

test("預覽映射：skus[].spec → skuName（漏掉＝規格名變成「規格 N」）", () => {
	assert.deepEqual(
		derive().skus.map((sku) => sku.name),
		["四兩裝", "一斤裝"],
	);
	const unmapped = deriveDetailLayoutProps({
		product: { skus: DETAIL.skus },
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(
		unmapped.skus.map((sku) => sku.name),
		["規格 1", "規格 2"],
		"derive 的規格名 fallback 已改變，這條反向斷言失去鑑別力",
	);
});

/* ── 兩處由 SKU 聚合（公開端是後端算好才下發的） ──────────────────────── */

test("預覽聚合：deliveryTemp 取所有 SKU 的溫層聯集，常溫→冷藏→冷凍", () => {
	assert.deepEqual(derive().supportedTempCodes, ["3201", "3202"]);
	assert.deepEqual(derive().supportedTemps, ["常溫", "冷藏"]);
});

test("預覽聚合：商品層 shippingMethods 取聯集並去重，配送業者 icon 因此畫得出來", () => {
	assert.deepEqual(
		derive().enabledCarriers.map((carrier) => carrier.method),
		["3001", "3002"],
	);
	const product = delegatedPreviewProduct(DETAIL, "2685914682");
	assert.equal(
		(product.shippingMethods as unknown[]).length,
		2,
		"逐欄相同的兩筆沒有去重",
	);
});

/* ── 三處刻意留空 ───────────────────────────────────────────────────── */

test("預覽不借用評價：rating／reviewCount／reviews 全部缺席 → 尚無評價", () => {
	const derived = derive();
	const product = delegatedPreviewProduct(DETAIL, "2685914682");
	assert.equal(product.rating, undefined);
	assert.equal(product.reviewCount, undefined);
	assert.equal(product.reviews, undefined);
	assert.deepEqual(derived.reviews, []);
	assert.equal(derived.reviewTotal, 0);
});

/* ── 認證：props 帶碼值，解析交給共用 layout ────────────────────────── */

test("預覽的認證原樣帶碼值，不在 props 層套 certLabel（與公開 PDP 走同一條路）", () => {
	// 後端存的就是 CertificationType 碼值（gateway `product.go` 的 CertificationItem
	// 原樣轉發），碼值→顯示名由 `ProductDetailLayout` 解析，而預覽與公開 PDP 共用那支
	// layout。在 props 層先解析等於讓預覽走一條公開頁不會走的路，兩邊就可能分岔。
	// ⚠️ 斷言的是**碼值**：改成中文名的那一刻，預覽就不再是「買家會看到什麼」。
	assert.deepEqual(derive().certifications, [
		{ name: "3701", certNo: "ORG-2024-001" },
	]);
});

/* ── 賣家卡的鍵 ─────────────────────────────────────────────────────── */

test("預覽注入 storeId，否則整張賣家卡不畫", () => {
	assert.equal(derive().sellerStoreId, "2685914682");
	const withoutStore = deriveDetailLayoutProps({
		product: delegatedPreviewProduct(DETAIL, ""),
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.equal(withoutStore.sellerStoreId, "");
});

/* ── 真的渲染一次 ───────────────────────────────────────────────────── */

/**
 * 上面每一條驗的都是**送進 layout 的 props**。這一條驗的是 layout 真的把它畫出來、
 * 而且畫出來的東西**沒有任何一條通往買家公開面的連結**。
 *
 * 為什麼靜態 gate 之外還需要它：`tests/delegated-public-surface-exit.test.ts` 掃的是
 * 源碼字面值，而 `ProductDetailLayout` 的公開連結（麵包屑、購買盒的商品連結、賣家卡）
 * 是**由 props 決定**的 —— `previewMode` 沒有守住哪一條，源碼掃描完全看不出來。
 */
test("預覽真的渲染得出來，且畫面上沒有通往買家公開面的連結", async () => {
	const React = (await import("react")).default;
	// `next/image` 的 default export 在裸 node runner 下是一個 object 而不是元件
	// （Next 的 client-component 包裝），renderToStaticMarkup 會直接 throw。換成
	// 一個只吐 <img> 的替身 —— 這條測試要驗的是連結，不是圖片元件本身。
	mock.module("next/image", {
		defaultExport: ({ alt, src }: { alt?: string; src?: string }) =>
			React.createElement("img", { alt: alt ?? "", src: String(src ?? "") }),
	});
	// 購買盒讀登入態。代行的瀏覽器在 www 上**沒有**登入態，所以 false 是這條路徑上
	// 的真實值 —— 這不是為了讓測試過而調的參數。
	const anonymousAuth = { isLoggedIn: false, user: null, loading: false };
	mock.module("@/lib/auth/context", {
		namedExports: {
			AuthProvider: ({ children }: { children: React.ReactNode }) => children,
			useAuth: () => anonymousAuth,
			useOptionalAuth: () => anonymousAuth,
		},
	});
	const { renderToStaticMarkup } = await import("react-dom/server");
	const { withAppRouter } = await import("./mocks/app-router");
	const { QueryClient, QueryClientProvider } = await import(
		"@tanstack/react-query"
	);
	const ProductDetailLayout = (
		await import("../features/product/ProductDetailLayout")
	).default;

	// 非預覽模式（③ 的反向自檢）會渲染 `ProductSellerIntro` → `SellerChatButton`，
	// 那一支用 react-query。SSR 不跑 effect，所以這個 client 不會發任何請求。
	const withProviders = (node: React.ReactElement) =>
		withAppRouter(
			React.createElement(
				QueryClientProvider,
				{ client: new QueryClient() },
				node,
			),
		);

	const derived = derive();
	const html = renderToStaticMarkup(
		withProviders(
			React.createElement(ProductDetailLayout, {
				...derived,
				previewMode: true,
				productId: DETAIL.productId,
				// 預覽頁自繪的賣家卡（不可點）。傳 null 等於不畫，那會讓下面的
				// 「沒有 /sellers 連結」變成空過 —— 所以這裡刻意放一個真的有內容的節點。
				sellerIntroSlot: React.createElement("aside", null, "梨山盛花園"),
			}),
		),
	);

	// ① 真的畫出草稿的內容（渲染成功、而且吃的是代行 DTO 而不是空殼）。
	assert.ok(html.includes("梨山高冷茶"), "商品名沒有渲染出來");
	assert.ok(
		html.includes("四兩裝"),
		"規格名沒有渲染出來（映射在渲染路徑上失效）",
	);
	assert.ok(html.includes("台中市和平區"), "產地沒有渲染出來");
	assert.ok(html.includes("梨山盛花園"), "自繪賣家卡沒有掛上去");

	// ⑤ 不可購買的商品必須畫出「不可購買」。
	// ⚠️ 這一條的錨**必須下在渲染結果**，不能下在 deriveDetailLayoutProps 的輸出。
	// 2026-08-19 dev 實測踩過：`purchasable:false` 確實一路傳到 ProductPurchaseBox
	// （React fiber 的 memoizedProps 實測為 false），但該檔 `:450` 的條件曾是
	// `outOfStock && !previewMode`，而預覽固定傳 previewMode ⇒ 不可購買分支**永遠
	// 不會渲染**，畫面與可購買商品逐項相同。derive 層的斷言全綠、完全擋不住。
	// previewMode 在那一格真正要擋的是會發 `/app/*` 請求的補貨通知鈕，不是整個狀態。
	assert.ok(
		html.includes("暫不可購買") || html.includes("已售完"),
		"不可購買的商品在預覽裡沒有畫出任何不可購買的字樣——代行者看不出買家買不到它",
	);
	assert.ok(
		!html.includes("補貨通知"),
		"預覽不得渲染補貨通知鈕：它會用操作者的買家身分打 /app/* 請求",
	);

	// ⑥ 反向：可購買的商品在預覽裡不得被畫成不可購買（防「一律畫成不可購買」）。
	const activeHtml = renderToStaticMarkup(
		withProviders(
			React.createElement(ProductDetailLayout, {
				...derive({
					...DETAIL,
					status: "4004",
					statusLabel: "已啟用",
					purchasable: true,
					publicationState: "4012",
					skus: DETAIL.skus.map((sku) => ({ ...sku, stockQty: "10" })),
				}),
				previewMode: true,
				productId: DETAIL.productId,
				sellerIntroSlot: React.createElement("aside", null, "梨山盛花園"),
			}),
		),
	);
	assert.ok(!activeHtml.includes("暫不可購買"), "可購買的商品被誤判成不可購買");

	// ② 畫面上一條公開面連結都沒有。`/products/`、`/market/`、`/sellers/` 分別是
	//    購買盒的商品連結、麵包屑的分類連結、賣家卡的店面連結 —— 三個不同的來源。
	for (const forbidden of [
		'href="/products/',
		'href="/market/',
		'href="/sellers/',
	]) {
		assert.ok(
			!html.includes(forbidden),
			`預覽畫面含 ${forbidden} ⇒ 代行者點下去會離開工作區（草稿還是 404）`,
		);
	}

	// ③ 反向自檢：非預覽模式下那些連結**確實會出現**，否則 ② 是空過的。
	const live = renderToStaticMarkup(
		withProviders(
			React.createElement(ProductDetailLayout, {
				...derived,
				previewMode: false,
				productId: DETAIL.productId,
			}),
		),
	);
	assert.ok(
		live.includes('href="/market/products?categories='),
		"非預覽模式也沒有麵包屑連結 ⇒ ② 的斷言沒有鑑別力",
	);
});

/**
 * 預覽存在的理由本身：「這個商品買家買不買得到」。
 *
 * ⚠️ 這一條的失敗方向是最壞的那一邊。`deriveDetailLayoutProps` 的判定是
 * `product.purchasable !== false` —— 欄位**缺席就恆為 true**，於是未上架商品的預覽
 * 會顯示成「可以購買」，而那正是這個畫面要回答的那個問題。
 *
 * 2026-08-19 dev 實測就踩到：gateway 已於 #2479 補上 `purchasable`／`publicationState`
 * 兩欄，但前端 DTO 與 mapper 都沒接 ⇒ 未啟用商品的預覽購買區塊與可買商品長得一樣，
 * 只靠按鈕 disabled 與兩句文字撐著。兩半各自正確，組合起來是不完整的。
 */
test("預覽映射：purchasable=false 必須傳到 derive 之後（漏掉＝草稿顯示成可以購買）", () => {
	assert.equal(
		delegatedPreviewProduct(DETAIL, "2685914682").purchasable,
		false,
		"mapper 必須帶出 purchasable",
	);
	assert.equal(
		derive().purchasable,
		false,
		"derive 之後仍必須是 false —— 這是畫面真正吃到的值",
	);

	// 反向：可購買的商品不得被誤判成不可買（防「寫死 false」）
	assert.equal(
		derive({
			...DETAIL,
			status: "4004",
			statusLabel: "已啟用",
			purchasable: true,
			publicationState: "published",
		}).purchasable,
		true,
	);
});

test("預覽映射：publicationState 必須傳到 derive 之後", () => {
	assert.equal(
		delegatedPreviewProduct(DETAIL, "2685914682").publicationState,
		"draft",
	);
});
