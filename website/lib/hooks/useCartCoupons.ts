"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	type CartAvailableCoupon,
	fetchAvailableCoupons,
} from "@/lib/coupons/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseCartCouponsResult {
	coupons: CartAvailableCoupon[];
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		if (signal?.aborted) {
			reject(signal.reason);
			return;
		}
		const timer = window.setTimeout(resolve, ms);
		signal?.addEventListener(
			"abort",
			() => {
				window.clearTimeout(timer);
				reject(signal.reason);
			},
			{ once: true },
		);
	});
}

/**
 * Cart-context coupon listing. Calls `/app/coupons/available?store_id+order_amount`
 * so the backend can return the discount it would apply for *this* basket
 * (`estimatedDiscount`), instead of a raw template list.
 *
 * Debounced (300ms) on input changes so we don't hammer the endpoint while
 * the buyer is fiddling with quantities. Disabled when no seller selected
 * (cart spans multiple sellers or empty) — caller should hide the picker.
 */
export function useCartCoupons({
	storeId,
	orderAmount,
}: {
	storeId: string | null;
	orderAmount: number;
}): UseCartCouponsResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();

	const enabled = hydrated && isLoggedIn && Boolean(storeId) && orderAmount > 0;

	const query = useQuery({
		queryKey: queryKeys.buyer.availableCoupons(identity, {
			storeId: storeId ?? "",
			orderAmount,
		}),
		queryFn: async ({ signal }) => {
			await delay(300, signal);
			const { records } = await fetchAvailableCoupons({
				storeId: storeId ?? undefined,
				orderAmount,
				pageSize: 50,
			});
			return records;
		},
		enabled,
		// keepPreviousData: orderAmount/seller change (debounced) as the buyer
		// edits quantities — keep the prior coupon list visible instead of
		// flashing empty. First-load fallback handled by `?? []` below.
		placeholderData: keepPreviousData,
	});

	return {
		coupons: enabled ? (query.data ?? []) : [],
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
