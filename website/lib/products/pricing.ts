/**
 * Product price display helper — single source of truth for the
 * "is this product on sale and what should the card render?" question.
 *
 * Background (issue #758):
 *   Backend now exposes three parallel price ranges on every product:
 *     - `priceMin / priceMax`         effective (sale price applied)
 *     - `listPriceMin / listPriceMax` original retail / strikethrough
 *     - `salePriceMin / salePriceMax` sale value, "0" when no sale
 *   The "on sale" signal is `salePriceMin !== "0"` AND
 *   `listPriceMin !== priceMin`. UI then shows `priceMin/Max` as the
 *   headline and `listPriceMin/Max` as the strikethrough.
 *
 * This helper accepts the new camelCase fields AND a legacy snake_case
 * fallback so the UI keeps working while dev / prod backends roll
 * the new schema. Each legacy read is marked with a TODO tagged to
 * the issue so the cleanup pass is a grep.
 */

export interface PriceFields {
	priceMin?: string | number | null;
	priceMax?: string | number | null;
	listPriceMin?: string | number | null;
	listPriceMax?: string | number | null;
	salePriceMin?: string | number | null;
	salePriceMax?: string | number | null;
}

export interface ProductPriceDisplay {
	/**
	 * Headline price (sale-aware). E.g. "NT$ 680" or "NT$ 680 – 1,200".
	 * Empty string when the product has no numeric price data.
	 */
	effectiveText: string;
	/**
	 * Original list price for strikethrough rendering. Only populated when
	 * `isOnSale` is true. Same format as `effectiveText`.
	 */
	originalText?: string;
	/**
	 * Pre-formatted savings amount for the "省 NT$ X" pill, e.g. "NT$ 100".
	 * Computed as `listPriceMin - priceMin`. Only populated when
	 * `isOnSale` is true.
	 */
	savingsText?: string;
	/**
	 * `true` when the product carries a positive sale price below its list
	 * price. False when sale fields are missing, zero, or equal to list.
	 */
	isOnSale: boolean;
}

function num(value: string | number | null | undefined): number {
	if (value == null) return 0;
	const n = typeof value === "number" ? value : Number(value);
	return Number.isFinite(n) && n > 0 ? n : 0;
}

// #758 的 legacy fallback 到此為止：`handler.ProductItem` / `ProductDetail`
// 的價格欄一律 camelCase（product.go:187-190、490-492），snake 分支從來沒有被
// 走到過，而它的存在讓「後端還沒穩定」這個早就不成立的前提繼續看起來成立。
function pickEffectiveMin(p: PriceFields): number {
	return num(p.priceMin);
}
function pickEffectiveMax(p: PriceFields): number {
	return num(p.priceMax);
}
function pickListMin(p: PriceFields): number {
	return num(p.listPriceMin);
}
function pickListMax(p: PriceFields): number {
	return num(p.listPriceMax);
}
function pickSaleMin(p: PriceFields): number {
	return num(p.salePriceMin);
}

/**
 * Format a min/max range into a display string. Returns "" when there's
 * no numeric data, "NT$ X" when min === max (or no max), and
 * "NT$ X – Y" otherwise. Numbers are localised with thousand separators.
 */
export function formatPriceRange(
	min: string | number | null | undefined,
	max?: string | number | null,
): string {
	const minN = num(min);
	if (!minN) return "";
	const maxN = num(max);
	if (!maxN || maxN === minN) {
		return `NT$ ${minN.toLocaleString()}`;
	}
	return `NT$ ${minN.toLocaleString()} – ${maxN.toLocaleString()}`;
}

/**
 * `true` when the product has an active sale below its list price.
 *
 * Per issue #758 the signal requires BOTH:
 *   1. salePriceMin > 0     (backend uses "0" to mean "no sale")
 *   2. listPriceMin differs from the headline priceMin
 * Either alone is insufficient — a product with salePriceMin > 0 but
 * the same list price would be a no-op sale, and listPriceMin alone
 * can't tell us whether a discount is currently active.
 */
export function hasSalePrice(product: PriceFields): boolean {
	const sale = pickSaleMin(product);
	const list = pickListMin(product);
	const effective = pickEffectiveMin(product);
	return sale > 0 && list > 0 && list !== effective;
}

/**
 * Compute the full display payload for a product card or PDP price
 * region. Caller renders `effectiveText` as the headline; if
 * `isOnSale`, also renders `originalText` with strikethrough and
 * `savingsText` inside the savings pill.
 */
export function getProductPriceDisplay(
	product: PriceFields,
): ProductPriceDisplay {
	const effectiveMin = pickEffectiveMin(product);
	const effectiveMax = pickEffectiveMax(product) || effectiveMin;
	const effectiveText = formatPriceRange(
		effectiveMin,
		effectiveMax > effectiveMin ? effectiveMax : undefined,
	);

	if (!hasSalePrice(product)) {
		return { effectiveText, isOnSale: false };
	}

	const listMin = pickListMin(product);
	const listMax = pickListMax(product) || listMin;
	const originalText = formatPriceRange(
		listMin,
		listMax > listMin ? listMax : undefined,
	);
	const savingsAmount = listMin - effectiveMin;
	const savingsText =
		savingsAmount > 0 ? `NT$ ${savingsAmount.toLocaleString()}` : undefined;

	return {
		effectiveText,
		originalText: originalText || undefined,
		savingsText,
		isOnSale: true,
	};
}
