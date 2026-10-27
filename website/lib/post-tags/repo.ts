import { apiClient } from "@/lib/api/client";

export interface PostTagSuggestion {
	name: string;
	postCount: number;
}

export interface TrendingPostTag {
	name: string;
	label: string;
	count: number;
}

/** `GET /public/tags/suggest` → `handler.SearchPostTagsResponse`
 *  （`post_hashtag.go:12-21`：`items[].{name, postCount}`，全 camelCase）。 */
interface SuggestResponse {
	items?: Array<{
		name?: string;
		postCount?: string | number;
	}>;
}

/** `GET /public/tags/trending` → `handler.TrendingPostTagsResponse`
 *  （`post_hashtag.go:74-84`：`items[].{hashtag, count}`）。
 *  沒有 `records` 外層，也沒有 `label` —— 顯示標籤一律由 `hashtag` 推導。 */
interface TrendingResponse {
	items?: Array<{ hashtag?: string; count?: string | number }>;
}

export const POST_TAG_LIMIT = 10;

export function normalizePostTag(raw: string | null | undefined): string {
	return String(raw ?? "")
		.trim()
		.replace(/^#+/, "")
		.trim();
}

export function formatPostTag(raw: string): string {
	const tag = normalizePostTag(raw);
	return tag ? `#${tag}` : "";
}

export function uniquePostTags(
	tags: readonly string[] | null | undefined,
): string[] {
	const seen = new Set<string>();
	const result: string[] = [];
	for (const raw of tags ?? []) {
		const tag = normalizePostTag(raw);
		if (!tag) continue;
		const key = tag.toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		result.push(tag);
	}
	return result;
}

export function extractPostTags(
	text: string,
	limit = POST_TAG_LIMIT,
): string[] {
	const result: string[] = [];
	const seen = new Set<string>();
	for (const match of text.matchAll(/#([\p{L}\p{N}_]+)/gu)) {
		const tag = normalizePostTag(match[1]);
		if (!tag) continue;
		const key = tag.toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		result.push(tag);
		if (result.length >= limit) break;
	}
	return result;
}

export interface PostTaggingMode {
	hashAt: number;
	prefix: string;
}

export function detectPostTaggingMode(
	textBeforeCaret: string,
	triggerAfterChars = 1,
): PostTaggingMode | null {
	if (!textBeforeCaret) return null;
	for (let i = textBeforeCaret.length - 1; i >= 0; i -= 1) {
		const ch = textBeforeCaret[i];
		if (ch === "#") {
			if (i > 0) {
				const prev = textBeforeCaret[i - 1];
				if (prev === "#" || isPostTagChar(prev)) return null;
			}
			const prefix = textBeforeCaret.slice(i + 1);
			if (prefix.length < triggerAfterChars) return null;
			return { hashAt: i, prefix };
		}
		if (!isPostTagChar(ch)) return null;
	}
	return null;
}

function isPostTagChar(ch: string): boolean {
	return /^[\p{L}\p{N}_]$/u.test(ch);
}

async function publicGet<T>(
	path: string,
	init?: { signal?: AbortSignal },
): Promise<T | null> {
	try {
		// Autocomplete/suggest — keep no-store (always fresh); anonymous → omit.
		const { data } = await apiClient.public.get<T>(path, {
			credentials: "omit",
			cache: "no-store",
			signal: init?.signal,
			throwOnHttpError: false,
			throwOnEnvelopeError: false,
		});
		return data ?? null;
	} catch {
		return null;
	}
}

export async function suggestPostTags({
	q,
	mode = "prefix",
	limit = 10,
	signal,
}: {
	q: string;
	mode?: "prefix" | "contain";
	limit?: number;
	signal?: AbortSignal;
}): Promise<PostTagSuggestion[]> {
	const tag = normalizePostTag(q);
	if (!tag) return [];
	const qs = new URLSearchParams({
		q: tag,
		mode,
		limit: String(Math.min(Math.max(limit, 1), 20)),
	});
	const data = await publicGet<SuggestResponse>(`/public/tags/suggest?${qs}`, {
		signal,
	});
	return (data?.items ?? []).flatMap((item) => {
		const name = normalizePostTag(item.name);
		if (!name) return [];
		return {
			name,
			postCount: Number(item.postCount ?? 0) || 0,
		};
	});
}

export async function fetchTrendingPostTags({
	limit = 20,
	startFrom,
	endTo,
	signal,
}: {
	limit?: number;
	startFrom?: string;
	endTo?: string;
	signal?: AbortSignal;
} = {}): Promise<TrendingPostTag[]> {
	const qs = new URLSearchParams({
		limit: String(Math.min(Math.max(limit, 1), 50)),
	});
	if (startFrom) qs.set("start_from", startFrom);
	if (endTo) qs.set("end_to", endTo);
	const data = await publicGet<TrendingResponse>(
		`/public/tags/trending?${qs}`,
		{
			signal,
		},
	);
	return (data?.items ?? []).flatMap((item) => {
		const name = normalizePostTag(item.hashtag);
		if (!name) return [];
		return {
			name,
			label: `#${name}`,
			count: Number(item.count ?? 0) || 0,
		};
	});
}
