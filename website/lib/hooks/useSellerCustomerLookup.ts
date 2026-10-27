"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchSellerCustomerLookup,
	type SellerCustomerLookup,
} from "@/lib/seller/repo";
import { useQueryIdentity } from "./useQueryIdentity";

export function useSellerCustomerLookup(
	buyerId: string | number | undefined,
	options: { enabled?: boolean } = {},
): SellerCustomerLookup | null {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const id = String(buyerId ?? "");
	const query = useQuery({
		queryKey: queryKeys.seller.customerLookup(identity, id),
		queryFn: ({ signal }) => fetchSellerCustomerLookup(id, { signal }),
		enabled:
			Boolean(id) &&
			isLoggedIn &&
			activeRole === "seller" &&
			(options.enabled ?? true),
		staleTime: 5 * 60_000,
		retry: 1,
	});
	return query.data ?? null;
}
