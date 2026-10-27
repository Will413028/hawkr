"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import {
	fetchSpatialSurface,
	isSpatialSurfaceUnavailable,
	type PricebookSelection,
	type SpatialSurfaceData,
	SpatialSurfaceUnavailableError,
	spatialSelectionKey,
} from "@/lib/pricebook/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

export interface UseSpatialSurfaceResult {
	data: SpatialSurfaceData | null;
	loading: boolean;
	error: string | null;
	/** status:"unavailable" 的診斷細節（例：當天全國市場共同休市）；一般網路
	 *  或契約錯誤沒有這個細節，維持 null，呼叫端該退回通用錯誤文案。 */
	unavailableDetail: SpatialSurfaceUnavailableError | null;
	retry: () => void;
}

/** Fetch only while the spatial tab is mounted. MapLibre and the large GeoJSON
 * response therefore stay out of the ordinary overview/forecast path. */
export function useSpatialSurface(
	selection: PricebookSelection,
	horizonDays: number,
): UseSpatialSurfaceResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.pricebook.spatialSurface(
			identity,
			spatialSelectionKey(selection),
			horizonDays,
		),
		queryFn: ({ signal }) =>
			fetchSpatialSurface(selection, horizonDays, signal),
		enabled: isLoggedIn && activeRole === "seller",
		// Match the private gateway freshness window. A different crop/group is
		// always a different key; returning to the same item after one minute may
		// refetch its authoritative adaptive scale instead of keeping it for 5 min.
		staleTime: 60_000,
		retry: false,
		// 「當天全國市場共同休市，沒有可用成交資料」是後端業務邏輯的預期結果，
		// 不是我方程式或契約壞掉；沒有這個 meta，全域 QueryCache onError 會把
		// 每一次市場休市都當成例外灌一筆噪音進 Sentry，見 lib/query/client.ts。
		meta: { isExpectedError: isSpatialSurfaceUnavailable },
	});

	return {
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
		unavailableDetail:
			query.error instanceof SpatialSurfaceUnavailableError
				? query.error
				: null,
		retry: () => {
			void query.refetch();
		},
	};
}
