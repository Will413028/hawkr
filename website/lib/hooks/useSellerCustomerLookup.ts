"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/lib/query/keys";
import { getCustomer } from "@/lib/seller/crm";
import { useCrmStoreId } from "./useCrmStoreId";
import { useQueryIdentity } from "./useQueryIdentity";

export interface SellerCustomerProfile {
	id: string;
	name: string;
	avatar?: string;
}

/**
 * 訂單快照沒有買家暱稱時，補這位買家目前的暱稱與頭像（`GetCustomer`）。客戶只在首筆完成訂單時建立，
 * 所以還沒有完成訂單的買家是 not_found：回 null（呼叫端用自己的後備），不是錯誤。其他失敗（網路、權限）照常拋出，
 * query 進入錯誤狀態，這裡也回 null——這是顯示用的補充資料，不是列表。
 */
export function useSellerCustomerLookup(
	buyerId: string | number | undefined,
	options: { enabled?: boolean } = {},
): SellerCustomerProfile | null {
	const identity = useQueryIdentity();
	const { storeId } = useCrmStoreId();
	const id = String(buyerId ?? "");
	const query = useQuery({
		queryKey: queryKeys.seller.customerLookup(identity, id),
		queryFn: async (): Promise<SellerCustomerProfile | null> => {
			try {
				const c = await getCustomer(storeId as string, id);
				return { id: c.id, name: c.name, avatar: c.avatar };
			} catch (e) {
				if (ConnectError.from(e).code === Code.NotFound) return null;
				throw e;
			}
		},
		enabled:
			Boolean(id) &&
			Boolean(storeId) &&
			identity.activeRole === "seller" &&
			(options.enabled ?? true),
		staleTime: 5 * 60_000,
		retry: 1,
	});
	return query.data ?? null;
}
