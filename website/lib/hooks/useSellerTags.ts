"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { type CustomerTag, fetchSellerTags } from "@/lib/seller/repo";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * The seller's whole tag pool (`/app/sellers/me/tags`) — powers the customer-
 * list filter chips and the tag-editor suggestions. Both mount at once while the
 * customer drawer is open, so they share one identity-scoped cache entry instead
 * of each firing its own `fetchSellerTags` (a genuine read dedup, unlike the
 * params-keyed customer list). Read-only and non-critical: a failure resolves to
 * an empty pool so the rest of the page keeps working.
 */
export function useSellerTags(): CustomerTag[] {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.seller.tags(identity),
		queryFn: fetchSellerTags,
		enabled: isLoggedIn && activeRole === "seller",
		staleTime: 60_000,
	});
	return query.data ?? [];
}
