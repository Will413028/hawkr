import { useInfiniteQuery, useQuery } from "@connectrpc/connect-query";
import {
	type Report,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useMemo, useState } from "react";
import { SOURCE_REPORTS_MAX } from "@/app/admin/merchants/[id]/violation-measure";
import type { CaseReportsView } from "./CaseSections";

const PAGE_SIZE = 100;

export interface CaseReports {
	view: CaseReportsView;
	/** 已讀到的通報（所有頁）。 */
	loaded: readonly Report[];
	/** 建立措施預先勾選的來源通報：案件裡可以當來源的通報中最早的 20 筆（伺服器篩選，依建立時間舊到新）。 */
	sourceReportIds: string[];
	/** 可以當來源的通報超過 20 筆。 */
	sourceOverflow: boolean;
	/** 來源通報讀完了：可以開建立措施的對話框（對話框的初始值只在開啟時取一次）。 */
	sourcesReady: boolean;
	refetch: () => Promise<unknown>;
}

/**
 * 案件裡的通報（舊到新），一頁 100 筆。`scanSources` 時另讀伺服器篩出的「可以當認定來源」的通報（citable_as_source，同樣舊到新）第一頁 21 筆：
 * 超過 20 筆就是有更多，預先勾選的永遠是最早的 20 筆；client 不翻頁、不自己判斷哪些通報可以當來源。
 */
export function useCaseReports(
	caseId: string,
	scanSources: boolean,
): CaseReports {
	const query = useInfiniteQuery(
		SellerGovernanceAdminService.method.listReports,
		{ caseId, pageSize: PAGE_SIZE, pageToken: "" },
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const sources = useQuery(
		SellerGovernanceAdminService.method.listReports,
		{
			caseId,
			citableAsSource: true,
			pageSize: SOURCE_REPORTS_MAX + 1,
			pageToken: "",
		},
		{ enabled: scanSources },
	);
	const [pageIndex, setPageIndex] = useState(0);
	const pages = query.data?.pages ?? [];
	const loaded = useMemo(() => pages.flatMap((page) => page.reports), [pages]);
	const citable = sources.data?.reports ?? [];

	const { hasNextPage, isError } = query;
	const index = Math.min(pageIndex, Math.max(pages.length - 1, 0));
	return {
		view: {
			status: query.isLoading ? "loading" : isError ? "error" : "ready",
			error: query.error,
			rows: pages[index]?.reports ?? [],
			page: index + 1,
			hasNext: index + 1 < pages.length || hasNextPage,
			onPrevious: () => setPageIndex(Math.max(index - 1, 0)),
			onNext: () => {
				void (async () => {
					if (index + 1 >= pages.length) {
						const res = await query.fetchNextPage();
						if (res.isError) return;
					}
					setPageIndex(index + 1);
				})();
			},
			onRetry: () => void query.refetch(),
		},
		loaded,
		sourceReportIds: citable.slice(0, SOURCE_REPORTS_MAX).map((r) => r.id),
		sourceOverflow: citable.length > SOURCE_REPORTS_MAX,
		sourcesReady: sources.isSuccess,
		refetch: () => Promise.all([query.refetch(), sources.refetch()]),
	};
}
