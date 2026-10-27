"use client";

import * as React from "react";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import { useUrlSearchParams } from "@/lib/hooks/useUrlSearchParams";
import {
	deriveMarketListView,
	type MarketListParams,
} from "@/lib/market/deriveListView";
import type { CategoryNode } from "@/lib/market/listFilters";
import MarketProductsListLayout from "./MarketProductsListLayout";

interface MarketProductsListClientProps {
	categoryList: CategoryNode[];
	sellerStories: StoryItem[];
	trendingHashtags: Array<{ name: string; label?: string }>;
	pageSize: number;
	/** Canonical (no-filter) page-1 products, server-rendered into the ISR HTML. */
	defaultProducts: MarketProductItem[];
	defaultInitialDone: boolean;
}

/**
 * Client owner of `/market/products`'s filter state.
 *
 * The route is ISR: the server renders the canonical first page (the SEO body +
 * ItemList JSON-LD) with no request-time input, and this component reads the
 * filter params from the URL — via `useUrlSearchParams`, not `useSearchParams`
 * (which would bail the route out of static rendering) — and derives the exact
 * same view the server used to produce with the same pure function. On the
 * server and the first client render the URL is param-less, so the canonical
 * view hydrates without mismatch; the real params are read on mount and on every
 * subsequent filter change, re-deriving chips / active states / hrefs and
 * handing the grid new `fetchParams` that it fetches client-side.
 */
export default function MarketProductsListClient({
	categoryList,
	sellerStories,
	trendingHashtags,
	pageSize,
	defaultProducts,
	defaultInitialDone,
}: MarketProductsListClientProps) {
	const search = useUrlSearchParams();
	const params: MarketListParams = React.useMemo(
		() => ({
			q: search.get("q") ?? undefined,
			category: search.get("category") ?? undefined,
			categories: search.get("categories") ?? undefined,
			temp: search.get("temp") ?? undefined,
			priceMin: search.get("priceMin") ?? undefined,
			priceMax: search.get("priceMax") ?? undefined,
			hashtags: search.get("hashtags") ?? undefined,
			certification_types: search.get("certification_types") ?? undefined,
			sort: search.get("sort") ?? undefined,
			tab: search.get("tab") ?? undefined,
		}),
		[search],
	);
	const view = React.useMemo(
		() =>
			deriveMarketListView(params, {
				categoryList,
				trendingHashtags,
			}),
		[params, categoryList, trendingHashtags],
	);

	// Only the canonical (no-filter, default sort, recommend tab) page carries an
	// authoritative SSR product seed. Everything else resolves its own page 1 on
	// the client:
	//   - follow tab       → seed an empty page so the grid's mount loadMore
	//                         fetches the buyer-scoped page 1 (no double fetch)
	//   - any other filter  → no seed; the grid fetches page 1 for the new key
	const isCanonical =
		!view.hasActiveFilters &&
		!view.isFollowTab &&
		view.currentSort === "推薦" &&
		view.currentTab === "recommend";
	let products = defaultProducts;
	let initialDone = defaultInitialDone;
	let seedInitialData = true;
	if (view.isFollowTab) {
		products = [];
		initialDone = false;
	} else if (!isCanonical) {
		products = [];
		initialDone = false;
		seedInitialData = false;
	}

	return (
		<MarketProductsListLayout
			categories={categoryList}
			sellerStories={sellerStories}
			currentCategories={view.currentCategories}
			currentTemp={view.currentTemp}
			currentPriceMin={view.currentPriceMin}
			currentPriceMax={view.currentPriceMax}
			currentHashtags={view.currentHashtags}
			currentCertificationTypes={view.currentCertificationTypes}
			currentSort={view.currentSort}
			currentQuery={view.currentQuery}
			currentTab={view.currentTab}
			fetchParams={view.fetchParams}
			pageSize={pageSize}
			appliedChips={view.appliedChips}
			clearAllHref={view.clearAllHref}
			sortLinks={view.sortLinks}
			tempLinks={view.tempLinks}
			certLinks={view.certLinks}
			hashtagLinks={view.hashtagLinks}
			clearTempHref={view.clearTempHref}
			products={products}
			initialDone={initialDone}
			seedInitialData={seedInitialData}
			hasActiveFilters={view.hasActiveFilters}
		/>
	);
}
