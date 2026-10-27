import { placeholders } from "../media/public-assets";
import { normalizePostTag, uniquePostTags } from "../post-tags/repo";
import { apiClient } from "./client";
import type { Paginated, PostDetail, PostListItem } from "./openapi-types";

interface FetchPostsOptions {
	page?: number;
	pageSize?: number;
	sellerId?: number | string;
	tag?: string;
	signal?: AbortSignal;
	/** Throw on transport/envelope failure so cache regeneration never records a
	 *  synthetic empty page as successful fresh data. */
	strict?: boolean;
	/** See `RequestOptions.visitorIp` (backend#2264). Set by the tab-bootstrap
	 *  route handler's direct feed read; the cached readers leave it unset. */
	visitorIp?: string | null;
}

export const FALLBACK_POST_IMAGE = placeholders.post;

/**
 * Normalize feed fields while preserving an empty canonical seller logo key.
 *
 * §231: the wire carries `imageKeys` / `logoKey` (bare objectKeys, SSOT) beside
 * the Gateway-composed `imageUrls` / `logoUrl` twins; only the keys are read here.
 * A post with no image gets the bundled local placeholder path in slot 0 —
 * `buildImageUrl` resolves an objectKey and a local `public/placeholders/` asset alike, and
 * every render site funnels through it, so downstream `imageKeys[0]` is always
 * renderable.
 */
export function normalizePost(p: PostListItem): PostListItem {
	const imageKeys = (p.imageKeys ?? [])
		.map((k) => k?.trim())
		.filter((k): k is string => Boolean(k));
	return {
		...p,
		imageKeys: imageKeys.length ? imageKeys : [FALLBACK_POST_IMAGE],
		logoKey: p.logoKey?.trim() ?? "",
		sellerName: p.sellerName?.trim() || "Hawkr 選品",
		content: p.content ?? "",
		tags: uniquePostTags(p.tags),
		product: p.product
			? {
					...p.product,
					imageKey: p.product.imageKey?.trim() || FALLBACK_POST_IMAGE,
					name: p.product.name?.trim() || "商品",
					price: p.product.price?.trim() || "",
				}
			: null,
	};
}

export async function fetchPublicPostsTyped({
	page = 1,
	pageSize = 12,
	sellerId,
	tag,
	signal,
	strict = false,
	visitorIp,
}: FetchPostsOptions = {}): Promise<Paginated<PostListItem>> {
	const query: Record<string, string> = {
		page: String(page),
		page_size: String(pageSize),
	};
	const normalizedTag = normalizePostTag(tag);
	if (normalizedTag) query.tag = normalizedTag;
	const path = sellerId
		? `/sellers/${encodeURIComponent(String(sellerId))}/posts`
		: "/posts";
	try {
		// No explicit cache directive: the browser path is always no-store
		// (lib/api/client.ts); on the server, getCachedPublicPosts wraps this in
		// an unstable_cache entry that owns caching + tag invalidation at its
		// layer, so the inner fetch stays directive-free.
		const { data } = await apiClient.public.get<Paginated<PostListItem>>(path, {
			query,
			signal,
			visitorIp,
			credentials: "omit",
			throwOnHttpError: strict,
			throwOnEnvelopeError: strict,
		});
		if (strict && data == null) throw new Error("posts payload missing");
		const records = (data?.records ?? []).map(normalizePost).filter((post) => {
			if (!normalizedTag || !sellerId) return true;
			return post.tags.some(
				(t) => t.toLowerCase() === normalizedTag.toLowerCase(),
			);
		});
		return {
			page: data?.page ?? page,
			pageSize: data?.pageSize ?? pageSize,
			total: data?.total ?? 0,
			records,
		};
	} catch (error) {
		if (strict) throw error;
		return { page, pageSize, total: 0, records: [] };
	}
}

/**
 * Picks the N most-viewed posts out of the latest batch — drives the
 * Hawkr 動態 megamenu's "本週主題" carousel. Backend has no dedicated
 * "popular this week" endpoint, so we fetch the latest `pool` posts
 * (recency-ordered) and re-rank by `viewCount`. The result skews
 * recent by construction, which is what we want for a weekly accent.
 *
 * Returns at most `count` posts; empty array on fetch failure.
 */
export async function fetchTopWeeklyPosts(
	count = 5,
	pool = 30,
): Promise<PostListItem[]> {
	const batch = await fetchPublicPostsTyped({ page: 1, pageSize: pool });
	if (!batch.records.length) return [];
	return [...batch.records]
		.sort((a, b) => (b.viewCount ?? 0) - (a.viewCount ?? 0))
		.slice(0, count);
}

/** 詳情回應的欄位正規化：公開與店主兩條讀取共用。 */
export function normalizePostDetail<T extends PostDetail>(data: T): T {
	return {
		...data,
		logoKey: data.logoKey?.trim() ?? "",
		content: data.content ?? "",
		imageKeys: data.imageKeys ?? [],
		products: data.products ?? [],
		tags: uniquePostTags(data.tags),
	};
}

export async function fetchPostDetail(
	id: number | string,
): Promise<PostDetail | null> {
	try {
		const { data } = await apiClient.public.get<PostDetail>(`/posts/${id}`, {
			cache: "no-store",
			credentials: "omit",
			throwOnHttpError: false,
			throwOnEnvelopeError: false,
		});
		if (!data) return null;
		return normalizePostDetail(data);
	} catch {
		return null;
	}
}

/**
 * Mutations: route through the gateway's toggle endpoint (`/posts/{id}/like`)
 * using the active session via authedFetch. The legacy `unlikePost` is kept
 * as an alias of `likePost` (toggle semantics) so callers compile without
 * changes; per-direction servers no longer exist.
 */
import { togglePostLike } from "../social/repo";

export async function likePost(id: number | string): Promise<boolean> {
	try {
		await togglePostLike(id);
		return true;
	} catch {
		return false;
	}
}

/** Toggle like — kept under this name for back-compat with PostDetailViewer. */
export async function unlikePost(id: number | string): Promise<boolean> {
	return likePost(id);
}

/** Record a share and let the backend bump the counter. */
export async function sharePost(id: number | string): Promise<boolean> {
	try {
		const { sharePost: recordShare } = await import("../social/repo");
		await recordShare(id);
		return true;
	} catch {
		return false;
	}
}
