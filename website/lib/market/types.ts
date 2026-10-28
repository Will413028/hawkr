export interface MarketProductItem {
	id: string;
	name: string;
	image: string;
	sellerName: string;
	/** Seller avatar/logo URL (raw backend path; resolved via <Avatar>). */
	sellerAvatar?: string;
	sellerId?: string;
	/** Seller's canonical path segment (`store_code`), when the source wire
	 *  carries one. Feeds `sellerPath()` so the card links the canonical
	 *  storefront URL instead of eating a 308 on every tap (#365). */
	sellerStoreCode?: string;
	origin: string;
	price: string;
	originalPrice?: string;
	isOnSale: boolean;
	hashtags: string[];
	/** Leaf category id the seller filed the product under (lv2 on the list
	 *  wire, lv3 where one was picked). Roll it up through
	 *  `lib/market/categoryGroups` — the wire carries no lv1. */
	categoryId?: string;
	categoryName?: string;
	unit: string;
	rating: number | null;
	reviews: number;
	href: string;
	/**
	 * Raw numeric values used by the client-side sort fallback below. Kept
	 * separate from formatted display strings so sorting stays numeric.
	 */
	priceValue: number;
	createdAt: number;
	salesCount: number;
	/** 剩餘庫存（SKU 加總）。Undefined when a source doesn't carry it. */
	stock?: number;
	/** Whether the current buyer has favourited this product. */
	isFavorited?: boolean;
	/** 被收藏數 — per-product favourite total. */
	favoriteCount?: number;
	/** Whether the current buyer has an ACTIVE restock subscription. */
	isRestockSubscribed?: boolean;
}
