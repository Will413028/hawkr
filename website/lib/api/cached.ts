import * as Sentry from "@sentry/nextjs";
import { unstable_cache } from "next/cache";
import { cache } from "react";
import type { ProductRecord } from "@/lib/market/listFilters";
import { fetchTrendingPostTags } from "@/lib/post-tags/repo";
import { fetchProductReviews } from "@/lib/reviews/repo";
import { fetchPopularHashtags } from "@/lib/search/repo";
import { PUBLIC_CACHE_TAGS, productTag, sellerTag } from "@/lib/seo/tags";
import {
	fetchCategoryTree,
	fetchMarketplace,
	fetchProducts,
	fetchPublicProduct,
	fetchSeller,
	fetchSellerProducts,
	fetchSellerReviewStats,
	fetchSellerReviews,
	fetchSellerStats,
	fetchSellers,
} from "./index";
import { fetchPublicPostsTyped } from "./posts";
import { prerenderSafe } from "./prerender";

/**
 * Cache-freshness tiers, in seconds — the single source of truth for every
 * `unstable_cache` revalidate window (the home tab-bootstrap route imports the
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
 * pages. Each wraps an existing fetcher in an `unstable_cache` scope keyed by
 * its args, stamps `tags` so POST /api/revalidate can bust it on a write-side
 * mutation, and a `revalidate` window as the TTL safety net. This replaces the
 * fetch's inner next:{revalidate} with one shared, tag-bustable entry.
 *
 * `unstable_cache` on its own does NOT dedup (corrected 2026-08-04,
 * backend#2192 — earlier wording claimed it "removes the need for a separate
 * React.cache dedup", which was wrong twice over, and no such layer had ever
 * existed here to remove). It calls `incrementalCache.get()` unconditionally on
 * every invocation, so a HIT still costs one cache-store read, and it has no
 * in-flight coalescing: Next's `pendingRevalidates` map covers only the
 * stale-revalidate branch, not concurrent cold reads of the same key. So three
 * components asking for one key in one render opened three concurrent reads
 * against the cache store — and the store answers a burst past its
 * concurrent-request ceiling with an error, not merely with latency.
 *
 * Every reader below is therefore wrapped in `React.cache` OUTSIDE its
 * `unstable_cache` scope: the first caller within a request creates the
 * promise, the rest await that same one, and the fan-out collapses to one read.
 *
 * Two properties of that wrapper shape the signatures here. React.cache keys on
 * the argument list by referential identity, so a reader takes primitives only
 * (an object arg would be a fresh reference per call and never coalesce). And
 * it keys on what the caller actually passed, not on defaults filled in
 * afterwards, so `f()` and `f(100)` are two entries — call sites pass every
 * argument explicitly to stay on one.
 *
 * Outside a request scope (route handlers, scripts) React.cache degrades to a
 * direct call: with no cache dispatcher installed it neither throws nor shares
 * state across requests (react/cjs/react.react-server.production.js:296-298;
 * the per-request dispatcher is installed by the Flight renderer in
 * react-server-dom-webpack-server.edge.production.js:683-697, 837).
 *
 * Server-only — never import from a 'use client' module (unstable_cache pulls
 * next/cache into the graph).
 */

// `meta` carries the attribution a miss needs to point at a specific upstream
// query: `key` is the fetcher's unstable_cache args (which id/page), `endpoint`
// the public API path it hit. Both ride on the span as searchable attributes.
type CacheMissMeta = { key?: string[]; endpoint?: string };

/**
 * Cross-request in-flight coalescing (backend#2264).
 *
 * `React.cache` (the wrapper each reader below already carries) collapses the
 * fan-out WITHIN one render. It cannot collapse anything across renders: it is a
 * per-request dispatcher, so two visitors landing on the same page in the same
 * isolate at the same moment each open their own cold read of the same key.
 * Measured duplication on the seller/market pages was 1.94–3.11×, and that
 * multiplier lands on the gateway as raw request count.
 *
 * The map is module scope, i.e. per Worker isolate and shared between visitors.
 * That is only safe because every entry here is an anonymous `/public/*` read
 * with `credentials: "omit"` and no viewer-derived field — an authed or
 * personalised read must NEVER be routed through this helper, or one visitor
 * would be served another's answer.
 */
const inFlightReads = new Map<string, Promise<unknown>>();

function singleFlight<T>(
	keyParts: string[],
	run: () => Promise<T>,
): Promise<T> {
	// NUL joiner: a key part may legally contain ":" (paths, encoded ids), and
	// two distinct key lists must never collide into one flight.
	const key = keyParts.join("\u0000");
	const joined = inFlightReads.get(key) as Promise<T> | undefined;
	if (joined) {
		// Joining is an optimisation, never a new failure mode. A joiner falls back
		// to its own read if the flight it joined fails, for two independent
		// reasons: on workerd the originator's request context can be torn down
		// (deadline raced, response already returned) while its fetch is still
		// pending, which surfaces to a joiner as an I/O error that has nothing to do
		// with the upstream; and without the fallback a joiner would silently lose
		// the retry it would otherwise have had. Under a real outage this costs
		// exactly what today costs — one attempt per caller.
		return joined.catch(() => run());
	}
	const started = run();
	const tracked = started.finally(() => {
		// Only clear our own entry: a later flight for the same key may already have
		// replaced it.
		if (inFlightReads.get(key) === tracked) inFlightReads.delete(key);
	});
	inFlightReads.set(key, tracked);
	return tracked;
}

/** Test-only: drops in-flight state so cases cannot leak into each other. */
export function __resetInFlightReadsForTest(): void {
	inFlightReads.clear();
}

/**
 * The coalescer on its own, for the aggregate cache owners that build their own
 * `unstable_cache` scope (the home tab-bootstrap route) instead of going through
 * `cachedRead`. Same constraint applies: anonymous public reads only.
 */
export const coalesceRead = singleFlight;

/**
 * One shared, tag-invalidated read: `unstable_cache` for the cross-request
 * entry, `singleFlight` for the concurrent cold reads `unstable_cache` does not
 * coalesce (its own `pendingRevalidates` map covers only the stale-revalidate
 * branch). Keyed on the same `keyParts` the cache entry uses, so the two layers
 * cannot disagree about what "the same read" means.
 */
export function cachedRead<T>(
	run: () => Promise<T>,
	keyParts: string[],
	options: { tags: string[]; revalidate: number },
): Promise<T> {
	return singleFlight(keyParts, () => unstable_cache(run, keyParts, options)());
}

/**
 * Wraps the one call that a cache miss actually pays for.
 *
 * The function body below only runs on a cache miss/regeneration
 * (`unstable_cache` serves hits without re-entering it), so every call here
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

export const getCachedProduct = cache(async (id: string) => {
	return cachedRead(
		() =>
			timedFetch("product", () => fetchPublicProduct(id), {
				key: ["product", id],
				endpoint: "/public/products/{id}",
			}),
		["product", id],
		{ tags: [productTag(id)], revalidate: CACHE_TTL.COMMERCE_LIVE },
	);
});

/** Every known caller immediately did `.catch(prerenderSafe(null))` — a
 * "not found" seller (invalid id, or the `generateStaticParams` sentinel) must
 * never be fatal to a prerender. Catching here instead of at each of the ~10
 * call sites means a caller can't ship without it (a missed `.catch` at one
 * such site took down the entire `next build` export when the seller-not-found
 * response moved off plain HTTP 404 — see backend SELLER_NOT_FOUND/9233).
 *
 * The `.catch` sits INSIDE the React.cache scope so the memoised value is the
 * already-resolved outcome: co-callers in one request see one verdict rather
 * than each re-deciding it. `prerenderSafe` is a pure function of NEXT_PHASE /
 * NODE_ENV / error type (lib/api/prerender.ts:35-47), so running it once per
 * request instead of once per call cannot change what any caller observes —
 * a runtime 5xx still rethrows, into every awaiting caller. */
export const getCachedSeller = cache(async (id: string) => {
	return cachedRead(
		() =>
			timedFetch("seller", () => fetchSeller(id, true), {
				key: ["seller", id],
				endpoint: "/public/sellers/{id}",
			}),
		["seller", id],
		{ tags: [sellerTag(id)], revalidate: CACHE_TTL.SELLER_PROFILE },
	).catch(prerenderSafe(null));
});

export const getCachedSellerReviewStats = cache(async (id: string) => {
	return cachedRead(
		() =>
			timedFetch(
				"seller-review-stats",
				() => fetchSellerReviewStats(id, true),
				{
					key: ["seller-review-stats", id],
					endpoint: "/public/sellers/{id}/reviews/stats",
				},
			),
		["seller-review-stats", id],
		{ tags: [sellerTag(id)], revalidate: CACHE_TTL.SELLER_PROFILE },
	);
});

/** Public seller aggregates (`/public/sellers/{id}/stats`) — the only wire
 *  that carries `followers`; the base seller payload has no subscriber field. */
export const getCachedSellerStats = cache(async (id: string) => {
	return cachedRead(
		() =>
			timedFetch("seller-stats", () => fetchSellerStats(id), {
				key: ["seller-stats", id],
				endpoint: "/public/sellers/{id}/stats",
			}),
		["seller-stats", id],
		{ tags: [sellerTag(id)], revalidate: CACHE_TTL.SELLER_PROFILE },
	);
});

// Reviews are cached under product:<id>. Primitive args (not an object) keep
// the cache key stable regardless of caller key-order. minRating-filtered
// reviews are intentionally NOT cached here — call fetchProductReviews directly
// for those (they bypass tag invalidation).
export const getCachedProductReviews = cache(
	async (id: string, page = 1, pageSize = 10) => {
		return cachedRead(
			() =>
				timedFetch(
					"product-reviews",
					() => fetchProductReviews(id, { page, pageSize }),
					{
						key: ["product-reviews", id, String(page), String(pageSize)],
						endpoint: "/public/products/{id}/reviews",
					},
				),
			["product-reviews", id, String(page), String(pageSize)],
			// Review bodies are not price/stock — the REVIEWS tier (long window) avoids
			// the per-minute regeneration the shared product:id tag would otherwise
			// force; the product:id tag still busts this entry on a write-side change.
			{ tags: [productTag(id)], revalidate: CACHE_TTL.REVIEWS },
		);
	},
);

export const getCachedSellerProducts = cache(
	async (id: string, page = 1, pageSize = 12) => {
		// A storefront's product cards carry price/stock/promotion state, so they
		// use the live-list window rather than the much longer seller-profile
		// window. Both seller and aggregate-product events can invalidate the page:
		// the `products:list` tag is busted by POST /api/revalidate for ANY product
		// mutation (parseRevalidateRequest expands product:<id> → products:list),
		// so a per-page product change refreshes this entry without needing the
		// result-derived per-product tags that unstable_cache cannot attach.
		return cachedRead(
			() =>
				timedFetch(
					"seller-products",
					() => fetchSellerProducts(id, page, pageSize, true),
					{
						key: ["seller-products", id, String(page), String(pageSize)],
						endpoint: "/public/sellers/{id}/products",
					},
				),
			["seller-products", id, String(page), String(pageSize)],
			{
				tags: [sellerTag(id), PUBLIC_CACHE_TAGS.products],
				revalidate: CACHE_TTL.COMMERCE_LIVE,
			},
		);
	},
);

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
/*
 * Anonymous landing-list reads.
 *
 * These are credentials:"omit" public reads with no viewer-specific fields, so
 * the entries are safe to share across sessions. Each entry is tagged with the
 * data domains it embeds, so POST /api/revalidate refreshes an aggregate only
 * when a record it actually renders changed; the inline revalidate
 * windows remain the safety net. Aggregate cache owners pass `strict` to their
 * fetcher so a transient backend outage throws (keeping the prior stale entry)
 * instead of committing a synthetic empty payload as fresh.
 */

export const getCachedMarketplace = cache(
	async (sellerLimit = 8, postLimit = 6, productLimit = 8) => {
		return cachedRead(
			() =>
				timedFetch(
					"marketplace",
					() =>
						fetchMarketplace(
							sellerLimit,
							postLimit,
							productLimit,
							undefined,
							true,
						),
					{
						key: [
							"marketplace",
							String(sellerLimit),
							String(postLimit),
							String(productLimit),
						],
						endpoint: "/public/marketplace",
					},
				),
			[
				"marketplace",
				String(sellerLimit),
				String(postLimit),
				String(productLimit),
			],
			{
				tags: [
					PUBLIC_CACHE_TAGS.products,
					PUBLIC_CACHE_TAGS.sellers,
					PUBLIC_CACHE_TAGS.posts,
				],
				revalidate: CACHE_TTL.COMMERCE_LIVE,
			},
		);
	},
);

/** First page of the unfiltered public products list (keyword ""). The home
 *  shelves and the origin-map aggregation share this single read. */
export const getCachedProductsFirstPage = cache(async (pageSize = 100) => {
	return cachedRead(
		() =>
			timedFetch(
				"products-first-page",
				() =>
					fetchProducts("", 1, pageSize, undefined, undefined, {
						strict: true,
					}),
				{
					key: ["products-first-page", String(pageSize)],
					endpoint: "/public/products",
				},
			),
		["products-first-page", String(pageSize)],
		{
			tags: [PUBLIC_CACHE_TAGS.products],
			revalidate: CACHE_TTL.COMMERCE_LIVE,
		},
	);
});

/**
 * Strict, bounded canonical pages used by browser freshness checks through a
 * same-origin route. A failed regeneration throws so OpenNext keeps the prior
 * stale entry instead of caching a synthetic empty catalog as fresh.
 */
export const getCachedStrictProductsPage = cache(
	async (page = 1, pageSize = 24) => {
		return cachedRead(
			() =>
				timedFetch(
					`products-page-${page}`,
					() =>
						fetchProducts("", page, pageSize, undefined, undefined, {
							strict: true,
						}),
					{
						key: ["products-strict-page", String(page), String(pageSize)],
						endpoint: "/public/products",
					},
				),
			["products-strict-page", String(page), String(pageSize)],
			{
				tags: [PUBLIC_CACHE_TAGS.products],
				revalidate: CACHE_TTL.COMMERCE_LIVE,
			},
		);
	},
);

export const getCachedRecommendedSellers = cache(async (limit = 8) => {
	return cachedRead(
		() =>
			timedFetch(
				"recommended-sellers",
				() => fetchSellers(limit, undefined, true),
				{
					key: ["recommended-sellers", String(limit)],
					endpoint: "/public/sellers/recommended",
				},
			),
		["recommended-sellers", String(limit)],
		{
			tags: [PUBLIC_CACHE_TAGS.sellers],
			revalidate: CACHE_TTL.DISCOVERY,
		},
	);
});

export const getCachedPublicPosts = cache(async (page = 1, pageSize = 8) => {
	return cachedRead(
		() =>
			timedFetch(
				"public-posts",
				() => fetchPublicPostsTyped({ page, pageSize, strict: true }),
				{
					key: ["public-posts", String(page), String(pageSize)],
					endpoint: "/public/posts",
				},
			),
		["public-posts", String(page), String(pageSize)],
		{
			tags: [PUBLIC_CACHE_TAGS.posts],
			revalidate: CACHE_TTL.COMMERCE_LIVE,
		},
	);
});

/**
 * Global category tree (`/public/categories/tree`) — the same content for every
 * viewer, credentials:"omit". The navbar megamenu (components/shared/RootDocument.tsx, so every
 * page) and /market/products both read it; without this wrapper each render
 * re-paid the full gateway round-trip. Tagged "categories" (TTL-only; the tree
 * changes rarely and no write-side busts it today) with the long taxonomy
 * window. Client callers (lib/hooks/useCategoryTree) keep TanStack Query.
 */
export const getCachedCategoryTree = cache(async () => {
	return cachedRead(
		() =>
			timedFetch("category-tree", () => fetchCategoryTree(true), {
				key: ["category-tree"],
				endpoint: "/public/categories/tree",
			}),
		["category-tree"],
		{
			tags: [PUBLIC_CACHE_TAGS.categories],
			revalidate: CACHE_TTL.TAXONOMY,
		},
	);
});

/**
 * Popular product hashtags (`/public/hashtags/popular`) — public chrome for the
 * navbar megamenu + market sidebar. Primitive `limit` arg keeps the cache key
 * stable; discovery content gets a longer TTL than price/stock lists.
 */
export const getCachedPopularHashtags = cache(async (limit = 8) => {
	return cachedRead(
		() =>
			timedFetch(
				"popular-hashtags",
				() => fetchPopularHashtags({ limit, strict: true }),
				{
					key: ["popular-hashtags", String(limit)],
					endpoint: "/public/hashtags/popular",
				},
			),
		["popular-hashtags", String(limit)],
		{
			tags: [PUBLIC_CACHE_TAGS.hashtags],
			revalidate: CACHE_TTL.DISCOVERY,
		},
	);
});

/**
 * Trending post tags (`/public/tags/trending`) — the footer's 熱門話題 column,
 * which renders on every public page. Distinct from `getCachedPopularHashtags`:
 * that one is product hashtags for the megamenu, these are post tags that link
 * into `/feed`. Tagged "posts" because trending-ness is recomputed from post
 * activity, so a post write busts it through the same POST /api/revalidate path.
 */
export const getCachedTrendingPostTags = cache(async (limit = 8) => {
	return cachedRead(
		() =>
			timedFetch("trending-post-tags", () => fetchTrendingPostTags({ limit }), {
				key: ["trending-post-tags", String(limit)],
				endpoint: "/public/tags/trending",
			}),
		["trending-post-tags", String(limit)],
		{
			tags: [PUBLIC_CACHE_TAGS.posts],
			revalidate: CACHE_TTL.DISCOVERY,
		},
	);
});

/**
 * Public seller reviews list (`/public/sellers/{id}/reviews`) for the seller
 * profile 評價 tab. Cached under seller:<id> so a write-side review/seller
 * mutation busts it via POST /api/revalidate; primitive args keep the key
 * stable per page.
 */
export const getCachedSellerReviews = cache(async (id: string, page = 1) => {
	return cachedRead(
		() =>
			timedFetch("seller-reviews", () => fetchSellerReviews(id, page, true), {
				key: ["seller-reviews", id, String(page)],
				endpoint: "/public/sellers/{id}/reviews",
			}),
		["seller-reviews", id, String(page)],
		{ tags: [sellerTag(id)], revalidate: CACHE_TTL.SELLER_PROFILE },
	);
});

export const getCachedSellerStorefrontProducts = cache(
	async (
		id: string,
		page = 1,
		pageSize = 12,
	): Promise<{
		records: ProductRecord[];
		total: number;
		totalPages: number;
	}> => {
		if (!/^\d+$/.test(id)) return { records: [], total: 0, totalPages: 0 };
		// `products:list` is busted by POST /api/revalidate for ANY product mutation
		// (parseRevalidateRequest expands product:<id> → products:list), so the fixed
		// seller + products tags cover "a product on this page changed" without the
		// result-derived per-product tags that unstable_cache cannot attach. `strict`
		// lets an outage (5xx / network) throw OUT of the cache body so the prior stale
		// grid is kept (StorefrontPage's .catch(prerenderSafe) soft-fails the build
		// prerender only — at runtime it re-throws to the error boundary); a genuine
		// filter miss stays a 200-empty grid (no throw) so StorefrontPage still falls
		// back to the leaner seller endpoint — an outage no longer masquerades as empty.
		return cachedRead(
			async () => {
				const json = await timedFetch(
					"seller-storefront-products",
					() =>
						fetchProducts("", page, pageSize, undefined, undefined, {
							sellerStoreIds: [id],
							strict: true,
						}),
					{
						key: [
							"seller-storefront-products",
							id,
							String(page),
							String(pageSize),
						],
						endpoint: "/public/products",
					},
				);
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
			},
			["seller-storefront-products", id, String(page), String(pageSize)],
			{
				tags: [sellerTag(id), PUBLIC_CACHE_TAGS.products],
				revalidate: CACHE_TTL.COMMERCE_LIVE,
			},
		);
	},
);
