/**
 * Shared record-mapping helpers for `/market/products`.
 *
 * Used by:
 *   - `features/market/products/MarketProductsListPage.tsx` (server)
 *   - `features/market/components/MarketProductsList.tsx`   (client load-more)
 *
 * Backend B-182 (May 2026) now accepts every filter on the server —
 * `price_min`, `price_max`, `store_ids`,
 * `hashtags`, `subscribed_only` — plus all 5 sort modes. The legacy
 * client-side `postFilterRecords` + `applySort` fallbacks were removed
 * once the server contract was confirmed; this file only keeps the
 * record-shape normalisers + the sidebar→wire sort mapping.
 */

import {
	getProductPriceDisplay,
	type PriceFields,
} from "@/lib/products/pricing";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import { readCategory } from "./categoryGroups";
import type { MarketProductItem } from "./types";

export interface ProductRecord extends PriceFields {
	id?: string | number;
	name?: string;
	/** §231 flat-pair contract: `imageKey`/`imageKeys` (bare objectKeys) are the
	 *  SSOT. The `imageUrl`/`imageUrls` twins are
	 *  `mediaurl.BuildURL(base, imageKey…)` (gateway product.go:323-326), i.e.
	 *  derived from these very keys — they can never supply an image the keys
	 *  lack, so they are deliberately not declared and must not be re-added as
	 *  a fallback. */
	imageKey?: string;
	imageKeys?: string[];
	seller?: {
		name?: string;
		city?: string;
		storeId?: string | number;
		id?: string | number;
		/** Canonical storefront path segment (backend `586ac3ea7` added it to
		 *  product list + detail). Present on the wire as camelCase. */
		storeCode?: string | number;
		/** B-238 canonical seller-logo display URL. */
		logoUrl?: string;
	};
	sellerName?: string;
	seller_name?: string;
	storeId?: string | number;
	origin?: string;
	price?: string | number;
	hashtags?: string[] | Array<{ id?: string | number; name?: string }>;
	/** Leaf category (lv2 on the list wire); see `lib/market/categoryGroups`. */
	categoryId?: string | number;
	categoryName?: string;
	categoryLv2Id?: string | number;
	unit?: string;
	skuNames?: string[];
	rating?: string | number;
	reviewCount?: string | number;
	reviews_count?: string | number;
	createdAt?: string | number;
	created_at?: string | number;
	salesCount?: string | number;
	sales_count?: string | number;
	/** `handler.ProductItem.stock` — 剩餘庫存 (sum across SKUs), string on the
	 *  wire like everything else. */
	stock?: string | number;
	/** `handler.ProductItem.isFavorited` — whether the current buyer has
	 *  favourited this product. Lets the card heart render correct state with
	 *  no per-card request. */
	isFavorited?: boolean;
	/** `handler.ProductItem.favoriteCount` — 被收藏數 (how many buyers favourited
	 *  this product). Pending backend support (B-192); when absent the card
	 *  shows just the heart, no number. */
	favoriteCount?: string | number;
	favorite_count?: string | number;
	/** `handler.ProductItem.isRestockSubscribed` — whether the current buyer
	 *  has an ACTIVE restock subscription on this product (backend issue #1005
	 *  / B-203, 39 §5.5b). Lets the sold-out card bell render correct state
	 *  with no per-card request. */
	isRestockSubscribed?: boolean;
}

export interface CategoryNode {
	id?: string | number;
	name?: string;
	children?: CategoryNode[];
}

/** Backend ships hashtags either as a plain string array or as
 * `{ id, name }` objects. Normalise to a deduped string list capped at 3. */
export function normaliseHashtags(raw: ProductRecord["hashtags"]): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim();
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= 3) break;
	}
	return out;
}

export function readSellerId(product: ProductRecord): string | undefined {
	const raw =
		product?.storeId ?? product?.seller?.storeId ?? product?.seller?.id;
	if (raw === undefined || raw === null) return undefined;
	return String(raw);
}

/** The seller's canonical path segment, when the product wire carries one. */
export function readSellerStoreCode(
	product: ProductRecord,
): string | undefined {
	const raw = String(product?.seller?.storeCode ?? "").trim();
	return raw || undefined;
}

export function readDisplayOrigin(product: ProductRecord): string {
	return product?.origin || "";
}

export function readPriceMin(product: ProductRecord): number | undefined {
	const raw = product.priceMin;
	if (raw === undefined || raw === null) return undefined;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : undefined;
}

export function readPriceMax(product: ProductRecord): number | undefined {
	const raw = product.priceMax;
	if (raw === undefined || raw === null) return undefined;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : undefined;
}

function readNumeric(
	...candidates: Array<string | number | undefined | null>
): number {
	for (const v of candidates) {
		if (v === undefined || v === null || v === "") continue;
		const n = typeof v === "number" ? v : Number(v);
		if (Number.isFinite(n)) return n;
	}
	return 0;
}

export function mapProduct(
	product: ProductRecord,
	index: number,
): MarketProductItem {
	const id = String(product?.id || index);
	const priceDisplay = getProductPriceDisplay(product);
	const priceValue = readNumeric(
		product.salePriceMin && Number(product.salePriceMin) > 0
			? product.salePriceMin
			: undefined,
		product.priceMin,
		product.listPriceMin,
		product.priceMax,
		product.price,
	);

	return {
		id,
		// Honest fallbacks only: a record missing name/price renders a neutral
		// placeholder instead of leaking editorial mock data into the live list.
		name: product?.name || "未命名商品",
		// Empty when the product carries no image, so the card's <AppImage>
		// renders the design-system ImageFallback (folder-image) rather than a
		// branded stand-in.
		// §231: the bare objectKey (SSOT) so <AppImage>/resolveCdnUrl can apply
		// the right preset.
		image: product?.imageKey || product?.imageKeys?.[0] || "",
		sellerName: product?.seller?.name || product?.sellerName || "Hawkr 選品",
		sellerAvatar: product?.seller?.logoUrl?.trim() || "",
		sellerId: readSellerId(product),
		sellerStoreCode: readSellerStoreCode(product),
		// Real origin/city only — no "產地直送" placeholder. Empty string when
		// the product carries no origin, so the card hides the "｜…" segment
		// and shows just the seller name.
		origin: readDisplayOrigin(product),
		price: priceDisplay.effectiveText || "",
		originalPrice: priceDisplay.originalText,
		isOnSale: priceDisplay.isOnSale,
		hashtags: normaliseHashtags(product?.hashtags),
		...readCategory(product),
		unit: product?.skuNames?.[0] || product?.unit || "",
		rating: parseRating(product?.rating),
		reviews: parseReviewCount(product?.reviewCount),
		href: `/products/${id}`,
		priceValue,
		createdAt: readNumeric(product.createdAt),
		salesCount: readNumeric(product.salesCount),
		stock: readStock(product),
		isFavorited: Boolean(product?.isFavorited),
		favoriteCount: readFavoriteCount(product),
		isRestockSubscribed: Boolean(product?.isRestockSubscribed),
	};
}

/** Stock as a number, or `undefined` when the wire doesn't carry it — "0 stock"
 *  (完售) must stay distinguishable from "no data" (card hides the segment). */
function readStock(product: ProductRecord): number | undefined {
	const raw = product.stock;
	if (raw === undefined || raw === null || raw === "") return undefined;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : undefined;
}

/** Returns the favourite total as a number, or `undefined` when the wire
 *  doesn't carry it — so the card can tell "0 favourites" apart from "no data"
 *  and hide the number entirely until the backend ships favoriteCount (B-192). */
function readFavoriteCount(product: ProductRecord): number | undefined {
	const raw = product.favoriteCount ?? product.favorite_count;
	if (raw === undefined || raw === null || raw === "") return undefined;
	const n = typeof raw === "number" ? raw : Number(raw);
	return Number.isFinite(n) ? n : undefined;
}

export function findCategoryId(
	categories: CategoryNode[],
	value: string,
): string | undefined {
	for (const category of categories) {
		if (
			category?.id != null &&
			(String(category.id) === value || category?.name === value)
		)
			return String(category.id);
		const children = Array.isArray(category?.children) ? category.children : [];
		const childMatch = findCategoryId(children, value);
		if (childMatch) return childMatch;
	}
	return undefined;
}

/** Resolve a category token from either the current stable-id URL schema or
 * the legacy name schema used by older inbound links. New UI writes ids so
 * duplicate labels such as "水耕" keep pointing at the exact branch selected. */
export function findCategoryNode(
	categories: CategoryNode[],
	value: string,
): CategoryNode | undefined {
	for (const category of categories) {
		if (
			(category?.id != null && String(category.id) === value) ||
			category?.name === value
		) {
			return category;
		}
		const children = Array.isArray(category?.children) ? category.children : [];
		const childMatch = findCategoryNode(children, value);
		if (childMatch) return childMatch;
	}
	return undefined;
}

/** Canonicalise category URL values to ids while retaining unknown legacy
 * values. Unknown values stay visible/removable instead of silently vanishing. */
export function normaliseCategorySelections(
	categories: CategoryNode[],
	values: string[],
): string[] {
	const out: string[] = [];
	const seen = new Set<string>();
	for (const raw of values) {
		const value = raw.trim();
		if (!value) continue;
		const node = findCategoryNode(categories, value);
		const token = node?.id != null ? String(node.id) : value;
		if (seen.has(token)) continue;
		seen.add(token);
		out.push(token);
	}
	return out;
}

export function mapSort(sort: string) {
	switch (sort) {
		case "最新上架":
			return "LATEST";
		case "銷量最高":
			return "SALES";
		case "價格低到高":
			return "PRICE_ASC";
		case "價格高到低":
			return "PRICE_DESC";
		case "評分最高":
			return "RATING";
		default:
			return undefined;
	}
}

/** Params snapshot passed from server → client so the client can request
 * subsequent pages with the same filter state as the initial render. */
export interface MarketFetchParams {
	keyword: string;
	categoryId?: string;
	sort?: string;
	priceMin?: number;
	priceMax?: number;
	hashtags?: string[];
	/** B-183: viewer-aware filter — backend reads buyer JWT to filter
	 * by either the buyer's favourited products (followedProductOnly)
	 * or the buyer's followed sellers' entire catalogue
	 * (followedSellerOnly). Either flag forces the client through the
	 * BFF proxy so the HTTP-only Bearer token is injected. */
	followedProductOnly?: boolean;
	followedSellerOnly?: boolean;
}
