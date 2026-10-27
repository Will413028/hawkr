"use client";

import {
	type InfiniteData,
	useInfiniteQuery,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { fetchProducts } from "@/lib/api";
import { fetchCanonicalMarketRecords } from "@/lib/api/main-tab-pages";
import { useListFreshnessScope } from "@/lib/hooks/useListFreshnessScope";
import { useMainTabActivationRefresh } from "@/lib/hooks/useMainTabActivationRefresh";
import {
	type MarketHeadPage,
	marketHeadRequiresReplacement,
	mergeMarketHeadPages,
	preserveMarketBuyerState,
} from "@/lib/market/headRefresh";
import {
	type MarketFetchParams,
	mapProduct,
	type ProductRecord,
} from "@/lib/market/listFilters";
import type { MarketListSnapshot } from "@/lib/market/listSnapshot";
import type { MarketProductItem } from "@/lib/market/types";
import {
	MAIN_TAB_FULL_RECONCILE_MS,
	MAIN_TAB_STALE_MS,
} from "@/lib/navigation/main-tab-freshness";
import { queryKeys } from "@/lib/query/keys";
import { readMainTabRuntime } from "@/lib/state/main-tab-freshness-store";
import { useQueryIdentity } from "./useQueryIdentity";

export type MarketProductsPage = MarketHeadPage;
type MarketQueryData = InfiniteData<MarketProductsPage, number>;

interface PendingMarketRefresh {
	data: MarketQueryData;
	newCount: number;
	generation: string;
	queryToken: object;
}

interface UseMarketProductsOptions {
	initialProducts: MarketProductItem[];
	initialDone: boolean;
	pageSize: number;
	fetchParams: MarketFetchParams;
	restoredSnapshot?: MarketListSnapshot | null;
	enableActivationRefresh?: boolean;
	/** Enable freshness checks for filtered/sorted/viewer-scoped lists without
	 * switching their requests to the canonical endpoint. Preview stays opt-out. */
	enableScopedActivationRefresh?: boolean;
	/**
	 * Whether `initialProducts` are the authoritative page-1 for `fetchParams`.
	 * The canonical (no-filter) `/market/products` first page is server-rendered
	 * into the ISR HTML (true); a client-applied filter passes false so the query
	 * fetches its own page 1 instead of seeding from the canonical products of a
	 * different key. A back-navigation snapshot always seeds regardless.
	 * Defaults to true so existing callers (e.g. the seller import preview, which
	 * seeds authoritative preview products) keep their current behaviour.
	 */
	seedInitialData?: boolean;
}

function dedupeProducts(pages: MarketProductsPage[]) {
	const seen = new Set<string>();
	const products: MarketProductItem[] = [];
	for (const page of pages) {
		for (const product of page.products) {
			if (seen.has(product.id)) continue;
			seen.add(product.id);
			products.push(product);
		}
	}
	return products;
}

function marketPagesToReconcile(pages: MarketProductsPage[]): number[] {
	const onlyPage = pages.length === 1 ? pages[0] : undefined;
	if (onlyPage && onlyPage.page > 1) {
		// Back-navigation stores all loaded products as one aggregate snapshot and
		// keeps the last cursor in `page`. Rebuild 1..N, not only page N.
		return Array.from({ length: onlyPage.page }, (_, index) => index + 1);
	}
	return Array.from(new Set(pages.map((page) => Math.max(1, page.page))));
}

function supersededRefreshError() {
	return new DOMException("tab query changed", "AbortError");
}

export function marketProductsListQueryKey(
	fetchParams: MarketFetchParams,
	pageSize: number,
	accountId: string | null,
) {
	const viewerScope =
		fetchParams.followedProductOnly || fetchParams.followedSellerOnly
			? { viewerId: accountId ?? "anon" }
			: {};
	return queryKeys.products.list({
		...fetchParams,
		...viewerScope,
		pageSize,
	});
}

export async function fetchMarketProductsPage({
	fetchParams,
	pageSize,
	page,
	signal,
}: {
	fetchParams: MarketFetchParams;
	pageSize: number;
	page: number;
	signal?: AbortSignal;
}): Promise<MarketProductsPage> {
	const json = await fetchProducts(
		fetchParams.keyword,
		page,
		pageSize,
		fetchParams.categoryId,
		fetchParams.sort,
		{
			hashtags: fetchParams.hashtags,
			temp: fetchParams.temp,
			priceMin: fetchParams.priceMin,
			priceMax: fetchParams.priceMax,
			certificationTypes: fetchParams.certificationTypes,
			followedProductOnly: fetchParams.followedProductOnly,
			followedSellerOnly: fetchParams.followedSellerOnly,
			signal,
		},
	);
	const rawRecords: ProductRecord[] =
		(
			json as {
				data?: { records?: ProductRecord[] };
				records?: ProductRecord[];
			}
		)?.data?.records ||
		(json as { records?: ProductRecord[] })?.records ||
		[];
	const products = rawRecords.map((record, index) =>
		mapProduct(record, (page - 1) * pageSize + index),
	);
	return {
		products,
		page,
		hasMore: rawRecords.length >= pageSize,
	};
}

export function nextMarketProductsPage(lastPage: MarketProductsPage) {
	return lastPage.hasMore ? lastPage.page + 1 : undefined;
}

export function useMarketProducts({
	initialProducts,
	initialDone,
	pageSize,
	fetchParams,
	restoredSnapshot,
	enableActivationRefresh = false,
	enableScopedActivationRefresh = false,
	seedInitialData = true,
}: UseMarketProductsOptions) {
	const { accountId } = useQueryIdentity();
	const queryClient = useQueryClient();
	// Followed-only lists are viewer-specific (favourited products / followed
	// sellers, resolved from the session cookie). Scope their cache key by viewer
	// so one buyer's followed list can't be served to another on the same tab.
	// The default (public) listing stays shared.
	const queryKey = React.useMemo(
		() => marketProductsListQueryKey(fetchParams, pageSize, accountId),
		[fetchParams, pageSize, accountId],
	);
	const queryGeneration = React.useMemo(
		() => JSON.stringify(queryKey),
		[queryKey],
	);
	// A restored snapshot belongs to the first complete query key seen by this
	// mounted list. A filter/viewer change must fetch or reuse its own cache.
	const restoreOriginRef = React.useRef({
		generation: queryGeneration,
		cacheUpdatedAt: queryClient.getQueryState(queryKey)?.dataUpdatedAt ?? 0,
		superseded: false,
	});
	const currentQueryState = queryClient.getQueryState(queryKey);
	const hasNewerCache =
		restoredSnapshot !== null &&
		restoredSnapshot !== undefined &&
		(restoreOriginRef.current.cacheUpdatedAt >
			(restoredSnapshot.savedAt ?? 0) ||
			((currentQueryState?.dataUpdateCount ?? 0) > 0 &&
				(currentQueryState?.dataUpdatedAt ?? 0) >
					(restoredSnapshot.savedAt ?? 0)));
	if (
		hasNewerCache ||
		restoreOriginRef.current.generation !== queryGeneration
	) {
		restoreOriginRef.current.superseded = true;
	}
	const restored = restoreOriginRef.current.superseded
		? null
		: (restoredSnapshot ?? null);
	const hasSeed = restored !== null || seedInitialData;
	const seedProducts = restored?.products ?? initialProducts;
	const seedPage = restored
		? restored.page
		: initialProducts.length === 0
			? 0
			: 1;
	const seedDone = restored?.done ?? initialDone;
	const queryGenerationRef = React.useRef(queryGeneration);
	queryGenerationRef.current = queryGeneration;
	const freshnessScope = useListFreshnessScope(queryKey);
	const seedTimestampRef = React.useRef({
		generation: queryGeneration,
		updatedAt: restored?.savedAt ?? Date.now(),
	});
	if (seedTimestampRef.current.generation !== queryGeneration) {
		seedTimestampRef.current = {
			generation: queryGeneration,
			updatedAt: restored?.savedAt ?? Date.now(),
		};
	}
	const initialDataUpdatedAt =
		restored?.savedAt ?? seedTimestampRef.current.updatedAt;
	const [snapshotResolutionReady, setSnapshotResolutionReady] =
		React.useState(false);
	React.useLayoutEffect(() => setSnapshotResolutionReady(true), []);
	const fetchPage = React.useCallback(
		async (page: number, signal?: AbortSignal) => {
			if (!enableActivationRefresh) {
				return fetchMarketProductsPage({
					fetchParams,
					pageSize,
					page,
					signal,
				});
			}
			const rawRecords = await fetchCanonicalMarketRecords(page, signal);
			return {
				products: rawRecords.map((record, index) =>
					mapProduct(record, (page - 1) * pageSize + index),
				),
				page,
				hasMore: rawRecords.length >= pageSize,
			} satisfies MarketProductsPage;
		},
		[enableActivationRefresh, fetchParams, pageSize],
	);
	const query = useInfiniteQuery({
		queryKey,
		queryFn: ({ pageParam, signal }) =>
			fetchPage(Number(pageParam) || 1, signal),
		initialPageParam: seedPage,
		// Only seed when the products are authoritative for this key (canonical
		// first page or a back-nav snapshot). A client-applied filter leaves this
		// undefined so TanStack fetches page 1 for the new key instead of showing
		// the canonical products under a filter that never matched them.
		initialData: hasSeed
			? {
					pages: [
						{
							products: seedProducts,
							page: seedPage,
							hasMore: !seedDone,
						},
					],
					pageParams: [seedPage],
				}
			: undefined,
		initialDataUpdatedAt: hasSeed ? initialDataUpdatedAt : undefined,
		getNextPageParam: nextMarketProductsPage,
		// Tab activation uses a page-one commerce reconcile below. A normal
		// infinite-query refetch would replay every page the buyer has loaded.
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: 30 * 60 * 1000,
		refetchOnMount: false,
	});

	// A hard document must hydrate from the SSR page-1 seed first. If a matching
	// back-navigation snapshot arrives in the layout phase immediately after that
	// commit, project it for the next render and synchronously replace the query
	// cache before paint. This preserves appended pages/pagination without ever
	// letting sessionStorage alter the HTML React initially hydrates.
	const restoredData = React.useMemo(
		() =>
			restored
				? {
						pages: [
							{
								products: restored.products,
								page: restored.page,
								hasMore: !restored.done,
							},
						],
						pageParams: [restored.page],
					}
				: null,
		[restored],
	);
	const appliedRestoredRef = React.useRef<MarketListSnapshot | null>(null);
	const restorePending =
		restoredData !== null && appliedRestoredRef.current !== restored;
	React.useLayoutEffect(() => {
		if (!restored || !restoredData || appliedRestoredRef.current === restored) {
			return;
		}
		appliedRestoredRef.current = restored;
		queryClient.setQueryData(queryKey, restoredData, {
			updatedAt: restored.savedAt ?? Date.now(),
		});
	}, [queryClient, queryKey, restored, restoredData]);

	const listWorkQueueRef = React.useRef<Promise<void>>(Promise.resolve());
	const enqueueListWork = React.useCallback(
		(work: () => Promise<void>): Promise<void> => {
			const run = listWorkQueueRef.current.catch(() => undefined).then(work);
			listWorkQueueRef.current = run.catch(() => undefined);
			return run;
		},
		[],
	);
	const [pendingRefresh, setPendingRefresh] =
		React.useState<PendingMarketRefresh | null>(null);
	const pendingRefreshRef = React.useRef<PendingMarketRefresh | null>(null);
	const commitPendingRefresh = React.useCallback(
		(next: PendingMarketRefresh | null) => {
			pendingRefreshRef.current = next;
			setPendingRefresh(next);
		},
		[],
	);
	const activePendingRefresh =
		pendingRefresh?.generation === queryGeneration ? pendingRefresh : null;
	React.useEffect(() => {
		if (
			pendingRefreshRef.current &&
			pendingRefreshRef.current.generation !== queryGeneration
		) {
			commitPendingRefresh(null);
		}
	}, [commitPendingRefresh, queryGeneration]);
	const refreshHead = React.useCallback(
		(signal: AbortSignal, onlyIfStale = false) => {
			const expectedGeneration = queryGeneration;
			return enqueueListWork(async () => {
				const queryToken = queryClient
					.getQueryCache()
					.find({ queryKey, exact: true });
				if (
					!queryToken ||
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration
				) {
					throw signal.reason ?? supersededRefreshError();
				}
				// A successful full reconciliation already includes page 1. This
				// check belongs inside the queue, after the earlier work completes.
				if (
					onlyIfStale &&
					Date.now() -
						(queryClient.getQueryState(queryKey)?.dataUpdatedAt ?? 0) <
						MAIN_TAB_STALE_MS.market
				) {
					return;
				}
				const freshPage = await fetchPage(1, signal);
				if (
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration ||
					queryClient.getQueryCache().find({ queryKey, exact: true }) !==
						queryToken
				) {
					throw supersededRefreshError();
				}
				const autoApply =
					!restored && typeof window !== "undefined" && window.scrollY <= 120;
				let requiresReplacement = false;
				let newCount = 0;
				let candidate: MarketQueryData = {
					pages: [freshPage],
					pageParams: [freshPage.page],
				};
				queryClient.setQueryData<MarketQueryData>(queryKey, (current) => {
					if (!current) return candidate;
					const freshPages = preserveMarketBuyerState(
						[freshPage],
						current.pages,
					);
					candidate = {
						pages: freshPages,
						pageParams: [freshPage.page],
					};
					const first = current.pages[0];
					requiresReplacement = marketHeadRequiresReplacement(
						first?.products ?? [],
						freshPages[0]?.products ?? [],
					);
					const existingIds = new Set(
						current.pages.flatMap((page) =>
							page.products.map((product) => product.id),
						),
					);
					newCount = freshPage.products.filter(
						(product) => !existingIds.has(product.id),
					).length;
					if (autoApply) return candidate;
					return {
						...current,
						pages: mergeMarketHeadPages(
							current.pages,
							freshPage.products,
							false,
						).pages,
					};
				});
				commitPendingRefresh(
					!autoApply && requiresReplacement
						? {
								data: candidate,
								newCount,
								generation: expectedGeneration,
								queryToken,
							}
						: null,
				);
			});
		},
		[
			commitPendingRefresh,
			enqueueListWork,
			fetchPage,
			queryClient,
			queryGeneration,
			queryKey,
			restored,
		],
	);
	const refreshAllPages = React.useCallback(
		(signal: AbortSignal) => {
			const expectedGeneration = queryGeneration;
			return enqueueListWork(async () => {
				const queryToken = queryClient
					.getQueryCache()
					.find({ queryKey, exact: true });
				if (
					!queryToken ||
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration
				) {
					throw signal.reason ?? supersededRefreshError();
				}
				const current = queryClient.getQueryData<MarketQueryData>(queryKey);
				if (!current || current.pages.length === 0) return;
				const rawFreshPages: MarketProductsPage[] = [];
				for (const page of marketPagesToReconcile(current.pages)) {
					if (
						signal.aborted ||
						queryGenerationRef.current !== expectedGeneration ||
						queryClient.getQueryCache().find({ queryKey, exact: true }) !==
							queryToken
					) {
						throw signal.reason ?? supersededRefreshError();
					}
					rawFreshPages.push(await fetchPage(page, signal));
				}
				if (
					signal.aborted ||
					queryGenerationRef.current !== expectedGeneration ||
					queryClient.getQueryCache().find({ queryKey, exact: true }) !==
						queryToken
				) {
					throw supersededRefreshError();
				}
				const freshPages = preserveMarketBuyerState(
					rawFreshPages,
					current.pages,
				);
				const candidate: MarketQueryData = {
					pages: freshPages,
					pageParams: freshPages.map((page) => page.page),
				};
				const requiresReplacement =
					current.pages.length !== freshPages.length ||
					current.pages.some((page, index) =>
						marketHeadRequiresReplacement(
							page.products,
							freshPages[index]?.products ?? [],
						),
					);
				const existingIds = new Set(
					current.pages.flatMap((page) =>
						page.products.map((product) => product.id),
					),
				);
				const newCount = freshPages
					.flatMap((page) => page.products)
					.filter((product) => !existingIds.has(product.id)).length;
				const autoApply =
					!restored && typeof window !== "undefined" && window.scrollY <= 120;
				if (autoApply) {
					queryClient.setQueryData(queryKey, candidate);
					commitPendingRefresh(null);
					return;
				}
				queryClient.setQueryData<MarketQueryData>(queryKey, {
					...current,
					pages: mergeMarketHeadPages(
						current.pages,
						freshPages.flatMap((page) => page.products),
						false,
					).pages,
				});
				commitPendingRefresh(
					requiresReplacement
						? {
								data: candidate,
								newCount,
								generation: expectedGeneration,
								queryToken,
							}
						: null,
				);
			});
		},
		[
			commitPendingRefresh,
			enqueueListWork,
			fetchPage,
			queryClient,
			queryGeneration,
			queryKey,
			restored,
		],
	);
	const activationEnabled =
		(enableActivationRefresh || enableScopedActivationRefresh) &&
		snapshotResolutionReady &&
		query.data !== undefined;
	const fullActivationRefresh = useMainTabActivationRefresh({
		scope: `${freshnessScope}:full`,
		// Head updates and remounts must not extend the full-list deadline.
		freshAt:
			readMainTabRuntime(`${freshnessScope}:full`).lastSuccessAt ||
			query.dataUpdatedAt,
		staleMs: MAIN_TAB_FULL_RECONCILE_MS,
		enabled: activationEnabled,
		refresh: refreshAllPages,
	});
	const activationRefresh = useMainTabActivationRefresh({
		scope: freshnessScope,
		freshAt: restored?.savedAt ?? query.dataUpdatedAt,
		staleMs: MAIN_TAB_STALE_MS.market,
		enabled: activationEnabled,
		refresh: (signal) => refreshHead(signal, true),
	});
	const applyPendingFresh = React.useCallback(() => {
		if (!activePendingRefresh) return;
		void enqueueListWork(async () => {
			const latest = pendingRefreshRef.current;
			if (
				!latest ||
				latest.generation !== queryGeneration ||
				queryGenerationRef.current !== queryGeneration ||
				queryClient.getQueryCache().find({ queryKey, exact: true }) !==
					latest.queryToken
			) {
				return;
			}
			queryClient.setQueryData(queryKey, latest.data);
			commitPendingRefresh(null);
			window.scrollTo({ top: 0, behavior: "auto" });
		});
	}, [
		activePendingRefresh,
		commitPendingRefresh,
		enqueueListWork,
		queryClient,
		queryGeneration,
		queryKey,
	]);

	const pages = restorePending ? restoredData.pages : (query.data?.pages ?? []);
	const products = React.useMemo(() => dedupeProducts(pages), [pages]);
	const lastPage = pages.at(-1);
	const page = lastPage?.page ?? seedPage;
	const refresh = React.useCallback(async () => {
		const controller = new AbortController();
		await refreshHead(controller.signal);
	}, [refreshHead]);
	const loadMore = React.useCallback(async () => {
		await enqueueListWork(async () => {
			if (!query.isFetchingNextPage && query.hasNextPage) {
				await query.fetchNextPage();
			}
		});
	}, [
		enqueueListWork,
		query.fetchNextPage,
		query.hasNextPage,
		query.isFetchingNextPage,
	]);

	return {
		products,
		page,
		done: !query.hasNextPage,
		loading: query.isFetchingNextPage,
		// True while a filter's own page 1 is in flight and no products have
		// arrived yet — lets the grid hold a placeholder instead of flashing the
		// "no results" empty state before the client fetch resolves.
		initialLoading: query.isPending,
		refreshing:
			activationRefresh.refreshing || fullActivationRefresh.refreshing,
		hasPendingFresh: activePendingRefresh !== null,
		pendingNewCount: activePendingRefresh?.newCount ?? 0,
		applyPendingFresh,
		refresh,
		loadMore,
	};
}
