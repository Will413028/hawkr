"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchShippingOnboardings,
	type ShippingOnboarding,
} from "@/lib/seller/shipping";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseShippingOnboardingsResult {
	rows: ShippingOnboarding[];
	/** True once a real fetch has resolved. Gate「尚未開通」copy on this so a
	 *  pending load never mislabels every method as not-opened. */
	loaded: boolean;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * The seller's 物流設定 onboarding rows (黑貓 / 7-11). Lets the商品上架
 * 可配送方案 picker tell「已開通」from「尚未開通」instead of blaming every
 * unavailable combination on size / weight. Shared, read-only and small, so the
 * per-SKU editors all read from one cached query.
 */
export function useShippingOnboardings(): UseShippingOnboardingsResult {
	const { activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";
	const query = useQuery({
		queryKey: queryKeys.seller.shippingOnboardings(identity),
		queryFn: fetchShippingOnboardings,
		enabled,
	});
	return {
		rows: query.data ?? [],
		loaded: enabled && query.isSuccess,
		loading: query.isLoading || query.isFetching,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
