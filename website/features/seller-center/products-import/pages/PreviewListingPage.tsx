"use client";

import { notFound } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import type { StoryItem } from "@/features/feed/components/StoryRail";
import MarketProductsList, {
	type MarketProductItem,
} from "@/features/market/components/MarketProductsList";
import MarketProductsListLayout, {
	type SidebarHashtagLink,
} from "@/features/market/products/MarketProductsListLayout";
import { fetchCategoryTree, fetchSeller, fetchSellers } from "@/lib/api";
import { ApiClientError } from "@/lib/api/client";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import type { CategoryNode, MarketFetchParams } from "@/lib/market/listFilters";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { fetchPopularHashtags } from "@/lib/search/repo";
import { recommendedSellerLogoUrl } from "@/lib/seller/recommended-logo";
import { storeIdOf } from "@/lib/seller/store";
import { PreviewBanner } from "../components/PreviewBanner";
import { UI_TEXT } from "../i18n";
import { getImportJob, getImportJobPreviewListing } from "../repo";
import type {
	ImportItemPreviewListingRecord,
	ImportPreviewListing,
} from "../types";

interface Props {
	jobId: string;
}

const PAGE_SIZE = 24;
const SORT_OPTIONS: Array<{ value: string; label: string }> = [
	{ value: "推薦", label: "推薦" },
	{ value: "最新上架", label: "最新上架" },
	{ value: "銷量最高", label: "銷量最高" },
	{ value: "價格低到高", label: "價格低到高" },
	{ value: "價格高到低", label: "價格高到低" },
	{ value: "評分最高", label: "評分最高" },
];
/**
 * 把 preview 列表 record 轉成 MarketProductItem — 純欄位 rename + 當下登入賣家資訊。
 *
 * Backend records 形狀為 `{ importItemId, importStatus, previewData: ProductListPresentation }`，
 * 預覽商品資訊在 `previewData` 內。
 *
 * 設計重點：origin / unit / rating 等 backend 列表卡沒帶的欄位**留空**，
 * 不寫死「預覽」/「件」placeholder。card 自己對空值有 fallback 行為（不
 * 顯示該段），與正式商城卡接到空欄位時的表現一致。
 */
function toMarketItem(
	record: ImportItemPreviewListingRecord,
	jobId: string,
	currentSellerName: string,
	currentSellerAvatar: string,
	currentSellerId: string | undefined,
): MarketProductItem {
	const data = record.previewData;
	const priceMin = data.priceMin ?? "0";
	const priceMax = data.priceMax ?? priceMin;
	const regularNum = Number(priceMin) || 0;
	const priceLabel =
		priceMin === priceMax ? `NT$ ${priceMin}` : `NT$ ${priceMin} ~ ${priceMax}`;
	return {
		id: record.importItemId,
		name: data.name,
		// presigned／原站 URL，不是平台媒體 key —— 不可再套 preset。
		image: data.imagePreviewUrl ?? "",
		sellerName: currentSellerName,
		sellerId: currentSellerId,
		sellerAvatar: currentSellerAvatar || undefined,
		origin: data.origin ?? "",
		price: priceLabel,
		originalPrice: undefined,
		isOnSale: false,
		hashtags: [],
		// 預覽列表卡沒 unit 資訊 → 留空（card 不顯示）。
		unit: "",
		rating: null,
		reviews: 0,
		href: `/seller-product-import-preview/${jobId}/items/${record.importItemId}`,
		priceValue: regularNum,
		createdAt: 0,
		salesCount: 0,
	};
}

export function PreviewListingPage({ jobId }: Props) {
	const toast = useNotificationToast();
	const { data: sellerSelf } = useSellerSelf();
	const [filename, setFilename] = React.useState("");
	const [page, setPage] = React.useState(1);
	const [listing, setListing] = React.useState<ImportPreviewListing | null>(
		null,
	);
	const [categories, setCategories] = React.useState<CategoryNode[]>([]);
	const [stories, setStories] = React.useState<StoryItem[]>([]);
	const [trendingHashtags, setTrendingHashtags] = React.useState<
		Array<{ name: string; label: string; count?: number }>
	>([]);
	const [currentSeller, setCurrentSeller] = React.useState<{
		name?: string;
		logoUrl?: string;
	} | null>(null);
	const [loading, setLoading] = React.useState(false);
	const [paginationAction, setPaginationAction] = React.useState<
		"previous" | "next" | null
	>(null);
	const [missing, setMissing] = React.useState(false);

	const storeId = storeIdOf(sellerSelf);

	// 抓當下登入賣家資訊 — 預覽列表卡掛在「我的賣場」名下，但用真實資料而不寫死。
	React.useEffect(() => {
		if (!storeId) {
			setCurrentSeller(null);
			return;
		}
		let cancelled = false;
		fetchSeller(storeId)
			.then((s) => {
				if (cancelled) return;
				setCurrentSeller(s as { name?: string; logoUrl?: string } | null);
			})
			.catch(() => undefined);
		return () => {
			cancelled = true;
		};
	}, [storeId]);

	const refresh = React.useCallback(
		async (nextPage: number) => {
			setLoading(true);
			setMissing(false);
			try {
				const [job, list] = await Promise.all([
					getImportJob(jobId),
					getImportJobPreviewListing(jobId, {
						page: nextPage,
						pageSize: PAGE_SIZE,
					}),
				]);
				setFilename(job.sourceFilename);
				setListing(list);
				setPage(nextPage);
			} catch (e) {
				if (e instanceof ApiClientError && e.status === 404) {
					setMissing(true);
					return;
				}
				toast.show({
					variant: "error",
					title: "載入預覽失敗",
					body: e instanceof Error ? e.message : "請稍後再試",
					durationMs: 5000,
				});
			} finally {
				setLoading(false);
			}
		},
		[jobId, toast],
	);

	React.useEffect(() => {
		void refresh(1);
		void fetchCategoryTree()
			.then((tree) => {
				if (Array.isArray(tree)) setCategories(tree as CategoryNode[]);
			})
			.catch(() => undefined);
		void fetchSellers(12)
			.then((sellers) => {
				const items: StoryItem[] = (sellers as Array<Record<string, unknown>>)
					.filter((s) => s?.storeId && s?.storeName)
					.map((s) => ({
						id: String(s.storeId),
						// Same snake_case segment as the market rail. The route is
						// noindex + seller-guarded, so no crawler sees these hrefs — but
						// FeedStories renders StoryRail without `onSelect`, so they are
						// real <Link>s a seller can click. Omitting the code would cost
						// that seller a 308 (#365).
						storeCode:
							typeof s.storeCode === "string"
								? s.storeCode.trim() || undefined
								: undefined,
						name: String(s.storeName),
						avatar: recommendedSellerLogoUrl(s),
						city: typeof s.city === "string" ? s.city : undefined,
						district: typeof s.district === "string" ? s.district : undefined,
					}))
					.slice(0, 12);
				setStories(items);
			})
			.catch(() => undefined);
		void fetchPopularHashtags({ limit: 8 })
			.then((tags) => setTrendingHashtags(tags))
			.catch(() => undefined);
	}, [refresh]);

	const changePage = async (action: "previous" | "next", nextPage: number) => {
		if (loading || paginationAction !== null) return;
		setPaginationAction(action);
		try {
			await refresh(nextPage);
		} finally {
			setPaginationAction(null);
		}
	};

	const cards = listing?.records ?? [];
	const currentSellerName = currentSeller?.name ?? "";
	const currentSellerAvatar = currentSeller?.logoUrl ?? "";
	const currentSellerId = storeId;
	const adaptedProducts = React.useMemo<MarketProductItem[]>(
		() =>
			cards.map((c) =>
				toMarketItem(
					c,
					jobId,
					currentSellerName,
					currentSellerAvatar,
					currentSellerId,
				),
			),
		[cards, jobId, currentSellerName, currentSellerAvatar, currentSellerId],
	);
	const totalCount = Number(listing?.total ?? 0) || 0;
	const totalPages = Math.max(1, Math.ceil(totalCount / PAGE_SIZE));

	// 預覽模式：所有 sidebar / sort link 都導去 `/market/products?...`。
	const sortLinks = SORT_OPTIONS.map((opt) => ({
		value: opt.value,
		label: opt.label,
		href: "#",
	}));
	const hashtagLinks: SidebarHashtagLink[] = trendingHashtags.map((tag) => ({
		name: tag.name,
		label: tag.label || `#${tag.name}`,
		href: `/market/products?hashtags=${encodeURIComponent(tag.name)}`,
		active: false,
	}));

	const fetchParams: MarketFetchParams = { keyword: "" };

	if (missing) notFound();

	const productGrid =
		loading && adaptedProducts.length === 0 ? (
			<p className="py-16 text-center text-sm text-hk-muted">載入中…</p>
		) : adaptedProducts.length === 0 ? (
			<div className="rounded-hk-xl border border-dashed border-hk-border-soft bg-hk-surface-card p-12 text-center">
				<p className="text-hk-muted">尚無可預覽商品</p>
				<p className="mt-1 text-xs text-hk-muted/80">
					圖片仍下載中或需修正商品不會出現在預覽
				</p>
			</div>
		) : (
			<MarketProductsList
				products={adaptedProducts}
				sortLinks={sortLinks}
				currentSort="推薦"
				fetchParams={fetchParams}
				pageSize={PAGE_SIZE}
				initialDone
				previewMode
			/>
		);

	return (
		<main className="market-page w-[min(1440px,calc(100%_-_48px))] mx-auto pt-[var(--page-start-gap,_6px)] pb-[60px]! max-[768px]:w-[min(100%_-_28px,680px)]">
			<header className="mt-2 mb-4 flex flex-wrap items-center justify-between gap-3 max-md:mt-0 max-md:mb-3">
				<div className="flex flex-col gap-1">
					<span className="text-xs font-semibold tracking-wider text-hk-primary uppercase">
						商城預覽 · 賣家批次匯入 #{jobId}
					</span>
					<span className="text-sm text-hk-muted">
						列表預覽（完整商城 layout）
					</span>
				</div>
				<button
					type="button"
					onClick={() => {
						if (typeof window !== "undefined") window.close();
					}}
					className="seller-page-header-cta inline-flex items-center gap-1.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-3 py-1.5 text-xs text-hk-muted hover:bg-hk-surface-glass-hover"
				>
					關閉此分頁
				</button>
			</header>
			<nav
				className="mb-4 flex flex-wrap items-center gap-2 text-sm text-hk-muted"
				aria-label="麵包屑"
			>
				<span>{UI_TEXT.preview.backToManage}</span>
				<span aria-hidden>·</span>
				<span>商城預覽（列表）</span>
			</nav>

			<div className="mb-6">
				<PreviewBanner
					jobFilename={filename || "—"}
					totalEligible={Number(listing?.totalEligible ?? "0") || 0}
					totalNeedsFix={Number(listing?.totalNeedsFix ?? "0") || 0}
					totalAssetsDownloading={Number(listing?.totalPending ?? "0") || 0}
					variant="listing"
				/>
			</div>

			<MarketProductsListLayout
				previewMode
				categories={categories}
				sellerStories={stories}
				currentCategories={[]}
				currentPriceMin=""
				currentPriceMax=""
				currentHashtags={[]}
				currentSort="推薦"
				currentQuery=""
				currentTab="recommend"
				appliedChips={[]}
				clearAllHref="/market/products"
				sortLinks={sortLinks}
				hashtagLinks={hashtagLinks}
				productGrid={productGrid}
			/>

			{totalPages > 1 ? (
				<nav
					className="mt-8 flex items-center justify-center gap-3 text-sm"
					aria-label="分頁"
				>
					<Button
						type="button"
						variant="bordered"
						color="default"
						disabled={page <= 1 || loading || paginationAction !== null}
						isLoading={paginationAction === "previous"}
						loadingLabel="載入上一頁中…"
						onClick={() => void changePage("previous", page - 1)}
						className="h-auto rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-hk-body transition-colors hover:bg-hk-surface-glass-hover disabled:cursor-not-allowed disabled:opacity-40"
					>
						上一頁
					</Button>
					<span className="font-mono text-hk-muted">
						第 {page} / {totalPages} 頁
					</span>
					<Button
						type="button"
						variant="bordered"
						color="default"
						disabled={
							page >= totalPages || loading || paginationAction !== null
						}
						isLoading={paginationAction === "next"}
						loadingLabel="載入下一頁中…"
						onClick={() => void changePage("next", page + 1)}
						className="h-auto rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-hk-body transition-colors hover:bg-hk-surface-glass-hover disabled:cursor-not-allowed disabled:opacity-40"
					>
						下一頁
					</Button>
				</nav>
			) : null}
		</main>
	);
}
