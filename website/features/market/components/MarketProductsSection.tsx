/**
 * Server-owned product loader for `/market/products`.
 *
 * Split out of `MarketProductsListPage` so the per-filter fetch stays isolated
 * from the layout code. The route's persistent root boundary commits the
 * sidebar and grid data together; the client layout renders the grid from
 * serialisable props so no Server Component node crosses that boundary.
 *
 * The follow tab is viewer-aware (needs the buyer JWT, only reachable from
 * the BFF), so SSR skips the fetch and lets the client `MarketProductsList`
 * request page 1 on mount.
 */

import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import { fetchProducts } from "@/lib/api";
import { getCachedProductsFirstPage } from "@/lib/api/cached";
import {
	type MarketFetchParams,
	mapProduct,
	type ProductRecord,
} from "@/lib/market/listFilters";

interface MarketProductsSectionLoadOptions {
	fetchParams: MarketFetchParams;
	pageSize: number;
	isFollowTab: boolean;
}

export async function loadMarketProductsSection({
	fetchParams,
	pageSize,
	isFollowTab,
}: MarketProductsSectionLoadOptions): Promise<{
	products: MarketProductItem[];
	initialDone: boolean;
}> {
	let products: MarketProductItem[] = [];
	let initialDone = false;

	if (!isFollowTab) {
		const isCanonicalNavbarLanding =
			!fetchParams.keyword &&
			!fetchParams.categoryId &&
			!fetchParams.sort &&
			!fetchParams.hashtags?.length &&
			fetchParams.priceMin == null &&
			fetchParams.priceMax == null;

		// Cache only the canonical anonymous first page. Filter values come from
		// the URL and are intentionally fetched directly so arbitrary queries do
		// not create an unbounded OpenNext/R2 key space. Use the shared 100-item
		// pool already consumed by Home, then slice the
		// visible grid; this keeps all three surfaces on one cache key.
		const productJson = await (isCanonicalNavbarLanding
			? getCachedProductsFirstPage(100)
			: fetchProducts(
					fetchParams.keyword,
					1,
					pageSize,
					fetchParams.categoryId,
					fetchParams.sort,
					{
						hashtags: fetchParams.hashtags,
						priceMin: fetchParams.priceMin,
						priceMax: fetchParams.priceMax,
					},
				)
		).catch(() => ({ data: { records: [] as unknown[] } }));
		const fetchedRecords = (productJson?.data?.records ||
			[]) as ProductRecord[];
		const rawRecords = isCanonicalNavbarLanding
			? fetchedRecords.slice(0, pageSize)
			: fetchedRecords;
		products = rawRecords.map(mapProduct);
		// `done` when backend returned fewer than the requested page size —
		// the client uses this to disable the explicit load-more action.
		initialDone = fetchedRecords.length < pageSize;
	}
	return { products, initialDone };
}
