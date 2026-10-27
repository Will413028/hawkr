"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { fetchMyAfterSales } from "@/lib/after-sales/buyer-repo";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseMyAfterSalesResult {
	records: AfterSaleCase[];
	/** `orderNo → latest AfterSaleCase` — INCLUDES terminal cases (the buyer
	 * needs to see a rejected case so they can appeal). */
	lookup: Map<string, AfterSaleCase>;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/** Latest case per order, all statuses (buyer view — unlike the seller lookup
 * which hides rejected/cancelled). */
function buildBuyerLookup(
	cases: ReadonlyArray<AfterSaleCase>,
): Map<string, AfterSaleCase> {
	const map = new Map<string, AfterSaleCase>();
	for (const c of cases) {
		if (!c.orderNo) continue;
		const existing = map.get(c.orderNo);
		if (!existing || (c.createdAt ?? "") > (existing.createdAt ?? "")) {
			map.set(c.orderNo, c);
		}
	}
	return map;
}

/**
 * Buyer-side after-sale list + `orderNo` lookup. Drives the 售後歷史 page and
 * the order-detail after-sale summary block. Scoped to the logged-in buyer.
 */
export function useMyAfterSales(): UseMyAfterSalesResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const enabled = hydrated && isLoggedIn;
	const query = useQuery({
		queryKey: queryKeys.buyer.afterSales(identity, { pageSize: 100 }),
		queryFn: async () => {
			const { records: list } = await fetchMyAfterSales({ pageSize: 100 });
			return list;
		},
		enabled,
		placeholderData: [] as AfterSaleCase[],
	});
	const records = enabled ? (query.data ?? []) : [];
	const lookup = React.useMemo(() => buildBuyerLookup(records), [records]);

	return {
		records,
		lookup,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
