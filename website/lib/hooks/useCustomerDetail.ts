"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { useAuth } from "@/lib/auth/context";
import { fetchOrders } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchSellerCustomerDetail,
	type SellerCustomerDetail,
} from "@/lib/seller/repo";
import { useQueryIdentity } from "./useQueryIdentity";

interface CustomerDetailData {
	detail: SellerCustomerDetail | null;
	orders: Order[];
	orderTotal: number;
}

interface UseCustomerDetailResult {
	data: CustomerDetailData | null;
	loading: boolean;
	error: string | null;
}

/**
 * One CRM customer's detail + purchase history, keyed by buyerId. The body
 * ({@link CustomerDetailPanel}) renders both as a full page and inside the
 * customer-list drawer, so keying by buyerId lets a seller flick between
 * customers — and reopen one — without refetching. Purchase history comes from
 * the proven seller orders pipeline filtered by buyer; the CRM
 * `/customers/{id}/orders` sub-endpoint returns empty. After a classification /
 * tag mutation, invalidate via {@link useInvalidateCustomerDetail} so the panel
 * reflects it on the next open.
 */
export function useCustomerDetail(
	buyerId: string | number,
	options?: { enabled?: boolean },
): UseCustomerDetailResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.seller.customer(identity, buyerId),
		queryFn: async (): Promise<CustomerDetailData> => {
			const [detail, orders] = await Promise.all([
				fetchSellerCustomerDetail(buyerId),
				fetchOrders({
					view: "seller",
					buyerId: String(buyerId),
					pageSize: 100,
				}),
			]);
			return { detail, orders: orders.records, orderTotal: orders.total };
		},
		enabled:
			Boolean(buyerId) &&
			isLoggedIn &&
			activeRole === "seller" &&
			(options?.enabled ?? true),
		staleTime: 30_000,
	});
	return {
		// `data` (not `error || !data`): a failed background refetch keeps the
		// last-good detail rather than blanking the panel.
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
	};
}

/**
 * Returns a callback that invalidates one customer's detail query so the panel
 * refetches — call it after a classification / tag mutation.
 */
export function useInvalidateCustomerDetail(): (
	buyerId: string | number,
) => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	// Stable ref so `handleMutate` (and any host callback wrapping it) keeps a
	// constant identity across re-renders.
	return useCallback(
		(buyerId) =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.customer(identity, buyerId),
			}),
		[queryClient, identity],
	);
}
