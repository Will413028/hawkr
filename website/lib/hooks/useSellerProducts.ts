"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import type { ProductListResult } from "@/lib/products/seller-repo";
import { queryKeys } from "@/lib/query/keys";
import { useSellerDataScope } from "@/lib/seller-data/scope";

interface UseSellerProductsResult {
	data: ProductListResult | null;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * Seller-owned products list (`/app/products`) as TanStack-managed server-state.
 * Unlike `useSellerDashboard` (one shared payload), each consumer reads a
 * different slice — Shell counts low-stock (pageSize 100), DashboardPage wants
 * just `.total` (pageSize 1) — so the params live in the queryKey and each
 * variant gets its own identity-scoped cache entry. The win is write-side:
 * after any product mutation, `useInvalidateSellerProducts` refetches every
 * variant, so the nav low-stock badge and dashboard product count stop going
 * stale. Read-only consumers use this hook; ProductsPage/InventoryPage keep
 * their own paginated/optimistic list state and only call the invalidator.
 *
 * 傳輸層、快取鍵與啟用條件一律取自 `useSellerDataScope()`：同一支 hook 在賣家中心
 * 讀 `/app/products`、在代行工作區讀代行 BFF，且兩者的快取永不相撞（56 號 §4.5）。
 */
export function useSellerProducts(
	params?: { page?: number; pageSize?: number; status?: number | string },
	options?: { enabled?: boolean },
): UseSellerProductsResult {
	const { isLoggedIn, activeRole } = useAuth();
	const scope = useSellerDataScope();
	const query = useQuery({
		queryKey: scope.keys.products(params),
		queryFn: () => scope.repos.products.fetchSellerProducts(params ?? {}),
		enabled:
			scope.canRead("products", isLoggedIn && activeRole === "seller") &&
			(options?.enabled ?? true),
		staleTime: 30_000,
		refetchOnWindowFocus: true,
	});
	return {
		// `data` (not `error || !data`): a failed background refetch keeps the
		// last-good list rather than dropping the badge/count to empty.
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * Invalidate the whole seller-products family (all page/pageSize/status
 * variants) — call after create / update / status / delete so the Shell badge
 * and dashboard count refetch. Passing no params yields the `{}` slot, which
 * partial-matches every variant.
 */
export function useInvalidateSellerProducts(): () => Promise<void> {
	const queryClient = useQueryClient();
	const scope = useSellerDataScope();
	return () =>
		Promise.all([
			queryClient.invalidateQueries({
				queryKey: scope.keys.products(),
			}),
			// 公開商品列表（發文頁商品選單等）同步失效，避免剛上/改的商品
			// 在 30s staleTime 窗內看不到。
			queryClient.invalidateQueries({
				queryKey: queryKeys.sellers.productsRoot(),
			}),
		]).then(() => undefined);
}
