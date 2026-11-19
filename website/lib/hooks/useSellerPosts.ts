"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { storeIdOf } from "@/lib/seller/store";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { useQueryIdentity } from "./useQueryIdentity";
import { useSellerSelf } from "./useSellerSelf";

interface UseSellerPostsResult {
	count: number;
	loading: boolean;
}

/**
 * Seller's posts count for the nav badge — the server's count (`GetStoreSocialSummary`; the
 * delegated workspace reads the delegated variant), not the length of a capped list page.
 * It has its own query key: the list query's cache entry holds a page, not a count.
 *
 * ⚠️ 店家鍵在代行下**不能**走 `useSellerSelf()`（那支在代行工作區不掛，見
 * `lib/seller-data/scope.tsx`），改由工作階段下發的 `scope.ownerStoreId` 提供。
 */
export function useSellerPosts(options?: {
	enabled?: boolean;
}): UseSellerPostsResult {
	const identity = useQueryIdentity();
	const scope = useSellerDataScope();
	const { data: seller, loading: sellerLoading } = useSellerSelf();
	const sellerId = scope.ownerStoreId ?? storeIdOf(seller);
	const query = useQuery({
		queryKey: scope.keys.posts(sellerId ?? "none", { kind: "count" }),
		queryFn: () => scope.repos.posts.fetchPostCount(),
		enabled:
			scope.canRead("posts", identity.activeRole === "seller") &&
			Boolean(sellerId) &&
			(options?.enabled ?? true),
		staleTime: 30_000,
		refetchOnWindowFocus: true,
	});
	return {
		count: query.data ?? 0,
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
