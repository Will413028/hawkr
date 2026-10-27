"use client";

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import Link from "@/components/shared/Link";
import FeedStories, {
	FEED_TABS,
	type StoryTabKey,
} from "@/features/feed/components/FeedStories";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import {
	fetchMarketProductsPage,
	marketProductsListQueryKey,
	nextMarketProductsPage,
} from "@/lib/hooks/useMarketProducts";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import type { MarketFetchParams } from "@/lib/market/listFilters";
import { updateUrlParams } from "@/lib/navigation/url-params";

interface MarketClientShellProps {
	stories: StoryItem[];
	/**
	 * Server-rendered sidebar passed through as a React node. It carries
	 * server data (categories, hashtag chips) and stays
	 * SSR; this shell just lays it out alongside the product grid.
	 */
	sidebar: React.ReactNode;
	/**
	 * Streamed product grid passed as a node slot (an async server section
	 * wrapped in `<Suspense>`). Rendering it as a slot lets the sidebar paint
	 * immediately while the product fetch resolves behind a skeleton.
	 */
	productGrid: React.ReactNode;
	/** Tab the SSR rendered for. Source of truth lives in URL (`?tab=...`)
	 * so deep links + refresh stay in the same tab; the follow tab in
	 * particular needs SSR to know up-front whether to skip the
	 * unauthenticated public-products fetch. */
	currentTab: StoryTabKey;
	/** Current filter snapshot used to warm the viewer-aware follow tab before
	 * committing the URL change. Omitted by the seller import preview. */
	fetchParams?: MarketFetchParams;
	pageSize?: number;
	/**
	 * Preview mode for the seller-side import preview shell. Tabs render
	 * but tap-through is suppressed so clicks don't navigate away from
	 * the preview tab.
	 */
	previewMode?: boolean;
	sortLinks: Array<{ value: string; label: string; href: string }>;
	currentSort: string;
}

/**
 * Glue layer between the (top) story rail + tabs and the (bottom) market
 * product grid, mirroring `features/feed/components/FeedClientShell.tsx`.
 *
 * Tab → product behaviour:
 *   - 推薦 / 附近商家  → unfiltered grid (rail re-sorts itself for 附近).
 *   - 追蹤中            → grid scoped to `followed_seller_only=true` via the
 *                         BFF proxy (B-183). SSR skips the products fetch
 *                         when this tab is active because the public-products
 *                         endpoint needs the buyer JWT.
 */
export default function MarketClientShell({
	stories,
	sidebar,
	productGrid,
	currentTab,
	fetchParams,
	pageSize,
	previewMode = false,
	sortLinks,
	currentSort,
}: MarketClientShellProps) {
	const queryClient = useQueryClient();
	const { accountId, activeRole } = useQueryIdentity();
	// The route is ISR: writing `?tab` via history.pushState re-reads the URL on
	// the client (useUrlSearchParams) and re-scopes the grid in place (the follow
	// tab still fetches the buyer-scoped products client-side), with no server
	// round trip / no useSearchParams bailout. `null` clears the tab (推薦).
	const setTab = React.useCallback((value: string | null) => {
		updateUrlParams({ tab: value });
	}, []);

	const followFetchParams = React.useMemo(
		() =>
			fetchParams ? { ...fetchParams, followedSellerOnly: true } : undefined,
		[fetchParams],
	);
	const prefetchFollowProducts = React.useCallback(async () => {
		if (
			previewMode ||
			!followFetchParams ||
			!pageSize ||
			accountId === null ||
			activeRole !== "buyer"
		) {
			return;
		}
		await queryClient.prefetchInfiniteQuery({
			queryKey: marketProductsListQueryKey(
				followFetchParams,
				pageSize,
				accountId,
			),
			queryFn: ({ pageParam, signal }) =>
				fetchMarketProductsPage({
					fetchParams: followFetchParams,
					pageSize,
					page: Number(pageParam) || 1,
					signal,
				}),
			initialPageParam: 1,
			getNextPageParam: nextMarketProductsPage,
			staleTime: 30_000,
		});
	}, [
		accountId,
		activeRole,
		followFetchParams,
		pageSize,
		previewMode,
		queryClient,
	]);

	// Warm the only viewer-aware market tab in the background. If the buyer
	// taps immediately, handleTabChange awaits this same query and leaves the
	// fully rendered recommendation grid in place until the follow result is
	// ready — no empty-state or skeleton flash.
	React.useEffect(() => {
		if (currentTab === "follow") return;
		void prefetchFollowProducts().catch(() => undefined);
	}, [currentTab, prefetchFollowProducts]);

	const handleTabChange = React.useCallback(
		(key: StoryTabKey) => {
			if (previewMode) return;
			if (key === currentTab) return;
			if (key === "follow") {
				void prefetchFollowProducts()
					.catch(() => undefined)
					.then(() => setTab("follow"));
				return;
			}
			void setTab(key === "recommend" ? null : key);
		},
		[currentTab, prefetchFollowProducts, previewMode, setTab],
	);

	return (
		<>
			{stories.length > 0 ? (
				<div
					inert={previewMode || undefined}
					className={
						previewMode ? "pointer-events-none select-none" : undefined
					}
				>
					<FeedStories
						items={stories}
						activeTab={currentTab}
						onTabChange={handleTabChange}
						previewMode={previewMode}
						hideTabsOnMobile
					/>
				</div>
			) : null}

			<div className="grid grid-cols-1 gap-6 items-start min-[960px]:grid-cols-[260px_minmax(0,1fr)]">
				<aside
					className="market-list-sidebar"
					aria-label="篩選與分類"
					inert={previewMode || undefined}
					style={
						previewMode
							? { pointerEvents: "none", userSelect: "none" }
							: undefined
					}
				>
					{/* CSS-only collapse: a visually-hidden checkbox toggles a
					 * sibling `~ .market-list-sidebar__body` via the
					 * `:checked` selector. Default state is unchecked so
					 * mobile lands on the product grid immediately. Desktop
					 * CSS hides the toggle and forces the body open. This
					 * avoids the modern <details> shadow-DOM
					 * `content-visibility: hidden` behavior which a plain
					 * `display: flex` override couldn't break through. */}
					<input
						type="checkbox"
						id="market-filter-toggle"
						className="market-list-sidebar__check"
						aria-label="展開篩選與分類"
					/>
					<label
						htmlFor="market-filter-toggle"
						className="market-list-sidebar__toggle"
					>
						<span>篩選與分類</span>
					</label>
					<div className="market-list-sidebar__body">
						<div className="market-list-sidebar__mobile-controls">
							<section className="market-list-sidebar__mobile-group">
								<h2>推薦商家</h2>
								<div
									className="market-list-sidebar__mobile-options market-list-sidebar__mobile-options--tabs"
									role="tablist"
									aria-label="推薦商家篩選"
								>
									{FEED_TABS.map((tab) => (
										<button
											key={tab.key}
											type="button"
											role="tab"
											aria-selected={tab.key === currentTab}
											data-active={tab.key === currentTab}
											onClick={() => handleTabChange(tab.key as StoryTabKey)}
										>
											{tab.label}
										</button>
									))}
								</div>
							</section>
							<section className="market-list-sidebar__mobile-group">
								<h2>商品排序</h2>
								<nav
									className="market-list-sidebar__mobile-options market-list-sidebar__mobile-options--sort"
									aria-label="商品排序"
								>
									{sortLinks.map((option) => (
										<Link
											key={option.value}
											href={option.href}
											scroll={false}
											data-active={option.value === currentSort}
											aria-current={
												option.value === currentSort ? "page" : undefined
											}
										>
											{option.label}
										</Link>
									))}
								</nav>
							</section>
						</div>
						{sidebar}
					</div>
				</aside>
				<div className="market-list-main flex flex-col gap-4 min-w-0">
					{productGrid}
				</div>
			</div>
		</>
	);
}
