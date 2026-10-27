"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { type DefaultErrorText, errorText } from "@hawkr/api/connect/errors";
import { MeasureState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { useAuth } from "@/lib/auth/context";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchHasMeasures,
	fetchMyMeasure,
	fetchMyMeasures,
	type SellerMeasure,
	type SubmitAppealInput,
	submitMeasureAppeal,
	submitMeasureRemediation,
} from "@/lib/seller/measures";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * 措施與申覆以帳號為範圍，伺服器只要求登入（店家全關、停權的帳號一樣讀得到），所以不看 active role。key 仍帶
 * 身分：切換工作區一律整頁重載，不會雙讀。
 */
function useMeasuresEnabled(): boolean {
	const { hydrated, isLoggedIn } = useAuth();
	return hydrated && isLoggedIn;
}

interface UseSellerMeasuresResult {
	records: SellerMeasure[];
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

function failureText(
	error: unknown,
	defaults: DefaultErrorText,
): string | null {
	return error ? errorText(error, defaults) : null;
}

/**
 * 賣家自己的措施（`ListMyMeasures`）。`state` 預設不篩（措施與申覆頁），停權公告傳 `ACTIVE`。
 */
export function useSellerMeasures(
	state: MeasureState = MeasureState.UNSPECIFIED,
): UseSellerMeasuresResult {
	const defaults = useDefaultErrorText();
	const identity = useQueryIdentity();
	const enabled = useMeasuresEnabled();
	const query = useQuery({
		queryKey: queryKeys.seller.measureList(identity, { state }),
		queryFn: () => fetchMyMeasures(state),
		enabled,
		placeholderData: [] as SellerMeasure[],
	});
	const records = enabled ? (query.data ?? []) : [];

	return {
		records,
		loading: query.isLoading || query.isPlaceholderData,
		error: failureText(query.error, defaults),
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * 有沒有措施（`ListMyMeasures` 的第一筆），給開店頁的提示卡。讀取中或失敗都是 false：提示卡不顯示、也不擋開店。
 */
export function useHasMeasures(): boolean {
	const identity = useQueryIdentity();
	const enabled = useMeasuresEnabled();
	const query = useQuery({
		queryKey: queryKeys.seller.measureExists(identity),
		queryFn: () => fetchHasMeasures(),
		enabled,
	});
	return query.data === true;
}

interface UseSellerMeasureResult {
	measure: SellerMeasure | null;
	/** 第一次讀取中（還沒有資料也沒有錯誤）。 */
	loading: boolean;
	error: string | null;
	/** 伺服器說找不到或不屬於目前帳號（`not_found`、`permission_denied`）。 */
	notFound: boolean;
	refetch: () => Promise<void>;
}

/**
 * 單筆措施（`GetMyMeasure`），給 `/seller/measures/[id]` 與兩個請求頁。列表已讀過就用列表裡的那筆當初值
 * （資料新舊沿用列表的更新時間），所以網址可以直接開啟或重新整理。
 */
export function useSellerMeasure(measureId: string): UseSellerMeasureResult {
	const defaults = useDefaultErrorText();
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	const enabled = useMeasuresEnabled();
	const listKey = queryKeys.seller.measureList(identity, {
		state: MeasureState.UNSPECIFIED,
	});
	const query = useQuery({
		queryKey: queryKeys.seller.measure(identity, measureId),
		queryFn: () => fetchMyMeasure(measureId),
		enabled,
		initialData: () =>
			queryClient
				.getQueryData<SellerMeasure[]>(listKey)
				?.find((measure) => measure.id === measureId),
		initialDataUpdatedAt: () =>
			queryClient.getQueryState(listKey)?.dataUpdatedAt,
	});
	const code = query.error ? ConnectError.from(query.error).code : undefined;
	return {
		measure: query.data ?? null,
		loading: query.isPending,
		error: failureText(query.error, defaults),
		notFound: code === Code.NotFound || code === Code.PermissionDenied,
		refetch: async () => {
			await query.refetch();
		},
	};
}

/**
 * 把伺服器剛回的措施寫回單筆與（不篩的）列表的快取（列表沒讀過就不動）；`id` 對得上才換，順序不變。
 */
export function useAdoptMeasure(): (latest: SellerMeasure) => void {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		(latest) => {
			queryClient.setQueryData(
				queryKeys.seller.measure(identity, latest.id),
				latest,
			);
			queryClient.setQueryData<SellerMeasure[]>(
				queryKeys.seller.measureList(identity, {
					state: MeasureState.UNSPECIFIED,
				}),
				(list) =>
					list?.map((measure) => (measure.id === latest.id ? latest : measure)),
			);
		},
		[queryClient, identity],
	);
}

/**
 * Returns a stable callback that invalidates every measure query (lists and single reads) so they
 * refetch after a successful appeal submission or a conflict.
 */
export function useInvalidateSellerMeasures(): () => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		() =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.measures(identity),
			}),
		[queryClient, identity],
	);
}

/**
 * 提出申覆，成功後把回傳的措施寫回快取，再讓措施 family（列表、開店頁的 exists、單筆）重新載入。不包成
 * `useMutation`：請求頁自己持有忙碌與錯誤狀態，只有一個呼叫端。
 */
export function useSubmitSellerAppeal(): (
	measure: Pick<SellerMeasure, "id" | "etag">,
	input: SubmitAppealInput,
) => Promise<SellerMeasure> {
	const invalidate = useInvalidateSellerMeasures();
	const adopt = useAdoptMeasure();
	return useCallback(
		async (measure, input) => {
			const result = await submitMeasureAppeal(measure, input);
			adopt(result);
			await invalidate();
			return result;
		},
		[invalidate, adopt],
	);
}

/** 請求解除；其餘同 `useSubmitSellerAppeal`。 */
export function useSubmitSellerRemediation(): (
	measure: Pick<SellerMeasure, "id" | "etag">,
	input: SubmitAppealInput,
) => Promise<SellerMeasure> {
	const invalidate = useInvalidateSellerMeasures();
	const adopt = useAdoptMeasure();
	return useCallback(
		async (measure, input) => {
			const result = await submitMeasureRemediation(measure, input);
			adopt(result);
			await invalidate();
			return result;
		},
		[invalidate, adopt],
	);
}
