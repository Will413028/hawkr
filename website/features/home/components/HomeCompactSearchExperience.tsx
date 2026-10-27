"use client";

import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { createPortal } from "react-dom";
import AppImage from "@/components/shared/AppImage";
import {
	buildPopularSearchItems,
	POPULAR_HASHTAG_LIMIT,
} from "@/features/home/components/popularSearch";
import AssistantEntryLink from "@/features/procurement-assistant/AssistantEntryLink";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { getProductPriceDisplay } from "@/lib/products/pricing";
import { queryKeys } from "@/lib/query/keys";
import type { SearchProductHit } from "@/lib/search/repo";
import { fetchPopularHashtags } from "@/lib/search/repo";
import { HomeSearchProductSkeleton } from "./HomeSearchSkeletons";
import { useHomeSearchQueries } from "./useHomeSearchQueries";

const SKELETON = Array.from({ length: 4 });

export interface HomeCompactSearchPanelCoords {
	top: number;
	left: number;
	width: number;
	maxHeight: number;
}

function productTitle(product: SearchProductHit) {
	return product.name?.trim() || "Hawkr 市集商品";
}

function productImage(product: SearchProductHit) {
	// §231: read the bare objectKey; resolveCdnUrl builds the display URL.
	// Empty when absent → <AppImage> shows the design-system ImageFallback.
	return product.imageKey || product.imageKeys?.[0] || "";
}

function productMeta(product: SearchProductHit) {
	const seller = (product.sellerName || product.seller?.name || "").trim();
	const origin = (
		product.seller?.city ||
		product.origin ||
		product.categoryName ||
		""
	).trim();
	return [origin, seller].filter(Boolean).join(" · ") || "Hawkr 市集商品";
}

function productPrice(product: SearchProductHit) {
	const display = getProductPriceDisplay(product);
	return display.effectiveText || product.price || "價格未定";
}

interface HomeCompactSearchExperienceProps {
	open: boolean;
	query: string;
	requestQuery: string;
	coords: HomeCompactSearchPanelCoords | null;
	panelRef: React.RefObject<HTMLDivElement | null>;
	onDismiss: (options?: { blurInput?: boolean }) => void;
	onNavigate: (href: string) => void;
}

export default function HomeCompactSearchExperience({
	open,
	query,
	requestQuery,
	coords,
	panelRef,
	onDismiss,
	onNavigate,
}: HomeCompactSearchExperienceProps) {
	const searchQueries = useHomeSearchQueries(open, requestQuery);
	const suggestions = searchQueries.suggestions.data ?? [];
	const products = searchQueries.products.data?.records ?? [];
	const productTotal = searchQueries.products.data?.total ?? 0;
	const loadingSuggest = searchQueries.suggestions.isPending;
	const loadingProducts = searchQueries.products.isPending;
	const suggestError = searchQueries.suggestions.isError;
	const productError = searchQueries.products.isError;
	const popularHashtagsQuery = useQuery({
		queryKey: queryKeys.productHashtags.popular({
			limit: POPULAR_HASHTAG_LIMIT,
		}),
		queryFn: () => fetchPopularHashtags({ limit: POPULAR_HASHTAG_LIMIT }),
		enabled: open,
		staleTime: 5 * 60 * 1000,
		retry: 1,
	});
	const popularSearchItems = React.useMemo(
		() => buildPopularSearchItems(popularHashtagsQuery.data),
		[popularHashtagsQuery.data],
	);
	const agentEntryRef = React.useRef<HTMLAnchorElement>(null);

	const goSearch = (q: string) => {
		const trimmed = q.trim();
		if (!trimmed) return;
		onDismiss({ blurInput: true });
		onNavigate(`/search?q=${encodeURIComponent(trimmed)}`);
	};

	const panel =
		open && coords
			? createPortal(
					<>
						<div
							data-home-search-backdrop
							aria-hidden="true"
							className="fixed inset-x-0 bottom-0 z-[205] hidden cursor-default touch-none bg-[rgba(45,49,45,0.2)] backdrop-blur-[5px] [@media(max-width:840px)]:block dark:bg-[rgb(var(--color-ink-950-rgb)/0.36)] [@media(prefers-reduced-transparency:reduce)]:backdrop-blur-none"
							style={{ top: Math.max(0, coords.top - 10) }}
							onPointerDown={(event) => event.stopPropagation()}
							onMouseDown={(event) => event.stopPropagation()}
							onClick={(event) => {
								event.preventDefault();
								event.stopPropagation();
								onDismiss({ blurInput: true });
							}}
						/>
						{createPanel()}
					</>,
					document.body,
				)
			: null;

	function createPanel() {
		if (!coords) return null;
		return (
			<div
				ref={panelRef}
				id="home-search-panel"
				role="dialog"
				aria-label="搜尋建議"
				className="fixed z-[60] overflow-hidden rounded-hk-lg border border-hk-border-soft bg-hk-surface-card shadow-hk-dropdown [@media(max-width:840px)]:z-[210] [@media(max-width:840px)]:overflow-y-auto [@media(max-width:840px)]:overscroll-contain"
				style={{
					top: coords.top,
					left: coords.left,
					width: coords.width,
					maxHeight: coords.maxHeight,
				}}
			>
				{HAWKR_AGENT_ENABLED ? (
					<div className="border-b border-hk-border-subtle p-3">
						<AssistantEntryLink
							ref={agentEntryRef}
							query={query}
							source="home-search"
							onNavigate={() => {
								onDismiss({ blurInput: true });
							}}
						/>
					</div>
				) : null}
				<div
					className={`grid h-full grid-cols-1 ${
						query.trim() ? "md:grid-cols-[1.05fr_0.95fr]" : ""
					}`}
				>
					{/* LEFT — product results while typing. At rest the panel is a
					 * single column with only the recommended keywords; while typing it
					 * splits into two columns (products | keyword suggestions). */}
					{query.trim() ? (
						<div className="flex min-h-0 flex-col gap-3 border-hk-border-subtle p-4 max-md:border-b md:border-r">
							<div className="flex items-baseline justify-between gap-2">
								<strong className="font-display text-hk-heading text-sm font-bold">
									相關商品
								</strong>
								<button
									type="button"
									className="text-hk-primary text-xs font-semibold hover:underline"
									onClick={() => goSearch(query)}
								>
									查看全部 →
								</button>
							</div>
							{productError && searchQueries.products.data ? (
								<p role="alert" className="m-0 text-hk-body text-xs">
									商品結果更新失敗，先顯示上次結果。
									<button
										type="button"
										className="min-h-11 px-2 text-hk-primary font-semibold"
										disabled={searchQueries.products.isFetching}
										onClick={() => void searchQueries.products.refetch()}
									>
										重新載入商品
									</button>
								</p>
							) : null}
							{loadingProducts && products.length === 0 ? (
								<HomeSearchProductSkeleton />
							) : productError && !searchQueries.products.data ? (
								<div
									className="flex min-h-28 flex-col items-center justify-center rounded-hk-md border border-dashed border-hk-border-soft bg-hk-surface-glass px-4 py-6 text-center"
									role="alert"
								>
									<strong className="text-hk-heading text-sm">
										商品結果暫時載入失敗
									</strong>
									<p className="mt-1 text-hk-body text-xs">
										搜尋文字已保留，可以重新載入。
									</p>
									<button
										type="button"
										className="mt-3 min-h-11 rounded-hk-pill border border-hk-primary-border px-4 text-hk-accent-on-tint text-xs font-bold hover:bg-hk-primary-subtle"
										disabled={searchQueries.products.isFetching}
										onClick={() => void searchQueries.products.refetch()}
									>
										重新載入
									</button>
								</div>
							) : products.length === 0 ? (
								<div className="flex min-h-28 flex-col items-center justify-center rounded-hk-md border border-dashed border-hk-border-soft bg-hk-surface-glass px-4 py-6 text-center">
									<strong className="text-hk-heading text-sm">
										目前沒有相關商品
									</strong>
									<p className="mt-1 text-hk-body text-xs">
										試試不同關鍵字，或直接查看完整搜尋結果。
									</p>
								</div>
							) : (
								<>
									<ul className="flex min-h-0 flex-col gap-1 overflow-y-auto pr-1">
										{products.slice(0, 8).map((product, index) => {
											const id = String(product.id || index);
											const image = productImage(product);
											const price = productPrice(product);
											return (
												<li key={`${id}-${product.name}`}>
													<button
														type="button"
														data-home-search-product
														className="group flex w-full gap-3 rounded-hk-md p-2 text-left transition-colors hover:bg-hk-surface-glass-hover"
														onClick={() => {
															onDismiss();
															onNavigate(`/products/${id}`);
														}}
													>
														<AppImage
															src={resolveCdnUrl(image)}
															alt=""
															width={80}
															height={64}
															sizes="80px"
															loading="lazy"
															className="h-16 min-h-16 max-h-16 w-20 shrink-0 overflow-hidden rounded-hk-md bg-default-100 object-cover transition-transform duration-300 group-hover:scale-105"
														/>
														<div className="min-w-0 flex-1">
															<span className="line-clamp-1 text-hk-body text-sm font-semibold leading-snug">
																{productTitle(product)}
															</span>
															<span className="mt-1 line-clamp-1 text-hk-body text-xs">
																{productMeta(product)}
															</span>
															<span className="mt-1 block text-hk-primary text-sm font-extrabold">
																{price}
															</span>
														</div>
													</button>
												</li>
											);
										})}
									</ul>
									{productTotal > products.length ? (
										<p className="px-2 text-hk-body text-xs">
											共 {productTotal.toLocaleString()} 件結果
										</p>
									) : null}
								</>
							)}
						</div>
					) : null}

					{/* RIGHT — keyword search / autocomplete */}
					<div className="flex min-h-0 flex-col gap-3 p-4">
						{query.trim() ? (
							<>
								<strong className="font-display text-hk-heading text-sm font-bold">
									關鍵字建議
								</strong>
								{suggestError && searchQueries.suggestions.data ? (
									<p role="alert" className="m-0 text-hk-body text-xs">
										關鍵字建議更新失敗，先顯示上次結果。
										<button
											type="button"
											className="min-h-11 px-2 text-hk-primary font-semibold"
											disabled={searchQueries.suggestions.isFetching}
											onClick={() => void searchQueries.suggestions.refetch()}
										>
											重新載入建議
										</button>
									</p>
								) : null}
								{loadingSuggest && suggestions.length === 0 ? (
									<p className="text-hk-body text-xs">搜尋中…</p>
								) : suggestError && !searchQueries.suggestions.data ? (
									<div className="grid gap-2" role="alert">
										<p className="m-0 text-hk-body text-xs">
											關鍵字建議暫時載入失敗。
										</p>
										<button
											type="button"
											className="min-h-11 justify-self-start rounded-hk-pill border border-hk-primary-border px-4 text-hk-accent-on-tint text-xs font-bold hover:bg-hk-primary-subtle"
											disabled={searchQueries.suggestions.isFetching}
											onClick={() => void searchQueries.suggestions.refetch()}
										>
											重新載入
										</button>
									</div>
								) : suggestions.length === 0 ? (
									<button
										type="button"
										className="flex items-center gap-2 rounded-hk-md px-2 py-2 text-left text-hk-body text-sm hover:bg-hk-surface-glass-hover"
										onClick={() => goSearch(query)}
									>
										<SearchGlyph />
										搜尋「<span className="font-semibold">{query.trim()}</span>
										」
									</button>
								) : (
									<ul className="flex flex-col gap-0.5 overflow-y-auto">
										{suggestions.map((s) => (
											<li key={s.keyword}>
												<button
													type="button"
													className="flex w-full items-center gap-2 rounded-hk-md px-2 py-2 text-left hover:bg-hk-surface-glass-hover"
													onClick={() => goSearch(s.keyword)}
												>
													<SearchGlyph />
													<span className="flex-1 truncate text-hk-body text-sm">
														{s.keyword}
													</span>
													{s.category ? (
														<span className="text-hk-faint text-xs">
															{s.category}
														</span>
													) : null}
												</button>
											</li>
										))}
									</ul>
								)}
							</>
						) : (
							<>
								<strong className="font-display text-hk-heading text-sm font-bold">
									熱門搜尋
								</strong>
								{popularSearchItems.length === 0 ? (
									<p className="m-0 text-hk-body text-xs" role="status">
										{popularHashtagsQuery.isPending
											? "熱門標籤載入中…"
											: popularHashtagsQuery.isError
												? "熱門標籤暫時無法載入，仍可直接輸入商品名稱搜尋。"
												: "目前沒有熱門標籤，試試直接輸入商品名稱搜尋。"}
									</p>
								) : null}
								<ul
									className="-mx-1 flex snap-x gap-2 overflow-x-auto px-1 pb-1 [scrollbar-width:thin] md:mx-0 md:flex-wrap md:overflow-visible md:px-0 md:pb-0"
									aria-busy={popularHashtagsQuery.isLoading}
								>
									{popularSearchItems.map((item) => (
										<li
											key={`${item.source}-${item.name}`}
											className="snap-start"
										>
											<button
												type="button"
												className="shrink-0 rounded-hk-pill border border-hk-border-soft bg-hk-surface-glass px-3 py-1.5 text-hk-body text-sm whitespace-nowrap transition-[background-color,border-color,transform] hover:border-hk-primary-border hover:bg-hk-primary-subtle active:translate-y-px"
												onClick={() => {
													onDismiss();
													onNavigate(item.href);
												}}
											>
												{item.name}
											</button>
										</li>
									))}
								</ul>
							</>
						)}
					</div>
				</div>
			</div>
		);
	}

	return panel;
}

function SearchGlyph({
	stroke = 1.7,
	size = 16,
	className = "shrink-0 text-hk-muted",
}: {
	stroke?: number;
	size?: number;
	className?: string;
}) {
	return (
		<svg
			viewBox="0 0 24 24"
			width={size}
			height={size}
			fill="none"
			stroke="currentColor"
			strokeWidth={stroke}
			aria-hidden="true"
			className={className}
		>
			<circle cx="11" cy="11" r="7" />
			<path d="m20 20-3.5-3.5" strokeLinecap="round" />
		</svg>
	);
}
