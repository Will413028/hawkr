"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	type ForecastCurveData,
	fetchForecastCurve,
	type PricebookSelection,
	selectionKey,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseForecastCurveResult {
	data: ForecastCurveData | null;
	loading: boolean;
	refreshing: boolean;
	retrying: boolean;
	error: string | null;
	refreshError: string | null;
	retry: () => void;
}

interface ForecastCurveQuerySnapshot {
	data: ForecastCurveData | undefined;
	isLoading: boolean;
	isFetching: boolean;
	isPlaceholderData: boolean;
	error: unknown;
}

type ForecastCurveState = Omit<UseForecastCurveResult, "retry">;

/**
 * Keep TanStack Query's previous-data placeholder as an internal transition
 * detail. A placeholder belongs to the previous selection, so exposing it
 * after the caller has already switched `selection` would label one market's
 * forecast as another market. Treat that interval as loading and withhold the
 * payload until the new query key has real data.
 */
export function mapForecastCurveQueryResult(
	query: ForecastCurveQuerySnapshot,
): ForecastCurveState {
	const hasSelectionMatchedData =
		!query.isPlaceholderData && query.data !== undefined;
	const error = query.error instanceof Error ? query.error.message : null;
	return {
		data: hasSelectionMatchedData ? (query.data ?? null) : null,
		loading: query.isLoading || query.isPlaceholderData,
		refreshing: hasSelectionMatchedData && query.isFetching,
		retrying:
			!hasSelectionMatchedData && query.isFetching && query.error != null,
		error: hasSelectionMatchedData ? null : error,
		refreshError: hasSelectionMatchedData ? error : null,
	};
}

/** 未來 7 天預測曲線（`/sellers/pricebook/forecast-curve`）。selection 為
 *  null 時（尚未選品項）不發請求。 */
export function useForecastCurve(
	selection: PricebookSelection | null,
): UseForecastCurveResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.pricebook.forecastCurve(
			identity,
			selection ? selectionKey(selection) : "",
		),
		queryFn: () => fetchForecastCurve(selection as PricebookSelection),
		enabled: isLoggedIn && activeRole === "seller" && selection != null,
		staleTime: 60_000,
		// Keep the previous payload warm inside TanStack Query during a market
		// switch. mapForecastCurveQueryResult deliberately withholds that
		// placeholder from consumers until the new selection has real data.
		placeholderData: keepPreviousData,
	});
	return {
		...mapForecastCurveQueryResult(query),
		retry: () => {
			void query.refetch();
		},
	};
}
