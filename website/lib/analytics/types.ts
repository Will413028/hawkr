/**
 * GA4 ecommerce types. `GA4Item` mirrors the GA4 recommended-events `items[]`
 * member; all monetary values are numeric NT$ (currency is the constant below).
 * Product/SKU ids are int64 snowflakes — always carried as strings so values
 * above 2^53 keep full precision.
 */

export const CURRENCY = "TWD";

export type AnalyticsRole = "buyer" | "seller";

export interface GA4Item {
	item_id: string;
	item_name: string;
	price?: number;
	quantity?: number;
	/** Seller display name. */
	item_brand?: string;
	/** Seller display name (GA4 affiliation slot). */
	affiliation?: string;
	/** SKU id (variant). Omitted in list contexts that have no SKU. */
	item_variant?: string;
	item_category?: string;
	item_category2?: string;
	item_category3?: string;
	index?: number;
	discount?: number;
	coupon?: string;
	item_list_id?: string;
	item_list_name?: string;
}

/** Loose param bag for a GA4 event payload. */
export type GA4EventParams = Record<string, unknown>;
