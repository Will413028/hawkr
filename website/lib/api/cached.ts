import * as Sentry from "@sentry/nextjs";
import { cacheLife, cacheTag } from "next/cache";
import { fetchRecommendedStores } from "@/lib/discovery/recommended";
import { defaultLocale } from "@/lib/intl/locales";
import type { ProductRecord } from "@/lib/market/listFilters";
import { fetchTrendingPostTags } from "@/lib/post-tags/repo";
import { fetchProductReviews } from "@/lib/reviews/repo";
import { fetchPopularHashtags } from "@/lib/search/repo";
import { PUBLIC_CACHE_TAGS, productTag, sellerTag } from "@/lib/seo/tags";
import {
	fetchCategoryTree,
	fetchHotProducts,
	fetchProducts,
	fetchPublicProduct,
	fetchSeller,
	fetchSellerProducts,
	fetchSellerReviewStats,
	fetchSellerReviews,
	fetchSellerStats,
} from "./index";
import { fetchPublicPostsTyped } from "./posts";
import { prerenderSafe } from "./prerender";

/**
 * Cache-freshness tiers, in seconds — the single source of truth for every
 * `'use cache'` revalidate window (the home tab-bootstrap route imports the
 * same model, so a TTL lives in exactly one place).
 *
 * Each value is a *safety net*: the maximum staleness tolerated when a write-side
 * invalidation event is silently missed — NOT the primary freshness path. That
 * path is POST /api/revalidate busting the entry's tag on a write-side mutation;
 * within a window a client route-cache entry can paint immediately while Next
 * refreshes it in the background (stale-while-revalidate).
 *
 * These entries are the *only* cache window on their reads: the fetchers below
 * carry no inner `next: { revalidate }`, so a tag bust here reaches the
 * backend on the next read instead of being absorbed by a second, untagged
 * fetch-level window (#339 M4).
 */
export const CACHE_TTL = {
	/** Price / stock / publish-state lists + product detail — commercially
	 *  sensitive (also covers the social feed landing list).
	 *
	 *  The window is a safety net, not the freshness path: a price/stock/publish
	 *  mutation busts `product:<id>` / `products:list` and surfaces on the next
	 *  read, which dev measurement confirmed is precise per entity after the tag
	 *  taxonomy converged (#339 M3). It is also the read frequency the backend
	 *  already saw — before M4 these reads sat behind a 120–300s inner fetch
	 *  window that a tag could not reach, so collapsing to a single 300s tagged
	 *  window keeps upstream load flat while making invalidation authoritative.
	 *  Shorter would multiply backend reads for freshness the tag path already
	 *  provides. */
	COMMERCE_LIVE: 300,
	/** Seller profile + aggregates (name / avatar / bio / stats) — low churn. */
	SELLER_PROFILE: 1_800,
	/** Editorial / ranking discovery (recommended sellers, hashtags). */
	DISCOVERY: 900,
	/** Review bodies — low volatility, tolerant of staleness. */
	REVIEWS: 900,
	/** Category taxonomy — no write-side invalidation event today; TTL is the
	 *  primary freshness path, so the window is long. */
	TAXONOMY: 21_600,
} as const;

/**
 * Cross-request-cached, tag-invalidated entity reads for the indexable SEO
 * pages. Each reader is one `'use cache'` function: `cacheTag` lets POST
 * /api/revalidate bust it on a write-side mutation, and `cacheLife` sets the
 * TTL safety net (`CACHE_TTL`). A reader takes primitives only: the arguments
 * are the cache key.
 *
 * `'use cache'` also collapses concurrent reads of one key, within a request and
 * across requests, so no extra in-flight layer sits in front of it
 * (next/dist/server/use-cache/use-cache-wrapper.js: `pendingCacheInvocations`
 * for one request, `crossRequestPendingCacheInvocations` across requests).
 *
 * The public wrappers fill argument defaults and call the cached function with
 * every argument explicit, so `f()` and `f(100)` are one entry.
 *
 * Never call these from a scope that reads cookies or headers, and never put a
 * personalised read behind a `'use cache'` function: every entry here is an
 * anonymous `/public/*` read with `credentials: "omit"`.
 *
 * Server-only — never import from a 'use client' module.
 */

// `meta` carries the attribution a miss needs to point at a specific upstream
// query: `key` is the fetcher's cache key (which id/page), `endpoint` the
// public API path it hit. Both ride on the span as searchable attributes.
type CacheMissMeta = { key?: string[]; endpoint?: string };

/** Applies a freshness tier from `CACHE_TTL` to the enclosing `'use cache'` scope. */
export function applyCacheTier(tier: keyof typeof CACHE_TTL): void {
	cacheLife({ revalidate: CACHE_TTL[tier] });
}

/**
 * A read that failed while `next build` was prerendering. Next treats ANY throw out of a `'use cache'`
 * function as fatal to the build, even when the caller catches it, and the build runs against an
 * upstream that may be down. So at build time a failed fill returns this marker instead of throwing, and
 * the public reader turns it back into the throw its callers already handle (`prerenderSafe`, `.catch`).
 * The entry is cached for a minute at most so a prerendered shell built on an outage regenerates soon.
 * At runtime a failed fill still throws: the cache keeps serving its stale entry.
 */
const BUILD_READ_FAILED = "buildReadFailed";
export type BuildReadFailed = { readonly [BUILD_READ_FAILED]: true };

function isBuildReadFailed(value: unknown): value is BuildReadFailed {
	return (
		typeof value === "object" &&
		value !== null &&
		BUILD_READ_FAILED in value &&
		(value as Record<string, unknown>)[BUILD_READ_FAILED] === true
	);
}

/**
 * The body of a `'use cache'` fill: `timedFetch` plus the build-time marker above. Call it only inside a
 * `'use cache'` function, and pass the result through `settled` before it leaves `lib/api/cached.ts`.
 */
export async function fillFetch<T>(
	label: string,
	fn: () => Promise<T>,
	meta?: CacheMissMeta,
): Promise<T | BuildReadFailed> {
	try {
		return await timedFetch(label, fn, meta);
	} catch (error) {
		if (process.env.NEXT_PHASE !== "phase-production-build") throw error;
		console.warn(`[cache-fill] ${label} failed during build:`, error);
		cacheLife({ revalidate: 60 });
		return { [BUILD_READ_FAILED]: true };
	}
}

/** Unwraps a cached fill: a build-time failure marker becomes the throw the caller would have seen. */
export function settled<T>(result: T | BuildReadFailed): T {
	if (isBuildReadFailed(result)) {
		throw new Error("upstream read failed while prerendering");
	}
	return result;
}

/**
 * Wraps the one call that a cache miss actually pays for.
 *
 * The function body below only runs on a cache miss/regeneration
 * (`'use cache'` serves hits without re-entering it), so every call here
 * already is the cache-miss timing. It emits two things, and **neither of them
 * is an alert**:
 *
 *   - a `console.log` line, at info level, which Workers Observability keeps
 *     queryable per invocation;
 *   - one Sentry span per miss — every miss, no threshold — so the timing is
 *     aggregatable (p50/p95 by `cache.miss` op, sliceable by label / cache_key
 *     / upstream_endpoint) instead of being reduced to a boolean at the point
 *     of emission.
 *
 * The threshold that used to live here (a `console.error` beacon plus a Sentry
 * warning above 2s) is gone on purpose. Seven days of production data put 17 of
 * 17 slow misses on the background stale-while-revalidate path: the visitor
 * already held the stale response and nobody was waiting on the fetch. So it
 * paged for something no user experienced, every day, in the same Discord
 * channel that carries real Worker failures — and a channel that cries wolf
 * gets muted wholesale, taking the real alerts with it. `console.error` also
 * made a *successful* invocation register as an error in Workers Observability,
 * which corrupts the error-rate signal that the tail worker's alerting is
 * derived from.
 *
 * A slow miss is still worth recording; it is just diagnostic data, to be
 * queried when someone is already investigating, not a page. If a slow miss
 * ever becomes user-visible it will surface where users are: a request-scoped
 * span (this one is a child of it) or a client-side timeout.
 *
 * `Sentry.startSpan` does not alter what the caller observes — it returns the
 * callback's value and rethrows its rejection unchanged
 * (@sentry/core .../utils/handleCallbackErrors.js), and with no client or
 * during `next build` it degrades to a non-recording span that still runs the
 * callback (@sentry/nextjs .../common/utils/nextSpan.js).
 *
 * Exported for the home tab-bootstrap route: its aggregate cache owners call raw
 * fetchers directly (to avoid nested SWR windows) and so miss the per-domain
 * wrappers' timing — they reuse this to stay on the same span.
 */
export async function timedFetch<T>(
	label: string,
	fn: () => Promise<T>,
	meta?: CacheMissMeta,
): Promise<T> {
	return Sentry.startSpan(
		{
			op: "cache.miss",
			name: label,
			attributes: {
				cache_label: label,
				...(meta?.key ? { cache_key: meta.key.join(":") } : {}),
				...(meta?.endpoint ? { upstream_endpoint: meta.endpoint } : {}),
			},
		},
		async () => {
			const startedAt = Date.now();
			try {
				return await fn();
			} finally {
				console.log(`[cache-miss] ${label} ${Date.now() - startedAt}ms`);
			}
		},
	);
}

export async function getCachedProduct(id: string) {
	return settled(await readProduct(id));
}

export async function getCachedSellerReviewStats(id: string) {
	return settled(await readSellerReviewStats(id));
}

/** Public seller aggregates (`/public/sellers/{id}/stats`) — the only wire
 *  that carries `followers`; the base seller payload has no subscriber field. */
export async function getCachedSellerStats(id: string) {
	return settled(await readSellerStats(id));
}

async function readProduct(id: string) {
	"use cache";
	cacheTag(productTag(id));
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch("product", () => fetchPublicProduct(id), {
		key: ["product", id],
		endpoint: "/public/products/{id}",
	});
}

async function readSeller(id: string) {
	"use cache";
	cacheTag(sellerTag(id));
	applyCacheTier("SELLER_PROFILE");
	return fillFetch("seller", () => fetchSeller(id, true), {
		key: ["seller", id],
		endpoint: "/public/sellers/{id}",
	});
}

/** Every known caller immediately did `.catch(prerenderSafe(null))` — a
 * "not found" seller (invalid id, or the `generateStaticParams` sentinel) must
 * never be fatal to a prerender. Catching here instead of at each of the ~10
 * call sites means a caller can't ship without it (a missed `.catch` at one
 * such site took down the entire `next build` export when the seller-not-found
 * response moved off plain HTTP 404 — see backend SELLER_NOT_FOUND/9233).
 *
 * The `.catch` sits OUTSIDE the cached read so a failure is never stored as the
 * entry; `prerenderSafe` is a pure function of NEXT_PHASE / NODE_ENV / error
 * type (lib/api/prerender.ts), so a runtime 5xx still rethrows into every
 * awaiting caller. */
export function getCachedSeller(id: string) {
	return readSeller(id).then(settled).catch(prerenderSafe(null));
}

async function readSellerReviewStats(id: string) {
	"use cache";
	cacheTag(sellerTag(id));
	applyCacheTier("SELLER_PROFILE");
	return fillFetch(
		"seller-review-stats",
		() => fetchSellerReviewStats(id, true),
		{
			key: ["seller-review-stats", id],
			endpoint: "/public/sellers/{id}/reviews/stats",
		},
	);
}

async function readSellerStats(id: string) {
	"use cache";
	cacheTag(sellerTag(id));
	applyCacheTier("SELLER_PROFILE");
	return fillFetch("seller-stats", () => fetchSellerStats(id), {
		key: ["seller-stats", id],
		endpoint: "/public/sellers/{id}/stats",
	});
}

async function readProductReviews(id: string, page: number, pageSize: number) {
	"use cache";
	// Review bodies are not price/stock — the REVIEWS tier (long window) avoids
	// the per-minute regeneration the shared product:id tag would otherwise
	// force; the product:id tag still busts this entry on a write-side change.
	cacheTag(productTag(id));
	applyCacheTier("REVIEWS");
	return fillFetch(
		"product-reviews",
		() => fetchProductReviews(id, { page, pageSize }),
		{
			key: ["product-reviews", id, String(page), String(pageSize)],
			endpoint: "/public/products/{id}/reviews",
		},
	);
}

// minRating-filtered reviews are intentionally NOT cached here — call
// fetchProductReviews directly for those (they bypass tag invalidation).
export function getCachedProductReviews(id: string, page = 1, pageSize = 10) {
	return readProductReviews(id, page, pageSize).then(settled);
}

async function readSellerProducts(id: string, page: number, pageSize: number) {
	"use cache";
	// A storefront's product cards carry price/stock/promotion state, so they
	// use the live-list window rather than the much longer seller-profile
	// window. Both seller and aggregate-product events can invalidate the page:
	// the `products:list` tag is busted by POST /api/revalidate for ANY product
	// mutation (parseRevalidateRequest expands product:<id> → products:list),
	// so a per-page product change refreshes this entry without needing the
	// result-derived per-product tags a cache scope cannot attach.
	cacheTag(sellerTag(id), PUBLIC_CACHE_TAGS.products);
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch(
		"seller-products",
		() => fetchSellerProducts(id, page, pageSize, true),
		{
			key: ["seller-products", id, String(page), String(pageSize)],
			endpoint: "/public/sellers/{id}/products",
		},
	);
}

export function getCachedSellerProducts(id: string, page = 1, pageSize = 12) {
	return readSellerProducts(id, page, pageSize).then(settled);
}

/*
 * Anonymous landing-list reads.
 *
 * These are credentials:"omit" public reads with no viewer-specific fields, so
 * the entries are safe to share across sessions. Each entry is tagged with the
 * data domains it embeds, so POST /api/revalidate refreshes an aggregate only
 * when a record it actually renders changed; the cacheLife windows remain the
 * safety net. Aggregate cache owners pass `strict` to their fetcher so a
 * transient backend outage throws (keeping the prior stale entry) instead of
 * committing a synthetic empty payload as fresh.
 */

async function readHotProducts(limit: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.products);
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch(
		"hot-products",
		() => fetchHotProducts(limit, { strict: true }),
		{
			key: ["hot-products", String(limit)],
			endpoint: "/public/products?sort=SALES",
		},
	);
}

export function getCachedHotProducts(limit = 8) {
	return readHotProducts(limit).then(settled);
}

async function readProductsFirstPage(pageSize: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.products);
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch(
		"products-first-page",
		() =>
			fetchProducts("", 1, pageSize, undefined, undefined, {
				strict: true,
			}),
		{
			key: ["products-first-page", String(pageSize)],
			endpoint: "/public/products",
		},
	);
}

/** First page of the unfiltered public products list (keyword ""). The home
 *  shelves and the origin-map aggregation share this single read. */
export function getCachedProductsFirstPage(pageSize = 100) {
	return readProductsFirstPage(pageSize).then(settled);
}

async function readStrictProductsPage(page: number, pageSize: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.products);
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch(
		`products-page-${page}`,
		() =>
			fetchProducts("", page, pageSize, undefined, undefined, {
				strict: true,
			}),
		{
			key: ["products-strict-page", String(page), String(pageSize)],
			endpoint: "/public/products",
		},
	);
}

/**
 * Strict, bounded canonical pages used by browser freshness checks through a
 * same-origin route. A failed regeneration throws so the cache keeps the prior
 * stale entry instead of caching a synthetic empty catalog as fresh.
 */
export function getCachedStrictProductsPage(page = 1, pageSize = 24) {
	return readStrictProductsPage(page, pageSize).then(settled);
}

async function readRecommendedSellers(limit: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.sellers);
	applyCacheTier("DISCOVERY");
	return fillFetch(
		"recommended-sellers",
		() => fetchRecommendedStores(limit, { locale: defaultLocale }),
		{
			key: ["recommended-sellers", String(limit)],
			endpoint: "DiscoveryPublicService/SuggestRecommendedSellers",
		},
	);
}

export function getCachedRecommendedSellers(limit = 8) {
	return readRecommendedSellers(limit).then(settled);
}

async function readPublicPosts(page: number, pageSize: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.posts);
	applyCacheTier("COMMERCE_LIVE");
	return fillFetch(
		"public-posts",
		() => fetchPublicPostsTyped({ page, pageSize, strict: true }),
		{
			key: ["public-posts", String(page), String(pageSize)],
			endpoint: "/public/posts",
		},
	);
}

export function getCachedPublicPosts(page = 1, pageSize = 8) {
	return readPublicPosts(page, pageSize).then(settled);
}

async function readCategoryTree() {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.categories);
	applyCacheTier("TAXONOMY");
	return fillFetch("category-tree", () => fetchCategoryTree(true), {
		key: ["category-tree"],
		endpoint: "/public/categories/tree",
	});
}

/**
 * Global category tree (`/public/categories/tree`) — the same content for every
 * viewer, credentials:"omit". The navbar megamenu (components/shared/RootDocument.tsx, so every
 * page) and /market/products both read it; without this wrapper each render
 * re-paid the full gateway round-trip. Tagged "categories" (TTL-only; the tree
 * changes rarely and no write-side busts it today) with the long taxonomy
 * window. Client callers (lib/hooks/useCategoryTree) keep TanStack Query.
 */
export async function getCachedCategoryTree() {
	return settled(await readCategoryTree());
}

async function readPopularHashtags(limit: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.hashtags);
	applyCacheTier("DISCOVERY");
	return fillFetch(
		"popular-hashtags",
		() => fetchPopularHashtags({ limit, strict: true }),
		{
			key: ["popular-hashtags", String(limit)],
			endpoint: "/public/hashtags/popular",
		},
	);
}

/**
 * Popular product hashtags (`/public/hashtags/popular`) — public chrome for the
 * navbar megamenu + market sidebar. Discovery content gets a longer TTL than
 * price/stock lists.
 */
export function getCachedPopularHashtags(limit = 8) {
	return readPopularHashtags(limit).then(settled);
}

async function readTrendingPostTags(limit: number) {
	"use cache";
	cacheTag(PUBLIC_CACHE_TAGS.posts);
	applyCacheTier("DISCOVERY");
	return fillFetch(
		"trending-post-tags",
		() => fetchTrendingPostTags({ limit }),
		{
			key: ["trending-post-tags", String(limit)],
			endpoint: "/public/tags/trending",
		},
	);
}

/**
 * Trending post tags (`/public/tags/trending`) — the footer's 熱門話題 column,
 * which renders on every public page. Distinct from `getCachedPopularHashtags`:
 * that one is product hashtags for the megamenu, these are post tags that link
 * into `/feed`. Tagged "posts" because trending-ness is recomputed from post
 * activity, so a post write busts it through the same POST /api/revalidate path.
 */
export function getCachedTrendingPostTags(limit = 8) {
	return readTrendingPostTags(limit).then(settled);
}

/**
 * Public seller reviews list (`/public/sellers/{id}/reviews`) for the seller
 * profile 評價 tab. Cached under seller:<id> so a write-side review/seller
 * mutation busts it via POST /api/revalidate.
 */
export async function getCachedSellerReviews(id: string, page = 1) {
	return readSellerReviews(id, page).then(settled);
}

async function readSellerReviews(id: string, page: number) {
	"use cache";
	cacheTag(sellerTag(id));
	applyCacheTier("SELLER_PROFILE");
	return fillFetch("seller-reviews", () => fetchSellerReviews(id, page, true), {
		key: ["seller-reviews", id, String(page)],
		endpoint: "/public/sellers/{id}/reviews",
	});
}

type StorefrontProducts = {
	records: ProductRecord[];
	total: number;
	totalPages: number;
};

async function readSellerStorefrontProducts(
	id: string,
	page: number,
	pageSize: number,
): Promise<StorefrontProducts | BuildReadFailed> {
	"use cache";
	// `products:list` is busted by POST /api/revalidate for ANY product mutation
	// (parseRevalidateRequest expands product:<id> → products:list), so the fixed
	// seller + products tags cover "a product on this page changed" without the
	// result-derived per-product tags a cache scope cannot attach. `strict`
	// lets an outage (5xx / network) throw OUT of the cache body so the prior stale
	// grid is kept (StorefrontPage's .catch(prerenderSafe) soft-fails the build
	// prerender only — at runtime it re-throws to the error boundary); a genuine
	// filter miss stays a 200-empty grid (no throw) so StorefrontPage still falls
	// back to the leaner seller endpoint — an outage no longer masquerades as empty.
	cacheTag(sellerTag(id), PUBLIC_CACHE_TAGS.products);
	applyCacheTier("COMMERCE_LIVE");
	const json = await fillFetch(
		"seller-storefront-products",
		() =>
			fetchProducts("", page, pageSize, undefined, undefined, {
				sellerStoreIds: [id],
				strict: true,
			}),
		{
			key: ["seller-storefront-products", id, String(page), String(pageSize)],
			endpoint: "/public/products",
		},
	);
	if (isBuildReadFailed(json)) return json;
	const data = (
		json as {
			data?: { records?: unknown[]; total?: number; totalPages?: number };
		} | null
	)?.data;
	const records = (data?.records ?? []) as ProductRecord[];
	const total = Number(data?.total ?? 0) || 0;
	const totalPages =
		Number(data?.totalPages ?? 0) || Math.ceil(total / pageSize) || 0;
	return { records, total, totalPages };
}

/**
 * Storefront product grid, sourced from the rich market endpoint
 * (`GET /public/products` filtered by `store_ids`) instead of the
 * leaner `/public/sellers/{id}/products`. The market endpoint carries the
 * fields the seller-scoped one omits (B-192) — sale price (so the card's
 * strikethrough renders) and `favoriteCount` (the heart badge) — so a
 * product looks identical here and on 市集.
 *
 * The route `id` is the seller's account id, the same id space
 * `store_ids` filters on (see `readSellerId`), so this is a direct
 * swap. Returns the same `{ records, total, totalPages }` shape as
 * `getCachedSellerProducts`; callers fall back to that when this is empty.
 */
export async function getCachedSellerStorefrontProducts(
	id: string,
	page = 1,
	pageSize = 12,
): Promise<StorefrontProducts> {
	if (!/^\d+$/.test(id)) return { records: [], total: 0, totalPages: 0 };
	return settled(await readSellerStorefrontProducts(id, page, pageSize));
}
