"use client";

import {
	keepPreviousData,
	useInfiniteQuery,
	useQuery,
} from "@tanstack/react-query";
import { fetchMyAfterSales } from "@/lib/after-sales/buyer-repo";
import { useAuth } from "@/lib/auth/context";
import { AfterSaleCaseStatus } from "@/lib/codemeta/enums";
import {
	denormalizeStatus,
	fetchOrder,
	fetchOrderStatusCounts,
	fetchOrders,
} from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { queryKeys } from "@/lib/query/keys";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * 賣家視角（`view: "seller"`）的訂單一律走 `useSellerDataScope()`：代行工作區沿用
 * 同一批頁面，但傳輸層、快取鍵與啟用條件都必須換（56 號 §4.5）。買家視角完全不受
 * 影響 —— 那條路徑沒有代行對應物。
 */

interface UseOrderListResult {
	orders: Order[];
	loading: boolean;
	refreshing: boolean;
	loadingMore: boolean;
	error: string | null;
	page: number;
	pageSize: number;
	total: number;
	hasMore: boolean;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
}

interface OrderQueryOptions {
	buyerId?: string;
	/** "seller" → fetch the seller (fulfilment) view; omit for buyer view. */
	view?: "seller";
}

/**
 * Paged orders hook. `status` flips refetch from page 1. The backend scopes
 * data by the JWT session identity sealed in the BFF cookie.
 */
export function useOrderList(
	status?: string,
	options: OrderQueryOptions = {},
): UseOrderListResult {
	const { buyerId, view } = options;
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const scope = useSellerDataScope();
	const seller = view === "seller";
	const queryParams = { status, buyerId, view };
	const query = useInfiniteQuery({
		queryKey: seller
			? scope.keys.orders(queryParams)
			: queryKeys.buyer.orders(identity, queryParams),
		queryFn: ({ pageParam, signal }) =>
			seller
				? scope.repos.orders.fetchOrders({
						status,
						page: pageParam,
						pageSize: 20,
						buyerId,
						view,
						signal,
					})
				: fetchOrders({
						status,
						page: pageParam,
						pageSize: 20,
						buyerId,
						view,
						signal,
					}),
		initialPageParam: 1,
		enabled: seller ? scope.canRead("orders", isLoggedIn) : isLoggedIn,
		// Keep the previous status tab's orders on screen while the next tab
		// loads, instead of flashing an empty list.
		placeholderData: keepPreviousData,
		getNextPageParam: (lastPage) => {
			const loaded = lastPage.page * lastPage.pageSize;
			return loaded < lastPage.total ? lastPage.page + 1 : undefined;
		},
	});
	const pages = query.data?.pages ?? [];
	const orders = pages.flatMap((page) => page.records);
	const lastPage = pages.at(-1);
	const page = lastPage?.page ?? 1;
	const pageSize = lastPage?.pageSize ?? 20;
	const total = lastPage?.total ?? 0;
	const error = query.error instanceof Error ? query.error.message : null;

	return {
		orders: isLoggedIn ? orders : [],
		loading: query.isLoading,
		refreshing: query.isFetching && !query.isFetchingNextPage,
		loadingMore: query.isFetchingNextPage,
		error,
		page,
		pageSize,
		total,
		hasMore: Boolean(query.hasNextPage),
		refresh: async () => {
			await query.refetch();
		},
		loadMore: async () => {
			if (!query.isFetchingNextPage && query.hasNextPage) {
				await query.fetchNextPage();
			}
		},
	};
}

interface UseAllOrdersResult {
	orders: Order[];
	loading: boolean;
	refreshing: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * Eagerly load the seller's ENTIRE order set (every page) in one query, so the
 * caller can classify orders into tabs client-side and count them without the
 * backend status-rollup ever disagreeing with the rendered list.
 *
 * The seller orders page derives both its stat-card counts and its table from
 * the same classified set, so a card can never outnumber (or jump away from)
 * the rows under it. Pagination is handled in-memory by the caller. Backend
 * scopes rows by the JWT session; we never pass `status`, so this is the full
 * fulfilment queue across every lifecycle stage.
 */
export function useAllSellerOrders(
	options: { view?: "seller"; enabled?: boolean } = {},
): UseAllOrdersResult {
	const { view, enabled: enabledOverride } = options;
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const scope = useSellerDataScope();
	const seller = view === "seller";
	const enabled =
		(seller ? scope.canRead("orders", isLoggedIn) : isLoggedIn) &&
		(enabledOverride ?? true);
	const query = useQuery({
		queryKey: seller
			? scope.keys.orders({ view, scope: "all" })
			: queryKeys.buyer.orders(identity, { type: "all", view }),
		queryFn: async ({ signal }) => {
			const pageSize = 100;
			const all: Order[] = [];
			let page = 1;
			// Hard page cap so a misbehaving backend pagination contract can't
			// spin this into an unbounded fetch loop.
			for (let guard = 0; guard < 100; guard += 1) {
				const res = seller
					? await scope.repos.orders.fetchOrders({
							page,
							pageSize,
							view,
							signal,
						})
					: await fetchOrders({ page, pageSize, view, signal });
				all.push(...res.records);
				const loaded = res.page * res.pageSize;
				if (res.records.length === 0 || loaded >= res.total) break;
				page += 1;
			}
			return all;
		},
		enabled,
	});

	return {
		orders: enabled ? (query.data ?? []) : [],
		loading: query.isLoading,
		refreshing: query.isFetching,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/** The one pseudo-status that isn't a real `OrderStatus` bucket: it maps to
 *  the buyer's after-sale case count, not an order-status rollup entry. Kept
 *  as a literal because callers already pass it verbatim as a tab key. */
const AFTER_SALE_KEY = "after_sale";

/** Buyer-visible in-progress `AfterSaleCaseStatus` codes — exactly the
 *  buyer after-sales page's non-「已結案」groups (`afterSaleGroupOf` in
 *  lib/after-sales/status.ts: pending 0/27/30 ∪ returning 20/25/26/28 ∪
 *  refunding 40). Terminal / auto-closed codes (33/50/60/70/71 and the
 *  auto-completed 31/32, which that page files under 已結案) are excluded so
 *  the 退款中 todo bucket only counts cases still needing attention. The
 *  backend splits this CSV per status (AfterSalesGrpcService statusFilter). */
const AFTER_SALE_IN_PROGRESS_CSV = [
	AfterSaleCaseStatus.PENDING, // 0 待處理
	AfterSaleCaseStatus.RETURN_PENDING, // 20 待退貨
	AfterSaleCaseStatus.RETURN_SHIPPING, // 25 退貨運送中
	AfterSaleCaseStatus.RETURN_AT_STORE, // 26 退貨已到店待取
	AfterSaleCaseStatus.RETURN_DELIVERED, // 27 退貨已送達
	AfterSaleCaseStatus.RETURN_FAILED, // 28 退貨物流失敗
	AfterSaleCaseStatus.RETURN_RECEIVED, // 30 已收退貨
	AfterSaleCaseStatus.REFUND_PROCESSING, // 40 退款處理中
].join(",");

/**
 * Per-status order counts, read from a single `/app/orders/status_counts`
 * rollup (buyer-scoped, JWT). The endpoint returns every non-zero
 * `OrderStatus` bucket's count plus the grand total, so we fold those raw
 * counts into each caller's `statusCsv` key locally — no more N parallel
 * `page_size=1` probes (the old Sentry N+1 DEV-3 fan-out).
 *
 * The map keys are the same `statusCsv` values that `useOrderList`
 * accepts — pass each tab's `statusCsv` (or `""` for the all-bucket) to
 * look up its count. Returns 0 for unknown keys / while loading.
 *
 * `after_sale` is a special key: it's a UI pseudo-status the order rollup
 * doesn't track, so it resolves to the buyer's count of in-progress
 * after-sale cases via `GET /app/after_sales` instead.
 */
export interface UseOrderStatusCountsResult {
	counts: Record<string, number>;
	error: string | null;
	loading: boolean;
	refresh: () => Promise<void>;
}

export function useOrderStatusCounts(
	statusCsvList: ReadonlyArray<string>,
	options: OrderQueryOptions = {},
): UseOrderStatusCountsResult {
	const { buyerId, view } = options;
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const scope = useSellerDataScope();
	// 代行沒有 `/orders/status_counts` 對應端點（§6.1.10 只有列表與明細），所以在
	// 代行 scope 下這支恆不啟用；總覽的數字改由列表的 `total` 自繪。
	const enabled =
		hydrated &&
		isLoggedIn &&
		(view === "seller" ? scope.mode === "seller" : true);

	// Callers pass a module-scope constant for `statusCsvList` (see
	// `COUNT_KEYS` in OrdersClient) so the array identity stays stable
	// across renders and this effect doesn't re-fire on every paint.
	const query = useQuery({
		queryKey:
			view === "seller"
				? scope.keys.orderCounts({
						statusCsvList,
						buyerId,
						view,
					})
				: queryKeys.buyer.orders(identity, {
						type: "counts",
						statusCsvList,
						buyerId,
					}),
		queryFn: async ({ signal }) => {
			// One rollup call for every real OrderStatus bucket, plus (only when a
			// caller actually asks for it) one after-sale count. The after-sale
			// fetch soft-degrades to 0 so a flaky /app/after_sales can't blank the
			// whole order-count panel; the rollup itself is the primary signal and
			// its failure surfaces through query.error.
			const needsAfterSale = statusCsvList.some((csv) =>
				csv.split(",").some((token) => token.trim() === AFTER_SALE_KEY),
			);
			const [rollup, afterSaleTotal] = await Promise.all([
				fetchOrderStatusCounts({ signal }),
				needsAfterSale
					? fetchMyAfterSales({
							pageSize: 1,
							status: AFTER_SALE_IN_PROGRESS_CSV,
						})
							.then((r) => r.total)
							.catch(() => 0)
					: Promise.resolve(0),
			]);
			const entries = statusCsvList.map((csv) => {
				// "" = 全部 → the rollup's grand total.
				if (!csv) return [csv, rollup.total] as const;
				// after_sale is a pseudo-status the rollup doesn't track.
				if (csv.split(",").some((token) => token.trim() === AFTER_SALE_KEY)) {
					return [csv, afterSaleTotal] as const;
				}
				// `denormalizeStatus` turns semantic names AND already-numeric CSVs
				// into a numeric-code CSV (reusing repo.ts's single name↔code
				// table); sum every raw OrderStatus bucket this tab spans.
				const sum = denormalizeStatus(csv)
					.split(",")
					.reduce(
						(acc, code) => acc + (rollup.countsByStatus[code.trim()] ?? 0),
						0,
					);
				return [csv, sum] as const;
			});
			return Object.fromEntries(entries);
		},
		enabled,
		placeholderData: {} as Record<string, number>,
	});

	return {
		counts: enabled ? (query.data ?? {}) : {},
		error: query.error instanceof Error ? query.error.message : null,
		loading: query.isLoading,
		refresh: async () => {
			await query.refetch();
		},
	};
}

interface UseOrderResult {
	order: Order | null;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

export function useOrder(
	orderNo: string | undefined,
	opts?: { view?: "seller" },
): UseOrderResult {
	const view = opts?.view;
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const scope = useSellerDataScope();
	const seller = view === "seller";
	const query = useQuery({
		queryKey: seller
			? scope.keys.order(orderNo ?? "", { view })
			: queryKeys.buyer.order(identity, orderNo ?? ""),
		queryFn: () =>
			seller
				? scope.repos.orders.fetchOrder(orderNo ?? "", { view: "seller" })
				: fetchOrder(orderNo ?? ""),
		// Gate on hydration + login: an order detail fetch under the pre-hydration
		// `anon` slot just caches a 401.
		enabled:
			Boolean(orderNo) &&
			(seller
				? scope.canRead("orders", hydrated && isLoggedIn)
				: hydrated && isLoggedIn),
	});

	return {
		order: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
