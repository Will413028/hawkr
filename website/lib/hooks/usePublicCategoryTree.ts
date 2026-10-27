"use client";

import { useQuery } from "@tanstack/react-query";
import {
	type CategoryNode,
	fetchPublicCategoryTree,
} from "@/lib/products/seller-repo";
import { queryKeys } from "@/lib/query/keys";

export interface UsePublicCategoryTreeResult {
	tree: CategoryNode[];
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * Public ACTIVE category tree for read-only classification browsing. Insights
 * uses this source so its JWT-only Pricebook access does not accidentally gain
 * the product-management capability requirement of `/app/categories/tree`.
 */
export function usePublicCategoryTree(
	options: { enabled?: boolean } = {},
): UsePublicCategoryTreeResult {
	const query = useQuery({
		queryKey: queryKeys.categories.publicTree(),
		queryFn: fetchPublicCategoryTree,
		enabled: options.enabled ?? true,
		staleTime: 60 * 60 * 1000,
	});
	return {
		tree: query.data ?? [],
		loading: query.isPending,
		error: query.error ? "分類暫時無法載入，請稍後再試。" : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
