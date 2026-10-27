/**
 * The rules the product page and the store page share when they generate their
 * <title> and <meta description> (ADR 2026-10-09-store-seo-platform-generated
 * D1: "same rule as the product page" is guaranteed by both importing this
 * file, not by two copies that happen to agree).
 *
 * Lengths are counted in code points, never UTF-16 units: a surrogate pair
 * (emoji, rare CJK extension characters) is one character and is never split.
 */

/** Display-truncation target for a description; not a Google limit. */
export const SEO_DESCRIPTION_MAX = 160;

/** Below this many characters a text is a fragment ("AAA"), not a summary. */
export const MIN_STANDALONE_TEXT = 10;

/** Between the segments of a <title>. */
export const TITLE_SEPARATOR = " | ";

/** Length in code points. */
export function codePointLength(text: string): number {
	return Array.from(text).length;
}

export function truncateSummary(
	text: string,
	max = SEO_DESCRIPTION_MAX,
): string {
	const chars = Array.from(text.trim());
	if (chars.length <= max) return chars.join("");
	return `${chars
		.slice(0, max - 1)
		.join("")
		.trimEnd()}…`;
}

/**
 * Joins title segments: an empty segment is dropped and a segment equal to the
 * one before it is not repeated (a seller named exactly like the brand).
 */
export function joinTitleSegments(
	segments: ReadonlyArray<string | null | undefined>,
): string {
	const kept: string[] = [];
	for (const raw of segments) {
		const segment = (raw ?? "").trim();
		if (!segment || kept[kept.length - 1] === segment) continue;
		kept.push(segment);
	}
	return kept.join(TITLE_SEPARATOR);
}
