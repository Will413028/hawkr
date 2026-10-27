"use client";

import { createClient } from "@connectrpc/connect";
import {
	useQuery as useConnectQuery,
	useTransport,
} from "@connectrpc/connect-query";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { skipToken, useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import {
	collectAccountIds,
	type FilterTab,
	listStoresRequest,
	mergeStoreLists,
	planStoreSearch,
	STORES_PER_ACCOUNT,
	type StoreSearch,
	textSearchRequest,
} from "./store-search";

export interface UseStoreSearchInput {
	/** 搜尋框已送出的字串。 */
	keyword: string;
	tab: FilterTab;
	pageToken: string;
	pageSize: number;
	/** false 時不發任何請求（例如搜尋框還沒有字）。預設 true。 */
	enabled?: boolean;
}

/**
 * 店家搜尋：店名、電話、商店代號直接走 ListStores(query)；登入信箱（email 在 identity 不是唯一的）先用
 * ListAccounts(email 完全相符、賣家角色) 走完所有頁找到帳號，再對每個帳號 ListStores(account_id) 合併（K5）。
 * 信箱搜尋的結果只有一頁（符合的店家很少）。
 *
 * 不保留上一個請求的資料：換關鍵字、分頁、翻頁時 `items` 是空的、`isLoading` 是 true，畫面不會把上一個條件的列
 * 顯示在新條件底下。
 */
export function useStoreSearch(input: UseStoreSearchInput) {
	const enabled = input.enabled ?? true;
	const transport = useTransport();
	const clients = useMemo(
		() => ({
			identity: createClient(IdentityAdminService, transport),
			seller: createClient(SellerAdminService, transport),
		}),
		[transport],
	);
	const search: StoreSearch = useMemo(
		() => planStoreSearch(input.keyword),
		[input.keyword],
	);

	const text = useConnectQuery(
		SellerAdminService.method.listStores,
		enabled && search.kind !== "email"
			? textSearchRequest(search, input)
			: skipToken,
	);

	const emailQuery = useQuery({
		queryKey: ["store-search", "email", search, input.tab],
		queryFn:
			enabled && search.kind === "email"
				? async () => {
						const ids = await collectAccountIds(search.email, (request) =>
							clients.identity.listAccounts(request),
						);
						const lists = await Promise.all(
							ids.map(async (accountId) => {
								const res = await clients.seller.listStores(
									listStoresRequest({
										tab: input.tab,
										accountId,
										pageSize: STORES_PER_ACCOUNT,
									}),
								);
								return res.stores;
							}),
						);
						return mergeStoreLists(lists);
					}
				: skipToken,
	});

	if (search.kind === "email") {
		return {
			items: emailQuery.data ?? [],
			nextPageToken: "",
			isLoading: emailQuery.isLoading,
			error: emailQuery.error,
			refetch: () => void emailQuery.refetch(),
		};
	}
	return {
		items: text.data?.stores ?? [],
		nextPageToken: text.data?.nextPageToken ?? "",
		isLoading: text.isLoading,
		error: text.error,
		refetch: () => void text.refetch(),
	};
}
