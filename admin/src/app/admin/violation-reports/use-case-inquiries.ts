import { useInfiniteQuery } from "@connectrpc/connect-query";
import type { CaseInquiry } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useEffect, useMemo } from "react";

const PAGE_SIZE = 100;

export interface CaseInquiriesRead {
	/** 已讀到的詢問（所有頁，舊到新）。 */
	inquiries: CaseInquiry[];
	/** 還在讀（第一頁，或後面的頁還沒讀完）。背景重讀時保留上一份資料、不算讀取中。 */
	loading: boolean;
	failed: boolean;
	error: unknown;
	retry: () => void;
}

/** 案件的詢問（舊到新），把所有頁讀完（一頁 100 筆）。 */
export function useCaseInquiries(caseId: string): CaseInquiriesRead {
	const query = useInfiniteQuery(
		SellerGovernanceAdminService.method.listCaseInquiries,
		{ caseId, pageSize: PAGE_SIZE, pageToken: "" },
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const { hasNextPage, isFetchingNextPage, isError, fetchNextPage } = query;
	useEffect(() => {
		if (hasNextPage && !isFetchingNextPage && !isError) void fetchNextPage();
	}, [hasNextPage, isFetchingNextPage, isError, fetchNextPage]);
	const pages = query.data?.pages;
	const inquiries = useMemo(
		() => (pages ?? []).flatMap((page) => page.inquiries),
		[pages],
	);
	return {
		inquiries,
		loading: query.isLoading || (hasNextPage && !isError),
		failed: !query.isLoading && isError,
		error: query.error,
		retry: () => void query.refetch(),
	};
}
