import type { MarketProductItem } from "@/lib/market/types";

export interface MarketHeadPage {
	products: MarketProductItem[];
	page: number;
	hasMore: boolean;
}

function withFreshCommerceFields(
	existing: MarketProductItem,
	fresh: MarketProductItem,
): MarketProductItem {
	return {
		...existing,
		price: fresh.price,
		originalPrice: fresh.originalPrice,
		isOnSale: fresh.isOnSale,
		priceValue: fresh.priceValue,
		stock: fresh.stock,
		salesCount: fresh.salesCount,
		favoriteCount: fresh.favoriteCount,
		rating: fresh.rating,
		reviews: fresh.reviews,
	};
}

function dedupe(products: MarketProductItem[]) {
	const seen = new Set<string>();
	return products.filter((product) => {
		if (seen.has(product.id)) return false;
		seen.add(product.id);
		return true;
	});
}

function presentationFingerprint(product: MarketProductItem): string {
	const {
		favoriteCount: _favoriteCount,
		isFavorited: _isFavorited,
		isOnSale: _isOnSale,
		isRestockSubscribed: _isRestockSubscribed,
		originalPrice: _originalPrice,
		price: _price,
		priceValue: _priceValue,
		rating: _rating,
		reviews: _reviews,
		salesCount: _salesCount,
		stock: _stock,
		...presentation
	} = product;
	return JSON.stringify(presentation);
}

export function marketHeadRequiresReplacement(
	existingProducts: MarketProductItem[],
	freshProducts: MarketProductItem[],
): boolean {
	if (existingProducts.length !== freshProducts.length) return true;
	return existingProducts.some((product, index) => {
		const fresh = freshProducts[index];
		return (
			product.id !== fresh?.id ||
			presentationFingerprint(product) !== presentationFingerprint(fresh)
		);
	});
}

/** Anonymous list refreshes may replace public fields, but never overwrite the
 * buyer-local favourite/restock state already held in the client cache. */
export function preserveMarketBuyerState(
	freshPages: MarketHeadPage[],
	currentPages: MarketHeadPage[],
): MarketHeadPage[] {
	const currentById = new Map(
		currentPages
			.flatMap((page) => page.products)
			.map((product) => [product.id, product] as const),
	);
	return freshPages.map((page) => ({
		...page,
		products: page.products.map((fresh) => {
			const current = currentById.get(fresh.id);
			if (!current) return fresh;
			return {
				...fresh,
				isFavorited: current.isFavorited,
				isRestockSubscribed: current.isRestockSubscribed,
			};
		}),
	}));
}

/**
 * Price/stock/sale fields reconcile immediately, but order is preserved.
 * Newly ranked products stay pending until the list is at the top or the buyer
 * accepts the "new products" pill; this avoids moving a product under a tap.
 */
export function mergeMarketHeadPages(
	pages: MarketHeadPage[],
	freshProducts: MarketProductItem[],
	prependNew: boolean,
): { pages: MarketHeadPage[]; newProducts: MarketProductItem[] } {
	const existingIds = new Set(
		pages.flatMap((page) => page.products.map((product) => product.id)),
	);
	const latestById = new Map(
		freshProducts.map((product) => [product.id, product] as const),
	);
	const updatedPages = pages.map((page) => ({
		...page,
		products: page.products.map((product) => {
			const fresh = latestById.get(product.id);
			return fresh ? withFreshCommerceFields(product, fresh) : product;
		}),
	}));
	const newProducts = dedupe(
		freshProducts.filter((product) => !existingIds.has(product.id)),
	);
	if (!prependNew || newProducts.length === 0 || updatedPages.length === 0) {
		return { pages: updatedPages, newProducts };
	}

	// Keep all loaded products. Repacking into the old fixed page sizes would
	// silently discard the tail while page metadata still advances to the next
	// cursor, making those products impossible to recover without a full reset.
	return {
		pages: updatedPages.map((page, index) =>
			index === 0
				? { ...page, products: dedupe([...newProducts, ...page.products]) }
				: page,
		),
		newProducts,
	};
}
