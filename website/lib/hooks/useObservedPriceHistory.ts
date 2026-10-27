"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	fetchObservedPriceHistory,
	type PricebookSelection,
	type PriceObservation,
	selectionKey,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseObservedPriceHistoryResult {
	observations: PriceObservation[];
	loading: boolean;
	refreshing: boolean;
	retrying: boolean;
	error: string | null;
	refreshError: string | null;
	retry: () => void;
}

interface ObservedPriceHistoryQuerySnapshot {
	data: { observations: PriceObservation[] } | undefined;
	isLoading: boolean;
	isFetching: boolean;
	error: unknown;
}

type ObservedPriceHistoryState = Omit<UseObservedPriceHistoryResult, "retry">;

export function mapObservedPriceHistoryQueryResult(
	query: ObservedPriceHistoryQuerySnapshot,
): ObservedPriceHistoryState {
	const hasActiveData = query.data !== undefined;
	const error = query.error instanceof Error ? query.error.message : null;
	return {
		observations: query.data?.observations ?? [],
		loading: query.isLoading,
		refreshing: hasActiveData && query.isFetching,
		retrying: !hasActiveData && query.isFetching && query.error != null,
		error: hasActiveData ? null : error,
		refreshError: hasActiveData ? error : null,
	};
}

/** 歷史實際成交價（`/sellers/pricebook/observed-price?history=true`）。
 *  selection 為 null 時（尚未選品項）不發請求。 */
export function useObservedPriceHistory(
	selection: PricebookSelection | null,
): UseObservedPriceHistoryResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.pricebook.observedPriceHistory(
			identity,
			selection ? selectionKey(selection) : "",
		),
		queryFn: () => fetchObservedPriceHistory(selection as PricebookSelection),
		enabled: isLoggedIn && activeRole === "seller" && selection != null,
		staleTime: 60_000,
	});
	return {
		...mapObservedPriceHistoryQueryResult(query),
		retry: () => {
			void query.refetch();
		},
	};
}
