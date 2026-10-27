import type { PopularHashtagHit } from "@/lib/search/repo";

export const POPULAR_HASHTAG_LIMIT = 12;

export interface PopularSearchItem {
	name: string;
	href: string;
	source: "popular-hashtag";
}

function cleanName(value: string) {
	return value.trim().replace(/^#+/, "");
}

/**
 * Converts the ranked product hashtags into homepage discovery links.
 * The endpoint ranks hashtags, not free-text queries, so live items must
 * enter the market's hashtag filter instead of pretending to be keyword
 * searches. An unavailable or empty endpoint stays empty rather than
 * presenting invented keywords as live popularity data.
 */
export function buildPopularSearchItems(
	popularHashtags?: PopularHashtagHit[] | null,
): PopularSearchItem[] {
	const seen = new Set<string>();
	const liveItems = (popularHashtags ?? []).flatMap((tag) => {
		const name = cleanName(tag.name);
		const key = name.toLocaleLowerCase("zh-TW");
		if (!name || seen.has(key)) return [];
		seen.add(key);
		return {
			name,
			href: `/market/products?hashtags=${encodeURIComponent(name)}`,
			source: "popular-hashtag" as const,
		};
	});

	return liveItems.slice(0, POPULAR_HASHTAG_LIMIT);
}

export function buildPopularSearchPlaceholder(items: PopularSearchItem[]) {
	const examples = items.slice(0, 3).map((item) => item.name);
	return examples.length > 0
		? `搜尋${examples.join("、")}...`
		: "搜尋新鮮食材...";
}
