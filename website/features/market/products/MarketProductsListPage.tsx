/**
 * Market — products listing (`/market/products`), ISR.
 *
 * The server fetches the param-independent sidebar data (categories / hashtags /
 * recommended sellers) and the canonical no-filter first
 * page, then ships that as the SEO body + ItemList JSON-LD. It reads no
 * request-time input, so the route stays statically renderable. All filter /
 * sort / tab params are read on the client by `MarketProductsListClient`, which
 * derives the view with the same pure function and fetches filtered pages
 * client-side — no per-request dynamic server render.
 */

import { Suspense } from "react";
import JsonLd from "@/components/seo/JsonLd";
import { recommendedStoriesFromStores } from "@/features/feed/recommendedStories";
import { loadMarketProductsSection } from "@/features/market/components/MarketProductsSection";
import {
	getCachedCategoryTree,
	getCachedPopularHashtags,
	getCachedRecommendedSellers,
} from "@/lib/api/cached";
import { type AppLocale, defaultLocale } from "@/lib/intl/locales";
import { deriveMarketListView } from "@/lib/market/deriveListView";
import type { CategoryNode } from "@/lib/market/listFilters";
import { itemListSchema } from "@/lib/seo/jsonld";
import { productUrl } from "@/lib/seo/urls";
import MarketProductsListClient from "./MarketProductsListClient";

const INITIAL_PAGE_SIZE = 24;

export default async function MarketProductsListPage({
	locale = defaultLocale,
}: {
	/** The locale of the page: the ItemList JSON-LD lists that locale's product URLs. */
	locale?: AppLocale;
} = {}) {
	const [categories, trendingHashtags, recommendedSellers] = await Promise.all([
		getCachedCategoryTree().catch(() => [] as CategoryNode[]),
		getCachedPopularHashtags(8).catch(() => []),
		getCachedRecommendedSellers(12).catch(() => []),
	]);
	const sellerStories = recommendedStoriesFromStores(recommendedSellers);
	const categoryList = Array.isArray(categories)
		? (categories as CategoryNode[])
		: [];
	const trendingHashtagList = trendingHashtags as Array<{
		name: string;
		label?: string;
	}>;

	const defaultView = deriveMarketListView(
		{},
		{
			categoryList,
			trendingHashtags: trendingHashtagList,
		},
	);
	const { products: initialProducts, initialDone } =
		await loadMarketProductsSection({
			fetchParams: defaultView.fetchParams,
			pageSize: INITIAL_PAGE_SIZE,
			isFollowTab: false,
		});
	const productUrls = initialProducts.map((product) =>
		productUrl(product.id, locale),
	);

	return (
		<div className="market-page market-public-page w-[min(1440px,calc(100%_-_48px))] mx-auto pt-[var(--page-start-gap,_6px)] pb-[60px]! max-[768px]:w-[min(100%_-_28px,680px)]">
			{productUrls.length > 0 && <JsonLd data={itemListSchema(productUrls)} />}
			<Suspense fallback={null}>
				<MarketProductsListClient
					categoryList={categoryList}
					sellerStories={sellerStories}
					trendingHashtags={trendingHashtagList}
					pageSize={INITIAL_PAGE_SIZE}
					defaultProducts={initialProducts}
					defaultInitialDone={initialDone}
				/>
			</Suspense>
		</div>
	);
}
