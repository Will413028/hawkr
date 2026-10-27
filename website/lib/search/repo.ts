/**
 * Search — `/public/search/*`.
 *
 * 2026-08-05 逐一實打 dev 對過一次，因為這段原本列的 7 個端點裡有 4 個根本不存在
 * （後端從未註冊，而 `publicGet` 對 404 回 undefined 不 throw，所以沒有任何跡象）。
 *
 * **實際存在**：
 *   GET  /public/search/products?q=&page=&size=
 *   GET  /public/search/suggest?q=
 *
 * **不存在（404）**：
 *   GET  /public/search/hot                ← 對應的 fetchHotKeywords() 已於本次移除（零呼叫端）
 *   POST /public/search/products/filter
 *
 * 後端 `gateway/internal/router/constants.go:267-269` 只註冊了
 * `/products`、`/suggest` 兩支。
 */

import { apiClient } from "../api/client";

export interface SearchSellerInfo {
	name?: string;
	/** B-238 canonical seller-logo display URL. */
	logoUrl?: string;
	address?: string;
	city?: string;
	district?: string;
	storeId?: number | string;
}

export interface SearchProductHit {
	id: number | string;
	name: string;
	/** §231 flat-pair: `imageKey`/`imageKeys` (bare objectKeys) are the SSOT.
	 *  The `imageUrl`/`imageUrls` twins are `mediaurl.BuildURL(base, imageKey…)`
	 *  (gateway search.go:226-229) — derived from these keys, so they can never
	 *  supply an image the keys lack. Not declared, so they cannot come back as
	 *  a fallback. */
	imageKey?: string;
	imageKeys?: string[];
	price?: string;
	original_price?: string;
	/** Gateway `/public/search/products` emits camelCase JSON body keys, same as
	 *  `/public/products` (§2: query strings are snake_case, bodies are not). */
	priceMin?: string;
	priceMax?: string;
	rating?: number;
	sellerName?: string;
	seller?: SearchSellerInfo;
	storeId?: number | string;
	categoryId?: number | string;
	categoryName?: string;
	origin?: string;
	stock?: number;
	sales?: number;
}

export interface SearchSuggestion {
	keyword: string;
	count?: number;
	category?: string;
}

async function publicGet<T>(
	path: string,
	init?: {
		next?: { revalidate?: number };
		signal?: AbortSignal;
		strict?: boolean;
		/** See `RequestOptions.visitorIp` — set only by route handlers, which are
		 *  the one server context here that can see the visitor (backend#2264). */
		visitorIp?: string | null;
	},
): Promise<T | null> {
	try {
		// The transport applies the default fetch timeout (8s) on both server and
		// client, so a server fetch can no longer hang the SSR stream (React #419).
		const { data } = await apiClient.public.get<T>(path, {
			next: init?.next,
			signal: init?.signal,
			visitorIp: init?.visitorIp,
			credentials: "omit",
			throwOnHttpError: init?.strict,
			throwOnEnvelopeError: init?.strict,
		});
		if (init?.strict && data == null) throw new Error("public payload missing");
		return data ?? null;
	} catch (error) {
		if (init?.strict) throw error;
		return null;
	}
}

export async function searchProducts({
	q,
	categoryId,
	priceMin,
	priceMax,
	sortBy,
	sortOrder,
	page = 1,
	pageSize = 20,
	visitorIp,
	signal,
	strict = false,
}: {
	q: string;
	categoryId?: number | string;
	priceMin?: number;
	priceMax?: number;
	sortBy?: string;
	sortOrder?: "asc" | "desc" | string;
	page?: number;
	pageSize?: number;
	/** Set by the `/api/public/search/products` route handler (backend#2264). */
	visitorIp?: string | null;
	/** Client queries consume cancellation; server callers retain their fallback. */
	signal?: AbortSignal;
	strict?: boolean;
}): Promise<{ records: SearchProductHit[]; total: number }> {
	if (!q.trim()) return { records: [], total: 0 };
	const qs = new URLSearchParams({
		q,
		page: String(page),
		page_size: String(pageSize),
	});
	if (categoryId) qs.set("category_id", String(categoryId));
	if (priceMin != null) qs.set("price_min", String(priceMin));
	if (priceMax != null) qs.set("price_max", String(priceMax));
	if (sortBy) qs.set("sort_by", sortBy);
	if (sortOrder) qs.set("sort_order", sortOrder);
	// 分頁 key 是 records（§16），四個計數以 string 過線（§4）——Number() 是
	// 型別轉換，不是相容分支。
	const data = await publicGet<{
		records?: SearchProductHit[];
		total?: string;
	}>(`/public/search/products?${qs}`, { visitorIp, signal, strict });
	return {
		records: data?.records ?? [],
		total: Number(data?.total ?? 0),
	};
}

export async function fetchSuggestions(
	q: string,
	/** Set by the `/api/public/search/suggest` route handler (backend#2264). */
	visitorIp?: string | null,
	/** The popover BFF must distinguish an outage from a successful empty list. */
	options?: { strict?: boolean; signal?: AbortSignal },
): Promise<SearchSuggestion[]> {
	if (!q.trim()) return [];
	const data = await publicGet<unknown>(
		`/public/search/suggest?q=${encodeURIComponent(q)}`,
		{ visitorIp, ...options },
	);
	// Backend returns a plain string[] (e.g. ["123","番茄"]); also tolerate a
	// { suggestions: [...] } wrapper and richer {keyword,category} objects in
	// case the endpoint grows. Normalise to SearchSuggestion and drop blanks /
	// dupes so the popover's `key={s.keyword}` stays defined and unique.
	const raw: unknown[] = Array.isArray(data)
		? data
		: ((data as { suggestions?: unknown[] })?.suggestions ?? []);
	const seen = new Set<string>();
	const out: SearchSuggestion[] = [];
	for (const item of raw) {
		const keyword =
			typeof item === "string"
				? item
				: typeof (item as SearchSuggestion)?.keyword === "string"
					? (item as SearchSuggestion).keyword
					: "";
		const trimmed = keyword.trim();
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(
			typeof item === "string"
				? { keyword: trimmed }
				: { ...(item as SearchSuggestion), keyword: trimmed },
		);
	}
	return out;
}

/* ============================================================
 *           Trending / search hashtags
 *
 * Backend exposes post hashtags through `/public/tags/*`.
 * Product hashtags are a separate namespace (`/public/hashtags/*`) and
 * should not drive feed filtering.
 * ============================================================ */

export interface SearchHashtagHit {
	name: string;
	label?: string;
	postCount: number;
	latestPostAt?: number;
}

interface TrendingHashtagRecord {
	hashtag?: string;
	name?: string;
	label?: string;
	count?: string | number;
	post_count?: string | number;
	postCount?: string | number;
	latest_post_at?: number;
}

function daysWindow(days: number) {
	const end = new Date();
	const start = new Date(end);
	start.setDate(end.getDate() - Math.max(days, 1));
	const fmt = (date: Date) => date.toISOString().slice(0, 10);
	return { startFrom: fmt(start), endTo: fmt(end) };
}

export async function fetchTrendingHashtags(
	limit = 8,
	days = 7,
): Promise<SearchHashtagHit[]> {
	const { startFrom, endTo } = daysWindow(days);
	const qs = new URLSearchParams({
		start_from: startFrom,
		end_to: endTo,
		limit: String(limit),
	});
	const data = await publicGet<{
		items?: TrendingHashtagRecord[];
		records?: TrendingHashtagRecord[];
	}>(`/public/tags/trending?${qs}`);
	const records = data?.items ?? data?.records ?? [];
	return records.flatMap((tag) => {
		const name = tag.hashtag || tag.name;
		if (!name) return [];
		return {
			name,
			label: tag.label || `#${name}`,
			postCount: Number(tag.count ?? tag.post_count ?? tag.postCount ?? 0),
			latestPostAt: tag.latest_post_at,
		};
	});
}

/* ============================================================
 *           Popular product hashtags
 *
 * `/public/hashtags/popular?startFrom=&endTo=&limit=` ranks hashtags
 * by distinct listed-product count over an inclusive date range. Used
 * by the Hawkr 市集 megamenu column and the /market/products sidebar —
 * semantics are "most-tagged products", not "most-active posts" (the
 * older /public/tags/trending endpoint covers that case and is still
 * used by the Hawkr 動態 navbar column).
 * ============================================================ */

export interface PopularHashtagHit {
	name: string;
	label: string;
	count: number;
}

interface PopularHashtagRecord {
	hashtag?: string;
	count?: string;
}

export async function fetchPopularHashtags(options?: {
	startFrom?: string;
	endTo?: string;
	limit?: number;
	/** Cache callers (getCachedPopularHashtags) pass strict so a transport failure
	 *  throws instead of caching an empty tag list as fresh; the client search
	 *  header (searchHashtags) stays soft (empty list on failure). */
	strict?: boolean;
}): Promise<PopularHashtagHit[]> {
	const qs = new URLSearchParams();
	if (options?.startFrom) qs.set("startFrom", options.startFrom);
	if (options?.endTo) qs.set("endTo", options.endTo);
	if (options?.limit != null) qs.set("limit", String(options.limit));
	const path = qs.toString()
		? `/public/hashtags/popular?${qs}`
		: "/public/hashtags/popular";
	// Popular tags shift slowly. This fetcher is uncached itself; the fixed-limit
	// server callers (navbar / market sidebar) read it via
	// `getCachedPopularHashtags` (lib/api/cached) so the search header doesn't
	// pay a backend round-trip per render.
	const data = await publicGet<{ records?: PopularHashtagRecord[] }>(path, {
		strict: options?.strict,
	});
	const records = data?.records ?? [];
	return records.flatMap((tag) => {
		if (!tag.hashtag) return [];
		return {
			name: tag.hashtag,
			label: `#${tag.hashtag}`,
			count: Number(tag.count ?? 0),
		};
	});
}

/**
 * Hashtag chips shown above the search results — these are **product**
 * hashtags (`/public/hashtags/popular`, ranked by distinct listed-product
 * count), not post hashtags. The search surface filters products, so the
 * chips that bias the user's next query must come from the same source as
 * the product grid below. The older post-side `/public/tags/trending`
 * + `/public/tags/suggest` flow was misleading because it surfaced tags
 * that no live product carried (e.g. `#好吃包` with zero matches).
 *
 * Backend has no `q=` filter on `/public/hashtags/popular` today, so for
 * a search-with-keyword we pull a wider list (limit 40) and substring-
 * match on the client. Empty-state just returns the top popular.
 */
export async function searchHashtags(q: string): Promise<SearchHashtagHit[]> {
	const keyword = q.trim().replace(/^#+/, "").toLowerCase();
	const popular = await fetchPopularHashtags({
		limit: keyword ? 40 : 10,
	});
	const matched = keyword
		? popular.filter((tag) => tag.name.toLowerCase().includes(keyword))
		: popular;
	return matched.map((tag) => ({
		name: tag.name,
		label: tag.label,
		postCount: tag.count,
	}));
}
