"use client";

import * as React from "react";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import Link from "@/components/shared/Link";
import MasonryGrid from "@/components/shared/MasonryGrid";
import { ProductGridSkeleton } from "@/components/shared/RouteLoadingSkeleton";
import Button from "@/components/ui/Button";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import { MARKET_PRODUCT_GRID_BREAKPOINTS } from "@/features/market/components/marketProductGridBreakpoints";
import { trackViewItemList } from "@/lib/analytics/events";
import { useHydrationSafeClientRender } from "@/lib/hooks/useHydrationSafeClientRender";
import { useMarketProducts } from "@/lib/hooks/useMarketProducts";
import type { MarketFetchParams } from "@/lib/market/listFilters";
import {
	type MarketListSnapshot,
	marketListSnapshotForInitialRender,
	peekMarketListSnapshot,
	saveMarketListSnapshot,
	takeMarketListSnapshot,
} from "@/lib/market/listSnapshot";
import type { MarketProductItem } from "@/lib/market/types";

export type { MarketProductItem } from "@/lib/market/types";

export interface SortLink {
	value: string;
	label: string;
	href: string;
}

interface MarketProductsListProps {
	products: MarketProductItem[];
	/**
	 * Pre-resolved sort options with hrefs. The parent server component
	 * builds the URLs (it already owns URL state) so this client doesn't
	 * need a function prop — functions can't cross the server/client
	 * boundary in App Router.
	 */
	sortLinks: SortLink[];
	currentSort: string;
	/**
	 * True when the "追蹤中" tab is active. Drives the empty-state copy
	 * and triggers an on-mount page-1 fetch through the BFF, because SSR
	 * cannot reach the authenticated `followed_seller_only` endpoint.
	 */
	isFollowMode?: boolean;
	hasActiveFilters?: boolean;
	/** Filter snapshot reused for load-more requests. */
	fetchParams: MarketFetchParams;
	/** Page size for both the initial SSR fetch and follow-up pages. */
	pageSize: number;
	/** True when the initial SSR page was already the last one. */
	initialDone: boolean;
	/**
	 * Whether `products` are the authoritative page-1 for `fetchParams`. The
	 * canonical (no-filter) route seeds true; a client-applied filter passes
	 * false so the grid fetches its own page 1. Defaults to true (preview seeds
	 * its own authoritative products).
	 */
	seedInitialData?: boolean;
	/**
	 * Preview mode for the seller-side import preview shell. Skips snapshot
	 * persistence, pagination, analytics emission
	 * and turns the sort nav into non-interactive labels. Cards already honor
	 * their own `previewMode` for cart / favourite / restock gating.
	 */
	previewMode?: boolean;
}

/**
 * Pure product grid. Tab state, login gating, and nearby sorting all
 * live one level up in `MarketClientShell` → `FeedStories`, mirroring
 * the feed page (`InfiniteFeed` is similarly stateless about tabs).
 */
export default function MarketProductsList({
	products: initialProducts,
	sortLinks,
	currentSort,
	isFollowMode = false,
	hasActiveFilters = false,
	fetchParams,
	pageSize,
	initialDone,
	seedInitialData = true,
	previewMode = false,
}: MarketProductsListProps) {
	const [hydrated, setHydrated] = React.useState(false);
	React.useEffect(() => setHydrated(true), []);
	const hydrationSafeClientRender = useHydrationSafeClientRender();

	// SSR delivers page 1; subsequent pages are appended after an explicit
	// request from the buyer. The follow tab is the one
	// exception: SSR skips the products fetch (no JWT), so we kick off
	// page 1 from `loadMore` on mount.
	// Back-navigation restore (卡片→詳情→返回). Browser state must never alter a
	// hard document's first hydration tree. A list mounted later by client
	// navigation can still peek synchronously and render the restored cards on
	// its first paint, preserving scroll continuity without a page-1 flash.
	const [restored, setRestored] = React.useState<MarketListSnapshot | null>(
		() =>
			marketListSnapshotForInitialRender(hydrationSafeClientRender, () =>
				previewMode
					? null
					: peekMarketListSnapshot(
							window.location.pathname + window.location.search,
						),
			),
	);
	const snapshotConsumedRef = React.useRef(false);
	React.useLayoutEffect(() => {
		if (previewMode || snapshotConsumedRef.current) return;
		snapshotConsumedRef.current = true;
		// Consume only after the SSR-matching hydration commit. Client-only mounts
		// already seeded `restored` above; a hard Back can now apply the same
		// snapshot in this pre-paint layout phase instead of discarding it and
		// flashing page 1 at the top.
		const snapshot = takeMarketListSnapshot(
			window.location.pathname + window.location.search,
		);
		if (!restored && snapshot) setRestored(snapshot);
	}, [previewMode, restored]);

	const {
		products,
		page,
		done,
		loading,
		initialLoading,
		loadMore,
		refreshing,
		hasPendingFresh,
		pendingNewCount,
		applyPendingFresh,
	} = useMarketProducts({
		initialProducts,
		initialDone,
		pageSize,
		fetchParams,
		restoredSnapshot: restored,
		seedInitialData,
		enableActivationRefresh:
			!previewMode &&
			!isFollowMode &&
			!hasActiveFilters &&
			currentSort === "推薦",
		enableScopedActivationRefresh: !previewMode,
	});
	// Re-assert the saved scroll while the restored list lays out. The masonry
	// computes its height asynchronously (ResizeObserver after paint), so the page
	// is briefly short and a single scrollTo would land at the top, then jump down
	// once the height settles — the flicker on back. Pin the target every frame
	// until it actually sticks (height grew enough) so the view tracks straight to
	// the saved position with no visible jump, then stop.
	React.useEffect(() => {
		if (!restored) return;
		// Restored items were already tracked on the way in — don't re-emit.
		lastTrackedCountRef.current = restored.products.length;
		const targetY = restored.scrollY;
		let raf = 0;
		let frames = 0;
		let settled = 0;
		const pin = () => {
			const root = document.documentElement;
			const previousBehavior = root.style.scrollBehavior;
			root.style.scrollBehavior = "auto";
			window.scrollTo(0, targetY);
			root.style.scrollBehavior = previousBehavior;
			frames += 1;
			settled = Math.abs(window.scrollY - targetY) < 2 ? settled + 1 : 0;
			// Stop once the position has held for a few frames (layout settled) or
			// after a safety cap (~1s) so we never pin against a genuine user scroll.
			if ((settled >= 3 && frames > 4) || frames > 60) {
				// Tell the card→detail reverse zoom the background is settled, so it
				// can start the shrink and uncover a stable list (no flash). See
				// runReverseZoom in components/shared/PageTransition.tsx.
				window.dispatchEvent(new Event("hk:market-list-restored"));
				return;
			}
			raf = window.requestAnimationFrame(pin);
		};
		raf = window.requestAnimationFrame(pin);
		return () => window.cancelAnimationFrame(raf);
	}, [restored]);

	// Snapshot the list whenever the buyer clicks through to a detail page
	// (product card or its seller link) so the back navigation can restore it.
	const snapshotStateRef = React.useRef({ products, page, done });
	snapshotStateRef.current = { products, page, done };
	React.useEffect(() => {
		const onClickCapture = (event: MouseEvent) => {
			if (
				event.defaultPrevented ||
				event.button !== 0 ||
				event.altKey ||
				event.ctrlKey ||
				event.metaKey ||
				event.shiftKey
			) {
				return;
			}
			if (!(event.target instanceof Element)) return;
			const anchor = event.target.closest<HTMLAnchorElement>("a[href]");
			if (!anchor) return;
			const href = anchor.getAttribute("href") ?? "";
			// 進詳情（返回時還原）與底部 Tab 切走（切回時像 App Tab 原地接續，
			// 見 PageTransition 的 tabKeyFor）都要存快照；還原端以 key 精確比對
			// 且有時效，對不上就自動作廢，多存無害。
			const isDetailNav = /^\/(products|sellers)\//.test(href);
			const isTabNav = href === "/" || href === "/feed" || href === "/search";
			if (!isDetailNav && !isTabNav) return;
			const { products, page, done } = snapshotStateRef.current;
			if (products.length === 0) return;
			saveMarketListSnapshot({
				key: window.location.pathname + window.location.search,
				products,
				page,
				done,
				scrollY: window.scrollY,
			});
		};
		if (previewMode) return undefined;
		document.addEventListener("click", onClickCapture, true);
		return () => document.removeEventListener("click", onClickCapture, true);
	}, [previewMode]);

	// Follow tab arrives without SSR products (no JWT in server fetch);
	// trigger page-1 load on mount + whenever the filter snapshot changes
	// while still empty.
	React.useEffect(() => {
		if (previewMode) return;
		if (page === 0 && !done && !loading) {
			loadMore();
		}
	}, [page, done, loading, loadMore, previewMode]);

	const visibleProducts = products;

	const listContext = React.useMemo(() => {
		const id = isFollowMode
			? "followed_sellers"
			: fetchParams.keyword
				? "search"
				: fetchParams.categoryId
					? `category_${fetchParams.categoryId}`
					: "market_products";
		const name = isFollowMode
			? "追蹤中商家"
			: fetchParams.keyword
				? "搜尋結果"
				: "Hawkr 市集";
		return { listId: id, listName: name };
	}, [isFollowMode, fetchParams.keyword, fetchParams.categoryId]);

	const lastTrackedCountRef = React.useRef(0);
	React.useEffect(() => {
		if (previewMode) return;
		// Client-only emission (SSR shell + load-more would double-count); fire
		// the newly-appended slice each time the list grows.
		if (products.length <= lastTrackedCountRef.current) {
			lastTrackedCountRef.current = products.length; // reset on filter change
			return;
		}
		const fresh = products.slice(lastTrackedCountRef.current);
		lastTrackedCountRef.current = products.length;
		trackViewItemList(fresh, listContext);
	}, [products, listContext, previewMode]);

	// Wait for the first fetch to resolve before showing the follow empty
	// state so we don't flash "no products" before the BFF responds.
	const followPending = isFollowMode && page === 0;
	const followEmpty =
		isFollowMode && !loading && page > 0 && visibleProducts.length === 0;
	// A client-applied filter fetches its own page 1 (no SSR seed). Hold a
	// placeholder while that first page loads so we don't flash "no results"
	// before it resolves. The canonical route is seeded, so this stays false
	// there — the ready marker below is unaffected on the default page.
	const initialPending =
		!isFollowMode &&
		!previewMode &&
		initialLoading &&
		visibleProducts.length === 0;

	return (
		<>
			{/* A streamed list can paint before its client handlers hydrate. This
			    readiness marker lets navigation tests (and future loading chrome)
			    distinguish a visible card from one that is actually interactive. */}
			<span data-hk-market-products-hydrated={hydrated} hidden />
			{hydrated && !followPending && !initialPending ? (
				<span data-hk-tab-content-ready="market" hidden />
			) : null}
			<span
				data-hk-market-background-refresh={refreshing ? "true" : "false"}
				hidden
			/>
			{hasPendingFresh ? (
				<div
					className="pointer-events-none fixed left-1/2 top-[calc(var(--site-nav-height,72px)+10px)] z-40 -translate-x-1/2"
					aria-live="polite"
				>
					<Button
						className="pointer-events-auto shadow-lg"
						variant="solid"
						color="primary"
						radius="full"
						onClick={applyPendingFresh}
						aria-label={
							pendingNewCount > 0
								? `顯示 ${pendingNewCount} 件新上架商品並回到頂端`
								: "套用最新商品內容並回到頂端"
						}
					>
						{pendingNewCount > 0
							? `有 ${pendingNewCount} 件新品上架`
							: "商品內容已更新"}
					</Button>
				</div>
			) : null}
			<header className="flex flex-wrap items-center justify-between gap-3 py-1.5">
				<p className="m-0 text-sm text-hk-muted">
					{followPending
						? "追蹤商家的商品"
						: initialPending
							? "商品載入中…"
							: `共 ${visibleProducts.length} 件商品`}
				</p>
				<nav className="market-list-main__sort" aria-label="排序">
					<span aria-hidden="true">排序：</span>
					{sortLinks.map((opt) =>
						previewMode ? (
							// 預覽模式：用 <a> + preventDefault 取代 <Link>，繼承
							// `.market-list-main__sort a` 的 CSS（padding / 顏色 /
							// active 高亮），避免整排被 opacity-60 打成「全部褪色」
							// 而與正式視覺不一致。
							// biome-ignore lint/a11y/useValidAnchor: preview mode no-op
							<a
								key={opt.value}
								href="#"
								data-active={opt.value === currentSort}
								aria-disabled="true"
								onClick={(e) => e.preventDefault()}
								style={{ cursor: "not-allowed" }}
							>
								{opt.label}
							</a>
						) : (
							<Link
								key={opt.value}
								href={opt.href}
								scroll={false}
								data-active={opt.value === currentSort}
								aria-current={opt.value === currentSort ? "page" : undefined}
							>
								{opt.label}
							</Link>
						),
					)}
				</nav>
			</header>

			{followPending || initialPending ? (
				<ProductGridSkeleton />
			) : followEmpty ? (
				<EmptyStatePanel
					className="col-[1/-1] min-h-[360px] bg-transparent! border-transparent!"
					icon="box"
					title="追蹤的商家目前沒有上架的商品"
					description="回到「推薦」看看，或追蹤更多商家來豐富這份清單。"
					iconSize={128}
				/>
			) : visibleProducts.length === 0 ? (
				hasActiveFilters ? (
					<EmptyStatePanel
						className="col-[1/-1] min-h-[360px] bg-transparent! border-transparent!"
						icon="box"
						title="目前沒有符合條件的商品"
						description="換個分類、價格範圍或搜尋關鍵字再試一次。"
						iconSize={128}
					/>
				) : (
					<EmptyStatePanel
						className="col-[1/-1] min-h-[360px] bg-transparent! border-transparent!"
						icon="box"
						title="目前沒有上架商品"
						description="商品準備好後會出現在這裡。"
						iconSize={128}
					/>
				)
			) : (
				<MasonryGrid
					breakpoints={MARKET_PRODUCT_GRID_BREAKPOINTS}
					className="market-product-grid"
					items={visibleProducts}
					getKey={(product) => product.id}
					renderItem={(product, index) => (
						<MarketProductCard
							product={product}
							priority={index === 0}
							imageLoading={index < 4 ? "eager" : "lazy"}
							listContext={listContext}
							index={index}
							previewMode={previewMode}
						/>
					)}
				/>
			)}
			{!done && visibleProducts.length > 0 ? (
				<div className="flex min-h-20 items-center justify-center py-6">
					<Button
						variant="bordered"
						color="primary"
						radius="full"
						isLoading={loading}
						loadingLabel="載入商品中…"
						onClick={() => void loadMore()}
					>
						載入更多商品
					</Button>
				</div>
			) : null}
			{done && visibleProducts.length > 0 ? (
				<p className="mt-6 mb-2 text-center text-[0.8125rem] tracking-[0.04em] text-foreground-soft">
					— 已經到底了 —
				</p>
			) : null}
		</>
	);
}
