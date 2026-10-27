import { useInfiniteQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { useEffect, useMemo } from "react";

const PAGE_SIZE = 100;

export interface AllMeasures {
	/** 已讀到的措施（所有頁）。 */
	measures: Measure[];
	/**
	 * 還在讀（第一頁，或後面的頁還沒讀完）：讀完之前結果不完整，不要拿來判斷「沒有」。背景重讀（失效後更新）時保留上一份資料、不算讀取中，
	 * 畫面不閃「載入中」；`fresh` 例外：每次掛載要的是最新的，重讀完之前快取不能當結果。
	 */
	loading: boolean;
	/** 讀取失敗（不是讀取中）。 */
	failed: boolean;
	error: unknown;
	retry: () => void;
}

/**
 * 依來源案件、來源通報或來源來函列出措施，把所有頁讀完（一頁 100 筆，有下一頁就繼續讀），不會因為超過一頁而悄悄少掉。
 * `fresh` 時每次掛載都重讀（不用快取）。
 */
export function useAllMeasures(
	filter:
		| { sourceCaseId: string }
		| { sourceReportId: string }
		| { sourceAuthorityOrderId: string },
	options: { fresh?: boolean } = {},
): AllMeasures {
	const query = useInfiniteQuery(
		SellerGovernanceAdminService.method.listMeasures,
		{ ...filter, pageSize: PAGE_SIZE, pageToken: "" },
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
			...(options.fresh ? { staleTime: 0, refetchOnMount: "always" } : {}),
		},
	);
	const { hasNextPage, isFetchingNextPage, isError, fetchNextPage } = query;
	useEffect(() => {
		if (hasNextPage && !isFetchingNextPage && !isError) void fetchNextPage();
	}, [hasNextPage, isFetchingNextPage, isError, fetchNextPage]);
	const reading = options.fresh ? query.isFetching : query.isLoading;
	const pages = query.data?.pages;
	const measures = useMemo(
		() => (pages ?? []).flatMap((page) => page.measures),
		[pages],
	);
	return {
		measures,
		loading: reading || (hasNextPage && !isError),
		failed: !reading && isError,
		error: query.error,
		retry: () => void query.refetch(),
	};
}
