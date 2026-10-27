/**
 * The title and description every SEO surface of a store page quotes — the
 * <title>, <meta description>, og:title and og:description — generated from
 * what the store page itself shows. There is no hand-written override: a
 * seller who wants a better search result edits the visible name or intro
 * (ADR 2026-10-09-store-seo-platform-generated).
 *
 * Same rules as the product page (lib/seo/product-summary.ts), shared through
 * lib/seo/summary-rules.ts:
 *   1. the store's own intro when it says enough to stand alone, cut to
 *      SEO_DESCRIPTION_MAX;
 *   2. otherwise the facts the record states — location, rating — joined
 *      behind the name.
 * Nothing is invented, and a missing field is left out, never replaced by an
 * empty string or a placeholder.
 */

import type { AppLocale } from "@/lib/intl/locales";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import { siteNameFor } from "@/lib/seo/metadata";
import {
	codePointLength,
	joinTitleSegments,
	MIN_STANDALONE_TEXT,
	truncateSummary,
} from "@/lib/seo/summary-rules";

export interface StoreSeoSource {
	name?: string | null;
	/** The store intro as the store page shows it. */
	description?: string | null;
	/** County / city name, e.g. 花蓮縣. */
	county?: string | null;
	/** Township / district name, e.g. 壽豐鄉. */
	town?: string | null;
	ratingAverage?: unknown;
	reviewCount?: unknown;
}

function clean(value: string | null | undefined): string {
	return (value ?? "").replace(/\s+/g, " ").trim();
}

function location(source: StoreSeoSource): string {
	return `${clean(source.county)}${clean(source.town)}`;
}

/**
 * `{店名} | {縣市}{鄉鎮} | {站名}`: the shared title rule (empty segments
 * dropped, equal neighbours not repeated, same separator as the product
 * title), with the site name of the page's locale. Callers must pass it
 * through `{ absolute }` and reuse it for og:title, or the root title.template
 * appends a second brand.
 */
export function storeSeoTitle(
	source: StoreSeoSource,
	locale: AppLocale,
): string {
	return joinTitleSegments([
		source.name,
		location(source),
		siteNameFor(locale),
	]);
}

/** Undefined when the store states nothing quotable (callers omit the tag). */
export function storeSeoDescription(
	source: StoreSeoSource,
): string | undefined {
	const name = clean(source.name);
	const intro = clean(source.description);
	if (codePointLength(intro) >= MIN_STANDALONE_TEXT) {
		return truncateSummary(intro);
	}

	const facts: string[] = [];
	const place = location(source);
	if (place) facts.push(`位於 ${place}`);
	const rating = parseRating(source.ratingAverage);
	if (rating != null) {
		const count = parseReviewCount(source.reviewCount);
		facts.push(
			count > 0
				? `評分 ${rating.toFixed(1)}（${count} 則評價）`
				: `評分 ${rating.toFixed(1)}`,
		);
	}
	if (intro) facts.push(intro);

	if (!facts.length) return name || undefined;
	return truncateSummary(
		name ? `${name}｜${facts.join("｜")}` : facts.join("｜"),
	);
}
