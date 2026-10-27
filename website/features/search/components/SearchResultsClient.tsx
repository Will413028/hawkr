"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "@/components/shared/Link";
import { ProductGridSkeleton } from "@/components/shared/RouteLoadingSkeleton";
import { Button } from "@/components/ui";
import MarketProductCard from "@/features/market/components/MarketProductCard";
import type { MarketProductItem } from "@/features/market/components/MarketProductsList";
import AssistantEntryLink from "@/features/procurement-assistant/AssistantEntryLink";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { getProductPriceDisplay } from "@/lib/products/pricing";
import { parseRating, parseReviewCount } from "@/lib/products/rating";
import { type SearchProductHit, searchProducts } from "@/lib/search/repo";

const sortOptions: Array<{ value: string; label: string }> = [
	{ value: "推薦", label: "推薦" },
	{ value: "最新上架", label: "最新上架" },
	{ value: "銷量最高", label: "銷量最高" },
	{ value: "價格低到高", label: "價格低到高" },
	{ value: "價格高到低", label: "價格高到低" },
];

function mapSort(label: string): { sortBy?: string; sortOrder?: string } {
	switch (label) {
		case "最新上架":
			return { sortBy: "created_at", sortOrder: "desc" };
		case "銷量最高":
			return { sortBy: "sales", sortOrder: "desc" };
		case "價格低到高":
			return { sortBy: "price", sortOrder: "asc" };
		case "價格高到低":
			return { sortBy: "price", sortOrder: "desc" };
		default:
			return {};
	}
}

interface SearchProductCard extends SearchProductHit {
	sellerName?: string;
	hashtags?: Array<string | { name?: string }>;
	created_at?: string | number;
	reviews_count?: string | number;
	reviewCount?: string | number;
	listPriceMin?: string;
	listPriceMax?: string;
	list_price_min?: string;
	list_price_max?: string;
	salePriceMin?: string;
	salePriceMax?: string;
	sale_price_min?: string;
	sale_price_max?: string;
}

function buildSearchSortHref(query: string, sort: string): string {
	const next = new URLSearchParams();
	if (query) next.set("q", query);
	if (sort !== "推薦") next.set("sort", sort);
	const qs = next.toString();
	return qs ? `/search?${qs}` : "/search";
}

function normaliseHashtags(
	raw: SearchProductCard["hashtags"],
	limit = 2,
): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim();
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= limit) break;
	}
	return out;
}

interface SearchProductsData {
	total: number;
	records: SearchProductCard[];
}

/**
 * Client-fetched search results. The route is ISR (server renders the landing
 * page for the no-query default); when the buyer submits a query the URL gains
 * `?q=` and this component fetches the matching products over the same
 * client-safe `/public/search/*` transport the server used, so no per-request
 * dynamic SSR is needed. A failure stays retryable.
 */
export default function SearchResultsClient({
	query,
	sort,
}: {
	query: string;
	sort: string;
}) {
	const { data, isPending, isError, isFetching, isPlaceholderData, refetch } =
		useQuery<SearchProductsData>({
			queryKey: ["search", "products", query, sort],
			queryFn: ({ signal }) => {
				const sortPayload = mapSort(sort);
				return searchProducts({
					q: query,
					page: 1,
					pageSize: 16,
					sortBy: sortPayload.sortBy,
					sortOrder: sortPayload.sortOrder,
					signal,
					strict: true,
				});
			},
			// Sorting the same keyword keeps its known cards in place until the
			// requested order arrives. Never relabel another keyword's products.
			placeholderData: (previous, previousQuery) =>
				previousQuery?.queryKey[2] === query ? previous : undefined,
			staleTime: 30_000,
			gcTime: 5 * 60 * 1000,
			enabled: Boolean(query),
		});

	const total = data?.total ?? 0;
	const products = data?.records ?? [];
	const initialError = isError && data === undefined;
	const refreshError = isError && data !== undefined;
	const showEmpty =
		!isPending &&
		data !== undefined &&
		!isPlaceholderData &&
		products.length === 0;

	return (
		<section
			className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)] pt-4"
			aria-label="搜尋結果"
			aria-busy={isFetching}
		>
			<div className="flex flex-wrap gap-4 items-center justify-between px-[18px] py-3.5 mb-[18px] bg-[var(--surface-card,rgba(255,255,255,0.8))] border border-[var(--border-soft,rgb(var(--color-ink-950-rgb)/0.08))] rounded-2xl shadow-xs dark:bg-white/4 dark:border-[rgba(245,222,165,0.14)]">
				<div className="text-[0.9375rem] text-[var(--ink-body)]">
					「
					<strong className="text-[var(--ink-heading)] font-bold">
						{query}
					</strong>
					」
					{initialError ? (
						<span> 的結果暫時無法載入</span>
					) : isPending ? (
						<span role="status"> 正在搜尋商品…</span>
					) : isFetching ? (
						<span role="status">
							{isPlaceholderData ? " 正在更新排序…" : " 正在更新搜尋結果…"}
						</span>
					) : (
						<>
							找到{" "}
							<strong className="text-[var(--ink-heading)] font-bold">
								{total}
							</strong>{" "}
							件商品
						</>
					)}
				</div>
				<nav className="market-list-main__sort flex!" aria-label="排序">
					<span aria-hidden="true">排序：</span>
					{sortOptions.map((opt) => (
						<Link
							key={opt.value}
							className="max-[840px]:inline-flex max-[840px]:min-h-11 max-[840px]:items-center"
							href={buildSearchSortHref(query, opt.value)}
							scroll={false}
							data-active={opt.value === sort}
							aria-current={opt.value === sort ? "page" : undefined}
						>
							{opt.label}
						</Link>
					))}
				</nav>
			</div>

			{refreshError ? (
				<div
					role="alert"
					className="mb-4 flex flex-wrap items-center gap-3 text-sm text-hk-muted"
				>
					<p className="m-0">搜尋結果更新失敗，先顯示上次結果。</p>
					<Button disabled={isFetching} onClick={() => void refetch()}>
						重新載入搜尋結果
					</Button>
				</div>
			) : null}
			{initialError ? (
				<div
					className="grid justify-items-center gap-4 px-6 py-[60px] text-center rounded-2xl bg-[var(--surface-card,rgba(255,255,255,0.7))] border border-dashed border-[var(--border-soft,rgb(var(--color-ink-950-rgb)/0.12))] text-[var(--ink-soft)] dark:bg-white/4 dark:border-[rgba(245,222,165,0.18)]"
					role="alert"
				>
					<div>
						<strong className="text-[var(--ink-heading)]">
							搜尋結果暫時載入失敗
						</strong>
						<p className="mt-2 mb-0 text-[0.9375rem]">
							你的搜尋文字已保留，重新載入不會改變條件。
						</p>
					</div>
					<Button disabled={isFetching} onClick={() => void refetch()}>
						重新載入搜尋結果
					</Button>
				</div>
			) : products.length > 0 ? (
				<div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-5 max-[680px]:grid-cols-2 max-[680px]:gap-3">
					{products.slice(0, 16).map((product, index) => {
						const id = String(product.id || index);
						// Empty when absent → MarketProductCard's <AppImage> shows the
						// design-system ImageFallback (folder-image) rather than a
						// branded stand-in.
						// B-238: render seller identity only from the canonical URL.
						const image = product.imageKey || product.imageKeys?.[0] || "";
						const sellerName = product.sellerName || product.seller?.name || "";
						const sellerAvatar = product.seller?.logoUrl?.trim() || "";
						const sellerId =
							String(product.storeId ?? product.seller?.storeId ?? "") ||
							undefined;
						// Prefer the seller's city as 產地 (the search hit's own
						// `origin` is unreliable — often echoes the store name).
						// Fall back to a legit product origin, else 產地直送 — matching
						// the market card's `readDisplayOrigin(...) || "產地直送"`.
						const origin =
							product.seller?.city ||
							(product.origin && product.origin !== sellerName
								? product.origin
								: "") ||
							"產地直送";
						const rating = parseRating(product.rating);
						const reviews = parseReviewCount(
							product.reviewCount ?? product.reviews_count,
						);
						const priceDisplay = getProductPriceDisplay(product);
						const hashtagList = normaliseHashtags(product.hashtags, 2);
						const item: MarketProductItem = {
							id,
							name: product.name || "Hawkr 市集商品",
							image,
							sellerName,
							sellerAvatar,
							sellerId,
							origin,
							price: priceDisplay.effectiveText || "價格未定",
							originalPrice: priceDisplay.originalText || undefined,
							isOnSale: priceDisplay.isOnSale,
							hashtags: hashtagList,
							unit: "份",
							rating,
							reviews,
							href: `/products/${id}`,
							priceValue: 0,
							createdAt: 0,
							salesCount: 0,
						};
						return (
							<MarketProductCard
								key={`product-${id}`}
								product={item}
								priority={index === 0}
								imageLoading={index < 4 ? "eager" : "lazy"}
							/>
						);
					})}
				</div>
			) : showEmpty ? (
				// data-search-empty 是給 CSS 用的鉤子：0 結果時空態內的入口才是
				// 主要 CTA，搜尋框下方那顆要讓位，避免同一屏出現兩個一樣的入口。
				<div
					data-search-empty
					className="grid justify-items-center gap-5 px-6 py-[60px] text-center rounded-2xl bg-[var(--surface-card,rgba(255,255,255,0.7))] border border-dashed border-[var(--border-soft,rgb(var(--color-ink-950-rgb)/0.12))] text-[var(--ink-soft)] dark:bg-white/4 dark:border-[rgba(245,222,165,0.18)]"
				>
					<p className="m-0 text-[0.9375rem]">
						沒有找到「{query}」的商品，試試別的關鍵字或熱門搜尋。
					</p>
					{HAWKR_AGENT_ENABLED ? (
						<AssistantEntryLink
							variant="page"
							query={query}
							source="search-empty"
							className="w-full max-w-[560px]"
						/>
					) : null}
				</div>
			) : (
				<ProductGridSkeleton variant="search" announce={false} />
			)}
		</section>
	);
}
