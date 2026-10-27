"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { storeIdOf } from "@/lib/seller/store";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { useSellerSelf } from "./useSellerSelf";

interface UseSellerPostsResult {
	count: number;
	loading: boolean;
}

/**
 * Seller's own posts count for the nav badge — shares the same queryKey as
 * SellerPostsClient's list query, so both read one cache entry.
 *
 * ⚠️ 店家鍵在代行下**不能**走 `useSellerSelf()`（那支在代行工作區不掛，見
 * `lib/seller-data/scope.tsx`），改由工作階段下發的 `scope.ownerStoreId` 提供。
 */
export function useSellerPosts(options?: {
	enabled?: boolean;
}): UseSellerPostsResult {
	const { isLoggedIn, activeRole } = useAuth();
	const scope = useSellerDataScope();
	const { data: seller, loading: sellerLoading } = useSellerSelf();
	const sellerId = scope.ownerStoreId ?? storeIdOf(seller);
	const query = useQuery({
		queryKey: scope.keys.posts(sellerId ?? "none", { pageSize: 50 }),
		queryFn: () =>
			scope.repos.posts.fetchSellerPosts(sellerId, { pageSize: 50 }),
		enabled:
			scope.canRead("posts", isLoggedIn && activeRole === "seller") &&
			Boolean(sellerId) &&
			(options?.enabled ?? true),
		staleTime: 30_000,
		refetchOnWindowFocus: true,
	});
	return {
		count: query.data?.length ?? 0,
		loading: sellerLoading || query.isLoading,
	};
}

/**
 * Invalidate the seller's own posts list (all pageSize variants — the `{}`
 * slot partial-matches every variant, same trick as
 * `useInvalidateSellerProducts`) and refetch immediately even though no
 * `SellerPostsClient`/nav badge may be mounted yet — call after create /
 * update / delete, awaited before navigating back to /seller-center/posts,
 * so the list isn't served the pre-redirect 30s-staleTime cache.
 */
export function useInvalidateSellerPosts(): () => Promise<void> {
	const queryClient = useQueryClient();
	const scope = useSellerDataScope();
	const { data: seller } = useSellerSelf();
	const sellerId = scope.ownerStoreId ?? storeIdOf(seller);
	return () =>
		queryClient.invalidateQueries({
			queryKey: scope.keys.posts(sellerId ?? "none"),
			refetchType: "all",
		});
}
