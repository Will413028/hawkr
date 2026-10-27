"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	fetchMarketOverview,
	type MarketOverviewData,
	type MarketOverviewQuery,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseMarketOverviewResult {
	data: MarketOverviewData | null;
	loading: boolean;
	refreshing: boolean;
	retrying: boolean;
	error: string | null;
	refreshError: string | null;
	retry: () => void;
}

interface MarketOverviewQuerySnapshot {
	data: MarketOverviewData | undefined;
	isLoading: boolean;
	isFetching: boolean;
	error: unknown;
}

type MarketOverviewState = Omit<UseMarketOverviewResult, "retry">;

export function mapMarketOverviewQueryResult(
	query: MarketOverviewQuerySnapshot,
): MarketOverviewState {
	const hasActiveData = query.data !== undefined;
	const message = query.error instanceof Error ? query.error.message : null;
	return {
		data: query.data ?? null,
		loading: query.isLoading,
		refreshing: hasActiveData && query.isFetching,
		retrying: !hasActiveData && query.isFetching && query.error != null,
		error: hasActiveData ? null : message,
		refreshError: hasActiveData ? message : null,
	};
}

/** Identity-scoped batch radar read. A single response carries every visible
 * official-price sparkline; selection-level observed/forecast hooks stay idle
 * while the seller is on the radar home. */
export function useMarketOverview(
	request: MarketOverviewQuery,
	enabled = true,
): UseMarketOverviewResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.pricebook.overview(identity, { ...request }),
		queryFn: () => fetchMarketOverview(request),
		enabled: enabled && isLoggedIn && activeRole === "seller",
		staleTime: 60_000,
	});
	return {
		...mapMarketOverviewQueryResult(query),
		retry: () => {
			void query.refetch();
		},
	};
}
