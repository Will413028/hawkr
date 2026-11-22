"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import type { GetMyReportResponse } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	useInfiniteQuery,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";
import { getMyReport, listMyReports } from "./api";

/** 詳情頁的資料：`GetMyReport` 與 `AddReportSupplement` 的回應共用這個形狀。 */
export type StoreReportDetail = Pick<
	GetMyReportResponse,
	"report" | "supplements" | "evidence"
>;

/** 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。 */
export function useMyReports() {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const query = useInfiniteQuery({
		queryKey: queryKeys.buyer.myReports(identity),
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			listMyReports(pageParam),
		),
		initialPageParam: "",
		getNextPageParam: (last) => last.nextPageToken || undefined,
	});
	const pages = query.data?.pages;
	const reports = useMemo(
		() => (pages ?? []).flatMap((p) => p.reports),
		[pages],
	);
	return {
		reports,
		loading: accountQueryPending(status, query.isPending),
		error: query.error,
		hasMore: query.hasNextPage,
		loadingMore: query.isFetchingNextPage,
		loadMore: () => void query.fetchNextPage(),
		refetch: () => void query.refetch(),
	};
}

/** 一筆店家檢舉的詳情（`GetMyReport`）。`notFound`：別人的或不存在的 id（伺服器不區分）。 */
export function useMyReport(reportId: string) {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const key = queryKeys.buyer.myReport(identity, reportId);
	const query = useQuery({
		queryKey: key,
		queryFn: accountQueryFn(status, () => getMyReport(reportId)),
	});
	const adopt = useCallback(
		(latest: StoreReportDetail) => {
			queryClient.setQueryData<StoreReportDetail>(key, {
				report: latest.report,
				supplements: latest.supplements,
				evidence: latest.evidence,
			});
		},
		[queryClient, key],
	);
	const invalidate = useCallback(
		() =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.buyer.myReports(identity),
			}),
		[queryClient, identity],
	);
	return {
		detail: query.data ?? null,
		loading: accountQueryPending(status, query.isPending),
		error: query.error,
		notFound:
			query.error != null &&
			ConnectError.from(query.error).code === Code.NotFound,
		refetch: () => void query.refetch(),
		adopt,
		invalidate,
	};
}
