"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import {
	fetchSellerAfterSaleArbitration,
	fetchSellerAfterSaleDetail,
} from "@/lib/after-sales/seller-repo";
import { afterSaleIsAppealLocked } from "@/lib/after-sales/status";
import type {
	AfterSaleArbitration,
	AfterSaleDetail,
} from "@/lib/after-sales/types";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface SellerAfterSaleDetailData {
	detail: AfterSaleDetail;
	/** Arbitration timeline — only pulled once the case is appeal-locked. */
	arbitration: AfterSaleArbitration | null;
}

interface UseSellerAfterSaleDetailResult {
	data: SellerAfterSaleDetailData | null;
	loading: boolean;
	error: string | null;
}

/**
 * One seller after-sale case detail (+ arbitration), keyed by caseNo. Opened
 * from the decision drawer ({@link AfterSaleDecisionDrawer}), so it gates on
 * `enabled` (the drawer's `open`). After confirm-receipt, invalidate via
 * {@link useInvalidateSellerAfterSaleDetail} instead of an imperative reload.
 */
export function useSellerAfterSaleDetail(
	caseNo: string,
	options?: { enabled?: boolean },
): UseSellerAfterSaleDetailResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.seller.afterSale(identity, caseNo),
		queryFn: async (): Promise<SellerAfterSaleDetailData> => {
			const detail = await fetchSellerAfterSaleDetail(caseNo);
			let arbitration: AfterSaleArbitration | null = null;
			if (afterSaleIsAppealLocked(detail.appealState)) {
				try {
					arbitration = await fetchSellerAfterSaleArbitration(caseNo);
				} catch {
					arbitration = null;
				}
			}
			return { detail, arbitration };
		},
		enabled:
			Boolean(caseNo) &&
			isLoggedIn &&
			activeRole === "seller" &&
			(options?.enabled ?? true),
		staleTime: 30_000,
	});
	return {
		// `data` (not `error || !data`): a failed background refetch keeps the
		// last-good detail rather than blanking the drawer.
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
	};
}

/**
 * Returns a stable callback that invalidates one case's detail query so the
 * drawer refetches — call it after confirm-receipt.
 */
export function useInvalidateSellerAfterSaleDetail(): (
	caseNo: string,
) => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		(caseNo) =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.afterSale(identity, caseNo),
			}),
		[queryClient, identity],
	);
}
