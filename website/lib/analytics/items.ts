/**
 * Domain type → GA4Item mappers. Centralizes the data traps:
 *  - currency is the constant TWD (site hardcodes NT$);
 *  - price comes from numeric fields / parsePriceValue, never the formatted
 *    string (which contains "NT$" + thousands separators);
 *  - item_id is a stringified int64 snowflake (precision-safe);
 *  - item_variant is the skuId (absent in list contexts → omitted).
 */

import type { CartInput } from "@/lib/cart";
import type { CartItem } from "@/lib/cart/types";
import { parsePriceValue } from "@/lib/cart/types";
import type { MarketProductItem } from "@/lib/market/types";
import type { OrderItem } from "@/lib/orders/types";
import type { GA4Item } from "./types";

/** Drop undefined keys so payloads stay clean in DebugView. */
function compact(item: GA4Item): GA4Item {
	for (const k of Object.keys(item) as (keyof GA4Item)[]) {
		if (item[k] === undefined) delete item[k];
	}
	return item;
}

export function marketItemToGA4(p: MarketProductItem, index?: number): GA4Item {
	return compact({
		item_id: String(p.id),
		item_name: p.name,
		price: p.priceValue || undefined,
		item_brand: p.sellerName || undefined,
		affiliation: p.sellerName || undefined,
		index,
	});
}

export function cartItemToGA4(it: CartItem): GA4Item {
	return compact({
		item_id: String(it.productId ?? it.id),
		item_name: it.name,
		price: it.priceValue || undefined,
		quantity: it.quantity,
		item_variant: it.skuId || undefined,
		item_brand: it.sellerName || undefined,
		affiliation: it.sellerName || undefined,
	});
}

export function cartInputToGA4(input: CartInput, quantity: number): GA4Item {
	return compact({
		item_id: String(input.productId ?? input.id),
		item_name: input.name,
		price: parsePriceValue(input.price) || undefined,
		quantity,
		item_variant: String(input.skuId ?? input.id),
		item_brand: input.sellerName || undefined,
		affiliation: input.sellerName || undefined,
	});
}

export function orderItemToGA4(it: OrderItem): GA4Item {
	return compact({
		item_id: String(it.productId),
		item_name: it.name,
		price: parsePriceValue(it.price) || undefined,
		quantity: Number(it.quantity) || undefined,
		item_variant: it.skuId || undefined,
	});
}

/** Sum of price*quantity across items, for the event-level `value`. */
export function sumValue(
	items: { price?: number; quantity?: number }[],
): number {
	return items.reduce(
		(total, it) => total + (it.price ?? 0) * (it.quantity ?? 1),
		0,
	);
}
