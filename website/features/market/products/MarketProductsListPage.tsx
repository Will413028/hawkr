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

import JsonLd from "@/components/seo/JsonLd";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import { loadMarketProductsSection } from "@/features/market/components/MarketProductsSection";
import {
	getCachedCategoryTree,
	getCachedPopularHashtags,
	getCachedRecommendedSellers,
} from "@/lib/api/cached";
import { type AppLocale, defaultLocale } from "@/lib/intl/locales";
import { deriveMarketListView } from "@/lib/market/deriveListView";
import type { CategoryNode } from "@/lib/market/listFilters";
import { recommendedSellerLogoUrl } from "@/lib/seller/recommended-logo";
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
	// Every sidebar / rail fetch is wrapped so a single backend hiccup renders an
	// empty section instead of crashing the whole page. All are param-independent
	// and cached, keeping this render static.
	const [categories, trendingHashtags, recommendedSellers] = await Promise.all([
		getCachedCategoryTree().catch(() => [] as CategoryNode[]),
		getCachedPopularHashtags(8).catch(() => []),
		getCachedRecommendedSellers(12).catch(() => []),
	]);
	// Populate city / lat / lng so the 附近商家 tab in FeedStories can sort the
	// rail by browser location (same shape as the feed page).
	const sellerStories: StoryItem[] = (
		recommendedSellers as Array<Record<string, unknown>>
	)
		.filter((seller) => seller?.storeId && seller?.storeName)
		.map((seller) => {
			// `/public/sellers/recommended` 的 record 沒有座標欄
			// （recommended_sellers.go:40-54 的 gin.H 就是全部欄位）——這裡讀
			// `latitude`/`lng` 是為了與 feed rail 共用同一個 StoryItem 形狀，
			// 目前恆為 undefined。
			const latRaw = seller.latitude ?? seller.lat ?? null;
			const lngRaw = seller.longitude ?? seller.lng ?? null;
			const latitude =
				typeof latRaw === "number"
					? latRaw
					: typeof latRaw === "string" && latRaw
						? Number(latRaw)
						: undefined;
			const longitude =
				typeof lngRaw === "number"
					? lngRaw
					: typeof lngRaw === "string" && lngRaw
						? Number(lngRaw)
						: undefined;
			// `/public/sellers/recommended` 自 backend#2235 起全欄 camelCase
			// （`recommended_sellers.go:40-42`：storeId / storeCode / storeName）。
			// StoryRail 拿 `storeCode` 走 `sellerPath()`；讀錯名字會靜默 fallback
			// 回 storeId，代價是推薦商家 rail 每點一次多一個 308（#365）。
			// rows 是 Record<string, unknown>，讀錯名字**不會**型別錯誤——只會
			// 恆為 undefined，所以這條靠 tests/seller-link-canonical 釘住。
			const storeCode =
				typeof seller.storeCode === "string" ? seller.storeCode.trim() : "";
			return {
				id: String(seller.storeId),
				storeCode: storeCode || undefined,
				name: String(seller.storeName),
				avatar: recommendedSellerLogoUrl(seller),
				city: typeof seller.city === "string" ? seller.city : undefined,
				district:
					typeof seller.district === "string" ? seller.district : undefined,
				latitude:
					latitude !== undefined && Number.isFinite(latitude)
						? latitude
						: undefined,
				longitude:
					longitude !== undefined && Number.isFinite(longitude)
						? longitude
						: undefined,
			} satisfies StoryItem;
		})
		.slice(0, 12);
	const categoryList = Array.isArray(categories)
		? (categories as CategoryNode[])
		: [];
	const trendingHashtagList = trendingHashtags as Array<{
		name: string;
		label?: string;
	}>;

	// Canonical (no-param) view — the SEO body shipped in the ISR HTML. Derived
	// with the same pure function the client re-runs against the real URL params.
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
		<main className="market-page market-public-page w-[min(1440px,calc(100%_-_48px))] mx-auto pt-[var(--page-start-gap,_6px)] pb-[60px]! max-[768px]:w-[min(100%_-_28px,680px)]">
			{productUrls.length > 0 && <JsonLd data={itemListSchema(productUrls)} />}
			<MarketProductsListClient
				categoryList={categoryList}
				sellerStories={sellerStories}
				trendingHashtags={trendingHashtagList}
				pageSize={INITIAL_PAGE_SIZE}
				defaultProducts={initialProducts}
				defaultInitialDone={initialDone}
			/>
		</main>
	);
}
