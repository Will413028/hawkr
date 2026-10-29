/**
 * Home snapshot builder — the pure transform layer shared by the server page
 * render and the client-side background refresh.
 *
 * This module is deliberately NOT `"use client"`: it holds only pure functions
 * over already-fetched anonymous data, so both `app/[locale]/(public)/page.tsx` (server,
 * ISR) and `features/home/HomePage.tsx` (client refetch) can call
 * `buildHomeSnapshot`. Fetching lives elsewhere — the server assembles the raw
 * bootstrap through `lib/api/home-bootstrap.server.ts`, the browser through
 * `fetchHomeTabBootstrap`. Keeping the transforms here (not in the client
 * HomePage module) is what lets the homepage's product/seller/post content
 * reach the server HTML for SEO.
 */

import type { HomeMerchantItem } from "@/features/home/components/HomeCompactMerchants";
import type { MarketplaceProduct } from "@/lib/api";
import type { PostListItem } from "@/lib/api/openapi-types";
import { normalizePost } from "@/lib/api/posts";
import type { HomeTabBootstrapV1 } from "@/lib/api/tab-bootstrap";
import { formatUnitLabel } from "@/lib/codemeta/enums";
import type { RecommendedStore } from "@/lib/discovery/recommended";
import {
	type CategoryGroup,
	categoryGroupMemberIds,
	findCategoryGroupByName,
	readCategory,
} from "@/lib/market/categoryGroups";
import type { MarketProductItem } from "@/lib/market/types";
import {
	getProductPriceDisplay,
	type PriceFields,
} from "@/lib/products/pricing";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import { storeLocation } from "@/lib/seller/store-location";
import { sellerPath } from "@/lib/seo/urls";

/**
 * How a discovery shelf decides which products it may show.
 *
 *   category → the product's leaf `categoryId` rolls up to this lv1 name in
 *              the bootstrap's `categoryGroups` (the seller's own filing);
 *   hashtag  → the product carries exactly this hashtag.
 *
 * Membership used to be a substring scan over name + origin + seller + unit +
 * hashtags, which filed a 文旦禮盒 under 海鮮 (the seller's name contained "鮮",
 * which was a seafood keyword) and under 加工食品 ("禮盒"). A shelf whose
 * heading says 水產 must only ever hold what the catalogue itself calls 海鮮.
 */
export type DiscoveryShelfMatch =
	| { category: string; hashtag?: never }
	| { hashtag: string; category?: never };

export interface DiscoveryShelfConfig {
	key: string;
	eyebrow: string;
	title: readonly [string, string];
	body: string;
	href: string;
	linkLabel: string;
	match: DiscoveryShelfMatch;
	offset: number;
	tone: string;
}

const discoveryShelfConfigs: readonly DiscoveryShelfConfig[] = [
	{
		key: "vegetables",
		eyebrow: "葉菜根莖 · VEGETABLES",
		title: ["今晚的青菜", "先補起來"],
		body: "葉菜、菇類、根莖和辛香料，適合每天打開冰箱就能煮。",
		href: "/market/products?categories=%E8%94%AC%E8%8F%9C",
		linkLabel: "看蔬菜",
		match: { category: "蔬菜" },
		offset: 8,
		tone: "green",
	},
	{
		key: "fruit",
		eyebrow: "當季水果 · FRUIT",
		title: ["果園直送", "熟度剛好"],
		body: "芒果、荔枝、香蕉與季節禮盒，先看這週最甜的批次。",
		href: "/market/products?categories=%E6%B0%B4%E6%9E%9C",
		linkLabel: "看水果",
		match: { category: "水果" },
		offset: 14,
		tone: "sun",
	},
	{
		key: "seafood",
		eyebrow: "冷凍海鮮 · SEAFOOD",
		title: ["水產到貨", "晚餐很好配"],
		body: "魚片、白蝦、文蛤和海味熟食，冷凍庫需要的安心庫存。",
		href: "/market/products?categories=%E6%B5%B7%E9%AE%AE",
		linkLabel: "看海鮮",
		match: { category: "海鮮" },
		offset: 20,
		tone: "blue",
	},
	{
		key: "breakfast",
		eyebrow: "早餐補給 · BREAKFAST",
		title: ["蛋奶麵包", "一週早餐不用想"],
		body: "雞蛋、鮮奶、蜂蜜與常備食材，讓早晨更快開始。",
		href: "/market/products?categories=%E8%9B%8B%E5%A5%B6",
		linkLabel: "看蛋奶",
		match: { category: "蛋奶" },
		offset: 26,
		tone: "cream",
	},
	{
		key: "meat",
		eyebrow: "安心肉品 · MEAT",
		title: ["家常肉品", "冷藏冷凍都備好"],
		body: "雞肉、豬肉、牛肉與料理包，從平日晚餐到週末聚餐都能接住。",
		href: "/market/products?categories=%E8%82%89%E9%A1%9E",
		linkLabel: "看肉品",
		match: { category: "肉類" },
		offset: 32,
		tone: "rose",
	},
	{
		key: "pantry",
		eyebrow: "職人加工 · PANTRY",
		title: ["醬料點心", "常備菜一次收"],
		body: "地方職人的醬料、果乾、熟食與常備菜，讓餐桌多一點變化。",
		href: "/market/products?categories=%E5%8A%A0%E5%B7%A5%E9%A3%9F%E5%93%81",
		linkLabel: "看加工食品",
		match: { category: "加工食品" },
		offset: 38,
		tone: "grain",
	},
	{
		key: "weekend",
		eyebrow: "週末餐桌 · WEEKEND TABLE",
		title: ["火鍋烤肉", "聚餐食材先備齊"],
		body: "適合多人共享的肉品、海鮮、蔬菜和湯底，週末不用臨時找。",
		href: "/market/products?hashtags=%E8%81%9A%E9%A4%90",
		linkLabel: "看聚餐食材",
		match: { hashtag: "聚餐" },
		offset: 44,
		tone: "forest",
	},
];

/** Backend objectKey, not a URL — every consumer runs it through
 *  `resolveCdnUrl`/`buildImageUrl`, which is the only sanctioned URL assembler
 *  (49 §6.5 / 10 檔 §25). Never hardcode a media host here: the domain is
 *  environment-specific and comes from NEXT_PUBLIC_MEDIA_BASE_URL alone. */
const FALLBACK_SELLER_COVER = "public/defaults/seller_cover.png";

export type ProductSource = MarketplaceProduct &
	PriceFields & {
		/** §231 flat-pair: `imageKey` (bare objectKey) is the SSOT. The paired
		 *  `imageUrl`/`imageUrls` are `mediaurl.BuildURL(base, imageKey)` —
		 *  derived from the very key we already read, so they can never carry an
		 *  image the keys lack. Not declared, so nobody re-adds them as a
		 *  fallback. Legacy `image`/`images` are not on the wire at all. */
		imageKey?: string;
		imageKeys?: string[];
		origin?: string;
		reviewCount?: string | number;
		/** gateway `SellerInfoItem`（product.go:546 — 五個吐 seller 物件的端點共用
		 *  的唯一映射點）。全欄 camelCase，沒有 snake 變體可退。 */
		seller?: {
			name?: string;
			/** B-238 canonical seller-logo display URL. */
			logoUrl?: string;
			storeId?: string | number;
			/** Canonical storefront path segment (#365). */
			storeCode?: string | number;
		};
		storeId?: string | number;
		unit?: string;
		isFavorited?: boolean;
		favoriteCount?: string | number;
		salesCount?: string | number;
		stock?: string | number;
		isRestockSubscribed?: boolean;
	};

function normaliseHashtags(raw: MarketplaceProduct["hashtags"]): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim().replace(/^#/, "");
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= 2) break;
	}
	return out;
}

function productSourceName(product: ProductSource): string {
	return (product.name ?? "").trim();
}

function productSourceSellerName(product: ProductSource): string {
	// `sellerName` is the flat convenience field on ProductItem; `seller.name`
	// is the nested block. Both are camelCase and both are genuinely emitted
	// (list endpoints fill the flat one, detail fills the nested one).
	return (product.sellerName || product.seller?.name || "").trim();
}

function hasRealProductIdentity(product: ProductSource): boolean {
	return (
		product.id !== undefined &&
		product.id !== null &&
		String(product.id).trim().length > 0 &&
		productSourceName(product).length > 0 &&
		productSourceSellerName(product).length > 0
	);
}

/** Same "0 ≠ no data" rule as `readStock` in lib/market/listFilters.ts — the
 *  card only shows 已售完 when the wire really said stock 0. */
function productSourceStock(product: ProductSource): number | undefined {
	const raw = product.stock;
	if (raw === undefined || raw === null || raw === "") return undefined;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : undefined;
}

function productSourceSellerId(product: ProductSource): string | undefined {
	const raw = product.storeId ?? product.seller?.storeId;
	if (raw === undefined || raw === null) return undefined;
	const id = String(raw).trim();
	return id.length > 0 ? id : undefined;
}

function productSourceSellerStoreCode(
	product: ProductSource,
): string | undefined {
	const code = String(product.seller?.storeCode ?? "").trim();
	return code.length > 0 ? code : undefined;
}

function toProduct(product: ProductSource, index: number): MarketProductItem {
	const id = String(product.id || index);
	const priceDisplay = getProductPriceDisplay(product);
	return {
		id,
		name: productSourceName(product),
		// Empty when absent → MarketProductCard's <AppImage> shows the
		// design-system ImageFallback (folder-image) rather than a branded
		// stand-in.
		image: product.imageKey || product.imageKeys?.[0] || "",
		sellerName: productSourceSellerName(product),
		sellerId: productSourceSellerId(product),
		sellerStoreCode: productSourceSellerStoreCode(product),
		// B-238: seller identity renders only the canonical display URL.
		sellerAvatar: product.seller?.logoUrl?.trim() || "",
		origin: product.origin?.trim() || "",
		price: priceDisplay.effectiveText || "價格未定",
		originalPrice: priceDisplay.originalText,
		isOnSale: priceDisplay.isOnSale,
		hashtags: normaliseHashtags(product.hashtags),
		...readCategory(product),
		unit: product.skuNames?.[0] || formatUnitLabel(product.unit) || "份",
		rating: parseRating(product.rating),
		reviews: parseReviewCount(product.reviewCount),
		href: `/products/${id}`,
		priceValue: Number(priceDisplay.effectiveText?.replace(/[^\d.]/g, "")) || 0,
		createdAt: 0,
		salesCount: Number(product.salesCount) || 0,
		stock: productSourceStock(product),
		isFavorited: Boolean(product.isFavorited),
		favoriteCount:
			product.favoriteCount == null || product.favoriteCount === ""
				? undefined
				: Number(product.favoriteCount) || 0,
		isRestockSubscribed: Boolean(product.isRestockSubscribed),
	};
}

function toSeller(store: RecommendedStore): HomeMerchantItem {
	const location = storeLocation(store.townCode);
	// 標籤（「本週推薦」等）屬呈現層，由 HomeCompactMerchants 依語系渲染；這裡只給地點，沒有就空字串。
	const place = location ? `${location.city} ${location.district}` : "";
	return {
		id: store.id,
		name: store.name,
		location: place,
		image: store.coverUrl || FALLBACK_SELLER_COVER,
		logo_url: store.logoUrl,
		rating: store.ratingAverage,
		reviewCount: store.reviewCount,
		followers: store.followerCount,
		href: sellerPath(store.storeCode),
	};
}

function getProductRecords(json: unknown): ProductSource[] {
	const data =
		typeof json === "object" && json !== null && "data" in json
			? (json as { data?: unknown }).data
			: json;
	const records =
		typeof data === "object" && data !== null && "records" in data
			? (data as { records?: unknown }).records
			: typeof data === "object" && data !== null && "items" in data
				? (data as { items?: unknown }).items
				: [];
	return Array.isArray(records) ? (records as ProductSource[]) : [];
}

function priceNumber(value: string | number | null | undefined): number {
	if (value == null) return 0;
	const n = typeof value === "number" ? value : Number(value);
	return Number.isFinite(n) ? n : 0;
}

function savingsValue(product: ProductSource): number {
	// `handler.ProductItem`（product.go:187-190）只有 camelCase。
	const list = priceNumber(product.listPriceMin);
	const effective = priceNumber(product.priceMin);
	return Math.max(0, list - effective);
}

function dedupeProducts(products: MarketProductItem[]): MarketProductItem[] {
	const seen = new Set<string>();
	const out: MarketProductItem[] = [];
	for (const product of products) {
		if (!product.id || seen.has(product.id)) continue;
		seen.add(product.id);
		out.push(product);
	}
	return out;
}

function cycleProducts(
	products: MarketProductItem[],
	offset: number,
	size = 8,
): MarketProductItem[] {
	if (!products.length) return [];
	const out: MarketProductItem[] = [];
	for (let i = 0; i < Math.min(size, products.length); i += 1) {
		out.push(products[(offset + i) % products.length]);
	}
	return out;
}

/**
 * Products a shelf may show, by the catalogue's own filing (see
 * `DiscoveryShelfMatch`). An unknown group — the tree was unsupplied, or the
 * shelf names an lv1 the tree does not have — yields nothing, so the shelf
 * hides rather than guesses.
 */
function pickShelfProducts(
	products: MarketProductItem[],
	match: DiscoveryShelfMatch,
	categoryGroups: readonly CategoryGroup[],
	offset: number,
	size = 8,
): MarketProductItem[] {
	if (!products.length) return [];
	let matches: MarketProductItem[];
	if (match.category !== undefined) {
		const group = findCategoryGroupByName(categoryGroups, match.category);
		if (!group) return [];
		const members = categoryGroupMemberIds(group);
		matches = products.filter(
			(product) =>
				product.categoryId !== undefined && members.has(product.categoryId),
		);
	} else {
		matches = products.filter((product) =>
			product.hashtags.includes(match.hashtag),
		);
	}
	return cycleProducts(matches, offset, size);
}

export type HomeDiscoveryShelf = DiscoveryShelfConfig & {
	products: MarketProductItem[];
};

/**
 * The complete, already-shaped home snapshot. Produced once on the server for
 * the initial (SEO) render and again in the browser for the background
 * price/stock reconcile — the identical shape lets react-query hand off from
 * one to the other with no structural change.
 */
export interface HomeSnapshot {
	editorRecommendedProducts: MarketProductItem[];
	newArrivalProducts: MarketProductItem[];
	dealProducts: MarketProductItem[];
	originProducts: MarketProductItem[];
	mappedSellers: HomeMerchantItem[];
	compactUpdates: PostListItem[];
	discoveryShelves: HomeDiscoveryShelf[];
}

/**
 * Pure transform: raw anonymous bootstrap → the fully-shaped home snapshot.
 *
 * No `window`/fetch: this is the exact body the browser `loadHomeSnapshot`
 * previously inlined after its fetch, lifted so the server page can call it too.
 */
export function buildHomeSnapshot(bootstrap: HomeTabBootstrapV1): HomeSnapshot {
	const {
		hotProducts,
		products: productsJson,
		recommendedSellers,
		posts,
	} = bootstrap;

	const productRecords = getProductRecords(productsJson);
	const realProductRecords = productRecords.filter(hasRealProductIdentity);
	const marketplaceHotProductRecords = (hotProducts ?? []).filter(
		hasRealProductIdentity,
	);
	const marketplaceHotProducts = marketplaceHotProductRecords.map(toProduct);
	const fetchedProducts = dedupeProducts([
		...realProductRecords.map(toProduct),
		...marketplaceHotProducts,
	]);
	const productPool = fetchedProducts;
	const editorRecommendedProducts = cycleProducts(
		dedupeProducts([...marketplaceHotProducts, ...productPool]),
		0,
		9,
	);
	const newArrivalProducts = cycleProducts(productPool, 0, 8);
	const weeklyDeals: MarketProductItem[] = realProductRecords
		.filter((product) => getProductPriceDisplay(product).isOnSale)
		.sort((a, b) => savingsValue(b) - savingsValue(a))
		.slice(0, 8)
		.map(toProduct);
	const hotDeals: MarketProductItem[] = marketplaceHotProductRecords
		.filter((product) => getProductPriceDisplay(product).isOnSale)
		.slice(0, 8)
		.map(toProduct);
	const dealProducts = cycleProducts(
		dedupeProducts([...weeklyDeals, ...hotDeals]),
		0,
		8,
	);
	const realOriginProducts = fetchedProducts.filter(
		(product) => product.origin,
	);
	const originProducts = cycleProducts(realOriginProducts, 16, 8);
	const mappedSellers: HomeMerchantItem[] = recommendedSellers
		.slice(0, 8)
		.map(toSeller);
	const feedPosts: PostListItem[] = (posts.records ?? []).map((raw) =>
		normalizePost(raw),
	);
	const compactUpdates = feedPosts.slice(0, 5);
	// A payload retained from before this field shipped has no `categoryGroups`;
	// read it as "tree unsupplied" (empty shelves), not as a crash.
	const categoryGroups = bootstrap.categoryGroups ?? [];
	const discoveryShelves: HomeDiscoveryShelf[] = discoveryShelfConfigs.map(
		(shelf) => ({
			...shelf,
			products: pickShelfProducts(
				productPool,
				shelf.match,
				categoryGroups,
				shelf.offset,
				8,
			),
		}),
	);

	return {
		editorRecommendedProducts,
		newArrivalProducts,
		dealProducts,
		originProducts,
		mappedSellers,
		compactUpdates,
		discoveryShelves,
	};
}

/**
 * De-duplicated product ids across every product-bearing home shelf, in visual
 * order. The server page maps these to canonical URLs for the ItemList
 * JSON-LD so crawlers see the same product set the HTML lists.
 */
export function homeSnapshotProductIds(snapshot: HomeSnapshot): string[] {
	const ids: string[] = [];
	const seen = new Set<string>();
	const push = (items: MarketProductItem[]) => {
		for (const product of items) {
			if (!product.id || seen.has(product.id)) continue;
			seen.add(product.id);
			ids.push(product.id);
		}
	};
	push(snapshot.editorRecommendedProducts);
	push(snapshot.newArrivalProducts);
	push(snapshot.dealProducts);
	push(snapshot.originProducts);
	for (const shelf of snapshot.discoveryShelves) push(shelf.products);
	return ids;
}
