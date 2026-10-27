"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	type CategoryResolutionData,
	fetchCategoryResolution,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseCategoryResolutionResult {
	data: CategoryResolutionData | null;
	loading: boolean;
	error: string | null;
	retry: () => void;
}

/** Pure gating predicate, factored out of the hook body so it's directly
 *  unit-testable without mounting a component (this repo has no React
 *  Testing Library — see the v2 frontend plan's Global Constraints). */
export function isCategoryResolutionEnabled(auth: {
	isLoggedIn: boolean;
	activeRole: string | null;
}): boolean {
	return auth.isLoggedIn && auth.activeRole === "seller";
}

/** 分類權威解析（`/sellers/pricebook/category-resolution`）。`categoryId` 為
 *  `undefined` 時（尚未選到 L3）不發請求——呼叫端仍須自行傳入 `enabled`，
 *  因為「已選到 L3」跟「categoryId 有值」是同一件事但呼叫端不該重複判斷。 */
export function useCategoryResolution(
	categoryId: string | undefined,
	options: { enabled?: boolean } = {},
): UseCategoryResolutionResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled =
		isCategoryResolutionEnabled({ isLoggedIn, activeRole }) &&
		categoryId != null &&
		(options.enabled ?? true);
	const result = useQuery({
		queryKey: queryKeys.pricebook.categoryResolution(
			identity,
			categoryId ?? "",
		),
		queryFn: () => fetchCategoryResolution(categoryId as string),
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
