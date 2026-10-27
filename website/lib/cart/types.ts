/**
 * Cart wire types — match openapi `/public/carts` shapes (handler.CartListResponse).
 * Quantities/prices ship as strings; convert at the UI edge when rendering totals.
 *
 * Backend item fields (handler.CartItemResponse):
 *   skuId, productId, name, price, quantity, status, imageKey, imageUrl,
 *   specName, deliveryTemps[], stockQty
 *
 * Backend group fields (handler.CartGroupResponse):
 *   storeId, sellerName, logo_url, sellerCity, sellerDistrict, items[]
 */

export interface CartItemWire {
	skuId: string;
	name: string;
	price: string;
	quantity: string;
	status?: string;
	/** §231 flat pair: `imageKey` is the objectKey SSOT, `imageUrl` its
	 *  Gateway-composed twin. Only the key is read. */
	imageKey?: string;
	imageUrl?: string;
	specName?: string;
	productId?: string | number;
	deliveryTemps?: string[];
	stockQty?: string;
	// row 級可購買性（backend issue #1127）。purchasable=false 時前端反灰 + disable checkbox；
	// unavailableReason ∈ {DELISTED, OUT_OF_STOCK, INSUFFICIENT_STOCK, LOOKUP_FAILED}
	purchasable?: boolean;
	unavailableReason?: string;
	// Legacy local-cache fields (older entries written before the wire
	// rename); kept readable so existing localStorage payloads still hydrate.
	image?: string;
	unit?: string;
	href?: string;
}

export interface CartGroupWire {
	storeId?: string;
	/** Canonical storefront path segment (backend `586ac3ea7`). */
	storeCode?: string;
	sellerName: string;
	/** B-238 canonical seller-logo display URL. */
	logoUrl?: string;
	sellerCity?: string;
	sellerDistrict?: string;
	items: CartItemWire[];
}

export interface CartSummary {
	groups: CartGroupWire[];
	totalItems: number;
	estimatedTotalAmount: string;
}

/** Normalized item used by the UI components (always has numeric quantity). */
export interface CartItem {
	id: string; // = skuId
	skuId: string;
	productId?: string | number;
	name: string;
	price: string;
	priceValue: number;
	listPrice?: number;
	quantity: number;
	status?: string;
	image?: string;
	unit?: string;
	sellerName?: string;
	sellerStoreId?: string;
	/** Seller's canonical path segment (`store_code`) — feeds `sellerPath` so
	 *  the 逛店 link points at the canonical storefront URL (#365). */
	sellerStoreCode?: string;
	sellerLogoUrl?: string;
	sellerCity?: string;
	sellerDistrict?: string;
	href?: string;
	deliveryTemps?: string[];
	stockQty?: number;
	purchasable?: boolean;
	unavailableReason?: string;
}

export function parsePriceValue(price?: string): number {
	if (!price) return 0;
	const numeric = String(price).replace(/[^\d.]/g, "");
	return Number(numeric || 0);
}

/** Flatten the wire `groups[]` into a single CartItem[] for legacy consumers. */
export function flattenGroups(groups: CartGroupWire[]): CartItem[] {
	const out: CartItem[] = [];
	for (const g of groups) {
		for (const it of g.items) {
			const productId = it.productId;
			const href =
				it.href ??
				(productId != null && productId !== ""
					? `/products/${productId}`
					: undefined);
			const stock = Number(it.stockQty);
			const rawList = (it as unknown as Record<string, unknown>).listPrice;
			const listPrice = Number(String(rawList ?? "").replace(/[^\d.]/g, ""));
			out.push({
				id: it.skuId,
				skuId: it.skuId,
				productId,
				name: it.name,
				price: it.price,
				priceValue: parsePriceValue(it.price),
				listPrice:
					Number.isFinite(listPrice) && listPrice > 0 ? listPrice : undefined,
				quantity: Number(it.quantity) || 0,
				status: it.status,
				// Wire → `imageKey` (§231). `it.image` is the legacy LOCAL-CACHE field, not a
				// backend guess: guest carts persisted before the rename still hydrate from it.
				image: it.imageKey ?? it.image,
				unit: it.specName ?? it.unit,
				sellerName: g.sellerName,
				sellerStoreId: g.storeId,
				sellerStoreCode: g.storeCode?.trim() || undefined,
				sellerLogoUrl: g.logoUrl?.trim() || undefined,
				sellerCity: g.sellerCity,
				sellerDistrict: g.sellerDistrict,
				href,
				deliveryTemps: it.deliveryTemps,
				stockQty: Number.isFinite(stock) ? stock : undefined,
				// 後端缺欄位時（舊版 backend / 訪客舊本地快取）forward-compat：default purchasable=true
				purchasable: it.purchasable !== false,
				unavailableReason: it.unavailableReason,
			});
		}
	}
	return out;
}

/**
 * Group a flat CartItem[] back into seller-keyed buckets — used when the
 * source is the legacy localStorage shape (which has no group structure).
 */
export function groupBySeller(items: CartItem[]): CartGroupWire[] {
	const map = new Map<string, CartGroupWire>();
	for (const it of items) {
		const key = it.sellerStoreId ?? it.sellerName ?? "unknown";
		const existing = map.get(key);
		const wireItem: CartItemWire = {
			skuId: it.skuId,
			name: it.name,
			price: it.price,
			quantity: String(it.quantity),
			status: it.status,
			imageUrl: it.image,
			specName: it.unit,
			productId: it.productId,
			href: it.href,
			deliveryTemps: it.deliveryTemps,
			stockQty:
				typeof it.stockQty === "number" ? String(it.stockQty) : undefined,
		};
		if (existing) {
			existing.items.push(wireItem);
		} else {
			map.set(key, {
				storeId: it.sellerStoreId ?? "",
				storeCode: it.sellerStoreCode,
				sellerName: it.sellerName ?? "",
				items: [wireItem],
			});
		}
	}
	return Array.from(map.values());
}
