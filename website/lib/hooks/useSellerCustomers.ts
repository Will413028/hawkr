"use client";

import { errorText } from "@hawkr/api/connect/errors";
import {
	keepPreviousData,
	useInfiniteQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import {
	type ListCustomersParams,
	listCustomers,
	type SellerCustomer,
} from "@/lib/seller/crm";
import { useCrmStoreId } from "./useCrmStoreId";
import { useQueryIdentity } from "./useQueryIdentity";

export type UseSellerCustomersParams = Omit<ListCustomersParams, "pageToken">;

interface UseSellerCustomersResult {
	/** 已載入的客戶；讀取失敗時是 null（與「沒有客戶」的空陣列分開）。 */
	data: SellerCustomer[] | null;
	loading: boolean;
	/** 還有下一頁（`nextPageToken` 非空）。 */
	hasMore: boolean;
	loadingMore: boolean;
	loadMore: () => Promise<void>;
	/** 伺服器文字（或該 code 的預設文字）；沒有失敗時是 null。 */
	error: string | null;
	refresh: () => Promise<void>;
}

/**
 * 店家的客戶列表（`CrmAppService.ListCustomers`），以 `page_token` 往後載入；沒有總數。
 * 篩選與排序在 queryKey，換一組就是新的快取；換篩選時保留上一組的列直到新的載入完。
 * `refetchOnWindowFocus` 關閉：重新讀取只在掛載、換篩選與 invalidate 時發生。
 */
export function useSellerCustomers(
	params: UseSellerCustomersParams = {},
): UseSellerCustomersResult {
	const identity = useQueryIdentity();
	const { storeId, loading: storeLoading, error: storeError } = useCrmStoreId();
	const defaults = useDefaultErrorText();
	const query = useInfiniteQuery({
		queryKey: queryKeys.seller.customers(identity, { storeId, ...params }),
		queryFn: ({ pageParam }) =>
			listCustomers(storeId as string, { ...params, pageToken: pageParam }),
		initialPageParam: "",
		getNextPageParam: (last) => last.nextPageToken || undefined,
		enabled: identity.activeRole === "seller" && Boolean(storeId),
		staleTime: 30_000,
		refetchOnWindowFocus: false,
		placeholderData: keepPreviousData,
	});
	const data = useMemo(
		() => query.data?.pages.flatMap((page) => page.customers) ?? null,
		[query.data],
	);
	const failure = storeError ?? query.error;
	return {
		data,
		loading: storeLoading || query.isLoading,
		hasMore: query.hasNextPage,
		loadingMore: query.isFetchingNextPage,
		loadMore: async () => {
			await query.fetchNextPage();
		},
		error: failure ? errorText(failure, defaults) : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * Invalidate the whole seller-customers family (all param variants) — call after
 * a customer classification / tag mutation so the list row badges + chips
 * refetch. Passing no params yields the `{}` slot, which partial-matches every
 * list variant without touching the per-buyer `customer` detail entries.
 */
export function useInvalidateSellerCustomers(): () => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		() =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.customers(identity),
			}),
		[queryClient, identity],
	);
}
