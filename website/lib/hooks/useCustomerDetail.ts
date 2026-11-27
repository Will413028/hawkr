"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { fetchOrders } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { queryKeys } from "@/lib/query/keys";
import { getCustomer, type SellerCustomer } from "@/lib/seller/crm";
import { useCrmStoreId } from "./useCrmStoreId";
import { useQueryIdentity } from "./useQueryIdentity";

interface CustomerDetailData {
	detail: SellerCustomer;
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
 * customers — and reopen one — without refetching. The customer is `GetCustomer`;
 * purchase history comes from the store order list filtered by buyer (CRM exposes no
 * orders, A6-D23 W6). After a classification /
 * tag mutation, invalidate via {@link useInvalidateCustomerDetail} so the panel
 * reflects it on the next open.
 */
export function useCustomerDetail(
	buyerId: string | number,
	options?: { enabled?: boolean },
): UseCustomerDetailResult {
	const identity = useQueryIdentity();
	const { storeId, loading: storeLoading, error: storeError } = useCrmStoreId();
	const defaults = useDefaultErrorText();
	const query = useQuery({
		queryKey: queryKeys.seller.customer(identity, buyerId),
		queryFn: async (): Promise<CustomerDetailData> => {
			const [detail, orders] = await Promise.all([
				getCustomer(storeId as string, String(buyerId)),
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
			Boolean(storeId) &&
			identity.activeRole === "seller" &&
			(options?.enabled ?? true),
		staleTime: 30_000,
	});
	const failure = storeError ?? query.error;
	return {
		data: query.data ?? null,
		loading: storeLoading || query.isLoading,
		error: failure ? errorText(failure, defaults) : null,
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
	return useCallback(
		(buyerId) =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.customer(identity, buyerId),
			}),
		[queryClient, identity],
	);
}
