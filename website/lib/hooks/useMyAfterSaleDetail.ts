"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import {
	fetchAfterSaleArbitration,
	fetchMyAfterSaleDetail,
} from "@/lib/after-sales/buyer-repo";
import { afterSaleIsAppealLocked } from "@/lib/after-sales/status";
import type {
	AfterSaleArbitration,
	AfterSaleDetail,
} from "@/lib/after-sales/types";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface AfterSaleDetailData {
	detail: AfterSaleDetail;
	/** Arbitration timeline — only pulled once the case is appeal-locked. */
	arbitration: AfterSaleArbitration | null;
}

interface UseMyAfterSaleDetailResult {
	data: AfterSaleDetailData | null;
	loading: boolean;
	error: string | null;
}

/**
 * One buyer after-sale case detail (+ arbitration), keyed by caseNo. Replaces
 * the per-mount `useEffect`+`load()` in {@link AfterSaleDetailClient}; after a
 * cancel / supplement / appeal, invalidate via
 * {@link useInvalidateMyAfterSaleDetail} instead of an imperative reload.
 */
export function useMyAfterSaleDetail(
	caseNo: string,
): UseMyAfterSaleDetailResult {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.buyer.afterSale(identity, caseNo),
		queryFn: async (): Promise<AfterSaleDetailData> => {
			const detail = await fetchMyAfterSaleDetail(caseNo);
			let arbitration: AfterSaleArbitration | null = null;
			if (afterSaleIsAppealLocked(detail.appealState)) {
				try {
					arbitration = await fetchAfterSaleArbitration(caseNo);
				} catch {
					arbitration = null;
				}
			}
			return { detail, arbitration };
		},
		enabled: Boolean(caseNo) && hydrated && isLoggedIn,
		staleTime: 30_000,
	});
	return {
		// `data` (not `error || !data`): a failed background refetch keeps the
		// last-good detail rather than blanking the page.
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
	};
}

/**
 * Returns a stable callback that invalidates one case's detail query so the
 * page refetches — call it after a buyer action (cancel / supplement / etc.).
 */
export function useInvalidateMyAfterSaleDetail(): (
	caseNo: string,
) => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		(caseNo) =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.buyer.afterSale(identity, caseNo),
			}),
		[queryClient, identity],
	);
}
