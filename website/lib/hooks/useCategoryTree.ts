"use client";

import { useQuery } from "@tanstack/react-query";
import {
	type CategoryNode,
	fetchCategoryTree,
} from "@/lib/products/seller-repo";
import { queryKeys } from "@/lib/query/keys";

interface UseCategoryTreeResult {
	tree: CategoryNode[];
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * GET /app/categories/tree 的共用 query。分類樹全站同一份、不分身分，
 * staleTime 1 小時讓建立/編輯/列表間往返共用同一份快取——取代原本
 * CategorySelector 的 module 快取與各頁 useEffect 手刻載入。
 */
export function useCategoryTree(
	options: { enabled?: boolean } = {},
): UseCategoryTreeResult {
	const query = useQuery({
		queryKey: queryKeys.categories.tree(),
		queryFn: fetchCategoryTree,
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
