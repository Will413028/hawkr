import { apiClient } from "./client";

export interface MarketplaceProduct {
	/**
	 * Backend issue #2110: `/public/products` carries the seller's **store
	 * surrogate** (product.store_id) under `storeId`. It is NOT the login
	 * account_id — do not look it up against accounts.
	 */
	storeId?: string;
	id: string | number;
	imageUrl?: string;
	name?: string;
	priceMax?: string | number;
	priceMin?: string | number;
	/**
	 * Issue #758: backend now returns three parallel price ranges.
	 * `priceMin/Max` is the effective (sale-aware) headline price;
	 * `listPriceMin/Max` is the original retail value (strikethrough);
	 * `salePriceMin/Max` is the sale value, "0" when no sale active.
	 * Use `lib/products/pricing.ts` helpers rather than reading these
	 * directly so the sale-vs-no-sale logic stays consistent.
	 */
	listPriceMin?: string | number;
	listPriceMax?: string | number;
	salePriceMin?: string | number;
	salePriceMax?: string | number;
	/**
	 * Issue #759: hashtag chips on cards / PDP. Response shape may be
	 * `string[]` on list endpoints and `Hashtag[]` (object with id+name)
	 * on detail; consumers should normalise via a small helper before
	 * rendering. Type kept permissive on read.
	 */
	hashtags?: string[] | Array<{ id?: number | string; name?: string }>;
	/**
	 * `handler.ProductItem.categoryId/categoryName`: the leaf the seller filed
	 * the product under (today's list wire answers lv2 in both `categoryId` and
	 * `categoryLv2Id`). No lv1 ancestor is on the list wire — see
	 * `lib/market/categoryGroups` for the roll-up.
	 */
	categoryId?: string | number;
	categoryName?: string;
	categoryLv2Id?: string | number;
	categoryLv2Name?: string;
	rating?: string | number;
	reviewCount?: string | number;
	sellerName?: string;
	seller?: {
		name?: string;
		storeId?: string | number;
		city?: string;
		district?: string;
		/** B-238 canonical seller-logo display URL. */
		logoUrl?: string;
	};
	skuNames?: string[];
}

function emptyRecords() {
	return { records: [], total: 0, totalPages: 0 };
}

/**
 * First candidate that is a non-empty array of non-blank strings, filtered down
 * to those strings. Used to fold a gateway response's parallel objectKey / URL
 * arrays (49 §6.1) into the single array a consumer reads. Returns `undefined`
 * when no candidate qualifies so the caller's own fallback still applies.
 */
/**
 * Non-empty strings out of one wire array. Deliberately single-argument: it used to
 * be variadic over a candidate list of field names, which is the shape that
 * hides which field actually carries the data (website#430 D4).
 */
function stringArray(value: unknown): string[] | undefined {
	if (!Array.isArray(value)) return undefined;
	const values = value.filter(
		(v): v is string => typeof v === "string" && v.trim().length > 0,
	);
	return values.length > 0 ? values : undefined;
}

/**
 * Untyped gateway entity / list payload. The seller / product / post domain is
 * still hand-read across ~10 consumers that index arbitrary fields. Phase c
 * (openapi codegen) replaces these with generated types; until then this
 * preserves the previous `any` contract. Centralised so the one unavoidable
 * `any` lives here rather than scattered across every fetcher.
 */
// biome-ignore lint/suspicious/noExplicitAny: Transitional untyped gateway entities require arbitrary property access until OpenAPI codegen lands.
type LooseJson = Record<string, any>;

/**
 * Optional filters for `/public/products`.
 *
 * Backend B-182 / B-183 (May 2026) promoted what used to be client-side
 * post-filters into real server query params:
 *   - `price_min` / `price_max`  numeric bounds (interval-overlap semantics)
 *   - `store_ids`                CSV of account ids
 *   - `followed_product_only`    bool — buyer's favourited products
 *   - `followed_seller_only`     bool — products from buyer's followed sellers
 *     (both followed_* require login; 400 if not authed)
 *
 * Plus the existing:
 *   - `hashtags` + `match` (any / all)
 *   - `sort` (LATEST / PRICE_ASC / PRICE_DESC / RATING / SALES)
 *   - `category_id` / `keyword` / `page` / `page_size`
 *
 * Each field is omitted from the request when empty so the URL stays
 * byte-identical for callers who don't supply an option.
 */
export interface FetchProductsOptions {
	hashtags?: string[];
	match?: "any" | "all";
	priceMin?: string | number;
	priceMax?: string | number;
	/** Multi-seller filter — replaces the legacy client-side
	 * post-filter that walked the products list. */
	sellerStoreIds?: Array<string | number>;
	/** When true, asks backend to keep only products the buyer has
	 * favourited (heart icon). Backend returns 400 if user isn't logged
	 * in; fetcher sends the session cookie for the viewer-aware filter. */
	followedProductOnly?: boolean;
	/** When true, asks backend to keep only products belonging to the
	 * buyer's followed sellers (subscriptions). Same cookie auth
	 * as `followedProductOnly`. */
	followedSellerOnly?: boolean;
	/** Forwarded from the TanStack queryFn so a rapid market-filter change
	 *  aborts the in-flight request instead of leaking it. */
	signal?: AbortSignal;
	/** See `RequestOptions.visitorIp` (backend#2264). Set only by the
	 *  `/api/public/*` route handlers, which can see the visitor; the cached
	 *  server readers leave it unset and are metered as SSR. */
	visitorIp?: string | null;
	/** Cache refreshes use strict transport so an outage cannot be mistaken for
	 *  a legitimate empty catalog and committed as fresh. */
	strict?: boolean;
}

export async function fetchProducts(
	keyword: string,
	page = 1,
	pageSize = 12,
	categoryId?: string,
	sort?: string,
	options?: FetchProductsOptions,
) {
	const params = new URLSearchParams({
		keyword,
		page: String(page),
		page_size: String(pageSize),
	});
	if (categoryId) params.set("category_id", categoryId);
	if (sort) params.set("sort", sort);
	if (options?.hashtags && options.hashtags.length > 0) {
		params.set("hashtags", options.hashtags.join(","));
	}
	if (options?.match) {
		params.set("match", options.match);
	}
	if (options?.priceMin !== undefined && options.priceMin !== "") {
		params.set("price_min", String(options.priceMin));
	}
	if (options?.priceMax !== undefined && options.priceMax !== "") {
		params.set("price_max", String(options.priceMax));
	}
	if (options?.sellerStoreIds?.length) {
		params.set("store_ids", options.sellerStoreIds.map(String).join(","));
	}
	if (options?.followedProductOnly) {
		params.set("followed_product_only", "true");
	}
	if (options?.followedSellerOnly) {
		params.set("followed_seller_only", "true");
	}
	// Viewer-aware filters require the buyer's session cookie on the public route.
	// Non-viewer branches also go through apiClient for consistency (same
	// credentials policy; server-side calls use env.serverApiBase automatically).
	// apiClient re-serialises a plain object back to URLSearchParams.
	const query = Object.fromEntries(params);
	const needsAuth = options?.followedProductOnly || options?.followedSellerOnly;
	if (needsAuth) {
		try {
			const envelope = await apiClient.public.get<LooseJson>("/products", {
				query,
				credentials: "include",
				signal: options?.signal,
				visitorIp: options?.visitorIp,
			});
			return envelope ?? { data: { records: [] } };
		} catch {
			return { data: { records: [] } };
		}
	}
	const envelope = await apiClient.public.get<LooseJson>("/products", {
		query,
		credentials: "omit",
		softFail: !options?.strict,
		signal: options?.signal,
		visitorIp: options?.visitorIp,
	});
	if (options?.strict && envelope.data == null) {
		throw new Error("products payload missing");
	}
	return envelope.data != null ? envelope : { code: 0, data: { records: [] } };
}

/**
 * 熱門商品：`/public/products?sort=SALES` 的第一頁，伺服器依銷量排序。回傳商品列，不含分頁資訊；
 * `strict` 時讀不到就丟錯，讓呼叫端的快取保留前一份而不是把空清單當成新資料。
 */
export async function fetchHotProducts(
	limit: number,
	options?: { signal?: AbortSignal; strict?: boolean },
): Promise<MarketplaceProduct[]> {
	const envelope = await fetchProducts("", 1, limit, undefined, "SALES", {
		signal: options?.signal,
		strict: options?.strict,
	});
	const records = (envelope as { data?: { records?: unknown } } | null)?.data
		?.records;
	return Array.isArray(records) ? (records as MarketplaceProduct[]) : [];
}

function isSellerAccountId(id: string): boolean {
	return /^\d+$/.test(id);
}

export async function fetchSeller(id: string, strict = false) {
	if (!isSellerAccountId(id)) return null;
	// Two failure policies by caller:
	//  - client (default): softFail — 404 / 5xx / timeout all resolve to null so a
	//    transient backend never crashes the calling component (e.g. chat context).
	//  - cache (strict, via getCachedSeller): softFailNotFound — a real 404 stays a
	//    cacheable null, but a 5xx / network error THROWS so the unstable_cache entry
	//    keeps its prior stale value instead of caching an outage-time null.
	//    prerenderSafe still guards the render layer at build as defense-in-depth.
	// The seller entity is still untyped (Phase c openapi codegen will type it); the
	// `any` data preserves the previous loose contract so the many seller consumers
	// compile unchanged.
	const { data: d } = await apiClient.public.get<LooseJson>(`/sellers/${id}`, {
		credentials: "omit",
		...(strict ? { softFailNotFound: true } : { softFail: true }),
	});
	if (!d) return null;
	// Mutate-in-place (d is a fresh per-request object) rather than spread — an
	// object spread would drop the Record index signature and re-narrow the result.
	d.name = d.name || d.storeName || d.sellerName || "";
	// §231: PublicSellerInfo sends `coverKey` (objectKey SSOT) + `coverUrl`
	// (Gateway-composed); every `coverImage` consumer runs it through
	// buildImageUrl/resolveCdnUrl, so the key is what we keep.
	d.coverImage = typeof d.coverKey === "string" ? d.coverKey : "";
	// `logo_url` 是**前端的顯示契約欄名**（B-238），不是 wire 名；wire 名是
	// `logoUrl`。這一層刻意換名並刪掉其他圖片來源，讓下游只有一條路可走。
	d.logo_url = typeof d.logoUrl === "string" ? d.logoUrl.trim() : "";
	for (const nonDisplayLogoField of [
		"logo",
		"logoUrl",
		"logoKey",
		"avatar",
	] as const) {
		delete d[nonDisplayLogoField];
	}
	// PublicSellerInfo already emits websiteUrl under exactly this name (seller.go).
	// It used to be re-derived from `website_url`, which after backend#2235
	// would have overwritten the real value with "".
	d.websiteUrl = d.websiteUrl || "";
	return d;
}

export async function fetchSellerProducts(
	id: string,
	page = 1,
	pageSize = 12,
	strict = false,
) {
	if (!isSellerAccountId(id)) return emptyRecords();
	// strict (via getCachedSellerProducts): a 5xx / network error throws so the
	// cache keeps stale; a 404 stays a cacheable empty list. Client callers keep
	// the soften-all default (empty list on any failure).
	// getCachedSellerProducts is the sole server caller and owns the window
	// (seller:<id> + products:list tags + COMMERCE_LIVE TTL), so there is no inner
	// next:{revalidate} here — it would hold a storefront's price/stock past a tag
	// bust it cannot see. The remaining callers are all 'use client' (chat context
	// panel, attachment picker, post composer, storefront pagination), where Next
	// never patches fetch and the option was inert anyway (#339 M4).
	const { data: d } = await apiClient.public.get<LooseJson>(
		`/sellers/${id}/products`,
		{
			query: { page: String(page), page_size: String(pageSize) },
			credentials: "omit",
			...(strict ? { softFailNotFound: true } : { softFail: true }),
		},
	);
	if (!d) return emptyRecords();
	return {
		records: d.records || d.items || [],
		total: d.total || 0,
		totalPages: d.totalPages || Math.ceil((d.total || 0) / pageSize),
	};
}

// Two failure policies by caller (mirrors fetchSellerProducts):
//  - strict (via getCachedSellerReviews): softFailNotFound — a 404 seller yields a
//    cacheable empty list, a 5xx / network error throws so the cache keeps its prior
//    stale page instead of caching an outage-time empty list.
//  - client (default): softFail — 404 / 5xx / timeout all resolve to an empty list so
//    a transient backend never throws into the client pagination path.
export async function fetchSellerReviews(id: string, page = 1, strict = false) {
	if (!isSellerAccountId(id)) return emptyRecords();
	const { data: d } = await apiClient.public.get<LooseJson>(
		`/sellers/${id}/reviews`,
		{
			query: { page: String(page), page_size: "10" },
			credentials: "omit",
			...(strict ? { softFailNotFound: true } : { softFail: true }),
		},
	);
	if (!d) return emptyRecords();
	return {
		// Backend returns the flat pair avatarKey/avatarUrl (see
		// PublicSellerReviewItem), while ReviewList reads `avatar` — so this
		// mapping is what makes buyer avatars resolve at all. §231: read the bare
		// objectKey; Avatar/buildImageUrl compose the URL.
		// Attached review photos arrive as imageKeys (+ the derived imageUrls
		// twin, which we don't read); ReviewList reads `images`, so without this
		// mapping they never rendered.
		records: (d.records || d.items || []).map((r: LooseJson) => ({
			...r,
			avatar: r.avatarKey,
			images: stringArray(r.imageKeys),
		})),
		total: d.total || 0,
		totalPages: d.totalPages || Math.ceil((d.total || 0) / 10),
	};
}

// strict (via getCachedSellerReviewStats): 5xx throws so the shared
// unstable_cache entry keeps stale; 404 stays a cacheable null. That wrapper is
// the sole server caller and owns the cache window (seller:<id> tag + SELLER_PROFILE
// TTL), so there is no inner next:{revalidate} here — a nested SWR layer would
// re-cache stale stats for an extra cycle. Client callers (default) soften all —
// e.g. products-import PreviewDetailPage reads it bare and degrades to rating 0.
export async function fetchSellerReviewStats(id: string, strict = false) {
	if (!isSellerAccountId(id)) return null;
	const { data } = await apiClient.public.get<LooseJson>(
		`/sellers/${id}/reviews/stats`,
		{
			credentials: "omit",
			...(strict ? { softFailNotFound: true } : { softFail: true }),
		},
	);
	return data ?? null;
}

export async function fetchProduct(sellerId: string, productId: string) {
	void sellerId;
	return fetchPublicProduct(productId);
}

export async function fetchPublicProduct(id: string) {
	// softFailNotFound: a real 404 → null, so a genuinely absent product (or the
	// app/[locale]/(public)/products/[id]/page.tsx id="0" sentinel) renders a cacheable
	// "找不到商品" shell. A 5xx / network error THROWS instead of softening — the
	// sole caller getCachedProduct then keeps its prior stale entry (runtime) and
	// getCachedProduct's .catch(prerenderSafe(null)) still absorbs it at build, so
	// a deploy-window outage never crashes the SSG export nor caches empty as fresh.
	// Product entity still untyped (Phase c openapi codegen will type it); `any`
	// preserves the previous loose contract for PDP / metadata consumers.
	// getCachedProduct is the only caller and owns the window (product:<id> tag +
	// COMMERCE_LIVE TTL), so there is no inner next:{revalidate} here — a second,
	// untagged fetch window is exactly what used to keep a just-changed price on
	// the PDP after its tag had already been busted (#339 M4).
	const { data } = await apiClient.public.get<LooseJson>(`/products/${id}`, {
		credentials: "omit",
		softFailNotFound: true,
	});
	return data ?? null;
}

/** `GET /public/sellers/{id}/stats` —— gateway `seller.go:137-144`。 */
export interface SellerStats {
	storeId?: number | string;
	followers?: number;
	rating?: number;
	totalOrders?: number;
	totalProducts?: number;
	totalSales?: number;
}

// Cache-only read (getCachedSellerStats is the sole caller): softFailNotFound —
// a 404 → cacheable null, a 5xx / network error throws so the cache keeps stale.
export async function fetchSellerStats(
	id: string,
): Promise<SellerStats | null> {
	if (!isSellerAccountId(id)) return null;
	const { data } = await apiClient.public.get<SellerStats>(
		`/sellers/${id}/stats`,
		{ credentials: "omit", softFailNotFound: true },
	);
	return data ?? null;
}

// strict (via getCachedCategoryTree): any transport failure throws so the cache
// keeps its prior tree rather than caching an empty taxonomy for the long 6h
// window. Client callers (Navbar / useCategoryTree) keep soften-all (empty tree).
export async function fetchCategoryTree(strict = false) {
	const { data } = await apiClient.public.get<unknown>("/categories/tree", {
		credentials: "omit",
		softFail: !strict,
	});
	if (strict && data == null) throw new Error("category tree payload missing");
	return data ?? [];
}
