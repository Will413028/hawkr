"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	type AfterSaleCase,
	buildAfterSaleLookup,
	fetchSellerAfterSales,
} from "@/lib/seller/after-sales";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseSellerAfterSalesResult {
	/** Raw cases (single page of 100) — drives the 售後處理 case table on the
	 * 異常處理 tab. Includes terminal cases (50/60/70); the panel filters them
	 * behind the 顯示已結案 toggle. */
	records: AfterSaleCase[];
	/** `orderNo → AfterSaleCase` (latest case per order; rejected/cancelled excluded). */
	lookup: Map<string, AfterSaleCase>;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * Seller-side after-sale case lookup, indexed by `orderNo`.
 *
 * Mirrors APP `sellerAfterSaleLookupProvider` (`keepAlive`-cached): the
 * Map drives the order list's "待處理 / 售後處理" chip counts and the
 * expanded row's after-sale summary block. Auto-refetches when the
 * active role flips so a seller-buyer toggle doesn't leak stale data.
 *
 * The lookup excludes REJECTED(60) / CANCELLED(70) cases — those didn't enter
 * the refund flow and would mislead the seller if surfaced on order rows. It
 * keeps CLOSED(50), so completed coupon/refund decisions can still move a
 * delivered order out of the generic 配送中 bucket.
 */
export function useSellerAfterSales(): UseSellerAfterSalesResult {
	const { activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";
	const query = useQuery({
		queryKey: queryKeys.seller.afterSales(identity, { pageSize: 100 }),
		queryFn: async () => {
			const { records } = await fetchSellerAfterSales({ pageSize: 100 });
			return records;
		},
		enabled,
		// placeholderData (not initialData): initialData seeds [] as fresh cache,
		// which the global 30s staleTime then treats as up-to-date and skips the
		// mount fetch — the seller case table would stay empty for up to 30s.
		placeholderData: [] as AfterSaleCase[],
	});
	const records = enabled ? (query.data ?? []) : [];

	return {
		records,
		lookup: buildAfterSaleLookup(records),
		loading: query.isLoading || query.isPlaceholderData,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
