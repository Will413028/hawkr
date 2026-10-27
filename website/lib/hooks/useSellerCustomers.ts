"use client";

import {
	keepPreviousData,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { useCallback } from "react";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchSellerCustomers,
	type SellerCustomer,
	type SellerCustomerType,
} from "@/lib/seller/repo";
import { useQueryIdentity } from "./useQueryIdentity";

export type UseSellerCustomersParams = {
	page?: number;
	pageSize?: number;
	keyword?: string;
	tagId?: string | number;
	customerType?: SellerCustomerType;
	sortBy?: "total_spent" | "order_count";
	sortOrder?: "asc" | "desc";
};

interface UseSellerCustomersResult {
	data: SellerCustomer[] | null;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * The seller's CRM customer list (`/app/sellers/me/customers`). Each consumer
 * passes its own page/pageSize/keyword (CustomersClient 50, messages workspace
 * 200, coupon picker debounced search), so — like `useSellerProducts` — the
 * params live in the queryKey and each variant is its own cache entry; there is
 * no read-side dedup. The win is write-side: after a classification / tag change
 * in the detail drawer, `useInvalidateSellerCustomers` refetches the list so the
 * row badges + chips update, replacing the manual `load()` + mutated-ref dance.
 *
 * `refetchOnWindowFocus` is off: CustomersClient layers two N-row enrichment
 * fan-outs (per-row tags + last-purchase) on top of this list, keyed off the
 * result reference, so a focus-driven refetch would re-fire 50+ requests for no
 * real freshness gain. Refetches stay intentional (mount / filter / invalidate).
 * `placeholderData: keepPreviousData` holds the prior filter's rows on screen
 * while a tab/sort/tag switch (a new queryKey) loads, instead of flashing the
 * whole table to skeleton — the same behaviour the old mutable-list version had.
 */
export function useSellerCustomers(
	params?: UseSellerCustomersParams,
): UseSellerCustomersResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.seller.customers(identity, params),
		queryFn: () => fetchSellerCustomers(params ?? {}),
		enabled: isLoggedIn && activeRole === "seller",
		staleTime: 30_000,
		refetchOnWindowFocus: false,
		placeholderData: keepPreviousData,
	});
	return {
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * Invalidate the whole seller-customers family (all param variants) — call after
 * a customer classification / tag mutation so the list row badges + chips
 * refetch. Passing no params yields the `{}` slot, which partial-matches every
 * list variant without touching the per-buyer `customer` detail entries.
 */
export function useInvalidateSellerCustomers(): () => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	// Stable ref: CustomersClient feeds this into `closeDrawer` → the detail
	// drawer's `onClose`, which the overlay tracks as an effect dependency. An
	// unstable callback would re-bind that effect (and steal focus) on any
	// re-render while the drawer is open.
	return useCallback(
		() =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.customers(identity),
			}),
		[queryClient, identity],
	);
}
