"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	buildShipmentAnomalyLookup,
	type FetchShipmentAnomaliesParams,
	fetchSellerShipmentAnomalies,
	fetchSellerShipmentAnomalyStats,
	type ShipmentAnomalyItem,
	type ShipmentAnomalyStats,
} from "@/lib/shipping/anomalies";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseSellerShipmentAnomaliesResult {
	records: ShipmentAnomalyItem[];
	total: number;
	page: number;
	pageSize: number;
	/** `orderNo → ShipmentAnomalyItem` lookup for the orders list red dot. */
	lookup: Map<string, ShipmentAnomalyItem>;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * Seller-side shipment anomalies — list + lookup map.
 *
 * Used by:
 *   - `SellerOrdersPage` (lookup map → BuyerCell red dot per row)
 *   - `AnomalyPanel` (orders-page 配送異常 tab — records → list, total drives stats)
 *
 * Auto-refetches when the active role flips to / away from seller so a
 * buyer-side view doesn't leak in stale data. Pulls 100 rows in one go to
 * match the seller's typical live anomaly volume (the dashboard is
 * page-able if it ever exceeds that).
 */
export function useSellerShipmentAnomalies(
	params: FetchShipmentAnomaliesParams = {},
): UseSellerShipmentAnomaliesResult {
	const { activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	// Stable params snapshot — JSON-stringify so the hook only refetches
	// when filter inputs actually change (matches useSellerAfterSales).
	const paramsKey = JSON.stringify(params);
	const enabled = isLoggedIn && activeRole === "seller";
	const emptyResult = {
		records: [] as ShipmentAnomalyItem[],
		total: 0,
		page: params.page ?? 1,
		pageSize: params.pageSize ?? 100,
	};
	const query = useQuery({
		queryKey: queryKeys.seller.shipmentAnomalies(identity, {
			paramsKey,
		}),
		queryFn: () => {
			const parsed = JSON.parse(paramsKey) as FetchShipmentAnomaliesParams;
			return fetchSellerShipmentAnomalies({
				pageSize: 100,
				...parsed,
			});
		},
		enabled,
		// placeholderData (not initialData): initialData writes the empty shape
		// into the cache as fresh, so the global 30s staleTime suppresses the
		// real mount fetch — sellers would see an empty list for up to 30s.
		placeholderData: emptyResult,
	});
	const result = enabled ? (query.data ?? emptyResult) : emptyResult;
	const records = result.records;

	return {
		records,
		total: result.total,
		page: result.page,
		pageSize: result.pageSize,
		lookup: buildShipmentAnomalyLookup(records),
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

interface UseSellerShipmentAnomalyStatsResult {
	stats: ShipmentAnomalyStats;
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/** Lightweight stats-only hook — used by the dashboard banner so the
 *  homepage doesn't need to fetch the full list just to know the count. */
export function useSellerShipmentAnomalyStats(): UseSellerShipmentAnomalyStatsResult {
	const { activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const emptyStats: ShipmentAnomalyStats = {
		total: 0,
		high: 0,
		medium: 0,
		low: 0,
	};
	const enabled = isLoggedIn && activeRole === "seller";
	const query = useQuery({
		queryKey: queryKeys.seller.shipmentAnomalyStats(identity),
		queryFn: fetchSellerShipmentAnomalyStats,
		enabled,
		// See note above: placeholderData keeps the mount fetch alive.
		placeholderData: emptyStats,
	});

	return {
		stats: enabled ? (query.data ?? emptyStats) : emptyStats,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
