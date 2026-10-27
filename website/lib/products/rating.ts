/**
 * Product / seller rating parsing.
 *
 * The OpenAPI spec types `rating` as **string** on every product endpoint
 * (`handler.ProductItem`, `handler.ProductHitData`,
 * `handler.PublicProductDetailItem`, `handler.ProductRatingResponse`)
 * — and **number** on seller endpoints (`handler.PublicSellerInfo`,
 * `handler.SellerStats`). Both can also be missing.
 *
 * The previous frontend pattern was `Number(value || fallback)` which
 * silently turned the string `"0"` (a legitimate "no reviews yet"
 * signal) into the literal number 0, because the string `"0"` is
 * truthy in JS — so the `||` fallback never fired and rating dropped
 * to 0 for every product without reviews.
 *
 * Use these helpers to read ratings consistently:
 *   parseRating(value)  → number | null    (null = no rating)
 *   formatRating(value) → string           ("4.5" or "—")
 *   parseReviewCount(value) → number       (0 when missing/invalid)
 */

export function parseRating(value: unknown): number | null {
	if (value == null) return null;
	const n =
		typeof value === "number" ? value : Number.parseFloat(String(value));
	if (!Number.isFinite(n)) return null;
	if (n <= 0) return null;
	if (n > 5) return 5;
	return n;
}

export function formatRating(value: unknown): string {
	const n = parseRating(value);
	return n == null ? "—" : n.toFixed(1);
}

/** Human-readable aggregate summary that never invents a five-star rating. */
export function formatRatingSummary(value: unknown): string {
	const n = parseRating(value);
	return n == null ? "尚無買家評價" : `評分 ${n.toFixed(1)}`;
}

export function parseReviewCount(value: unknown): number {
	if (value == null) return 0;
	const n =
		typeof value === "number" ? value : Number.parseInt(String(value), 10);
	if (!Number.isFinite(n) || n < 0) return 0;
	return n;
}

/** True when the product has at least one review (rating > 0 + count > 0). */
export function hasRating(rating: unknown, reviewCount?: unknown): boolean {
	const r = parseRating(rating);
	if (r == null) return false;
	if (reviewCount == null) return true;
	return parseReviewCount(reviewCount) > 0;
}
