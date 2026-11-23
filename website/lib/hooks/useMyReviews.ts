"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchMyReviewsPage,
	MY_REVIEWS_PAGE_SIZE,
	type Review,
} from "@/lib/reviews/repo";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseMyReviewsResult {
	reviews: Review[];
	loading: boolean;
	loadingMore: boolean;
	/** 讀取失敗（與「沒有評價」分開）。 */
	error: unknown;
	hasMore: boolean;
	loadMore: () => Promise<void>;
}

/** 買家自己寫過的評價（`ListMyReviews`，page_token 分頁）。商品圖、規格與單價由伺服器隨評價回傳。 */
export function useMyReviews(): UseMyReviewsResult {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const query = useInfiniteQuery({
		queryKey: queryKeys.buyer.myReviews(identity),
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchMyReviewsPage({
				pageToken: pageParam,
				pageSize: MY_REVIEWS_PAGE_SIZE,
			}),
		),
		initialPageParam: "",
		getNextPageParam: (lastPage) => lastPage.nextPageToken || undefined,
	});
	const pages = query.data?.pages;
	const reviews = useMemo(
		(): Review[] =>
			status === "signedIn" ? (pages?.flatMap((p) => p.reviews) ?? []) : [],
		[status, pages],
	);
	return {
		reviews,
		loading: accountQueryPending(status, query.isLoading),
		loadingMore: query.isFetchingNextPage,
		error: query.error,
		hasMore: Boolean(query.hasNextPage),
		loadMore: async () => {
			if (!query.isFetchingNextPage && query.hasNextPage) {
				await query.fetchNextPage();
			}
		},
	};
}
