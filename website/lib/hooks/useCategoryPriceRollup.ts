"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	type CategoryPriceRollupData,
	fetchCategoryPriceRollup,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { isCategoryResolutionEnabled } from "./useCategoryResolution";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseCategoryPriceRollupResult {
	data: CategoryPriceRollupData | null;
	loading: boolean;
	error: string | null;
	retry: () => void;
}

/** 分類彙整價格快照（`/sellers/pricebook/category-price-rollup`）——分類樹任一
 *  層（L1/L2/L3）的每日彙整。`categoryId` 為 `undefined` 時（尚未選到任何分類，
 *  或已解析到 L3、改走 `useCategoryResolution` 的完整品項流程）不發請求，比照
 *  `useCategoryResolution` 沿用同一組登入賣家 gating 判斷。 */
export function useCategoryPriceRollup(
	categoryId: string | undefined,
	options: { enabled?: boolean } = {},
): UseCategoryPriceRollupResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled =
		isCategoryResolutionEnabled({ isLoggedIn, activeRole }) &&
		categoryId != null &&
		(options.enabled ?? true);
	const result = useQuery({
		queryKey: queryKeys.pricebook.categoryPriceRollup(
			identity,
			categoryId ?? "",
		),
		queryFn: () => fetchCategoryPriceRollup(categoryId as string),
		enabled,
		staleTime: 30_000,
	});
	return {
		data: result.data ?? null,
		loading: result.isLoading || result.isFetching,
		error: result.error instanceof Error ? result.error.message : null,
		retry: () => {
			void result.refetch();
		},
	};
}
